package e2e

import (
	"context"
	"fmt"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"testing"
	"time"
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

func startCatalyst(t *testing.T, ctx context.Context) *catalystContainer {
	req := testcontainers.ContainerRequest{
		Image:        "livepeerci/catalyst:pr-42",
		ExposedPorts: []string{tcp(webConsolePort), tcp(serfPort), tcp(httpPort), tcp(rtmpPort)},
	}
	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	require.NoError(t, err)

	catalyst := &catalystContainer{Container: container}

	mappedPort, err := container.MappedPort(ctx, webConsolePort)
	require.NoError(t, err)
	catalyst.webConsole = string(mappedPort)

	mappedPort, err = container.MappedPort(ctx, serfPort)
	require.NoError(t, err)
	catalyst.serf = string(mappedPort)

	mappedPort, err = container.MappedPort(ctx, httpPort)
	require.NoError(t, err)
	catalyst.http = string(mappedPort)

	mappedPort, err = container.MappedPort(ctx, rtmpPort)
	require.NoError(t, err)
	catalyst.rtmp = string(mappedPort)

	return catalyst
}

func tcp(p string) string {
	return fmt.Sprintf("%s/tcp", p)
}

func TestMultiNodeCatalyst(t *testing.T) {
	// given
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// when
	c1 := startCatalyst(t, ctx)
	defer c1.Terminate(ctx)
	c2 := startCatalyst(t, ctx)
	defer c2.Terminate(ctx)

	// then
	time.Sleep(30 * time.Second)
	// TODO
}
