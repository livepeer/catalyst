package e2e

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"testing"

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

	// when
	box := startBoxWithEnv(ctx, t, boxName, network.name)
	defer box.Terminate(ctx)

	err := startRecordTester(ctx)
	require.NoError(t, err)
}

func startBoxWithEnv(ctx context.Context, t *testing.T, hostname, network string) *catalystContainer {
	req := testcontainers.ContainerRequest{
		Image:        "livepeer/in-a-box",
		Hostname:     hostname,
		Name:         hostname,
		Networks:     []string{network},
		ExposedPorts: []string{"1935:1935/tcp", "8888:8888/tcp"},
		ShmSize:      1000000000,
		WaitingFor:   wait.NewLogStrategy("API server listening"),
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

func startRecordTester(ctx context.Context) error {
	err := run(
		ctx,
		"go",
		"run",
		"github.com/livepeer/stream-tester/cmd/recordtester",
		"-api-server=http://127.0.0.1:8888",
		"-api-token=f61b3cdb-d173-4a7a-a0d3-547b871a56f9",
		"-test-dur=1m",
		"-file=https://github.com/livepeer/catalyst-api/assets/136638730/1f71068a-0396-43c2-b870-95a6ad644ffb",
	)
	if err != nil {
		return fmt.Errorf("error running recordtester: %w", err)
	}

	return nil
}

func run(ctx context.Context, prog string, args ...string) error {
	cmd := exec.CommandContext(ctx, prog, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Start()
	if err != nil {
		return fmt.Errorf("error invoking %s: %w", prog, err)
	}

	err = cmd.Wait()
	if err != nil {
		return fmt.Errorf("error running %s: %w", prog, err)
	}
	return nil
}
