package tui

import (
	"fmt"
	"strings"
)

type actionKind string

const (
	actionNone       actionKind = ""
	actionDoctor     actionKind = "doctor"
	actionPreflight  actionKind = "preflight"
	actionBuild      actionKind = "build"
	actionVerify     actionKind = "verify"
	actionRun        actionKind = "run"
	actionWorkflow   actionKind = "workflow"
	actionCacheList  actionKind = "cache-list"
	actionCacheClear actionKind = "cache-clear"
)

type commandResult struct {
	model  Model
	logs   []logMsg
	action actionKind
}

func parseFields(input string) ([]string, error) {
	var (
		fields []string
		cur    strings.Builder
		quote  rune
	)
	for _, r := range strings.TrimSpace(input) {
		switch {
		case quote != 0:
			if r == quote {
				quote = 0
			} else {
				cur.WriteRune(r)
			}
		case r == '\'' || r == '"':
			quote = r
		case r == ' ' || r == '\t':
			if cur.Len() > 0 {
				fields = append(fields, cur.String())
				cur.Reset()
			}
		default:
			cur.WriteRune(r)
		}
	}
	if quote != 0 {
		return nil, fmt.Errorf("unclosed quote")
	}
	if cur.Len() > 0 {
		fields = append(fields, cur.String())
	}
	return fields, nil
}

func (m Model) dispatch(input string) (Model, []logMsg, actionKind) {
	result := m.executeCommand(input)
	return result.model, result.logs, result.action
}

func (m Model) executeCommand(input string) commandResult {
	fields, err := parseFields(input)
	if err != nil {
		return m.withLog(LogError, "command parse failed: "+err.Error())
	}
	if len(fields) == 0 {
		return commandResult{model: m}
	}
	if !strings.HasPrefix(fields[0], "/") {
		return m.withLog(LogError, "unknown command: "+fields[0])
	}
	cmd := strings.ToLower(strings.TrimPrefix(fields[0], "/"))
	args := fields[1:]

	switch cmd {
	case "help":
		return m.withLog(LogInfo, helpText())
	case "clear":
		m.logs.Clear()
		m.logScroll = 0
		return commandResult{model: m}
	case "mem":
		return m.setSourceArg(args, "memory image", InputModeMem, func(next *Model, v string) { next.memPath = v })
	case "banner-file":
		return m.setSourceArg(args, "banner file", InputModeBannerFile, func(next *Model, v string) { next.bannerFile = v })
	case "debug-package":
		return m.setSourceArg(args, "debug package", InputModeDebugPackage, func(next *Model, v string) { next.debugPackage = v })
	case "debug-package-url":
		return m.setSourceArg(args, "debug package URL", InputModeDebugPackageURL, func(next *Model, v string) { next.debugPackageURL = v })
	case "repo-url":
		return m.setSourceArg(args, "repo URL", InputModeRepoURL, func(next *Model, v string) { next.repoURL = v })
	case "vmlinux":
		return m.setSourceArg(args, "vmlinux", InputModeVMLINUX, func(next *Model, v string) { next.vmlinuxPath = v })
	case "symbol", "symbols":
		return m.setStringArg(args, "symbols path", func(next *Model, v string) { next.symbolsPath = v })
	case "mode", "source":
		return m.setInputMode(args)
	case "out":
		return m.setStringArg(args, "output dir", func(next *Model, v string) { next.outDir = v })
	case "cache-dir":
		return m.setStringArg(args, "cache dir", func(next *Model, v string) { next.cacheDir = v })
	case "symbol-sources":
		return m.setStringArg(args, "symbol sources", func(next *Model, v string) { next.symbolSourcesPath = v })
	case "vol":
		return m.setStringArg(args, "vol path", func(next *Model, v string) { next.volPath = v })
	case "meow":
		return m.setStringArg(args, "meow path", func(next *Model, v string) { next.meowPath = v })
	case "plugin":
		res := m.setStringArg(args, "plugin", func(next *Model, v string) { next.plugin = v })
		if len(args) > 0 && !knownPlugin(args[0]) {
			res.logs = append(res.logs, logMsg{level: LogWarn, message: "plugin is not in built-in list; execution is still allowed"})
		}
		return res
	case "plugin-args":
		m.pluginArgs = append([]string(nil), args...)
		return m.withLog(LogSuccess, fmt.Sprintf("plugin args set: %s", strings.Join(m.pluginArgs, " ")))
	case "manual":
		return m.setManual(args)
	case "distro":
		return m.setStringArg(args, "distro", func(next *Model, v string) { next.distro = v })
	case "kernel":
		return m.setStringArg(args, "kernel", func(next *Model, v string) { next.kernel = v })
	case "pkgver":
		return m.setStringArg(args, "package version", func(next *Model, v string) { next.packageVersion = v })
	case "arch":
		return m.setStringArg(args, "architecture", func(next *Model, v string) { next.arch = v })
	case "remote":
		return m.setBoolArg(args, "remote symbols", func(next *Model, enabled bool) { next.noRemoteSymbols = !enabled })
	case "no-remote-symbols":
		return m.setBoolArg(args, "no remote symbols", func(next *Model, enabled bool) { next.noRemoteSymbols = enabled })
	case "force":
		return m.setBoolArg(args, "force", func(next *Model, enabled bool) { next.force = enabled })
	case "unset":
		return m.unsetCommand(args)
	case "reset":
		return m.resetCommand(args)
	case "doctor":
		return commandResult{model: m, action: actionDoctor}
	case "preflight":
		return commandResult{model: m, action: actionPreflight}
	case "build":
		return commandResult{model: m, action: actionBuild}
	case "verify":
		if strings.TrimSpace(m.memPath) == "" {
			return m.withLog(LogError, "missing memory image: /mem <path>")
		}
		return commandResult{model: m, action: actionVerify}
	case "run":
		if strings.TrimSpace(m.memPath) == "" {
			return m.withLog(LogError, "missing memory image: /mem <path>")
		}
		if strings.TrimSpace(m.plugin) == "" {
			return m.withLog(LogError, "missing plugin: /plugin <name>")
		}
		return commandResult{model: m, action: actionRun}
	case "workflow":
		return commandResult{model: m, action: actionWorkflow}
	case "cache":
		return m.cacheCommand(args)
	default:
		return m.withLog(LogError, "unknown command: /"+cmd)
	}
}

func (m Model) setStringArg(args []string, label string, set func(*Model, string)) commandResult {
	if len(args) == 0 || strings.TrimSpace(args[0]) == "" {
		return m.withLog(LogInfo, "usage: /"+strings.ReplaceAll(label, " ", "-")+" <value>")
	}
	set(&m, args[0])
	return m.withLog(LogSuccess, label+" set: "+args[0])
}

func (m Model) setSourceArg(args []string, label string, mode InputMode, set func(*Model, string)) commandResult {
	res := m.setStringArg(args, label, set)
	if len(res.logs) > 0 && res.logs[0].level == LogSuccess {
		res.model.inputModeValue = mode
		res.logs[0].message += " (source: " + string(mode) + ")"
	}
	return res
}

func (m Model) setInputMode(args []string) commandResult {
	if len(args) == 0 || strings.TrimSpace(args[0]) == "" {
		return m.withLog(LogInfo, "usage: /mode mem|banner-file|debug-package|debug-package-url|repo-url|vmlinux|manual")
	}
	mode, ok := ParseInputMode(args[0])
	if !ok {
		return m.withLog(LogError, "unknown input mode: "+args[0])
	}
	m.inputModeValue = mode
	return m.withLog(LogSuccess, "source mode set: "+string(mode))
}

func (m Model) setManual(args []string) commandResult {
	for i := 0; i < len(args); i++ {
		if i+1 >= len(args) {
			return m.withLog(LogError, "manual option requires a value: "+args[i])
		}
		switch args[i] {
		case "--distro":
			i++
			m.distro = args[i]
		case "--kernel":
			i++
			m.kernel = args[i]
		case "--pkgver":
			i++
			m.packageVersion = args[i]
		case "--arch":
			i++
			m.arch = args[i]
		default:
			return m.withLog(LogError, "unknown manual option: "+args[i])
		}
	}
	if m.arch == "" {
		m.arch = "amd64"
	}
	m.inputModeValue = InputModeManual
	return m.withLog(LogSuccess, "manual input set")
}

func (m Model) unsetCommand(args []string) commandResult {
	if len(args) == 0 {
		return m.withLog(LogInfo, "usage: /unset <field>")
	}
	field := strings.ToLower(strings.TrimSpace(args[0]))
	previousMode := m.inputMode()
	var sourceMode InputMode
	switch field {
	case "mem", "memory":
		m.memPath = ""
		sourceMode = InputModeMem
	case "banner", "banner-file":
		m.bannerFile = ""
		sourceMode = InputModeBannerFile
	case "debug-package", "debug-pkg", "package":
		m.debugPackage = ""
		sourceMode = InputModeDebugPackage
	case "debug-package-url", "debug-url", "package-url":
		m.debugPackageURL = ""
		sourceMode = InputModeDebugPackageURL
	case "repo", "repo-url":
		m.repoURL = ""
		sourceMode = InputModeRepoURL
	case "vmlinux":
		m.vmlinuxPath = ""
		sourceMode = InputModeVMLINUX
	case "distro":
		m.distro = ""
	case "kernel":
		m.kernel = ""
	case "pkgver", "package-version":
		m.packageVersion = ""
	case "arch":
		m.arch = "amd64"
	case "symbols", "symbol":
		m.symbolsPath = ""
	case "out":
		m.outDir = ""
	case "cache-dir", "cache":
		m.cacheDir = ""
	case "symbol-sources":
		m.symbolSourcesPath = ""
	case "plugin":
		m.plugin = ""
	case "plugin-args":
		m.pluginArgs = nil
	case "source", "mode":
		m.inputModeValue = m.detectInputMode()
	default:
		return m.withLog(LogError, "unknown unset field: "+field)
	}
	if sourceMode != "" && previousMode == sourceMode {
		m.inputModeValue = m.detectInputMode()
	}
	return m.withLog(LogSuccess, "unset: "+field)
}

func (m Model) resetCommand(args []string) commandResult {
	target := "inputs"
	if len(args) > 0 && strings.TrimSpace(args[0]) != "" {
		target = strings.ToLower(strings.TrimSpace(args[0]))
	}
	switch target {
	case "inputs", "input", "source", "sources":
		m.clearInputs()
		m.inputModeValue = InputModeManual
		return m.withLog(LogSuccess, "inputs reset")
	case "all":
		m.clearInputs()
		defaults := Options{}.WithDefaults()
		m.symbolsPath = defaults.SymbolsPath
		m.outDir = defaults.OutDir
		m.cacheDir = ""
		m.symbolSourcesPath = ""
		m.plugin = defaults.Plugin
		m.pluginArgs = nil
		m.noRemoteSymbols = false
		m.force = false
		m.cacheClearForce = false
		m.lastDoctorResult = nil
		m.lastPreflightResult = nil
		m.lastBuildResult = nil
		m.lastVerifyResult = nil
		m.lastCacheResult = nil
		m.lastPluginResult = nil
		m.inputModeValue = InputModeManual
		return m.withLog(LogSuccess, "all editable state reset")
	default:
		return m.withLog(LogError, "usage: /reset inputs|all")
	}
}

func (m *Model) clearInputs() {
	m.memPath = ""
	m.bannerFile = ""
	m.debugPackage = ""
	m.debugPackageURL = ""
	m.repoURL = ""
	m.vmlinuxPath = ""
	m.distro = ""
	m.kernel = ""
	m.packageVersion = ""
	m.arch = "amd64"
}

func (m Model) detectInputMode() InputMode {
	return Options{
		MemPath:         m.memPath,
		BannerFile:      m.bannerFile,
		DebugPackage:    m.debugPackage,
		DebugPackageURL: m.debugPackageURL,
		RepoURL:         m.repoURL,
		VMLinuxPath:     m.vmlinuxPath,
		Kernel:          m.kernel,
		PackageVersion:  m.packageVersion,
	}.detectInputMode()
}

func (m Model) setBoolArg(args []string, label string, set func(*Model, bool)) commandResult {
	enabled := true
	if len(args) > 0 {
		switch strings.ToLower(args[0]) {
		case "on", "true", "1", "yes", "enable", "enabled":
			enabled = true
		case "off", "false", "0", "no", "disable", "disabled":
			enabled = false
		default:
			return m.withLog(LogError, "expected on/off for "+label)
		}
	}
	set(&m, enabled)
	return m.withLog(LogSuccess, fmt.Sprintf("%s set: %t", label, enabled))
}

func (m Model) cacheCommand(args []string) commandResult {
	if len(args) == 0 || args[0] == "list" {
		return commandResult{model: m, action: actionCacheList}
	}
	if args[0] != "clear" {
		return m.withLog(LogError, "usage: /cache list | /cache clear --confirm [--force]")
	}
	confirm := false
	force := false
	for _, arg := range args[1:] {
		switch arg {
		case "--confirm":
			confirm = true
		case "--force":
			force = true
		default:
			return m.withLog(LogError, "unknown cache clear option: "+arg)
		}
	}
	if !confirm {
		return m.withLog(LogWarn, "cache clear requires explicit confirmation: /cache clear --confirm")
	}
	m.cacheClearForce = force
	return commandResult{model: m, action: actionCacheClear}
}

func (m Model) withLog(level LogLevel, message string) commandResult {
	return commandResult{model: m, logs: []logMsg{{level: level, message: message}}}
}

func helpText() string {
	return strings.Join([]string{
		"commands:",
		"  /mode <source> | /source <source>",
		"  /mem <path> | /banner-file <path> | /debug-package <path>",
		"  /debug-package-url <url> | /repo-url <url> | /vmlinux <path>",
		"  /manual --distro <name> --kernel <release> --pkgver <version> [--arch <arch>]",
		"  /distro <name> | /kernel <release> | /pkgver <version> | /arch <arch>",
		"  /symbol[s] <path> | /out <dir> | /cache-dir <dir> | /symbol-sources <path>",
		"  /vol <path> | /meow <path> | /plugin <name> | /plugin-args [args...]",
		"  /remote on|off | /no-remote-symbols on|off | /force on|off",
		"  /doctor | /preflight | /build | /verify | /run | /workflow",
		"  /cache list | /cache clear --confirm [--force]",
		"  /unset <field> | /reset inputs|all | /clear | /help",
		"keys: i focus input | r run plugin | x cancel | q quit",
	}, "\n")
}

func knownPlugin(name string) bool {
	for _, plugin := range AllPlugins() {
		if plugin.Name == name {
			return true
		}
	}
	return false
}
