package runner

import (
	"context"
	"runtime"
	"strings"
	"testing"
)

func TestStreamOutputDisplayCollectsOutputAndCallsBack(t *testing.T) {
	name, args := streamTestCommand()
	var lines []string
	result, err := StreamOutputDisplay(context.Background(), "stream-test", name, func(line string) {
		lines = append(lines, line)
	}, args...)
	if err != nil {
		t.Fatalf("StreamOutputDisplay returned error: %v", err)
	}
	if result.Command != "stream-test" {
		t.Fatalf("command: got %q", result.Command)
	}
	if !strings.Contains(result.Output, "VOLSYM_STAGE=extract") || !strings.Contains(result.Output, "done") {
		t.Fatalf("expected output to contain both lines: %q", result.Output)
	}
	if len(lines) != 2 {
		t.Fatalf("callback line count: got %d want 2: %#v", len(lines), lines)
	}
}

func streamTestCommand() (string, []string) {
	if runtime.GOOS == "windows" {
		return "cmd", []string{"/c", "echo VOLSYM_STAGE=extract && echo done"}
	}
	return "sh", []string{"-c", "printf '%s\n%s\n' 'VOLSYM_STAGE=extract' 'done'"}
}
