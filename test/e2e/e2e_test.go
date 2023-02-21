package e2e

import (
	"context"
	"flag"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/hashicorp/serf/cmd/serf/command"
	glog "github.com/magicsong/color-glog"
	"github.com/stretchr/testify/require"
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

func TestMultiNodeCatalyst(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping testing in short mode")
	}

	// given
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	network := createNetwork(ctx, t, params.NetworkName)
	defer network.Remove(ctx)

	h1 := randomString("catalyst-")
	h2 := randomString("catalyst-")

	// when
	c1 := startCatalyst(ctx, t, params.ImageName, h1, network.name, defaultMistConfig(h1))
	defer c1.Terminate(ctx)
	c2 := startCatalyst(ctx, t, params.ImageName, h2, network.name, mistConfigConnectTo(h2, h1))
	defer c2.Terminate(ctx)

	// then
	requireMembersJoined(t, c1, c2)

	wg := sync.WaitGroup{}
	for i := 0; i < 5; i += 1 {
		wg.Add(1)
		go func(i int) {
			hs := randomString("ffmpeg-")
			stream := randomString("stream+")
			s := startStream(ctx, t, hs, network.name, fmt.Sprintf("rtmp://%s/live/%s", c1.hostname, stream))
			defer s.Terminate(ctx)

			requireReplicatedStream(t, c2, stream)
			requireStreamRedirection(t, c1, c2, stream)
			wg.Done()
		}(i)
	}
	wg.Wait()
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

func requireReplicatedStream(t *testing.T, c *catalystContainer, stream string) {
	correctStream := func() bool {
		resp, err := http.Get(fmt.Sprintf("http://localhost:%s/hls/%s/index.m3u8", c.http, stream))
		if err != nil {
			return false
		}
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return false
		}
		content := string(body)
		for _, expected := range []string{"RESOLUTION=1280x720", "FRAME-RATE=30", "index.m3u8"} {
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

func requireStreamRedirection(t *testing.T, c1, c2 *catalystContainer, stream string) {
	require := require.New(t)
	wildcard := strings.Split(stream, "+")[1]
	redirect := func() bool {
		client := &http.Client{
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		}
		resp, err := client.Get(fmt.Sprintf("http://localhost:%s/hls/%s/index.m3u8", c1.httpCatalyst, wildcard))
		if err != nil {
			return false
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusFound {
			return false
		}

		c1URL := fmt.Sprintf("http://%s/hls/%s/index.m3u8", c1.hostname, stream)
		c2URL := fmt.Sprintf("http://%s/hls/%s/index.m3u8", c2.hostname, stream)
		rURL := resp.Header.Get("Location")
		glog.Infof("c1URL=%s c2URL=%s rURL=%s", c1URL, c2URL, rURL)
		return strings.Contains(rURL, c1URL) || strings.Contains(rURL, c2URL)
	}
	require.Eventually(redirect, 5*time.Minute, time.Second)
}
