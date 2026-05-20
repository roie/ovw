package tui

import (
	"ovw/internal/config"

	"github.com/charmbracelet/bubbles/key"
)

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

func actionKeys(cfg config.Config) config.ActionKeyConfig {
	keys := cfg.Keys.Actions
	defaults := config.Default().Keys.Actions
	if keys.Editor == "" {
		keys.Editor = defaults.Editor
	}
	if keys.Terminal == "" {
		keys.Terminal = defaults.Terminal
	}
	if keys.Runner == "" {
		keys.Runner = defaults.Runner
	}
	if keys.Note == "" {
		keys.Note = defaults.Note
	}
	if keys.Status == "" {
		keys.Status = defaults.Status
	}
	if keys.Pin == "" {
		keys.Pin = defaults.Pin
	}
	if keys.Hide == "" {
		keys.Hide = defaults.Hide
	}
	return keys
}

func (m Model) actionKeys() config.ActionKeyConfig {
	return actionKeys(m.config)
}

func (m Model) isNoteKey(value string) bool {
	return value == m.actionKeys().Note
}

func (m Model) isStatusKey(value string) bool {
	return value == m.actionKeys().Status
}

func (m Model) isPinKey(value string) bool {
	return value == m.actionKeys().Pin
}

func isReloadKey(value string) bool {
	return value == "ctrl+r" || value == "f5"
}

func (m Model) isRunnerKey(value string) bool {
	return value == m.actionKeys().Runner
}

func (m Model) isOpenKey(value string) bool {
	return value == m.actionKeys().Editor
}

func (m Model) isTerminalKey(value string) bool {
	return value == m.actionKeys().Terminal
}

func (m Model) isVisibilityKey(value string) bool {
	return value == m.actionKeys().Hide
}

func isHelpKey(value string) bool {
	return value == "?"
}

func isCommandKey(value string) bool {
	return value == ":" || value == "ctrl+p"
}

func isBackspaceKey(value string) bool {
	return value == "backspace" || value == "ctrl+h"
}

func isDeleteKey(value string) bool {
	return value == "delete" || value == "ctrl+d"
}

func isMoveStartKey(value string) bool {
	return value == "ctrl+a"
}

func isMoveEndKey(value string) bool {
	return value == "ctrl+e"
}

func isClearBeforeKey(value string) bool {
	return value == "ctrl+u"
}

func isClearAfterKey(value string) bool {
	return value == "ctrl+k"
}

func isDeletePreviousWordKey(value string) bool {
	return value == "ctrl+w"
}
