package tui

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"ovw/internal/app"
	"ovw/internal/config"

	tea "github.com/charmbracelet/bubbletea"
)

type configListField int

const (
	configListNone configListField = iota
	configListRoots
	configListIgnoreDirs
)

type configInputKind int

const (
	configInputNone configInputKind = iota
	configInputStaleDays
	configInputEditor
	configInputShell
	configInputKeyEditor
	configInputKeyTerminal
	configInputKeyRunner
	configInputKeyNote
	configInputKeyStatus
	configInputKeyPin
	configInputKeyHide
)

type configSavedMsg struct {
	result       app.OverviewResult
	preservePath string
	err          error
}

type configOpenedMsg struct {
	result app.OverviewResult
	err    error
}

type configRootCandidatesMsg struct {
	candidates []string
}

type configRootCountsMsg struct {
	counts map[string]int
}

type configRow struct {
	Label string
	Value string
}

type configSortPreset struct {
	By  string
	Dir string
}

type configNoteRow struct {
	Label   string
	Checked bool
}

type configKeyRow struct {
	Label string
	Value string
	Kind  configInputKind
}

func (m *Model) openConfig() {
	m.screen = screenConfig
	m.configDraft = m.config
	m.configSelected = 0
	m.configErr = ""
	m.configListField = configListNone
	m.configListSel = 0
	m.configListInput = ""
	m.configListCursor = 0
	m.configListEditing = false
	m.configInputKind = configInputNone
	m.configInput = ""
	m.configCursor = 0
	m.configRootPicker = rootPicker{}
	m.configRootInput = ""
	m.configRootCursor = 0
	m.configNoteSel = 0
	m.configKeySel = 0
}

func (m Model) configRows() []configRow {
	cfg := m.configDraft
	return []configRow{
		{Label: "Project folders", Value: listSummary(cfg.Roots)},
		{Label: "Ignored folders", Value: listSummary(cfg.IgnoreDirs)},
		{Label: "Include nested projects", Value: boolSummary(cfg.ScanNestedProjects)},
		{Label: "Show unpushed commits", Value: boolSummary(cfg.ShowUnpushed)},
		{Label: "Mark stale after", Value: daysSummary(cfg.StaleDays)},
		{Label: "Note display", Value: noteDisplaySummary(cfg)},
		{Label: "Keyboard shortcuts", Value: actionKeysSummary(cfg.Keys.Actions)},
		{Label: "Editor", Value: emptySummary(cfg.Editor)},
		{Label: "Terminal", Value: terminalSummary(cfg.Shell)},
		{Label: "Default sort", Value: cfg.SortBy + " " + cfg.SortDir},
		{Label: "Open settings file", Value: rawConfigPath(m.configPaths.Config)},
		{Label: "Reset settings", Value: ""},
	}
}

func actionKeysSummary(keys config.ActionKeyConfig) string {
	keys = actionKeys(config.Config{Keys: config.KeyConfig{Actions: keys}})
	return fmt.Sprintf("%s/%s/%s", keys.Editor, keys.Terminal, keys.Runner)
}

func listSummary(values []string) string {
	if len(values) == 0 {
		return "empty"
	}
	if len(values) == 1 {
		return values[0]
	}
	return fmt.Sprintf("%s +%d", values[0], len(values)-1)
}

func boolSummary(value bool) string {
	if value {
		return "on"
	}
	return "off"
}

func daysSummary(days int) string {
	if days == 1 {
		return "1 day"
	}
	return fmt.Sprintf("%d days", days)
}

func noteDisplaySummary(cfg config.Config) string {
	switch {
	case cfg.NoteShowBranch && cfg.NoteFallbackCommit && cfg.NoteFallbackDescription:
		return "branch + smart fallback"
	case !cfg.NoteShowBranch && cfg.NoteFallbackCommit && cfg.NoteFallbackDescription:
		return "smart fallback"
	case cfg.NoteShowBranch && !cfg.NoteFallbackCommit && !cfg.NoteFallbackDescription:
		return "branch only"
	case !cfg.NoteShowBranch && !cfg.NoteFallbackCommit && !cfg.NoteFallbackDescription:
		return "manual only"
	case cfg.NoteShowBranch && !cfg.NoteFallbackCommit && cfg.NoteFallbackDescription:
		return "branch + description fallback"
	case !cfg.NoteShowBranch && !cfg.NoteFallbackCommit && cfg.NoteFallbackDescription:
		return "description fallback"
	case cfg.NoteShowBranch && cfg.NoteFallbackCommit && !cfg.NoteFallbackDescription:
		return "branch + commit fallback"
	case !cfg.NoteShowBranch && cfg.NoteFallbackCommit && !cfg.NoteFallbackDescription:
		return "commit fallback"
	default:
		return "custom"
	}
}

func emptySummary(value string) string {
	if strings.TrimSpace(value) == "" {
		return "empty"
	}
	return value
}

func terminalSummary(value string) string {
	if strings.TrimSpace(value) == "" {
		return "default"
	}
	return value
}

func rawConfigPath(path string) string {
	if path == "" {
		return "config.toml"
	}
	return path
}

func (m Model) updateConfig(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	rows := m.configRows()
	switch value := msg.String(); {
	case isEscapeKey(value):
		m.screen = screenTable
		m.configErr = ""
	case isDownKey(value):
		m.configSelected = wrapPickerSelection(m.configSelected, len(rows), 1)
	case isUpKey(value):
		m.configSelected = wrapPickerSelection(m.configSelected, len(rows), -1)
	case isLeftKey(value):
		m.changeConfigRow()
	case isRightKey(value):
		m.incrementConfigRow()
	case value == "s":
		if err := config.Validate(m.configDraft); err != nil {
			m.configErr = err.Error()
			return m, nil
		}
		m.loading = true
		return m, m.saveConfig()
	case isEnterKey(value):
		return m.editConfigRow()
	}
	return m, nil
}

func (m Model) editConfigRow() (tea.Model, tea.Cmd) {
	switch m.configSelected {
	case 0:
		m.openConfigRoots()
		return m, tea.Batch(m.loadConfigRootCandidates(), m.countConfigRootPaths(m.configRootPicker.visiblePaths()))
	case 1:
		m.openConfigList(configListIgnoreDirs)
	case 4:
		return m.openConfigInput(configInputStaleDays, strconv.Itoa(m.configDraft.StaleDays)), m.startInputCursorBlink()
	case 5:
		m.openConfigNote()
	case 6:
		m.openConfigKeys()
	case 7:
		return m.openConfigInput(configInputEditor, m.configDraft.Editor), m.startInputCursorBlink()
	case 8:
		return m.openConfigInput(configInputShell, m.configDraft.Shell), m.startInputCursorBlink()
	case 10:
		m.loading = true
		return m, m.openConfigFile()
	case 11:
		roots := append([]string{}, m.configDraft.Roots...)
		m.configDraft = config.Default()
		m.configDraft.Roots = roots
		m.configErr = ""
	}
	m.configErr = ""
	return m, nil
}

func (m *Model) changeConfigRow() {
	switch m.configSelected {
	case 0:
		m.openConfigRoots()
	case 1:
		m.openConfigList(configListIgnoreDirs)
	case 4:
		if m.configDraft.StaleDays > 1 {
			m.configDraft.StaleDays--
		}
	case 2:
		m.configDraft.ScanNestedProjects = !m.configDraft.ScanNestedProjects
	case 3:
		m.configDraft.ShowUnpushed = !m.configDraft.ShowUnpushed
	case 5:
		m.openConfigNote()
	case 6:
		m.openConfigKeys()
	case 9:
		m.cycleConfigSort(-1)
	}
	m.configErr = ""
}

func (m *Model) incrementConfigRow() {
	switch m.configSelected {
	case 0:
		m.openConfigRoots()
		return
	case 1:
		m.openConfigList(configListIgnoreDirs)
		return
	case 4:
		m.configDraft.StaleDays++
	case 5:
		m.openConfigNote()
		return
	case 6:
		m.openConfigKeys()
		return
	case 9:
		m.cycleConfigSort(1)
		return
	default:
		m.changeConfigRow()
	}
}

func (m *Model) openConfigNote() {
	m.screen = screenConfigNote
	m.configNoteSel = 0
	m.configErr = ""
}

func (m *Model) openConfigKeys() {
	m.screen = screenConfigKeys
	m.configKeySel = 0
	m.configErr = ""
}

func (m *Model) openConfigList(field configListField) {
	m.screen = screenConfigList
	m.configListField = field
	m.configListSel = 0
	m.configListInput = ""
	m.configListCursor = 0
	m.configListEditing = false
	m.configErr = ""
}

func (m *Model) openConfigRoots() {
	m.screen = screenConfigRoots
	m.configErr = ""
	if len(m.configRootPicker.options) == 0 {
		m.configRootPicker = newRootPicker(m.configDraft.Roots, m.configDraft.Roots)
		return
	}
	m.configRootPicker = newRootPicker(m.configRootPicker.options, m.configDraft.Roots)
}

func (m Model) loadConfigRootCandidates() tea.Cmd {
	cwd := m.request.Cwd
	return func() tea.Msg {
		candidates, err := config.RootCandidates(cwd)
		if err != nil {
			return configRootCandidatesMsg{candidates: append([]string{}, m.configDraft.Roots...)}
		}
		return configRootCandidatesMsg{candidates: candidates}
	}
}

func (m *Model) applyConfigRootCandidates(candidates []string) {
	options := rootPickerOptionsWithRoots(candidates, m.configDraft.Roots)
	m.configRootPicker = newRootPicker(options, m.configDraft.Roots)
}

func (m Model) openConfigInput(kind configInputKind, value string) Model {
	m.screen = screenConfigInput
	m.configInputKind = kind
	m.configInput = value
	m.configCursor = len([]rune(value))
	m.configErr = ""
	return m
}

func configSortPresets() []configSortPreset {
	return configSortPresetsFor(config.Default())
}

func configSortPresetsFor(cfg config.Config) []configSortPreset {
	options := sortOptions(cfg)
	presets := make([]configSortPreset, 0, len(options)*2)
	for _, option := range options {
		presets = append(presets,
			configSortPreset{By: option.Value, Dir: "desc"},
			configSortPreset{By: option.Value, Dir: "asc"},
		)
	}
	return presets
}

func (m *Model) cycleConfigSort(delta int) {
	presets := configSortPresetsFor(m.configDraft)
	if len(presets) == 0 {
		return
	}
	selected := 0
	for index, preset := range presets {
		if m.configDraft.SortBy == preset.By && m.configDraft.SortDir == preset.Dir {
			selected = index
			break
		}
	}
	next := wrapPickerSelection(selected, len(presets), delta)
	m.configDraft.SortBy = presets[next].By
	m.configDraft.SortDir = presets[next].Dir
}

func (m Model) updateConfigList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	values := m.configListValues()
	if (m.configListInput != "" || m.configListEditing) && (msg.Type == tea.KeyRunes || msg.Type == tea.KeySpace) {
		m.configListInput, m.configListCursor = textInsert(m.configListInput, m.configListCursor, inputText(msg))
		return m, nil
	}
	switch value := msg.String(); {
	case isEscapeKey(value):
		m.screen = screenConfig
		m.configErr = ""
	case value == "s":
		if m.configListInput == "" && !m.configListEditing {
			m.screen = screenConfig
			m.configErr = ""
			return m, nil
		}
		m.configListInput, m.configListCursor = textInsert(m.configListInput, m.configListCursor, inputText(msg))
	case isDownKey(value):
		m.configListSel = wrapPickerSelection(m.configListSel, len(values), 1)
	case isUpKey(value):
		m.configListSel = wrapPickerSelection(m.configListSel, len(values), -1)
	case value == " ":
		if m.configListInput != "" || m.configListEditing {
			m.configListInput, m.configListCursor = textInsert(m.configListInput, m.configListCursor, " ")
			return m, nil
		}
		if len(values) == 0 {
			return m, nil
		}
		m.configListSel = clampIndex(m.configListSel, len(values))
		m.configListInput = values[m.configListSel]
		m.configListCursor = len([]rune(m.configListInput))
		m.configListEditing = true
	case isEnterKey(value):
		return m.saveConfigListInput(), nil
	case value == "d":
		if m.configListInput != "" || m.configListEditing {
			m.configListInput, m.configListCursor = textInsert(m.configListInput, m.configListCursor, inputText(msg))
			return m, nil
		}
		if len(values) > 0 {
			m.setConfigListValues(deleteStringAt(values, clampIndex(m.configListSel, len(values))))
			m.configListSel = clampIndex(m.configListSel, len(m.configListValues()))
		}
	case isLeftKey(value):
		if m.configListInput != "" || m.configListEditing {
			m.configListCursor = textMoveLeft(m.configListInput, m.configListCursor)
		} else {
			m.moveConfigListItem(-1)
		}
	case isRightKey(value):
		if m.configListInput != "" || m.configListEditing {
			m.configListCursor = textMoveRight(m.configListInput, m.configListCursor)
		} else {
			m.moveConfigListItem(1)
		}
	case value == "left":
		m.configListCursor = textMoveLeft(m.configListInput, m.configListCursor)
	case value == "right":
		m.configListCursor = textMoveRight(m.configListInput, m.configListCursor)
	case isMoveStartKey(value):
		m.configListCursor = textMoveStart(m.configListInput, m.configListCursor)
	case isMoveEndKey(value):
		m.configListCursor = textMoveEnd(m.configListInput, m.configListCursor)
	case isClearBeforeKey(value):
		m.configListInput, m.configListCursor = textClearBefore(m.configListInput, m.configListCursor)
	case isClearAfterKey(value):
		m.configListInput, m.configListCursor = textClearAfter(m.configListInput, m.configListCursor)
	case isDeletePreviousWordKey(value):
		m.configListInput, m.configListCursor = textDeletePreviousWord(m.configListInput, m.configListCursor)
	case isBackspaceKey(value):
		m.configListInput, m.configListCursor = textBackspace(m.configListInput, m.configListCursor)
	case isDeleteKey(value):
		m.configListInput, m.configListCursor = textDelete(m.configListInput, m.configListCursor)
	default:
		m.configListInput, m.configListCursor = textInsert(m.configListInput, m.configListCursor, inputText(msg))
		m.configListEditing = false
	}
	return m, nil
}

func (m Model) updateConfigInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch value := msg.String(); {
	case isEscapeKey(value):
		if isConfigKeyInput(m.configInputKind) {
			m.screen = screenConfigKeys
		} else {
			m.screen = screenConfig
		}
		m.configErr = ""
	case isEnterKey(value):
		return m.saveConfigInput()
	case value == "left":
		m.configCursor = textMoveLeft(m.configInput, m.configCursor)
	case value == "right":
		m.configCursor = textMoveRight(m.configInput, m.configCursor)
	case isMoveStartKey(value):
		m.configCursor = textMoveStart(m.configInput, m.configCursor)
	case isMoveEndKey(value):
		m.configCursor = textMoveEnd(m.configInput, m.configCursor)
	case isClearBeforeKey(value):
		m.configInput, m.configCursor = textClearBefore(m.configInput, m.configCursor)
	case isClearAfterKey(value):
		m.configInput, m.configCursor = textClearAfter(m.configInput, m.configCursor)
	case isDeletePreviousWordKey(value):
		m.configInput, m.configCursor = textDeletePreviousWord(m.configInput, m.configCursor)
	case isBackspaceKey(value):
		m.configInput, m.configCursor = textBackspace(m.configInput, m.configCursor)
	case isDeleteKey(value):
		m.configInput, m.configCursor = textDelete(m.configInput, m.configCursor)
	default:
		m.configInput, m.configCursor = textInsert(m.configInput, m.configCursor, inputText(msg))
	}
	return m, nil
}

func isConfigKeyInput(kind configInputKind) bool {
	switch kind {
	case configInputKeyEditor, configInputKeyTerminal, configInputKeyRunner, configInputKeyNote, configInputKeyStatus, configInputKeyPin, configInputKeyHide:
		return true
	default:
		return false
	}
}

func (m Model) configNoteRows() []configNoteRow {
	return []configNoteRow{
		{Label: "Show branch", Checked: m.configDraft.NoteShowBranch},
		{Label: "Use latest commit when note is empty", Checked: m.configDraft.NoteFallbackCommit},
		{Label: "Use project description when note and commit are empty", Checked: m.configDraft.NoteFallbackDescription},
	}
}

func (m Model) updateConfigNote(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	rows := m.configNoteRows()
	switch value := msg.String(); {
	case isEscapeKey(value), isEnterKey(value), value == "s":
		m.screen = screenConfig
		m.configErr = ""
	case isDownKey(value):
		m.configNoteSel = wrapPickerSelection(m.configNoteSel, len(rows), 1)
	case isUpKey(value):
		m.configNoteSel = wrapPickerSelection(m.configNoteSel, len(rows), -1)
	case value == " ", isLeftKey(value), isRightKey(value):
		switch clampIndex(m.configNoteSel, len(rows)) {
		case 0:
			m.configDraft.NoteShowBranch = !m.configDraft.NoteShowBranch
		case 1:
			m.configDraft.NoteFallbackCommit = !m.configDraft.NoteFallbackCommit
		case 2:
			m.configDraft.NoteFallbackDescription = !m.configDraft.NoteFallbackDescription
		}
	}
	return m, nil
}

func (m Model) configKeyRows() []configKeyRow {
	keys := actionKeys(m.configDraft)
	return []configKeyRow{
		{Label: "Open editor", Value: keys.Editor, Kind: configInputKeyEditor},
		{Label: "Open terminal", Value: keys.Terminal, Kind: configInputKeyTerminal},
		{Label: "Run script", Value: keys.Runner, Kind: configInputKeyRunner},
		{Label: "Edit note", Value: keys.Note, Kind: configInputKeyNote},
		{Label: "Set status", Value: keys.Status, Kind: configInputKeyStatus},
		{Label: "Pin project", Value: keys.Pin, Kind: configInputKeyPin},
		{Label: "Hide project", Value: keys.Hide, Kind: configInputKeyHide},
	}
}

func (m Model) updateConfigKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	rows := m.configKeyRows()
	switch value := msg.String(); {
	case isEscapeKey(value), value == "s":
		m.screen = screenConfig
		m.configErr = ""
	case isDownKey(value):
		m.configKeySel = wrapPickerSelection(m.configKeySel, len(rows), 1)
	case isUpKey(value):
		m.configKeySel = wrapPickerSelection(m.configKeySel, len(rows), -1)
	case isEnterKey(value), value == " ":
		row := rows[clampIndex(m.configKeySel, len(rows))]
		return m.openConfigInput(row.Kind, row.Value), m.startInputCursorBlink()
	}
	return m, nil
}

func (m Model) updateConfigRoots(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	rows := m.configRootPicker.visibleRows()
	switch value := msg.String(); {
	case isEscapeKey(value), value == "s":
		m.configDraft.Roots = m.configRootPicker.selectedRoots()
		m.screen = screenConfig
		m.configErr = ""
	case isDownKey(value):
		m.configRootPicker.selected = wrapPickerSelection(m.configRootPicker.selected, len(rows)+1, 1)
	case isUpKey(value):
		m.configRootPicker.selected = wrapPickerSelection(m.configRootPicker.selected, len(rows)+1, -1)
	case isRightKey(value):
		if m.configRootPicker.selected < len(rows) {
			children := m.configRootPicker.expandPath(rows[m.configRootPicker.selected].Path)
			return m, m.countConfigRootPaths(children)
		}
	case isLeftKey(value):
		if m.configRootPicker.selected < len(rows) {
			m.configRootPicker.collapseOrSelectParent(rows[m.configRootPicker.selected])
		}
	case value == " ":
		if m.configRootPicker.selected < len(rows) {
			m.configRootPicker.toggleRow(rows[m.configRootPicker.selected])
		}
	case isEnterKey(value):
		if m.configRootPicker.selected >= len(rows) {
			m.screen = screenConfigRootsInput
			m.configRootInput = ""
			m.configRootCursor = 0
			m.configErr = ""
			return m, m.startInputCursorBlink()
		}
		m.configDraft.Roots = m.configRootPicker.selectedRoots()
		m.screen = screenConfig
		m.configErr = ""
	}
	return m, nil
}

func (m Model) updateConfigRootsInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch value := msg.String(); {
	case isEscapeKey(value):
		m.screen = screenConfigRoots
		m.configErr = ""
	case isEnterKey(value):
		root := strings.TrimSpace(m.configRootInput)
		if root == "" {
			m.screen = screenConfigRoots
			return m, nil
		}
		if m.configRootPicker.checked == nil {
			m.configRootPicker.checked = map[string]bool{}
		}
		m.configRootPicker.checked[root] = true
		m.configRootPicker.addOption(root)
		m.configRootInput = ""
		m.configRootCursor = 0
		m.screen = screenConfigRoots
		m.configRootPicker.selectPath(root)
	case value == "left":
		m.configRootCursor = textMoveLeft(m.configRootInput, m.configRootCursor)
	case value == "right":
		m.configRootCursor = textMoveRight(m.configRootInput, m.configRootCursor)
	case isMoveStartKey(value):
		m.configRootCursor = textMoveStart(m.configRootInput, m.configRootCursor)
	case isMoveEndKey(value):
		m.configRootCursor = textMoveEnd(m.configRootInput, m.configRootCursor)
	case isClearBeforeKey(value):
		m.configRootInput, m.configRootCursor = textClearBefore(m.configRootInput, m.configRootCursor)
	case isClearAfterKey(value):
		m.configRootInput, m.configRootCursor = textClearAfter(m.configRootInput, m.configRootCursor)
	case isDeletePreviousWordKey(value):
		m.configRootInput, m.configRootCursor = textDeletePreviousWord(m.configRootInput, m.configRootCursor)
	case isBackspaceKey(value):
		m.configRootInput, m.configRootCursor = textBackspace(m.configRootInput, m.configRootCursor)
	case isDeleteKey(value):
		m.configRootInput, m.configRootCursor = textDelete(m.configRootInput, m.configRootCursor)
	default:
		m.configRootInput, m.configRootCursor = textInsert(m.configRootInput, m.configRootCursor, inputText(msg))
	}
	return m, nil
}

func (m Model) saveConfigInput() (tea.Model, tea.Cmd) {
	value := strings.TrimSpace(m.configInput)
	switch m.configInputKind {
	case configInputStaleDays:
		days, err := strconv.Atoi(value)
		if err != nil || days <= 0 {
			m.configErr = "stale days must be a positive number"
			return m, nil
		}
		m.configDraft.StaleDays = days
		m.screen = screenConfig
	case configInputEditor:
		m.configDraft.Editor = value
		m.screen = screenConfig
	case configInputShell:
		m.configDraft.Shell = value
		m.screen = screenConfig
	case configInputKeyEditor, configInputKeyTerminal, configInputKeyRunner, configInputKeyNote, configInputKeyStatus, configInputKeyPin, configInputKeyHide:
		next := m.configDraft
		setActionKey(&next.Keys.Actions, m.configInputKind, value)
		if err := config.Validate(next); err != nil {
			m.configErr = err.Error()
			return m, nil
		}
		m.configDraft = next
		m.screen = screenConfigKeys
	}
	m.configErr = ""
	return m, nil
}

func setActionKey(keys *config.ActionKeyConfig, kind configInputKind, value string) {
	switch kind {
	case configInputKeyEditor:
		keys.Editor = value
	case configInputKeyTerminal:
		keys.Terminal = value
	case configInputKeyRunner:
		keys.Runner = value
	case configInputKeyNote:
		keys.Note = value
	case configInputKeyStatus:
		keys.Status = value
	case configInputKeyPin:
		keys.Pin = value
	case configInputKeyHide:
		keys.Hide = value
	}
}

func (m Model) saveConfigListInput() Model {
	value := strings.TrimSpace(m.configListInput)
	if value == "" {
		m.configErr = "value cannot be empty"
		return m
	}
	values := append([]string{}, m.configListValues()...)
	if m.configListEditing && len(values) > 0 {
		index := clampIndex(m.configListSel, len(values))
		values[index] = value
		m.configListSel = index
	} else {
		values = append(values, value)
		m.configListSel = len(values) - 1
	}
	m.setConfigListValues(values)
	m.configListInput = ""
	m.configListCursor = 0
	m.configListEditing = false
	m.configErr = ""
	return m
}

func (m Model) configListTitle() string {
	switch m.configListField {
	case configListRoots:
		return "Project folders"
	case configListIgnoreDirs:
		return "Ignored folders"
	default:
		return "Config list"
	}
}

func (m Model) configListValues() []string {
	switch m.configListField {
	case configListRoots:
		return m.configDraft.Roots
	case configListIgnoreDirs:
		return m.configDraft.IgnoreDirs
	default:
		return nil
	}
}

func (m *Model) setConfigListValues(values []string) {
	switch m.configListField {
	case configListRoots:
		m.configDraft.Roots = values
	case configListIgnoreDirs:
		m.configDraft.IgnoreDirs = values
	}
}

func (m Model) configListSelected() int {
	return clampIndex(m.configListSel, len(m.configListValues()))
}

func (m *Model) moveConfigListItem(delta int) {
	values := append([]string{}, m.configListValues()...)
	if len(values) == 0 {
		return
	}
	index := clampIndex(m.configListSel, len(values))
	next := index + delta
	if next < 0 || next >= len(values) {
		return
	}
	values[index], values[next] = values[next], values[index]
	m.configListSel = next
	m.setConfigListValues(values)
}

func (m Model) countConfigRootPaths(paths []string) tea.Cmd {
	missing := m.configRootPicker.missingCountPaths(paths)
	if len(missing) == 0 {
		return nil
	}
	counter := func(paths []string) map[string]int {
		return app.CountProjectRootsWithConfig(paths, m.configDraft)
	}
	return func() tea.Msg {
		return configRootCountsMsg{counts: counter(missing)}
	}
}

func deleteStringAt(values []string, index int) []string {
	if len(values) == 0 {
		return values
	}
	index = clampIndex(index, len(values))
	next := append([]string{}, values[:index]...)
	next = append(next, values[index+1:]...)
	return next
}

func clampIndex(index, length int) int {
	if length <= 0 {
		return 0
	}
	if index < 0 {
		return 0
	}
	if index >= length {
		return length - 1
	}
	return index
}

func (m Model) configInputTitle() string {
	switch m.configInputKind {
	case configInputStaleDays:
		return "Mark stale after"
	case configInputEditor:
		return "Editor"
	case configInputShell:
		return "Terminal"
	case configInputKeyEditor:
		return "Open editor shortcut"
	case configInputKeyTerminal:
		return "Open terminal shortcut"
	case configInputKeyRunner:
		return "Run script shortcut"
	case configInputKeyNote:
		return "Edit note shortcut"
	case configInputKeyStatus:
		return "Set status shortcut"
	case configInputKeyPin:
		return "Pin project shortcut"
	case configInputKeyHide:
		return "Hide project shortcut"
	default:
		return "Config"
	}
}

func (m Model) configInputPlaceholder() string {
	switch m.configInputKind {
	case configInputStaleDays:
		return "30"
	case configInputEditor:
		return "code"
	case configInputShell:
		return "empty uses default terminal"
	case configInputKeyEditor:
		return "o"
	case configInputKeyTerminal:
		return "t"
	case configInputKeyRunner:
		return "r"
	case configInputKeyNote:
		return "n"
	case configInputKeyStatus:
		return "m"
	case configInputKeyPin:
		return "p"
	case configInputKeyHide:
		return "x"
	default:
		return ""
	}
}

func (m Model) saveConfig() tea.Cmd {
	cfg := m.configDraft
	path := m.configPaths.Config
	writer := m.configWriter
	if writer == nil {
		writer = config.Write
	}
	preservePath := m.selectedProjectPath()
	return func() tea.Msg {
		if path == "" {
			paths, err := config.Paths()
			if err != nil {
				return configSavedMsg{err: err}
			}
			path = paths.Config
		}
		if err := writer(path, cfg); err != nil {
			return configSavedMsg{err: err}
		}
		loader := m.loader
		if loader == nil {
			loader = app.LoadOverview
		}
		result, err := loader(m.request)
		if err != nil {
			return configSavedMsg{err: err}
		}
		return configSavedMsg{result: result, preservePath: preservePath}
	}
}

func (m Model) openConfigFile() tea.Cmd {
	path := m.configPaths.Config
	editor := m.config.Editor
	runEditor := m.editor
	if runEditor == nil {
		runEditor = OpenEditor
	}
	return func() tea.Msg {
		if path == "" {
			paths, err := config.Paths()
			if err != nil {
				return configOpenedMsg{err: err}
			}
			path = paths.Config
		}
		if strings.TrimSpace(path) == "" {
			return configOpenedMsg{err: errors.New("config path is empty")}
		}
		if err := runEditor(editor, path); err != nil {
			return configOpenedMsg{err: err}
		}
		loader := m.loader
		if loader == nil {
			loader = app.LoadOverview
		}
		result, err := loader(m.request)
		if err != nil {
			return configOpenedMsg{err: err}
		}
		return configOpenedMsg{result: result}
	}
}

func configView(rows []configRow, selected int, errText string) string {
	labels := make([]string, 0, len(rows))
	for _, row := range rows {
		value := row.Value
		if value != "" {
			value = modalMuted(value)
		}
		labels = append(labels, row.Label+"  "+value)
	}
	lines := modalOptionLines(labels, selected)
	if errText != "" {
		lines = append(lines, "", errorStyle.Render(errText))
	}
	lines = append(lines, "", actionHint("enter", "edit")+" · "+actionHint("←→", "change")+" · "+actionHint("s", "save"))
	return modalView("Settings", lines, 64)
}

func configListView(title string, values []string, selected int, input string, cursor int, editing bool, errText string, cursorState ...inputCursorState) string {
	placeholder := "type to add..."
	if editing {
		placeholder = "edit selected..."
	}
	lines := inputModalLines(input, placeholder, 56, cursor, cursorState...)
	lines = append(lines, "")
	if len(values) == 0 {
		lines = append(lines, modalMuted("No values yet"))
	} else {
		lines = append(lines, modalOptionLines(values, selected)...)
	}
	if errText != "" {
		lines = append(lines, "", errorStyle.Render(errText))
	}
	lines = append(lines, "", actionHint("enter", "add/save")+" · "+actionHint("space", "edit")+" · "+actionHint("d", "delete")+" · "+actionHint("←→", "reorder")+" · "+actionHint("s", "done"))
	return modalView(title, lines, 64)
}

func configInputView(title, value, placeholder string, cursor int, errText string, cursorState ...inputCursorState) string {
	lines := inputModalLines(value, placeholder, 56, cursor, cursorState...)
	if errText != "" {
		lines = append(lines, "", errorStyle.Render(errText))
	}
	lines = append(lines, "", actionHint("enter", "save"))
	return modalView(title, lines, 60)
}

func configNoteView(rows []configNoteRow, selected int, errText string) string {
	labels := make([]string, 0, len(rows))
	for _, row := range rows {
		box := "[ ]"
		if row.Checked {
			box = "[x]"
		}
		labels = append(labels, box+" "+row.Label)
	}
	lines := modalOptionLines(labels, selected)
	if errText != "" {
		lines = append(lines, "", errorStyle.Render(errText))
	}
	lines = append(lines, "", actionHint("space", "toggle")+" · "+actionHint("enter", "done"))
	return modalView("Note display", lines, 72)
}

func configKeysView(rows []configKeyRow, selected int, errText string) string {
	labels := make([]string, 0, len(rows))
	for _, row := range rows {
		labels = append(labels, row.Label+"  "+modalMuted(row.Value))
	}
	lines := modalOptionLines(labels, selected)
	if errText != "" {
		lines = append(lines, "", errorStyle.Render(errText))
	}
	lines = append(lines, "", actionHint("enter", "edit")+" · "+actionHint("s", "done"))
	return modalView("Keyboard shortcuts", lines, 64)
}

func configRootsView(rows []setupRow, selected int, errText string) string {
	lines := modalSetupRowLines(rows, selected)
	lines = append(lines, modalCustomPathLine(selected == len(rows)))
	if errText != "" {
		lines = append(lines, "", errorStyle.Render(errText))
	}
	lines = append(lines, "", actionHint("space", "toggle")+" · "+actionHint("←→", "expand/collapse")+" · "+actionHint("enter", "done"))
	return modalView("Project folders", lines, 72)
}

func modalSetupRowLines(rows []setupRow, selected int) []string {
	lines := make([]string, 0, len(rows))
	rowTexts := make([]string, len(rows))
	maxWidth := 0
	maxCountWidth := 0
	for index, row := range rows {
		box := "[ ]"
		if row.Checked {
			box = "[x]"
		} else if row.Partial {
			box = "[-]"
		}
		prefix := "  "
		if row.Expandable {
			if row.Expanded {
				prefix = "▾ "
			} else {
				prefix = "▸ "
			}
		}
		indent := strings.Repeat("  ", row.Depth)
		line := indent + prefix + box + " " + row.Label
		rowTexts[index] = line
		if width := lipglossWidth(line); width > maxWidth {
			maxWidth = width
		}
		if row.Count != nil {
			if width := lipglossWidth(strconv.Itoa(*row.Count)); width > maxCountWidth {
				maxCountWidth = width
			}
		}
	}
	for index, row := range rows {
		line := rowTexts[index]
		count := ""
		if row.Count != nil {
			count = strconv.Itoa(*row.Count)
			gap := maxWidth - lipglossWidth(line) + 2
			if gap < 2 {
				gap = 2
			}
			countGap := maxCountWidth - lipglossWidth(count)
			line += strings.Repeat(" ", gap+countGap)
		}
		if index == selected {
			rendered := modalAccent("> " + line)
			if count != "" {
				rendered += modalMuted(count)
			}
			lines = append(lines, rendered)
			continue
		}
		if count != "" {
			line += count
		}
		lines = append(lines, "  "+modalMuted(line))
	}
	return lines
}

func modalCustomPathLine(selected bool) string {
	label := "custom path..."
	if selected {
		return modalAccent("> " + label)
	}
	return "  " + modalMuted(label)
}

func configRootInputView(value string, cursor int, errText string, cursorState ...inputCursorState) string {
	lines := inputModalLines(value, "~/Projects", 64, cursor, cursorState...)
	if errText != "" {
		lines = append(lines, "", errorStyle.Render(errText))
	}
	lines = append(lines, "", actionHint("enter", "add")+" · "+actionHint("esc", "back"))
	return modalView("Custom folder", lines, 68)
}
