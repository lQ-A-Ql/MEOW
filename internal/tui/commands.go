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
		return commandResult{model: m}
	case "mem":
		return m.setStringArg(args, "memory image", func(next *Model, v string) { next.memPath = v })
	case "banner-file":
		return m.setStringArg(args, "banner file", func(next *Model, v string) { next.bannerFile = v })
	case "debug-package":
		return m.setStringArg(args, "debug package", func(next *Model, v string) { next.debugPackage = v })
	case "debug-package-url":
		return m.setStringArg(args, "debug package URL", func(next *Model, v string) { next.debugPackageURL = v })
	case "repo-url":
		return m.setStringArg(args, "repo URL", func(next *Model, v string) { next.repoURL = v })
	case "vmlinux":
		return m.setStringArg(args, "vmlinux", func(next *Model, v string) { next.vmlinuxPath = v })
	case "symbol":
		return m.setStringArg(args, "symbols path", func(next *Model, v string) { next.symbolsPath = v })
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
	case "remote":
		return m.setBoolArg(args, "remote symbols", func(next *Model, enabled bool) { next.noRemoteSymbols = !enabled })
	case "force":
		return m.setBoolArg(args, "force", func(next *Model, enabled bool) { next.force = enabled })
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
	return m.withLog(LogSuccess, "manual input set")
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
		"  /mem <path> | /banner-file <path> | /debug-package <path>",
		"  /debug-package-url <url> | /repo-url <url> | /vmlinux <path>",
		"  /manual --distro <name> --kernel <release> --pkgver <version> [--arch <arch>]",
		"  /symbol <path> | /out <dir> | /cache-dir <dir> | /symbol-sources <path>",
		"  /vol <path> | /meow <path> | /plugin <name> | /plugin-args [args...]",
		"  /remote on|off | /force on|off",
		"  /doctor | /preflight | /build | /verify | /run | /workflow",
		"  /cache list | /cache clear --confirm [--force]",
		"  /clear | /help",
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
