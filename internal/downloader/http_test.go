package downloader

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestDownloadAndCacheHit(t *testing.T) {
	var hits int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		_, _ = w.Write([]byte("ddeb"))
	}))
	defer server.Close()

	dest := filepath.Join(t.TempDir(), "pkg.ddeb")
	var progressCalls int
	first, err := Download(context.Background(), server.Client(), server.URL+"/pkg.ddeb", dest, false, func(progress Progress) {
		progressCalls++
		if progress.Downloaded < 0 {
			t.Fatalf("negative progress: %#v", progress)
		}
	})
	if err != nil {
		t.Fatalf("Download first: %v", err)
	}
	if first.CacheHit {
		t.Fatal("first download should not be cache hit")
	}
	if progressCalls == 0 {
		t.Fatal("expected progress callback")
	}

	second, err := Download(context.Background(), server.Client(), server.URL+"/pkg.ddeb", dest, false, nil)
	if err != nil {
		t.Fatalf("Download second: %v", err)
	}
	if !second.CacheHit {
		t.Fatal("second download should be cache hit")
	}
	if hits != 1 {
		t.Fatalf("expected 1 server hit, got %d", hits)
	}
	data, err := os.ReadFile(dest)
	if err != nil || string(data) != "ddeb" {
		t.Fatalf("unexpected file: %q %v", data, err)
	}
}
