package e2e

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"golang.org/x/sync/errgroup"
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

	// when
	box := startBoxWithEnv(ctx, t, boxName, network.name)
	defer box.Terminate(ctx)

	eg, ctx := errgroup.WithContext(ctx)
	eg.Go(func() error {
		return startRecordTester(ctx, false)
	})
	eg.Go(func() error {
		return startRecordTester(ctx, true)
	})
	require.NoError(t, eg.Wait())
}

func startBoxWithEnv(ctx context.Context, t *testing.T, hostname, network string) *catalystContainer {
	req := testcontainers.ContainerRequest{
		Image:        "livepeer/in-a-box",
		Hostname:     hostname,
		Name:         hostname,
		Networks:     []string{network},
		ExposedPorts: []string{"1935:1935/tcp", "8888:8888/tcp"},
		ShmSize:      1000000000,
		WaitingFor:   wait.NewLogStrategy("API server listening").WithStartupTimeout(3 * time.Minute),
		Env: map[string]string{
			"LP_API_FRONTEND": "false",
		},
	}
	genericContainerRequest := testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          false,
	}
	lc := logConsumer{name: hostname}
	err := testcontainers.WithLogConsumers(&lc)(&genericContainerRequest)
	require.NoError(t, err)
	container, err := testcontainers.GenericContainer(ctx, genericContainerRequest)
	require.NoError(t, err)

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
