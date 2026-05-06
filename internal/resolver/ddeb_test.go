package resolver

import (
	"compress/gzip"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"path"
	"strings"
	"testing"

	"meow/internal/banner"
)

func TestGenerateCandidates(t *testing.T) {
	info := &banner.KernelInfo{
		Distro:         "ubuntu",
		KernelRelease:  "5.4.0-163-generic",
		PackageVersion: "5.4.0-163.180",
		Arch:           "amd64",
		SourcePackage:  "linux",
	}

	result := GenerateCandidates(info)

	if result.RepoBase != "http://ddebs.ubuntu.com/pool/main/l/linux" {
		t.Errorf("RepoBase: got %q", result.RepoBase)
	}

	if len(result.Candidates) != 3 {
		t.Fatalf("expected 3 candidates, got %d", len(result.Candidates))
	}

	expected := []string{
		"linux-image-unsigned-5.4.0-163-generic-dbgsym_5.4.0-163.180_amd64.ddeb",
		"linux-image-5.4.0-163-generic-dbgsym_5.4.0-163.180_amd64.ddeb",
		"linux-modules-5.4.0-163-generic-dbgsym_5.4.0-163.180_amd64.ddeb",
	}

	for i, want := range expected {
		got := result.Candidates[i]
		if !strings.Contains(got, want) {
			t.Errorf("candidate[%d]: want to contain %q, got %q", i, want, got)
		}
		if !strings.HasPrefix(got, "http://ddebs.ubuntu.com/pool/main/l/linux/") {
			t.Errorf("candidate[%d] missing prefix: %q", i, got)
		}
	}

	if result.PackageName != expected[0] {
		t.Errorf("PackageName: got %q, want %q", result.PackageName, expected[0])
	}
}

func TestGenerateRPMCandidiateNames_TrimsKernelArchSuffix(t *testing.T) {
	info := &banner.KernelInfo{
		Distro:        "rocky",
		KernelRelease: "4.18.0-513.5.1.el8_9.x86_64",
		Arch:          "amd64",
	}
	names := GenerateRPMCandidiateNames(info)
	want := "kernel-debuginfo-4.18.0-513.5.1.el8_9.x86_64.rpm"
	if names[0] != want {
		t.Fatalf("name: got %q want %q", names[0], want)
	}
}

func TestResolveRpmRepo_PrimaryGzipFound(t *testing.T) {
	info := &banner.KernelInfo{
		Distro:        "rocky",
		KernelRelease: "4.18.0-513.5.1.el8_9.x86_64",
		Arch:          "amd64",
	}
	primary := `<?xml version="1.0" encoding="UTF-8"?>
<metadata xmlns="http://linux.duke.edu/metadata/common">
  <package type="rpm">
    <name>kernel-debuginfo</name>
    <arch>x86_64</arch>
    <location href="Packages/k/kernel-debuginfo-4.18.0-513.5.1.el8_9.x86_64.rpm"/>
  </package>
</metadata>`
	var gz strings.Builder
	writer := gzip.NewWriter(&gz)
	_, _ = writer.Write([]byte(primary))
	_ = writer.Close()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repodata/repomd.xml":
			_, _ = w.Write([]byte(`<repomd><data type="primary"><location href="repodata/primary.xml.gz"/></data></repomd>`))
		case "/repodata/primary.xml.gz":
			_, _ = w.Write([]byte(gz.String()))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	result, err := ResolveRpmRepo(context.Background(), info, server.URL, server.Client())
	if err != nil {
		t.Fatalf("ResolveRpmRepo: %v", err)
	}
	want := server.URL + "/Packages/k/kernel-debuginfo-4.18.0-513.5.1.el8_9.x86_64.rpm"
	if result.FoundURL != want {
		t.Fatalf("FoundURL: got %q want %q", result.FoundURL, want)
	}
	if result.SupportLevel != SupportAutoDownload {
		t.Fatalf("SupportLevel: %q", result.SupportLevel)
	}
}

func TestGenerateCandidates_Debian(t *testing.T) {
	info := &banner.KernelInfo{
		Distro:         "debian",
		KernelRelease:  "5.10.0-35-amd64",
		PackageVersion: "5.10.237-1",
		Arch:           "amd64",
		SourcePackage:  "linux",
	}

	result := GenerateCandidates(info)
	if result.RepoBase != "https://deb.debian.org/debian/pool/main/l/linux" {
		t.Fatalf("RepoBase: got %q", result.RepoBase)
	}
	if len(result.Candidates) != 2 {
		t.Fatalf("candidates: got %d", len(result.Candidates))
	}
	want := "linux-image-5.10.0-35-amd64-dbg_5.10.237-1_amd64.deb"
	if !strings.Contains(result.Candidates[0], want) {
		t.Fatalf("candidate[0]: got %q want contains %q", result.Candidates[0], want)
	}
}

func TestGenerateCandidates_RHELManualRPM(t *testing.T) {
	info := &banner.KernelInfo{
		Distro:         "rhel",
		KernelRelease:  "4.18.0-513.5.1.el8_9.x86_64",
		PackageVersion: "4.18.0-513.5.1.el8_9.x86_64",
		Arch:           "amd64",
		SourcePackage:  "linux",
	}

	result := GenerateCandidates(info)
	if result.RepoBase != "" || len(result.Candidates) != 0 || result.PackageName != "" {
		t.Fatalf("rhel without repo should not get fake URL candidates: %#v", result)
	}
	if result.PackageFormat != FormatRPM || result.SupportLevel != SupportManualPackage || result.ManualReason == "" {
		t.Fatalf("rhel should report manual RPM support: %#v", result)
	}
}

func TestResolveUbuntuDDEB_HEADFound(t *testing.T) {
	info := testKernelInfo()
	expectedPackage := "linux-image-5.4.0-163-generic-dbgsym_5.4.0-163.180_amd64.ddeb"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodHead {
			t.Fatalf("expected HEAD, got %s", r.Method)
		}
		if path.Base(r.URL.Path) == expectedPackage {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	result, err := ResolveUbuntuDDEBWithBase(context.Background(), info, server.URL, server.Client())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.FoundURL != server.URL+"/"+expectedPackage {
		t.Fatalf("FoundURL: got %q", result.FoundURL)
	}
	if result.PackageName != expectedPackage {
		t.Fatalf("PackageName: got %q", result.PackageName)
	}
}

func TestResolveUbuntuDDEB_FallbackRange(t *testing.T) {
	info := testKernelInfo()
	expectedPackage := "linux-image-unsigned-5.4.0-163-generic-dbgsym_5.4.0-163.180_amd64.ddeb"
	sawRange := false

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodHead {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if r.Method != http.MethodGet {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		if r.Header.Get("Range") != "bytes=0-0" {
			t.Fatalf("missing range header: %q", r.Header.Get("Range"))
		}
		if path.Base(r.URL.Path) == expectedPackage {
			sawRange = true
			w.WriteHeader(http.StatusPartialContent)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	result, err := ResolveUbuntuDDEBWithBase(context.Background(), info, server.URL, server.Client())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !sawRange {
		t.Fatal("expected range fallback request")
	}
	if result.FoundURL != server.URL+"/"+expectedPackage {
		t.Fatalf("FoundURL: got %q", result.FoundURL)
	}
}

func TestResolveUbuntuDDEB_NotFound(t *testing.T) {
	info := testKernelInfo()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	result, err := ResolveUbuntuDDEBWithBase(context.Background(), info, server.URL, server.Client())
	if !errors.Is(err, ErrPackageNotFound) {
		t.Fatalf("expected ErrPackageNotFound, got %v", err)
	}
	if result.FoundURL != "" {
		t.Fatalf("FoundURL should be empty, got %q", result.FoundURL)
	}
}

func TestResolveUbuntuDDEB_NetworkError(t *testing.T) {
	info := testKernelInfo()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	base := server.URL
	server.Close()

	result, err := ResolveUbuntuDDEBWithBase(context.Background(), info, base, server.Client())
	if !errors.Is(err, ErrPackageNotFound) {
		t.Fatalf("expected ErrPackageNotFound, got %v", err)
	}
	if result.FoundURL != "" {
		t.Fatalf("FoundURL should be empty, got %q", result.FoundURL)
	}
}

func TestResolveUbuntuDDEB_ProgressCallback(t *testing.T) {
	info := testKernelInfo()
	var events []ProbeEvent

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	_, _ = ResolveUbuntuDDEBWithBaseProgress(context.Background(), info, server.URL, server.Client(), func(event ProbeEvent) {
		events = append(events, event)
	})

	if len(events) != 3 {
		t.Fatalf("events: got %d want 3", len(events))
	}
	if events[0].Index != 1 || events[0].Total != 3 {
		t.Fatalf("first event: %#v", events[0])
	}
	if !strings.Contains(events[0].URL, "linux-image-unsigned") {
		t.Fatalf("first event URL: %q", events[0].URL)
	}
}

func testKernelInfo() *banner.KernelInfo {
	return &banner.KernelInfo{
		Distro:         "ubuntu",
		KernelRelease:  "5.4.0-163-generic",
		PackageVersion: "5.4.0-163.180",
		Arch:           "amd64",
		SourcePackage:  "linux",
	}
}
