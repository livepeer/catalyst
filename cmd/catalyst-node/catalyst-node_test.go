package main

import (
	"fmt"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

const (
	closestNodeAddr = "someurl.com"
	playbackID      = "abc_XYZ-123"
)

var prefixes = [...]string{"video", "videorec", "stream", "playback"}

func randomPlaybackID(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789_-"

	res := make([]byte, length)
	for i := 0; i < length; i++ {
		res[i] = charset[rand.Intn(length)]
	}
	return string(res)
}

func TestPlaybackIDParserWithPrefix(t *testing.T) {
	for i := 0; i < rand.Int()%16+1; i++ {
		id := randomPlaybackID(rand.Int()%24 + 1)
		path := fmt.Sprintf("/hls/%s+%s/index.m3u8", prefixes[rand.Intn(len(prefixes))], id)
		playbackID, _, parsed := parsePlaybackID(path)
		if !parsed {
			t.Fail()
		}
		require.Equal(t, id, playbackID)
	}
}

func TestPlaybackIDParserWithSegment(t *testing.T) {
	for i := 0; i < rand.Int()%16+1; i++ {
		id := randomPlaybackID(rand.Int()%24 + 1)
		seg := "2_1"
		path := fmt.Sprintf("/hls/%s+%s/%s/index.m3u8", prefixes[rand.Intn(len(prefixes))], id, seg)
		playbackID, suffix, parsed := parsePlaybackID(path)
		if !parsed {
			t.Fail()
		}
		require.Equal(t, id, playbackID)
		require.Equal(t, fmt.Sprintf("/hls/%%s/%s/index.m3u8", seg), suffix)
	}
}

func TestPlaybackIDParserWithoutPrefix(t *testing.T) {
	for i := 0; i < rand.Int()%16+1; i++ {
		id := randomPlaybackID(rand.Int()%24 + 1)
		path := fmt.Sprintf("/hls/%s/index.m3u8", id)
		playbackID, _, parsed := parsePlaybackID(path)
		if !parsed {
			t.Fail()
		}
		require.Equal(t, id, playbackID)
	}
}

func getHLSURLs(proto, host string) []string {
	var urls []string
	for _, prefix := range prefixes {
		urls = append(urls, fmt.Sprintf("%s://%s/hls/%s+%s/index.m3u8", proto, host, prefix, playbackID))
	}
	return urls
}

func getJSURLs(proto, host string) []string {
	var urls []string
	for _, prefix := range prefixes {
		urls = append(urls, fmt.Sprintf("%s://%s/json_%s+%s.js", proto, host, prefix, playbackID))
	}
	return urls
}

func getHLSURLsWithSeg(proto, host, seg string) []string {
	var urls []string
	for _, prefix := range prefixes {
		urls = append(urls, fmt.Sprintf("%s://%s/hls/%s+%s/%s/index.m3u8", proto, host, prefix, playbackID, seg))
	}
	return urls
}

func TestRedirectHandler404(t *testing.T) {
	defaultFunc := getClosestNode
	getClosestNode = func(string, string, string, string) (string, error) {
		return closestNodeAddr, fmt.Errorf("No node found")
	}
	defer func() { getClosestNode = defaultFunc }()

	path := fmt.Sprintf("/hls/%s/index.m3u8", playbackID)

	requireReq(t, path).
		result(nil).
		hasStatus(http.StatusFound).
		hasHeader("Location", getHLSURLs("http", closestNodeAddr)...)

	requireReq(t, path).
		withHeader("X-Forwarded-Proto", "https").
		result(nil).
		hasStatus(http.StatusFound).
		hasHeader("Location", getHLSURLs("https", closestNodeAddr)...)
}

func TestRedirectHandlerHLS_Correct(t *testing.T) {
	defaultFunc := getClosestNode
	getClosestNode = func(string, string, string, string) (string, error) { return closestNodeAddr, nil }
	defer func() { getClosestNode = defaultFunc }()

	path := fmt.Sprintf("/hls/%s/index.m3u8", playbackID)

	requireReq(t, path).
		result(nil).
		hasStatus(http.StatusFound).
		hasHeader("Location", getHLSURLs("http", closestNodeAddr)...)

	requireReq(t, path).
		withHeader("X-Forwarded-Proto", "https").
		result(nil).
		hasStatus(http.StatusFound).
		hasHeader("Location", getHLSURLs("https", closestNodeAddr)...)
}

func TestRedirectHandlerHLS_SegmentInPath(t *testing.T) {
	defaultFunc := getClosestNode
	getClosestNode = func(string, string, string, string) (string, error) { return closestNodeAddr, nil }
	defer func() { getClosestNode = defaultFunc }()

	seg := "4_1"
	getParams := "mTrack=0&iMsn=4&sessId=1274784345"
	path := fmt.Sprintf("/hls/%s/%s/index.m3u8?%s", playbackID, seg, getParams)

	requireReq(t, path).
		result(nil).
		hasStatus(http.StatusFound).
		hasHeader("Location", getHLSURLsWithSeg("http", closestNodeAddr, seg)...)
}

func TestRedirectHandlerHLS_InvalidPath(t *testing.T) {
	requireReq(t, "/hls").result(nil).hasStatus(http.StatusNotFound)
	requireReq(t, "/hls").result(nil).hasStatus(http.StatusNotFound)
	requireReq(t, "/hls/").result(nil).hasStatus(http.StatusNotFound)
	requireReq(t, "/hls/12345").result(nil).hasStatus(http.StatusNotFound)
	requireReq(t, "/hls/12345/somepath").result(nil).hasStatus(http.StatusNotFound)
}

func TestRedirectHandlerJS_Correct(t *testing.T) {
	defaultFunc := getClosestNode
	getClosestNode = func(string, string, string, string) (string, error) { return closestNodeAddr, nil }
	defer func() { getClosestNode = defaultFunc }()

	path := fmt.Sprintf("/json_%s.js", playbackID)

	requireReq(t, path).
		result(nil).
		hasStatus(http.StatusFound).
		hasHeader("Location", getJSURLs("http", closestNodeAddr)...)

	requireReq(t, path).
		withHeader("X-Forwarded-Proto", "https").
		result(nil).
		hasStatus(http.StatusFound).
		hasHeader("Location", getJSURLs("https", closestNodeAddr)...)
}

func TestNodeHostRedirect(t *testing.T) {
	hostCli := &catalystNodeCliFlags{NodeHost: "right-host"}
	// Success case; get past the redirect handler and 404
	requireReq(t, "/any/path").
		withHeader("Host", "right-host").
		result(hostCli).
		hasStatus(http.StatusNotFound)

	requireReq(t, "/any/path").
		withHeader("Host", "wrong-host").
		result(hostCli).
		hasStatus(http.StatusFound).
		hasHeader("Location", "http://right-host/any/path")

	requireReq(t, "/any/path").
		withHeader("Host", "wrong-host").
		withHeader("X-Forwarded-Proto", "https").
		result(hostCli).
		hasStatus(http.StatusFound).
		hasHeader("Location", "https://right-host/any/path")
}

type httpReq struct {
	*testing.T
	*http.Request
}

type httpCheck struct {
	*testing.T
	*httptest.ResponseRecorder
}

func requireReq(t *testing.T, path string) httpReq {
	req, err := http.NewRequest("GET", path, nil)
	if err != nil {
		t.Fatal(err)
	}

	return httpReq{t, req}
}

func (hr httpReq) withHeader(key, value string) httpReq {
	hr.Header.Set(key, value)
	return hr
}

func (hr httpReq) result(cli *catalystNodeCliFlags) httpCheck {
	if cli == nil {
		cli = &catalystNodeCliFlags{}
	}
	rr := httptest.NewRecorder()
	redirectHandler(prefixes[:], cli.NodeHost).ServeHTTP(rr, hr.Request)
	return httpCheck{hr.T, rr}
}

func (hc httpCheck) hasStatus(code int) httpCheck {
	require.Equal(hc, code, hc.Code)
	return hc
}

func (hc httpCheck) hasHeader(key string, values ...string) httpCheck {
	header := hc.Header().Get(key)
	require.Contains(hc, values, header)
	return hc
}
