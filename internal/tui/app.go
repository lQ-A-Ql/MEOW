package tui

import (
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ── Messages ────────────────────────────────────────────────

type logMsg struct {
	level   LogLevel
	message string
}

type taskDoneMsg struct {
	err error
}

// ── Model ───────────────────────────────────────────────────

type Model struct {
	meowPath    string
	volPath     string
	memPath     string
	symbolsPath string
	outDir      string
	bannerFile  string
	plugin      string

	width        int
	height       int
	input        textinput.Model
	vp           viewport.Model
	logs         *LogStore
	inputFocused bool

	running    bool
	cancelFunc context.CancelFunc
}

func NewModel() Model {
	ti := textinput.New()
	ti.Placeholder = "输入命令... (/help)"
	ti.CharLimit = 256
	ti.Width = 60

	vp := viewport.New(80, 24)

	return Model{
		meowPath:    "../meow",
		volPath:     "vol",
		symbolsPath: "./symbols",
		outDir:      "./symbols/linux",
		plugin:      "linux.pslist.PsList",
		input:       ti,
		vp:          vp,
		logs:        NewLogStore(),
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(tea.WindowSize(), textinput.Blink)
}

// ── Update ──────────────────────────────────────────────────

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.applyLayout()
		return m, nil

	case tea.MouseMsg:
		return m.onMouse(msg)

	case tea.KeyMsg:
		return m.onKey(msg)

	case logMsg:
		m.logs.Append(msg.level, msg.message)
		m.refreshContent()
		return m, nil

	case taskDoneMsg:
		m.running = false
		m.cancelFunc = nil
		if msg.err != nil {
			m.logs.Append(LogError, fmt.Sprintf("失败: %v", msg.err))
		} else {
			m.logs.Append(LogSuccess, "完成")
		}
		m.refreshContent()
		return m, nil
	}

	if m.inputFocused {
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m *Model) refreshContent() {
	m.vp.SetContent(strings.Join(m.logs.RenderLines(), "\n"))
	m.vp.GotoBottom()
}

func (m *Model) applyLayout() {
	if m.width < 20 || m.height < 10 {
		return
	}
	barContentW := m.width - 6
	if barContentW < 5 {
		barContentW = 5
	}
	m.input.Width = barContentW
}

// ── Mouse ───────────────────────────────────────────────────

func (m Model) onMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	if msg.Action != tea.MouseActionPress || msg.Button != tea.MouseButtonLeft {
		return m, nil
	}

	// Command bar is at the bottom 3 rows
	barTop := m.height - 3
	if msg.Y >= barTop {
		if !m.inputFocused {
			m.inputFocused = true
			return m, m.input.Focus()
		}
	} else {
		if m.inputFocused {
			m.inputFocused = false
			m.input.Blur()
			m.input.SetValue("")
		}
	}
	return m, nil
}

// ── Keyboard ────────────────────────────────────────────────

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
			if value != "" {
				return m.dispatchCommand(value)
			}
			return m, nil
		}
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		return m, cmd
	}

	switch msg.String() {
	case "ctrl+c":
		if m.cancelFunc != nil {
			m.cancelFunc()
		}
		return m, tea.Quit
	case "q":
		return m, tea.Quit
	case "i", ":":
		m.inputFocused = true
		return m, m.input.Focus()
	case "r":
		return m.startAction("run")
	case "x":
		if m.running && m.cancelFunc != nil {
			m.cancelFunc()
			m.logs.Append(LogWarn, "取消中...")
			m.refreshContent()
		}
		return m, nil
	}
	return m, nil
}

// ── Command dispatch ────────────────────────────────────────

func (m Model) dispatchCommand(input string) (tea.Model, tea.Cmd) {
	input = strings.TrimSpace(input)
	if !strings.HasPrefix(input, "/") {
		return m, emitLog(LogError, "未知命令: "+input+" (输入 /help)")
	}

	parts := strings.SplitN(input, " ", 2)
	cmd := strings.ToLower(parts[0][1:])
	args := ""
	if len(parts) > 1 {
		args = strings.TrimSpace(parts[1])
	}

	switch cmd {
	case "help":
		return m, emitLog(LogInfo, strings.Join([]string{
			"可用命令:",
			"  /mem <path>       设置内存镜像路径",
			"  /symbol <path>    设置符号表路径",
			"  /plugin <name>    设置当前 Volatility 插件",
			"  /run              执行当前插件",
			"  /banner           提取内核 banner",
			"  /build            运行 meow build --dry-run",
			"  /verify           验证符号表",
			"  /clear            清空输出日志",
			"  /help             显示此帮助",
			"",
			"快捷键: i/点击 输入 | r 执行 | x 取消 | q 退出",
		}, "\n"))
	case "mem":
		if args == "" {
			return m, emitLog(LogInfo, "用法: /mem <内存镜像路径>")
		}
		m.memPath = args
		return m, emitLog(LogSuccess, "✓ 内存镜像: "+args)
	case "symbol":
		if args == "" {
			return m, emitLog(LogInfo, "用法: /symbol <符号表路径>")
		}
		m.symbolsPath = args
		return m, emitLog(LogSuccess, "✓ 符号表: "+args)
	case "plugin":
		if args == "" {
			return m, emitLog(LogInfo, "用法: /plugin <插件名>")
		}
		m.plugin = args
		return m, emitLog(LogSuccess, "✓ 插件: "+args)
	case "clear":
		m.logs.Clear()
		m.vp.SetContent("")
		return m, nil
	case "run":
		return m.startAction("run")
	case "banner":
		return m.startAction("banner")
	case "build":
		return m.startAction("build")
	case "verify":
		return m.startAction("verify")
	default:
		return m, emitLog(LogError, "✗ 未知命令: /"+cmd)
	}
}

func emitLog(level LogLevel, msg string) tea.Cmd {
	return func() tea.Msg {
		return logMsg{level: level, message: msg}
	}
}

// ── Async actions ───────────────────────────────────────────

func (m Model) startAction(action string) (tea.Model, tea.Cmd) {
	if m.running {
		return m, emitLog(LogWarn, "任务运行中，按 x 取消")
	}
	needMem := action == "run" || action == "banner" || action == "verify"
	if needMem && m.memPath == "" {
		return m, emitLog(LogError, "✗ 请先设置: /mem <path>")
	}

	ctx, cancel := context.WithCancel(context.Background())
	m.running = true
	m.cancelFunc = cancel

	p := actionParams{
		action: action, volPath: m.volPath, meowPath: m.meowPath,
		memPath: m.memPath, symbols: m.symbolsPath,
		bannerFile: m.bannerFile, plugin: m.plugin,
	}

	return m, tea.Batch(
		emitLog(LogInfo, "⟶ 执行: "+action),
		func() tea.Msg { return taskDoneMsg{err: executeAction(ctx, p)} },
	)
}

type actionParams struct {
	action                                     string
	volPath, meowPath                          string
	memPath, symbols                           string
	bannerFile, plugin                         string
}

func executeAction(ctx context.Context, p actionParams) error {
	switch p.action {
	case "run":
		args := []string{"-f", p.memPath}
		if p.symbols != "" {
			args = append(args, "-s", p.symbols)
		}
		args = append(args, p.plugin)
		_, err := runStream(ctx, p.volPath, args...)
		return err
	case "banner":
		_, err := runStream(ctx, p.volPath, "-f", p.memPath, "banners.Banners")
		return err
	case "build":
		args := []string{"--json", "build", "--dry-run"}
		if p.bannerFile != "" {
			args = append(args, "--banner-file", p.bannerFile)
		}
		args = append(args, "--no-remote-symbols")
		_, err := runStream(ctx, p.meowPath, args...)
		return err
	case "verify":
		_, err := runStream(ctx, p.meowPath, "--json", "verify",
			"--mem", p.memPath, "--symbols", p.symbols)
		return err
	default:
		return fmt.Errorf("unknown action: %s", p.action)
	}
}

// ── View ────────────────────────────────────────────────────

func (m Model) View() string {
	if m.width == 0 {
		return ""
	}

	logo := m.buildLogo()
	left := m.buildLeftPanel()
	right := m.buildRightPanel()

	// Measure actual rendered widths (includes border + padding)
	leftW := lipgloss.Width(left)
	rightW := lipgloss.Width(right)

	// Center fills remaining space
	centerW := m.width - leftW - rightW
	if centerW < 10 {
		centerW = 10
	}

	// Viewport = center content area minus border(2)
	vpW := centerW - 2
	vpH := m.height - 8 - 3 - 2 // logo - bar - border
	if vpH < 1 {
		vpH = 1
	}
	if vpW < 1 {
		vpW = 1
	}
	m.vp.Width = vpW
	m.vp.Height = vpH

	center := m.buildCenterPanel(centerW, vpW, vpH)
	bar := m.buildBar()

	main := lipgloss.JoinHorizontal(lipgloss.Top, left, center, right)
	return lipgloss.JoinVertical(lipgloss.Left, logo, main, bar)
}

func (m Model) buildLogo() string {
	raw := []string{
		"███╗   ███╗███████╗ ██████╗ ██╗    ██╗",
		"████╗ ████║██╔════╝██╔═══██╗██║    ██║",
		"██╔████╔██║█████╗  ██║   ██║██║ █╗ ██║",
		"██║╚██╔╝██║██╔══╝  ██║   ██║██║███╗██║",
		"██║ ╚═╝ ██║███████╗╚██████╔╝╚███╔███╔╝",
		"╚═╝     ╚═╝╚══════╝ ╚═════╝  ╚══╝╚══╝",
		"",
		"=^..^=__/  meow~ Vol3 Linux Symbol Builder",
	}
	var lines []string
	for _, l := range raw {
		lines = append(lines, gradientLine(l, m.width))
	}
	return lipgloss.NewStyle().Width(m.width).Align(lipgloss.Center).Render(
		strings.Join(lines, "\n"),
	)
}

func (m Model) buildLeftPanel() string {
	if m.width < 60 {
		return ""
	}

	rows := []string{
		titleStyle.Render("镜像信息"), "",
		labelStyle.Render("内存镜像:"),
	}
	if m.memPath != "" {
		rows = append(rows, valueStyle.Render("  "+m.memPath))
	} else {
		rows = append(rows, mutedStyle.Render("  <未设置>"))
	}
	rows = append(rows, "",
		labelStyle.Render("符号表:"), valueStyle.Render("  "+m.symbolsPath), "",
		labelStyle.Render("输出目录:"), valueStyle.Render("  "+m.outDir), "",
		labelStyle.Render("当前插件:"),
		lipgloss.NewStyle().Foreground(accentColor2).Render("  "+m.plugin), "",
	)
	if m.running {
		rows = append(rows, warnStyle.Bold(true).Render("⟳ 运行中..."))
	} else {
		rows = append(rows, mutedStyle.Render("空闲"))
	}

	return panelStyle.Width(30).Render(strings.Join(rows, "\n"))
}

func (m Model) buildCenterPanel(contentW, vpW, vpH int) string {
	m.vp.Width = vpW
	m.vp.Height = vpH
	return panelStyle.Width(contentW).Render(m.vp.View())
}

func (m Model) buildRightPanel() string {
	if m.width < 80 {
		return ""
	}

	rows := []string{
		titleStyle.Render("插件列表"),
		mutedStyle.Render(" /plugin <name> 切换"), "",
	}
	for _, cat := range PluginCategories {
		rows = append(rows, warnStyle.Bold(true).Render(cat.Icon+" "+cat.Name))
		for _, p := range cat.Plugins {
			if m.plugin == p.Name {
				rows = append(rows, successStyle.Render("▸ ")+successStyle.Bold(true).Render(p.Name))
			} else {
				rows = append(rows, mutedStyle.Render("  ")+lipgloss.NewStyle().Foreground(brightColor).Render(p.Name))
			}
			rows = append(rows, mutedStyle.Render("    "+p.Description))
		}
		rows = append(rows, "")
	}
	return panelStyle.Width(38).Render(strings.Join(rows, "\n"))
}

func (m Model) buildBar() string {
	border := inactiveBorder
	hint := "i/点击 输入 | r 执行 | q 退出"
	if m.inputFocused {
		border = activeBorder
		hint = "Esc 取消 | Enter 确认"
	}

	left := lipgloss.NewStyle().Bold(true).Foreground(accentColor).Render(" > ")
	right := "  " + mutedStyle.Render(hint)
	bar := left + m.input.View() + right

	return border.Width(m.width).Render(bar)
}

// ── Gradient ────────────────────────────────────────────────

func gradientLine(line string, totalCols int) string {
	runes := []rune(line)
	if totalCols <= 0 {
		totalCols = len(runes)
	}
	var sb strings.Builder
	for col, ch := range runes {
		if ch == ' ' {
			sb.WriteRune(ch)
			continue
		}
		r, g, b := gradientRGB(col, totalCols)
		sb.WriteString(fmt.Sprintf("\033[38;2;%d;%d;%dm%c\033[0m", r, g, b, ch))
	}
	return sb.String()
}

func gradientRGB(col, totalCols int) (int, int, int) {
	if totalCols <= 1 {
		totalCols = 2
	}
	t := float64(col) / float64(totalCols-1)
	switch {
	case t < 0.33:
		s := t / 0.33
		return lerp(0, 120, s), lerp(200, 80, s), lerp(255, 255, s)
	case t < 0.66:
		s := (t - 0.33) / 0.33
		return lerp(120, 255, s), lerp(80, 0, s), lerp(255, 200, s)
	default:
		s := (t - 0.66) / 0.34
		return lerp(255, 255, s), lerp(0, 150, s), lerp(200, 50, s)
	}
}

func lerp(a, b int, t float64) int {
	return a + int(float64(b-a)*t)
}
