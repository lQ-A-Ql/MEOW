package symbols

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"meow/internal/banner"
)

var (
	ubuntuDebugNameRe = regexp.MustCompile(`^linux-(?:image-unsigned|image|modules)-(.+)-dbgsym_([^_]+)_([^_]+)\.ddeb$`)
	debianDebugNameRe = regexp.MustCompile(`^linux-image-(.+)-(?:dbg|dbgsym)_([^_]+)_([^_]+)\.deb$`)
	rpmDebugNameRe    = regexp.MustCompile(`^(?:kernel|kernel-core|kernel-debug|kernel-uek)-debuginfo-(.+)\.([^.]+)\.rpm$`)
)

func FileName(info banner.KernelInfo) string {
	distro := titleOrDefault(info.Distro, "Ubuntu")
	kernel := safePart(defaultString(info.KernelRelease, "unknown-kernel"))
	pkgver := safePart(defaultString(info.PackageVersion, "manual"))
	arch := safePart(defaultString(info.Arch, "amd64"))
	return fmt.Sprintf("%s_%s_%s_%s.json.xz", distro, kernel, pkgver, arch)
}

func InferFromDDEB(filePath string) (*banner.KernelInfo, bool) {
	name := filepath.Base(filePath)
	if match := ubuntuDebugNameRe.FindStringSubmatch(name); len(match) == 4 {
		return &banner.KernelInfo{
			Distro:         "ubuntu",
			KernelRelease:  match[1],
			PackageVersion: match[2],
			Arch:           match[3],
			SourcePackage:  "linux",
		}, true
	}
	if match := debianDebugNameRe.FindStringSubmatch(name); len(match) == 4 {
		return &banner.KernelInfo{
			Distro:         "debian",
			KernelRelease:  match[1],
			PackageVersion: match[2],
			Arch:           match[3],
			SourcePackage:  "linux",
		}, true
	}
	if match := rpmDebugNameRe.FindStringSubmatch(name); len(match) == 3 {
		kernel := match[1]
		return &banner.KernelInfo{
			Distro:         rpmDistroFromKernel(kernel),
			KernelRelease:  kernel,
			PackageVersion: kernel,
			Arch:           debArch(match[2]),
			SourcePackage:  "kernel",
		}, true
	}
	return nil, false
}

func PackageFormat(filePathOrURL string) string {
	lower := strings.ToLower(filePathOrURL)
	switch {
	case strings.HasSuffix(lower, ".ddeb"):
		return "ddeb"
	case strings.HasSuffix(lower, ".deb"):
		return "deb"
	case strings.HasSuffix(lower, ".rpm"):
		return "rpm"
	case strings.HasSuffix(lower, ".json.xz"):
		return "isf"
	case strings.Contains(filepath.Base(lower), "vmlinux"):
		return "vmlinux"
	default:
		return "unknown"
	}
}

func InferFromVMLINUX(filePath string) *banner.KernelInfo {
	name := filepath.Base(filePath)
	kernel := strings.TrimPrefix(name, "vmlinux-")
	if kernel == name || kernel == "" {
		kernel = "manual"
	}
	return &banner.KernelInfo{
		Distro:         "ubuntu",
		KernelRelease:  kernel,
		PackageVersion: "manual",
		Arch:           "amd64",
		SourcePackage:  "linux",
	}
}

func MergeManual(base *banner.KernelInfo, distro, kernel, pkgver, arch string) banner.KernelInfo {
	info := banner.KernelInfo{}
	if base != nil {
		info = *base
	}
	if distro != "" {
		info.Distro = strings.ToLower(distro)
	}
	if kernel != "" {
		info.KernelRelease = kernel
	}
	if pkgver != "" {
		info.PackageVersion = pkgver
	}
	if arch != "" {
		info.Arch = arch
	}
	if info.Distro == "" {
		info.Distro = "ubuntu"
	}
	if info.Arch == "" {
		info.Arch = "amd64"
	}
	if info.SourcePackage == "" {
		info.SourcePackage = "linux"
	}
	return info
}

func safePart(value string) string {
	replacer := strings.NewReplacer("\\", "_", "/", "_", ":", "_", "*", "_", "?", "_", "\"", "_", "<", "_", ">", "_", "|", "_", " ", "_")
	return replacer.Replace(value)
}

func titleOrDefault(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return strings.ToUpper(value[:1]) + strings.ToLower(value[1:])
}

func defaultString(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func debArch(arch string) string {
	switch arch {
	case "x86_64":
		return "amd64"
	case "aarch64":
		return "arm64"
	default:
		return arch
	}
}

func rpmDistroFromKernel(kernel string) string {
	lower := strings.ToLower(kernel)
	switch {
	case strings.Contains(lower, ".el"):
		return "rhel"
	case strings.Contains(lower, ".fc"):
		return "fedora"
	default:
		return "rhel"
	}
}
