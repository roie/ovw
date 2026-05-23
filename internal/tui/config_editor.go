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
	configListFieldOptions
)

type configInputKind int

const (
	configInputNone configInputKind = iota
	configInputStaleDays
	configInputEditor
	configInputShell
	configInputKeyDetails
	configInputKeyEditor
	configInputKeyTerminal
	configInputKeyRunner
	configInputKeyNote
	configInputKeyStatus
	configInputKeyPin
	configInputKeyHide
	configInputFieldLabel
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

type configFieldRow struct {
	Label string
	Value string
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
	m.configFieldSel = 0
	m.configFieldIndex = -1
	m.configFieldAdding = false
	m.configFieldDraft = config.FieldConfig{}
	m.configFieldRowSel = 0
	m.configFieldInput = ""
	m.configFieldCursor = 0
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
		{Label: "Fields", Value: fieldsSummary(cfg.Fields)},
		{Label: "Note display", Value: noteDisplaySummary(cfg)},
		{Label: "Keyboard shortcuts", Value: actionKeysSummary(cfg.Keys.Actions)},
		{Label: "Editor", Value: emptySummary(cfg.Editor)},
		{Label: "Terminal", Value: terminalSummary(cfg.Shell)},
		{Label: "Default sort", Value: cfg.SortBy + " " + cfg.SortDir},
		{Label: "Open settings file", Value: rawConfigPath(m.configPaths.Config)},
		{Label: "Reset settings", Value: ""},
	}
}

func fieldsSummary(fields []config.FieldConfig) string {
	if len(fields) == 0 {
		return ""
	}
	if len(fields) == 1 {
		return fieldDisplayLabel(fields[0])
	}
	return fmt.Sprintf("%s +%d", fieldDisplayLabel(fields[0]), len(fields)-1)
}

func actionKeysSummary(keys config.ActionKeyConfig) string {
	keys = actionKeys(config.Config{Keys: config.KeyConfig{Actions: keys}})
	return fmt.Sprintf("%s/%s/%s/%s", keys.Details, keys.Editor, keys.Terminal, keys.Runner)
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
	case isConfigSaveKey(value):
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
		m.openConfigFields()
	case 6:
		m.openConfigNote()
	case 7:
		m.openConfigKeys()
	case 8:
		return m.openConfigInput(configInputEditor, m.configDraft.Editor), m.startInputCursorBlink()
	case 9:
		return m.openConfigInput(configInputShell, m.configDraft.Shell), m.startInputCursorBlink()
	case 11:
		m.loading = true
		return m, m.openConfigFile()
	case 12:
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
		m.openConfigFields()
	case 6:
		m.openConfigNote()
	case 7:
		m.openConfigKeys()
	case 10:
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
		m.openConfigFields()
		return
	case 6:
		m.openConfigNote()
		return
	case 7:
		m.openConfigKeys()
		return
	case 10:
		m.cycleConfigSort(1)
		return
	default:
		m.changeConfigRow()
	}
}

func (m *Model) openConfigFields() {
	m.screen = screenConfigFields
	m.configFieldSel = clampIndex(m.configFieldSel, len(m.configDraft.Fields))
	m.configFieldIndex = -1
	m.configFieldRowSel = 0
	m.configErr = ""
}

func (m *Model) openConfigField(index int) {
	m.screen = screenConfigField
	m.configFieldIndex = clampIndex(index, len(m.configDraft.Fields))
	m.configFieldAdding = false
	if len(m.configDraft.Fields) > 0 {
		m.configFieldDraft = m.configDraft.Fields[m.configFieldIndex]
	}
	m.configFieldRowSel = 0
	m.configErr = ""
}

func (m *Model) openNewConfigField() {
	m.screen = screenConfigField
	m.configFieldIndex = -1
	m.configFieldAdding = true
	m.configFieldDraft = config.FieldConfig{Type: "text"}
	m.configFieldRowSel = 0
	m.configErr = ""
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
	if (m.configListField == configListFieldOptions || m.configListInput != "" || m.configListEditing) && (msg.Type == tea.KeyRunes || msg.Type == tea.KeySpace) {
		m.configListInput, m.configListCursor = textInsert(m.configListInput, m.configListCursor, inputText(msg))
		return m, nil
	}
	switch value := msg.String(); {
	case isEscapeKey(value):
		if m.configListField == configListFieldOptions {
			m.screen = screenConfigField
		} else {
			m.screen = screenConfig
		}
		m.configErr = ""
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
		if m.configListField == configListFieldOptions && m.configListInput == "" && !m.configListEditing && len(values) > 0 {
			m.configListSel = clampIndex(m.configListSel, len(values))
			m.configListInput = values[m.configListSel]
			m.configListCursor = len([]rune(m.configListInput))
			m.configListEditing = true
			return m, nil
		}
		return m.saveConfigListInput(), nil
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
	case value == "delete":
		if m.configListInput == "" && !m.configListEditing && len(values) > 0 {
			m.setConfigListValues(deleteStringAt(values, clampIndex(m.configListSel, len(values))))
			m.configListSel = clampIndex(m.configListSel, len(m.configListValues()))
		} else {
			m.configListInput, m.configListCursor = textDelete(m.configListInput, m.configListCursor)
		}
	case isDeleteKey(value):
		m.configListInput, m.configListCursor = textDelete(m.configListInput, m.configListCursor)
	default:
		m.configListInput, m.configListCursor = textInsert(m.configListInput, m.configListCursor, inputText(msg))
		m.configListEditing = false
	}
	return m, nil
}

func (m Model) updateConfigInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if isConfigKeyInput(m.configInputKind) {
		return m.updateConfigShortcutInput(msg)
	}
	switch value := msg.String(); {
	case isEscapeKey(value):
		if m.configInputKind == configInputFieldLabel {
			m.screen = screenConfigField
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

func (m Model) updateConfigShortcutInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	value := strings.TrimSpace(msg.String())
	switch {
	case isEscapeKey(value):
		m.screen = screenConfigKeys
		m.configErr = ""
		return m, nil
	case value == "":
		return m, nil
	default:
		return m.saveConfigShortcut(value)
	}
}

func (m Model) updateConfigFields(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	count := len(m.configDraft.Fields)
	if msg.Type == tea.KeyRunes || msg.Type == tea.KeySpace {
		m.configFieldInput, m.configFieldCursor = textInsert(m.configFieldInput, m.configFieldCursor, inputText(msg))
		m.configErr = ""
		return m, nil
	}
	switch value := msg.String(); {
	case isEscapeKey(value):
		m.screen = screenConfig
		m.configErr = ""
	case isDownKey(value):
		m.configFieldSel = wrapPickerSelection(m.configFieldSel, count, 1)
	case isUpKey(value):
		m.configFieldSel = wrapPickerSelection(m.configFieldSel, count, -1)
	case isLeftKey(value):
		if m.configFieldInput != "" {
			m.configFieldCursor = textMoveLeft(m.configFieldInput, m.configFieldCursor)
		} else {
			m.moveConfigField(-1)
		}
	case isRightKey(value):
		if m.configFieldInput != "" {
			m.configFieldCursor = textMoveRight(m.configFieldInput, m.configFieldCursor)
		} else {
			m.moveConfigField(1)
		}
	case value == "delete":
		if m.configFieldInput != "" {
			m.configFieldInput, m.configFieldCursor = textDelete(m.configFieldInput, m.configFieldCursor)
		} else if m.configFieldSel < len(m.configDraft.Fields) {
			m.deleteConfigField(m.configFieldSel)
		}
	case isBackspaceKey(value):
		m.configFieldInput, m.configFieldCursor = textBackspace(m.configFieldInput, m.configFieldCursor)
	case isDeletePreviousWordKey(value):
		m.configFieldInput, m.configFieldCursor = textDeletePreviousWord(m.configFieldInput, m.configFieldCursor)
	case isClearBeforeKey(value):
		m.configFieldInput, m.configFieldCursor = textClearBefore(m.configFieldInput, m.configFieldCursor)
	case isClearAfterKey(value):
		m.configFieldInput, m.configFieldCursor = textClearAfter(m.configFieldInput, m.configFieldCursor)
	case isMoveStartKey(value):
		m.configFieldCursor = textMoveStart(m.configFieldInput, m.configFieldCursor)
	case isMoveEndKey(value):
		m.configFieldCursor = textMoveEnd(m.configFieldInput, m.configFieldCursor)
	case isEnterKey(value):
		if strings.TrimSpace(m.configFieldInput) != "" {
			m.addConfigFieldFromInput()
			return m, nil
		}
		if len(m.configDraft.Fields) == 0 {
			return m, nil
		}
		m.openConfigField(m.configFieldSel)
	}
	return m, nil
}

func (m *Model) addConfigFieldFromInput() {
	label := strings.TrimSpace(m.configFieldInput)
	if label == "" {
		return
	}
	field := config.FieldConfig{ID: fieldIDFromLabel(label), Label: label, Type: "text"}
	if field.ID == "" {
		m.configErr = "Use a field label with letters or numbers."
		return
	}
	next := m.configDraft
	next.Fields = append(append([]config.FieldConfig{}, next.Fields...), field)
	if err := config.Validate(next); err != nil {
		m.configErr = configFieldErrorMessage(err)
		return
	}
	m.configDraft = next
	m.configFieldSel = len(m.configDraft.Fields) - 1
	m.configFieldInput = ""
	m.configFieldCursor = 0
	m.configErr = ""
}

func (m Model) configFieldRows() []configFieldRow {
	field := m.currentConfigField()
	if field == nil {
		return nil
	}
	rows := []configFieldRow{
		{Label: "Label", Value: fieldDisplayLabel(*field)},
		{Label: "Type", Value: field.Type},
	}
	if field.Type == "select" {
		rows = append(rows, configFieldRow{Label: "Options", Value: listSummary(field.Options)})
	}
	return rows
}

func (m Model) updateConfigField(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	rows := m.configFieldRows()
	if len(rows) == 0 {
		m.screen = screenConfigFields
		return m, nil
	}
	switch value := msg.String(); {
	case isEscapeKey(value):
		m.screen = screenConfigFields
		m.configErr = ""
	case isConfigSaveKey(value):
		if m.saveConfigFieldDraft() {
			m.screen = screenConfigFields
		}
	case isDownKey(value):
		m.configFieldRowSel = wrapPickerSelection(m.configFieldRowSel, len(rows), 1)
	case isUpKey(value):
		m.configFieldRowSel = wrapPickerSelection(m.configFieldRowSel, len(rows), -1)
	case isLeftKey(value):
		m.changeConfigFieldRow(-1)
	case isRightKey(value):
		m.changeConfigFieldRow(1)
	case isEnterKey(value), value == " ":
		return m.editConfigFieldRow()
	}
	return m, nil
}

func (m Model) editConfigFieldRow() (tea.Model, tea.Cmd) {
	field := m.currentConfigField()
	if field == nil {
		m.screen = screenConfigFields
		return m, nil
	}
	row := m.configFieldRows()[clampIndex(m.configFieldRowSel, len(m.configFieldRows()))]
	switch row.Label {
	case "Label":
		return m.openConfigInput(configInputFieldLabel, field.Label), m.startInputCursorBlink()
	case "Type":
		return m, nil
	case "Options":
		m.openConfigList(configListFieldOptions)
	}
	return m, nil
}

func (m *Model) changeConfigFieldRow(delta int) {
	field := m.currentConfigField()
	if field == nil {
		return
	}
	row := m.configFieldRows()[clampIndex(m.configFieldRowSel, len(m.configFieldRows()))]
	switch row.Label {
	case "Type":
		m.cycleConfigFieldType(delta)
	}
	m.configErr = ""
}

func (m *Model) cycleConfigFieldType(delta int) {
	types := []string{"text", "select", "checkbox"}
	field := m.currentConfigField()
	if field == nil {
		return
	}
	index := indexOfString(types, field.Type)
	if index < 0 {
		index = 0
	}
	next := wrapPickerSelection(index, len(types), delta)
	m.configFieldDraft.Type = types[next]
	if types[next] != "select" {
		m.configFieldDraft.Options = nil
	}
	m.configFieldRowSel = clampIndex(m.configFieldRowSel, len(m.configFieldRows()))
}

func (m *Model) moveConfigField(delta int) {
	fields := append([]config.FieldConfig{}, m.configDraft.Fields...)
	if len(fields) == 0 || m.configFieldSel >= len(fields) {
		return
	}
	index := clampIndex(m.configFieldSel, len(fields))
	next := index + delta
	if next < 0 || next >= len(fields) {
		return
	}
	fields[index], fields[next] = fields[next], fields[index]
	m.configDraft.Fields = fields
	m.configFieldSel = next
}

func (m *Model) deleteConfigField(index int) {
	if len(m.configDraft.Fields) == 0 {
		return
	}
	index = clampIndex(index, len(m.configDraft.Fields))
	fieldColumn := "field:" + m.configDraft.Fields[index].ID
	next := append([]config.FieldConfig{}, m.configDraft.Fields[:index]...)
	next = append(next, m.configDraft.Fields[index+1:]...)
	m.configDraft.Fields = next
	m.configDraft.Columns = deleteString(m.configDraft.Columns, fieldColumn)
	m.configDraft.ColumnOrder = deleteString(m.configDraft.ColumnOrder, fieldColumn)
	m.configFieldSel = clampIndex(index, len(m.configDraft.Fields)+1)
	m.configFieldIndex = -1
	m.configErr = ""
}

func deleteString(values []string, value string) []string {
	next := make([]string, 0, len(values))
	for _, candidate := range values {
		if candidate != value {
			next = append(next, candidate)
		}
	}
	return next
}

func (m *Model) saveConfigFieldDraft() bool {
	field := m.configFieldDraft
	if strings.TrimSpace(field.Label) == "" {
		m.configErr = "Add a label before saving."
		return false
	}
	if strings.TrimSpace(field.ID) == "" && strings.TrimSpace(field.Label) != "" {
		field.ID = fieldIDFromLabel(field.Label)
	}
	next := m.configDraft
	if m.configFieldAdding {
		next.Fields = append(append([]config.FieldConfig{}, next.Fields...), field)
	} else if m.configFieldIndex >= 0 && m.configFieldIndex < len(next.Fields) {
		fields := append([]config.FieldConfig{}, next.Fields...)
		fields[m.configFieldIndex] = field
		next.Fields = fields
	} else {
		m.screen = screenConfigFields
		return false
	}
	if err := config.Validate(next); err != nil {
		m.configErr = configFieldErrorMessage(err)
		return false
	}
	m.configDraft = next
	if m.configFieldAdding {
		m.configFieldSel = len(m.configDraft.Fields) - 1
	} else {
		m.configFieldSel = clampIndex(m.configFieldIndex, len(m.configDraft.Fields))
	}
	m.configFieldAdding = false
	m.configFieldIndex = -1
	m.configErr = ""
	return true
}

func configFieldErrorMessage(err error) string {
	if err == nil {
		return ""
	}
	message := err.Error()
	if strings.Contains(message, "already used") {
		return "A field with this label already exists."
	}
	return message
}

func (m Model) currentConfigField() *config.FieldConfig {
	if m.configFieldAdding {
		return &m.configFieldDraft
	}
	if m.configFieldIndex < 0 || m.configFieldIndex >= len(m.configDraft.Fields) {
		return nil
	}
	return &m.configFieldDraft
}

func fieldDisplayLabel(field config.FieldConfig) string {
	label := strings.TrimSpace(field.Label)
	if label != "" {
		return label
	}
	return titleFromFieldID(field.ID)
}

func titleFromFieldID(id string) string {
	parts := strings.FieldsFunc(id, func(r rune) bool {
		return r == '-' || r == '_'
	})
	for index, part := range parts {
		if part == "" {
			continue
		}
		parts[index] = strings.ToUpper(part[:1]) + part[1:]
	}
	return strings.Join(parts, " ")
}

func fieldIDFromLabel(label string) string {
	lower := strings.ToLower(strings.TrimSpace(label))
	var out strings.Builder
	lastUnderscore := false
	for _, r := range lower {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			out.WriteRune(r)
			lastUnderscore = false
			continue
		}
		if r == '_' || r == ' ' || r == '/' || r == '.' {
			if out.Len() > 0 && !lastUnderscore {
				out.WriteByte('_')
				lastUnderscore = true
			}
		}
	}
	return strings.Trim(out.String(), "_")
}

func isConfigSaveKey(value string) bool {
	return value == "ctrl+s"
}

func isConfigKeyInput(kind configInputKind) bool {
	switch kind {
	case configInputKeyDetails, configInputKeyEditor, configInputKeyTerminal, configInputKeyRunner, configInputKeyNote, configInputKeyStatus, configInputKeyPin, configInputKeyHide:
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
	case isEscapeKey(value), isEnterKey(value):
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
		{Label: "Show details", Value: keys.Details, Kind: configInputKeyDetails},
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
	case isEscapeKey(value):
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
	case isEscapeKey(value):
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
	case configInputKeyDetails, configInputKeyEditor, configInputKeyTerminal, configInputKeyRunner, configInputKeyNote, configInputKeyStatus, configInputKeyPin, configInputKeyHide:
		return m.saveConfigShortcut(value)
	case configInputFieldLabel:
		if m.currentConfigField() == nil {
			m.screen = screenConfigFields
			return m, nil
		}
		m.configFieldDraft.Label = value
		if strings.TrimSpace(m.configFieldDraft.ID) == "" {
			m.configFieldDraft.ID = fieldIDFromLabel(value)
		}
		m.screen = screenConfigField
	}
	m.configErr = ""
	return m, nil
}

func (m Model) saveConfigShortcut(value string) (tea.Model, tea.Cmd) {
	value = strings.TrimSpace(value)
	if label, ok := m.configShortcutOwner(value); ok {
		m.configErr = fmt.Sprintf("Shortcut %q is already used by %s.", value, label)
		return m, nil
	}
	next := m.configDraft
	setActionKey(&next.Keys.Actions, m.configInputKind, value)
	if err := config.Validate(next); err != nil {
		m.configErr = friendlyConfigShortcutError(value, err)
		return m, nil
	}
	m.configDraft = next
	m.screen = screenConfigKeys
	m.configErr = ""
	return m, nil
}

func (m Model) configShortcutOwner(value string) (string, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", false
	}
	for _, row := range m.configKeyRows() {
		if row.Kind == m.configInputKind {
			continue
		}
		if strings.TrimSpace(row.Value) == value {
			return row.Label, true
		}
	}
	return "", false
}

func friendlyConfigShortcutError(value string, err error) string {
	message := err.Error()
	if strings.Contains(message, "key is reserved") {
		return fmt.Sprintf("Shortcut %q is reserved.", value)
	}
	if strings.Contains(message, "expected a key") {
		return "Press a shortcut to save."
	}
	if strings.Contains(message, "already used by") {
		return fmt.Sprintf("Shortcut %q is already used.", value)
	}
	return message
}

func setActionKey(keys *config.ActionKeyConfig, kind configInputKind, value string) {
	switch kind {
	case configInputKeyDetails:
		keys.Details = value
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
	case configListFieldOptions:
		return "Options"
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
	case configListFieldOptions:
		if field := m.currentConfigField(); field != nil {
			return field.Options
		}
		return nil
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
	case configListFieldOptions:
		if m.currentConfigField() != nil {
			m.configFieldDraft.Options = values
		}
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
	case configInputKeyDetails:
		return "Show details shortcut"
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
	case configInputFieldLabel:
		return "Field label"
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
	case configInputKeyDetails:
		return "enter"
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
	case configInputFieldLabel:
		return "Field label"
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
	lines = append(lines, "", actionHint("enter", "edit")+" · "+actionHint("←→", "change")+" · "+actionHint("ctrl+s", "save"))
	return modalView("Settings", lines, 64)
}

func configListView(title string, values []string, selected int, input string, cursor int, editing bool, errText string, height int, cursorState ...inputCursorState) string {
	placeholder := "type to add..."
	if editing {
		placeholder = "edit selected..."
	}
	lines := inputModalLines(input, placeholder, 56, cursor, cursorState...)
	lines = append(lines, "")
	if len(values) == 0 {
		lines = append(lines, modalMuted("No values yet"))
	} else {
		optionHeight := configListOptionHeight(height, len(lines), errText != "")
		lines = append(lines, visibleConfigListOptionLines(values, selected, optionHeight)...)
	}
	if errText != "" {
		lines = append(lines, "", errorStyle.Render(errText))
	}
	lines = append(lines, "", configListFooter(title, len(values)))
	return modalView(title, lines, 64)
}

func configListOptionHeight(height int, beforeOptions int, hasError bool) int {
	if height <= 0 {
		return 1 << 30
	}
	// Frame lines, existing input/separator lines, optional error block,
	// blank separator before footer, and footer line.
	reserved := 4 + beforeOptions + 2
	if hasError {
		reserved += 2
	}
	available := height - reserved
	if available < 1 {
		return 1
	}
	return available
}

func visibleConfigListOptionLines(values []string, selected int, height int) []string {
	lines := modalOptionLines(values, selected)
	if height <= 0 || len(lines) <= height {
		return lines
	}
	selected = clampIndex(selected, len(lines))
	start := selected - height + 1
	if start < 0 {
		start = 0
	}
	end := start + height
	if end > len(lines) {
		end = len(lines)
	}
	return append([]string{}, lines[start:end]...)
}

func configListFooter(title string, valueCount int) string {
	if title != "Options" {
		return actionHint("enter", "add/save") + " · " + actionHint("space", "edit") + " · " + actionHint("del", "delete") + " · " + actionHint("esc", "done")
	}
	if valueCount == 0 {
		return actionHint("enter", "add") + " · " + actionHint("esc", "done")
	}
	return actionHint("enter", "edit/add") + " · " + actionHint("del", "delete") + " · " + actionHint("esc", "done")
}

func configInputView(title, value, placeholder string, cursor int, errText string, cursorState ...inputCursorState) string {
	lines := inputModalLines(value, placeholder, 56, cursor, cursorState...)
	if errText != "" {
		lines = append(lines, "", errorStyle.Render(errText))
	}
	lines = append(lines, "", actionHint("enter", "save"))
	return modalView(title, lines, 60)
}

func configShortcutCaptureView(title, current, errText string) string {
	lines := []string{modalMuted("press shortcut...")}
	if current != "" {
		lines = append(lines, "", "Current  "+modalMuted(current))
	}
	if errText != "" {
		lines = append(lines, "", errorStyle.Render(errText))
	}
	lines = append(lines, "", actionHint("esc", "cancel"))
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
	lines = append(lines, "", actionHint("enter", "edit")+" · "+actionHint("esc", "done"))
	return modalView("Keyboard shortcuts", lines, 64)
}

func configFieldsView(fields []config.FieldConfig, selected int, input string, cursor int, errText string, cursorState ...inputCursorState) string {
	lines := inputModalLines(input, "type to add...", 64, cursor, cursorState...)
	lines = append(lines, "")
	labels := make([]string, 0, len(fields))
	for _, field := range fields {
		labels = append(labels, fieldDisplayLabel(field)+"  "+modalMuted(field.Type))
	}
	if len(labels) == 0 {
		lines = append(lines, modalMuted("No fields yet"))
	} else {
		lines = append(lines, modalOptionLines(labels, selected)...)
	}
	if errText != "" {
		lines = append(lines, "", errorStyle.Render(errText))
	}
	lines = append(lines, "", actionHint("enter", "add/edit")+" · "+actionHint("del", "delete")+" · "+actionHint("←→", "reorder")+" · "+actionHint("esc", "done"))
	return modalView("Fields", lines, 72)
}

func configFieldView(rows []configFieldRow, selected int, errText string) string {
	labels := make([]string, 0, len(rows))
	for _, row := range rows {
		value := row.Value
		if value != "" {
			value = "  " + modalMuted(value)
		}
		labels = append(labels, row.Label+value)
	}
	lines := modalOptionLines(labels, selected)
	if errText != "" {
		lines = append(lines, "", errorStyle.Render(errText))
	}
	lines = append(lines, "", actionHint("enter", "edit")+" · "+actionHint("←→", "change")+" · "+actionHint("ctrl+s", "save"))
	return modalView("Field", lines, 64)
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
