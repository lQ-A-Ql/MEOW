package tui

import (
	"reflect"
	"strings"
	"testing"
)

func TestParseFieldsSupportsQuotedPaths(t *testing.T) {
	got, err := parseFields(`/mem "C:\cases\memory image.raw" --flag 'two words'`)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"/mem", `C:\cases\memory image.raw`, "--flag", "two words"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("fields: got %#v want %#v", got, want)
	}
}

func TestParseFieldsRejectsUnclosedQuote(t *testing.T) {
	if _, err := parseFields(`/mem "missing`); err == nil {
		t.Fatal("expected unclosed quote error")
	}
}

func TestExecuteCommandSetsAllCoreInputs(t *testing.T) {
	m := NewModelWithOptions(Options{})
	commands := []string{
		`/mem "memory image.raw"`,
		`/banner-file banner.txt`,
		`/debug-package kernel.rpm`,
		`/debug-package-url https://example.test/kernel.rpm`,
		`/repo-url https://repo.example/debug`,
		`/vmlinux /tmp/vmlinux`,
		`/symbol ./symbols-custom`,
		`/out ./out`,
		`/cache-dir ./cache`,
		`/symbol-sources ./sources.txt`,
		`/vol ./vol`,
		`/meow ./meow`,
		`/plugin linux.custom.Plugin`,
		`/plugin-args --pid 1 --physical`,
		`/manual --distro ubuntu --kernel 5.4.0 --pkgver 5.4.0-1 --arch amd64`,
		`/remote off`,
		`/force on`,
	}
	for _, command := range commands {
		result := m.executeCommand(command)
		m = result.model
	}

	if m.memPath != "memory image.raw" || m.bannerFile != "banner.txt" || m.debugPackage != "kernel.rpm" {
		t.Fatalf("paths not set: %#v", m)
	}
	if m.debugPackageURL == "" || m.repoURL == "" || m.vmlinuxPath == "" {
		t.Fatalf("alternate inputs not set: %#v", m)
	}
	if m.symbolsPath != "./symbols-custom" || m.outDir != "./out" || m.cacheDir != "./cache" {
		t.Fatalf("environment paths not set: %#v", m)
	}
	if m.volPath != "./vol" || m.meowPath != "./meow" {
		t.Fatalf("tool paths not set: %#v", m)
	}
	if m.plugin != "linux.custom.Plugin" || !reflect.DeepEqual(m.pluginArgs, []string{"--pid", "1", "--physical"}) {
		t.Fatalf("plugin not set: %#v", m)
	}
	if m.distro != "ubuntu" || m.kernel != "5.4.0" || m.packageVersion != "5.4.0-1" || m.arch != "amd64" {
		t.Fatalf("manual input not set: %#v", m)
	}
	if !m.noRemoteSymbols || !m.force {
		t.Fatalf("booleans not set: remote disabled=%t force=%t", m.noRemoteSymbols, m.force)
	}
}

func TestCacheClearRequiresConfirm(t *testing.T) {
	m := NewModelWithOptions(Options{})
	result := m.executeCommand(`/cache clear`)
	if result.action != actionNone {
		t.Fatalf("expected no action without confirm, got %q", result.action)
	}
	if len(result.logs) == 0 || !strings.Contains(result.logs[0].message, "requires explicit confirmation") {
		t.Fatalf("expected confirmation warning, got %#v", result.logs)
	}

	result = m.executeCommand(`/cache clear --force --confirm`)
	if result.action != actionCacheClear {
		t.Fatalf("expected cache clear action, got %q", result.action)
	}
	if !result.model.cacheClearForce {
		t.Fatal("expected force flag to be recorded")
	}
}

func TestBuildArgsByInputMode(t *testing.T) {
	tests := []struct {
		name string
		m    Model
		want []string
	}{
		{
			name: "mem",
			m:    NewModelWithOptions(Options{MemPath: "mem.raw", NoRemoteSymbols: true}),
			want: []string{"--json", "build", "--dry-run", "--mem", "mem.raw", "--arch", "amd64", "--out", "symbols/linux", "--cache-dir", "cache", "--symbol-sources", "sources.txt", "--vol", "vol", "--no-remote-symbols"},
		},
		{
			name: "debug package",
			m:    NewModelWithOptions(Options{DebugPackage: "pkg.rpm"}),
			want: []string{"--json", "build", "--dry-run", "--debug-package", "pkg.rpm", "--arch", "amd64", "--out", "symbols/linux", "--cache-dir", "cache", "--symbol-sources", "sources.txt", "--vol", "vol"},
		},
		{
			name: "repo manual",
			m:    NewModelWithOptions(Options{RepoURL: "https://repo", Kernel: "k", PackageVersion: "p", Distro: "centos"}),
			want: []string{"--json", "build", "--dry-run", "--repo-url", "https://repo", "--kernel", "k", "--pkgver", "p", "--distro", "centos", "--arch", "amd64", "--out", "symbols/linux", "--cache-dir", "cache", "--symbol-sources", "sources.txt", "--vol", "vol"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.m.outDir = "symbols/linux"
			tt.m.cacheDir = "cache"
			tt.m.symbolSourcesPath = "sources.txt"
			got := tt.m.buildArgs(true)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("args:\ngot  %#v\nwant %#v", got, tt.want)
			}
		})
	}
}
