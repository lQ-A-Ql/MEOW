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

type tickMsg struct{}

// ImageInfo holds parsed banner data for display.

type ImageInfo struct {
	Distro  string
	Kernel  string
	Arch    string
	PkgVer  string
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
	width         int
	height        int
	input         textinput.Model
	logVP         viewport.Model
	logs          *LogStore
	inputFocused  bool

	// Running task
	running    bool
	cancelFunc context.CancelFunc

	// Image info
	imageInfo *ImageInfo

	// Logo
	logoLines []string
}

func NewModel() Model {
	ti := textinput.New()
	ti.Placeholder = "输入命令... (/help 查看帮助)"
	ti.CharLimit = 256

	vp := viewport.New(0, 0)

	logo := []string{
		"  ███╗   ███╗███████╗ ██████╗ ██╗    ██╗██╗   ██╗██╗██╗",
		"  ████╗ ████║██╔════╝██╔═══██╗██║    ██║██║   ██║██║██║",
		"  ██╔████╔██║█████╗  ██║   ██║██║ █╗ ██║██║   ██║██║██║",
		"  ██║╚██╔╝██║██╔══╝  ██║   ██║██║███╗██║╚██╗ ██╔╝██║██║",
		"  ██║ ╚═╝ ██║███████╗╚██████╔╝╚███╔███╔╝ ╚████╔╝ ██║██║",
		"  ╚═╝     ╚═╝╚══════╝ ╚═════╝  ╚══╝╚══╝   ╚═══╝  ╚═╝╚═╝",
		"                       ~~~ Vol3 Linux Symbol Builder",
	}

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
		logoLines:   logo,
	}
}

func (m Model) Init() tea.Cmd {
	return textinput.Blink
}

// Update

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
		m.logVP.SetContent(strings.Join(m.logs.RenderLines(), "\n"))
		m.logVP.GotoBottom()
		return m, nil

	case taskDoneMsg:
		m.running = false
		m.cancelFunc = nil
		if msg.err != nil {
			m.logs.Append(LogError, fmt.Sprintf("失败: %v", msg.err))
		} else {
			m.logs.Append(LogSuccess, "完成")
		}
		m.logVP.SetContent(strings.Join(m.logs.RenderLines(), "\n"))
		m.logVP.GotoBottom()
		return m, nil
	}

	var cmd tea.Cmd
	if m.inputFocused {
		m.input, cmd = m.input.Update(msg)
	}
	return m, cmd
}

func (m *Model) resize() {
	logoH := len(m.logoLines) + 1
	cmdBarH := 3
	// Borders + padding: each panel has 2 border rows + 2 padding rows = 4
	panelsH := m.height - logoH - cmdBarH
	if panelsH < 5 {
		panelsH = 5
	}

	leftW := 30  // 28 content + 2 border
	rightW := 38 // 36 content + 2 border
	centerW := m.width - leftW - rightW
	if centerW < 20 {
		centerW = 20
	}

	contentH := panelsH - 4 // subtract border(2) + padding(2)
	if contentH < 1 {
		contentH = 1
	}

	m.logVP.Width = centerW - 4   // border(2) + padding(2)
	m.logVP.Height = contentH
	m.input.Width = m.width - 10
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// When input is focused, let textinput handle most keys
	if m.inputFocused {
		switch msg.String() {
		case "esc":
			m.inputFocused = false
			m.input.Blur()
			return m, nil
		case "enter":
			value := m.input.Value()
			m.input.SetValue("")
			if value != "" {
				return m, m.executeCommand(value)
			}
			return m, nil
		}
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		return m, cmd
	}

	// Global keys
	switch msg.String() {
	case "ctrl+c", "q":
		if m.running && m.cancelFunc != nil {
			m.cancelFunc()
			m.logs.Append(LogWarn, "已取消")
		}
		return m, tea.Quit

	case "esc":
		return m, tea.Quit

	case "i", ":":
		m.inputFocused = true
		return m, m.input.Focus()

	case "r":
		return m, m.runAction("run")

	case "x":
		if m.running && m.cancelFunc != nil {
			m.cancelFunc()
			m.logs.Append(LogWarn, "取消中...")
		}
		return m, nil
	}

	return m, nil
}

// Command execution

func (m Model) executeCommand(input string) tea.Cmd {
	input = strings.TrimSpace(input)
	if !strings.HasPrefix(input, "/") {
		m.logs.Append(LogError, fmt.Sprintf("未知命令: %s (输入 /help 查看帮助)", input))
		return nil
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
		return nil

	case "mem":
		if args == "" {
			m.logs.Append(LogInfo, "用法: /mem <内存镜像路径>")
			return nil
		}
		m.memPath = args
		m.logs.Append(LogSuccess, fmt.Sprintf("内存镜像路径: %s", args))
		return nil

	case "symbol":
		if args == "" {
			m.logs.Append(LogInfo, "用法: /symbol <符号表路径>")
			return nil
		}
		m.symbolsPath = args
		m.logs.Append(LogSuccess, fmt.Sprintf("符号表路径: %s", args))
		return nil

	case "plugin":
		if args == "" {
			m.logs.Append(LogInfo, "用法: /plugin <插件名>")
			return nil
		}
		m.plugin = args
		m.logs.Append(LogSuccess, fmt.Sprintf("当前插件: %s", args))
		return nil

	case "clear":
		m.logs.Clear()
		return nil

	case "run":
		return m.runAction("run")
	case "banner":
		return m.runAction("banner")
	case "build":
		return m.runAction("build")
	case "verify":
		return m.runAction("verify")

	default:
		m.logs.Append(LogError, fmt.Sprintf("未知命令: /%s (输入 /help 查看帮助)", cmd))
		return nil
	}
}

// runAction starts a subprocess task.

func (m Model) runAction(action string) tea.Cmd {
	if m.running {
		m.logs.Append(LogWarn, "任务运行中，请等待完成或按 x 取消")
		return nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	m.cancelFunc = cancel

	return tea.Batch(
		func() tea.Msg {
			m.logs.Append(LogInfo, fmt.Sprintf("执行: %s", action))
			var err error
			switch action {
			case "run":
				err = m.actionRun(ctx)
			case "banner":
				err = m.actionBanner(ctx)
			case "build":
				err = m.actionBuild(ctx)
			case "verify":
				err = m.actionVerify(ctx)
			}
			return taskDoneMsg{err: err}
		},
		func() tea.Msg {
			m.running = true
			return nil
		},
	)
}

func (m Model) actionRun(ctx context.Context) error {
	args := []string{"-f", m.memPath}
	if m.symbolsPath != "" {
		args = append(args, "-s", m.symbolsPath)
	}
	args = append(args, m.plugin)

	_, err := runner.StreamOutputDisplay(ctx, fmt.Sprintf("vol %s", strings.Join(args, " ")),
		m.volPath, func(line string) {}, args...)
	return err
}

func (m Model) actionBanner(ctx context.Context) error {
	args := []string{"-f", m.memPath, "banners.Banners"}
	_, err := runner.StreamOutputDisplay(ctx, "vol banners.Banners",
		m.volPath, func(line string) {}, args...)
	return err
}

func (m Model) actionBuild(ctx context.Context) error {
	args := []string{"--json", "build", "--dry-run"}
	if m.bannerFile != "" {
		args = append(args, "--banner-file", m.bannerFile)
	}
	args = append(args, "--no-remote-symbols")

	_, err := runner.StreamOutputDisplay(ctx, "meow build --dry-run",
		m.meowPath, func(line string) {}, args...)
	return err
}

func (m Model) actionVerify(ctx context.Context) error {
	args := []string{"--json", "verify", "--mem", m.memPath, "--symbols", m.symbolsPath}
	_, err := runner.StreamOutputDisplay(ctx, "meow verify",
		m.meowPath, func(line string) {}, args...)
	return err
}

// View

func (m Model) View() string {
	if m.width == 0 {
		return "Initializing..."
	}

	// Logo
	logo := lipgloss.NewStyle().Width(m.width).Align(lipgloss.Center).Render(
		strings.Join(m.logoLines, "\n"),
	)

	// Left panel
	left := m.renderLeftPanel()

	// Center panel
	center := m.renderCenterPanel()

	// Right panel
	right := m.renderRightPanel()

	// Join panels horizontally
	panels := lipgloss.JoinHorizontal(lipgloss.Top, left, center, right)

	// Command bar
	cmdBar := m.renderCommandBar()

	return lipgloss.JoinVertical(lipgloss.Left, logo, panels, cmdBar)
}

func (m Model) renderLeftPanel() string {
	var lines []string

	lines = append(lines, titleStyle.Render("镜像信息"))
	lines = append(lines, "")

	if m.memPath != "" {
		lines = append(lines, labelStyle.Render("内存镜像:"))
		lines = append(lines, valueStyle.Render("  "+m.memPath))
	} else {
		lines = append(lines, labelStyle.Render("内存镜像:"))
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
			lines = append(lines, valueStyle.Render(fmt.Sprintf("  发行版: %s", m.imageInfo.Distro)))
		}
		if m.imageInfo.Kernel != "" {
			lines = append(lines, valueStyle.Render(fmt.Sprintf("  内核:   %s", m.imageInfo.Kernel)))
		}
		if m.imageInfo.Arch != "" {
			lines = append(lines, valueStyle.Render(fmt.Sprintf("  架构:   %s", m.imageInfo.Arch)))
		}
	}

	lines = append(lines, "")
	if m.running {
		lines = append(lines, warnStyle.Bold(true).Render("⟳ 运行中..."))
	} else {
		lines = append(lines, mutedStyle.Render("空闲"))
	}

	content := strings.Join(lines, "\n")
	return leftPanelStyle.Render(content)
}

func (m Model) renderCenterPanel() string {
	content := m.logVP.View()
	return centerPanelStyle.Render(content)
}

func (m Model) renderRightPanel() string {
	var lines []string

	lines = append(lines, titleStyle.Render("插件列表"))
	lines = append(lines, mutedStyle.Render(" /plugin <name> 切换"))
	lines = append(lines, "")

	for _, cat := range PluginCategories {
		lines = append(lines, warnStyle.Bold(true).Render(fmt.Sprintf("%s %s", cat.Icon, cat.Name)))
		for _, p := range cat.Plugins {
			if m.plugin == p.Name {
				lines = append(lines, successStyle.Render("▸ ")+successStyle.Bold(true).Render(p.Name))
			} else {
				lines = append(lines, mutedStyle.Render("  ")+mutedStyle.Foreground(brightColor).Render(p.Name))
			}
			lines = append(lines, mutedStyle.Render("    "+p.Description))
		}
		lines = append(lines, "")
	}

	content := strings.Join(lines, "\n")
	return rightPanelStyle.Render(content)
}

func (m Model) renderCommandBar() string {
	borderStyle := inactiveBorder
	hint := mutedStyle.Render("i 聚焦 | r 执行 | q 退出")
	if m.inputFocused {
		borderStyle = activeBorder
		hint = mutedStyle.Render("Esc 取消 | Enter 确认")
	}

	bar := lipgloss.JoinHorizontal(lipgloss.Center,
		lipgloss.NewStyle().Bold(true).Foreground(accentColor).Render(" > "),
		m.input.View(),
		"  ",
		hint,
	)

	return borderStyle.Width(m.width - 2).Render(bar)
}
