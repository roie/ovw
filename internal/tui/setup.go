package tui

import (
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"ovw/internal/app"
	"ovw/internal/config"

	tea "github.com/charmbracelet/bubbletea"
)

var ErrSetupCancelled = errors.New("setup cancelled")

type setupModel struct {
	options   []string
	checked   map[string]bool
	expanded  map[string]bool
	children  map[string][]string
	counts    map[string]int
	selected  int
	input     string
	cursor    int
	inputting bool
	err       error
	done      bool
	cancelled bool
	creator   configRootsCreator
	counter   setupCounter
}

type setupCounter func([]string) map[string]int

type setupCountsMsg struct {
	counts map[string]int
}

func RunSetupWithOptions(opts app.Options, candidates []string) error {
	return RunSetupWithConfig(opts, candidates, nil, config.Default())
}

func RunSetupWithRoots(opts app.Options, candidates, roots []string) error {
	return RunSetupWithConfig(opts, candidates, roots, config.Default())
}

func RunSetupWithConfig(opts app.Options, candidates, roots []string, cfg config.Config) error {
	model := setupModel{
		options:  candidates,
		checked:  checkedSetupRoots(candidates, roots),
		expanded: map[string]bool{},
		children: map[string][]string{},
		counts:   map[string]int{},
		creator:  app.CreateConfigRoots,
		counter: func(paths []string) map[string]int {
			return app.CountProjectRootsWithConfig(paths, cfg)
		},
	}
	model.revealCheckedRoots()
	programOptions := []tea.ProgramOption{}
	if opts.In != nil {
		programOptions = append(programOptions, tea.WithInput(opts.In))
	}
	if opts.Out != nil {
		programOptions = append(programOptions, tea.WithOutput(opts.Out))
	}
	finalModel, err := tea.NewProgram(model, programOptions...).Run()
	if err != nil {
		return err
	}
	result, ok := finalModel.(setupModel)
	if !ok {
		return nil
	}
	if result.cancelled {
		return ErrSetupCancelled
	}
	return result.err
}

func checkedSetupRoots(candidates, roots []string) map[string]bool {
	if len(roots) == 0 {
		return checkedOnboardingOptions(candidates)
	}
	checked := make(map[string]bool, len(roots))
	for _, root := range roots {
		checked[root] = true
	}
	return checked
}

func (m setupModel) Init() tea.Cmd {
	return m.countSetupPaths(m.visibleSetupPaths())
}

func (m setupModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.inputting {
			return m.updateInput(msg)
		}
		return m.updatePicker(msg)
	case setupCreatedMsg:
		m.err = nil
		m.done = true
		return m, tea.Quit
	case setupFailedMsg:
		m.err = msg.err
		return m, nil
	case setupCountsMsg:
		if m.counts == nil {
			m.counts = map[string]int{}
		}
		for path, count := range msg.counts {
			m.counts[path] = count
		}
		return m, nil
	}
	return m, nil
}

func (m setupModel) View() string {
	if m.inputting {
		errText := ""
		if m.err != nil {
			errText = m.err.Error()
		}
		return onboardingInputView(m.input, m.cursor, errText)
	}
	errText := ""
	if m.err != nil {
		errText = m.err.Error()
	}
	return onboardingSetupView(m.visibleSetupRows(), m.selected, errText)
}

func (m setupModel) updatePicker(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	rows := m.visibleSetupRows()
	switch value := msg.String(); {
	case isQuitKey(value):
		m.cancelled = true
		return m, tea.Quit
	case isDownKey(value):
		m.selected = wrapPickerSelection(m.selected, len(rows)+1, 1)
	case isUpKey(value):
		m.selected = wrapPickerSelection(m.selected, len(rows)+1, -1)
	case isRightKey(value):
		if m.selected < len(rows) {
			children := m.expandSetupPath(rows[m.selected].Path)
			return m, m.countSetupPaths(children)
		}
	case isLeftKey(value):
		if m.selected < len(rows) {
			row := rows[m.selected]
			if row.Depth > 0 {
				m.selectSetupPath(row.Parent)
				break
			}
			if row.Expandable || row.Expanded {
				m.expanded[row.Path] = false
			}
		}
	case value == " ":
		if m.selected < len(rows) {
			m.toggleSetupRow(rows[m.selected])
		}
	case isEnterKey(value):
		if m.selected >= len(rows) {
			m.inputting = true
			m.input = ""
			m.cursor = 0
			m.err = nil
			return m, nil
		}
		m.err = nil
		return m, m.createConfig(m.selectedRoots())
	}
	return m, nil
}

func (m setupModel) updateInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch value := msg.String(); {
	case isEscapeKey(value):
		m.inputting = false
		m.err = nil
	case isEnterKey(value):
		root := strings.TrimSpace(m.input)
		if root == "" {
			return m, m.createConfig(nil)
		}
		roots := m.selectedRoots()
		roots = append(roots, root)
		m.err = nil
		return m, m.createConfig(roots)
	case value == "left":
		m.cursor = textMoveLeft(m.input, m.cursor)
	case value == "right":
		m.cursor = textMoveRight(m.input, m.cursor)
	case isBackspaceKey(value):
		m.input, m.cursor = textBackspace(m.input, m.cursor)
	case isDeleteKey(value):
		m.input, m.cursor = textDelete(m.input, m.cursor)
	default:
		m.input, m.cursor = textInsert(m.input, m.cursor, inputText(msg))
	}
	return m, nil
}

func (m setupModel) selectedRoots() []string {
	roots := make([]string, 0, len(m.checked))
	seen := map[string]bool{}
	for _, row := range m.knownSetupRows() {
		if m.checked[row.Path] {
			roots = append(roots, row.Path)
			seen[row.Path] = true
		}
	}
	extra := make([]string, 0, len(m.checked))
	for path, checked := range m.checked {
		if checked && !seen[path] {
			extra = append(extra, path)
		}
	}
	sort.Strings(extra)
	roots = append(roots, extra...)
	return roots
}

func (m setupModel) visibleSetupRows() []setupRow {
	rows := make([]setupRow, 0, len(m.options))
	for _, option := range m.options {
		rows = append(rows, m.visibleSetupRowsFor(option, "", 0)...)
	}
	return rows
}

func (m setupModel) visibleSetupPaths() []string {
	rows := m.visibleSetupRows()
	paths := make([]string, 0, len(rows))
	for _, row := range rows {
		paths = append(paths, row.Path)
	}
	return paths
}

func (m setupModel) visibleSetupRowsFor(path, parent string, depth int) []setupRow {
	children, known := m.children[path]
	expanded := m.expanded[path] && len(children) > 0
	checked := m.isSetupPathChecked(path)
	rows := []setupRow{
		{
			Path:       path,
			Label:      setupRowLabel(path, depth),
			Parent:     parent,
			Depth:      depth,
			Checked:    checked,
			Partial:    !checked && m.hasCheckedSetupDescendant(path),
			Expandable: !known || len(children) > 0,
			Expanded:   expanded,
			Count:      m.setupPathCount(path),
		},
	}
	if !expanded {
		return rows
	}
	for _, child := range children {
		rows = append(rows, m.visibleSetupRowsFor(child, path, depth+1)...)
	}
	return rows
}

func (m setupModel) knownSetupRows() []setupRow {
	rows := make([]setupRow, 0, len(m.options))
	for _, option := range m.options {
		rows = append(rows, m.knownSetupRowsFor(option, "", 0)...)
	}
	return rows
}

func (m setupModel) knownSetupRowsFor(path, parent string, depth int) []setupRow {
	rows := []setupRow{{Path: path, Parent: parent, Depth: depth}}
	for _, child := range m.children[path] {
		rows = append(rows, m.knownSetupRowsFor(child, path, depth+1)...)
	}
	return rows
}

func (m setupModel) hasCheckedSetupDescendant(root string) bool {
	for path, checked := range m.checked {
		if checked && setupIsDescendant(root, path) {
			return true
		}
	}
	return false
}

func (m setupModel) hasCheckedSetupAncestor(path string) bool {
	for checkedPath, checked := range m.checked {
		if checked && setupIsDescendant(checkedPath, path) {
			return true
		}
	}
	return false
}

func (m setupModel) isSetupPathChecked(path string) bool {
	return m.checked[path] || m.hasCheckedSetupAncestor(path) || m.hasAllCheckedSetupChildren(path)
}

func (m setupModel) hasAllCheckedSetupChildren(path string) bool {
	children, known := m.children[path]
	if !known || len(children) == 0 {
		return false
	}
	for _, child := range children {
		if !m.isSetupPathChecked(child) {
			return false
		}
	}
	return true
}

func (m *setupModel) expandSetupPath(path string) []string {
	if m.expanded == nil {
		m.expanded = map[string]bool{}
	}
	m.ensureSetupChildren(path)
	if len(m.children[path]) > 0 {
		m.expanded[path] = true
	}
	return m.children[path]
}

func mergeCheckedSetupChildren(root string, children []string, checked map[string]bool) []string {
	seen := make(map[string]bool, len(children))
	merged := make([]string, 0, len(children))
	for _, child := range children {
		seen[child] = true
		merged = append(merged, child)
	}
	extra := make([]string, 0, len(checked))
	for path, isChecked := range checked {
		if !isChecked || !setupIsDirectChild(root, path) || seen[path] {
			continue
		}
		extra = append(extra, path)
	}
	sort.Strings(extra)
	return append(merged, extra...)
}

func (m *setupModel) revealCheckedRoots() {
	roots := make([]string, 0, len(m.checked))
	for root, checked := range m.checked {
		if checked {
			roots = append(roots, root)
		}
	}
	sort.Strings(roots)
	for _, root := range roots {
		m.revealCheckedRoot(root)
	}
}

func (m *setupModel) revealCheckedRoot(root string) {
	for _, option := range m.options {
		if !setupIsDescendant(option, root) {
			continue
		}
		current := option
		for current != "" && current != root {
			children := m.ensureSetupChildren(current)
			next := setupDirectChildOnPath(current, root, children)
			if next == "" {
				break
			}
			m.expanded[current] = true
			current = next
		}
		return
	}
}

func (m *setupModel) ensureSetupChildren(path string) []string {
	if children, ok := m.children[path]; ok {
		return children
	}
	children, _ := discoverSetupChildren(path)
	children = mergeCheckedSetupChildren(path, children, m.checked)
	if m.children == nil {
		m.children = map[string][]string{}
	}
	m.children[path] = children
	return children
}

func (m setupModel) setupPathCount(path string) *int {
	if m.counts == nil {
		return nil
	}
	count, ok := m.counts[path]
	if !ok {
		return nil
	}
	return &count
}

func (m setupModel) countSetupPaths(paths []string) tea.Cmd {
	missing := make([]string, 0, len(paths))
	for _, path := range paths {
		if path == "" {
			continue
		}
		if m.counts != nil {
			if _, ok := m.counts[path]; ok {
				continue
			}
		}
		missing = append(missing, path)
	}
	if len(missing) == 0 {
		return nil
	}
	counter := m.counter
	if counter == nil {
		counter = app.CountProjectRoots
	}
	return func() tea.Msg {
		return setupCountsMsg{counts: counter(missing)}
	}
}

func setupDirectChildOnPath(parent, target string, children []string) string {
	for _, child := range children {
		if normalizeSetupPath(child) == normalizeSetupPath(target) || setupIsDescendant(child, target) {
			return child
		}
	}
	return ""
}

func discoverSetupChildren(root string) ([]string, error) {
	expanded, err := config.ExpandPath(root)
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(expanded)
	if err != nil {
		return nil, err
	}
	children := []string{}
	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		children = append(children, filepath.Join(root, entry.Name()))
	}
	return children, nil
}

func (m *setupModel) toggleSetupRow(row setupRow) {
	if m.checked == nil {
		m.checked = checkedOnboardingOptions(m.options)
	}
	if !m.checked[row.Path] && m.hasCheckedSetupAncestor(row.Path) {
		m.excludeSetupPathFromCheckedAncestor(row.Path)
		m.uncheckSetupDescendants(row.Path)
		return
	}
	if !m.checked[row.Path] && m.hasAllCheckedSetupChildren(row.Path) {
		m.uncheckSetupDescendants(row.Path)
		return
	}
	next := !m.checked[row.Path]
	m.checked[row.Path] = next
	if !next {
		return
	}
	if row.Depth == 0 {
		m.uncheckSetupDescendants(row.Path)
		return
	}
	if row.Parent != "" {
		m.uncheckSetupAncestors(row.Path)
		m.uncheckSetupDescendants(row.Path)
	}
}

func (m *setupModel) excludeSetupPathFromCheckedAncestor(path string) {
	ancestors := make([]string, 0, len(m.checked))
	for checkedPath, checked := range m.checked {
		if checked && setupIsDescendant(checkedPath, path) {
			ancestors = append(ancestors, checkedPath)
		}
	}
	sort.Slice(ancestors, func(i, j int) bool {
		return len([]rune(ancestors[i])) > len([]rune(ancestors[j]))
	})
	for _, ancestor := range ancestors {
		m.checked[ancestor] = false
		m.checkSetupSiblingsAlongPath(ancestor, path)
	}
}

func (m *setupModel) checkSetupSiblingsAlongPath(root, excluded string) {
	current := root
	for current != "" && current != excluded {
		children := m.ensureSetupChildren(current)
		next := setupDirectChildOnPath(current, excluded, children)
		for _, child := range children {
			if child != next && child != excluded {
				m.checked[child] = true
			}
		}
		if next == "" {
			return
		}
		current = next
	}
}

func (m *setupModel) uncheckSetupDescendants(path string) {
	for checkedPath := range m.checked {
		if setupIsDescendant(path, checkedPath) {
			m.checked[checkedPath] = false
		}
	}
}

func (m *setupModel) uncheckSetupAncestors(path string) {
	for checkedPath := range m.checked {
		if setupIsDescendant(checkedPath, path) {
			m.checked[checkedPath] = false
		}
	}
}

func (m *setupModel) selectSetupPath(path string) {
	rows := m.visibleSetupRows()
	for index, row := range rows {
		if row.Path == path {
			m.selected = index
			return
		}
	}
}

func setupRowLabel(path string, depth int) string {
	if depth == 0 {
		return path
	}
	return filepath.Base(path)
}

func setupIsDescendant(parent, child string) bool {
	parent = normalizeSetupPath(parent)
	child = normalizeSetupPath(child)
	return child != parent && strings.HasPrefix(child, parent+"/")
}

func setupIsDirectChild(parent, child string) bool {
	if !setupIsDescendant(parent, child) {
		return false
	}
	relative := strings.TrimPrefix(normalizeSetupPath(child), normalizeSetupPath(parent)+"/")
	return !strings.Contains(relative, "/")
}

func normalizeSetupPath(value string) string {
	if expanded, err := config.ExpandPath(value); err == nil {
		value = expanded
	}
	return strings.TrimRight(filepath.ToSlash(value), "/")
}

func (m setupModel) createConfig(roots []string) tea.Cmd {
	return func() tea.Msg {
		creator := m.creator
		if creator == nil {
			creator = app.CreateConfigRoots
		}
		if _, _, err := creator(roots); err != nil {
			return setupFailedMsg{err: err}
		}
		return setupCreatedMsg{}
	}
}

type setupCreatedMsg struct{}

type setupFailedMsg struct {
	err error
}
