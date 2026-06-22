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

const (
	compactHeightLimit = 18
	compactWidthLimit  = 40
	logOnlyBodyLimit   = 22
)

type Model struct {
	meowPath          string
	volPath           string
	inputModeValue    InputMode
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
	logScroll     int
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
		inputModeValue:    options.InputMode(),
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
		m.input.Width = max(4, msg.Width-18)
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
	case "up", "k":
		m.scrollLogs(1)
		return m, nil
	case "down", "j":
		m.scrollLogs(-1)
		return m, nil
	case "pgup", "ctrl+b":
		m.scrollLogs(max(1, m.logVisibleRows()))
		return m, nil
	case "pgdown", "ctrl+f":
		m.scrollLogs(-max(1, m.logVisibleRows()))
		return m, nil
	case "home":
		m.logScroll = m.maxLogScroll()
		return m, nil
	case "end":
		m.logScroll = 0
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
	if validInputMode(m.inputModeValue) {
		return m.inputModeValue
	}
	return m.detectInputMode()
}

func (m Model) View() string {
	if m.width == 0 {
		return "initializing..."
	}
	if m.compactViewMode() {
		return m.compactView()
	}
	logo := renderCompactLogo(m.width)
	bar := m.commandBar()
	bodyHeight := max(0, m.height-lipgloss.Height(logo)-lipgloss.Height(bar))
	body := m.renderBody(bodyHeight)
	return trimViewHeight(lipgloss.JoinVertical(lipgloss.Left, logo, body, bar), m.height)
}

func (m Model) compactViewMode() bool {
	return (m.height > 0 && m.height <= compactHeightLimit) || (m.width > 0 && m.width < compactWidthLimit)
}

func (m Model) compactView() string {
	bar := m.compactCommandBar()
	bodyHeight := max(0, m.height-lipgloss.Height(bar))
	body := m.compactLogView(bodyHeight, max(1, m.width))
	return trimViewHeight(lipgloss.JoinVertical(lipgloss.Left, body, bar), m.height)
}

func (m Model) compactLogView(height, width int) string {
	if height <= 0 {
		return ""
	}
	rows := []string{m.compactStatusLine(width)}
	logRows := m.visibleLogLines(max(0, height-len(rows)), width)
	rows = append(rows, logRows...)
	return trimRows(rows, height)
}

func (m Model) compactStatusLine(width int) string {
	if width < lipgloss.Width("MEOW") {
		return logoStyle.Render(truncateLine("MEOW", width))
	}
	logo := renderGradientText("MEOW")
	status := "[OK] idle"
	if m.running {
		status = "[RUN] " + string(m.runningAction)
	}
	tail := fmt.Sprintf(" %s mode:%s plugin:%s", status, m.inputMode(), shortValue(m.plugin))
	if m.logScroll > 0 {
		tail += fmt.Sprintf(" scroll +%d", min(m.logScroll, m.maxLogScroll()))
	}
	remainingWidth := width - lipgloss.Width(logo)
	if remainingWidth <= 0 {
		return logo
	}
	tail = clipLine(tail, remainingWidth)
	return logo + mutedStyle.Render(tail)
}

func renderCompactLogo(width int) string {
	if width < lipgloss.Width("MEOW") {
		return logoStyle.Render(truncateLine("MEOW", width))
	}
	title := renderGradientText("MEOW") + mutedStyle.Render(" - Linux Symbol Builder")
	if width < 70 {
		return renderGradientText("MEOW")
	}
	return title
}

func renderGradientText(text string) string {
	runes := []rune(text)
	if len(runes) == 0 {
		return ""
	}
	out := strings.Builder{}
	total := max(1, len(runes)-1)
	for i, r := range runes {
		t := float64(i) / float64(total)
		red := lerpInt(96, 167, t)
		green := lerpInt(165, 139, t)
		blue := lerpInt(250, 250, t)
		color := lipgloss.Color(fmt.Sprintf("#%02X%02X%02X", red, green, blue))
		out.WriteString(lipgloss.NewStyle().Bold(true).Foreground(color).Render(string(r)))
	}
	return out.String()
}

func lerpInt(a, b int, t float64) int {
	return a + int(float64(b-a)*t)
}

func (m Model) renderBody(height int) string {
	if height <= 0 {
		return ""
	}
	if height < logOnlyBodyLimit {
		return m.centerPanel(max(1, m.width), height)
	}
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
		return m.centerPanel(max(1, m.width), height)
	}
}

func (m Model) leftPanel(width, height int) string {
	contentHeight := panelContentHeight(height)
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
	return renderPanel(width, height, trimRows(rows, contentHeight))
}

func (m Model) centerPanel(width, height int) string {
	contentHeight := panelContentHeight(height)
	rows := m.workflowHeaderRows(min(m.logScroll, m.maxLogScrollForContent(contentHeight)), contentHeight)
	maxLogLines := max(0, contentHeight-len(rows))
	logLines := m.visibleLogLines(maxLogLines, panelContentWidth(width))
	rows = append(rows, logLines...)
	return renderPanel(width, height, trimRows(rows, contentHeight))
}

func (m Model) workflowHeaderRows(scroll, maxRows int) []string {
	title := "Workflow / Logs"
	if scroll > 0 {
		title += mutedStyle.Render(fmt.Sprintf("  scroll +%d", scroll))
	}
	rows := []string{titleStyle.Render(title)}
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
	if maxRows > len(rows)+1 {
		rows = append(rows, "")
	}
	return rows
}

func (m Model) visibleLogLines(limit, width int) []string {
	if limit <= 0 {
		return nil
	}
	entries := m.logs.Entries()
	if len(entries) == 0 {
		return nil
	}
	maxOffset := max(0, len(entries)-limit)
	offset := min(m.logScroll, maxOffset)
	if offset < 0 {
		offset = 0
	}
	end := len(entries) - offset
	start := max(0, end-limit)
	return renderLogEntries(entries[start:end], width)
}

func renderLogEntries(entries []LogEntry, width int) []string {
	lines := make([]string, 0, len(entries))
	width = max(1, width)
	for _, entry := range entries {
		prefix := fmt.Sprintf("[%s] ", entry.Time)
		line := prefix
		if lipgloss.Width(prefix) >= width {
			line = clipLine(prefix, width)
		} else {
			messageWidth := width - lipgloss.Width(prefix)
			line += clipLine(entry.Message, messageWidth)
		}
		lines = append(lines, logColor(entry.Level).Render(line))
	}
	return lines
}

func (m Model) rightPanel(width, height int) string {
	contentHeight := panelContentHeight(height)
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
	return renderPanel(width, height, trimRows(rows, contentHeight))
}

func (m Model) commandBar() string {
	hint := "i input | r run | x cancel | q quit"
	if m.inputFocused {
		hint = "enter submit | esc cancel"
	}
	contentWidth := max(1, m.width-2)
	prefix := " > "
	inputWidth := max(1, contentWidth-lipgloss.Width(prefix)-lipgloss.Width(hint)-2)
	m.input.Width = inputWidth
	content := prefix + m.input.View()
	if lipgloss.Width(content)+2+lipgloss.Width(hint) <= contentWidth {
		content += "  " + mutedStyle.Render(hint)
	}
	return inactiveBorder.Width(contentWidth).Render(content)
}

func (m Model) compactCommandBar() string {
	width := max(1, m.width)
	hint := " i/r/x/q"
	if m.inputFocused {
		hint = " enter/esc"
	}
	prefix := ">"
	inputWidth := max(1, width-lipgloss.Width(prefix)-lipgloss.Width(hint)-1)
	m.input.Width = inputWidth
	content := prefix + m.input.View()
	if lipgloss.Width(content)+lipgloss.Width(hint) <= width {
		content += mutedStyle.Render(hint)
	}
	return clipLine(content, width)
}

func renderPanel(width, height int, content string) string {
	contentWidth := panelContentWidth(width)
	return panelStyle.
		Width(contentWidth).
		Height(panelContentHeight(height)).
		Render(content)
}

func panelContentWidth(width int) int {
	return max(1, width-4)
}

func panelContentHeight(height int) int {
	return max(0, height-2)
}

func (m Model) logVisibleRows() int {
	if m.compactViewMode() {
		bodyHeight := max(0, m.height-lipgloss.Height(m.compactCommandBar()))
		return max(0, bodyHeight-1)
	}
	logoHeight := lipgloss.Height(renderCompactLogo(m.width))
	barHeight := lipgloss.Height(m.commandBar())
	bodyHeight := max(0, m.height-logoHeight-barHeight)
	contentHeight := panelContentHeight(bodyHeight)
	return max(0, contentHeight-len(m.workflowHeaderRows(0, contentHeight)))
}

func (m Model) maxLogScroll() int {
	return max(0, len(m.logs.Entries())-max(1, m.logVisibleRows()))
}

func (m Model) maxLogScrollForContent(contentHeight int) int {
	visible := max(1, contentHeight-len(m.workflowHeaderRows(0, contentHeight)))
	return max(0, len(m.logs.Entries())-visible)
}

func (m *Model) scrollLogs(delta int) {
	m.logScroll += delta
	if m.logScroll < 0 {
		m.logScroll = 0
	}
	maxScroll := m.maxLogScroll()
	if m.logScroll > maxScroll {
		m.logScroll = maxScroll
	}
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

func clipLine(line string, width int) string {
	if width <= 0 {
		return ""
	}
	if lipgloss.Width(line) <= width {
		return line
	}
	if width <= lipgloss.Width("...") {
		return strings.Repeat(".", width)
	}
	runes := []rune(line)
	out := strings.Builder{}
	for _, r := range runes {
		next := out.String() + string(r)
		if lipgloss.Width(next+"...") > width {
			break
		}
		out.WriteRune(r)
	}
	return out.String() + "..."
}

func truncateLine(line string, width int) string {
	if width <= 0 {
		return ""
	}
	if lipgloss.Width(line) <= width {
		return line
	}
	runes := []rune(line)
	out := strings.Builder{}
	for _, r := range runes {
		next := out.String() + string(r)
		if lipgloss.Width(next) > width {
			break
		}
		out.WriteRune(r)
	}
	return out.String()
}

func trimViewHeight(view string, maxHeight int) string {
	if maxHeight <= 0 {
		return ""
	}
	lines := strings.Split(view, "\n")
	if len(lines) > maxHeight {
		lines = lines[:maxHeight]
	}
	return strings.Join(lines, "\n")
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

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
