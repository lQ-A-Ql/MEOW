package tui

import (
	"path/filepath"
	"strings"
)

type InputMode string

const (
	InputModeMem             InputMode = "mem"
	InputModeBannerFile      InputMode = "banner-file"
	InputModeDebugPackage    InputMode = "debug-package"
	InputModeDebugPackageURL InputMode = "debug-package-url"
	InputModeRepoURL         InputMode = "repo-url"
	InputModeVMLINUX         InputMode = "vmlinux"
	InputModeManual          InputMode = "manual"
)

type Options struct {
	MeowPath          string
	VolPath           string
	MemPath           string
	SymbolsPath       string
	OutDir            string
	CacheDir          string
	SymbolSourcesPath string
	BannerFile        string
	DebugPackage      string
	DebugPackageURL   string
	RepoURL           string
	VMLinuxPath       string
	Distro            string
	Kernel            string
	PackageVersion    string
	Arch              string
	Plugin            string
	PluginArgs        []string
	NoRemoteSymbols   bool
	Force             bool
	Runner            Runner
}

func (o Options) WithDefaults() Options {
	if o.MeowPath == "" {
		o.MeowPath = "meow"
	}
	if o.VolPath == "" {
		o.VolPath = "vol"
	}
	if o.SymbolsPath == "" {
		o.SymbolsPath = "./symbols"
	}
	if o.OutDir == "" {
		o.OutDir = filepath.Join(".", "symbols", "linux")
	}
	if o.Arch == "" {
		o.Arch = "amd64"
	}
	if o.Plugin == "" {
		o.Plugin = "linux.pslist.PsList"
	}
	if o.Runner == nil {
		o.Runner = ExecRunner{}
	}
	return o
}

func (o Options) InputMode() InputMode {
	switch {
	case strings.TrimSpace(o.DebugPackage) != "":
		return InputModeDebugPackage
	case strings.TrimSpace(o.DebugPackageURL) != "":
		return InputModeDebugPackageURL
	case strings.TrimSpace(o.RepoURL) != "":
		return InputModeRepoURL
	case strings.TrimSpace(o.VMLinuxPath) != "":
		return InputModeVMLINUX
	case strings.TrimSpace(o.BannerFile) != "":
		return InputModeBannerFile
	case strings.TrimSpace(o.Kernel) != "" || strings.TrimSpace(o.PackageVersion) != "":
		return InputModeManual
	case strings.TrimSpace(o.MemPath) != "":
		return InputModeMem
	default:
		return InputModeManual
	}
}

type DoctorCheck struct {
	Name    string `json:"name"`
	OK      bool   `json:"ok"`
	Detail  string `json:"detail,omitempty"`
	Warning bool   `json:"warning,omitempty"`
}

type BuildSummary struct {
	Success        bool     `json:"success"`
	DryRun         bool     `json:"dry_run"`
	Distro         string   `json:"distro"`
	Kernel         string   `json:"kernel"`
	PackageVersion string   `json:"package_version"`
	Arch           string   `json:"arch"`
	OutputDir      string   `json:"output_dir"`
	SymbolPath     string   `json:"symbol_path"`
	PackagePath    string   `json:"package_path,omitempty"`
	FoundURL       string   `json:"found_url,omitempty"`
	Candidates     []string `json:"candidates,omitempty"`
	CacheHit       bool     `json:"cache_hit,omitempty"`
	Duration       string   `json:"duration,omitempty"`
	Error          string   `json:"error,omitempty"`
}

type VerifySummary struct {
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
	Output  string `json:"output,omitempty"`
}

type CacheEntry struct {
	URL          string `json:"url"`
	Filename     string `json:"filename"`
	Path         string `json:"path"`
	Size         int64  `json:"size"`
	SHA256       string `json:"sha256"`
	DownloadedAt string `json:"downloaded_at"`
	CacheHit     bool   `json:"cache_hit,omitempty"`
}
