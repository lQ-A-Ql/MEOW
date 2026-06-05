package cmd

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
)

func init() {
	Register("tuitest", "bubbletea 诊断测试", runTUITest)
}

type testModel struct {
	width  int
	height int
	keys   []string
	msgs   []string
}

func (m testModel) Init() tea.Cmd {
	return nil
}

func (m testModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.msgs = append(m.msgs, fmt.Sprintf("WindowSize: %dx%d", msg.Width, msg.Height))
	case tea.KeyMsg:
		m.keys = append(m.keys, msg.String())
		if msg.String() == "q" || msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
	default:
		m.msgs = append(m.msgs, fmt.Sprintf("Other: %T", msg))
	}
	return m, nil
}

func (m testModel) View() string {
	s := fmt.Sprintf("Size: %dx%d\n\n", m.width, m.height)
	s += "Last keys pressed:\n"
	start := 0
	if len(m.keys) > 10 {
		start = len(m.keys) - 10
	}
	for _, k := range m.keys[start:] {
		s += "  " + k + "\n"
	}
	s += "\nMessages:\n"
	start = 0
	if len(m.msgs) > 5 {
		start = len(m.msgs) - 5
	}
	for _, msg := range m.msgs[start:] {
		s += "  " + msg + "\n"
	}
	s += "\nPress any key (q to quit)"
	return s
}

func runTUITest(args []string) {
	p := tea.NewProgram(testModel{}, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
