package banner

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	reDebianKernelPkgVer = regexp.MustCompile(`#.*\bDebian\s+((?:[0-9]+:)?[0-9][0-9A-Za-z:.+~]*-[0-9][0-9A-Za-z:.+~]*)(?:\s|\))`)
	reDebianPkgVer       = regexp.MustCompile(`\bDebian\s+((?:[0-9]+:)?[0-9][0-9A-Za-z:.+~]*-[0-9][0-9A-Za-z:.+~]*)(?:\s|\))`)
)

func ParseDebianBanner(banner string) (*KernelInfo, error) {
	info := &KernelInfo{
		Distro:        "debian",
		Arch:          detectArch(banner),
		SourcePackage: "linux",
		Banner:        banner,
	}

	match := reAnyKernelRelease.FindStringSubmatch(banner)
	if match == nil {
		return nil, fmt.Errorf("无法提取 kernel release")
	}
	info.KernelRelease = strings.TrimSpace(match[1])

	match = reDebianKernelPkgVer.FindStringSubmatch(banner)
	if match != nil {
		info.PackageVersion = strings.TrimSpace(match[1])
	} else {
		matches := reDebianPkgVer.FindAllStringSubmatch(banner, -1)
		if len(matches) > 0 && len(matches[len(matches)-1]) > 1 {
			info.PackageVersion = strings.TrimSpace(matches[len(matches)-1][1])
		}
	}
	if info.PackageVersion == "" {
		return info, fmt.Errorf("无法提取 Debian package version")
	}
	return info, nil
}
