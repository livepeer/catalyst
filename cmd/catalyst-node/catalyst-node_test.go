package main

import (
	"bytes"
	"fmt"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v4"
	serfclient "github.com/hashicorp/serf/client"
	"github.com/stretchr/testify/require"
)

const (
	closestNodeAddr = "someurl.com"
	playbackID      = "abc_XYZ-123"
)

var fakeSerfMember = &serfclient.Member{
	Tags: map[string]string{
		"http":  fmt.Sprintf("http://%s", closestNodeAddr),
		"https": fmt.Sprintf("https://%s", closestNodeAddr),
		"dtsc":  fmt.Sprintf("dtsc://%s", closestNodeAddr),
	},
}

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

func getHLSURLsWithSeg(proto, host, seg, query string) []string {
	var urls []string
	for _, prefix := range prefixes {
		urls = append(urls, fmt.Sprintf("%s://%s/hls/%s+%s/%s/index.m3u8?%s", proto, host, prefix, playbackID, seg, query))
	}
	return urls
}

func TestRedirectHandler404(t *testing.T) {
	defaultFunc := getClosestNode
	getClosestNode = func(string, string, string, string) (string, error) {
		return closestNodeAddr, fmt.Errorf("No node found")
	}
	defer func() { getClosestNode = defaultFunc }()

	defaultSerf := getSerfMember
	getSerfMember = func(string) (*serfclient.Member, error) { return fakeSerfMember, nil }
	defer func() { getSerfMember = defaultSerf }()

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
	defaultSerf := getSerfMember
	getSerfMember = func(string) (*serfclient.Member, error) { return fakeSerfMember, nil }
	defer func() { getSerfMember = defaultSerf }()

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
	defaultSerf := getSerfMember
	getSerfMember = func(string) (*serfclient.Member, error) { return fakeSerfMember, nil }
	defer func() { getSerfMember = defaultSerf }()

	seg := "4_1"
	getParams := "mTrack=0&iMsn=4&sessId=1274784345"
	path := fmt.Sprintf("/hls/%s/%s/index.m3u8?%s", playbackID, seg, getParams)

	requireReq(t, path).
		result(nil).
		hasStatus(http.StatusFound).
		hasHeader("Location", getHLSURLsWithSeg("http", closestNodeAddr, seg, getParams)...)
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
	defaultSerf := getSerfMember
	getSerfMember = func(string) (*serfclient.Member, error) { return fakeSerfMember, nil }
	defer func() { getSerfMember = defaultSerf }()

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
	requireReq(t, "http://right-host/any/path").
		withHeader("Host", "right-host").
		result(hostCli).
		hasStatus(http.StatusNotFound)

	requireReq(t, "http://wrong-host/any/path").
		result(hostCli).
		hasStatus(http.StatusFound).
		hasHeader("Location", "http://right-host/any/path")

	requireReq(t, "http://wrong-host/any/path?foo=bar").
		result(hostCli).
		hasStatus(http.StatusFound).
		hasHeader("Location", "http://right-host/any/path?foo=bar")

	requireReq(t, "http://wrong-host/any/path").
		withHeader("X-Forwarded-Proto", "https").
		result(hostCli).
		hasStatus(http.StatusFound).
		hasHeader("Location", "https://right-host/any/path")
}

func TestNodeHostPortRedirect(t *testing.T) {
	hostCli := &catalystNodeCliFlags{NodeHost: "right-host:20443"}

	requireReq(t, "http://wrong-host/any/path").
		result(hostCli).
		hasStatus(http.StatusFound).
		hasHeader("Location", "http://right-host:20443/any/path")

	requireReq(t, "http://wrong-host:1234/any/path").
		result(hostCli).
		hasStatus(http.StatusFound).
		hasHeader("Location", "http://right-host:20443/any/path")

	requireReq(t, "http://wrong-host:7777/any/path").
		withHeader("X-Forwarded-Proto", "https").
		result(hostCli).
		hasStatus(http.StatusFound).
		hasHeader("Location", "https://right-host:20443/any/path")

	hostCli = &catalystNodeCliFlags{NodeHost: "right-host"}
	requireReq(t, "http://wrong-host:7777/any/path").
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

type MockClient struct {
	StatusCode int
	Body       []byte
	*http.Client
}

func TestUserNewAccessVerification(t *testing.T) {
	playbackId := "1bbbqz6753hcli1t"
	publicKey := "LS0tLS1CRUdJTiBQVUJMSUMgS0VZLS0tLS0KTUZrd0V3WUhLb1pJemowQ0FRWUlLb1pJemowREFRY0RRZ0FFNzRoTHBSUkx0TzBQS01Vb08yV3ptY2xOemFBaQp6RTd2UnUrdmtHQXFEVzBEVzB5eW9LV3ZKakZNcWdOb0dCakpiZDM2c3ZiTzhVRnN6aXlSZzJYdXlnPT0KLS0tLS1FTkQgUFVCTElDIEtFWS0tLS0tCg=="
	privateKey := "-----BEGIN PRIVATE KEY-----\nMIGHAgEAMBMGByqGSM49AgEGCCqGSM49AwEHBG0wawIBAQQgG1jxreAnbEd/RdtA\nNWIfTiwJzlU7KoBtKlllSMinLtChRANCAATviEulFEu07Q8oxSg7ZbOZyU3NoCLM\nTu9G76+QYCoNbQNbTLKgpa8mMUyqA2gYGMlt3fqy9s7xQWzOLJGDZe7K\n-----END PRIVATE KEY-----\n"
	expiration := time.Now().Add(time.Duration(1 * time.Hour))
	gateURL := "http://localhost:3004/api/access-control/gate"

	// Successful playback with token
	{
		token, _ := craftToken(privateKey, publicKey, playbackId, expiration)
		payload := []byte(fmt.Sprint(playbackId, "\n1\n2\n3\nhttp://localhost:8080/hls/", playbackId, "/index.m3u8?stream=", playbackId, "&token=", token, "\n5"))
		queryGate = func(ac *PlaybackAccessControl, body []byte) (int, int64, int64, error) {
			return 204, 120, 300, nil
		}

		result := executeTest(gateURL, token, payload, queryGate)
		require.Equal(t, "true", result)
	}

	// Successful playback without token
	{
		token := ""
		payload := []byte(fmt.Sprint(playbackId, "\n1\n2\n3\nhttp://localhost:8080/hls/", playbackId, "/index.m3u8?stream=", playbackId, "&token=", token, "\n5"))
		queryGate = func(ac *PlaybackAccessControl, body []byte) (int, int64, int64, error) {
			return 204, 120, 300, nil
		}

		result := executeTest(gateURL, token, payload, queryGate)
		require.Equal(t, "true", result)
	}

	// Fails when token is invalid
	{
		token := "x"
		payload := []byte(fmt.Sprint(playbackId, "\n1\n2\n3\nhttp://localhost:8080/hls/", playbackId, "/index.m3u8?stream=", playbackId, "&token=", token, "\n5"))
		queryGate = func(ac *PlaybackAccessControl, body []byte) (int, int64, int64, error) {
			return 204, 120, 300, nil
		}

		result := executeTest(gateURL, token, payload, queryGate)
		require.Equal(t, "true", result)
	}
}

func executeTest(gateURL, token string, payload []byte, request func(ac *PlaybackAccessControl, body []byte) (int, int64, int64, error)) string {
	req, _ := http.NewRequest("POST", "/triggers", bytes.NewReader(payload))
	req.Header.Add("X-Trigger", UserNewTrigger)

	rr := httptest.NewRecorder()
	handler := triggerHandler(gateURL)

	originalQueryGate := queryGate
	queryGate = request

	handler.ServeHTTP(rr, req)

	queryGate = originalQueryGate

	return rr.Body.String()
}

func craftToken(sk, publicKey, playbackId string, expiration time.Time) (string, error) {
	privateKey, err := jwt.ParseECPrivateKeyFromPEM([]byte(sk))
	if err != nil {
		return "", err
	}

	token := jwt.NewWithClaims(jwt.SigningMethodES256, jwt.MapClaims{
		"sub": playbackId,
		"pub": publicKey,
		"exp": jwt.NewNumericDate(expiration),
	})
	ss, err := token.SignedString(privateKey)

	if err != nil {
		return "", err
	}

	return ss, nil
}
