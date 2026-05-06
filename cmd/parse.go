package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	bannerpkg "meow/internal/banner"
	"meow/internal/log"
	"meow/internal/resolver"
	sourcespkg "meow/internal/symbolsources"
)

var (
	parseBanner     string
	parseBannerFile string
	parseDistro     string
	parseSources    string
	parseNoRemote   bool
	parseJSON       bool
)

func init() {
	fs := Register("parse", "解析 banner 并查询远程符号源", runParse)
	fs.StringVar(&parseBanner, "banner", "", "直接传入 banner 字符串")
	fs.StringVar(&parseBannerFile, "banner-file", "", "从文件读取 banner")
	fs.StringVar(&parseDistro, "distro", "", "发行版覆盖，例如 ubuntu/debian")
	fs.StringVar(&parseSources, "symbol-sources", sourcespkg.DefaultPath(), "远程符号源 TXT")
	fs.BoolVar(&parseNoRemote, "no-remote-symbols", false, "禁用远程符号库查询")
	fs.BoolVar(&log.Verbose, "verbose", false, "输出详细日志")
	fs.BoolVar(&parseJSON, "json", false, "以 JSON 格式输出")
}

func runParse(args []string) {
	var bannerText string
	jsonMode := parseJSON || JSONFlag

	switch {
	case parseBanner != "":
		bannerText = parseBanner
	case parseBannerFile != "":
		data, err := os.ReadFile(parseBannerFile)
		if err != nil {
			log.Fatal("无法读取 banner 文件: %v", err)
		}
		bannerText = strings.TrimSpace(string(data))
	default:
		var err error
		bannerText, err = readBannerFromTerminal(jsonMode)
		if err != nil {
			log.Fatal("读取 banner 失败: %v", err)
		}
	}

	if !jsonMode {
		log.Info("正在解析 banner")
	}

	info, err := bannerpkg.ParseBanner(bannerText)
	if err != nil {
		log.Error("Banner 解析失败: %v", err)
		if info != nil && info.KernelRelease != "" {
			log.Info("已提取内核: %s", info.KernelRelease)
		}
		os.Exit(1)
	}
	info = mergeParseDistro(info, parseDistro)

	result := resolver.GenerateCandidates(info)
	sources, sourceNames, sourceErr := loadSymbolSourcesForOutput(parseSources, parseNoRemote)
	if sourceErr != nil {
		log.Fatal("读取符号源失败: %v", sourceErr)
	}
	remoteCandidates := remoteSymbolCandidates(sources)
	remoteSource := ""
	remoteWarnings := []string{}
	if !parseNoRemote && info.Banner != "" {
		match, warnings, err := sourcespkg.Find(context.Background(), &http.Client{Timeout: 20 * time.Second}, sources, info.Banner)
		remoteWarnings = warnings
		if err != nil {
			log.Fatal("查询远程符号源失败: %v", err)
		}
		if match != nil {
			remoteSource = match.Source.Name
			remoteCandidates = []string{match.URL}
			result.SupportLevel = resolver.SupportAutoDownload
			result.PackageFormat = resolver.FormatISF
			result.ManualReason = ""
		}
	}

	if jsonMode {
		output, _ := json.MarshalIndent(parseResultJSON{
			Distro:                 info.Distro,
			Codename:               info.Codename,
			Kernel:                 info.KernelRelease,
			PackageVersion:         info.PackageVersion,
			Arch:                   info.Arch,
			SourcePackage:          info.SourcePackage,
			RepoBase:               result.RepoBase,
			PackageName:            result.PackageName,
			PackageFormat:          result.PackageFormat,
			SupportLevel:           result.SupportLevel,
			ManualReason:           result.ManualReason,
			Candidates:             result.Candidates,
			SymbolSourcesPath:      parseSources,
			SymbolSources:          sourceNames,
			RemoteSymbolCandidates: remoteCandidates,
			RemoteSymbolSource:     remoteSource,
			RemoteSymbolWarnings:   remoteWarnings,
		}, "", "  ")
		fmt.Println(string(output))
		return
	}

	fmt.Println()
	log.Info("解析结果")
	fmt.Printf("    发行版        %s\n", info.Distro)
	fmt.Printf("    代号          %s\n", info.Codename)
	fmt.Printf("    内核          %s\n", info.KernelRelease)
	fmt.Printf("    包版本        %s\n", info.PackageVersion)
	fmt.Printf("    架构          %s\n", info.Arch)
	fmt.Printf("    源码包        %s\n", info.SourcePackage)
	fmt.Printf("    支持级别       %s\n", result.SupportLevel)
	if result.ManualReason != "" {
		fmt.Printf("    手工原因       %s\n", result.ManualReason)
	}
	fmt.Printf("    符号源文件     %s\n", parseSources)
	if remoteSource != "" {
		fmt.Printf("    远程命中       %s\n", remoteSource)
	}
	for _, warning := range remoteWarnings {
		log.Warn("远程符号源失败: %s", warning)
	}
	fmt.Printf("    仓库基地址     %s\n", result.RepoBase)
	if len(result.Candidates) > 0 {
		fmt.Printf("    候选包        %s\n", result.PackageName)
		fmt.Printf("    候选 URL\n")
		for i, candidate := range result.Candidates {
			fmt.Printf("      %d. %s\n", i+1, candidate)
		}
	}
	fmt.Println()
}

func loadSymbolSourcesForOutput(filePath string, disabled bool) ([]sourcespkg.Source, []string, error) {
	if disabled {
		return nil, nil, nil
	}
	sources, err := sourcespkg.Load(filePath)
	if err != nil {
		return nil, nil, err
	}
	names := make([]string, 0, len(sources))
	for _, source := range sources {
		names = append(names, source.Name)
	}
	return sources, names, nil
}

func remoteSymbolCandidates(sources []sourcespkg.Source) []string {
	candidates := make([]string, 0, len(sources))
	for _, source := range sources {
		candidates = append(candidates, source.IndexURL)
	}
	return candidates
}

func mergeParseDistro(info *bannerpkg.KernelInfo, distro string) *bannerpkg.KernelInfo {
	if info != nil && strings.TrimSpace(distro) != "" {
		info.Distro = strings.ToLower(strings.TrimSpace(distro))
	}
	return info
}

type parseResultJSON struct {
	Distro                 string   `json:"distro"`
	Codename               string   `json:"codename"`
	Kernel                 string   `json:"kernel"`
	PackageVersion         string   `json:"package_version"`
	Arch                   string   `json:"arch"`
	SourcePackage          string   `json:"source_package"`
	RepoBase               string   `json:"repo_base"`
	PackageName            string   `json:"package_name,omitempty"`
	PackageFormat          string   `json:"package_format,omitempty"`
	SupportLevel           string   `json:"support_level,omitempty"`
	ManualReason           string   `json:"manual_reason,omitempty"`
	Candidates             []string `json:"candidates"`
	SymbolSourcesPath      string   `json:"symbol_sources_path,omitempty"`
	SymbolSources          []string `json:"symbol_sources,omitempty"`
	RemoteSymbolCandidates []string `json:"remote_symbol_candidates,omitempty"`
	RemoteSymbolSource     string   `json:"remote_symbol_source,omitempty"`
	RemoteSymbolWarnings   []string `json:"remote_symbol_warnings,omitempty"`
}
