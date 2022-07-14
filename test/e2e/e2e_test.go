package e2e

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/hashicorp/serf/client"
	"github.com/hashicorp/serf/cmd/serf/command"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
)

const (
	webConsolePort = "4242"
	serfPort       = "7373"
	httpPort       = "8080"
	rtmpPort       = "1935"
)

type catalystContainer struct {
	testcontainers.Container
	webConsole string
	serf       string
	http       string
	rtmp       string
}

func startCatalyst(t *testing.T, ctx context.Context, hostname string, network string) *catalystContainer {
	configAbsPath, err := filepath.Abs("../../config")
	require.NoError(t, err)

	var req = testcontainers.ContainerRequest{
		Image:        "livepeer/catalyst",
		ExposedPorts: []string{tcp(webConsolePort), tcp(serfPort), tcp(httpPort), tcp(rtmpPort)},
		ShmSize:      1874000000,
		Hostname:     hostname,
		Name:         hostname,
		Networks:     []string{network},
		Mounts: []testcontainers.ContainerMount{{
			Source: testcontainers.GenericBindMountSource{
				HostPath: configAbsPath,
			},
			Target: "/config"},
		},
		Cmd: []string{"MistController", "-c", fmt.Sprintf("/config/%s.json", hostname)},
	}
	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	require.NoError(t, err)

	catalyst := &catalystContainer{Container: container}

	mappedPort, err := container.MappedPort(ctx, webConsolePort)
	require.NoError(t, err)
	catalyst.webConsole = string(mappedPort.Port())

	mappedPort, err = container.MappedPort(ctx, serfPort)
	require.NoError(t, err)
	catalyst.serf = string(mappedPort.Port())

	mappedPort, err = container.MappedPort(ctx, httpPort)
	require.NoError(t, err)
	catalyst.http = string(mappedPort.Port())

	mappedPort, err = container.MappedPort(ctx, rtmpPort)
	require.NoError(t, err)
	catalyst.rtmp = string(mappedPort.Port())

	return catalyst
}

func tcp(p string) string {
	return fmt.Sprintf("%s/tcp", p)
}

func TestMultiNodeCatalyst(t *testing.T) {
	// given
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	// TODO: start docker-compose with rabbitmq
	// TODO: create network

	// when
	c1 := startCatalyst(t, ctx, "catalyst-one", "catalyst-nodes")
	defer c1.Terminate(ctx)
	c2 := startCatalyst(t, ctx, "catalyst-two", "catalyst-nodes")
	defer c2.Terminate(ctx)

	// then
	// TODO: Change from 5s to active waiting
	time.Sleep(5 * time.Second)
	members := fetchMembers(t, c1.serf)
	require.Len(t, members, 2)
}

func fetchMembers(t *testing.T, serfPort string) []client.Member {
	client, err := command.RPCClient(fmt.Sprintf("127.0.0.1:%s", serfPort), "")
	require.NoError(t, err)
	members, err := client.Members()
	require.NoError(t, err)
	return members
}
