package backend

import (
	"context"
	"os"
	"runtime"
	"strings"
	"testing"
)

func TestShellQuote(t *testing.T) {
	got := ShellQuote("/tmp/a'b")
	want := `'/tmp/a'\''b'`
	if got != want {
		t.Fatalf("ShellQuote: got %q want %q", got, want)
	}
}

func TestShellQuoteSpaces(t *testing.T) {
	got := ShellQuote("/path with spaces")
	want := "'/path with spaces'"
	if got != want {
		t.Fatalf("ShellQuote: got %q want %q", got, want)
	}
}

func TestShellQuoteEmpty(t *testing.T) {
	got := ShellQuote("")
	want := "''"
	if got != want {
		t.Fatalf("ShellQuote: got %q want %q", got, want)
	}
}

func TestNativeBashUsesNonInteractiveCleanBash(t *testing.T) {
	n := Native{}
	args := n.bashArgs("echo ok")
	want := []string{"--noprofile", "--norc", "-c", "echo ok"}
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
	script := debugPackageBuildScript()
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
		"cpio -itv",
		"cpio --no-absolute-filenames --no-preserve-owner -idmv",
		"validate_archive_entries",
		"validate_archive_entry_types",
		"ensure_path_under",
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

func TestArchiveEntryIsSafe(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		{"./usr/lib/debug/boot/vmlinux-test", true},
		{"usr/lib/debug/boot/vmlinux-test", true},
		{"", false},
		{"/etc/passwd", false},
		{"../etc/passwd", false},
		{"usr/../../etc/passwd", false},
		{`C:\Windows\win.ini`, false},
		{`usr\lib\vmlinux`, false},
		{"usr/lib/\nmalicious", false},
	}
	for _, tt := range tests {
		got := archiveEntryIsSafe(tt.name)
		if got != tt.want {
			t.Fatalf("archiveEntryIsSafe(%q): got %v want %v", tt.name, got, tt.want)
		}
	}
}

func TestSafeArchiveShellFunctionRejectsUnsafeTypes(t *testing.T) {
	script := safeArchiveShellFunction()
	for _, fragment := range []string{
		"validate_archive_entry_types",
		`entry_type="${detail:0:1}"`,
		`-|d)`,
		`unsafe archive link entry`,
	} {
		if !strings.Contains(script, fragment) {
			t.Fatalf("expected safe archive function to contain %q", fragment)
		}
	}
}

func TestSafeArchiveShellFunctionExecutesValidation(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("safe archive shell validation requires Linux bash")
	}

	tmp := t.TempDir()
	unsafeEntries := writeTestFile(t, tmp, "entries.txt", "../evil\n")
	unsafeTypes := writeTestFile(t, tmp, "types.txt", "lrwxrwxrwx root/root 0 2026-01-01 00:00 ./usr/link -> /etc/passwd\n")
	unsafeLinks := writeTestFile(t, tmp, "links.txt", "-rw-r--r-- root/root 0 2026-01-01 00:00 ./usr/link -> /etc/passwd\n")
	safeTypes := writeTestFile(t, tmp, "safe-types.txt", "drwxr-xr-x root/root 0 2026-01-01 00:00 ./usr/\n-rw-r--r-- root/root 0 2026-01-01 00:00 ./usr/vmlinux\n")

	tests := []struct {
		name    string
		command string
		wantErr string
	}{
		{
			name:    "unsafe path",
			command: "validate_archive_entries " + ShellQuote(unsafeEntries),
			wantErr: "unsafe archive entry",
		},
		{
			name:    "unsafe type",
			command: "validate_archive_entry_types " + ShellQuote(unsafeTypes),
			wantErr: "unsafe archive entry type",
		},
		{
			name:    "unsafe link",
			command: "validate_archive_entry_types " + ShellQuote(unsafeLinks),
			wantErr: "unsafe archive link entry",
		},
		{
			name:    "safe types",
			command: "validate_archive_entry_types " + ShellQuote(safeTypes),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Native{}.Bash(context.Background(), safeArchiveShellFunction()+tt.command)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("expected validation to pass: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected validation error containing %q", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("error %q does not contain %q", err.Error(), tt.wantErr)
			}
		})
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

func TestParseExtractFileMarkerEdgeCases(t *testing.T) {
	current, total, file := parseExtractFileMarker("0/1:./file")
	if current != 0 || total != 1 {
		t.Fatalf("progress: got %d/%d", current, total)
	}
	if file != "./file" {
		t.Fatalf("file: got %q", file)
	}
}

func TestParseIntMarker(t *testing.T) {
	got := parseIntMarker("VOLSYM_EXTRACT_TOTAL=42", "VOLSYM_EXTRACT_TOTAL=")
	if got != 42 {
		t.Fatalf("parseIntMarker: got %d want 42", got)
	}
}

func TestParseIntMarkerInvalid(t *testing.T) {
	got := parseIntMarker("VOLSYM_EXTRACT_TOTAL=abc", "VOLSYM_EXTRACT_TOTAL=")
	if got != 0 {
		t.Fatalf("parseIntMarker: got %d want 0", got)
	}
}

func TestParseMarker(t *testing.T) {
	output := "some line\nVOLSYM_VMLINUX=/path/to/vmlinux\nother line"
	got := parseMarker(output, "VOLSYM_VMLINUX=")
	want := "/path/to/vmlinux"
	if got != want {
		t.Fatalf("parseMarker: got %q want %q", got, want)
	}
}

func TestParseMarkerNotFound(t *testing.T) {
	output := "some line\nother line"
	got := parseMarker(output, "VOLSYM_VMLINUX=")
	if got != "" {
		t.Fatalf("parseMarker: got %q want empty", got)
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

func TestPackageFormatFromPath(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{"kernel-debuginfo-4.18.0.x86_64.rpm", "rpm"},
		{"linux-image-5.10.0-35-amd64-dbg_5.10.237-1_amd64.deb", "deb"},
		{"linux-image-unsigned-5.4.0-163-generic-dbgsym_5.4.0-163.180_amd64.ddeb", "ddeb"},
		{"vmlinux-5.4.0-163-generic", "unknown"},
		{"", "unknown"},
	}
	for _, tt := range tests {
		got := packageFormatFromPath(tt.path)
		if got != tt.want {
			t.Fatalf("packageFormatFromPath(%q): got %q want %q", tt.path, got, tt.want)
		}
	}
}

func writeTestFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := dir + "/" + name
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestDoctorReturnsOSCheck(t *testing.T) {
	checks := Doctor(context.Background())
	if len(checks) == 0 {
		t.Fatal("expected at least one check")
	}
	if checks[0].Name != "OS" {
		t.Fatalf("first check name: got %q want OS", checks[0].Name)
	}
	if runtime.GOOS == "linux" && !checks[0].OK {
		t.Fatal("expected OS check to pass on Linux")
	}
	if runtime.GOOS != "linux" && checks[0].OK {
		t.Fatal("expected OS check to fail on non-Linux")
	}
}
