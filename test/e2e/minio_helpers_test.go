package e2e

import (
	"context"
	"fmt"
	"net/url"
	"testing"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

func minioClientForURL(rawURL, username, password string) (*minio.Client, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}
	if u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return nil, fmt.Errorf("invalid MinIO URL %q", rawURL)
	}
	return minio.New(u.Host, &minio.Options{
		Creds:        credentials.NewStaticV4(username, password, ""),
		Secure:       u.Scheme == "https",
		Region:       region,
		BucketLookup: minio.BucketLookupPath,
	})
}

func waitForMinio(
	ctx context.Context,
	t *testing.T,
	rawURL, username, password, description string,
	ready func(context.Context, *minio.Client) error,
) {
	t.Helper()

	client, err := minioClientForURL(rawURL, username, password)
	if err != nil {
		t.Fatalf("could not create MinIO client for %s: %s", description, redactQuickTunnelURLs(err.Error()))
	}

	deadline := time.Now().Add(time.Minute)
	for {
		requestCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		err = ready(requestCtx, client)
		cancel()
		if err == nil {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("%s did not become ready: %s", description, redactQuickTunnelURLs(err.Error()))
		}
		time.Sleep(time.Second)
	}
}
