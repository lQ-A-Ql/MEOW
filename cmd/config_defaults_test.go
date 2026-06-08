package cmd

import (
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestApplyConfigDefaultsFillsMissingFields(t *testing.T) {
	cfg := applyConfigDefaults(configFile{})
	if cfg.CacheDir == "" || cfg.OutputDir == "" || cfg.SymbolSourcesPath == "" || cfg.VolatilityPath == "" {
		t.Fatalf("expected defaults to be filled: %#v", cfg)
	}
	if cfg.DownloadTimeoutSeconds <= 0 || cfg.MaxRetries <= 0 {
		t.Fatalf("expected numeric defaults: %#v", cfg)
	}
}

func TestBuildConfigDefaultsRespectExplicitFlags(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	writeTestConfig(t, home, configFile{
		CacheDir:               filepath.Join(home, "cache-from-config"),
		OutputDir:              filepath.Join(home, "out-from-config"),
		SymbolSourcesPath:      filepath.Join(home, "sources-from-config.txt"),
		VolatilityPath:         "vol-from-config",
		DownloadTimeoutSeconds: 123,
		MaxRetries:             7,
	})

	buildCacheDir = ""
	buildOut = filepath.Join(".", "symbols", "linux")
	buildSymbolSources = ""
	buildVolPath = "vol"
	buildDownloadTimeout = 30 * time.Minute
	fs := flag.NewFlagSet("build-test", flag.ContinueOnError)
	fs.StringVar(&buildOut, "out", buildOut, "")
	if err := fs.Parse([]string{"--out", "flag-out"}); err != nil {
		t.Fatal(err)
	}
	replaceCommandFlagsForTest(t, "build", fs)

	applyBuildConfigDefaults(true)

	if buildCacheDir != filepath.Join(home, "cache-from-config") {
		t.Fatalf("cache dir: %q", buildCacheDir)
	}
	if buildOut != "flag-out" {
		t.Fatalf("explicit out flag should win, got %q", buildOut)
	}
	if buildSymbolSources != filepath.Join(home, "sources-from-config.txt") {
		t.Fatalf("symbol sources: %q", buildSymbolSources)
	}
	if buildVolPath != "vol-from-config" {
		t.Fatalf("vol path: %q", buildVolPath)
	}
	if buildDownloadTimeout != 123*time.Second {
		t.Fatalf("download timeout: %s", buildDownloadTimeout)
	}
}

func TestParseCacheVerifyConfigDefaults(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	writeTestConfig(t, home, configFile{
		CacheDir:               filepath.Join(home, "cache-config"),
		OutputDir:              filepath.Join(home, "out-config"),
		SymbolSourcesPath:      filepath.Join(home, "sources-config.txt"),
		VolatilityPath:         "vol-config",
		DownloadTimeoutSeconds: 60,
		MaxRetries:             3,
	})

	replaceCommandFlagsForTest(t, "parse", flag.NewFlagSet("parse-test", flag.ContinueOnError))
	replaceCommandFlagsForTest(t, "cache", flag.NewFlagSet("cache-test", flag.ContinueOnError))
	replaceCommandFlagsForTest(t, "verify", flag.NewFlagSet("verify-test", flag.ContinueOnError))
	parseSources = ""
	cacheDir = ""
	verifyVolPath = "vol"

	applyParseConfigDefaults(true)
	applyCacheConfigDefaults(true)
	applyVerifyConfigDefaults(true)

	if parseSources != filepath.Join(home, "sources-config.txt") {
		t.Fatalf("parse sources: %q", parseSources)
	}
	if cacheDir != filepath.Join(home, "cache-config") {
		t.Fatalf("cache dir: %q", cacheDir)
	}
	if verifyVolPath != "vol-config" {
		t.Fatalf("verify vol path: %q", verifyVolPath)
	}
}

func writeTestConfig(t *testing.T, home string, cfg configFile) {
	t.Helper()
	path := filepath.Join(home, ".meow", "config.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func replaceCommandFlagsForTest(t *testing.T, name string, fs *flag.FlagSet) {
	t.Helper()
	cmd := Commands[name]
	if cmd == nil {
		t.Fatalf("command %q not registered", name)
	}
	original := cmd.Flags
	cmd.Flags = fs
	t.Cleanup(func() {
		cmd.Flags = original
	})
}
