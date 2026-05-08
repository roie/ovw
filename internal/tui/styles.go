package tui

import "github.com/charmbracelet/lipgloss"

const (
	accentColor              = "75"
	mutedColor               = "244"
	modalSurfaceColor        = "236"
	selectionForegroundColor = "252"
	selectionBackgroundColor = "24"
)

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(accentColor))

	mutedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(mutedColor))

	hintKeyStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("252"))

	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("203"))

	selectedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(selectionForegroundColor)).
			Background(lipgloss.Color(selectionBackgroundColor))
)
