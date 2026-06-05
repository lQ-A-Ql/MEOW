package tui

import "github.com/charmbracelet/lipgloss"

var (
	// Colors
	bgColor       = lipgloss.Color("#0F172A")
	borderColor   = lipgloss.Color("#334155")
	accentColor   = lipgloss.Color("#60A5FA")
	accentColor2  = lipgloss.Color("#A78BFA")
	successColor  = lipgloss.Color("#34D399")
	warnColor     = lipgloss.Color("#FBBF24")
	errorColor    = lipgloss.Color("#EF4444")
	mutedColor    = lipgloss.Color("#6B7280")
	textColor     = lipgloss.Color("#D1D5DB")
	brightColor   = lipgloss.Color("#E5E7EB")
	stdoutColor   = lipgloss.Color("#93C5FD")
	stderrColor   = lipgloss.Color("#FCA5A5")

	// Panel styles
	panelStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(borderColor).
			Padding(0, 1)

	leftPanelStyle = panelStyle.Width(28)

	centerPanelStyle = panelStyle

	rightPanelStyle = panelStyle.Width(36)

	// Text styles
	titleStyle = lipgloss.NewStyle().Bold(true).Foreground(accentColor)
	labelStyle = lipgloss.NewStyle().Foreground(mutedColor)
	valueStyle = lipgloss.NewStyle().Foreground(brightColor)
	mutedStyle = lipgloss.NewStyle().Foreground(mutedColor)

	successStyle = lipgloss.NewStyle().Foreground(successColor)
	warnStyle    = lipgloss.NewStyle().Foreground(warnColor)
	errorStyle   = lipgloss.NewStyle().Foreground(errorColor)
	stdoutStyle  = lipgloss.NewStyle().Foreground(stdoutColor)
	stderrStyle  = lipgloss.NewStyle().Foreground(stderrColor)

	// Active input border
	activeBorder = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(accentColor)

	inactiveBorder = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(borderColor)

	// Logo
	logoStyle = lipgloss.NewStyle().Bold(true).Foreground(accentColor)
)

func logColor(level LogLevel) lipgloss.Style {
	switch level {
	case "info":
		return lipgloss.NewStyle().Foreground(textColor)
	case "success":
		return successStyle
	case "warn":
		return warnStyle
	case "error":
		return errorStyle
	case "stdout":
		return stdoutStyle
	case "stderr":
		return stderrStyle
	default:
		return lipgloss.NewStyle().Foreground(textColor)
	}
}
