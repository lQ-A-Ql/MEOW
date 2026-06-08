package tui

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

type runnerCall struct {
	command string
	args    []string
}

type fakeRunner struct {
	calls []runnerCall
	reply func(command string, args []string, onLine StreamCallback) (CommandResult, error)
}

func (r *fakeRunner) Run(ctx context.Context, command string, args []string, onLine StreamCallback) (CommandResult, error) {
	r.calls = append(r.calls, runnerCall{command: command, args: append([]string(nil), args...)})
	if r.reply != nil {
		return r.reply(command, args, onLine)
	}
	return CommandResult{Command: command, Args: args, Code: 0, Stdout: "{}\n"}, nil
}

func TestWorkflowRunsDoctorPreflightBuildAndSkipsVerifyWithoutMem(t *testing.T) {
	runner := &fakeRunner{reply: func(command string, args []string, onLine StreamCallback) (CommandResult, error) {
		onLine(StreamStdout, "stream:"+strings.Join(args, " "))
		switch {
		case reflect.DeepEqual(args, []string{"--json", "doctor"}):
			return CommandResult{Command: command, Args: args, Stdout: `[{"name":"OS","ok":true}]`}, nil
		case containsArg(args, "--dry-run"):
			return CommandResult{Command: command, Args: args, Stdout: `{"success":true,"dry_run":true,"symbol_path":"symbols/linux/dry.json.xz"}`}, nil
		default:
			return CommandResult{Command: command, Args: args, Stdout: `{"success":true,"symbol_path":"symbols/linux/live.json.xz"}`}, nil
		}
	}}
	m := NewModelWithOptions(Options{Runner: runner, BannerFile: "banner.txt", MeowPath: "meow", VolPath: "vol"})
	var streamed []string
	result := m.runAction(context.Background(), actionWorkflow, func(kind StreamType, line string) {
		streamed = append(streamed, string(kind)+":"+line)
	})

	if result.err != nil {
		t.Fatalf("workflow: %v", result.err)
	}
	if len(runner.calls) != 3 {
		t.Fatalf("expected doctor/preflight/build calls, got %#v", runner.calls)
	}
	if result.model.lastDoctorResult == nil || result.model.lastPreflightResult == nil || result.model.lastBuildResult == nil {
		t.Fatalf("expected workflow results to be recorded: %#v", result.model)
	}
	if result.model.symbolsPath != "symbols" {
		t.Fatalf("symbols path inferred from build result: %q", result.model.symbolsPath)
	}
	if len(streamed) != 3 {
		t.Fatalf("expected stream callbacks, got %#v", streamed)
	}
	if !eventsContain(result.events, "verify skipped") {
		t.Fatalf("expected verify skip warning, got %#v", result.events)
	}
}

func TestVerifyAndRunArgs(t *testing.T) {
	runner := &fakeRunner{reply: func(command string, args []string, onLine StreamCallback) (CommandResult, error) {
		if command == "meow" {
			return CommandResult{Command: command, Args: args, Stdout: `{"success":true}`}, nil
		}
		return CommandResult{Command: command, Args: args, Stdout: "plugin output\n"}, nil
	}}
	m := NewModelWithOptions(Options{
		Runner:      runner,
		MeowPath:    "meow",
		VolPath:     "vol",
		MemPath:     "mem.raw",
		SymbolsPath: "symbols",
		Plugin:      "linux.pslist.PsList",
		PluginArgs:  []string{"--pid", "1"},
	})

	if result := m.runAction(context.Background(), actionVerify, nil); result.err != nil {
		t.Fatalf("verify: %v", result.err)
	}
	if result := m.runAction(context.Background(), actionRun, nil); result.err != nil {
		t.Fatalf("run: %v", result.err)
	}

	wantVerify := []string{"--json", "verify", "--mem", "mem.raw", "--symbols", "symbols", "--vol", "vol"}
	if !reflect.DeepEqual(runner.calls[0].args, wantVerify) {
		t.Fatalf("verify args: got %#v want %#v", runner.calls[0].args, wantVerify)
	}
	wantRun := []string{"-f", "mem.raw", "-s", "symbols", "linux.pslist.PsList", "--pid", "1"}
	if !reflect.DeepEqual(runner.calls[1].args, wantRun) {
		t.Fatalf("run args: got %#v want %#v", runner.calls[1].args, wantRun)
	}
}

func TestViewResponsiveWidths(t *testing.T) {
	for _, width := range []int{120, 80, 60} {
		t.Run(fmt.Sprint(width), func(t *testing.T) {
			m := NewModelWithOptions(Options{MemPath: "mem.raw"})
			model, _ := m.Update(tea.WindowSizeMsg{Width: width, Height: 24})
			view := model.(Model).View()
			if !strings.Contains(view, "MEOW") || !strings.Contains(view, "Workflow") {
				t.Fatalf("view missing expected content at width %d:\n%s", width, view)
			}
		})
	}
}

func TestExecRunnerStreamsAndReturnsNonZero(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping process test in short mode")
	}
	command, args := testCommand()
	var lines []string
	result, err := ExecRunner{}.Run(context.Background(), command, args, func(kind StreamType, line string) {
		lines = append(lines, string(kind)+":"+line)
	})
	if err == nil {
		t.Fatal("expected non-zero command error")
	}
	if result.Code == 0 {
		t.Fatalf("expected non-zero exit code: %#v", result)
	}
	if len(lines) < 2 {
		t.Fatalf("expected stdout/stderr callbacks, got %#v", lines)
	}
}

func TestExecRunnerCancel(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping process test in short mode")
	}
	command, args := sleepCommand()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	_, err := ExecRunner{}.Run(ctx, command, args, nil)
	if err == nil || !strings.Contains(err.Error(), "command canceled") {
		t.Fatalf("expected cancel error, got %v", err)
	}
}

func containsArg(args []string, value string) bool {
	for _, arg := range args {
		if arg == value {
			return true
		}
	}
	return false
}

func eventsContain(events []workflowEvent, text string) bool {
	for _, event := range events {
		if strings.Contains(event.message, text) {
			return true
		}
	}
	return false
}
