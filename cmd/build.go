package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	backendpkg "meow/internal/backend"
	bannerpkg "meow/internal/banner"
	cachepkg "meow/internal/cache"
	"meow/internal/downloader"
	"meow/internal/log"
	"meow/internal/resolver"
	"meow/internal/symbols"
	sourcespkg "meow/internal/symbolsources"
	"meow/internal/volatility"
)

var (
	buildBanner          string
	buildBannerFile      string
	buildMem             string
	buildKernel          string
	buildPkgver          string
	buildDistro          string
	buildArch            string
	buildBackend         string
	buildWslDistro       string
	buildOut             string
	buildCacheDir        string
	buildDDEB            string
	buildDDEBURL         string
	buildDebugPackage    string
	buildDebugPackageURL string
	buildRepoURL         string
	buildSymbolSources   string
	buildNoRemoteSymbols bool
	buildVMLINUX         string
	buildVolPath         string
	buildForce           bool
	buildDryRun          bool
	buildJSON            bool
	buildProbeTimeout    time.Duration
	buildDownloadTimeout time.Duration
)

type buildSummary struct {
	Success        bool     `json:"success"`
	DryRun         bool     `json:"dry_run"`
	Distro         string   `json:"distro"`
	Codename       string   `json:"codename,omitempty"`
	Kernel         string   `json:"kernel"`
	PackageVersion string   `json:"package_version"`
	Arch           string   `json:"arch"`
	Backend        string   `json:"backend"`
	OutputDir      string   `json:"output_dir"`
	SymbolPath     string   `json:"symbol_path"`
	VmlinuxPath    string   `json:"vmlinux_path,omitempty"`
	PackagePath    string   `json:"package_path,omitempty"`
	FoundURL       string   `json:"found_url,omitempty"`
	Candidates     []string `json:"candidates,omitempty"`
	CacheHit       bool     `json:"cache_hit,omitempty"`
	Duration       string   `json:"duration,omitempty"`
	SymbolSource   string   `json:"symbol_source,omitempty"`
	PackageFormat  string   `json:"package_format,omitempty"`
	SupportLevel   string   `json:"support_level,omitempty"`
	ManualReason   string   `json:"manual_reason,omitempty"`
	SymbolSources  []string `json:"symbol_sources,omitempty"`
}

func init() {
	fs := Register("build", "生成/下载 Volatility 3 Linux ISF 符号表", runBuild)
	fs.StringVar(&buildBanner, "banner", "", "直接传入 banner 字符串")
	fs.StringVar(&buildBannerFile, "banner-file", "", "从文件读取 banner")
	fs.StringVar(&buildMem, "mem", "", "内存镜像路径")
	fs.StringVar(&buildKernel, "kernel", "", "kernel release，例如 5.4.0-163-generic")
	fs.StringVar(&buildPkgver, "pkgver", "", "包版本，例如 5.4.0-163.180")
	fs.StringVar(&buildDistro, "distro", "", "发行版；默认从 banner/文件名推断，手工模式默认 ubuntu")
	fs.StringVar(&buildArch, "arch", "amd64", "架构")
	fs.StringVar(&buildBackend, "backend", "wsl", "后端: wsl/native")
	fs.StringVar(&buildWslDistro, "wsl-distro", "", "WSL 发行版名称，空则使用默认")
	fs.StringVar(&buildOut, "out", filepath.Join(".", "symbols", "linux"), "输出目录")
	fs.StringVar(&buildCacheDir, "cache-dir", cachepkg.DefaultDir(), "缓存目录")
	fs.StringVar(&buildDDEB, "ddeb", "", "本地 debug package 路径，支持 .ddeb/.deb")
	fs.StringVar(&buildDDEBURL, "ddeb-url", "", "手工指定 debug package URL")
	fs.StringVar(&buildDebugPackage, "debug-package", "", "本地 debug package 路径，支持 .ddeb/.deb/.rpm")
	fs.StringVar(&buildDebugPackageURL, "debug-package-url", "", "手工指定 debug package URL")
	fs.StringVar(&buildRepoURL, "repo-url", "", "RPM repo base URL，需包含 repodata/repomd.xml")
	fs.StringVar(&buildSymbolSources, "symbol-sources", sourcespkg.DefaultPath(), "远程符号源 TXT")
	fs.BoolVar(&buildNoRemoteSymbols, "no-remote-symbols", false, "禁用远程符号库查询")
	fs.StringVar(&buildVMLINUX, "vmlinux", "", "本地 vmlinux 路径")
	fs.StringVar(&buildVolPath, "vol", "vol", "Volatility 3 命令路径")
	fs.DurationVar(&buildProbeTimeout, "probe-timeout", 30*time.Second, "ddeb 存在性探测总超时，例如 30s、2m；0 表示不限制")
	fs.DurationVar(&buildDownloadTimeout, "download-timeout", 30*time.Minute, "下载超时时间，例如 30m、2h；0 表示不限制")
	fs.BoolVar(&buildForce, "force", false, "强制重新下载/生成")
	fs.BoolVar(&buildDryRun, "dry-run", false, "只解析和检查，不执行下载生成")
	fs.BoolVar(&log.Verbose, "verbose", false, "输出详细日志")
	fs.BoolVar(&buildJSON, "json", false, "以 JSON 格式输出")
}

func runBuild(args []string) {
	start := time.Now()
	jsonMode := buildJSON || JSONFlag

	summary, err := build(context.Background(), jsonMode)
	if summary != nil {
		summary.Duration = time.Since(start).Round(time.Millisecond).String()
	}
	if err != nil {
		if jsonMode {
			printBuildJSON(summary, err)
		} else {
			printBuildError(summary, err)
		}
		os.Exit(1)
	}
	if summary != nil {
		summary.Duration = time.Since(start).Round(time.Millisecond).String()
	}
	if jsonMode {
		printBuildJSON(summary, nil)
		return
	}
	printBuildSuccess(summary)
}

func build(ctx context.Context, jsonMode bool) (*buildSummary, error) {
	if buildBackend == "" {
		buildBackend = "wsl"
	}
	if buildBackend != "wsl" {
		return nil, fmt.Errorf("native backend 尚未支持；MVP 请使用 --backend wsl")
	}
	if !jsonMode {
		log.Info("准备构建符号表")
	}
	if !buildDryRun {
		if err := cachepkg.EnsureLayout(buildCacheDir); err != nil {
			return nil, fmt.Errorf("初始化缓存目录失败: %w", err)
		}
	}

	info, sourcePackage, candidates, foundURL, cacheHit, symbolSource, packageFormat, supportLevel, manualReason, sourceNames, err := resolveBuildInput(ctx, jsonMode)
	if err != nil {
		return partialSummary(info, candidates, foundURL, "", symbolSource, packageFormat, supportLevel, manualReason, sourceNames), err
	}
	if !buildDryRun {
		if err := cachepkg.EnsureLayout(buildCacheDir); err != nil {
			return nil, fmt.Errorf("初始化缓存目录失败: %w", err)
		}
	}
	if !buildDryRun && sourcePackage != "" {
		if _, err := os.Stat(sourcePackage); err != nil {
			return nil, fmt.Errorf("debug package 文件不可用: %w", err)
		}
	}
	if buildVMLINUX != "" && !buildDryRun {
		if _, err := os.Stat(buildVMLINUX); err != nil {
			return nil, fmt.Errorf("vmlinux 文件不可用: %w", err)
		}
	}

	info = symbols.MergeManual(&info, buildDistro, buildKernel, buildPkgver, buildArch)
	symbolName := symbols.FileName(info)
	symbolPath := filepath.Join(buildOut, symbolName)
	if symbolSource == "remote_isf" {
		if sourcePackage != "" {
			symbolPath = sourcePackage
		} else if foundURL != "" {
			symbolPath = filepath.Join(buildOut, sourcespkg.SymbolFileName(foundURL))
		}
	}
	summary := &buildSummary{
		Success:        false,
		DryRun:         buildDryRun,
		Distro:         info.Distro,
		Codename:       info.Codename,
		Kernel:         info.KernelRelease,
		PackageVersion: info.PackageVersion,
		Arch:           info.Arch,
		Backend:        buildBackend,
		OutputDir:      buildOut,
		SymbolPath:     symbolPath,
		PackagePath:    sourcePackage,
		FoundURL:       foundURL,
		Candidates:     candidates,
		CacheHit:       cacheHit,
		SymbolSource:   symbolSource,
		PackageFormat:  packageFormat,
		SupportLevel:   supportLevel,
		ManualReason:   manualReason,
		SymbolSources:  sourceNames,
	}

	if buildDryRun {
		summary.Success = true
		return summary, nil
	}

	if symbolSource == "remote_isf" {
		if sourcePackage == "" {
			return summary, fmt.Errorf("远程符号源命中但未得到本地符号路径")
		}
		summary.SymbolPath = sourcePackage
		summary.Success = true
		if !jsonMode {
			log.Success("远程符号表已下载")
		}
		return summary, nil
	}

	if !buildForce {
		if fileExists(symbolPath) {
			summary.Success = true
			if !jsonMode {
				log.Success("符号表已存在，使用 --force 可重新生成")
			}
			return summary, nil
		}
	}

	if err := os.MkdirAll(buildOut, 0o755); err != nil {
		return summary, fmt.Errorf("创建输出目录失败: %w", err)
	}

	req := backendpkg.BuildRequest{
		DDEBPath:       sourcePackage,
		PackageFormat:  packageFormat,
		VmlinuxPath:    buildVMLINUX,
		Kernel:         info.KernelRelease,
		PackageVersion: info.PackageVersion,
		Arch:           info.Arch,
		OutDir:         buildOut,
		WorkDir:        filepath.Join(cachepkg.ExtractedDir(buildCacheDir), safeWorkDirName(symbolName)),
		SymbolFileName: symbolName,
		Force:          buildForce,
	}
	var stageUpdate func(string)
	var extractUpdate func(int, int, string)
	stageProgress := newStageProgress(!jsonMode)
	if stageProgress != nil {
		defer stageProgress.Close()
		stageUpdate = stageProgress.Update
		extractUpdate = stageProgress.UpdateExtract
	}
	wsl := backendpkg.WSL{Distro: buildWslDistro, Verbose: log.Verbose, Stage: stageUpdate, Extract: extractUpdate}

	if buildVMLINUX != "" {
		if !jsonMode {
			log.Info("从本地 vmlinux 生成符号表")
		}
		out, err := wsl.BuildFromVMLINUX(ctx, req)
		if err != nil {
			return summary, fmt.Errorf("WSL vmlinux 构建失败: %w", err)
		}
		summary.SymbolPath = out.SymbolPath
		summary.VmlinuxPath = out.VmlinuxPath
		summary.Success = true
		return summary, nil
	}

	if sourcePackage == "" {
		return summary, fmt.Errorf("缺少 debug package 输入；可使用 --debug-package、--debug-package-url、--ddeb、--ddeb-url、--banner 或 --banner-file")
	}
	if !jsonMode {
		log.Info("通过 WSL 解包 debug package 并运行 dwarf2json")
	}
	out, err := wsl.BuildFromDDEB(ctx, req)
	if err != nil {
		return summary, fmt.Errorf("WSL ddeb 构建失败: %w", err)
	}
	summary.SymbolPath = out.SymbolPath
	summary.VmlinuxPath = out.VmlinuxPath
	summary.Success = true
	return summary, nil
}

func resolveBuildInput(ctx context.Context, jsonMode bool) (bannerpkg.KernelInfo, string, []string, string, bool, string, string, string, string, []string, error) {
	debugPackage := firstNonEmpty(buildDebugPackage, buildDDEB)
	switch {
	case buildVMLINUX != "":
		info := symbols.MergeManual(symbols.InferFromVMLINUX(buildVMLINUX), buildDistro, buildKernel, buildPkgver, buildArch)
		return info, "", nil, "", false, "manual", resolver.FormatVMLINUX, resolver.SupportVMLINUXOnly, "", nil, nil
	case debugPackage != "":
		info, _ := symbols.InferFromDDEB(debugPackage)
		merged := symbols.MergeManual(info, buildDistro, buildKernel, buildPkgver, buildArch)
		if merged.KernelRelease == "" || merged.PackageVersion == "" {
			return merged, debugPackage, nil, "", false, "manual", resolver.FormatUnknown, resolver.SupportManualPackage, "", nil, fmt.Errorf("无法从 debug package 文件名推断 kernel/pkgver；请指定 --kernel 和 --pkgver")
		}
		return merged, debugPackage, nil, "", false, "manual", symbols.PackageFormat(debugPackage), resolver.SupportManualPackage, "", nil, nil
	case buildBanner != "" || buildBannerFile != "":
		info, err := parseBuildBanner()
		if err != nil {
			return bannerpkg.KernelInfo{}, "", nil, "", false, "", "", "", "", nil, err
		}
		return resolveAndDownload(ctx, *info, jsonMode)
	case buildMem != "":
		if !jsonMode {
			log.Info("从内存镜像提取 banner")
		}
		text, _, err := volatility.ExtractBanner(ctx, buildVolPath, buildMem)
		if err != nil {
			return bannerpkg.KernelInfo{}, "", nil, "", false, "", "", "", "", nil, err
		}
		info, err := bannerpkg.ParseBanner(text)
		if err != nil {
			return *info, "", nil, "", false, "", "", "", "", nil, err
		}
		return resolveAndDownload(ctx, *info, jsonMode)
	default:
		info := symbols.MergeManual(nil, buildDistro, buildKernel, buildPkgver, buildArch)
		if info.KernelRelease == "" || info.PackageVersion == "" {
			bannerText, err := readBannerFromTerminal(jsonMode)
			if err != nil {
				return info, "", nil, "", false, "", "", "", "", nil, fmt.Errorf("需要终端输入 banner，或使用 --vmlinux、--debug-package、--mem，或手工指定 --kernel + --pkgver: %w", err)
			}
			parsed, err := bannerpkg.ParseBanner(bannerText)
			if err != nil {
				return info, "", nil, "", false, "", "", "", "", nil, err
			}
			return resolveAndDownload(ctx, *parsed, jsonMode)
		}
		return resolveAndDownload(ctx, info, jsonMode)
	}
}

func parseBuildBanner() (*bannerpkg.KernelInfo, error) {
	if buildBanner != "" {
		return bannerpkg.ParseBanner(buildBanner)
	}
	data, err := os.ReadFile(buildBannerFile)
	if err != nil {
		return nil, fmt.Errorf("无法读取 banner 文件: %w", err)
	}
	return bannerpkg.ParseBanner(strings.TrimSpace(string(data)))
}

func resolveAndDownload(ctx context.Context, info bannerpkg.KernelInfo, jsonMode bool) (bannerpkg.KernelInfo, string, []string, string, bool, string, string, string, string, []string, error) {
	info.Distro = strings.ToLower(info.Distro)
	sources, sourceNames, err := loadSymbolSourcesForOutput(buildSymbolSources, buildNoRemoteSymbols)
	if err != nil {
		return info, "", nil, "", false, "", "", "", "", nil, err
	}
	if !buildNoRemoteSymbols && info.Banner != "" {
		match, warnings, err := sourcespkg.Find(ctx, &http.Client{Timeout: 20 * time.Second}, sources, info.Banner)
		if !jsonMode {
			for _, warning := range warnings {
				log.Warn("远程符号源失败: %s", warning)
			}
		}
		if err != nil {
			return info, "", nil, "", false, "", "", "", "", sourceNames, err
		}
		if match != nil {
			if buildDryRun {
				return info, "", []string{match.URL}, match.URL, false, "remote_isf", resolver.FormatISF, resolver.SupportAutoDownload, "", sourceNames, nil
			}
			path, meta, err := downloadRemoteSymbol(ctx, match.URL, match.SymbolPath, jsonMode)
			cacheHit := false
			if meta != nil {
				cacheHit = meta.CacheHit
			}
			return info, path, []string{match.URL}, match.URL, cacheHit, "remote_isf", resolver.FormatISF, resolver.SupportAutoDownload, "", sourceNames, err
		}
	}

	debugPackageURL := firstNonEmpty(buildDebugPackageURL, buildDDEBURL)
	if debugPackageURL != "" {
		if buildDryRun {
			return info, "", []string{debugPackageURL}, debugPackageURL, false, "debug_package", symbols.PackageFormat(debugPackageURL), resolver.SupportManualPackage, "", sourceNames, nil
		}
		path, meta, err := downloadPackage(ctx, debugPackageURL, jsonMode)
		cacheHit := false
		if meta != nil {
			cacheHit = meta.CacheHit
		}
		return info, path, []string{debugPackageURL}, debugPackageURL, cacheHit, "debug_package", symbols.PackageFormat(debugPackageURL), resolver.SupportManualPackage, "", sourceNames, err
	}

	if buildRepoURL != "" {
		if !jsonMode {
			log.Info("从 RPM repo metadata 查找 debug package")
		}
		repoCtx := ctx
		var cancel context.CancelFunc
		if buildProbeTimeout > 0 {
			repoCtx, cancel = context.WithTimeout(ctx, buildProbeTimeout)
			defer cancel()
		}
		resolved, err := resolver.ResolveRpmRepo(repoCtx, &info, buildRepoURL, &http.Client{})
		if err != nil {
			return info, "", resolved.Candidates, "", false, "debug_package", resolved.PackageFormat, resolved.SupportLevel, resolved.ManualReason, sourceNames, fmt.Errorf("RPM repo 未找到对应 debug package: repo=%s reason=%w", buildRepoURL, err)
		}
		if buildDryRun {
			return info, "", resolved.Candidates, resolved.FoundURL, false, "debug_package", resolved.PackageFormat, resolved.SupportLevel, resolved.ManualReason, sourceNames, nil
		}
		path, meta, err := downloadPackage(ctx, resolved.FoundURL, jsonMode)
		cacheHit := false
		if meta != nil {
			cacheHit = meta.CacheHit
		}
		return info, path, resolved.Candidates, resolved.FoundURL, cacheHit, "debug_package", resolved.PackageFormat, resolved.SupportLevel, resolved.ManualReason, sourceNames, err
	}

	result := resolver.GenerateCandidates(&info)
	if !jsonMode {
		log.Info("解析 %s debug package 候选", info.Distro)
	}
	if buildDryRun {
		return info, "", result.Candidates, "", false, "debug_package", result.PackageFormat, result.SupportLevel, result.ManualReason, sourceNames, nil
	}
	if len(result.Candidates) == 0 {
		return info, "", nil, "", false, "manual", result.PackageFormat, result.SupportLevel, result.ManualReason, sourceNames, fmt.Errorf("%s 暂不支持自动定位 debug package；请使用 --vmlinux、--debug-package-url 或 --debug-package", info.Distro)
	}

	if !jsonMode {
		log.Info("探测 debug package 是否存在")
	}
	probeCtx := ctx
	var cancel context.CancelFunc
	if buildProbeTimeout > 0 {
		probeCtx, cancel = context.WithTimeout(ctx, buildProbeTimeout)
		defer cancel()
	}
	probeProgress := newProbeProgress(jsonMode)
	resolved, err := resolver.ResolvePackageProgress(probeCtx, &info, &http.Client{}, probeProgress)
	if probeProgress != nil {
		probeProgress(resolver.ProbeEvent{})
	}
	if err != nil {
		return info, "", result.Candidates, "", false, "debug_package", result.PackageFormat, result.SupportLevel, result.ManualReason, sourceNames, fmt.Errorf("未找到对应 debug symbol 包: timeout=%s reason=%w；可重试、使用 --probe-timeout 2m，或用 --debug-package-url / --debug-package 手工指定", buildProbeTimeout, err)
	}

	filePath, meta, err := downloadPackage(ctx, resolved.FoundURL, jsonMode)
	cacheHit := false
	if meta != nil {
		cacheHit = meta.CacheHit
	}
	return info, filePath, resolved.Candidates, resolved.FoundURL, cacheHit, "debug_package", resolved.PackageFormat, resolved.SupportLevel, resolved.ManualReason, sourceNames, err
}

func downloadPackage(ctx context.Context, rawURL string, jsonMode bool) (string, *cachepkg.DownloadMeta, error) {
	dest := cachepkg.DownloadFilePath(buildCacheDir, rawURL)
	if !jsonMode {
		log.Info("下载 debug package: %s", rawURL)
	}
	return downloadTo(ctx, rawURL, dest, jsonMode)
}

func downloadRemoteSymbol(ctx context.Context, rawURL, symbolPath string, jsonMode bool) (string, *cachepkg.DownloadMeta, error) {
	dest := filepath.Join(buildOut, sourcespkg.SymbolFileName(symbolPath))
	if !jsonMode {
		log.Info("下载远程符号表: %s", rawURL)
	}
	return downloadTo(ctx, rawURL, dest, jsonMode)
}

func downloadTo(ctx context.Context, rawURL, dest string, jsonMode bool) (string, *cachepkg.DownloadMeta, error) {
	if !jsonMode {
		log.Info("下载: %s", rawURL)
	}
	progress := newDownloadProgress(jsonMode)
	result, err := downloader.Download(ctx, &http.Client{Timeout: buildDownloadTimeout}, rawURL, dest, buildForce, progress)
	if progress != nil {
		progress(downloader.Progress{Done: true})
	}
	if err != nil {
		return "", nil, fmt.Errorf("下载失败: url=%s cache=%s timeout=%s reason=%w；如果网络较慢，可重试或使用 --download-timeout 2h", rawURL, dest, buildDownloadTimeout, err)
	}
	meta := result.DownloadMeta
	if err := cachepkg.WriteDownloadMeta(buildCacheDir, meta); err != nil {
		return "", &meta, fmt.Errorf("写入缓存元数据失败: %w", err)
	}
	return dest, &meta, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func partialSummary(info bannerpkg.KernelInfo, candidates []string, foundURL, symbolPath, symbolSource, packageFormat, supportLevel, manualReason string, sourceNames []string) *buildSummary {
	return &buildSummary{
		Success:        false,
		DryRun:         buildDryRun,
		Distro:         info.Distro,
		Codename:       info.Codename,
		Kernel:         info.KernelRelease,
		PackageVersion: info.PackageVersion,
		Arch:           info.Arch,
		Backend:        buildBackend,
		OutputDir:      buildOut,
		SymbolPath:     symbolPath,
		FoundURL:       foundURL,
		Candidates:     candidates,
		SymbolSource:   symbolSource,
		PackageFormat:  packageFormat,
		SupportLevel:   supportLevel,
		ManualReason:   manualReason,
		SymbolSources:  sourceNames,
	}
}

func printBuildJSON(summary *buildSummary, err error) {
	if summary == nil {
		summary = &buildSummary{}
	}
	if err != nil {
		type withError struct {
			*buildSummary
			Error string `json:"error"`
		}
		data, _ := json.MarshalIndent(withError{buildSummary: summary, Error: err.Error()}, "", "  ")
		fmt.Println(string(data))
		return
	}
	data, _ := json.MarshalIndent(summary, "", "  ")
	fmt.Println(string(data))
}

func printBuildSuccess(summary *buildSummary) {
	if summary == nil {
		return
	}
	if summary.DryRun {
		log.Success("dry-run 完成，不下载、不生成")
	} else {
		log.Success("符号表生成成功")
	}
	fmt.Printf("    Distro          : %s\n", summary.Distro)
	fmt.Printf("    Codename        : %s\n", summary.Codename)
	fmt.Printf("    Kernel          : %s\n", summary.Kernel)
	fmt.Printf("    Package Version : %s\n", summary.PackageVersion)
	fmt.Printf("    Arch            : %s\n", summary.Arch)
	if summary.FoundURL != "" {
		fmt.Printf("    Found URL       : %s\n", summary.FoundURL)
	}
	if summary.SymbolSource != "" {
		fmt.Printf("    Symbol Source   : %s\n", summary.SymbolSource)
	}
	if summary.PackageFormat != "" {
		fmt.Printf("    Package Format  : %s\n", summary.PackageFormat)
	}
	if summary.SupportLevel != "" {
		fmt.Printf("    Support Level   : %s\n", summary.SupportLevel)
	}
	if summary.ManualReason != "" {
		fmt.Printf("    Manual Reason   : %s\n", summary.ManualReason)
	}
	if len(summary.SymbolSources) > 0 {
		fmt.Printf("    Symbol Sources  : %s\n", strings.Join(summary.SymbolSources, ", "))
	}
	if len(summary.Candidates) > 0 {
		fmt.Printf("    Candidate URLs\n")
		for i, candidate := range summary.Candidates {
			fmt.Printf("      %d. %s\n", i+1, candidate)
		}
	}
	fmt.Printf("    Output          : %s\n", summary.SymbolPath)
}

func printBuildError(summary *buildSummary, err error) {
	log.Error("%v", err)
	if summary != nil {
		if len(summary.Candidates) > 0 {
			fmt.Fprintln(os.Stderr, "Tried URLs:")
			for _, candidate := range summary.Candidates {
				fmt.Fprintf(os.Stderr, "  - %s\n", candidate)
			}
		}
		if summary.Kernel != "" || summary.PackageVersion != "" {
			fmt.Fprintf(os.Stderr, "Kernel: %s\nPackage Version: %s\nArch: %s\n", summary.Kernel, summary.PackageVersion, summary.Arch)
		}
	}
	if errors.Is(err, resolver.ErrPackageNotFound) {
		fmt.Fprintln(os.Stderr, "建议：检查 banner 是否来自受支持的官方内核，或使用 --debug-package-url / --debug-package / --vmlinux 手工指定。")
	}
}

func fileExists(filePath string) bool {
	info, err := os.Stat(filePath)
	return err == nil && !info.IsDir()
}

func safeWorkDirName(name string) string {
	replacer := strings.NewReplacer("\\", "_", "/", "_", ":", "_", "*", "_", "?", "_", "\"", "_", "<", "_", ">", "_", "|", "_")
	return replacer.Replace(name)
}
