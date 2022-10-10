package accesscontrol

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/stretchr/testify/require"
)

type MockClient struct {
	StatusCode int
	Body       []byte
	*http.Client
}

const playbackID = "1bbbqz6753hcli1t"
const publicKey = "LS0tLS1CRUdJTiBQVUJMSUMgS0VZLS0tLS0KTUZrd0V3WUhLb1pJemowQ0FRWUlLb1pJemowREFRY0RRZ0FFNzRoTHBSUkx0TzBQS01Vb08yV3ptY2xOemFBaQp6RTd2UnUrdmtHQXFEVzBEVzB5eW9LV3ZKakZNcWdOb0dCakpiZDM2c3ZiTzhVRnN6aXlSZzJYdXlnPT0KLS0tLS1FTkQgUFVCTElDIEtFWS0tLS0tCg=="
const privateKey = "-----BEGIN PRIVATE KEY-----\nMIGHAgEAMBMGByqGSM49AgEGCCqGSM49AwEHBG0wawIBAQQgG1jxreAnbEd/RdtA\nNWIfTiwJzlU7KoBtKlllSMinLtChRANCAATviEulFEu07Q8oxSg7ZbOZyU3NoCLM\nTu9G76+QYCoNbQNbTLKgpa8mMUyqA2gYGMlt3fqy9s7xQWzOLJGDZe7K\n-----END PRIVATE KEY-----\n"
const gateURL = "http://localhost:3000/api/access-control/gate"

var expiration = time.Now().Add(time.Duration(1 * time.Hour))

var allowAccess = func(ac *PlaybackAccessControl, body []byte) (bool, int32, int32, error) {
	return true, 120, 300, nil
}

var denyAccess = func(ac *PlaybackAccessControl, body []byte) (bool, int32, int32, error) {
	return false, 120, 300, nil
}

func TestAllowedAccessValidToken(t *testing.T) {
	token, _ := craftToken(privateKey, publicKey, playbackID, expiration)
	payload := []byte(fmt.Sprint(playbackID, "\n1\n2\n3\nhttp://localhost:8080/hls/", playbackID, "/index.m3u8?stream=", playbackID, "&token=", token, "\n5"))

	result := executeFlow(token, payload, TriggerHandler(gateURL), allowAccess)
	require.Equal(t, "true", result)
}

func TestAllowdAccessAbsentToken(t *testing.T) {
	token := ""
	payload := []byte(fmt.Sprint(playbackID, "\n1\n2\n3\nhttp://localhost:8080/hls/", playbackID, "/index.m3u8?stream=", playbackID, "&token=", token, "\n5"))

	result := executeFlow(token, payload, TriggerHandler(gateURL), allowAccess)
	require.Equal(t, "true", result)
}

func TestDeniedAccessInvalidToken(t *testing.T) {
	token := "x"
	payload := []byte(fmt.Sprint(playbackID, "\n1\n2\n3\nhttp://localhost:8080/hls/", playbackID, "/index.m3u8?stream=", playbackID, "&jwt=", token, "\n5"))

	result := executeFlow(token, payload, TriggerHandler(gateURL), allowAccess)
	require.Equal(t, "false", result)
}

func TestDeniedAccess(t *testing.T) {
	token, _ := craftToken(privateKey, publicKey, playbackID, expiration)
	payload := []byte(fmt.Sprint(playbackID, "\n1\n2\n3\nhttp://localhost:8080/hls/", playbackID, "/index.m3u8?stream=", playbackID, "&jwt=", token, "\n5"))

	result := executeFlow(token, payload, TriggerHandler(gateURL), denyAccess)
	require.Equal(t, "false", result)
}

func TestDeniedAccessForMissingClaims(t *testing.T) {
	token, _ := craftToken(privateKey, "", playbackID, expiration)
	payload := []byte(fmt.Sprint(playbackID, "\n1\n2\n3\nhttp://localhost:8080/hls/", playbackID, "/index.m3u8?stream=", playbackID, "&jwt=", token, "\n5"))

	result := executeFlow(token, payload, TriggerHandler(gateURL), allowAccess)
	require.Equal(t, "false", result)
}

func TestExpiredToken(t *testing.T) {
	token, _ := craftToken(privateKey, publicKey, playbackID, time.Now().Add(time.Second*-10))
	payload := []byte(fmt.Sprint(playbackID, "\n1\n2\n3\nhttp://localhost:8080/hls/", playbackID, "/index.m3u8?stream=", playbackID, "&jwt=", token, "\n5"))

	result := executeFlow(token, payload, TriggerHandler(gateURL), allowAccess)
	require.Equal(t, "false", result)
}

func TestCacheHit(t *testing.T) {
	token, _ := craftToken(privateKey, publicKey, playbackID, expiration)
	payload := []byte(fmt.Sprint(playbackID, "\n1\n2\n3\nhttp://localhost:8080/hls/", playbackID, "/index.m3u8?stream=", playbackID, "&jwt=", token, "\n5"))
	handler := TriggerHandler(gateURL)

	var callCount = 0
	var countableAllowAccess = func(ac *PlaybackAccessControl, body []byte) (bool, int32, int32, error) {
		callCount++
		return true, 10, 20, nil
	}

	executeFlow(token, payload, handler, countableAllowAccess)
	require.Equal(t, 1, callCount)

	executeFlow(token, payload, handler, countableAllowAccess)
	require.Equal(t, 1, callCount)
}

func TestStaleCache(t *testing.T) {
	token, _ := craftToken(privateKey, publicKey, playbackID, expiration)
	payload := []byte(fmt.Sprint(playbackID, "\n1\n2\n3\nhttp://localhost:8080/hls/", playbackID, "/index.m3u8?stream=", playbackID, "&jwt=", token, "\n5"))
	handler := TriggerHandler(gateURL)

	var callCount = 0
	var countableAllowAccess = func(ac *PlaybackAccessControl, body []byte) (bool, int32, int32, error) {
		callCount++
		return true, -10, 20, nil
	}

	// Cache entry is absent and a first remote call is done
	executeFlow(token, payload, handler, countableAllowAccess)
	// Flow is executed a second time, cache is used but a remote call is scheduled
	executeFlow(token, payload, handler, countableAllowAccess)
	// Remote call count is still 1
	require.Equal(t, 1, callCount)

	// Assign testable function again since executeFlow() restores it
	original := queryGate
	queryGate = countableAllowAccess

	// After the scheduled call is executed and call count is incremented
	time.Sleep(1 * time.Second)
	require.Equal(t, 2, callCount)

	queryGate = original

	executeFlow(token, payload, handler, countableAllowAccess)
	require.Equal(t, 2, callCount)

}

func TestInvalidCache(t *testing.T) {
	token, _ := craftToken(privateKey, publicKey, playbackID, expiration)
	payload := []byte(fmt.Sprint(playbackID, "\n1\n2\n3\nhttp://localhost:8080/hls/", playbackID, "/index.m3u8?stream=", playbackID, "&jwt=", token, "\n5"))
	handler := TriggerHandler(gateURL)

	var callCount = 0
	var countableAllowAccess = func(ac *PlaybackAccessControl, body []byte) (bool, int32, int32, error) {
		callCount++
		return true, -10, -20, nil
	}

	executeFlow(token, payload, handler, countableAllowAccess)
	executeFlow(token, payload, handler, countableAllowAccess)

	require.Equal(t, 2, callCount)
}

func executeFlow(token string, payload []byte, handler http.Handler, request func(ac *PlaybackAccessControl, body []byte) (bool, int32, int32, error)) string {
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
