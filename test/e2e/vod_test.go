package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
)

const (
	minioPort = "9000"
	username  = "ROOTNAME"
	password  = "CHANGEME123"
	region    = "us-east-1"
	inBucket  = "inbucket"
	source    = "source.mp4"
	outBucket = "outbucket"
)

func TestVod(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping testing in short mode")
	}

	// given
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	network := createNetwork(ctx, t)
	defer network.Remove(ctx)

	m := createMinio(ctx, t, network.name)
	defer m.Terminate(ctx)
	createSourceBucket(t, m)
	uploadSourceVideo(ctx, t, m)
	createDestBucket(t, m)
	storageURL := startQuickTunnel(t, fmt.Sprintf("http://127.0.0.1:%s", m.port))
	waitForTunneledMinio(ctx, t, storageURL)
	callbacks := startCallbackTunnel(t)

	h := randomString("catalyst-")
	sourceOutputURL, err := objectStoreURL(storageURL, username, password, inBucket, "")
	require.NoError(t, err)
	mistConfig := defaultMistConfigWithLivepeerProcess(h, sourceOutputURL)
	mistConfig.setAPIServer(callbacks.apiServerURL)
	mistConfig.setNonLoopbackOrchestrator(h)
	c := startCatalyst(ctx, t, h, network.name, mistConfig)
	defer c.Terminate(ctx)
	waitForCatalystReady(ctx, t, c)

	// when
	requestID := processVod(t, storageURL, callbacks.callbackURL, c)
	if err := callbacks.waitForCompletion(requestID, 10*time.Minute); err != nil {
		dumpContainerLogs(ctx, t, c.Container)
		t.Fatalf("VOD callback failed: %s", redactQuickTunnelURLs(err.Error()))
	}

	// then
	requireOutputFiles(ctx, t, m)
}

type minioContainer struct {
	testcontainers.Container
	hostname string
	port     string
	ip       string
}

func (c *minioContainer) Terminate(ctx context.Context) {
	c.StopLogProducer()
	c.Container.Terminate(ctx)
}

func createMinio(ctx context.Context, t *testing.T, network string) *minioContainer {
	hostname := randomString("minio-")
	envVars := map[string]string{"MINIO_ROOT_USER": username, "MINIO_ROOT_PASSWORD": password}
	req := testcontainers.ContainerRequest{
		Image:        "quay.io/minio/minio",
		ExposedPorts: []string{tcp(minioPort), tcp("9090")},
		Hostname:     hostname,
		Name:         hostname,
		Networks:     []string{network},
		Env:          envVars,
		Cmd:          []string{"server", "/data", "--console-address", ":9090"},
	}
	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	require.NoError(t, err)

	// Redirect container logs to the standard logger
	lc := logConsumer{name: hostname}
	err = container.StartLogProducer(ctx)
	require.NoError(t, err)
	container.FollowOutput(&lc)

	// Store mapped ports
	minio := &minioContainer{
		Container: container,
		hostname:  hostname,
	}

	mappedPort, err := container.MappedPort(ctx, minioPort)
	require.NoError(t, err)
	minio.port = mappedPort.Port()

	// container IP
	cid := container.GetContainerID()
	dockerClient, err := testcontainers.NewDockerClient()
	require.NoError(t, err)
	inspect, err := dockerClient.ContainerInspect(ctx, cid)
	require.NoError(t, err)
	minio.ip = inspect.NetworkSettings.Networks[network].IPAddress

	return minio
}

func createSourceBucket(t *testing.T, m *minioContainer) {
	createBucket(t, m, inBucket)
}

func createDestBucket(t *testing.T, m *minioContainer) {
	createBucket(t, m, outBucket)
}

func waitForTunneledMinio(ctx context.Context, t *testing.T, storageURL string) {
	t.Helper()
	waitForMinio(ctx, t, storageURL, username, password, "MinIO tunnel", func(ctx context.Context, client *minio.Client) error {
		_, err := client.StatObject(ctx, inBucket, source, minio.StatObjectOptions{})
		return err
	})
}

func createBucket(t *testing.T, m *minioContainer, bucket string) {
	err := minioClient(t, m).MakeBucket(context.Background(), bucket, minio.MakeBucketOptions{Region: region, ObjectLocking: true})
	require.NoError(t, err)
}

func uploadSourceVideo(ctx context.Context, t *testing.T, m *minioContainer) {
	_, err := minioClient(t, m).FPutObject(ctx, inBucket, source, source, minio.PutObjectOptions{})
	require.NoError(t, err)
}

func minioClient(t *testing.T, m *minioContainer) *minio.Client {
	cli, err := minioClientForURL(fmt.Sprintf("http://127.0.0.1:%s", m.port), username, password)
	require.NoError(t, err)
	return cli
}

func waitForCatalystReady(ctx context.Context, t *testing.T, c *catalystContainer) {
	t.Helper()

	client := &http.Client{Timeout: 5 * time.Second}
	deadline := time.Now().Add(5 * time.Minute)
	var lastErr error
	for {
		apiURL := fmt.Sprintf("http://127.0.0.1:%s/ok", c.catalystAPIInternal)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
		require.NoError(t, err)
		resp, err := client.Do(req)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				err = fmt.Errorf("Catalyst API readiness returned %s", resp.Status)
			}
		}
		if err == nil {
			probeCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
			exitCode, output, execErr := c.Container.Exec(probeCtx, []string{
				"curl", "--fail", "--silent", "--show-error", "--insecure", "--max-time", "5",
				"https://127.0.0.1:8936/discovery",
			})
			var probeOutput []byte
			if output != nil {
				probeOutput, _ = io.ReadAll(io.LimitReader(output, 4096))
			}
			cancel()
			if execErr == nil && exitCode == 0 {
				return
			}
			lastErr = fmt.Errorf("orchestrator readiness probe failed with exit code %d: %v: %s", exitCode, execErr, redactQuickTunnelURLs(string(probeOutput)))
		} else {
			lastErr = err
		}
		if time.Now().After(deadline) {
			dumpContainerLogs(context.Background(), t, c.Container)
			t.Fatalf("Catalyst did not become ready: %v", lastErr)
		}
		time.Sleep(time.Second)
	}
}

func processVod(t *testing.T, storageURL, callbackURL string, c *catalystContainer) string {
	sourceVideoURL, err := objectStoreURL(storageURL, username, password, inBucket, source)
	require.NoError(t, err)
	destURL, err := objectStoreURL(storageURL, username, password, outBucket, "")
	require.NoError(t, err)
	var jsonData = fmt.Sprintf(`{
			"url": "%s",
			"callback_url": "%s",
		"output_locations": [
			{
									"type": "object_store",
									"url": "%s",
									"outputs": {
										"hls": "enabled"
									}
							}
		]
		}`, sourceVideoURL, callbackURL, destURL)

	url := fmt.Sprintf("http://127.0.0.1:%s/api/vod", c.catalystAPIInternal)
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer([]byte(jsonData)))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer IAmAuthorized")

	resp, err := (&http.Client{Timeout: 30 * time.Second}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4096))
	require.NoError(t, err)
	if resp.StatusCode != http.StatusOK {
		dumpContainerLogs(context.Background(), t, c.Container)
	}
	require.Equal(t, http.StatusOK, resp.StatusCode, "unexpected response: %s", redactQuickTunnelURLs(strings.TrimSpace(string(body))))
	var result struct {
		RequestID string `json:"request_id"`
	}
	require.NoError(t, json.Unmarshal(body, &result))
	require.NotEmpty(t, result.RequestID)
	return result.RequestID
}

func requireOutputFiles(ctx context.Context, t *testing.T, m *minioContainer) {
	cli := minioClient(t, m)
	var files []string
	// The VOD completion callback is sent after the manifests have been written,
	// but segment uploads can still be in flight through the object-store tunnel.
	// Keep the test container alive until those uploads are observable in MinIO.
	timeoutAt := time.Now().Add(2 * time.Minute)

	expectedFiles := []string{
		"index.m3u8",

		"360p0/index.m3u8",
		"360p0/0.ts",

		"720p0/index.m3u8",
		"720p0/0.ts",

		"1080p0/index.m3u8",
		"1080p0/0.ts",

		"metadata.json",
	}

	for timeoutAt.After(time.Now()) {
		files = []string{}
		for o := range cli.ListObjects(ctx, outBucket, minio.ListObjectsOptions{Recursive: true}) {
			require.NoError(t, o.Err)
			files = append(files, o.Key)
		}
		if len(files) < len(expectedFiles) {
			time.Sleep(500 * time.Millisecond)
			continue
		}
		break
	}

	require.Equal(t, len(expectedFiles), len(files), "Expected %v but got %v", expectedFiles, files)
	for _, expectedFile := range expectedFiles {
		require.Contains(t, files, expectedFile)
	}
}
