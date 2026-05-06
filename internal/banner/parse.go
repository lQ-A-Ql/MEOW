package banner

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

var reAnyKernelRelease = regexp.MustCompile(`Linux version\s+(\S+)`)

func ParseBanner(text string) (*KernelInfo, error) {
	banner := strings.TrimSpace(text)
	if banner == "" {
		return nil, fmt.Errorf("banner 为空")
	}
	if strings.Contains(banner, "Ubuntu") {
		if info, err := ParseUbuntuBanner(banner); err == nil || info != nil {
			return info, err
		}
	}
	if strings.Contains(banner, "Debian") || strings.Contains(banner, "debian-kernel@") {
		return ParseDebianBanner(banner)
	}
	if info, ok := parseKnownServerBanner(banner); ok {
		return info, nil
	}
	if info := partialKernelInfo(banner); info != nil {
		return info, errors.New("无法识别发行版；可用 --distro/--kernel/--pkgver 手工指定，或使用 --vmlinux")
	}
	return nil, fmt.Errorf("无法提取 kernel release")
}

func parseKnownServerBanner(banner string) (*KernelInfo, bool) {
	lower := strings.ToLower(banner)
	distro := ""
	switch {
	case strings.Contains(lower, "rocky"):
		distro = "rocky"
	case strings.Contains(lower, "alma"):
		distro = "alma"
	case strings.Contains(lower, "centos"):
		distro = "centos"
	case regexp.MustCompile(`\bfc[0-9]+`).MatchString(lower), strings.Contains(lower, "fedora"):
		distro = "fedora"
	case strings.Contains(lower, "el9"), strings.Contains(lower, "el8"), strings.Contains(lower, "el7"):
		distro = "rhel"
	case strings.Contains(lower, "amzn"):
		distro = "amazon"
	case strings.Contains(lower, "suse"), strings.Contains(lower, "opensuse"):
		distro = "suse"
	default:
		return nil, false
	}

	info := partialKernelInfo(banner)
	if info == nil {
		return nil, false
	}
	info.Distro = distro
	info.PackageVersion = info.KernelRelease
	return info, true
}

func partialKernelInfo(banner string) *KernelInfo {
	match := reAnyKernelRelease.FindStringSubmatch(banner)
	if match == nil {
		return nil
	}
	return &KernelInfo{
		Distro:        "unknown",
		KernelRelease: strings.TrimSpace(match[1]),
		Arch:          detectArch(banner),
		SourcePackage: "linux",
		Banner:        banner,
	}
}

func detectArch(banner string) string {
	lower := strings.ToLower(banner)
	switch {
	case strings.Contains(lower, "x86_64"), strings.Contains(lower, "amd64"):
		return "amd64"
	case strings.Contains(lower, "aarch64"), strings.Contains(lower, "arm64"):
		return "arm64"
	default:
		return "amd64"
	}
}
