package tui

import (
	"strings"

	"ovw/internal/buildinfo"
)

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

func isLeftKey(value string) bool {
	return value == "left" || value == "h"
}

func isRightKey(value string) bool {
	return value == "right" || value == "l"
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

func isAddKey(value string) bool {
	return value == "a"
}

func isFilterKey(value string) bool {
	return value == "f"
}

func isSortKey(value string) bool {
	return value == "s"
}

func isColumnsKey(value string) bool {
	return value == "c"
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

func isTerminalKey(value string) bool {
	return value == "t"
}

func isVisibilityKey(value string) bool {
	return value == "x"
}

func isHelpKey(value string) bool {
	return value == "?"
}

func isBackspaceKey(value string) bool {
	return value == "backspace" || value == "ctrl+h"
}

func isDeleteKey(value string) bool {
	return value == "delete" || value == "ctrl+d"
}

func footerView(width int) string {
	keys := defaultKeyMap()
	left := "↑↓ move · ←→ scroll · / search · f filter · s sort · enter details · n note · m status · r reload · o open · t terminal · esc back · " + keys.Help.Help().Key + " " + keys.Help.Help().Desc + " · " + keys.Quit.Help().Key + " " + keys.Quit.Help().Desc
	withColumns := "↑↓ move · ←→ scroll · / search · f filter · s sort · c columns · enter details · n note · m status · r reload · o open · t terminal · esc back · " + keys.Help.Help().Key + " " + keys.Help.Help().Desc + " · " + keys.Quit.Help().Key + " " + keys.Quit.Help().Desc
	right := "ovw " + buildinfo.Version
	if width <= 0 {
		return mutedStyle.Render(left)
	}
	if lipglossWidth(withColumns)+lipglossWidth(right)+2 <= width {
		left = withColumns
	}
	if lipglossWidth(left)+lipglossWidth(right)+2 > width {
		return mutedStyle.Render(left)
	}
	gap := width - lipglossWidth(left) - lipglossWidth(right)
	return mutedStyle.Render(left + strings.Repeat(" ", gap) + right)
}
