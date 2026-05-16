package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

type commandAction struct {
	Label    string
	Shortcut string
	Aliases  []string
	Run      func(Model) (Model, tea.Cmd)
}

func (m *Model) openCommandPalette() {
	m.screen = screenCommand
	m.searching = false
	m.commandInput = ""
	m.commandCursor = 0
	m.commandSelected = 0
}

func (m Model) updateCommand(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch value := msg.String(); {
	case isEscapeKey(value):
		m.screen = screenTable
	case isDownKey(value):
		m.commandSelected = wrapPickerSelection(m.commandSelected, len(m.filteredCommandActions()), 1)
	case isUpKey(value):
		m.commandSelected = wrapPickerSelection(m.commandSelected, len(m.filteredCommandActions()), -1)
	case isEnterKey(value):
		actions := m.filteredCommandActions()
		if len(actions) == 0 {
			m.screen = screenTable
			return m, nil
		}
		if m.commandSelected >= len(actions) {
			m.commandSelected = len(actions) - 1
		}
		return actions[m.commandSelected].Run(m)
	case value == "left":
		m.commandCursor = textMoveLeft(m.commandInput, m.commandCursor)
	case value == "right":
		m.commandCursor = textMoveRight(m.commandInput, m.commandCursor)
	case isMoveStartKey(value):
		m.commandCursor = textMoveStart(m.commandInput, m.commandCursor)
	case isMoveEndKey(value):
		m.commandCursor = textMoveEnd(m.commandInput, m.commandCursor)
	case isClearBeforeKey(value):
		m.commandInput, m.commandCursor = textClearBefore(m.commandInput, m.commandCursor)
		m.commandSelected = 0
	case isClearAfterKey(value):
		m.commandInput, m.commandCursor = textClearAfter(m.commandInput, m.commandCursor)
		m.commandSelected = 0
	case isDeletePreviousWordKey(value):
		m.commandInput, m.commandCursor = textDeletePreviousWord(m.commandInput, m.commandCursor)
		m.commandSelected = 0
	case isBackspaceKey(value):
		m.commandInput, m.commandCursor = textBackspace(m.commandInput, m.commandCursor)
		m.commandSelected = 0
	case isDeleteKey(value):
		m.commandInput, m.commandCursor = textDelete(m.commandInput, m.commandCursor)
		m.commandSelected = 0
	default:
		m.commandInput, m.commandCursor = textInsert(m.commandInput, m.commandCursor, inputText(msg))
		m.commandSelected = 0
	}
	m.clampCommandSelection()
	return m, nil
}

func (m *Model) clampCommandSelection() {
	actions := m.filteredCommandActions()
	if len(actions) == 0 {
		m.commandSelected = 0
		return
	}
	if m.commandSelected >= len(actions) {
		m.commandSelected = len(actions) - 1
	}
	if m.commandSelected < 0 {
		m.commandSelected = 0
	}
}

func (m Model) filteredCommandActions() []commandAction {
	query := strings.ToLower(strings.TrimSpace(m.commandInput))
	actions := m.commandActions()
	if query == "" {
		return actions
	}
	filtered := make([]commandAction, 0, len(actions))
	for _, action := range actions {
		if commandMatches(action, query) {
			filtered = append(filtered, action)
		}
	}
	return filtered
}

func commandMatches(action commandAction, query string) bool {
	if strings.Contains(strings.ToLower(action.Label), query) {
		return true
	}
	for _, alias := range action.Aliases {
		if strings.Contains(strings.ToLower(alias), query) {
			return true
		}
	}
	return false
}

func (m Model) commandActions() []commandAction {
	actions := []commandAction{
		{Label: "Search projects", Shortcut: "/", Aliases: []string{"search", "find"}, Run: func(m Model) (Model, tea.Cmd) {
			m.screen = screenTable
			m.searching = true
			m.searchCursor = textCursor(m.search, m.searchCursor)
			return m, nil
		}},
		{Label: "Open in editor", Shortcut: "o", Aliases: []string{"open", "editor"}, Run: commandOpenSelectedProject},
		{Label: "Open terminal here", Shortcut: "t", Aliases: []string{"terminal", "shell"}, Run: commandOpenSelectedTerminal},
		{Label: "Show details", Shortcut: "enter", Aliases: []string{"details", "detail"}, Run: commandShowDetails},
		{Label: "Edit note", Shortcut: "n", Aliases: []string{"note"}, Run: commandEditNote},
		{Label: "Set status", Shortcut: "m", Aliases: []string{"status"}, Run: commandSetStatus},
		{Label: m.pinCommandLabel(), Shortcut: "p", Aliases: []string{"pin", "unpin"}, Run: commandTogglePin},
		{Label: "Filter projects", Shortcut: "f", Aliases: []string{"filter"}, Run: commandFilterProjects},
		{Label: "Sort projects", Shortcut: "s", Aliases: []string{"sort"}, Run: commandSortProjects},
	}
	if m.canRunSelectedScript() {
		actions = append(actions, commandAction{Label: "Run script", Shortcut: "r", Aliases: []string{"runner", "script", "run"}, Run: commandOpenRunner})
	}
	actions = append(actions, commandAction{Label: "Choose columns", Shortcut: "c", Aliases: []string{"columns"}, Run: func(m Model) (Model, tea.Cmd) {
		m.openColumns()
		return m, nil
	}})
	if !m.loading {
		actions = append(actions, commandAction{Label: "Add project", Shortcut: "a", Aliases: []string{"add"}, Run: commandAddProject})
	}
	actions = append(actions,
		commandAction{Label: "Reload projects", Shortcut: "ctrl+r or F5", Aliases: []string{"reload", "refresh"}, Run: commandReloadProjects},
		commandAction{Label: "Settings", Aliases: []string{"config", "configure", "settings", "preferences"}, Run: func(m Model) (Model, tea.Cmd) {
			m.openConfig()
			return m, nil
		}},
		commandAction{Label: "Show help", Shortcut: "?", Aliases: []string{"help"}, Run: func(m Model) (Model, tea.Cmd) {
			m.screen = screenHelp
			return m, nil
		}},
		commandAction{Label: "Quit", Shortcut: "q", Aliases: []string{"quit", "exit"}, Run: func(m Model) (Model, tea.Cmd) {
			return m, tea.Quit
		}},
	)
	return actions
}

func (m Model) canRunSelectedScript() bool {
	project, ok := m.currentProject()
	return ok && len(runnerScriptOptions(project, "")) > 0
}

func (m Model) pinCommandLabel() string {
	project, ok := m.currentProject()
	if ok && project.Pinned {
		return "Unpin project"
	}
	return "Pin project"
}

func commandAddProject(m Model) (Model, tea.Cmd) {
	if m.loading {
		m.screen = screenTable
		return m, nil
	}
	m.screen = screenAdd
	m.addInput = ""
	m.addCursor = 0
	m.addErr = ""
	return m, m.startInputCursorBlink()
}

func commandOpenSelectedProject(m Model) (Model, tea.Cmd) {
	m.screen = screenTable
	if !m.canOpenDetail() {
		return m, nil
	}
	return m, m.openSelectedProject()
}

func commandOpenSelectedTerminal(m Model) (Model, tea.Cmd) {
	m.screen = screenTable
	if !m.canOpenDetail() {
		return m, nil
	}
	return m, m.openSelectedTerminal()
}

func commandOpenRunner(m Model) (Model, tea.Cmd) {
	if !m.canOpenDetail() {
		m.screen = screenTable
		return m, nil
	}
	return m.openRunner()
}

func commandShowDetails(m Model) (Model, tea.Cmd) {
	if !m.canOpenDetail() {
		m.screen = screenTable
		return m, nil
	}
	m.screen = screenDetail
	m.detailModalY = 0
	m.detailsExpanded = false
	return m, nil
}

func commandEditNote(m Model) (Model, tea.Cmd) {
	project, ok := m.currentProject()
	if !ok {
		m.screen = screenTable
		return m, nil
	}
	m.screen = screenNote
	m.noteInput = project.Note.Value
	m.noteCursor = len([]rune(m.noteInput))
	return m, m.startInputCursorBlink()
}

func commandSetStatus(m Model) (Model, tea.Cmd) {
	project, ok := m.currentProject()
	if !ok {
		m.screen = screenTable
		return m, nil
	}
	m.screen = screenStatus
	m.statusSelected = m.currentStatusIndex(project.Status.Value)
	return m, nil
}

func commandTogglePin(m Model) (Model, tea.Cmd) {
	if !m.canOpenDetail() {
		m.screen = screenTable
		return m, nil
	}
	m.screen = screenTable
	m.loading = true
	return m, m.togglePin()
}

func commandFilterProjects(m Model) (Model, tea.Cmd) {
	m.screen = screenFilter
	m.filterSelected = m.currentFilterIndex()
	return m, nil
}

func commandSortProjects(m Model) (Model, tea.Cmd) {
	m.screen = screenSort
	m.sortSelected = m.currentSortIndex()
	return m, nil
}

func commandReloadProjects(m Model) (Model, tea.Cmd) {
	m.screen = screenTable
	m.loading = true
	return m, m.reloadOverview("", "Reloaded")
}

func commandView(input string, cursor int, actions []commandAction, selected int, cursorState ...inputCursorState) string {
	lines := inputModalLines(input, "type a command...", 52, cursor, cursorState...)
	if len(actions) == 0 {
		lines = append(lines, "", modalMuted("No commands"))
		return modalView("Commands", lines, 56)
	}
	lines = append(lines, commandOptionLines(actions, selected, 50)...)
	return modalView("Commands", lines, 56)
}

func commandOptionLines(actions []commandAction, selected int, width int) []string {
	lines := make([]string, 0, len(actions))
	for index, action := range actions {
		line := commandOptionLine(action, width)
		if index == selected {
			lines = append(lines, modalAccent("> "+line))
			continue
		}
		lines = append(lines, modalMuted("  "+line))
	}
	return lines
}

func commandOptionLine(action commandAction, width int) string {
	if action.Shortcut == "" {
		return action.Label
	}
	gap := width - lipglossWidth(action.Label) - lipglossWidth(action.Shortcut)
	if gap < 2 {
		gap = 2
	}
	return action.Label + strings.Repeat(" ", gap) + action.Shortcut
}
