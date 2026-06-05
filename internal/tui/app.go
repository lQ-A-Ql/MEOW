package tui

import (
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"meow/internal/runner"
)

// Messages

type outputMsg struct {
	level   LogLevel
	message string
}

type taskDoneMsg struct {
	err error
}

// ImageInfo holds parsed banner data for display.

type ImageInfo struct {
	Distro string
	Kernel string
	Arch   string
	PkgVer string
}

// Model is the main TUI model.

type Model struct {
	// Paths
	meowPath    string
	volPath     string
	memPath     string
	symbolsPath string
	outDir      string
	bannerFile  string

	// Plugin
	plugin string

	// UI
	width        int
	height       int
	input        textinput.Model
	logVP        viewport.Model
	logs         *LogStore
	inputFocused bool

	// Running task
	running    bool
	cancelFunc context.CancelFunc

	// Image info
	imageInfo *ImageInfo
}

func NewModel() Model {
	ti := textinput.New()
	ti.Placeholder = "输入命令... (/help 查看帮助)"
	ti.CharLimit = 256

	vp := viewport.New(0, 0)

	return Model{
		meowPath:    "../meow",
		volPath:     "vol",
		memPath:     "",
		symbolsPath: "./symbols",
		outDir:      "./symbols/linux",
		bannerFile:  "",
		plugin:      "linux.pslist.PsList",
		input:       ti,
		logVP:       vp,
		logs:        NewLogStore(),
	}
}

func (m Model) Init() tea.Cmd {
	return textinput.Blink
}

// ── Update ──────────────────────────────────────────────────

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.resize()
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)

	case outputMsg:
		m.logs.Append(msg.level, msg.message)
		m.refreshLogVP()
		return m, nil

	case taskDoneMsg:
		m.running = false
		m.cancelFunc = nil
		if msg.err != nil {
			m.logs.Append(LogError, fmt.Sprintf("失败: %v", msg.err))
		} else {
			m.logs.Append(LogSuccess, "完成")
		}
		m.refreshLogVP()
		return m, nil
	}

	// Forward to textinput when focused
	if m.inputFocused {
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m *Model) refreshLogVP() {
	m.logVP.SetContent(strings.Join(m.logs.RenderLines(), "\n"))
	m.logVP.GotoBottom()
}

func (m *Model) resize() {
	if m.width == 0 || m.height == 0 {
		return
	}

	logoH := 8 // 6 logo lines + subtitle + blank
	cmdBarH := 3
	panelBorderH := 4 // top/bottom border + padding

	panelsH := m.height - logoH - cmdBarH
	if panelsH < 6 {
		panelsH = 6
	}

	leftW := 30
	rightW := 38
	centerW := m.width - leftW - rightW - 3 // 3 for gaps
	if centerW < 20 {
		centerW = 20
		leftW = (m.width - centerW - 3) / 2
		rightW = m.width - centerW - leftW - 3
	}

	contentH := panelsH - panelBorderH
	if contentH < 1 {
		contentH = 1
	}

	leftPanelStyle = leftPanelStyle.Width(leftW)
	centerPanelStyle = centerPanelStyle.Width(centerW)
	rightPanelStyle = rightPanelStyle.Width(rightW)

	m.logVP.Width = centerW - 4
	m.logVP.Height = contentH
	m.input.Width = m.width - 12
}

// ── Keyboard ────────────────────────────────────────────────

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.inputFocused {
		switch msg.String() {
		case "esc":
			m.inputFocused = false
			m.input.Blur()
			return m, nil
		case "enter":
			value := m.input.Value()
			m.input.SetValue("")
			m.inputFocused = false
			m.input.Blur()
			if value != "" {
				return m.executeCommand(value)
			}
			return m, nil
		}
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		return m, cmd
	}

	switch msg.String() {
	case "ctrl+c", "q":
		if m.running && m.cancelFunc != nil {
			m.cancelFunc()
		}
		return m, tea.Quit

	case "esc":
		return m, tea.Quit

	case "i", ":":
		m.inputFocused = true
		return m, m.input.Focus()

	case "r":
		return m.runAction("run")

	case "x":
		if m.running && m.cancelFunc != nil {
			m.cancelFunc()
			m.logs.Append(LogWarn, "取消中...")
			m.refreshLogVP()
		}
		return m, nil
	}

	return m, nil
}

// ── Command execution (synchronous model mutations) ─────────

func (m Model) executeCommand(input string) (tea.Model, tea.Cmd) {
	input = strings.TrimSpace(input)
	if !strings.HasPrefix(input, "/") {
		m.logs.Append(LogError, fmt.Sprintf("未知命令: %s (输入 /help 查看帮助)", input))
		m.refreshLogVP()
		return m, nil
	}

	parts := strings.SplitN(input, " ", 2)
	cmd := parts[0][1:]
	args := ""
	if len(parts) > 1 {
		args = strings.TrimSpace(parts[1])
	}

	switch cmd {
	case "help":
		m.logs.Append(LogInfo, strings.Join([]string{
			"可用命令:",
			"",
			"  /mem <path>       设置内存镜像路径",
			"  /symbol <path>    设置符号表路径",
			"  /plugin <name>    设置当前 Volatility 插件",
			"",
			"  /run              执行当前插件",
			"  /banner           提取内核 banner",
			"  /build            运行 meow build --dry-run",
			"  /verify           验证符号表",
			"",
			"  /clear            清空输出日志",
			"  /help             显示此帮助",
			"",
			"快捷键: i 聚焦输入 | r 执行 | x 取消 | q 退出",
		}, "\n"))
		m.refreshLogVP()
		return m, nil

	case "mem":
		if args == "" {
			m.logs.Append(LogInfo, "用法: /mem <内存镜像路径>")
		} else {
			m.memPath = args
			m.logs.Append(LogSuccess, fmt.Sprintf("✓ 内存镜像: %s", args))
		}
		m.refreshLogVP()
		return m, nil

	case "symbol":
		if args == "" {
			m.logs.Append(LogInfo, "用法: /symbol <符号表路径>")
		} else {
			m.symbolsPath = args
			m.logs.Append(LogSuccess, fmt.Sprintf("✓ 符号表: %s", args))
		}
		m.refreshLogVP()
		return m, nil

	case "plugin":
		if args == "" {
			m.logs.Append(LogInfo, "用法: /plugin <插件名>")
		} else {
			m.plugin = args
			m.logs.Append(LogSuccess, fmt.Sprintf("✓ 插件: %s", args))
		}
		m.refreshLogVP()
		return m, nil

	case "clear":
		m.logs.Clear()
		m.refreshLogVP()
		return m, nil

	case "run":
		return m.runAction("run")
	case "banner":
		return m.runAction("banner")
	case "build":
		return m.runAction("build")
	case "verify":
		return m.runAction("verify")

	default:
		m.logs.Append(LogError, fmt.Sprintf("✗ 未知命令: /%s (输入 /help 查看帮助)", cmd))
		m.refreshLogVP()
		return m, nil
	}
}

// ── Async action execution ──────────────────────────────────

func (m Model) runAction(action string) (tea.Model, tea.Cmd) {
	if m.running {
		m.logs.Append(LogWarn, "任务运行中，请等待完成或按 x 取消")
		m.refreshLogVP()
		return m, nil
	}

	if m.memPath == "" && (action == "run" || action == "banner" || action == "verify") {
		m.logs.Append(LogError, "✗ 请先设置内存镜像: /mem <path>")
		m.refreshLogVP()
		return m, nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	m.running = true
	m.cancelFunc = cancel
	m.logs.Append(LogInfo, fmt.Sprintf("⟶ 执行: %s", action))
	m.refreshLogVP()

	// Capture paths by value for the goroutine
	volPath := m.volPath
	meowPath := m.meowPath
	memPath := m.memPath
	symbolsPath := m.symbolsPath
	bannerFile := m.bannerFile
	plugin := m.plugin

	cmd := func() tea.Msg {
		var err error
		switch action {
		case "run":
			err = doRun(ctx, volPath, memPath, symbolsPath, plugin)
		case "banner":
			err = doBanner(ctx, volPath, memPath)
		case "build":
			err = doBuild(ctx, meowPath, bannerFile)
		case "verify":
			err = doVerify(ctx, meowPath, memPath, symbolsPath)
		}
		return taskDoneMsg{err: err}
	}

	return m, cmd
}

func doRun(ctx context.Context, volPath, memPath, symbolsPath, plugin string) error {
	args := []string{"-f", memPath}
	if symbolsPath != "" {
		args = append(args, "-s", symbolsPath)
	}
	args = append(args, plugin)
	_, err := runner.StreamOutputDisplay(ctx, "vol "+strings.Join(args, " "), volPath, nil, args...)
	return err
}

func doBanner(ctx context.Context, volPath, memPath string) error {
	args := []string{"-f", memPath, "banners.Banners"}
	_, err := runner.StreamOutputDisplay(ctx, "vol banners.Banners", volPath, nil, args...)
	return err
}

func doBuild(ctx context.Context, meowPath, bannerFile string) error {
	args := []string{"--json", "build", "--dry-run"}
	if bannerFile != "" {
		args = append(args, "--banner-file", bannerFile)
	}
	args = append(args, "--no-remote-symbols")
	_, err := runner.StreamOutputDisplay(ctx, "meow build --dry-run", meowPath, nil, args...)
	return err
}

func doVerify(ctx context.Context, meowPath, memPath, symbolsPath string) error {
	args := []string{"--json", "verify", "--mem", memPath, "--symbols", symbolsPath}
	_, err := runner.StreamOutputDisplay(ctx, "meow verify", meowPath, nil, args...)
	return err
}

// ── View ────────────────────────────────────────────────────

func (m Model) View() string {
	if m.width == 0 {
		return ""
	}

	logo := m.renderLogo()
	left := m.renderLeftPanel()
	center := m.renderCenterPanel()
	right := m.renderRightPanel()
	cmdBar := m.renderCommandBar()

	panels := lipgloss.JoinHorizontal(lipgloss.Top, left, center, right)
	return lipgloss.JoinVertical(lipgloss.Left, logo, panels, cmdBar)
}

func (m Model) renderLogo() string {
	logoLines := []string{
		"███╗   ███╗███████╗ ██████╗ ██╗    ██╗",
		"████╗ ████║██╔════╝██╔═══██╗██║    ██║",
		"██╔████╔██║█████╗  ██║   ██║██║ █╗ ██║",
		"██║╚██╔╝██║██╔══╝  ██║   ██║██║███╗██║",
		"██║ ╚═╝ ██║███████╗╚██████╔╝╚███╔███╔╝",
		"╚═╝     ╚═╝╚══════╝ ╚═════╝  ╚══╝╚══╝",
	}
	subtitle := "=^..^=__/  meow~ Vol3 Linux Symbol Builder v0.1.0"

	var rendered []string
	for _, line := range logoLines {
		rendered = append(rendered, gradientLine(line, m.width))
	}
	rendered = append(rendered, gradientLine(subtitle, m.width))

	return lipgloss.NewStyle().Width(m.width).Align(lipgloss.Center).Render(
		strings.Join(rendered, "\n"),
	)
}

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

func (m Model) renderLeftPanel() string {
	var lines []string

	lines = append(lines, titleStyle.Render("镜像信息"))
	lines = append(lines, "")

	lines = append(lines, labelStyle.Render("内存镜像:"))
	if m.memPath != "" {
		lines = append(lines, valueStyle.Render("  "+m.memPath))
	} else {
		lines = append(lines, mutedStyle.Render("  <未设置>"))
	}

	lines = append(lines, "")
	lines = append(lines, labelStyle.Render("符号表:"))
	lines = append(lines, valueStyle.Render("  "+m.symbolsPath))

	lines = append(lines, "")
	lines = append(lines, labelStyle.Render("输出目录:"))
	lines = append(lines, valueStyle.Render("  "+m.outDir))

	lines = append(lines, "")
	lines = append(lines, labelStyle.Render("当前插件:"))
	lines = append(lines, lipgloss.NewStyle().Foreground(accentColor2).Render("  "+m.plugin))

	if m.imageInfo != nil {
		lines = append(lines, "")
		lines = append(lines, successStyle.Bold(true).Render("Banner 信息"))
		if m.imageInfo.Distro != "" {
			lines = append(lines, valueStyle.Render("  发行版: "+m.imageInfo.Distro))
		}
		if m.imageInfo.Kernel != "" {
			lines = append(lines, valueStyle.Render("  内核:   "+m.imageInfo.Kernel))
		}
		if m.imageInfo.Arch != "" {
			lines = append(lines, valueStyle.Render("  架构:   "+m.imageInfo.Arch))
		}
		if m.imageInfo.PkgVer != "" {
			lines = append(lines, valueStyle.Render("  版本:   "+m.imageInfo.PkgVer))
		}
	}

	lines = append(lines, "")
	if m.running {
		lines = append(lines, warnStyle.Bold(true).Render("⟳ 运行中..."))
	} else {
		lines = append(lines, mutedStyle.Render("空闲"))
	}

	return leftPanelStyle.Render(strings.Join(lines, "\n"))
}

func (m Model) renderCenterPanel() string {
	return centerPanelStyle.Render(m.logVP.View())
}

func (m Model) renderRightPanel() string {
	var lines []string

	lines = append(lines, titleStyle.Render("插件列表"))
	lines = append(lines, mutedStyle.Render(" /plugin <name> 切换"))
	lines = append(lines, "")

	for _, cat := range PluginCategories {
		lines = append(lines, warnStyle.Bold(true).Render(cat.Icon+" "+cat.Name))
		for _, p := range cat.Plugins {
			if m.plugin == p.Name {
				lines = append(lines, successStyle.Render("▸ ")+successStyle.Bold(true).Render(p.Name))
			} else {
				lines = append(lines, mutedStyle.Render("  ")+lipgloss.NewStyle().Foreground(brightColor).Render(p.Name))
			}
			lines = append(lines, mutedStyle.Render("    "+p.Description))
		}
		lines = append(lines, "")
	}

	return rightPanelStyle.Render(strings.Join(lines, "\n"))
}

func (m Model) renderCommandBar() string {
	border := inactiveBorder
	hint := mutedStyle.Render("i 聚焦 | r 执行 | q 退出")
	if m.inputFocused {
		border = activeBorder
		hint = mutedStyle.Render("Esc 取消 | Enter 确认")
	}

	bar := lipgloss.JoinHorizontal(lipgloss.Center,
		lipgloss.NewStyle().Bold(true).Foreground(accentColor).Render(" > "),
		m.input.View(),
		"  ",
		hint,
	)

	return border.Width(m.width - 2).Render(bar)
}
