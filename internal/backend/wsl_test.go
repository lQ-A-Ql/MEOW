package backend

import (
	"strings"
	"testing"
)

func TestShellQuote(t *testing.T) {
	got := ShellQuote("C:\\Users\\QAQ\\a'b")
	want := `'C:\Users\QAQ\a'\''b'`
	if got != want {
		t.Fatalf("ShellQuote: got %q want %q", got, want)
	}
}

func TestWindowsPathToWSL(t *testing.T) {
	got, err := WindowsPathToWSL(`C:\Users\QAQ\symbols linux`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "/mnt/c/Users/QAQ/symbols linux"
	if got != want {
		t.Fatalf("WindowsPathToWSL: got %q want %q", got, want)
	}
}

func TestWSLBashUsesNonInteractiveCleanBash(t *testing.T) {
	w := WSL{Distro: "Ubuntu"}
	args := w.bashArgs("echo ok")
	want := []string{"-d", "Ubuntu", "--exec", "bash", "--noprofile", "--norc", "-c", "echo ok"}
	if len(args) != len(want) {
		t.Fatalf("args length: got %d want %d: %#v", len(args), len(want), args)
	}
	for i := range want {
		if args[i] != want[i] {
			t.Fatalf("arg[%d]: got %q want %q", i, args[i], want[i])
		}
	}
}

func TestDDEBBuildScriptEmitsProgressStages(t *testing.T) {
	script := ddebBuildScript()
	for _, stage := range []string{
		"VOLSYM_STAGE=extract",
		"VOLSYM_EXTRACT_TOTAL=",
		"VOLSYM_EXTRACT_FILE=",
		"VOLSYM_STAGE=find_vmlinux",
		"VOLSYM_STAGE=dwarf2json",
		"VOLSYM_STAGE=compress",
		"VOLSYM_STAGE=move",
		"VOLSYM_STAGE=done",
	} {
		if !strings.Contains(script, stage) {
			t.Fatalf("expected ddeb build script to contain %q", stage)
		}
	}
}

func TestDebugPackageBuildScriptSupportsRPM(t *testing.T) {
	script := debugPackageBuildScript()
	for _, fragment := range []string{
		"rpm2cpio",
		"cpio -idmv",
		"/usr/lib/debug/lib/modules/$KERNEL/vmlinux",
		"/usr/lib/debug/lib64/modules/$KERNEL/vmlinux",
		"vmlinux*.gz",
		"vmlinux*.xz",
		"vmlinux*.zst",
		"gzip -dc",
		"zstd -dc",
	} {
		if !strings.Contains(script, fragment) {
			t.Fatalf("expected debug package build script to contain %q", fragment)
		}
	}
}

func TestParseExtractFileMarker(t *testing.T) {
	current, total, file := parseExtractFileMarker("12/34:./usr/lib/debug/boot/vmlinux-test")
	if current != 12 || total != 34 {
		t.Fatalf("progress: got %d/%d", current, total)
	}
	if file != "./usr/lib/debug/boot/vmlinux-test" {
		t.Fatalf("file: got %q", file)
	}
}

func TestVMLINUXBuildScriptEmitsProgressStages(t *testing.T) {
	script := vmlinuxBuildScript()
	for _, stage := range []string{
		"VOLSYM_STAGE=dwarf2json",
		"VOLSYM_STAGE=compress",
		"VOLSYM_STAGE=move",
		"VOLSYM_STAGE=done",
	} {
		if !strings.Contains(script, stage) {
			t.Fatalf("expected vmlinux build script to contain %q", stage)
		}
	}
}
