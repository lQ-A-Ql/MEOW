package resolver

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path"
	"strings"
	"time"

	"meow/internal/banner"
)

const (
	ubuntuDdebBase = "http://ddebs.ubuntu.com/pool/main/l/linux"
	debianDebBase  = "https://deb.debian.org/debian/pool/main/l/linux"
)

var ErrPackageNotFound = errors.New("未找到对应 debug symbol 包")

const (
	SupportAutoDownload  = "auto_download"
	SupportManualPackage = "manual_package"
	SupportVMLINUXOnly   = "vmlinux_only"

	FormatDDEB    = "ddeb"
	FormatDEB     = "deb"
	FormatRPM     = "rpm"
	FormatVMLINUX = "vmlinux"
	FormatISF     = "isf"
	FormatUnknown = "unknown"
)

type Resolver interface {
	Match(info *banner.KernelInfo) bool
	Candidates(info *banner.KernelInfo) *banner.ResolveResult
	SupportLevel() string
}

type ProbeEvent struct {
	Index int
	Total int
	URL   string
}

type ProbeFunc func(ProbeEvent)

func GenerateCandidates(info *banner.KernelInfo) *banner.ResolveResult {
	for _, r := range registry {
		if r.Match(info) {
			return r.Candidates(info)
		}
	}
	return genericManualResolver{}.Candidates(info)
}

func GenerateUbuntuCandidatesWithBase(info *banner.KernelInfo, base string) *banner.ResolveResult {
	result := &banner.ResolveResult{
		KernelInfo:    *info,
		RepoBase:      base,
		PackageFormat: FormatDDEB,
		SupportLevel:  SupportAutoDownload,
	}

	kernel := info.KernelRelease
	pkgver := info.PackageVersion
	arch := info.Arch

	candidates := []struct {
		name string
	}{
		{fmt.Sprintf("linux-image-unsigned-%s-dbgsym_%s_%s.ddeb", kernel, pkgver, arch)},
		{fmt.Sprintf("linux-image-%s-dbgsym_%s_%s.ddeb", kernel, pkgver, arch)},
		{fmt.Sprintf("linux-modules-%s-dbgsym_%s_%s.ddeb", kernel, pkgver, arch)},
	}

	for _, c := range candidates {
		result.Candidates = append(result.Candidates,
			fmt.Sprintf("%s/%s", base, c.name))
	}

	if len(result.Candidates) > 0 {
		result.PackageName = candidates[0].name
	}

	return result
}

func GenerateDebianCandidatesWithBase(info *banner.KernelInfo, base string) *banner.ResolveResult {
	result := &banner.ResolveResult{
		KernelInfo:    *info,
		RepoBase:      base,
		PackageFormat: FormatDEB,
		SupportLevel:  SupportAutoDownload,
	}

	kernel := info.KernelRelease
	pkgver := info.PackageVersion
	arch := info.Arch
	candidates := []string{
		fmt.Sprintf("linux-image-%s-dbg_%s_%s.deb", kernel, pkgver, arch),
		fmt.Sprintf("linux-image-%s-dbgsym_%s_%s.deb", kernel, pkgver, arch),
	}
	for _, name := range candidates {
		result.Candidates = append(result.Candidates, fmt.Sprintf("%s/%s", base, name))
	}
	if len(candidates) > 0 {
		result.PackageName = candidates[0]
	}
	return result
}

func GenerateRPMCandidiateNames(info *banner.KernelInfo) []string {
	kernel := strings.TrimSuffix(info.KernelRelease, "."+rpmArch(info.Arch))
	arch := rpmArch(info.Arch)
	names := []string{
		fmt.Sprintf("kernel-debuginfo-%s.%s.rpm", kernel, arch),
		fmt.Sprintf("kernel-core-debuginfo-%s.%s.rpm", kernel, arch),
		fmt.Sprintf("kernel-debug-debuginfo-%s.%s.rpm", kernel, arch),
	}
	if info.Distro == "oracle" || strings.Contains(strings.ToLower(kernel), "uek") {
		names = append(names, fmt.Sprintf("kernel-uek-debuginfo-%s.%s.rpm", kernel, arch))
	}
	return names
}

func GenerateRPMCandidatesWithBase(info *banner.KernelInfo, base string) *banner.ResolveResult {
	result := &banner.ResolveResult{
		KernelInfo:    *info,
		RepoBase:      base,
		PackageFormat: FormatRPM,
		SupportLevel:  SupportManualPackage,
		ManualReason:  "RPM 系发行版默认不绕过订阅或企业仓库；请提供 --debug-package、--debug-package-url 或 --repo-url",
	}
	for _, name := range GenerateRPMCandidiateNames(info) {
		if base != "" {
			result.Candidates = append(result.Candidates, strings.TrimRight(base, "/")+"/"+name)
		}
	}
	if len(result.Candidates) > 0 {
		result.PackageName = path.Base(result.Candidates[0])
	}
	return result
}

type ubuntuResolver struct{}
type debianResolver struct{}
type rpmManualResolver struct{}
type genericManualResolver struct{}

var registry = []Resolver{
	ubuntuResolver{},
	debianResolver{},
	rpmManualResolver{},
	genericManualResolver{},
}

func (ubuntuResolver) Match(info *banner.KernelInfo) bool { return info.Distro == "ubuntu" }
func (ubuntuResolver) Candidates(info *banner.KernelInfo) *banner.ResolveResult {
	return GenerateUbuntuCandidatesWithBase(info, ubuntuDdebBase)
}
func (ubuntuResolver) SupportLevel() string { return SupportAutoDownload }

func (debianResolver) Match(info *banner.KernelInfo) bool { return info.Distro == "debian" }
func (debianResolver) Candidates(info *banner.KernelInfo) *banner.ResolveResult {
	return GenerateDebianCandidatesWithBase(info, debianDebBase)
}
func (debianResolver) SupportLevel() string { return SupportAutoDownload }

func (rpmManualResolver) Match(info *banner.KernelInfo) bool {
	switch info.Distro {
	case "rhel", "centos", "rocky", "alma", "fedora", "amazon", "oracle", "suse", "opensuse":
		return true
	default:
		return false
	}
}
func (rpmManualResolver) Candidates(info *banner.KernelInfo) *banner.ResolveResult {
	return GenerateRPMCandidatesWithBase(info, "")
}
func (rpmManualResolver) SupportLevel() string { return SupportManualPackage }

func (genericManualResolver) Match(info *banner.KernelInfo) bool { return true }
func (genericManualResolver) Candidates(info *banner.KernelInfo) *banner.ResolveResult {
	return &banner.ResolveResult{
		KernelInfo:    *info,
		PackageFormat: FormatUnknown,
		SupportLevel:  SupportVMLINUXOnly,
		ManualReason:  "未知或定制发行版；请使用 --vmlinux 或手工 debug package",
	}
}
func (genericManualResolver) SupportLevel() string { return SupportVMLINUXOnly }

func ResolveUbuntuDDEB(ctx context.Context, info *banner.KernelInfo, client *http.Client) (*banner.ResolveResult, error) {
	return ResolvePackageWithBase(ctx, info, ubuntuDdebBase, client)
}

func ResolveUbuntuDDEBProgress(ctx context.Context, info *banner.KernelInfo, client *http.Client, progress ProbeFunc) (*banner.ResolveResult, error) {
	return ResolvePackageWithBaseProgress(ctx, info, ubuntuDdebBase, client, progress)
}

func ResolveUbuntuDDEBWithBase(ctx context.Context, info *banner.KernelInfo, base string, client *http.Client) (*banner.ResolveResult, error) {
	return ResolvePackageWithBase(ctx, info, base, client)
}

func ResolveUbuntuDDEBWithBaseProgress(ctx context.Context, info *banner.KernelInfo, base string, client *http.Client, progress ProbeFunc) (*banner.ResolveResult, error) {
	return ResolvePackageWithBaseProgress(ctx, info, base, client, progress)
}

func ResolvePackageProgress(ctx context.Context, info *banner.KernelInfo, client *http.Client, progress ProbeFunc) (*banner.ResolveResult, error) {
	switch info.Distro {
	case "debian":
		return ResolvePackageWithBaseProgress(ctx, info, debianDebBase, client, progress)
	case "ubuntu":
		return ResolvePackageWithBaseProgress(ctx, info, ubuntuDdebBase, client, progress)
	default:
		result := GenerateCandidates(info)
		return result, fmt.Errorf("%w: %s 暂不支持自动定位 debug package", ErrPackageNotFound, info.Distro)
	}
}

func ResolveRpmRepo(ctx context.Context, info *banner.KernelInfo, repoURL string, client *http.Client) (*banner.ResolveResult, error) {
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	result := GenerateRPMCandidatesWithBase(info, repoURL)
	primaryURL, err := fetchPrimaryMetadataURL(ctx, client, repoURL)
	if err != nil {
		return result, err
	}
	packages, err := fetchPrimaryPackages(ctx, client, primaryURL)
	if err != nil {
		return result, err
	}
	wanted := map[string]bool{}
	for _, name := range GenerateRPMCandidiateNames(info) {
		wanted[name] = true
	}
	for _, pkg := range packages {
		if wanted[pkg.FileName] {
			result.FoundURL = joinURL(repoURL, pkg.Location.Href)
			result.PackageName = pkg.FileName
			result.Candidates = []string{result.FoundURL}
			result.SupportLevel = SupportAutoDownload
			result.ManualReason = ""
			return result, nil
		}
	}
	return result, ErrPackageNotFound
}

func ResolvePackageWithBase(ctx context.Context, info *banner.KernelInfo, base string, client *http.Client) (*banner.ResolveResult, error) {
	return ResolvePackageWithBaseProgress(ctx, info, base, client, nil)
}

func ResolvePackageWithBaseProgress(ctx context.Context, info *banner.KernelInfo, base string, client *http.Client, progress ProbeFunc) (*banner.ResolveResult, error) {
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}

	result := generateCandidatesWithBase(info, base)
	var firstProbeErr error
	for i, candidate := range result.Candidates {
		if progress != nil {
			progress(ProbeEvent{Index: i + 1, Total: len(result.Candidates), URL: candidate})
		}
		found, err := probeDDEB(ctx, client, candidate)
		if err != nil {
			if firstProbeErr == nil {
				firstProbeErr = err
			}
			continue
		}
		if found {
			result.FoundURL = candidate
			result.PackageName = path.Base(candidate)
			return result, nil
		}
	}

	if firstProbeErr != nil {
		return result, errors.Join(ErrPackageNotFound, firstProbeErr)
	}
	return result, ErrPackageNotFound
}

func generateCandidatesWithBase(info *banner.KernelInfo, base string) *banner.ResolveResult {
	if info.Distro == "debian" {
		return GenerateDebianCandidatesWithBase(info, base)
	}
	return GenerateUbuntuCandidatesWithBase(info, base)
}

type repoMD struct {
	Data []struct {
		Type     string `xml:"type,attr"`
		Location struct {
			Href string `xml:"href,attr"`
		} `xml:"location"`
	} `xml:"data"`
}

type primaryMetadata struct {
	Packages []rpmPackage `xml:"package"`
}

type rpmPackage struct {
	Name     string `xml:"name"`
	Arch     string `xml:"arch"`
	Location struct {
		Href string `xml:"href,attr"`
	} `xml:"location"`
	FileName string
}

func fetchPrimaryMetadataURL(ctx context.Context, client *http.Client, repoURL string) (string, error) {
	raw, err := fetchBytes(ctx, client, strings.TrimRight(repoURL, "/")+"/repodata/repomd.xml")
	if err != nil {
		return "", err
	}
	var meta repoMD
	if err := xml.Unmarshal(raw, &meta); err != nil {
		return "", err
	}
	for _, data := range meta.Data {
		if data.Type == "primary" || data.Type == "primary_db" {
			return joinURL(repoURL, data.Location.Href), nil
		}
	}
	return "", fmt.Errorf("primary metadata not found in repomd.xml")
}

func fetchPrimaryPackages(ctx context.Context, client *http.Client, primaryURL string) ([]rpmPackage, error) {
	raw, err := fetchBytes(ctx, client, primaryURL)
	if err != nil {
		return nil, err
	}
	if strings.HasSuffix(primaryURL, ".gz") {
		reader, err := gzip.NewReader(bytes.NewReader(raw))
		if err != nil {
			return nil, err
		}
		defer reader.Close()
		raw, err = io.ReadAll(reader)
		if err != nil {
			return nil, err
		}
	}
	var meta primaryMetadata
	if err := xml.Unmarshal(raw, &meta); err != nil {
		return nil, err
	}
	for i := range meta.Packages {
		meta.Packages[i].FileName = path.Base(meta.Packages[i].Location.Href)
	}
	return meta.Packages, nil
}

func fetchBytes(ctx context.Context, client *http.Client, rawURL string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if !isSuccessStatus(resp.StatusCode) {
		return nil, fmt.Errorf("metadata fetch failed: %s", resp.Status)
	}
	return io.ReadAll(resp.Body)
}

func joinURL(base, href string) string {
	if strings.HasPrefix(href, "http://") || strings.HasPrefix(href, "https://") {
		return href
	}
	return strings.TrimRight(base, "/") + "/" + strings.TrimLeft(href, "/")
}

func rpmArch(arch string) string {
	switch arch {
	case "amd64":
		return "x86_64"
	case "arm64":
		return "aarch64"
	default:
		return arch
	}
}

func probeDDEB(ctx context.Context, client *http.Client, candidate string) (bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, candidate, nil)
	if err != nil {
		return false, err
	}

	resp, err := client.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	if isSuccessStatus(resp.StatusCode) {
		return true, nil
	}
	if shouldFallbackToRange(resp.StatusCode) {
		return probeDDEBRange(ctx, client, candidate)
	}

	return false, nil
}

func probeDDEBRange(ctx context.Context, client *http.Client, candidate string) (bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, candidate, nil)
	if err != nil {
		return false, err
	}
	req.Header.Set("Range", "bytes=0-0")

	resp, err := client.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	return isSuccessStatus(resp.StatusCode), nil
}

func isSuccessStatus(statusCode int) bool {
	return statusCode >= http.StatusOK && statusCode < http.StatusMultipleChoices
}

func shouldFallbackToRange(statusCode int) bool {
	switch statusCode {
	case http.StatusForbidden, http.StatusMethodNotAllowed, http.StatusNotImplemented:
		return true
	default:
		return false
	}
}
