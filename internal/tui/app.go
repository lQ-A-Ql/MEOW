package tui

import (
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type logMsg struct {
	level   LogLevel
	message string
}

type streamMsg struct {
	level   LogLevel
	message string
}

type actionDoneMsg struct {
	action actionKind
	result workflowResult
}

type Model struct {
	meowPath          string
	volPath           string
	memPath           string
	symbolsPath       string
	outDir            string
	cacheDir          string
	symbolSourcesPath string
	bannerFile        string
	debugPackage      string
	debugPackageURL   string
	repoURL           string
	vmlinuxPath       string
	distro            string
	kernel            string
	packageVersion    string
	arch              string
	plugin            string
	pluginArgs        []string
	noRemoteSymbols   bool
	force             bool
	cacheClearForce   bool

	width, height int
	input         textinput.Model
	logs          *LogStore
	inputFocused  bool
	running       bool
	runningAction actionKind
	cancelFunc    context.CancelFunc
	eventCh       <-chan tea.Msg
	runner        Runner

	lastDoctorResult    []DoctorCheck
	lastPreflightResult *BuildSummary
	lastBuildResult     *BuildSummary
	lastVerifyResult    *VerifySummary
	lastCacheResult     []CacheEntry
	lastPluginResult    *CommandResult
}

func NewModel() Model {
	return NewModelWithOptions(Options{})
}

func NewModelWithOptions(options Options) Model {
	options = options.WithDefaults()
	ti := textinput.New()
	ti.Placeholder = "enter command... (/help)"
	ti.CharLimit = 1024
	ti.Width = 60

	return Model{
		meowPath:          options.MeowPath,
		volPath:           options.VolPath,
		memPath:           options.MemPath,
		symbolsPath:       options.SymbolsPath,
		outDir:            options.OutDir,
		cacheDir:          options.CacheDir,
		symbolSourcesPath: options.SymbolSourcesPath,
		bannerFile:        options.BannerFile,
		debugPackage:      options.DebugPackage,
		debugPackageURL:   options.DebugPackageURL,
		repoURL:           options.RepoURL,
		vmlinuxPath:       options.VMLinuxPath,
		distro:            options.Distro,
		kernel:            options.Kernel,
		packageVersion:    options.PackageVersion,
		arch:              options.Arch,
		plugin:            options.Plugin,
		pluginArgs:        append([]string(nil), options.PluginArgs...),
		noRemoteSymbols:   options.NoRemoteSymbols,
		force:             options.Force,
		input:             ti,
		logs:              NewLogStore(),
		runner:            options.Runner,
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(tea.WindowSize(), textinput.Blink)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.input.Width = max(20, msg.Width-12)
		return m, nil
	case tea.KeyMsg:
		return m.onKey(msg)
	case logMsg:
		m.logs.Append(msg.level, msg.message)
		return m, nil
	case streamMsg:
		m.logs.Append(msg.level, msg.message)
		return m, waitActionEvent(m.eventCh)
	case actionDoneMsg:
		next := msg.result.model
		next.width = m.width
		next.height = m.height
		next.input = m.input
		next.inputFocused = m.inputFocused
		next.logs = m.logs
		next.runner = m.runner
		next.running = false
		next.runningAction = actionNone
		next.cancelFunc = nil
		next.eventCh = nil
		m = next
		for _, event := range msg.result.events {
			m.logs.Append(event.level, event.message)
		}
		if msg.result.err != nil {
			m.logs.Append(LogError, "[ERR] "+msg.result.err.Error())
		} else {
			m.logs.Append(LogSuccess, "[OK] "+string(msg.action)+" complete")
		}
		return m, nil
	}
	if m.inputFocused {
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m Model) onKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.inputFocused {
		switch msg.String() {
		case "esc":
			m.inputFocused = false
			m.input.Blur()
			m.input.SetValue("")
			return m, nil
		case "enter":
			value := m.input.Value()
			m.input.SetValue("")
			m.inputFocused = false
			m.input.Blur()
			return m.handleInput(value)
		}
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		return m, cmd
	}

	switch msg.String() {
	case "ctrl+c", "q":
		if m.cancelFunc != nil {
			m.cancelFunc()
		}
		return m, tea.Quit
	case "i", ":":
		m.inputFocused = true
		return m, m.input.Focus()
	case "r":
		return m.startAction(actionRun)
	case "x":
		if m.running && m.cancelFunc != nil {
			m.cancelFunc()
			m.logs.Append(LogWarn, "[WARN] canceling "+string(m.runningAction))
		}
		return m, nil
	}
	return m, nil
}

func (m Model) handleInput(input string) (tea.Model, tea.Cmd) {
	next, logs, action := m.dispatch(input)
	for _, entry := range logs {
		next.logs.Append(entry.level, entry.message)
	}
	if action != actionNone {
		return next.startAction(action)
	}
	return next, nil
}

func (m Model) startAction(action actionKind) (tea.Model, tea.Cmd) {
	if m.running {
		m.logs.Append(LogWarn, "[WARN] task running; press x to cancel")
		return m, nil
	}
	if action == actionVerify || action == actionRun {
		if strings.TrimSpace(m.memPath) == "" {
			m.logs.Append(LogError, "[ERR] missing memory image: /mem <path>")
			return m, nil
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	eventCh := make(chan tea.Msg, 64)
	m.running = true
	m.runningAction = action
	m.cancelFunc = cancel
	m.eventCh = eventCh
	m.logs.Append(LogInfo, "[RUN] "+string(action))
	actionModel := m
	return m, func() tea.Msg {
		go func() {
			result := actionModel.runAction(ctx, action, func(kind StreamType, line string) {
				level := LogStdout
				if kind == StreamStderr {
					level = LogStderr
				}
				eventCh <- streamMsg{level: level, message: line}
			})
			eventCh <- actionDoneMsg{action: action, result: result}
			close(eventCh)
		}()
		return waitActionEvent(eventCh)()
	}
}

func waitActionEvent(ch <-chan tea.Msg) tea.Cmd {
	return func() tea.Msg {
		if ch == nil {
			return nil
		}
		msg, ok := <-ch
		if !ok {
			return nil
		}
		return msg
	}
}

func (m Model) inputMode() InputMode {
	return Options{
		MemPath:         m.memPath,
		BannerFile:      m.bannerFile,
		DebugPackage:    m.debugPackage,
		DebugPackageURL: m.debugPackageURL,
		RepoURL:         m.repoURL,
		VMLinuxPath:     m.vmlinuxPath,
		Kernel:          m.kernel,
		PackageVersion:  m.packageVersion,
	}.InputMode()
}

func (m Model) View() string {
	if m.width == 0 {
		return "initializing..."
	}
	logo := renderCompactLogo(m.width)
	bar := m.commandBar()
	bodyHeight := max(3, m.height-lipgloss.Height(logo)-lipgloss.Height(bar))
	body := m.renderBody(bodyHeight)
	return lipgloss.JoinVertical(lipgloss.Left, logo, body, bar)
}

func renderCompactLogo(width int) string {
	title := "MEOW - Linux Symbol Builder"
	if width < 70 {
		return logoStyle.Render(title)
	}
	return logoStyle.Render("MEOW") + mutedStyle.Render("  Linux Symbol Builder")
}

func (m Model) renderBody(height int) string {
	switch {
	case m.width >= 100:
		leftW := 31
		rightW := 34
		centerW := max(20, m.width-leftW-rightW-2)
		return lipgloss.JoinHorizontal(
			lipgloss.Top,
			m.leftPanel(leftW, height),
			m.centerPanel(centerW, height),
			m.rightPanel(rightW, height),
		)
	case m.width >= 70:
		leftW := 30
		centerW := max(20, m.width-leftW-1)
		return lipgloss.JoinHorizontal(lipgloss.Top, m.leftPanel(leftW, height), m.centerPanel(centerW, height))
	default:
		return m.centerPanel(max(20, m.width), height)
	}
}

func (m Model) leftPanel(width, height int) string {
	rows := []string{
		titleStyle.Render("Input"),
		labelStyle.Render("mode: ") + valueStyle.Render(string(m.inputMode())),
		labelStyle.Render("mem: ") + valueStyle.Render(shortValue(m.memPath)),
		labelStyle.Render("banner: ") + valueStyle.Render(shortValue(m.bannerFile)),
		labelStyle.Render("debug pkg: ") + valueStyle.Render(shortValue(firstNonEmpty(m.debugPackage, m.debugPackageURL))),
		labelStyle.Render("repo: ") + valueStyle.Render(shortValue(m.repoURL)),
		labelStyle.Render("vmlinux: ") + valueStyle.Render(shortValue(m.vmlinuxPath)),
		labelStyle.Render("manual: ") + valueStyle.Render(shortValue(strings.TrimSpace(m.kernel+" "+m.packageVersion))),
		"",
		titleStyle.Render("Environment"),
		labelStyle.Render("meow: ") + valueStyle.Render(shortValue(m.meowPath)),
		labelStyle.Render("vol: ") + valueStyle.Render(shortValue(m.volPath)),
		labelStyle.Render("symbols: ") + valueStyle.Render(shortValue(m.symbolsPath)),
		labelStyle.Render("out: ") + valueStyle.Render(shortValue(m.outDir)),
		labelStyle.Render("cache: ") + valueStyle.Render(shortValue(m.cacheDir)),
		labelStyle.Render("remote: ") + valueStyle.Render(fmt.Sprintf("%t", !m.noRemoteSymbols)),
		labelStyle.Render("force: ") + valueStyle.Render(fmt.Sprintf("%t", m.force)),
	}
	if m.running {
		rows = append(rows, "", warnStyle.Render("[RUN] "+string(m.runningAction)))
	} else {
		rows = append(rows, "", mutedStyle.Render("[OK] idle"))
	}
	return panelStyle.Width(width).Height(height).Render(trimRows(rows, height-2))
}

func (m Model) centerPanel(width, height int) string {
	rows := []string{titleStyle.Render("Workflow / Logs")}
	if m.lastPreflightResult != nil {
		rows = append(rows, successStyle.Render("[OK] preflight: "+summaryLine(m.lastPreflightResult)))
	}
	if m.lastBuildResult != nil {
		rows = append(rows, successStyle.Render("[OK] build: "+summaryLine(m.lastBuildResult)))
	}
	if m.lastVerifyResult != nil {
		status := "[OK]"
		style := successStyle
		if !m.lastVerifyResult.Success {
			status = "[ERR]"
			style = errorStyle
		}
		rows = append(rows, style.Render(status+" verify"))
	}
	rows = append(rows, "")
	logLines := m.logs.RenderLines()
	maxLogLines := max(1, height-len(rows)-2)
	if len(logLines) > maxLogLines {
		logLines = logLines[len(logLines)-maxLogLines:]
	}
	rows = append(rows, logLines...)
	return panelStyle.Width(width).Height(height).Render(trimRows(rows, height-2))
}

func (m Model) rightPanel(width, height int) string {
	rows := []string{
		titleStyle.Render("Plugin / Results"),
		labelStyle.Render("plugin: ") + valueStyle.Render(shortValue(m.plugin)),
		labelStyle.Render("args: ") + valueStyle.Render(shortValue(strings.Join(m.pluginArgs, " "))),
		"",
		titleStyle.Render("Doctor"),
	}
	if len(m.lastDoctorResult) == 0 {
		rows = append(rows, mutedStyle.Render("-"))
	} else {
		for _, check := range m.lastDoctorResult {
			status := "[OK]"
			style := successStyle
			if !check.OK {
				status = "[ERR]"
				style = errorStyle
			} else if check.Warning {
				status = "[WARN]"
				style = warnStyle
			}
			rows = append(rows, style.Render(status+" "+check.Name))
		}
	}
	rows = append(rows, "", titleStyle.Render("Cache"))
	rows = append(rows, valueStyle.Render(fmt.Sprintf("%d entries", len(m.lastCacheResult))))
	rows = append(rows, "", titleStyle.Render("Known Linux Plugins"))
	for _, cat := range PluginCategories {
		for _, plugin := range cat.Plugins {
			prefix := "  "
			if plugin.Name == m.plugin {
				prefix = "> "
			}
			rows = append(rows, mutedStyle.Render(prefix)+valueStyle.Render(plugin.Name))
		}
	}
	return panelStyle.Width(width).Height(height).Render(trimRows(rows, height-2))
}

func (m Model) commandBar() string {
	hint := "i input | r run | x cancel | q quit"
	if m.inputFocused {
		hint = "enter submit | esc cancel"
	}
	return inactiveBorder.Width(max(20, m.width-2)).Render(" > " + m.input.View() + "  " + mutedStyle.Render(hint))
}

func trimRows(rows []string, maxRows int) string {
	if maxRows <= 0 {
		return ""
	}
	if len(rows) > maxRows {
		rows = rows[:maxRows]
	}
	return strings.Join(rows, "\n")
}

func shortValue(value string) string {
	if strings.TrimSpace(value) == "" {
		return "-"
	}
	if len(value) > 34 {
		return value[:31] + "..."
	}
	return value
}

func summaryLine(summary *BuildSummary) string {
	if summary == nil {
		return "-"
	}
	return strings.TrimSpace(strings.Join([]string{
		emptyDash(summary.Distro),
		emptyDash(summary.Kernel),
		emptyDash(summary.SymbolPath),
	}, " "))
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
