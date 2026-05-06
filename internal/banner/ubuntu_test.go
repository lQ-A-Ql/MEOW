package banner

import (
	"testing"
)

const ubuntuBanner = "Linux version 5.4.0-163-generic (buildd@lcy02-amd64-067) (gcc version 9.4.0 (Ubuntu 9.4.0-1ubuntu1~20.04.2)) #180-Ubuntu SMP Tue Sep 5 13:21:23 UTC 2023 (Ubuntu 5.4.0-163.180-generic 5.4.246)"

func TestParseUbuntuBanner_Success(t *testing.T) {
	info, err := ParseUbuntuBanner(ubuntuBanner)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if info.Distro != "ubuntu" {
		t.Errorf("distro: got %q, want %q", info.Distro, "ubuntu")
	}
	if info.Codename != "focal" {
		t.Errorf("codename: got %q, want %q", info.Codename, "focal")
	}
	if info.KernelRelease != "5.4.0-163-generic" {
		t.Errorf("kernel: got %q, want %q", info.KernelRelease, "5.4.0-163-generic")
	}
	if info.PackageVersion != "5.4.0-163.180" {
		t.Errorf("pkgver: got %q, want %q", info.PackageVersion, "5.4.0-163.180")
	}
	if info.Arch != "amd64" {
		t.Errorf("arch: got %q, want %q", info.Arch, "amd64")
	}
	if info.SourcePackage != "linux" {
		t.Errorf("source_package: got %q, want %q", info.SourcePackage, "linux")
	}
}

func TestParseUbuntuBanner_CodenameFallback(t *testing.T) {
	banners := map[string]string{
		"18.04": "bionic",
		"20.04": "focal",
		"22.04": "jammy",
		"24.04": "noble",
	}
	for ver, code := range banners {
		b := "Linux version 5.4.0-1-generic (gcc version 1.0 (Ubuntu 1.0-1ubuntu1~" + ver + ".1)) ... (Ubuntu 5.4.0-1.1-generic 5.4)"
		info, err := ParseUbuntuBanner(b)
		if err != nil {
			t.Fatalf("unexpected error for %v: %v", code, err)
		}
		if info.Codename != code {
			t.Errorf("codename for %v: got %q, want %q", code, info.Codename, code)
		}
	}
}

func TestParseUbuntuBanner_NoPackageVersion(t *testing.T) {
	b := "Linux version 5.4.0-163-generic (some stuff) no Ubuntu pkgver here"
	info, err := ParseUbuntuBanner(b)
	if err == nil {
		t.Fatal("expected error for missing package version")
	}
	if info == nil || info.KernelRelease != "5.4.0-163-generic" {
		t.Error("expected kernel release to be extracted even on pkgver error")
	}
}

func TestParseUbuntuBanner_IgnoresGCCUbuntuVersion(t *testing.T) {
	b := "Linux version 5.4.0-163-generic (buildd@test) (gcc version 9.4.0 (Ubuntu 9.4.0-1ubuntu1~20.04.2)) #180-Ubuntu SMP"
	info, err := ParseUbuntuBanner(b)
	if err == nil {
		t.Fatal("expected error for missing kernel package version")
	}
	if info == nil || info.PackageVersion != "" {
		t.Fatalf("expected no package version, got %#v", info)
	}
}

func TestParseUbuntuBanner_PackageVersionWithSuffix(t *testing.T) {
	b := "Linux version 5.15.0-1053-azure (buildd@test) (gcc version 9.4.0 (Ubuntu 9.4.0-1ubuntu1~20.04.2)) #59 SMP (Ubuntu 5.15.0-1053.59~20.04.1-azure 5.15.132)"
	info, err := ParseUbuntuBanner(b)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.PackageVersion != "5.15.0-1053.59~20.04.1" {
		t.Errorf("pkgver: got %q", info.PackageVersion)
	}
}

func TestParseUbuntuBanner_NoKernelRelease(t *testing.T) {
	_, err := ParseUbuntuBanner("no kernel info")
	if err == nil {
		t.Fatal("expected error for missing kernel release")
	}
}
