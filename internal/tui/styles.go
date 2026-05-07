package tui

import "github.com/charmbracelet/lipgloss"

var (
	appStyle = lipgloss.NewStyle().
			Padding(1, 2)

	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("86"))

	mutedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("244"))

	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("203"))

	selectedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("229")).
			Background(lipgloss.Color("57"))
)
