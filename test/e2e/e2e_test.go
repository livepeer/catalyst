package e2e

import (
	"context"
	"flag"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/serf/cmd/serf/command"
	glog "github.com/magicsong/color-glog"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
)

const (
	webConsolePort = "4242"
	serfPort       = "7373"
	httpPort       = "8080"
	rtmpPort       = "1935"
)

type cliParams struct {
	ImageName   string
	NetworkName string
}

var params cliParams

func init() {
	flag.StringVar(&params.ImageName, "image", "livepeer/catalyst", "Docker image to use when loading container")
	flag.StringVar(&params.NetworkName, "network", randomString("catalyst-test-"), "Docker network name to use when starting")
}

func randomString(prefix string) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	const length = 8

	res := make([]byte, length)
	for i := 0; i < length; i++ {
		res[i] = charset[rand.Intn(length)]
	}
	return fmt.Sprintf("%s%s", prefix, string(res))
}

func TestMain(m *testing.M) {
	flag.Parse()
	os.Exit(m.Run())
}

type network struct {
	testcontainers.Network
	name string
}

type catalystContainer struct {
	testcontainers.Container
	webConsole string
	serf       string
	http       string
	rtmp       string
}

func (c *catalystContainer) Terminate(ctx context.Context) {
	c.StopLogProducer()
	c.Container.Terminate(ctx)
}

func TestMultiNodeCatalyst(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping testing in short mode")
	}

	// given
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	network := createNetwork(ctx, t)
	defer network.Remove(ctx)

	h1 := randomString("catalyst-")
	h2 := randomString("catalyst-")

	// when
	c1 := startCatalyst(ctx, t, h1, network.name, mistConfigConnectTo(h2))
	defer c1.Terminate(ctx)
	c2 := startCatalyst(ctx, t, h2, network.name, mistConfigConnectTo(h1))
	defer c2.Terminate(ctx)

	// then
	requireTwoMembers(t, c1, c2)
	requireReplicatedStream(t, c1, c2)
}

func createNetwork(ctx context.Context, t *testing.T) *network {
	name := params.NetworkName
	net, err := testcontainers.GenericNetwork(ctx, testcontainers.GenericNetworkRequest{
		NetworkRequest: testcontainers.NetworkRequest{Name: name},
	})
	require.NoError(t, err)

	return &network{Network: net, name: name}
}

func mistConfigConnectTo(host string) mistConfig {
	mc := defaultMistConfig()
	mc.Config.Protocols = append(mc.Config.Protocols, protocol{
		Connector: "livepeer-catalyst-node",
		RetryJoin: host,
		RPCAddr:   fmt.Sprintf("0.0.0.0:%s", serfPort),
	})
	return mc
}

type logConsumer struct {
	name string
}

func (lc *logConsumer) Accept(l testcontainers.Log) {
	glog.Infof("[%s] %s", lc.name, string(l.Content))
}

func startCatalyst(ctx context.Context, t *testing.T, hostname, network string, mc mistConfig) *catalystContainer {
	mcPath, err := mc.toTmpFile(t.TempDir())
	require.NoError(t, err)
	configAbsPath := filepath.Dir(mcPath)
	mcFile := filepath.Base(mcPath)

	req := testcontainers.ContainerRequest{
		Image:        params.ImageName,
		ExposedPorts: []string{tcp(webConsolePort), tcp(serfPort), tcp(httpPort), tcp(rtmpPort)},
		Hostname:     hostname,
		Networks:     []string{network},
		Mounts: []testcontainers.ContainerMount{{
			Source: testcontainers.GenericBindMountSource{
				HostPath: configAbsPath,
			},
			Target:   "/config",
			ReadOnly: true},
		},
		Cmd: []string{"MistController", "-c", fmt.Sprintf("/config/%s", mcFile)},
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
	catalyst := &catalystContainer{Container: container}

	mappedPort, err := container.MappedPort(ctx, webConsolePort)
	require.NoError(t, err)
	catalyst.webConsole = mappedPort.Port()

	mappedPort, err = container.MappedPort(ctx, serfPort)
	require.NoError(t, err)
	catalyst.serf = mappedPort.Port()

	mappedPort, err = container.MappedPort(ctx, httpPort)
	require.NoError(t, err)
	catalyst.http = mappedPort.Port()

	mappedPort, err = container.MappedPort(ctx, rtmpPort)
	require.NoError(t, err)
	catalyst.rtmp = mappedPort.Port()

	return catalyst
}

func tcp(p string) string {
	return fmt.Sprintf("%s/tcp", p)
}

func requireTwoMembers(t *testing.T, containers ...*catalystContainer) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go logIfStuck(ctx, "STUCK in requireTwoMembers")

	c1 := containers[0]
	numberOfMembersIsTwo := func() bool {
		client, err := command.RPCClient(fmt.Sprintf("127.0.0.1:%s", c1.serf), "")
		if err != nil {
			return false
		}
		members, err := client.Members()
		if err != nil {
			return false
		}
		return len(members) == 2
	}
	require.Eventually(t, numberOfMembersIsTwo, 30*time.Minute, time.Second)
}

func requireReplicatedStream(t *testing.T, c1 *catalystContainer, c2 *catalystContainer) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go logIfStuck(ctx, "STUCK in requireReplicatedStream")

	// Send a stream to the node catalyst-one
	ffmpegParams := []string{"-re", "-f", "lavfi", "-i", "testsrc=size=1920x1080:rate=30,format=yuv420p", "-f", "lavfi", "-i", "sine", "-c:v", "libx264", "-b:v", "1000k", "-x264-params", "keyint=60", "-c:a", "aac", "-f", "flv"}
	ffmpegParams = append(ffmpegParams, fmt.Sprintf("rtmp://localhost:%s/live/stream+foo", c1.rtmp))
	cmd := exec.Command("ffmpeg", ffmpegParams...)
	err := cmd.Start()
	require.NoError(t, err)
	defer cmd.Process.Kill()

	// Read stream from the node catalyst-two
	correctStream := func() bool {
		resp, err := http.Get(fmt.Sprintf("http://localhost:%s/hls/stream+foo/index.m3u8", c2.http))
		if err != nil {
			return false
		}
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return false
		}
		content := string(body)
		for _, expected := range []string{"RESOLUTION=1920x1080", "FRAME-RATE=30", "index.m3u8"} {
			if !strings.Contains(content, expected) {
				return false
			}
		}
		return true
	}
	require.Eventually(t, correctStream, 30*time.Minute, time.Second)
}

func logIfStuck(ctx context.Context, message string) {
	timer := time.NewTimer(5 * time.Minute)
	defer timer.Stop()
	select {
	case <-timer.C:
		fmt.Println()
		fmt.Println("#####")
		fmt.Println(message)
		fmt.Println()
	case <-ctx.Done():
	}
}
