package banner

type KernelInfo struct {
	Distro         string `json:"distro"`
	Codename       string `json:"codename"`
	KernelRelease  string `json:"kernel"`
	PackageVersion string `json:"package_version"`
	Arch           string `json:"arch"`
	SourcePackage  string `json:"source_package"`
	Banner         string `json:"banner,omitempty"`
}

type ResolveResult struct {
	KernelInfo    KernelInfo `json:"kernel_info"`
	Candidates    []string   `json:"candidates"`
	FoundURL      string     `json:"found_url,omitempty"`
	PackageName   string     `json:"package_name,omitempty"`
	RepoBase      string     `json:"repo_base"`
	PackageFormat string     `json:"package_format,omitempty"`
	SupportLevel  string     `json:"support_level,omitempty"`
	ManualReason  string     `json:"manual_reason,omitempty"`
}

type BuildResult struct {
	SymbolPath      string  `json:"symbol_path"`
	VmlinuxPath     string  `json:"vmlinux_path,omitempty"`
	PackagePath     string  `json:"package_path,omitempty"`
	DurationSeconds float64 `json:"duration_seconds"`
	Success         bool    `json:"success"`
}
