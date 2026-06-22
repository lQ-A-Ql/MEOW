package cmd

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"

	"meow/internal/tui"
)

var tuiOptions tui.Options
var tuiPluginArgs stringListFlag

func init() {
	fs := Register("tui", "启动终端交互界面 (TUI)", runTUI)
	fs.Var(sourceModeFlag{target: &tuiOptions.SourceMode}, "source", "active TUI source mode")
	fs.StringVar(&tuiOptions.MeowPath, "meow", "", "meow executable path")
	fs.StringVar(&tuiOptions.VolPath, "vol", "vol", "Volatility 3 command path")
	fs.StringVar(&tuiOptions.MemPath, "mem", "", "memory image path")
	fs.StringVar(&tuiOptions.SymbolsPath, "symbols", "./symbols", "symbols directory")
	fs.StringVar(&tuiOptions.OutDir, "out", filepath.Join(".", "symbols", "linux"), "output directory")
	fs.StringVar(&tuiOptions.CacheDir, "cache-dir", "", "cache directory")
	fs.StringVar(&tuiOptions.SymbolSourcesPath, "symbol-sources", "", "remote symbol source TXT")
	fs.StringVar(&tuiOptions.BannerFile, "banner-file", "", "banner file path")
	fs.StringVar(&tuiOptions.DebugPackage, "debug-package", "", "local debug package path")
	fs.StringVar(&tuiOptions.DebugPackageURL, "debug-package-url", "", "debug package URL")
	fs.StringVar(&tuiOptions.RepoURL, "repo-url", "", "RPM repo base URL")
	fs.StringVar(&tuiOptions.VMLinuxPath, "vmlinux", "", "local vmlinux path")
	fs.StringVar(&tuiOptions.Distro, "distro", "", "distro override")
	fs.StringVar(&tuiOptions.Kernel, "kernel", "", "kernel release")
	fs.StringVar(&tuiOptions.PackageVersion, "pkgver", "", "package version")
	fs.StringVar(&tuiOptions.Arch, "arch", "amd64", "architecture")
	fs.StringVar(&tuiOptions.Plugin, "plugin", "", "Volatility plugin name")
	fs.Var(&tuiPluginArgs, "plugin-arg", "Volatility plugin argument; may be repeated")
	fs.BoolVar(&tuiOptions.NoRemoteSymbols, "no-remote-symbols", false, "disable remote symbol lookup")
	fs.BoolVar(&tuiOptions.Force, "force", false, "force rebuild/download")
}

func runTUI(args []string) {
	tuiOptions.PluginArgs = append([]string(nil), tuiPluginArgs...)
	options := applyTUIConfigDefaults(tuiOptions)
	m := tui.NewModelWithOptions(options)
	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "TUI error: %v\n", err)
		os.Exit(1)
	}
}

func applyTUIConfigDefaults(options tui.Options) tui.Options {
	cfg, err := readOrDefaultConfig()
	if err == nil {
		fs := Commands["tui"].Flags
		if !flagWasSet(fs, "cache-dir") {
			options.CacheDir = cfg.CacheDir
		}
		if !flagWasSet(fs, "out") {
			options.OutDir = cfg.OutputDir
		}
		if !flagWasSet(fs, "symbol-sources") {
			options.SymbolSourcesPath = cfg.SymbolSourcesPath
		}
		if !flagWasSet(fs, "vol") {
			options.VolPath = cfg.VolatilityPath
		}
	}
	if options.MeowPath == "" {
		if exe, err := os.Executable(); err == nil && exe != "" {
			options.MeowPath = exe
		}
	}
	return options.WithDefaults()
}

type sourceModeFlag struct {
	target *tui.InputMode
}

func (f sourceModeFlag) String() string {
	if f.target == nil {
		return ""
	}
	return string(*f.target)
}

func (f sourceModeFlag) Set(value string) error {
	mode, ok := tui.ParseInputMode(value)
	if !ok {
		return fmt.Errorf("unknown source mode: %s", value)
	}
	*f.target = mode
	return nil
}

type stringListFlag []string

func (f *stringListFlag) String() string {
	return fmt.Sprint([]string(*f))
}

func (f *stringListFlag) Set(value string) error {
	*f = append(*f, value)
	return nil
}

var _ flag.Value = sourceModeFlag{}
var _ flag.Value = (*stringListFlag)(nil)
