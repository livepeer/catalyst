package e2e

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
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

	h := randomString("catalyst-")
	c := startCatalyst(ctx, t, h, network.name, defaultMistConfigWithLivepeerProcess(h))
	defer c.Terminate(ctx)
	waitForCatalystAPI(t, c)

	// when
	processVod(t, m, c)

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
	dockerClient, _, _, err := testcontainers.NewDockerClient()
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

func createBucket(t *testing.T, m *minioContainer, bucket string) {
	err := minioClient(t, m).MakeBucket(context.Background(), bucket, minio.MakeBucketOptions{Region: region, ObjectLocking: true})
	require.NoError(t, err)
}

func uploadSourceVideo(ctx context.Context, t *testing.T, m *minioContainer) {
	_, err := minioClient(t, m).FPutObject(ctx, inBucket, source, source, minio.PutObjectOptions{})
	require.NoError(t, err)
}

func minioClient(t *testing.T, m *minioContainer) *minio.Client {
	cli, err := minio.New(fmt.Sprintf("127.0.0.1:%s", m.port), &minio.Options{
		Creds: credentials.NewStaticV4(username, password, ""),
	})
	require.NoError(t, err)
	return cli
}

func waitForCatalystAPI(t *testing.T, c *catalystContainer) {
	catalystAPIStarted := func() bool {
		url := fmt.Sprintf("http://127.0.0.1:%s/ok", c.catalystAPIInternal)
		resp, err := http.Get(url)
		return err == nil && resp.StatusCode == http.StatusOK
	}

	require.Eventually(t, catalystAPIStarted, 5*time.Minute, time.Second)
}

func processVod(t *testing.T, m *minioContainer, c *catalystContainer) {
	sourceVideoURL := fmt.Sprintf("s3+http://%s:%s@%s:9000/%s/%s", username, password, m.hostname, inBucket, source)
	destURL := fmt.Sprintf("s3+http://%s:%s@%s:9000/%s/output.m3u8", username, password, m.hostname, outBucket)
	var jsonData = fmt.Sprintf(`{
		"url": "%s",
		"callback_url": "http://todo-callback.com",
		"output_locations": [
			{
									"type": "object_store",
									"url": "%s",
									"outputs": {
										"source_segments": true,
										"transcoded_segments": true
									}
							}
		]
	}`, sourceVideoURL, destURL)

	url := fmt.Sprintf("http://127.0.0.1:%s/api/vod", c.catalystAPIInternal)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer([]byte(jsonData)))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer IAmAuthorized")
	client := &http.Client{}
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
}

func requireOutputFiles(ctx context.Context, t *testing.T, m *minioContainer) {
	cli := minioClient(t, m)
	var files []string
	timeoutAt := time.Now().Add(5 * time.Minute)

	expectedFiles := []string{
		"index.m3u8",

		"source/source.mp4.dtsh",
		"source/output.m3u8",
		"source/0.ts",

		"360p0/index.m3u8",
		"360p0/0.ts",

		"720p0/index.m3u8",
		"720p0/0.ts",

		"1080p0/index.m3u8",
		"1080p0/0.ts",
	}

	for timeoutAt.After(time.Now()) {
		files = []string{}
		for o := range cli.ListObjects(ctx, outBucket, minio.ListObjectsOptions{Recursive: true}) {
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
