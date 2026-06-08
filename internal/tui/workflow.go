package tui

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
)

type workflowEvent struct {
	level   LogLevel
	message string
}

type workflowResult struct {
	model  Model
	events []workflowEvent
	err    error
}

func (m Model) runAction(ctx context.Context, action actionKind, onLine StreamCallback) workflowResult {
	switch action {
	case actionDoctor:
		return m.runDoctor(ctx, onLine)
	case actionPreflight:
		return m.runBuild(ctx, true, onLine)
	case actionBuild:
		return m.runBuild(ctx, false, onLine)
	case actionVerify:
		return m.runVerify(ctx, onLine)
	case actionRun:
		return m.runPlugin(ctx, onLine)
	case actionWorkflow:
		return m.runWorkflow(ctx, onLine)
	case actionCacheList:
		return m.runCacheList(ctx, onLine)
	case actionCacheClear:
		return m.runCacheClear(ctx, onLine)
	default:
		return workflowResult{model: m}
	}
}

func (m Model) runWorkflow(ctx context.Context, onLine StreamCallback) workflowResult {
	events := []workflowEvent{}
	current := m
	for _, step := range []actionKind{actionDoctor, actionPreflight, actionBuild} {
		events = append(events, workflowEvent{level: LogInfo, message: "[RUN] " + string(step)})
		result := current.runAction(ctx, step, onLine)
		current = result.model
		events = append(events, result.events...)
		if result.err != nil {
			return workflowResult{model: current, events: events, err: result.err}
		}
	}
	if strings.TrimSpace(current.memPath) == "" {
		events = append(events, workflowEvent{level: LogWarn, message: "[WARN] verify skipped: memory image is not set"})
		return workflowResult{model: current, events: events}
	}
	result := current.runVerify(ctx, onLine)
	current = result.model
	events = append(events, result.events...)
	return workflowResult{model: current, events: events, err: result.err}
}

func (m Model) runDoctor(ctx context.Context, onLine StreamCallback) workflowResult {
	result, err := m.runner.Run(ctx, m.meowPath, []string{"--json", "doctor"}, onLine)
	if err != nil {
		return workflowResult{model: m, err: commandError(err, result)}
	}
	var checks []DoctorCheck
	if err := decodeJSON(result.Stdout, &checks); err != nil {
		return workflowResult{model: m, err: err}
	}
	m.lastDoctorResult = checks
	return workflowResult{model: m, events: []workflowEvent{{level: LogSuccess, message: "[OK] doctor checks: " + fmt.Sprint(len(checks))}}}
}

func (m Model) runBuild(ctx context.Context, dryRun bool, onLine StreamCallback) workflowResult {
	args := m.buildArgs(dryRun)
	result, err := m.runner.Run(ctx, m.meowPath, args, onLine)
	var summary BuildSummary
	if result.Stdout != "" {
		_ = json.Unmarshal([]byte(result.Stdout), &summary)
	}
	if err != nil {
		if summary.Error != "" {
			return workflowResult{model: m, err: fmt.Errorf("%s", summary.Error)}
		}
		return workflowResult{model: m, err: commandError(err, result)}
	}
	if err := decodeJSON(result.Stdout, &summary); err != nil {
		return workflowResult{model: m, err: err}
	}
	if dryRun {
		m.lastPreflightResult = &summary
		return workflowResult{model: m, events: []workflowEvent{{level: LogSuccess, message: "[OK] preflight complete"}}}
	}
	m.lastBuildResult = &summary
	if summary.SymbolPath != "" {
		m.symbolsPath = inferSymbolsPath(summary.SymbolPath)
	}
	return workflowResult{model: m, events: []workflowEvent{{level: LogSuccess, message: "[OK] build complete: " + emptyDash(summary.SymbolPath)}}}
}

func (m Model) runVerify(ctx context.Context, onLine StreamCallback) workflowResult {
	if strings.TrimSpace(m.memPath) == "" {
		return workflowResult{model: m, err: fmt.Errorf("missing memory image")}
	}
	args := []string{"--json", "verify", "--mem", m.memPath, "--symbols", m.symbolsPath, "--vol", m.volPath}
	result, err := m.runner.Run(ctx, m.meowPath, args, onLine)
	var summary VerifySummary
	if result.Stdout != "" {
		_ = json.Unmarshal([]byte(result.Stdout), &summary)
	}
	if err != nil {
		if summary.Error != "" {
			return workflowResult{model: m, err: fmt.Errorf("%s", summary.Error)}
		}
		return workflowResult{model: m, err: commandError(err, result)}
	}
	if err := decodeJSON(result.Stdout, &summary); err != nil {
		return workflowResult{model: m, err: err}
	}
	m.lastVerifyResult = &summary
	if !summary.Success {
		return workflowResult{model: m, err: fmt.Errorf("%s", emptyDash(summary.Error))}
	}
	return workflowResult{model: m, events: []workflowEvent{{level: LogSuccess, message: "[OK] verify complete"}}}
}

func (m Model) runPlugin(ctx context.Context, onLine StreamCallback) workflowResult {
	args := []string{"-f", m.memPath}
	if strings.TrimSpace(m.symbolsPath) != "" {
		args = append(args, "-s", m.symbolsPath)
	}
	args = append(args, m.plugin)
	args = append(args, m.pluginArgs...)
	result, err := m.runner.Run(ctx, m.volPath, args, onLine)
	m.lastPluginResult = &result
	if err != nil {
		return workflowResult{model: m, err: commandError(err, result)}
	}
	return workflowResult{model: m, events: []workflowEvent{{level: LogSuccess, message: "[OK] plugin complete"}}}
}

func (m Model) runCacheList(ctx context.Context, onLine StreamCallback) workflowResult {
	args := []string{"--json", "cache", "list"}
	if strings.TrimSpace(m.cacheDir) != "" {
		args = append(args, "--cache-dir", m.cacheDir)
	}
	result, err := m.runner.Run(ctx, m.meowPath, args, onLine)
	if err != nil {
		return workflowResult{model: m, err: commandError(err, result)}
	}
	var entries []CacheEntry
	if err := decodeJSON(result.Stdout, &entries); err != nil {
		return workflowResult{model: m, err: err}
	}
	m.lastCacheResult = entries
	return workflowResult{model: m, events: []workflowEvent{{level: LogSuccess, message: "[OK] cache entries: " + fmt.Sprint(len(entries))}}}
}

func (m Model) runCacheClear(ctx context.Context, onLine StreamCallback) workflowResult {
	args := []string{"--json", "cache", "clear"}
	if strings.TrimSpace(m.cacheDir) != "" {
		args = append(args, "--cache-dir", m.cacheDir)
	}
	if m.cacheClearForce {
		args = append(args, "--force")
	}
	result, err := m.runner.Run(ctx, m.meowPath, args, onLine)
	if err != nil {
		return workflowResult{model: m, err: commandError(err, result)}
	}
	m.lastCacheResult = nil
	return workflowResult{model: m, events: []workflowEvent{{level: LogSuccess, message: "[OK] cache cleared"}}}
}

func (m Model) buildArgs(dryRun bool) []string {
	args := []string{"--json", "build"}
	if dryRun {
		args = append(args, "--dry-run")
	}
	add := func(flag, value string) {
		if strings.TrimSpace(value) != "" {
			args = append(args, flag, value)
		}
	}
	switch m.inputMode() {
	case InputModeMem:
		add("--mem", m.memPath)
	case InputModeBannerFile:
		add("--banner-file", m.bannerFile)
	case InputModeDebugPackage:
		add("--debug-package", m.debugPackage)
	case InputModeDebugPackageURL:
		add("--debug-package-url", m.debugPackageURL)
	case InputModeRepoURL:
		add("--repo-url", m.repoURL)
		add("--kernel", m.kernel)
		add("--pkgver", m.packageVersion)
		add("--distro", m.distro)
	case InputModeVMLINUX:
		add("--vmlinux", m.vmlinuxPath)
	case InputModeManual:
		add("--kernel", m.kernel)
		add("--pkgver", m.packageVersion)
		add("--distro", m.distro)
	}
	add("--arch", m.arch)
	add("--out", m.outDir)
	add("--cache-dir", m.cacheDir)
	add("--symbol-sources", m.symbolSourcesPath)
	add("--vol", m.volPath)
	if m.noRemoteSymbols {
		args = append(args, "--no-remote-symbols")
	}
	if m.force {
		args = append(args, "--force")
	}
	return args
}

func decodeJSON(content string, target any) error {
	if strings.TrimSpace(content) == "" {
		return fmt.Errorf("empty JSON output")
	}
	if err := json.Unmarshal([]byte(content), target); err != nil {
		return fmt.Errorf("parse JSON output: %w", err)
	}
	return nil
}

func commandError(err error, result CommandResult) error {
	if result.Stderr != "" {
		return fmt.Errorf("%w: %s", err, strings.TrimSpace(result.Stderr))
	}
	if result.Stdout != "" {
		return fmt.Errorf("%w: %s", err, strings.TrimSpace(result.Stdout))
	}
	return err
}

func inferSymbolsPath(symbolPath string) string {
	clean := filepath.Clean(symbolPath)
	dir := filepath.Dir(clean)
	if filepath.Base(dir) == "linux" {
		return filepath.Dir(dir)
	}
	return dir
}

func emptyDash(value string) string {
	if strings.TrimSpace(value) == "" {
		return "-"
	}
	return value
}
