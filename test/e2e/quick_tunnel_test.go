package e2e

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

var quickTunnelURLPattern = regexp.MustCompile(`https://[a-z0-9-]+\.trycloudflare\.com`)

type synchronizedBuffer struct {
	mu sync.Mutex
	bytes.Buffer
}

func (b *synchronizedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.Buffer.Write(p)
}

func (b *synchronizedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.Buffer.String()
}

// startQuickTunnel exposes origin through a temporary trycloudflare.com URL.
// The cloudflared process is stopped automatically when the test completes.
func startQuickTunnel(t *testing.T, origin string) string {
	t.Helper()

	originURL, err := url.Parse(origin)
	require.NoError(t, err)
	require.Equal(t, "http", originURL.Scheme)
	require.NotEmpty(t, originURL.Host)

	cloudflared, err := exec.LookPath("cloudflared")
	require.NoError(t, err, "cloudflared is required to run the E2E tests")

	ctx, cancel := context.WithCancel(context.Background())
	output := &synchronizedBuffer{}
	cmd := exec.CommandContext(
		ctx,
		cloudflared,
		"tunnel",
		"--no-autoupdate",
		"--protocol", "http2",
		"--url", origin,
	)
	cmd.Stdout = output
	cmd.Stderr = output
	require.NoError(t, cmd.Start())

	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	stop := func() {
		cancel()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			_ = cmd.Process.Kill()
			<-done
		}
	}

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	timer := time.NewTimer(time.Minute)
	defer timer.Stop()

	for {
		select {
		case err := <-done:
			cancel()
			t.Fatalf("cloudflared exited before creating a tunnel: %v\n%s", err, output.String())
		case <-ticker.C:
			if publicURL := quickTunnelURLPattern.FindString(output.String()); publicURL != "" {
				t.Cleanup(stop)
				t.Logf("quick tunnel %s -> %s", publicURL, origin)
				return publicURL
			}
		case <-timer.C:
			stop()
			t.Fatalf("timed out creating a quick tunnel for %s\n%s", origin, output.String())
		}
	}
}

func startCallbackTunnel(t *testing.T) (apiServerURL, callbackURL string) {
	t.Helper()

	callbackServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/task-runner") {
			http.NotFound(w, r)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(callbackServer.Close)

	apiServerURL = startQuickTunnel(t, callbackServer.URL)
	waitForPublicHTTP(t, apiServerURL+"/task-runner/ready")
	return apiServerURL, apiServerURL + "/task-runner/vod-test"
}

func waitForPublicHTTP(t *testing.T, endpoint string) {
	t.Helper()

	client := &http.Client{Timeout: 5 * time.Second}
	deadline := time.Now().Add(time.Minute)
	for {
		resp, err := client.Get(endpoint)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusMultipleChoices {
				return
			}
			err = fmt.Errorf("unexpected status %s", resp.Status)
		}
		if time.Now().After(deadline) {
			t.Fatalf("tunnel did not become ready: %v", err)
		}
		time.Sleep(time.Second)
	}
}

func tunneledObjectStoreURL(t *testing.T, publicURL, username, password, bucket, key string) string {
	t.Helper()

	u, err := url.Parse(publicURL)
	require.NoError(t, err)
	require.Equal(t, "https", u.Scheme)
	require.NotEmpty(t, u.Hostname())

	u.Scheme = "s3+https"
	u.User = url.UserPassword(username, password)
	u.Path = "/" + strings.Trim(strings.Join([]string{bucket, key}, "/"), "/")
	return u.String()
}

func TestTunneledObjectStoreURL(t *testing.T) {
	require.Equal(
		t,
		"s3+https://access:secret@example.trycloudflare.com/bucket/path/source.mp4",
		tunneledObjectStoreURL(t, "https://example.trycloudflare.com", "access", "secret", "bucket", "path/source.mp4"),
	)
	require.Equal(
		t,
		"s3+https://access:secret@example.trycloudflare.com/bucket",
		tunneledObjectStoreURL(t, "https://example.trycloudflare.com", "access", "secret", "bucket", ""),
	)
}
