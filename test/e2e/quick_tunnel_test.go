package e2e

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
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

var quickTunnelURLPattern = regexp.MustCompile(`(?i)(?:s3\+)?https://(?:[^/@\s]+@)?[a-z0-9-]+\.trycloudflare\.com`)

func redactQuickTunnelURLs(value string) string {
	return quickTunnelURLPattern.ReplaceAllString(value, "[redacted quick tunnel]")
}

// TODO: Remove quick tunnels after catalyst-api supports optional VOD callbacks
// and an empty-by-default, exact host:port allowlist for trusted private object
// stores at both validation and dial time. The in-a-box configuration can then
// use its local MinIO and Task Runner endpoints without exposing them publicly.
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
	output := &lockedBuffer{}
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
			t.Fatalf("cloudflared exited before creating a tunnel: %v\n%s", err, redactQuickTunnelURLs(output.String()))
		case <-ticker.C:
			if publicURL := quickTunnelURLPattern.FindString(output.String()); publicURL != "" {
				t.Cleanup(stop)
				return publicURL
			}
		case <-timer.C:
			stop()
			t.Fatalf("timed out creating a quick tunnel for %s\n%s", origin, redactQuickTunnelURLs(output.String()))
		}
	}
}

func TestRedactQuickTunnelURLs(t *testing.T) {
	input := "API: https://example-name.trycloudflare.com/task-runner; storage: s3+https://access:secret@store.trycloudflare.com/bucket"
	redacted := redactQuickTunnelURLs(input)
	require.NotContains(t, redacted, "example-name.trycloudflare.com")
	require.NotContains(t, redacted, "store.trycloudflare.com")
	require.NotContains(t, redacted, "access:secret")
	require.Equal(t, "API: [redacted quick tunnel]/task-runner; storage: [redacted quick tunnel]/bucket", redacted)
}

type callbackStatus struct {
	RequestID string `json:"request_id"`
	Status    string `json:"status"`
	Error     string `json:"error,omitempty"`
}

type callbackTunnel struct {
	apiServerURL string
	callbackURL  string
	terminal     chan callbackStatus
	mu           sync.Mutex
	last         callbackStatus
}

func startCallbackTunnel(t *testing.T) *callbackTunnel {
	t.Helper()

	callbacks := &callbackTunnel{terminal: make(chan callbackStatus, 1)}
	callbackServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/task-runner") {
			http.NotFound(w, r)
			return
		}
		if r.Method == http.MethodPost {
			body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			var status callbackStatus
			if err := json.Unmarshal(body, &status); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			callbacks.mu.Lock()
			callbacks.last = status
			callbacks.mu.Unlock()
			if status.Status == "success" || status.Status == "error" {
				select {
				case callbacks.terminal <- status:
				default:
				}
			}
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(callbackServer.Close)

	callbacks.apiServerURL = startQuickTunnel(t, callbackServer.URL)
	callbacks.callbackURL = callbacks.apiServerURL + "/task-runner/vod-test"
	waitForPublicHTTP(t, callbacks.apiServerURL+"/task-runner/ready")
	return callbacks
}

func (c *callbackTunnel) waitForCompletion(requestID string, timeout time.Duration) error {
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	for {
		select {
		case status := <-c.terminal:
			if status.RequestID != requestID {
				continue
			}
			if status.Status == "error" {
				return fmt.Errorf("VOD job %s failed: %s", requestID, status.Error)
			}
			return nil
		case <-timer.C:
			c.mu.Lock()
			last := c.last
			c.mu.Unlock()
			return fmt.Errorf("timed out waiting for VOD job %s; last callback: %+v", requestID, last)
		}
	}
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
			t.Fatalf("tunnel did not become ready: %s", redactQuickTunnelURLs(err.Error()))
		}
		time.Sleep(time.Second)
	}
}

func objectStoreURL(baseURL, username, password, bucket, key string) (string, error) {
	u, err := url.Parse(baseURL)
	if err != nil {
		return "", err
	}
	switch u.Scheme {
	case "http":
		u.Scheme = "s3+http"
	case "https":
		u.Scheme = "s3+https"
	default:
		return "", fmt.Errorf("invalid object-store base URL %q", baseURL)
	}
	if u.Hostname() == "" {
		return "", fmt.Errorf("invalid object-store base URL %q", baseURL)
	}
	u.User = url.UserPassword(username, password)
	u.Path = "/" + strings.Trim(strings.Join([]string{bucket, key}, "/"), "/")
	return u.String(), nil
}

func TestObjectStoreURL(t *testing.T) {
	httpsURL, err := objectStoreURL("https://example.trycloudflare.com", "access", "secret", "bucket", "path/source.mp4")
	require.NoError(t, err)
	require.Equal(t, "s3+https://access:secret@example.trycloudflare.com/bucket/path/source.mp4", httpsURL)

	httpURL, err := objectStoreURL("http://minio:9000", "access", "secret", "bucket", "")
	require.NoError(t, err)
	require.Equal(t, "s3+http://access:secret@minio:9000/bucket", httpURL)

	_, err = objectStoreURL("ftp://example.com", "access", "secret", "bucket", "")
	require.Error(t, err)
}

func TestWaitForCompletion(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		callbacks := &callbackTunnel{terminal: make(chan callbackStatus, 1)}
		callbacks.terminal <- callbackStatus{RequestID: "request-1", Status: "success"}
		require.NoError(t, callbacks.waitForCompletion("request-1", time.Second))
	})

	t.Run("error", func(t *testing.T) {
		callbacks := &callbackTunnel{terminal: make(chan callbackStatus, 1)}
		callbacks.terminal <- callbackStatus{RequestID: "request-1", Status: "error", Error: "transcode failed"}
		require.EqualError(t, callbacks.waitForCompletion("request-1", time.Second), "VOD job request-1 failed: transcode failed")
	})
}
