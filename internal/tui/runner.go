package tui

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"
)

// runStream executes a command and returns combined output.
// It streams stdout/stderr line by line via the returned channel.
func runStream(ctx context.Context, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", fmt.Errorf("stdout pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return "", fmt.Errorf("stderr pipe: %w", err)
	}

	var (
		buf  bytes.Buffer
		mu   sync.Mutex
		errs []error
	)

	appendLine := func(line string) {
		mu.Lock()
		buf.WriteString(line)
		buf.WriteByte('\n')
		mu.Unlock()
	}

	scan := func(r io.Reader, wg *sync.WaitGroup) {
		defer wg.Done()
		scanner := bufio.NewScanner(r)
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		for scanner.Scan() {
			appendLine(scanner.Text())
		}
		if err := scanner.Err(); err != nil {
			mu.Lock()
			errs = append(errs, err)
			mu.Unlock()
		}
	}

	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("start: %w", err)
	}

	var wg sync.WaitGroup
	wg.Add(2)
	go scan(stdout, &wg)
	go scan(stderr, &wg)

	waitErr := cmd.Wait()
	wg.Wait()

	mu.Lock()
	output := buf.String()
	scanErrs := errs
	mu.Unlock()

	if waitErr != nil {
		if ctx.Err() != nil {
			return output, fmt.Errorf("已取消")
		}
		return output, fmt.Errorf("%s: %w\n%s", name, waitErr, strings.TrimSpace(output))
	}
	if len(scanErrs) > 0 {
		return output, fmt.Errorf("读取输出失败: %w", scanErrs[0])
	}
	return output, nil
}
