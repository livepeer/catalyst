package main

import (
	"fmt"
	"github.com/stretchr/testify/require"
	"net/http"
	"net/http/httptest"
	"testing"
)

const (
	closestNodeAddr = "someurl.com"
	playbackID      = "abc_XYZ-123"
)

func TestRedirectHandler_Correct(t *testing.T) {
	defaultFunc := getClosestNode
	getClosestNode = func(string) (string, error) { return closestNodeAddr, nil }
	defer func() { getClosestNode = defaultFunc }()

	requireReq(t, fmt.Sprintf("/hls/%s/index.m3u8", playbackID)).
		result().
		hasStatus(http.StatusFound).
		hasHeader("Location", fmt.Sprintf("http://%s/hls/%s/index.m3u8", closestNodeAddr, playbackID))

	requireReq(t, fmt.Sprintf("/hls/%s/index.m3u8", playbackID)).
		withHeader("X-Forwarded-Proto", "https").
		result().
		hasStatus(http.StatusFound).
		hasHeader("Location", fmt.Sprintf("https://%s/hls/%s/index.m3u8", closestNodeAddr, playbackID))
}

func TestRedirectHandler_InvalidPath(t *testing.T) {
	requireReq(t, "/hls").result().hasStatus(http.StatusNotFound)
	requireReq(t, "/hls").result().hasStatus(http.StatusNotFound)
	requireReq(t, "/hls/").result().hasStatus(http.StatusNotFound)
	requireReq(t, "/hls/12345").result().hasStatus(http.StatusNotFound)
	requireReq(t, "/hls/12345/somepath").result().hasStatus(http.StatusNotFound)
	requireReq(t, "/hls/12345/somepath/index.m3u8").result().hasStatus(http.StatusNotFound)
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

func (hr httpReq) result() httpCheck {
	rr := httptest.NewRecorder()
	redirectHlsHandler().ServeHTTP(rr, hr.Request)
	return httpCheck{hr.T, rr}
}

func (hc httpCheck) hasStatus(code int) httpCheck {
	require.Equal(hc, code, hc.Code)
	return hc
}

func (hc httpCheck) hasHeader(key, value string) httpCheck {
	require.Equal(hc, value, hc.Header().Get(key))
	return hc
}
