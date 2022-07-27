package e2e

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"strconv"
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
	flag.StringVar(&params.NetworkName, "network", fmt.Sprintf("catalyst-test-%s", randomString()), "Docker network name to use when starting")
}

func randomString() string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	const len = 8

	res := make([]byte, len)
	for i := 0; i < len; i++ {
		res[i] = charset[rand.Intn(len)]
	}
	return string(res)
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

	network := createNetwork(t, ctx)
	defer network.Remove(ctx)

	// when
	c1 := startCatalyst(t, ctx, "catalyst-one", network.name)
	defer c1.Terminate(ctx)
	c2 := startCatalyst(t, ctx, "catalyst-two", network.name)
	defer c2.Terminate(ctx)

//XXX: Calling ffmpeg here does not work. Uncomment this and comment out the ffmpeg call inside requireReplicatedStream then run 'ps aux | grep ffmpeg' while the test is running
//        ffmpegParams := []string{"-re", "-f", "lavfi", "-i", "testsrc=size=1920x1080:rate=30,format=yuv420p", "-f", "lavfi", "-i", "sine", "-c:v", "libx264", "-b:v", "1000k", "-x264-params", "keyint=60", "-c:a", "aac", "-f", "flv"}
//        ffmpegParams = append(ffmpegParams, fmt.Sprintf("rtmp://localhost:%s/live/stream+foo", "1935"))
//        cmd := exec.Command("ffmpeg", ffmpegParams...)
//        err := cmd.Start()
//        defer cmd.Process.Kill()

	// then
	requireTwoMembers(t, c1, c2)
	requireReplicatedStream(t, c1, c2)
}

func createNetwork(t *testing.T, ctx context.Context) *network {
	name := params.NetworkName
	net, err := testcontainers.GenericNetwork(ctx, testcontainers.GenericNetworkRequest{
		NetworkRequest: testcontainers.NetworkRequest{Name: name},
	})
	require.NoError(t, err)

	return &network{Network: net, name: name}
}

type logConsumer struct {
	name string
}

func (lc *logConsumer) Accept(l testcontainers.Log) {
	glog.Infof("[%s] %s", lc.name, string(l.Content))
}

func startCatalyst(t *testing.T, ctx context.Context, hostname, network string) *catalystContainer {
	configAbsPath, err := filepath.Abs("../../config")
	require.NoError(t, err)

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
		Cmd: []string{"MistController", "-c", fmt.Sprintf("/config/%s.json", hostname)},
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

func requireTwoMembers(t *testing.T, c1 *catalystContainer, c2 *catalystContainer) {
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
	require.Eventually(t, numberOfMembersIsTwo, 5*time.Minute, time.Second)
}

func requireReplicatedStream(t *testing.T, c1 *catalystContainer, c2 *catalystContainer) {
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
				glog.Info("Failed to get HLS manifest")
				return false
			}
		}
		glog.Info("Got HLS manifest!")
		return true
	}
	require.Eventually(t, correctStream, 5*time.Minute, time.Second)

	requireProtocolLoad(t, c1, c2)
}

type results struct {
	protocol      string
	viewers       float64
	viewersPassed float64
	score         float64
}

func runProtocolLoadTest(t *testing.T, prot string, url string, viewers int, timeout int) *results {
	dir := t.TempDir()
	fmt.Printf("Testing %s url: %s with %d viewers using tmp dir (%s)\n", prot, url, viewers, dir)
	cmdMistLoadTest := exec.Command("../../bin/./MistLoadTest", "-o", dir, "-n", strconv.Itoa(viewers), "-t", strconv.Itoa(timeout), url)
	out, err := cmdMistLoadTest.Output()
	require.NoError(t, err)
	if err != nil {
		glog.Fatalf("Failed to start MistLoadTest: %s", err)
	}
	glog.Infof("MistLoadTest stdout: %s", out)

	defer cmdMistLoadTest.Process.Kill()

	return parseProtocolResults(t, dir, prot)
}

func parseProtocolResults(t *testing.T, testdir string, prot string) *results {
	matches, err := filepath.Glob(testdir + "/" + "*" + strings.ToUpper(prot) + "*json")
	if err != nil {
		t.Fatalf("Glob failed: %v", err)
	}

	if len(matches) == 0 || len(matches) > 1 {
		t.Fatalf("Expected only one results file but got %v files: %v", len(matches), matches)
	}

	content, err := ioutil.ReadFile(matches[0])
	if err != nil {
		t.Fatalf("Error while opening results file: %s", err)
	}
	glog.Infof("Parsing file: %s", matches[0])

	var payload map[string]interface{}
	err = json.Unmarshal(content, &payload)
	if err != nil {
		t.Fatalf("Error during json.Unmarshal(): %s", err)
	}

	vtotal := float64(payload["viewers"].(float64))
	vpass := float64(payload["viewers_passed"].(float64))
	r := results{
		protocol: prot, 
		viewers: vtotal, 
		viewersPassed: vpass, 		
		score: vpass / vtotal,
	}

	return &r
}

// This test will spawn multiple viewers at node catalyst-two and attempt streaming.
// Any hiccups in playback or failure to connect will be captured and reported via MistLoadTest.
func requireProtocolLoad(t *testing.T, c1 *catalystContainer, c2 *catalystContainer) {
	tests := map[string]struct {
		url     string
		viewers int
		timeout int
		score   float64
	}{
		"hls": {url: fmt.Sprintf("http://localhost:%s/hls/stream+foo/index.m3u8", c2.http), viewers: 5, timeout: 30, score: 0.8},
	}

	// Test each protocol defined in the tests map above
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			r := runProtocolLoadTest(t, name, tc.url, tc.viewers, tc.timeout)
			glog.Infof("Protocol under test: %v, viewers: %v, viewers-passed: %v, score: %v", r.protocol, r.viewers, r.viewersPassed, r.score)
	                if (r.score < tc.score) {
				t.Fatalf("Failed %s test with score: %v", r.protocol, r.score)
			}

		})
	}
}
