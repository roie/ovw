package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
)

type Model struct {
	width  int
	height int
}

func New() Model {
	return Model{}
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if isQuitKey(msg.String()) {
			return m, tea.Quit
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	}
	return m, nil
}

func (m Model) View() string {
	return renderShell(m)
}

func (m Model) Size() (int, int) {
	return m.width, m.height
}

func Run() error {
	program := tea.NewProgram(New(), tea.WithAltScreen())
	_, err := program.Run()
	return err
}

func renderShell(m Model) string {
	body := titleStyle.Render("ovw")
	if m.width > 0 && m.height > 0 {
		body += "\n" + mutedStyle.Render(fmt.Sprintf("%dx%d", m.width, m.height))
	}
	body += "\n\n" + mutedStyle.Render("TUI loading...")
	body += "\n\n" + footerView()
	return appStyle.Render(body)
}
