package symbolsources

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoad_DefaultWhenMissing(t *testing.T) {
	sources, err := Load(filepath.Join(t.TempDir(), "missing.txt"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(sources) != 1 || sources[0].Name != DefaultName {
		t.Fatalf("sources: %#v", sources)
	}
}

func TestFindRejectsOversizedIndex(t *testing.T) {
	const banner = "Linux version 5.4.0-test"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(strings.Repeat("x", MaxIndexBytes+1)))
	}))
	defer server.Close()

	sources := []Source{{
		Name:       "large",
		IndexURL:   server.URL,
		RawBaseURL: server.URL + "/raw/",
	}}
	match, warnings, err := Find(context.Background(), server.Client(), sources, banner)
	if err != nil {
		t.Fatalf("Find should return warnings, not fatal error: %v", err)
	}
	if match != nil {
		t.Fatalf("expected no match, got %#v", match)
	}
	if len(warnings) != 1 || !strings.Contains(warnings[0], "too large") {
		t.Fatalf("expected too large warning, got %#v", warnings)
	}
}

func TestReadLimitedRejectsOversizedContent(t *testing.T) {
	_, err := readLimited(strings.NewReader("abcdef"), 3, "fixture", "test://fixture")
	if err == nil {
		t.Fatal("expected limit error")
	}
	if !strings.Contains(err.Error(), "fixture too large") {
		t.Fatalf("unexpected error: %v", err)
	}
	if errors.Is(err, context.Canceled) {
		t.Fatalf("unexpected wrapped context error: %v", err)
	}
}

func TestLoad_ParsesCommentsAndBlankLines(t *testing.T) {
	file := filepath.Join(t.TempDir(), "sources.txt")
	content := "\n# name|index_url|raw_base_url\nlocal|https://example.test/index.json|https://example.test/raw/\n"
	if err := os.WriteFile(file, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	sources, err := Load(file)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(sources) != 1 || sources[0].Name != "local" || sources[0].IndexURL != "https://example.test/index.json" {
		t.Fatalf("sources: %#v", sources)
	}
}

func TestLoad_InvalidLineReportsLineNumber(t *testing.T) {
	file := filepath.Join(t.TempDir(), "sources.txt")
	if err := os.WriteFile(file, []byte("# ok\ninvalid\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := Load(file)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), file+":2:") {
		t.Fatalf("error lacks line number: %v", err)
	}
}

func TestJoinRawURL(t *testing.T) {
	got := JoinRawURL("https://example.test/base/", "/linux/symbol.json.xz")
	want := "https://example.test/base/linux/symbol.json.xz"
	if got != want {
		t.Fatalf("JoinRawURL: got %q want %q", got, want)
	}
}

func TestFindExactBannerMatch(t *testing.T) {
	const banner = "Linux version 5.4.0-test"
	index := map[string][]string{banner: {"linux/Ubuntu_test.json.xz"}}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/banners.json" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(index)
	}))
	defer server.Close()

	sources := []Source{{
		Name:       "fixture",
		IndexURL:   server.URL + "/banners.json",
		RawBaseURL: server.URL + "/raw/",
	}}
	match, warnings, err := Find(context.Background(), server.Client(), sources, banner)
	if err != nil {
		t.Fatalf("Find: %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("warnings: %#v", warnings)
	}
	if match == nil || match.Source.Name != "fixture" || match.URL != server.URL+"/raw/linux/Ubuntu_test.json.xz" {
		t.Fatalf("match: %#v", match)
	}
}

func TestParseBannerIndexAcceptsStringValues(t *testing.T) {
	index, err := parseBannerIndex([]byte(`{"Linux version test":"linux/test.json.xz"}`))
	if err != nil {
		t.Fatalf("parseBannerIndex: %v", err)
	}
	if index["Linux version test"] != "linux/test.json.xz" {
		t.Fatalf("index: %#v", index)
	}
}

func TestParseBannerIndexAcceptsArrayValues(t *testing.T) {
	index, err := parseBannerIndex([]byte(`{"Linux version test":["linux/test.json.xz","linux/other.json.xz"]}`))
	if err != nil {
		t.Fatalf("parseBannerIndex: %v", err)
	}
	if index["Linux version test"] != "linux/test.json.xz" {
		t.Fatalf("index: %#v", index)
	}
}
