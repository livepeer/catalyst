package e2e

import (
	"context"
	"flag"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"
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
	webConsolePort   = "4242"
	httpPort         = "8080"
	httpCatalystPort = "8090"
	rtmpPort         = "1935"
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
	webConsole   string
	serf         string
	http         string
	httpCatalyst string
	catalystAPI  string
	rtmp         string
	ip           string
	hostname     string
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
	c1 := startCatalyst(ctx, t, h1, network.name, defaultMistConfig(h1))
	defer c1.Terminate(ctx)
	c2 := startCatalyst(ctx, t, h2, network.name, mistConfigConnectTo(h2, h1))
	defer c2.Terminate(ctx)

	// then
	requireMembersJoined(t, c1, c2)

	hs := randomString("ffmpeg-")
	s := startStream(ctx, t, hs, network.name, fmt.Sprintf("rtmp://%s/live/stream+foo", c1.hostname))
	defer s.Terminate(ctx)

	requireReplicatedStream(t, c2)
	requireStreamRedirection(t, c1, c2)
}

func createNetwork(ctx context.Context, t *testing.T) *network {
	name := params.NetworkName
	net, err := testcontainers.GenericNetwork(ctx, testcontainers.GenericNetworkRequest{
		NetworkRequest: testcontainers.NetworkRequest{Name: name},
	})
	require.NoError(t, err)

	return &network{Network: net, name: name}
}

func mistConfigConnectTo(host string, connectToHost string) mistConfig {
	mc := defaultMistConfig(host)
	for i, p := range mc.Config.Protocols {
		if p.Connector == "livepeer-catalyst-node" {
			p.RetryJoin = fmt.Sprintf("%s:%s", connectToHost, advertisePort)
			mc.Config.Protocols[i] = p
		}
	}
	return mc
}

type logConsumer struct {
	name string
}

func (lc *logConsumer) Accept(l testcontainers.Log) {
	glog.Infof("[%s] %s", lc.name, string(l.Content))
}

func startCatalyst(ctx context.Context, t *testing.T, hostname, network string, mc mistConfig) *catalystContainer {
	return startCatalystWithEnv(ctx, t, hostname, network, mc, nil)
}

func startCatalystWithEnv(ctx context.Context, t *testing.T, hostname, network string, mc mistConfig, env map[string]string) *catalystContainer {
	mcPath, err := mc.toTmpFile(t.TempDir())
	require.NoError(t, err)
	configAbsPath := filepath.Dir(mcPath)
	mcFile := filepath.Base(mcPath)

	envVars := map[string]string{"CATALYST_NODE_HTTP_ADDR": "0.0.0.0:8090"}
	for k, v := range env {
		envVars[k] = v
	}
	req := testcontainers.ContainerRequest{
		Image:        params.ImageName,
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

func tcp(p string) string {
	return fmt.Sprintf("%s/tcp", p)
}

func requireMembersJoined(t *testing.T, containers ...*catalystContainer) {
	c1 := containers[0]
	correctMembersNumber := func() bool {
		client, err := command.RPCClient(fmt.Sprintf("127.0.0.1:%s", c1.serf), "")
		if err != nil {
			return false
		}
		members, err := client.Members()
		if err != nil {
			return false
		}
		return len(members) == len(containers)
	}
	require.Eventually(t, correctMembersNumber, 5*time.Minute, time.Second)
}

func startStream(ctx context.Context, t *testing.T, hostname, network, target string) *catalystContainer {
	ffmpegParams := []string{"-re", "/BigBuckBunny.mp4", "-c", "copy", "-f", "flv", target}

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
		Started:          false,
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

func requireReplicatedStream(t *testing.T, c *catalystContainer) {
	correctStream := func() bool {
		resp, err := http.Get(fmt.Sprintf("http://localhost:%s/hls/stream+foo/index.m3u8", c.http))
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
				glog.Info("Failed to get HLS manifest")
				return false
			}
		}
		glog.Info("Got HLS manifest!")
		return true
	}
	require.Eventually(t, correctStream, 5*time.Minute, time.Second)
}

func requireNotReplicatedStream(t *testing.T, c *catalystContainer) {
	require := require.New(t)

	resp, err := http.Get(fmt.Sprintf("http://localhost:%s/hls/stream+foo/index.m3u8", c.http))
	require.NoError(err)

	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	require.NoError(err)

	content := string(body)
	for _, expected := range []string{"RESOLUTION=1920x1080", "FRAME-RATE=30", "index.m3u8"} {
		if strings.Contains(content, expected) {
			require.Fail("Stream should not be replicated", "Received replicated stream '%s'", content)
		}
	}
}

func requireStreamRedirection(t *testing.T, c1 *catalystContainer, c2 *catalystContainer) {
	require := require.New(t)
	redirect := func() bool {
		client := &http.Client{
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		}
		resp, err := client.Get(fmt.Sprintf("http://localhost:%s/hls/foo/index.m3u8", c1.httpCatalyst))
		if err != nil {
			return false
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusFound {
			return false
		}

		c1URL := fmt.Sprintf("http://%s/hls/stream+foo/index.m3u8", c1.hostname)
		c2URL := fmt.Sprintf("http://%s/hls/stream+foo/index.m3u8", c2.hostname)
		rURL := resp.Header.Get("Location")
		glog.Infof("c1URL=%s c2URL=%s rURL=%s", c1URL, c2URL, rURL)
		return strings.Contains(rURL, c1URL) || strings.Contains(rURL, c2URL)
	}
	require.Eventually(redirect, 5*time.Minute, time.Second)
}
