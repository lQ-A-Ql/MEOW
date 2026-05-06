package symbols

import (
	"testing"

	"meow/internal/banner"
)

func TestFileName(t *testing.T) {
	got := FileName(banner.KernelInfo{
		Distro:         "ubuntu",
		KernelRelease:  "5.4.0-163-generic",
		PackageVersion: "5.4.0-163.180",
		Arch:           "amd64",
	})
	want := "Ubuntu_5.4.0-163-generic_5.4.0-163.180_amd64.json.xz"
	if got != want {
		t.Fatalf("FileName: got %q want %q", got, want)
	}
}

func TestInferFromDDEB(t *testing.T) {
	info, ok := InferFromDDEB(`C:\tmp\linux-image-unsigned-5.4.0-163-generic-dbgsym_5.4.0-163.180_amd64.ddeb`)
	if !ok {
		t.Fatal("expected inference success")
	}
	if info.KernelRelease != "5.4.0-163-generic" || info.PackageVersion != "5.4.0-163.180" || info.Arch != "amd64" {
		t.Fatalf("unexpected info: %#v", info)
	}
}

func TestInferFromDDEB_Debian(t *testing.T) {
	info, ok := InferFromDDEB(`C:\tmp\linux-image-5.10.0-35-amd64-dbg_5.10.237-1_amd64.deb`)
	if !ok {
		t.Fatal("expected inference success")
	}
	if info.Distro != "debian" || info.KernelRelease != "5.10.0-35-amd64" || info.PackageVersion != "5.10.237-1" || info.Arch != "amd64" {
		t.Fatalf("unexpected info: %#v", info)
	}
}

func TestInferFromDDEB_RPM(t *testing.T) {
	info, ok := InferFromDDEB(`C:\tmp\kernel-debuginfo-4.18.0-513.5.1.el8_9.x86_64.x86_64.rpm`)
	if !ok {
		t.Fatal("expected inference success")
	}
	if info.Distro != "rhel" || info.KernelRelease != "4.18.0-513.5.1.el8_9.x86_64" || info.PackageVersion != info.KernelRelease || info.Arch != "amd64" {
		t.Fatalf("unexpected info: %#v", info)
	}
}

func TestPackageFormat(t *testing.T) {
	cases := map[string]string{
		"foo.ddeb":        "ddeb",
		"foo.deb":         "deb",
		"foo.rpm":         "rpm",
		"foo.json.xz":     "isf",
		"vmlinux-5.10.0":  "vmlinux",
		"unknown-package": "unknown",
	}
	for input, want := range cases {
		if got := PackageFormat(input); got != want {
			t.Fatalf("PackageFormat(%q): got %q want %q", input, got, want)
		}
	}
}
