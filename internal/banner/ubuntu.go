package banner

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	reKernelRelease = regexp.MustCompile(`Linux version\s+(\S+)`)
	rePkgVer        = regexp.MustCompile(`Ubuntu\s+((?:[0-9]+:)?[0-9][0-9A-Za-z:.+~]*-[0-9][0-9A-Za-z:.+~]*)(?:-[A-Za-z0-9]+)?(?:\s|\))`)
	reGCC           = regexp.MustCompile(`~([0-9]+\.[0-9]+)`)
)

var codenameMap = map[string]string{
	"18.04": "bionic",
	"20.04": "focal",
	"22.04": "jammy",
	"24.04": "noble",
}

func ParseUbuntuBanner(banner string) (*KernelInfo, error) {
	info := &KernelInfo{
		Distro:        "ubuntu",
		Arch:          "amd64",
		SourcePackage: "linux",
		Banner:        banner,
	}

	match := reKernelRelease.FindStringSubmatch(banner)
	if match == nil {
		return nil, fmt.Errorf("无法提取 kernel release")
	}
	info.KernelRelease = strings.TrimSpace(match[1])

	if strings.Contains(banner, "x86_64") {
		info.Arch = "amd64"
	}

	info.PackageVersion = extractUbuntuPackageVersion(banner, info.KernelRelease)

	match = reGCC.FindStringSubmatch(banner)
	if match != nil {
		if codename, ok := codenameMap[match[1]]; ok {
			info.Codename = codename
		}
	}

	if info.PackageVersion == "" {
		return info, fmt.Errorf("无法提取 Ubuntu package version")
	}

	return info, nil
}

func extractUbuntuPackageVersion(banner, kernelRelease string) string {
	kernelBase := kernelPackageBase(kernelRelease)
	for _, match := range rePkgVer.FindAllStringSubmatch(banner, -1) {
		if len(match) < 2 {
			continue
		}
		pkgver := strings.TrimSpace(match[1])
		if strings.HasPrefix(pkgver, kernelBase) {
			return pkgver
		}
	}
	return ""
}

func kernelPackageBase(kernelRelease string) string {
	parts := strings.Split(kernelRelease, "-")
	if len(parts) < 3 {
		return kernelRelease
	}
	return strings.Join(parts[:len(parts)-1], "-")
}
