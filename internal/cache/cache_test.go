package cache

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDownloadPathsAndMeta(t *testing.T) {
	root := t.TempDir()
	empty, err := ListDownloadMeta(filepath.Join(root, "missing"))
	if err != nil {
		t.Fatalf("ListDownloadMeta empty: %v", err)
	}
	if empty == nil || len(empty) != 0 {
		t.Fatalf("expected empty slice, got %#v", empty)
	}

	rawURL := "http://example.test/pool/linux-image-dbgsym_1_amd64.ddeb"
	path := DownloadFilePath(root, rawURL)
	if filepath.Dir(path) != DownloadsDir(root) {
		t.Fatalf("unexpected download dir: %s", path)
	}

	meta := NewDownloadMeta(rawURL, path, "abc", 123, false)
	if err := WriteDownloadMeta(root, meta); err != nil {
		t.Fatalf("WriteDownloadMeta: %v", err)
	}
	got, err := ReadDownloadMeta(root, rawURL)
	if err != nil {
		t.Fatalf("ReadDownloadMeta: %v", err)
	}
	if got.URL != rawURL || got.SHA256 != "abc" || got.Size != 123 {
		t.Fatalf("unexpected meta: %#v", got)
	}

	list, err := ListDownloadMeta(root)
	if err != nil {
		t.Fatalf("ListDownloadMeta: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 meta, got %d", len(list))
	}

	if err := os.MkdirAll(filepath.Join(root, "downloads"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := EnsureLayout(root); err != nil {
		t.Fatalf("EnsureLayout: %v", err)
	}
	if err := Clear(root, false); err != nil {
		t.Fatalf("Clear: %v", err)
	}
	if _, err := os.Stat(DownloadsDir(root)); err != nil {
		t.Fatalf("expected downloads dir after clear: %v", err)
	}
	if _, err := os.Stat(SentinelPath(root)); err != nil {
		t.Fatalf("expected sentinel after clear: %v", err)
	}
}

func TestClearRefusesCustomDirWithoutSentinel(t *testing.T) {
	root := t.TempDir()
	if err := Clear(root, false); err == nil {
		t.Fatal("expected custom dir without sentinel to be rejected")
	}
}

func TestClearAllowsCustomDirWithForce(t *testing.T) {
	root := t.TempDir()
	if err := Clear(root, true); err != nil {
		t.Fatalf("Clear force: %v", err)
	}
	if _, err := os.Stat(SentinelPath(root)); err != nil {
		t.Fatalf("expected sentinel after force clear: %v", err)
	}
}

func TestClearRefusesCurrentWorkingDirectoryEvenWithForce(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := Clear(cwd, true); err == nil {
		t.Fatal("expected cwd to be rejected")
	}
}

func TestClearRefusesCurrentWorkingDirectoryAncestorEvenWithForce(t *testing.T) {
	parent := t.TempDir()
	child := filepath.Join(parent, "workspace")
	if err := os.MkdirAll(child, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(child)

	if err := validateClearRoot(parent, true); err == nil {
		t.Fatal("expected cwd ancestor to be rejected")
	}
}
