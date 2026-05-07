package tui

import "github.com/charmbracelet/bubbles/key"

type keyMap struct {
	Quit key.Binding
	Help key.Binding
	Up   key.Binding
	Down key.Binding
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
		Up: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("↑/k", "move up"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("↓/j", "move down"),
		),
	}
}

func isQuitKey(value string) bool {
	return value == "q" || value == "ctrl+c"
}

func isUpKey(value string) bool {
	return value == "up" || value == "k"
}

func isDownKey(value string) bool {
	return value == "down" || value == "j"
}

func isEnterKey(value string) bool {
	return value == "enter"
}

func isEscapeKey(value string) bool {
	return value == "esc"
}

func isSearchKey(value string) bool {
	return value == "/"
}

func isFilterKey(value string) bool {
	return value == "f"
}

func isSortKey(value string) bool {
	return value == "s"
}

func isNoteKey(value string) bool {
	return value == "n"
}

func isStatusKey(value string) bool {
	return value == "m"
}

func isReloadKey(value string) bool {
	return value == "r"
}

func isOpenKey(value string) bool {
	return value == "o"
}

func isHelpKey(value string) bool {
	return value == "?"
}

func isBackspaceKey(value string) bool {
	return value == "backspace" || value == "ctrl+h"
}

func footerView() string {
	keys := defaultKeyMap()
	return mutedStyle.Render(keys.Down.Help().Key + " " + keys.Down.Help().Desc + " · / search · f filter · s sort · n note · m status · r reload · o open · ? help · enter details · esc back · " + keys.Quit.Help().Key + " " + keys.Quit.Help().Desc)
}
