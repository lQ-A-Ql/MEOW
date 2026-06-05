package cmd

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"meow/internal/tui"
)

func init() {
	Register("tui", "启动终端交互界面 (TUI)", runTUI)
}

func runTUI(args []string) {
	m := tui.NewModel()
	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseAllMotion())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "TUI error: %v\n", err)
		os.Exit(1)
	}
}
