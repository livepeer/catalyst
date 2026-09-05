package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

func TestBoxRecording(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping testing in short mode")
	}

	// given
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	network := createNetwork(ctx, t)
	defer network.Remove(ctx)

	boxName := randomString("box-")
	publicURL := startQuickTunnel(t, "http://127.0.0.1:8888")
	orchestratorURL, orchestratorHostPort := startOrchestratorTunnel(t)

	// when
	box := startBoxWithEnv(ctx, t, boxName, network.name, publicURL, orchestratorURL, orchestratorHostPort)
	defer box.Terminate(ctx)
	waitForBoxMinio(t, publicURL)
	configureBoxObjectStores(t, publicURL)

	for _, mode := range []struct {
		name     string
		copyOnly bool
	}{
		{name: "copy-only", copyOnly: true},
		{name: "transcoded", copyOnly: false},
	} {
		t.Run(mode.name, func(t *testing.T) {
			if err := startRecordTester(ctx, mode.copyOnly); err != nil {
				dumpContainerLogs(ctx, t, box.Container)
				require.NoError(t, err)
			}
		})
	}
}

func startBoxWithEnv(ctx context.Context, t *testing.T, hostname, network, publicURL, orchestratorURL, orchestratorHostPort string) *catalystContainer {
	req := testcontainers.ContainerRequest{
		Image:        "livepeer/in-a-box",
		Hostname:     hostname,
		Name:         hostname,
		Networks:     []string{network},
		ExposedPorts: []string{"1935:1935/tcp", "8888:8888/tcp", fmt.Sprintf("%s:8936/tcp", orchestratorHostPort)},
		ShmSize:      1000000000,
		WaitingFor:   wait.NewLogStrategy("API server listening").WithStartupTimeout(3 * time.Minute),
		Env: map[string]string{
			"LP_API_FRONTEND":      "false",
			"E2E_PUBLIC_URL":       publicURL,
			"E2E_ORCHESTRATOR_URL": orchestratorURL,
		},
		Cmd: []string{
			"bash",
			"-ceu",
			`sed -i \
  -e "s|\"api-server\": \"http://127.0.0.1:3004\"|\"api-server\": \"${E2E_PUBLIC_URL}\"|" \
  -e "s|\"own-base-url\": \"http://127.0.0.1:3060/task-runner\"|\"own-base-url\": \"${E2E_PUBLIC_URL}/task-runner\"|" \
  -e "s|\"orchAddr\": \"127.0.0.1:8936\"|\"orchAddr\": \"${E2E_ORCHESTRATOR_URL}\"|g" \
  -e "s|\"serviceAddr\": \"127.0.0.1:8936\"|\"httpAddr\": \"https://0.0.0.0:8936\", \"serviceAddr\": \"${E2E_ORCHESTRATOR_URL}\"|" \
  /etc/livepeer/full-stack.json
grep -Fq "\"api-server\": \"${E2E_PUBLIC_URL}\"" /etc/livepeer/full-stack.json
grep -Fq "\"own-base-url\": \"${E2E_PUBLIC_URL}/task-runner\"" /etc/livepeer/full-stack.json
grep -Fq "\"serviceAddr\": \"${E2E_ORCHESTRATOR_URL}\"" /etc/livepeer/full-stack.json
exec /usr/local/bin/catalyst -- /usr/local/bin/MistController -c /etc/livepeer/full-stack.json`,
		},
	}
	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          false,
	})
	require.NoError(t, err)

	// Redirect container logs to the standard logger
	lc := logConsumer{name: hostname}
	err = container.StartLogProducer(ctx)
	require.NoError(t, err)
	container.FollowOutput(&lc)

	err = container.Start(ctx)
	require.NoError(t, err)

	// Store mapped ports
	catalyst := &catalystContainer{
		Container: container,
		hostname:  hostname,
	}

	mappedPort, err := container.MappedPort(ctx, boxPort)
	require.NoError(t, err)
	catalyst.box = mappedPort.Port()

	mappedPort, err = container.MappedPort(ctx, rtmpPort)
	require.NoError(t, err)
	catalyst.rtmp = mappedPort.Port()

	// container IP
	cid := container.GetContainerID()
	dockerClient, err := testcontainers.NewDockerClient()
	require.NoError(t, err)
	inspect, err := dockerClient.ContainerInspect(ctx, cid)
	require.NoError(t, err)
	catalyst.ip = inspect.NetworkSettings.Networks[network].IPAddress

	return catalyst
}

const boxAPIToken = "f61b3cdb-d173-4a7a-a0d3-547b871a56f9"

var boxObjectStores = map[string]string{
	"917a2f18-f7a8-4ae3-a849-6efd4aac8e59": "os-vod",
	"517873a4-487c-40ad-872f-027f4bc6bd98": "os-catalyst-vod",
	"cab9266f-5583-4532-9630-7be10d92affe": "os-private",
	"0926e4ba-b726-4386-92ee-5c4583f62f0a": "os-recordings",
}

func waitForBoxMinio(t *testing.T, publicURL string) {
	t.Helper()

	u, err := url.Parse(publicURL)
	require.NoError(t, err)
	cli, err := minio.New(u.Host, &minio.Options{
		Creds:        credentials.NewStaticV4("admin", "password", ""),
		Secure:       true,
		Region:       region,
		BucketLookup: minio.BucketLookupPath,
	})
	require.NoError(t, err)

	deadline := time.Now().Add(time.Minute)
	for {
		requestCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		exists, err := cli.BucketExists(requestCtx, "os-recordings")
		cancel()
		if err == nil && exists {
			return
		}
		if err == nil {
			err = fmt.Errorf("recordings bucket does not exist")
		}
		if time.Now().After(deadline) {
			t.Fatalf("in-a-box MinIO tunnel did not become ready: %v", err)
		}
		time.Sleep(time.Second)
	}
}

func configureBoxObjectStores(t *testing.T, publicURL string) {
	t.Helper()

	client := &http.Client{Timeout: 10 * time.Second}
	for id, bucket := range boxObjectStores {
		deadline := time.Now().Add(time.Minute)
		for {
			err := patchBoxObjectStore(client, publicURL, id, bucket)
			if err == nil {
				break
			}
			if time.Now().After(deadline) {
				t.Fatalf("could not configure object store %s: %v", bucket, err)
			}
			time.Sleep(time.Second)
		}
	}
}

func patchBoxObjectStore(client *http.Client, publicURL, id, bucket string) error {
	storeURL, err := boxObjectStoreURL(publicURL, bucket)
	if err != nil {
		return err
	}
	payload, err := json.Marshal(map[string]string{
		"url":       storeURL,
		"publicUrl": strings.TrimRight(publicURL, "/") + "/" + bucket,
	})
	if err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodPatch, strings.TrimRight(publicURL, "/")+"/api/object-store/"+id, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+boxAPIToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("unexpected status %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}
	return nil
}

func boxObjectStoreURL(publicURL, bucket string) (string, error) {
	u, err := url.Parse(publicURL)
	if err != nil {
		return "", err
	}
	if u.Scheme != "https" || u.Hostname() == "" {
		return "", fmt.Errorf("invalid tunnel URL %q", publicURL)
	}
	u.Scheme = "s3+https"
	u.User = url.UserPassword("admin", "password")
	u.Path = "/" + bucket
	return u.String(), nil
}

func TestBoxObjectStoreURL(t *testing.T) {
	got, err := boxObjectStoreURL("https://example.trycloudflare.com", "os-recordings")
	require.NoError(t, err)
	require.Equal(t, "s3+https://admin:password@example.trycloudflare.com/os-recordings", got)
}

func startRecordTester(ctx context.Context, recordingCopyOnly bool) error {
	startTime := time.Now()
	fmt.Printf("starting record tester copyOnly=%v\n", recordingCopyOnly)
	args := []string{
		"run",
		"github.com/livepeer/stream-tester/cmd/recordtester",
		"-api-server=http://127.0.0.1:8888",
		"-api-token=f61b3cdb-d173-4a7a-a0d3-547b871a56f9",
		"-test-dur=1m",
		"-file=https://github.com/livepeer/catalyst-api/assets/136638730/1f71068a-0396-43c2-b870-95a6ad644ffb",
		"-skip-source-playback",
	}
	if recordingCopyOnly {
		args = append(args, `-recording-spec={"profiles":[]}`)
	}

	output, err := run(ctx, "go", args...)
	fmt.Printf("finished record tester copyOnly=%v duration=%s error=%v output:\n%s\n", recordingCopyOnly, time.Since(startTime), err, output)
	if err != nil {
		return fmt.Errorf("error running recordtester (copyOnly=%v): %w", recordingCopyOnly, err)
	}
	return nil
}

type lockedBuffer struct {
	mu sync.Mutex
	bytes.Buffer
}

func (lw *lockedBuffer) Write(p []byte) (n int, err error) {
	lw.mu.Lock()
	defer lw.mu.Unlock()
	return lw.Buffer.Write(p)
}

func run(ctx context.Context, prog string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, prog, args...)
	cmd.Stdin = os.Stdin
	output := &lockedBuffer{}
	cmd.Stdout = output
	cmd.Stderr = output
	err := cmd.Start()
	if err != nil {
		return output.Bytes(), fmt.Errorf("error invoking %s: %w", prog, err)
	}

	err = cmd.Wait()
	if err != nil {
		return output.Bytes(), fmt.Errorf("error running %s: %w", prog, err)
	}
	return output.Bytes(), nil
}
