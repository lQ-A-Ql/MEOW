package banner

import "testing"

const debianBanner = "Linux version 5.10.0-35-amd64 (debian-kernel@lists.debian.org) (gcc-10 (Debian 10.2.1-6) 10.2.1 20210110, GNU ld (GNU Binutils for Debian) 2.35.2) #1 SMP Debian 5.10.237-1 (2025-05-19)"

func TestParseBanner_Debian(t *testing.T) {
	info, err := ParseBanner(debianBanner)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.Distro != "debian" {
		t.Fatalf("distro: got %q", info.Distro)
	}
	if info.KernelRelease != "5.10.0-35-amd64" {
		t.Fatalf("kernel: got %q", info.KernelRelease)
	}
	if info.PackageVersion != "5.10.237-1" {
		t.Fatalf("pkgver: got %q", info.PackageVersion)
	}
}

func TestParseBanner_CentOSFamilyPartial(t *testing.T) {
	b := "Linux version 4.18.0-513.5.1.el8_9.x86_64 (mockbuild@x86-01.mbox.centos.org) #1 SMP"
	info, err := ParseBanner(b)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.Distro != "centos" {
		t.Fatalf("distro: got %q", info.Distro)
	}
	if info.KernelRelease != "4.18.0-513.5.1.el8_9.x86_64" {
		t.Fatalf("kernel: got %q", info.KernelRelease)
	}
	if info.Arch != "amd64" {
		t.Fatalf("arch: got %q", info.Arch)
	}
}

func TestParseBanner_RockyAlmaFedora(t *testing.T) {
	cases := []struct {
		banner string
		distro string
	}{
		{"Linux version 5.14.0-362.8.1.el9_3.x86_64 (mockbuild@iad1-prod-build002.bld.rockylinux.org) #1 SMP", "rocky"},
		{"Linux version 5.14.0-362.8.1.el9_3.x86_64 (mockbuild@x86-64-01.almalinux.org) #1 SMP", "alma"},
		{"Linux version 6.8.9-300.fc40.x86_64 (mockbuild@bkernel01) #1 SMP", "fedora"},
	}
	for _, tc := range cases {
		info, err := ParseBanner(tc.banner)
		if err != nil {
			t.Fatalf("%s: %v", tc.distro, err)
		}
		if info.Distro != tc.distro {
			t.Fatalf("distro: got %q want %q", info.Distro, tc.distro)
		}
	}
}
