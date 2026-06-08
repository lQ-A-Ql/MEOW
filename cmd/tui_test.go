package cmd

import (
	"flag"
	"path/filepath"
	"testing"

	tuipkg "meow/internal/tui"
)

func TestApplyTUIConfigDefaultsUsesConfigAndExecutableDefault(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	writeTestConfig(t, home, configFile{
		CacheDir:               filepath.Join(home, "cache"),
		OutputDir:              filepath.Join(home, "out"),
		SymbolSourcesPath:      filepath.Join(home, "sources.txt"),
		VolatilityPath:         "vol-config",
		DownloadTimeoutSeconds: 60,
		MaxRetries:             3,
	})
	replaceCommandFlagsForTest(t, "tui", flag.NewFlagSet("tui-test", flag.ContinueOnError))

	got := applyTUIConfigDefaults(tuipkg.Options{})
	if got.MeowPath == "" {
		t.Fatal("expected meow path to default to current executable")
	}
	if got.CacheDir != filepath.Join(home, "cache") {
		t.Fatalf("cache dir: %q", got.CacheDir)
	}
	if got.OutDir != filepath.Join(home, "out") {
		t.Fatalf("out dir: %q", got.OutDir)
	}
	if got.SymbolSourcesPath != filepath.Join(home, "sources.txt") {
		t.Fatalf("symbol sources: %q", got.SymbolSourcesPath)
	}
	if got.VolPath != "vol-config" {
		t.Fatalf("vol path: %q", got.VolPath)
	}
}

func TestApplyTUIConfigDefaultsRespectsExplicitFlags(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	writeTestConfig(t, home, configFile{
		CacheDir:               filepath.Join(home, "cache"),
		OutputDir:              filepath.Join(home, "out"),
		SymbolSourcesPath:      filepath.Join(home, "sources.txt"),
		VolatilityPath:         "vol-config",
		DownloadTimeoutSeconds: 60,
		MaxRetries:             3,
	})

	fs := flag.NewFlagSet("tui-test", flag.ContinueOnError)
	fs.String("cache-dir", "", "")
	fs.String("out", "", "")
	fs.String("symbol-sources", "", "")
	fs.String("vol", "", "")
	if err := fs.Parse([]string{"--cache-dir", "flag-cache", "--out", "flag-out", "--symbol-sources", "flag-sources", "--vol", "flag-vol"}); err != nil {
		t.Fatal(err)
	}
	replaceCommandFlagsForTest(t, "tui", fs)

	got := applyTUIConfigDefaults(tuipkg.Options{
		CacheDir:          "flag-cache",
		OutDir:            "flag-out",
		SymbolSourcesPath: "flag-sources",
		VolPath:           "flag-vol",
		MeowPath:          "flag-meow",
	})
	if got.CacheDir != "flag-cache" || got.OutDir != "flag-out" || got.SymbolSourcesPath != "flag-sources" || got.VolPath != "flag-vol" || got.MeowPath != "flag-meow" {
		t.Fatalf("explicit flags not respected: %#v", got)
	}
}
