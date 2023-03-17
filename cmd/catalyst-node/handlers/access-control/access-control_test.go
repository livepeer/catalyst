package accesscontrol

import (
	"bytes"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/stretchr/testify/require"
)

const (
	playbackID     = "1bbbqz6753hcli1t"
	plusPlaybackID = "video+1bbbqz6753hcli1t"
	publicKey      = `LS0tLS1CRUdJTiBQVUJMSUMgS0VZLS0tLS0KTUZrd0V3WUhLb1pJemowQ0FRWUlLb1pJemowREFRY0RRZ0FFNzRoTHBSUkx0TzBQS01Vb08yV3ptY2xOemFBaQp6RTd2UnUrdmtHQXFEVzBEVzB5eW9LV3ZKakZNcWdOb0dCakpiZDM2c3ZiTzhVRnN6aXlSZzJYdXlnPT0KLS0tLS1FTkQgUFVCTElDIEtFWS0tLS0tCg==`
	privateKey     = `-----BEGIN PRIVATE KEY-----
MIGHAgEAMBMGByqGSM49AgEGCCqGSM49AwEHBG0wawIBAQQgG1jxreAnbEd/RdtA
NWIfTiwJzlU7KoBtKlllSMinLtChRANCAATviEulFEu07Q8oxSg7ZbOZyU3NoCLM
Tu9G76+QYCoNbQNbTLKgpa8mMUyqA2gYGMlt3fqy9s7xQWzOLJGDZe7K
-----END PRIVATE KEY-----
`
)

var expiration = time.Now().Add(time.Duration(1 * time.Hour))

type stubGateClient struct{}

func (g *stubGateClient) QueryGate(body []byte) (bool, int32, int32, error) {
	return queryGate(body)
}

var queryGate = func(body []byte) (bool, int32, int32, error) {
	return false, 0, 0, errors.New("not implemented")
}

var allowAccess = func(body []byte) (bool, int32, int32, error) {
	return true, 120, 300, nil
}

var denyAccess = func(body []byte) (bool, int32, int32, error) {
	return false, 120, 300, nil
}

func testTriggerHandler() http.Handler {
	return (&PlaybackAccessControl{
		cache:      make(map[string]map[string]*PlaybackAccessControlEntry),
		gateClient: &stubGateClient{},
	}).TriggerHandler()
}

func TestAllowedAccessValidToken(t *testing.T) {
	token, _ := craftToken(privateKey, publicKey, playbackID, expiration)
	payload := []byte(fmt.Sprint(playbackID, "\n1\n2\n3\nhttp://localhost:8080/hls/", playbackID, "/index.m3u8?stream=", playbackID, "&jwt=", token, "\n5"))

	result := executeFlow(payload, testTriggerHandler(), allowAccess)
	require.Equal(t, "true", result)
}

func TestAllowedAccessValidTokenWithPrefix(t *testing.T) {
	token, _ := craftToken(privateKey, publicKey, playbackID, expiration)
	payload := []byte(fmt.Sprint(plusPlaybackID, "\n1\n2\n3\nhttp://localhost:8080/hls/", plusPlaybackID, "/index.m3u8?stream=", plusPlaybackID, "&jwt=", token, "\n5"))

	result := executeFlow(payload, testTriggerHandler(), allowAccess)
	require.Equal(t, "true", result)
}

func TestAllowdAccessAbsentToken(t *testing.T) {
	token := ""
	payload := []byte(fmt.Sprint(playbackID, "\n1\n2\n3\nhttp://localhost:8080/hls/", playbackID, "/index.m3u8?stream=", playbackID, "&jwt=", token, "\n5"))

	result := executeFlow(payload, testTriggerHandler(), allowAccess)
	require.Equal(t, "true", result)
}

func TestDeniedAccessInvalidToken(t *testing.T) {
	token := "x"
	payload := []byte(fmt.Sprint(playbackID, "\n1\n2\n3\nhttp://localhost:8080/hls/", playbackID, "/index.m3u8?stream=", playbackID, "&jwt=", token, "\n5"))

	result := executeFlow(payload, testTriggerHandler(), allowAccess)
	require.Equal(t, "false", result)
}

func TestDeniedAccess(t *testing.T) {
	token, _ := craftToken(privateKey, publicKey, playbackID, expiration)
	payload := []byte(fmt.Sprint(playbackID, "\n1\n2\n3\nhttp://localhost:8080/hls/", playbackID, "/index.m3u8?stream=", playbackID, "&jwt=", token, "\n5"))

	result := executeFlow(payload, testTriggerHandler(), denyAccess)
	require.Equal(t, "false", result)
}

func TestDeniedAccessForMissingClaims(t *testing.T) {
	token, _ := craftToken(privateKey, "", playbackID, expiration)
	payload := []byte(fmt.Sprint(playbackID, "\n1\n2\n3\nhttp://localhost:8080/hls/", playbackID, "/index.m3u8?stream=", playbackID, "&jwt=", token, "\n5"))

	result := executeFlow(payload, testTriggerHandler(), allowAccess)
	require.Equal(t, "false", result)
}

func TestExpiredToken(t *testing.T) {
	token, _ := craftToken(privateKey, publicKey, playbackID, time.Now().Add(time.Second*-10))
	payload := []byte(fmt.Sprint(playbackID, "\n1\n2\n3\nhttp://localhost:8080/hls/", playbackID, "/index.m3u8?stream=", playbackID, "&jwt=", token, "\n5"))

	result := executeFlow(payload, testTriggerHandler(), allowAccess)
	require.Equal(t, "false", result)
}

func TestCacheHit(t *testing.T) {
	token, _ := craftToken(privateKey, publicKey, playbackID, expiration)
	payload := []byte(fmt.Sprint(playbackID, "\n1\n2\n3\nhttp://localhost:8080/hls/", playbackID, "/index.m3u8?stream=", playbackID, "&jwt=", token, "\n5"))
	handler := testTriggerHandler()

	var callCount = 0
	var countableAllowAccess = func(body []byte) (bool, int32, int32, error) {
		callCount++
		return true, 10, 20, nil
	}

	executeFlow(payload, handler, countableAllowAccess)
	require.Equal(t, 1, callCount)

	executeFlow(payload, handler, countableAllowAccess)
	require.Equal(t, 1, callCount)
}

func TestStaleCache(t *testing.T) {
	token, _ := craftToken(privateKey, publicKey, playbackID, expiration)
	payload := []byte(fmt.Sprint(playbackID, "\n1\n2\n3\nhttp://localhost:8080/hls/", playbackID, "/index.m3u8?stream=", playbackID, "&jwt=", token, "\n5"))
	handler := testTriggerHandler()

	var callCount = 0
	var countableAllowAccess = func(body []byte) (bool, int32, int32, error) {
		callCount++
		return true, -10, 20, nil
	}

	// Assign testable function ourselves so executeFlow() can't restore original
	original := queryGate
	queryGate = countableAllowAccess
	defer func() { queryGate = original }()

	// Cache entry is absent and a first remote call is done
	executeFlow(payload, handler, countableAllowAccess)
	// Flow is executed a second time, cache is used but a remote call is scheduled
	executeFlow(payload, handler, countableAllowAccess)
	// Remote call count is still 1
	require.Equal(t, 1, callCount)

	// After the scheduled call is executed and call count is incremented
	time.Sleep(1 * time.Second)
	require.Equal(t, 2, callCount)

	queryGate = original

	executeFlow(payload, handler, countableAllowAccess)
	require.Equal(t, 2, callCount)
}

func TestInvalidCache(t *testing.T) {
	token, _ := craftToken(privateKey, publicKey, playbackID, expiration)
	payload := []byte(fmt.Sprint(playbackID, "\n1\n2\n3\nhttp://localhost:8080/hls/", playbackID, "/index.m3u8?stream=", playbackID, "&jwt=", token, "\n5"))
	handler := testTriggerHandler()

	var callCount = 0
	var countableAllowAccess = func(body []byte) (bool, int32, int32, error) {
		callCount++
		return true, -10, -20, nil
	}

	executeFlow(payload, handler, countableAllowAccess)
	executeFlow(payload, handler, countableAllowAccess)

	require.Equal(t, 2, callCount)
}

func executeFlow(payload []byte, handler http.Handler, request func(body []byte) (bool, int32, int32, error)) string {
	original := queryGate
	queryGate = request

	req, _ := http.NewRequest("POST", "/triggers", bytes.NewReader(payload))
	req.Header.Add("X-Trigger", UserNewTrigger)

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	queryGate = original

	return rr.Body.String()
}

func craftToken(sk, publicKey, playbackID string, expiration time.Time) (string, error) {
	privateKey, err := jwt.ParseECPrivateKeyFromPEM([]byte(sk))
	if err != nil {
		return "", err
	}

	token := jwt.NewWithClaims(jwt.SigningMethodES256, jwt.MapClaims{
		"sub": playbackID,
		"pub": publicKey,
		"exp": jwt.NewNumericDate(expiration),
	})

	ss, err := token.SignedString(privateKey)
	if err != nil {
		return "", err
	}

	return ss, nil
}
