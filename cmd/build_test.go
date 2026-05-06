package cmd

import (
	"strings"
	"testing"

	"meow/internal/downloader"
	"meow/internal/resolver"
)

func TestFormatDownloadProgressUsesStablePixelCat(t *testing.T) {
	line := formatDownloadProgress(downloader.Progress{Downloaded: 50, Total: 100})
	if !strings.Contains(line, "=^..^=__/") {
		t.Fatalf("expected stable pixel cat in progress bar: %q", line)
	}
	if strings.Contains(line, "🐱") {
		t.Fatalf("emoji should not be used in progress bar: %q", line)
	}
	if !strings.Contains(line, "50.0%") {
		t.Fatalf("expected percent in progress bar: %q", line)
	}
}

func TestFormatStageProgressUsesRunningPixelCat(t *testing.T) {
	line := formatStageProgress("compress", 1)
	if !strings.Contains(line, "压缩 ISF") {
		t.Fatalf("expected stage label in progress bar: %q", line)
	}
	if !strings.Contains(line, "=^..^=__\\") {
		t.Fatalf("expected running stable pixel cat frame in progress bar: %q", line)
	}
	if strings.Contains(line, "🐱") {
		t.Fatalf("emoji should not be used in progress bar: %q", line)
	}
	if !strings.Contains(line, "构建符号") {
		t.Fatalf("expected whole-build progress label: %q", line)
	}
	if strings.Contains(line, "100.0%") {
		t.Fatalf("stage progress should not claim exact completion before done: %q", line)
	}
}

func TestFormatBuildProgressShowsExtractSubProgress(t *testing.T) {
	line := formatBuildProgress("extract", 1, extractProgressEvent{
		current: 7,
		total:   10,
		file:    "./usr/lib/debug/boot/vmlinux-5.4.0-163-generic",
	})
	if !strings.Contains(line, "构建符号") {
		t.Fatalf("expected overall progress: %q", line)
	}
	if !strings.Contains(line, "解包文件") {
		t.Fatalf("expected extract sub progress: %q", line)
	}
	if !strings.Contains(line, "7/10") {
		t.Fatalf("expected file count progress: %q", line)
	}
	if !strings.Contains(line, "\n") {
		t.Fatalf("expected two progress lines: %q", line)
	}
}

func TestFormatBuildProgressHidesExtractSubProgressOutsideExtract(t *testing.T) {
	line := formatBuildProgress("compress", 1, extractProgressEvent{
		current: 7,
		total:   10,
		file:    "ignored",
	})
	if strings.Contains(line, "解包文件") {
		t.Fatalf("did not expect extract sub progress outside extract stage: %q", line)
	}
}

func TestFormatProbeProgressShowsCandidateIndex(t *testing.T) {
	line := formatProbeProgress(resolver.ProbeEvent{
		Index: 2,
		Total: 3,
		URL:   "http://example.test/linux-image-dbgsym_1_amd64.ddeb",
	})
	if !strings.Contains(line, "探测包") {
		t.Fatalf("expected probe label: %q", line)
	}
	if !strings.Contains(line, "2/3") {
		t.Fatalf("expected candidate index: %q", line)
	}
	if !strings.Contains(line, "linux-image-dbgsym_1_amd64.ddeb") {
		t.Fatalf("expected package basename: %q", line)
	}
}
