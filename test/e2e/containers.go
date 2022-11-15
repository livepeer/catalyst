package e2e

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"

	glog "github.com/magicsong/color-glog"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
)

const (
	webConsolePort   = "4242"
	httpPort         = "8080"
	httpCatalystPort = "8090"
	rtmpPort         = "1935"
)

type logConsumer struct {
	name string
}

func (lc *logConsumer) Accept(l testcontainers.Log) {
	glog.Infof("[%s] %s", lc.name, string(l.Content))
}

type catalystContainer struct {
	testcontainers.Container
	webConsole   string
	serf         string
	http         string
	httpCatalyst string
	catalystAPI  string
	rtmp         string
	ip           string
	hostname     string
}

type network struct {
	testcontainers.Network
	name string
}

func (c *catalystContainer) Terminate(ctx context.Context) {
	c.StopLogProducer()
	c.Container.Terminate(ctx)
}

func startCatalyst(ctx context.Context, t *testing.T, image, hostname, network string, mc mistConfig) *catalystContainer {
	return startCatalystWithEnv(ctx, t, image, hostname, network, mc, nil)
}

func tcp(p string) string {
	return fmt.Sprintf("%s/tcp", p)
}

func startCatalystWithEnv(ctx context.Context, t *testing.T, image, hostname, network string, mc mistConfig, env map[string]string) *catalystContainer {
	mcPath, err := mc.toTmpFile(t.TempDir())
	require.NoError(t, err)
	configAbsPath := filepath.Dir(mcPath)
	mcFile := filepath.Base(mcPath)

	envVars := map[string]string{"CATALYST_NODE_HTTP_ADDR": "0.0.0.0:8090"}
	for k, v := range env {
		envVars[k] = v
	}
	req := testcontainers.ContainerRequest{
		Image:        image,
		ExposedPorts: []string{tcp(webConsolePort), tcp(serfPort), tcp(httpPort), tcp(httpCatalystPort), tcp(catalystAPIPort), tcp(rtmpPort)},
		Hostname:     hostname,
		Name:         hostname,
		Networks:     []string{network},
		Env:          envVars,
		Mounts: []testcontainers.ContainerMount{{
			Source: testcontainers.GenericBindMountSource{
				HostPath: configAbsPath,
			},
			Target:   "/config",
			ReadOnly: true},
		},
		Cmd:     []string{"MistController", "-c", fmt.Sprintf("/config/%s", mcFile)},
		ShmSize: 1000000000,
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
	catalyst := &catalystContainer{
		Container: container,
		hostname:  hostname,
	}

	mappedPort, err := container.MappedPort(ctx, webConsolePort)
	require.NoError(t, err)
	catalyst.webConsole = mappedPort.Port()

	mappedPort, err = container.MappedPort(ctx, serfPort)
	require.NoError(t, err)
	catalyst.serf = mappedPort.Port()

	mappedPort, err = container.MappedPort(ctx, httpPort)
	require.NoError(t, err)
	catalyst.http = mappedPort.Port()

	mappedPort, err = container.MappedPort(ctx, httpCatalystPort)
	require.NoError(t, err)
	catalyst.httpCatalyst = mappedPort.Port()

	mappedPort, err = container.MappedPort(ctx, catalystAPIPort)
	require.NoError(t, err)
	catalyst.catalystAPI = mappedPort.Port()

	mappedPort, err = container.MappedPort(ctx, rtmpPort)
	require.NoError(t, err)
	catalyst.rtmp = mappedPort.Port()

	// container IP
	cid := container.GetContainerID()
	dockerClient, _, _, err := testcontainers.NewDockerClient()
	require.NoError(t, err)
	inspect, err := dockerClient.ContainerInspect(ctx, cid)
	require.NoError(t, err)
	catalyst.ip = inspect.NetworkSettings.Networks[network].IPAddress

	return catalyst
}

func createNetwork(ctx context.Context, t *testing.T, name string) *network {
	net, err := testcontainers.GenericNetwork(ctx, testcontainers.GenericNetworkRequest{
		NetworkRequest: testcontainers.NetworkRequest{Name: name},
	})
	require.NoError(t, err)

	return &network{Network: net, name: name}
}

func startStream(ctx context.Context, t *testing.T, hostname, network, target string) *catalystContainer {
	ffmpegParams := []string{"-re", "-i", "/BigBuckBunny.mp4", "-c", "copy", "-f", "flv", target}

	req := testcontainers.ContainerRequest{
		Image:    "iameli/ffmpeg-and-bunny",
		Hostname: hostname,
		Name:     hostname,
		Networks: []string{network},
		Cmd:      ffmpegParams,
		ShmSize:  1000000000,
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
	catalyst := &catalystContainer{
		Container: container,
		hostname:  hostname,
	}

	// container IP
	cid := container.GetContainerID()
	dockerClient, _, _, err := testcontainers.NewDockerClient()
	require.NoError(t, err)
	inspect, err := dockerClient.ContainerInspect(ctx, cid)
	require.NoError(t, err)
	catalyst.ip = inspect.NetworkSettings.Networks[network].IPAddress

	return catalyst
}
