package tui

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"sync"
	"time"
)

type StreamType string

const (
	StreamStdout StreamType = "stdout"
	StreamStderr StreamType = "stderr"
)

type CommandResult struct {
	Command  string
	Args     []string
	Code     int
	Stdout   string
	Stderr   string
	Duration time.Duration
}

type StreamCallback func(StreamType, string)

type Runner interface {
	Run(ctx context.Context, command string, args []string, onLine StreamCallback) (CommandResult, error)
}

type ExecRunner struct{}

func (ExecRunner) Run(ctx context.Context, command string, args []string, onLine StreamCallback) (CommandResult, error) {
	start := time.Now()
	cmd := exec.CommandContext(ctx, command, args...)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return CommandResult{}, fmt.Errorf("stdout pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return CommandResult{}, fmt.Errorf("stderr pipe: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return CommandResult{}, fmt.Errorf("start %s: %w", command, err)
	}

	var (
		stdoutBuf  bytes.Buffer
		stderrBuf  bytes.Buffer
		scanErrs   []error
		mu         sync.Mutex
		callbackMu sync.Mutex
		wg         sync.WaitGroup
	)

	scan := func(kind StreamType, r io.Reader, dst *bytes.Buffer) {
		defer wg.Done()
		scanner := bufio.NewScanner(r)
		scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
		for scanner.Scan() {
			line := scanner.Text()
			mu.Lock()
			dst.WriteString(line)
			dst.WriteByte('\n')
			mu.Unlock()
			if onLine != nil {
				callbackMu.Lock()
				onLine(kind, line)
				callbackMu.Unlock()
			}
		}
		if err := scanner.Err(); err != nil {
			mu.Lock()
			scanErrs = append(scanErrs, err)
			mu.Unlock()
		}
	}

	wg.Add(2)
	go scan(StreamStdout, stdout, &stdoutBuf)
	go scan(StreamStderr, stderr, &stderrBuf)

	waitErr := cmd.Wait()
	wg.Wait()

	mu.Lock()
	result := CommandResult{
		Command:  command,
		Args:     append([]string(nil), args...),
		Code:     exitCode(waitErr),
		Stdout:   stdoutBuf.String(),
		Stderr:   stderrBuf.String(),
		Duration: time.Since(start),
	}
	errs := append([]error(nil), scanErrs...)
	mu.Unlock()

	if waitErr != nil {
		if ctx.Err() != nil {
			return result, fmt.Errorf("command canceled: %w", ctx.Err())
		}
		return result, fmt.Errorf("%s exited with code %d", command, result.Code)
	}
	if len(errs) > 0 {
		return result, fmt.Errorf("read command output: %w", errs[0])
	}
	return result, nil
}

func exitCode(err error) int {
	if err == nil {
		return 0
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode()
	}
	return -1
}
