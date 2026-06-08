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
	fs := Register("parse", "parse banner and query remote symbol sources", runParse)
	fs.StringVar(&parseBanner, "banner", "", "banner string")
	fs.StringVar(&parseBannerFile, "banner-file", "", "read banner from file")
	fs.StringVar(&parseDistro, "distro", "", "override distro, for example ubuntu/debian")
	fs.StringVar(&parseSources, "symbol-sources", sourcespkg.DefaultPath(), "remote symbol source TXT")
	fs.BoolVar(&parseNoRemote, "no-remote-symbols", false, "disable remote symbol source lookup")
	fs.BoolVar(&log.Verbose, "verbose", false, "verbose logging")
	fs.BoolVar(&parseJSON, "json", false, "output JSON")
}

func runParse(args []string) {
	jsonMode := parseJSON || JSONFlag
	applyParseConfigDefaults(jsonMode)

	bannerText, err := parseBannerInput(jsonMode)
	if err != nil {
		log.Fatal("read banner failed: %v", err)
	}

	if !jsonMode {
		log.Info("parsing banner")
	}

	info, err := bannerpkg.ParseBanner(bannerText)
	if err != nil {
		log.Error("Banner parse failed: %v", err)
		if info != nil && info.KernelRelease != "" {
			log.Info("extracted kernel: %s", info.KernelRelease)
		}
		os.Exit(1)
	}
	info = mergeParseDistro(info, parseDistro)

	result := resolver.GenerateCandidates(info)
	sources, sourceNames, sourceErr := loadSymbolSourcesForOutput(parseSources, parseNoRemote)
	if sourceErr != nil {
		log.Fatal("read symbol sources failed: %v", sourceErr)
	}
	remoteCandidates := remoteSymbolCandidates(sources)
	remoteSource := ""
	remoteWarnings := []string{}
	if !parseNoRemote && info.Banner != "" {
		match, warnings, err := sourcespkg.Find(context.Background(), &http.Client{Timeout: 20 * time.Second}, sources, info.Banner)
		remoteWarnings = warnings
		if err != nil {
			log.Fatal("query remote symbol sources failed: %v", err)
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
	log.Info("parse result")
	fmt.Printf("    Distro          %s\n", info.Distro)
	fmt.Printf("    Codename        %s\n", info.Codename)
	fmt.Printf("    Kernel          %s\n", info.KernelRelease)
	fmt.Printf("    Package Version %s\n", info.PackageVersion)
	fmt.Printf("    Arch            %s\n", info.Arch)
	fmt.Printf("    Source Package  %s\n", info.SourcePackage)
	fmt.Printf("    Support Level   %s\n", result.SupportLevel)
	if result.ManualReason != "" {
		fmt.Printf("    Manual Reason   %s\n", result.ManualReason)
	}
	fmt.Printf("    Symbol Sources  %s\n", parseSources)
	if remoteSource != "" {
		fmt.Printf("    Remote Hit      %s\n", remoteSource)
	}
	for _, warning := range remoteWarnings {
		log.Warn("remote symbol source failed: %s", warning)
	}
	fmt.Printf("    Repo Base       %s\n", result.RepoBase)
	if len(result.Candidates) > 0 {
		fmt.Printf("    Package         %s\n", result.PackageName)
		fmt.Printf("    Candidate URLs\n")
		for i, candidate := range result.Candidates {
			fmt.Printf("      %d. %s\n", i+1, candidate)
		}
	}
	fmt.Println()
}

func parseBannerInput(jsonMode bool) (string, error) {
	switch {
	case parseBanner != "":
		return parseBanner, nil
	case parseBannerFile != "":
		data, err := os.ReadFile(parseBannerFile)
		if err != nil {
			return "", err
		}
		return strings.TrimSpace(string(data)), nil
	default:
		return readBannerFromTerminal(jsonMode)
	}
}

func applyParseConfigDefaults(jsonMode bool) {
	cfg, err := readOrDefaultConfig()
	if err != nil {
		if !jsonMode {
			log.Warn("failed to read config defaults: %v", err)
		}
		return
	}
	if !flagWasSet(Commands["parse"].Flags, "symbol-sources") {
		parseSources = cfg.SymbolSourcesPath
	}
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
