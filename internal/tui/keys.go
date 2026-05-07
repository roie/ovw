package tui

import "github.com/charmbracelet/bubbles/key"

type keyMap struct {
	Quit key.Binding
	Help key.Binding
}

func defaultKeyMap() keyMap {
	return keyMap{
		Quit: key.NewBinding(
			key.WithKeys("q", "ctrl+c"),
			key.WithHelp("q", "quit"),
		),
		Help: key.NewBinding(
			key.WithKeys("?"),
			key.WithHelp("?", "help"),
		),
	}
}

func isQuitKey(value string) bool {
	return value == "q" || value == "ctrl+c"
}

func footerView() string {
	keys := defaultKeyMap()
	return mutedStyle.Render(keys.Quit.Help().Key + " " + keys.Quit.Help().Desc)
}
