package tui

import (
	"errors"
	"fmt"
	"sort"
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
	m.configRootOptions = nil
	m.configRootChecked = nil
	m.configRootExpanded = nil
	m.configRootChildren = nil
	m.configRootCounts = nil
	m.configRootSelected = 0
	m.configRootInput = ""
	m.configRootCursor = 0
}

func (m Model) configRows() []configRow {
	cfg := m.configDraft
	return []configRow{
		{Label: "Project folders", Value: listSummary(cfg.Roots)},
		{Label: "Ignored folders", Value: listSummary(cfg.IgnoreDirs)},
		{Label: "Include nested projects", Value: boolSummary(cfg.ScanNestedProjects)},
		{Label: "Show unpushed commits", Value: boolSummary(cfg.ShowUnpushed)},
		{Label: "Mark stale after", Value: daysSummary(cfg.StaleDays)},
		{Label: "Note fallback commit", Value: boolSummary(cfg.NoteFallbackCommit)},
		{Label: "Note fallback description", Value: boolSummary(cfg.NoteFallbackDescription)},
		{Label: "Note show branch", Value: boolSummary(cfg.NoteShowBranch)},
		{Label: "Editor", Value: emptySummary(cfg.Editor)},
		{Label: "Terminal", Value: terminalSummary(cfg.Shell)},
		{Label: "Default sort", Value: cfg.SortBy + " " + cfg.SortDir},
		{Label: "Open settings file", Value: rawConfigPath(m.configPaths.Config)},
		{Label: "Reset settings", Value: ""},
	}
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
		return m, tea.Batch(m.loadConfigRootCandidates(), m.countConfigRootPaths(m.visibleConfigRootPaths()))
	case 1:
		m.openConfigList(configListIgnoreDirs)
	case 4:
		return m.openConfigInput(configInputStaleDays, strconv.Itoa(m.configDraft.StaleDays)), m.startInputCursorBlink()
	case 8:
		return m.openConfigInput(configInputEditor, m.configDraft.Editor), m.startInputCursorBlink()
	case 9:
		return m.openConfigInput(configInputShell, m.configDraft.Shell), m.startInputCursorBlink()
	case 10:
		m.configDraft.SortBy = nextSortBy(m.configDraft.SortBy)
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
		m.configDraft.NoteFallbackCommit = !m.configDraft.NoteFallbackCommit
	case 6:
		m.configDraft.NoteFallbackDescription = !m.configDraft.NoteFallbackDescription
	case 7:
		m.configDraft.NoteShowBranch = !m.configDraft.NoteShowBranch
	case 10:
		if m.configDraft.SortDir == "asc" {
			m.configDraft.SortDir = "desc"
		} else {
			m.configDraft.SortDir = "asc"
		}
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
	default:
		m.changeConfigRow()
	}
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
	m.configRootSelected = 0
	m.configRootExpanded = map[string]bool{}
	m.configRootChildren = map[string][]string{}
	m.configRootCounts = map[string]int{}
	m.configRootChecked = checkedSetupRoots(m.configRootOptions, m.configDraft.Roots)
	if len(m.configRootOptions) == 0 {
		m.configRootOptions = append([]string{}, m.configDraft.Roots...)
	}
	m.revealConfigCheckedRoots()
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
	seen := map[string]bool{}
	options := make([]string, 0, len(candidates)+len(m.configDraft.Roots))
	for _, path := range candidates {
		if path == "" || seen[path] {
			continue
		}
		options = append(options, path)
		seen[path] = true
	}
	for _, path := range m.configDraft.Roots {
		if path == "" || seen[path] {
			continue
		}
		if hasConfigRootOptionAncestor(options, path) {
			continue
		}
		options = append(options, path)
		seen[path] = true
	}
	m.configRootOptions = options
	m.configRootChecked = checkedSetupRoots(options, m.configDraft.Roots)
	m.revealConfigCheckedRoots()
}

func hasConfigRootOptionAncestor(options []string, path string) bool {
	for _, option := range options {
		if normalizeSetupPath(option) == normalizeSetupPath(path) || setupIsDescendant(option, path) {
			return true
		}
	}
	return false
}

func (m Model) openConfigInput(kind configInputKind, value string) Model {
	m.screen = screenConfigInput
	m.configInputKind = kind
	m.configInput = value
	m.configCursor = len([]rune(value))
	m.configErr = ""
	return m
}

func nextSortBy(value string) string {
	options := []string{"activity", "updated", "name", "status"}
	for index, option := range options {
		if value == option {
			return options[(index+1)%len(options)]
		}
	}
	return options[0]
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
		m.screen = screenConfig
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

func (m Model) updateConfigRoots(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	rows := m.visibleConfigRootRows()
	switch value := msg.String(); {
	case isEscapeKey(value), value == "s":
		m.configDraft.Roots = m.selectedConfigRoots()
		m.screen = screenConfig
		m.configErr = ""
	case isDownKey(value):
		m.configRootSelected = wrapPickerSelection(m.configRootSelected, len(rows)+1, 1)
	case isUpKey(value):
		m.configRootSelected = wrapPickerSelection(m.configRootSelected, len(rows)+1, -1)
	case isRightKey(value):
		if m.configRootSelected < len(rows) {
			children := m.expandConfigRootPath(rows[m.configRootSelected].Path)
			return m, m.countConfigRootPaths(children)
		}
	case isLeftKey(value):
		if m.configRootSelected < len(rows) {
			row := rows[m.configRootSelected]
			if row.Depth > 0 {
				m.selectConfigRootPath(row.Parent)
				break
			}
			if row.Expandable || row.Expanded {
				m.configRootExpanded[row.Path] = false
			}
		}
	case value == " ":
		if m.configRootSelected < len(rows) {
			m.toggleConfigRootRow(rows[m.configRootSelected])
		}
	case isEnterKey(value):
		if m.configRootSelected >= len(rows) {
			m.screen = screenConfigRootsInput
			m.configRootInput = ""
			m.configRootCursor = 0
			m.configErr = ""
			return m, m.startInputCursorBlink()
		}
		m.configDraft.Roots = m.selectedConfigRoots()
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
		if m.configRootChecked == nil {
			m.configRootChecked = map[string]bool{}
		}
		m.configRootChecked[root] = true
		m.addConfigRootOption(root)
		m.configRootInput = ""
		m.configRootCursor = 0
		m.screen = screenConfigRoots
		m.selectConfigRootPath(root)
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
	}
	m.configErr = ""
	return m, nil
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

func (m Model) visibleConfigRootRows() []setupRow {
	rows := make([]setupRow, 0, len(m.configRootOptions))
	for _, option := range m.configRootOptions {
		rows = append(rows, m.visibleConfigRootRowsFor(option, "", 0)...)
	}
	return rows
}

func (m Model) visibleConfigRootPaths() []string {
	rows := m.visibleConfigRootRows()
	paths := make([]string, 0, len(rows))
	for _, row := range rows {
		paths = append(paths, row.Path)
	}
	return paths
}

func (m Model) visibleConfigRootRowsFor(path, parent string, depth int) []setupRow {
	children, known := m.configRootChildren[path]
	expanded := m.configRootExpanded[path] && len(children) > 0
	checked := m.isConfigRootChecked(path)
	rows := []setupRow{
		{
			Path:       path,
			Label:      setupRowLabel(path, depth),
			Parent:     parent,
			Depth:      depth,
			Checked:    checked,
			Partial:    !checked && m.hasCheckedConfigRootDescendant(path),
			Expandable: !known || len(children) > 0,
			Expanded:   expanded,
			Count:      m.configRootPathCount(path),
		},
	}
	if !expanded {
		return rows
	}
	for _, child := range children {
		rows = append(rows, m.visibleConfigRootRowsFor(child, path, depth+1)...)
	}
	return rows
}

func (m Model) knownConfigRootRows() []setupRow {
	rows := make([]setupRow, 0, len(m.configRootOptions))
	for _, option := range m.configRootOptions {
		rows = append(rows, m.knownConfigRootRowsFor(option, "", 0)...)
	}
	return rows
}

func (m Model) knownConfigRootRowsFor(path, parent string, depth int) []setupRow {
	rows := []setupRow{{Path: path, Parent: parent, Depth: depth}}
	for _, child := range m.configRootChildren[path] {
		rows = append(rows, m.knownConfigRootRowsFor(child, path, depth+1)...)
	}
	return rows
}

func (m Model) selectedConfigRoots() []string {
	roots := make([]string, 0, len(m.configRootChecked))
	seen := map[string]bool{}
	for _, row := range m.knownConfigRootRows() {
		if m.configRootChecked[row.Path] {
			roots = append(roots, row.Path)
			seen[row.Path] = true
		}
	}
	extra := make([]string, 0, len(m.configRootChecked))
	for path, checked := range m.configRootChecked {
		if checked && !seen[path] {
			extra = append(extra, path)
		}
	}
	sort.Strings(extra)
	roots = append(roots, extra...)
	return roots
}

func (m Model) hasCheckedConfigRootDescendant(path string) bool {
	for checkedPath, checked := range m.configRootChecked {
		if checked && setupIsDescendant(path, checkedPath) {
			return true
		}
	}
	return false
}

func (m Model) isConfigRootChecked(path string) bool {
	if m.configRootChecked[path] {
		return true
	}
	return m.hasCheckedConfigRootAncestor(path)
}

func (m Model) hasCheckedConfigRootAncestor(path string) bool {
	for checkedPath, checked := range m.configRootChecked {
		if checked && setupIsDescendant(checkedPath, path) {
			return true
		}
	}
	return false
}

func (m Model) hasAllCheckedConfigRootChildren(path string) bool {
	children := m.ensureConfigRootChildren(path)
	if len(children) == 0 {
		return false
	}
	for _, child := range children {
		if !m.isConfigRootChecked(child) {
			return false
		}
	}
	return true
}

func (m *Model) toggleConfigRootRow(row setupRow) {
	if m.configRootChecked == nil {
		m.configRootChecked = checkedSetupRoots(m.configRootOptions, m.configDraft.Roots)
	}
	if !m.configRootChecked[row.Path] && m.hasCheckedConfigRootAncestor(row.Path) {
		m.excludeConfigRootFromCheckedAncestor(row.Path)
		m.uncheckConfigRootDescendants(row.Path)
		return
	}
	if !m.configRootChecked[row.Path] && m.hasAllCheckedConfigRootChildren(row.Path) {
		m.uncheckConfigRootDescendants(row.Path)
		return
	}
	next := !m.configRootChecked[row.Path]
	m.configRootChecked[row.Path] = next
	if !next {
		return
	}
	if row.Depth == 0 {
		m.uncheckConfigRootDescendants(row.Path)
		return
	}
	if row.Parent != "" {
		m.uncheckConfigRootAncestors(row.Path)
		m.uncheckConfigRootDescendants(row.Path)
	}
}

func (m *Model) excludeConfigRootFromCheckedAncestor(path string) {
	ancestors := make([]string, 0, len(m.configRootChecked))
	for checkedPath, checked := range m.configRootChecked {
		if checked && setupIsDescendant(checkedPath, path) {
			ancestors = append(ancestors, checkedPath)
		}
	}
	sort.Slice(ancestors, func(i, j int) bool {
		return len([]rune(ancestors[i])) > len([]rune(ancestors[j]))
	})
	for _, ancestor := range ancestors {
		m.configRootChecked[ancestor] = false
		m.checkConfigRootSiblingsAlongPath(ancestor, path)
	}
}

func (m *Model) checkConfigRootSiblingsAlongPath(root, excluded string) {
	current := root
	for current != "" && current != excluded {
		children := m.ensureConfigRootChildren(current)
		next := setupDirectChildOnPath(current, excluded, children)
		for _, child := range children {
			if child != next && child != excluded {
				m.configRootChecked[child] = true
			}
		}
		if next == "" {
			return
		}
		current = next
	}
}

func (m *Model) uncheckConfigRootDescendants(path string) {
	for checkedPath := range m.configRootChecked {
		if setupIsDescendant(path, checkedPath) {
			m.configRootChecked[checkedPath] = false
		}
	}
}

func (m *Model) uncheckConfigRootAncestors(path string) {
	for checkedPath := range m.configRootChecked {
		if setupIsDescendant(checkedPath, path) {
			m.configRootChecked[checkedPath] = false
		}
	}
}

func (m *Model) selectConfigRootPath(path string) {
	rows := m.visibleConfigRootRows()
	for index, row := range rows {
		if row.Path == path {
			m.configRootSelected = index
			return
		}
	}
}

func (m *Model) revealConfigCheckedRoots() {
	for checkedPath, checked := range m.configRootChecked {
		if !checked {
			continue
		}
		m.revealConfigRootPath(checkedPath)
	}
}

func (m *Model) revealConfigRootPath(path string) {
	for _, option := range m.configRootOptions {
		if option == path || !setupIsDescendant(option, path) {
			continue
		}
		current := option
		for current != "" && current != path {
			m.configRootExpanded[current] = true
			children := m.ensureConfigRootChildren(current)
			next := setupDirectChildOnPath(current, path, children)
			if next == "" {
				return
			}
			current = next
		}
	}
}

func (m *Model) addConfigRootOption(path string) {
	for _, existing := range m.configRootOptions {
		if existing == path {
			return
		}
	}
	m.configRootOptions = append(m.configRootOptions, path)
}

func (m *Model) ensureConfigRootChildren(path string) []string {
	if children, ok := m.configRootChildren[path]; ok {
		return children
	}
	children, err := discoverSetupChildren(path)
	if err != nil {
		children = nil
	}
	children = mergeCheckedSetupChildren(path, children, m.configRootChecked)
	m.configRootChildren[path] = children
	return children
}

func (m *Model) expandConfigRootPath(path string) []string {
	children := m.ensureConfigRootChildren(path)
	if len(children) > 0 {
		m.configRootExpanded[path] = true
	}
	return children
}

func (m Model) configRootPathCount(path string) *int {
	if m.configRootCounts == nil {
		return nil
	}
	count, ok := m.configRootCounts[path]
	if !ok {
		return nil
	}
	return &count
}

func (m Model) countConfigRootPaths(paths []string) tea.Cmd {
	missing := make([]string, 0, len(paths))
	for _, path := range paths {
		if _, ok := m.configRootCounts[path]; !ok {
			missing = append(missing, path)
		}
	}
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
