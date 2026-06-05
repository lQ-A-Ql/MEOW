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

type taskDoneMsg struct {
	err error
}

type Model struct {
	meowPath, volPath, memPath string
	symbolsPath, outDir        string
	bannerFile, plugin         string

	width, height int
	input         textinput.Model
	logs          *LogStore
	inputFocused  bool
	running       bool
	cancelFunc    context.CancelFunc
}

func NewModel() Model {
	ti := textinput.New()
	ti.Placeholder = "输入命令... (/help)"
	ti.CharLimit = 256
	ti.Width = 60

	return Model{
		meowPath:    "../meow",
		volPath:     "vol",
		symbolsPath: "./symbols",
		outDir:      "./symbols/linux",
		plugin:      "linux.pslist.PsList",
		input:       ti,
		logs:        NewLogStore(),
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
		m.input.Width = m.width - 10
		return m, nil
	case tea.KeyMsg:
		return m.onKey(msg)
	case logMsg:
		m.logs.Append(msg.level, msg.message)
		return m, nil
	case taskDoneMsg:
		m.running = false
		m.cancelFunc = nil
		if msg.err != nil {
			m.logs.Append(LogError, fmt.Sprintf("失败: %v", msg.err))
		} else {
			m.logs.Append(LogSuccess, "完成")
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
			v := m.input.Value()
			m.input.SetValue("")
			m.inputFocused = false
			m.input.Blur()
			if v != "" { return m.dispatch(v) }
			return m, nil
		}
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		return m, cmd
	}
	switch msg.String() {
	case "ctrl+c", "q":
		if m.cancelFunc != nil { m.cancelFunc() }
		return m, tea.Quit
	case "i", ":":
		m.inputFocused = true
		return m, m.input.Focus()
	case "r": return m.startAction("run")
	case "x":
		if m.running && m.cancelFunc != nil {
			m.cancelFunc()
			m.logs.Append(LogWarn, "取消中...")
		}
		return m, nil
	}
	return m, nil
}

func (m Model) dispatch(input string) (tea.Model, tea.Cmd) {
	input = strings.TrimSpace(input)
	if !strings.HasPrefix(input, "/") {
		return m, emitLog(LogError, "未知命令: "+input)
	}
	p := strings.SplitN(input, " ", 2)
	cmd := strings.ToLower(p[0][1:])
	args := ""
	if len(p) > 1 { args = strings.TrimSpace(p[1]) }

	switch cmd {
	case "help":
		return m, emitLog(LogInfo, "可用命令:\n  /mem <path>    设置内存镜像\n  /symbol <path> 设置符号表\n  /plugin <name> 设置插件\n  /run           执行插件\n  /banner        提取banner\n  /build         dry-run构建\n  /verify        验证符号\n  /clear         清空日志\n  /help          显示帮助\n\n快捷键: i 输入 | r 执行 | x 取消 | q 退出")
	case "mem":
		if args == "" { return m, emitLog(LogInfo, "用法: /mem <path>") }
		m.memPath = args; return m, emitLog(LogSuccess, "✓ 内存镜像: "+args)
	case "symbol":
		if args == "" { return m, emitLog(LogInfo, "用法: /symbol <path>") }
		m.symbolsPath = args; return m, emitLog(LogSuccess, "✓ 符号表: "+args)
	case "plugin":
		if args == "" { return m, emitLog(LogInfo, "用法: /plugin <name>") }
		m.plugin = args; return m, emitLog(LogSuccess, "✓ 插件: "+args)
	case "clear": m.logs.Clear(); return m, nil
	case "run": return m.startAction("run")
	case "banner": return m.startAction("banner")
	case "build": return m.startAction("build")
	case "verify": return m.startAction("verify")
	default: return m, emitLog(LogError, "✗ 未知: /"+cmd)
	}
}

func emitLog(level LogLevel, msg string) tea.Cmd {
	return func() tea.Msg { return logMsg{level: level, message: msg} }
}

func (m Model) startAction(action string) (tea.Model, tea.Cmd) {
	if m.running { return m, emitLog(LogWarn, "任务运行中，按 x 取消") }
	if (action == "run" || action == "banner" || action == "verify") && m.memPath == "" {
		return m, emitLog(LogError, "✗ 请先设置: /mem <path>")
	}
	ctx, cancel := context.WithCancel(context.Background())
	m.running = true
	m.cancelFunc = cancel
	p := actP{action, m.volPath, m.meowPath, m.memPath, m.symbolsPath, m.bannerFile, m.plugin}
	return m, tea.Batch(emitLog(LogInfo, "⟶ 执行: "+action), func() tea.Msg { return taskDoneMsg{err: doAct(ctx, p)} })
}

type actP struct{ action, vol, meow, mem, sym, banner, plugin string }

func doAct(ctx context.Context, p actP) error {
	switch p.action {
	case "run":
		args := []string{"-f", p.mem}
		if p.sym != "" { args = append(args, "-s", p.sym) }
		args = append(args, p.plugin)
		_, err := runStream(ctx, p.vol, args...); return err
	case "banner":
		_, err := runStream(ctx, p.vol, "-f", p.mem, "banners.Banners"); return err
	case "build":
		args := []string{"--json", "build", "--dry-run"}
		if p.banner != "" { args = append(args, "--banner-file", p.banner) }
		args = append(args, "--no-remote-symbols")
		_, err := runStream(ctx, p.meow, args...); return err
	case "verify":
		_, err := runStream(ctx, p.meow, "--json", "verify", "--mem", p.mem, "--symbols", p.sym); return err
	}
	return fmt.Errorf("unknown: %s", p.action)
}

// ── View ────────────────────────────────────────────────────

func (m Model) View() string {
	if m.width == 0 { return "正在初始化..." }
	w, h := m.width, m.height

	// Logo (7 lines)
	logoRaw := []string{
		"███╗   ███╗███████╗ ██████╗ ██╗    ██╗",
		"████╗ ████║██╔════╝██╔═══██╗██║    ██║",
		"██╔████╔██║█████╗  ██║   ██║██║ █╗ ██║",
		"██║╚██╔╝██║██╔══╝  ██║   ██║██║███╗██║",
		"██║ ╚═╝ ██║███████╗╚██████╔╝╚███╔███╔╝",
		"╚═╝     ╚═╝╚══════╝ ╚═════╝  ╚══╝╚══╝",
		"=^..^=__/  meow~ Vol3 Linux Symbol Builder",
	}
	var logoB strings.Builder
	for _, l := range logoRaw {
		logoB.WriteString(gradientLine(l, w))
		logoB.WriteByte('\n')
	}
	logoStr := strings.TrimRight(logoB.String(), "\n")
	logoH := 7

	// Bar (1 line)
	bar := " > " + m.input.View() + "  " + mutedStyle.Render("i 输入 | r 执行 | q 退出")
	barH := 1

	// Panel area
	panelH := h - logoH - barH
	if panelH < 3 { panelH = 3 }

	leftW, rightW := 0, 0
	if w >= 80 {
		leftW = 28; rightW = 36
	} else if w >= 55 {
		leftW = 22
	}

	// Left panel
	left := ""
	if leftW > 0 {
		rows := []string{titleStyle.Render("镜像信息"), "",
			labelStyle.Render("插件: ") + valueStyle.Render(m.plugin),
			labelStyle.Render("符号: ") + valueStyle.Render(m.symbolsPath),
			"",
		}
		if m.memPath != "" {
			rows = append(rows, labelStyle.Render("镜像: ")+valueStyle.Render(m.memPath))
		} else {
			rows = append(rows, labelStyle.Render("镜像: ")+mutedStyle.Render("<未设置>"))
		}
		rows = append(rows, "")
		if m.running {
			rows = append(rows, warnStyle.Bold(true).Render("⟳ 运行中..."))
		} else {
			rows = append(rows, mutedStyle.Render("空闲"))
		}
		left = panelStyle.Width(leftW).Render(strings.Join(rows, "\n"))
	}

	// Right panel
	right := ""
	if rightW > 0 {
		rows := []string{titleStyle.Render("插件列表"), ""}
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
		}
		right = panelStyle.Width(rightW).Render(strings.Join(rows, "\n"))
	}

	// Log lines (fill available height)
	logLines := m.logs.RenderLines()
	start := 0
	if len(logLines) > panelH-2 { start = len(logLines) - (panelH - 2) }
	if start < 0 { start = 0 }
	visible := logLines[start:]
	logContent := strings.Join(visible, "\n")

	// Left padding
	logView := " " + logContent

	// Join
	parts := []string{logoStr}
	if left != "" || right != "" {
		parts = append(parts, lipgloss.JoinHorizontal(lipgloss.Top, left, logView, right))
	} else {
		parts = append(parts, logView)
	}
	parts = append(parts, bar)

	return lipgloss.JoinVertical(lipgloss.Left, parts...)
}

// ── Gradient ────────────────────────────────────────────────

func gradientLine(line string, totalCols int) string {
	runes := []rune(line)
	if totalCols <= 0 { totalCols = len(runes) }
	var sb strings.Builder
	for col, ch := range runes {
		if ch == ' ' { sb.WriteRune(ch); continue }
		r, g, b := gradientRGB(col, totalCols)
		sb.WriteString(fmt.Sprintf("\033[38;2;%d;%d;%dm%c\033[0m", r, g, b, ch))
	}
	return sb.String()
}

func gradientRGB(col, totalCols int) (int, int, int) {
	if totalCols <= 1 { totalCols = 2 }
	t := float64(col) / float64(totalCols-1)
	switch {
	case t < 0.33: s := t / 0.33; return lerp(0, 120, s), lerp(200, 80, s), lerp(255, 255, s)
	case t < 0.66: s := (t - 0.33) / 0.33; return lerp(120, 255, s), lerp(80, 0, s), lerp(255, 200, s)
	default: s := (t - 0.66) / 0.34; return lerp(255, 255, s), lerp(0, 150, s), lerp(200, 50, s)
	}
}

func lerp(a, b int, t float64) int { return a + int(float64(b-a)*t) }
