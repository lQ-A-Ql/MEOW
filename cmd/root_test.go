package cmd

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
)

func TestRootUsageListsAllGlobalFlags(t *testing.T) {
	output := captureStderr(t, func() {
		printUsage()
	})
	for _, want := range []string{
		"--verbose",
		"--json",
		"-h, --help",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("usage missing %q:\n%s", want, output)
		}
	}
}

func TestParseGlobalFlagsAcceptsHelpAliases(t *testing.T) {
	for _, flag := range []string{"-h", "--help", "-help"} {
		_, _, ok := parseGlobalFlags([]string{flag})
		if ok {
			t.Fatalf("%s should request usage", flag)
		}
	}
}

func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stderr
	read, write, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stderr = write
	fn()
	_ = write.Close()
	os.Stderr = old
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, read)
	_ = read.Close()
	return buf.String()
}
