package e2e

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/golang/glog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
)

const (
	webConsolePort = "4242"
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
	webConsole          string
	http                string
	catalystAPI         string
	catalystAPIInternal string
	rtmp                string
	ip                  string
	hostname            string
	box                 string
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
	c1 := startCatalyst(ctx, t, h1, network.name, defaultMistConfig(h1, ""))
	defer c1.Terminate(ctx)
	c2 := startCatalyst(ctx, t, h2, network.name, mistConfigConnectTo(h2, h1))
	defer c2.Terminate(ctx)

	// then
	requireMembersJoined(t, c1, c2)

	p := startStream(t, c1)
	defer p.Kill()

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
	mc := defaultMistConfig(host, "")
	for i, p := range mc.Config.Protocols {
		if p.Connector == "livepeer-catalyst-api" {
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
	log.Printf("[%s] %s", lc.name, string(l.Content))
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
		Image: params.ImageName,
		ExposedPorts: []string{
			tcp(webConsolePort),
			tcp(httpPort),
			tcp(catalystAPIPort),
			tcp(catalystAPIInternalPort),
			tcp(rtmpPort),
		},
		Hostname: hostname,
		Name:     hostname,
		Networks: []string{network},
		Env:      envVars,
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

	mappedPort, err = container.MappedPort(ctx, httpPort)
	require.NoError(t, err)
	catalyst.http = mappedPort.Port()

	mappedPort, err = container.MappedPort(ctx, catalystAPIPort)
	require.NoError(t, err)
	catalyst.catalystAPI = mappedPort.Port()

	mappedPort, err = container.MappedPort(ctx, catalystAPIInternalPort)
	require.NoError(t, err)
	catalyst.catalystAPIInternal = mappedPort.Port()

	mappedPort, err = container.MappedPort(ctx, rtmpPort)
	require.NoError(t, err)
	catalyst.rtmp = mappedPort.Port()
	// panic(fmt.Sprintf("catalyst.webConsole: %s, catalyst.http: %s, catalyst.catalystAPI: %s, catalyst.catalystAPIInternal: %s, catalyst.rtmp: %s", catalyst.webConsole, catalyst.http, catalyst.catalystAPI, catalyst.catalystAPIInternal, catalyst.rtmp))

	// container IP
	cid := container.GetContainerID()
	dockerClient, err := testcontainers.NewDockerClient()
	require.NoError(t, err)
	inspect, err := dockerClient.ContainerInspect(ctx, cid)
	require.NoError(t, err)
	catalyst.ip = inspect.NetworkSettings.Networks[network].IPAddress

	return catalyst
}

func tcp(p string) string {
	return fmt.Sprintf("%s/tcp", p)
}

type Member struct {
	Name string            `json:"name"`
	Tags map[string]string `json:"tags"`
}

func requireMembersJoined(t *testing.T, containers ...*catalystContainer) {
	c1 := containers[0]
	correctMembersNumber := func() bool {
		path := fmt.Sprintf("http://127.0.0.1:%s/admin/members", c1.catalystAPIInternal)
		res, err := http.Get(path)
		if err != nil {
			return false
		}
		body, err := ioutil.ReadAll(res.Body)
		if err != nil {
			return false
		}
		members := []Member{}
		if err := json.Unmarshal(body, &members); err != nil {
			return false
		}
		return len(members) == len(containers)
	}
	require.Eventually(t, correctMembersNumber, 5*time.Minute, time.Second)
}

func startStream(t *testing.T, c *catalystContainer) *os.Process {
	ffmpegParams := []string{"-re", "-f", "lavfi", "-i", "testsrc=size=1920x1080:rate=30,format=yuv420p", "-f", "lavfi", "-i", "sine", "-c:v", "libx264", "-b:v", "1000k", "-x264-params", "keyint=60", "-c:a", "aac", "-f", "flv"}
	ffmpegParams = append(ffmpegParams, fmt.Sprintf("rtmp://127.0.0.1:%s/live/stream+foo", c.rtmp))
	cmd := exec.Command("ffmpeg", ffmpegParams...)
	glog.Info("Spawning ffmpeg stream to ingest")
	err := cmd.Start()
	require.NoError(t, err)
	return cmd.Process
}

func requireReplicatedStream(t *testing.T, c *catalystContainer) {
	var errorMsg string
	correctStream := func() bool {
		u := fmt.Sprintf("http://127.0.0.1:%s/hls/stream+foo/index.m3u8", c.http)
		resp, err := http.Get(u)
		if err != nil {
			errorMsg = fmt.Sprintf("error fetching manifest: %s", err)
			return false
		}
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			errorMsg = fmt.Sprintf("error reading response body: %s", err)
			return false
		}
		content := string(body)
		for _, expected := range []string{"RESOLUTION=1920x1080", "FRAME-RATE=30", "index.m3u8"} {
			if !strings.Contains(content, expected) {
				errorMsg = fmt.Sprintf("incorrect manifest: %s did not contain %s", content, expected)
				return false
			}
		}
		glog.Info("Got HLS manifest!")
		return true
	}
	require.Eventually(t, correctStream, 5*time.Minute, time.Second, fmt.Sprintf("Error Detail: %s", errorMsg))
}

func requireStreamRedirection(t *testing.T, c1 *catalystContainer, c2 *catalystContainer) {
	require := require.New(t)
	redirect := func(collectT *assert.CollectT) {
		client := &http.Client{
			CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
				return http.ErrUseLastResponse
			},
		}
		resp, err := client.Get(fmt.Sprintf("http://127.0.0.1:%s/hls/foo/index.m3u8", c1.catalystAPI))
		assert.NoError(collectT, err)
		defer resp.Body.Close()
		assert.Equal(collectT, http.StatusTemporaryRedirect, resp.StatusCode)

		c1URL := fmt.Sprintf("http://%s/hls/stream+foo/index.m3u8", c1.hostname)
		c2URL := fmt.Sprintf("http://%s/hls/stream+foo/index.m3u8", c2.hostname)
		rURL := resp.Header.Get("Location")

		containsURL := strings.Contains(rURL, c1URL) || strings.Contains(rURL, c2URL)
		assert.True(collectT, containsURL, "c1URL=%s c2URL=%s rURL=%s", c1URL, c2URL, rURL)
	}
	require.EventuallyWithT(redirect, 1*time.Minute, time.Second)
}
