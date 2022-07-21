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

func cleanupFiles(ext string) {
	matches, err := filepath.Glob(ext)
	if err != nil {
		panic(fmt.Errorf("Glob failed: %w", err))
	}
	glog.Infof("Removing leftover files: %s", matches)
	for _, filename := range matches {
		os.Remove(filename)
	}
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

	// Define a set of protocols to test at output of node catalyst-two
	protocols := map[string]string{
		"hls": fmt.Sprintf("http://localhost:%s/hls/stream+foo/index.m3u8", c2.http),
		// add more protocol <-> url mapping here to test in the future
	}

	// rm any old output files from previous MistLoadTest runs
	cleanupFiles("*json")
	cleanupFiles("*html")

	// For each protocol defined above, check for replicated stream output
	// at node catalyst-two (implicitly tests DTSC path in-between the nodes
	for prot, url := range protocols {

		glog.Infof("Testing %s using url: %s", prot, url)

		cmdMistLoadTest := exec.Command("../../bin/./MistLoadTest", url)
		out, err := cmdMistLoadTest.Output()
		require.NoError(t, err)
		if err != nil {
			glog.Fatalf("Failed to start MistLoadTest: %s", err)
		}
		glog.Infof("MistLoadTest stdout: %s", out)

		defer cmdMistLoadTest.Process.Kill()

	}

	// Parse output results for each protocol tested above
	var p bool = true
	for prot := range protocols {

		matches, err := filepath.Glob("*" + strings.ToUpper(prot) + "*json")
		if err != nil {
			panic(fmt.Errorf("Glob failed: %w", err))
		}
		fmt.Println(matches)

		content, err := ioutil.ReadFile(matches[0])
		if err != nil {
			glog.Fatalf("Error while opening output file: %s", err)
			p = false
			break
		}

		var payload map[string]interface{}
		err = json.Unmarshal(content, &payload)
		if err != nil {
			glog.Fatalf("Error during json.Unmarshal(): %s", err)
			p = false
			break
		}

		v := payload["viewers"]
		vp := payload["viewers_passed"]

		glog.Infof("Result: %d/%d viewers passed successfully for %s protocol", int(vp.(float64)), int(v.(float64)), prot)
		if v != vp {
			p = false
			break
		}
	}

	// rm output files
	cleanupFiles("*json")
	cleanupFiles("*html")

	require.Equal(t, p, true, "Failed to verify output stream")

}
