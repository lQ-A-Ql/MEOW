package runner

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"
)

type Result struct {
	Command string
	Output  string
}

func LookPath(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

func CombinedOutput(ctx context.Context, name string, args ...string) (*Result, error) {
	return CombinedOutputDisplay(ctx, formatCommand(name, args), name, args...)
}

func CombinedOutputDisplay(ctx context.Context, displayCommand, name string, args ...string) (*Result, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	err := cmd.Run()
	result := &Result{
		Command: displayCommand,
		Output:  buf.String(),
	}
	if err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return result, fmt.Errorf("%s timed out: %w", result.Command, ctx.Err())
		}
		return result, fmt.Errorf("%s failed: %w\n%s", result.Command, err, strings.TrimSpace(result.Output))
	}
	return result, nil
}

func StreamOutputDisplay(ctx context.Context, displayCommand, name string, onLine func(string), args ...string) (*Result, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return &Result{Command: displayCommand}, err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return &Result{Command: displayCommand}, err
	}

	var (
		buf        bytes.Buffer
		bufMu      sync.Mutex
		callbackMu sync.Mutex
		scanErrs   []error
		scanErrMu  sync.Mutex
	)

	appendLine := func(line string) {
		bufMu.Lock()
		buf.WriteString(line)
		buf.WriteByte('\n')
		bufMu.Unlock()

		if onLine != nil {
			callbackMu.Lock()
			onLine(line)
			callbackMu.Unlock()
		}
	}
	recordScanErr := func(err error) {
		if err == nil {
			return
		}
		scanErrMu.Lock()
		scanErrs = append(scanErrs, err)
		scanErrMu.Unlock()
	}
	scan := func(r io.Reader, wg *sync.WaitGroup) {
		defer wg.Done()
		scanner := bufio.NewScanner(r)
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		for scanner.Scan() {
			appendLine(scanner.Text())
		}
		recordScanErr(scanner.Err())
	}

	if err := cmd.Start(); err != nil {
		return &Result{Command: displayCommand}, err
	}

	var wg sync.WaitGroup
	wg.Add(2)
	go scan(stdout, &wg)
	go scan(stderr, &wg)

	waitErr := cmd.Wait()
	wg.Wait()

	result := &Result{Command: displayCommand}
	bufMu.Lock()
	result.Output = buf.String()
	bufMu.Unlock()

	if waitErr != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return result, fmt.Errorf("%s timed out: %w", result.Command, ctx.Err())
		}
		return result, fmt.Errorf("%s failed: %w\n%s", result.Command, waitErr, strings.TrimSpace(result.Output))
	}
	scanErrMu.Lock()
	defer scanErrMu.Unlock()
	if len(scanErrs) > 0 {
		return result, fmt.Errorf("%s output read failed: %w", result.Command, scanErrs[0])
	}
	return result, nil
}

func formatCommand(name string, args []string) string {
	parts := append([]string{name}, args...)
	return strings.Join(parts, " ")
}
