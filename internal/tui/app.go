package tui

import (
	"fmt"
	"io"
	"strings"
	"time"

	"ovw/internal/app"
	"ovw/internal/config"
	ovwformat "ovw/internal/format"
	"ovw/internal/gitactivity"
	"ovw/internal/project"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type overviewLoader func(app.Options) (app.OverviewResult, error)
type metadataUpdater func(string, app.MetadataUpdate) (app.MetadataUpdateResult, error)
type visibilityUpdater func(string, bool) (app.MetadataUpdateResult, error)
type editorRunner func(string, string) error
type terminalRunner func(string, string) tea.Cmd
type recentLoader func(string, time.Time) ([]ovwformat.RecentCommit, error)

type screenMode int

const (
	screenTable screenMode = iota
	screenDetail
	screenFilter
	screenSort
	screenNote
	screenStatus
	screenStatusInput
	screenHelp
)

type Model struct {
	request  app.Options
	loader   overviewLoader
	updater  metadataUpdater
	visible  visibilityUpdater
	editor   editorRunner
	terminal terminalRunner
	recent   recentLoader

	width          int
	height         int
	selected       int
	screen         screenMode
	search         string
	searching      bool
	filterSelected int
	activeFilter   string
	sortSelected   int
	activeSort     string
	noteInput      string
	statusSelected int
	statusInput    string
	message        string
	loading        bool
	loadErr        error
	config         config.Config
	projects       []project.Project
	recentByPath   map[string][]ovwformat.RecentCommit
}

func New() Model {
	return NewWithOptions(app.Options{})
}

func NewWithOptions(opts app.Options) Model {
	if opts.Out == nil {
		opts.Out = io.Discard
	}
	return Model{
		request:      opts,
		loader:       app.LoadOverview,
		updater:      app.UpdateProjectMetadata,
		visible:      app.SetProjectHidden,
		editor:       runEditor,
		terminal:     runTerminal,
		recent:       loadRecentCommits,
		activeFilter: optionsFromRequest(opts),
		activeSort:   sortFromRequest(opts),
		loading:      true,
		recentByPath: map[string][]ovwformat.RecentCommit{},
	}
}

func NewWithLoader(loader overviewLoader) Model {
	model := NewWithOptions(app.Options{})
	model.loader = loader
	return model
}

func (m Model) Init() tea.Cmd {
	return m.loadOverview()
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.searching {
			return m.updateSearch(msg)
		}
		if m.screen == screenFilter {
			return m.updateFilter(msg)
		}
		if m.screen == screenSort {
			return m.updateSort(msg)
		}
		if m.screen == screenNote {
			return m.updateNote(msg)
		}
		if m.screen == screenStatus {
			return m.updateStatusPicker(msg)
		}
		if m.screen == screenStatusInput {
			return m.updateStatusInput(msg)
		}
		if m.screen == screenDetail {
			return m.updateDetail(msg)
		}
		if isEscapeKey(msg.String()) && m.screen == screenHelp {
			m.screen = screenTable
			return m, nil
		}
		if isQuitKey(msg.String()) {
			return m, tea.Quit
		}
		if isEnterKey(msg.String()) && m.canOpenDetail() {
			m.screen = screenDetail
			return m, nil
		}
		if isSearchKey(msg.String()) {
			m.screen = screenTable
			m.searching = true
			return m, nil
		}
		if isFilterKey(msg.String()) {
			m.screen = screenFilter
			m.filterSelected = m.currentFilterIndex()
			return m, nil
		}
		if isSortKey(msg.String()) {
			m.screen = screenSort
			m.sortSelected = m.currentSortIndex()
			return m, nil
		}
		if isNoteKey(msg.String()) && m.canOpenDetail() {
			project, _ := m.currentProject()
			m.screen = screenNote
			m.noteInput = project.Note.Value
			return m, nil
		}
		if isStatusKey(msg.String()) && m.canOpenDetail() {
			m.screen = screenStatus
			m.statusSelected = 0
			return m, nil
		}
		if isReloadKey(msg.String()) {
			path := m.selectedProjectPath()
			m.screen = screenTable
			m.loading = true
			return m, m.reloadOverview(path, "Reloaded")
		}
		if isOpenKey(msg.String()) && m.canOpenDetail() {
			return m, m.openSelectedProject()
		}
		if isTerminalKey(msg.String()) && m.canOpenDetail() {
			return m, m.openSelectedTerminal()
		}
		if isHelpKey(msg.String()) {
			m.screen = screenHelp
			return m, nil
		}
		if isDownKey(msg.String()) {
			m.moveSelection(1)
			return m, m.loadSelectedRecent()
		}
		if isUpKey(msg.String()) {
			m.moveSelection(-1)
			return m, m.loadSelectedRecent()
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, m.loadSelectedRecent()
	case overviewLoadedMsg:
		m.loading = false
		m.loadErr = nil
		m.config = msg.result.Config
		m.projects = msg.result.Projects
		m.recentByPath = map[string][]ovwformat.RecentCommit{}
		if msg.message != "" {
			m.message = msg.message
		}
		if msg.preservePath != "" {
			m.selectProjectPath(msg.preservePath)
		} else if m.selected >= len(m.projects) {
			m.selected = 0
		}
		return m, m.loadSelectedRecent()
	case overviewLoadFailedMsg:
		m.loading = false
		m.loadErr = msg.err
	case metadataSavedMsg:
		m.loading = false
		m.loadErr = nil
		m.config = msg.result.Config
		m.projects = msg.result.Projects
		m.recentByPath = map[string][]ovwformat.RecentCommit{}
		m.message = msg.message
		if msg.preservePath != "" {
			m.selectProjectPath(msg.preservePath)
		} else {
			m.clampSelection()
		}
		return m, m.loadSelectedRecent()
	case metadataFailedMsg:
		m.loading = false
		m.message = "Failed to write metadata: " + msg.err.Error()
	case editorOpenedMsg:
		m.message = msg.message
	case editorFailedMsg:
		m.message = "Editor failed: " + msg.err.Error()
	case terminalOpenedMsg:
		m.message = msg.message
	case terminalFailedMsg:
		m.message = "Terminal failed: " + msg.err.Error()
	case recentLoadedMsg:
		if m.recentByPath == nil {
			m.recentByPath = map[string][]ovwformat.RecentCommit{}
		}
		if msg.err == nil {
			m.recentByPath[msg.path] = msg.commits
		}
	}
	return m, nil
}

func (m Model) updateDetail(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch value := msg.String(); {
	case isEscapeKey(value):
		m.screen = screenTable
	case isVisibilityKey(value):
		m.screen = screenTable
		m.loading = true
		return m, m.toggleVisibility()
	}
	return m, nil
}

func (m Model) updateSearch(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	value := msg.String()
	switch {
	case isEscapeKey(value):
		m.searching = false
		m.search = ""
	case isEnterKey(value):
		m.searching = false
	case isBackspaceKey(value):
		runes := []rune(m.search)
		if len(runes) > 0 {
			m.search = string(runes[:len(runes)-1])
		}
	case msg.Type == tea.KeyRunes:
		m.search += string(msg.Runes)
	}
	m.clampSelection()
	return m, m.loadSelectedRecent()
}

func (m Model) updateFilter(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	options := m.filterOptions()
	switch value := msg.String(); {
	case isEscapeKey(value):
		m.screen = screenTable
	case isDownKey(value):
		if m.filterSelected < len(options)-1 {
			m.filterSelected++
		}
	case isUpKey(value):
		if m.filterSelected > 0 {
			m.filterSelected--
		}
	case isEnterKey(value):
		if len(options) == 0 {
			m.screen = screenTable
			return m, nil
		}
		m.applyFilter(options[m.filterSelected])
		m.screen = screenTable
		m.loading = true
		m.selected = 0
		return m, m.loadOverview()
	}
	return m, nil
}

func (m Model) updateSort(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	options := sortOptions()
	switch value := msg.String(); {
	case isEscapeKey(value):
		m.screen = screenTable
	case isDownKey(value):
		if m.sortSelected < len(options)-1 {
			m.sortSelected++
		}
	case isUpKey(value):
		if m.sortSelected > 0 {
			m.sortSelected--
		}
	case isEnterKey(value):
		m.applySort(options[m.sortSelected])
		m.screen = screenTable
		m.loading = true
		m.selected = 0
		return m, m.loadOverview()
	}
	return m, nil
}

func (m Model) updateNote(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch value := msg.String(); {
	case isEscapeKey(value):
		m.screen = screenTable
	case isEnterKey(value):
		m.screen = screenTable
		m.loading = true
		return m, m.saveNote()
	case isBackspaceKey(value):
		runes := []rune(m.noteInput)
		if len(runes) > 0 {
			m.noteInput = string(runes[:len(runes)-1])
		}
	default:
		m.noteInput += inputText(msg)
	}
	return m, nil
}

func (m Model) updateStatusPicker(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	options := m.statusOptions()
	switch value := msg.String(); {
	case isEscapeKey(value):
		m.screen = screenTable
	case isDownKey(value):
		if m.statusSelected < len(options)-1 {
			m.statusSelected++
		}
	case isUpKey(value):
		if m.statusSelected > 0 {
			m.statusSelected--
		}
	case isEnterKey(value):
		option := options[m.statusSelected]
		switch option.Kind {
		case statusOptionCustom:
			m.screen = screenStatusInput
			m.statusInput = ""
		case statusOptionClear:
			m.screen = screenTable
			m.loading = true
			return m, m.saveStatus("", "Status cleared")
		default:
			m.screen = screenTable
			m.loading = true
			return m, m.saveStatus(option.Value, "Status saved")
		}
	}
	return m, nil
}

func (m Model) updateStatusInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch value := msg.String(); {
	case isEscapeKey(value):
		m.screen = screenTable
	case isEnterKey(value):
		m.screen = screenTable
		m.loading = true
		return m, m.saveStatus(m.statusInput, "Status saved")
	case isBackspaceKey(value):
		runes := []rune(m.statusInput)
		if len(runes) > 0 {
			m.statusInput = string(runes[:len(runes)-1])
		}
	default:
		m.statusInput += inputText(msg)
	}
	return m, nil
}

func inputText(msg tea.KeyMsg) string {
	if msg.Type == tea.KeySpace {
		return " "
	}
	if msg.Type == tea.KeyRunes {
		return string(msg.Runes)
	}
	return ""
}

func (m Model) View() string {
	return renderShell(m)
}

func (m Model) Size() (int, int) {
	return m.width, m.height
}

func (m Model) canOpenDetail() bool {
	return !m.loading && m.loadErr == nil && len(m.visibleProjects()) > 0
}

func (m *Model) moveSelection(delta int) {
	if m.screen != screenTable {
		return
	}
	visible := m.visibleProjects()
	if len(visible) == 0 {
		m.selected = 0
		return
	}
	m.selected += delta
	if m.selected < 0 {
		m.selected = 0
	}
	last := len(visible) - 1
	if m.selected > last {
		m.selected = last
	}
}

func (m *Model) clampSelection() {
	visible := m.visibleProjects()
	if len(visible) == 0 || m.selected < 0 {
		m.selected = 0
		return
	}
	if m.selected >= len(visible) {
		m.selected = len(visible) - 1
	}
}

func Run() error {
	return RunWithOptions(app.Options{})
}

func RunWithOptions(opts app.Options) error {
	program := tea.NewProgram(NewWithOptions(opts), tea.WithAltScreen())
	_, err := program.Run()
	return err
}

type overviewLoadedMsg struct {
	result       app.OverviewResult
	preservePath string
	message      string
}

type overviewLoadFailedMsg struct {
	err error
}

type metadataSavedMsg struct {
	message      string
	result       app.OverviewResult
	preservePath string
}

type metadataFailedMsg struct {
	err error
}

type editorOpenedMsg struct {
	message string
}

type editorFailedMsg struct {
	err error
}

type terminalOpenedMsg struct {
	message string
}

type terminalFailedMsg struct {
	err error
}

type recentLoadedMsg struct {
	path    string
	commits []ovwformat.RecentCommit
	err     error
}

func (m Model) loadOverview() tea.Cmd {
	return m.reloadOverview("", "")
}

func (m Model) reloadOverview(preservePath, message string) tea.Cmd {
	return func() tea.Msg {
		result, err := m.loader(m.request)
		if err != nil {
			return overviewLoadFailedMsg{err: err}
		}
		return overviewLoadedMsg{result: result, preservePath: preservePath, message: message}
	}
}

func (m Model) saveNote() tea.Cmd {
	project, ok := m.currentProject()
	note := m.noteInput
	return func() tea.Msg {
		if !ok {
			return metadataFailedMsg{err: errNoProjectSelected{}}
		}
		if _, err := m.updater(project.Path, app.MetadataUpdate{Note: &note}); err != nil {
			return metadataFailedMsg{err: err}
		}
		result, err := m.loader(m.request)
		if err != nil {
			return overviewLoadFailedMsg{err: err}
		}
		return metadataSavedMsg{message: "Note saved", result: result, preservePath: project.Path}
	}
}

func (m Model) saveStatus(status, message string) tea.Cmd {
	project, ok := m.currentProject()
	return func() tea.Msg {
		if !ok {
			return metadataFailedMsg{err: errNoProjectSelected{}}
		}
		if _, err := m.updater(project.Path, app.MetadataUpdate{Status: &status}); err != nil {
			return metadataFailedMsg{err: err}
		}
		result, err := m.loader(m.request)
		if err != nil {
			return overviewLoadFailedMsg{err: err}
		}
		return metadataSavedMsg{message: message, result: result, preservePath: project.Path}
	}
}

func (m Model) toggleVisibility() tea.Cmd {
	project, ok := m.currentProject()
	hidden := !project.Hidden
	return func() tea.Msg {
		if !ok {
			return metadataFailedMsg{err: errNoProjectSelected{}}
		}
		visible := m.visible
		if visible == nil {
			visible = app.SetProjectHidden
		}
		if _, err := visible(project.Path, hidden); err != nil {
			return metadataFailedMsg{err: err}
		}
		result, err := m.loader(m.request)
		if err != nil {
			return overviewLoadFailedMsg{err: err}
		}
		message := "Project hidden"
		if !hidden {
			message = "Project unhidden"
		}
		preservePath := project.Path
		if hidden {
			preservePath = ""
		}
		return metadataSavedMsg{message: message, result: result, preservePath: preservePath}
	}
}

func (m Model) openSelectedProject() tea.Cmd {
	project, ok := m.currentProject()
	editor := m.config.Editor
	return func() tea.Msg {
		if !ok {
			return editorFailedMsg{err: errNoProjectSelected{}}
		}
		if err := m.editor(editor, project.Path); err != nil {
			return editorFailedMsg{err: err}
		}
		return editorOpenedMsg{message: "Opened " + project.Name}
	}
}

func (m Model) openSelectedTerminal() tea.Cmd {
	project, ok := m.currentProject()
	if !ok {
		return func() tea.Msg {
			return terminalFailedMsg{err: errNoProjectSelected{}}
		}
	}
	runner := m.terminal
	if runner == nil {
		runner = runTerminal
	}
	return runner(project.Path, project.Name)
}

func (m Model) loadSelectedRecent() tea.Cmd {
	if !m.showInlineDetail() {
		return nil
	}
	project, ok := m.currentProject()
	if !ok || !project.Activity.HasGit || !project.Activity.HasCommits {
		return nil
	}
	if _, ok := m.recentByPath[project.Path]; ok {
		return nil
	}
	path := project.Path
	loader := m.recent
	if loader == nil {
		loader = loadRecentCommits
	}
	return func() tea.Msg {
		commits, err := loader(path, time.Now())
		return recentLoadedMsg{path: path, commits: commits, err: err}
	}
}

func loadRecentCommits(path string, now time.Time) ([]ovwformat.RecentCommit, error) {
	commits, err := gitactivity.Recent(path, 3)
	if err != nil {
		return nil, err
	}
	return ovwformat.RecentCommits(commits, now), nil
}

type errNoProjectSelected struct{}

func (errNoProjectSelected) Error() string {
	return "no project selected"
}

func renderShell(m Model) string {
	body := titleStyle.Render("ovw")
	switch {
	case m.loading && len(m.projects) == 0:
		body += "\n\n" + mutedStyle.Render("Loading projects...")
	case m.loadErr != nil:
		body += "\n\n" + errorStyle.Render("Failed to load projects: "+m.loadErr.Error())
	default:
		visible := m.visibleProjects()
		body = headerView(m)
		if m.message != "" {
			body += " " + mutedStyle.Render(m.message)
		}
		if len(visible) == 0 && m.search != "" {
			body += "\n\n" + mutedStyle.Render("No projects match search")
		} else {
			content := m.tablePanel(visible)
			switch m.screen {
			case screenDetail:
				project, ok := m.currentProject()
				content = overlayModal(content, detailModalView(project, ok, m.contentWidth()), m.contentWidth())
			case screenHelp:
				content = overlayModal(content, helpView(), m.contentWidth())
			case screenFilter:
				content = overlayModal(content, filterView(m.filterOptions(), m.filterSelected), m.contentWidth())
			case screenSort:
				content = overlayModal(content, sortView(sortOptions(), m.sortSelected), m.contentWidth())
			case screenNote:
				project, _ := m.currentProject()
				content = overlayModal(content, noteView(m.noteInput, project.Note.Display), m.contentWidth())
			case screenStatus:
				content = overlayModal(content, statusView(m.statusOptions(), m.statusSelected), m.contentWidth())
			case screenStatusInput:
				content = overlayModal(content, statusInputView(m.statusInput), m.contentWidth())
			}
			body += "\n\n" + content
		}
	}
	body += "\n\n" + footerView()
	return body
}

func headerView(m Model) string {
	parts := []string{
		formatProjectCount(len(m.projects)),
		selectedPosition(m.selected, len(m.visibleProjects())),
		"filter: " + headerFilter(m.activeFilter),
	}
	if m.activeSort != "" {
		parts = append(parts, "sort: "+m.activeSort)
	}
	if m.search != "" || m.searching {
		parts = append(parts, "search: "+m.searchDisplay())
	}
	return titleStyle.Render("ovw") + "  " + mutedStyle.Render(strings.Join(parts, "  "))
}

func selectedPosition(selected, total int) string {
	if total <= 0 {
		return "0/0"
	}
	if selected < 0 {
		selected = 0
	}
	if selected >= total {
		selected = total - 1
	}
	return fmt.Sprintf("%d/%d", selected+1, total)
}

func headerFilter(value string) string {
	if value == "" {
		return "all"
	}
	return value
}

func (m Model) tablePanel(visible []project.Project) string {
	contentWidth := m.contentWidth()
	tableHeight := m.tableHeight()
	if m.showInlineDetail() {
		gap := 3
		tableWidth, detailWidth := splitPanelWidths(contentWidth, gap)
		if tableWidth < 40 {
			tableWidth = contentWidth
		} else {
			detail, _ := m.currentProject()
			if commits, ok := m.recentByPath[detail.Path]; ok {
				detail.Activity.RecentCommits = commits
			}
			return joinColumns(
				tableView(visible, m.selected, tableWidth, tableHeight),
				detailSummaryView(detail, detailWidth),
				gap,
			)
		}
	}
	return tableView(visible, m.selected, contentWidth, tableHeight)
}

func (m Model) showInlineDetail() bool {
	return m.isTableLayoutScreen() && m.contentWidth() >= 110 && len(m.visibleProjects()) > 0
}

func (m Model) isTableLayoutScreen() bool {
	switch m.screen {
	case screenTable, screenDetail, screenHelp, screenFilter, screenSort, screenNote, screenStatus, screenStatusInput:
		return true
	default:
		return false
	}
}

func (m Model) contentWidth() int {
	return m.width
}

func (m Model) tableHeight() int {
	if m.height <= 0 {
		return 0
	}
	height := m.height - 4
	if height < 4 {
		return 4
	}
	return height
}

func splitPanelWidths(width, gap int) (int, int) {
	dividerWidth := 2
	detailWidth := width * 30 / 100
	if detailWidth < 38 {
		detailWidth = 38
	}
	if detailWidth > 48 {
		detailWidth = 48
	}
	tableWidth := width - detailWidth - gap - dividerWidth
	if tableWidth < 72 {
		detailWidth = width - 72 - gap - dividerWidth
		if detailWidth < 0 {
			detailWidth = 0
		}
		tableWidth = width - detailWidth - gap - dividerWidth
	}
	return tableWidth, detailWidth
}

func joinColumns(left, right string, gap int) string {
	leftLines := strings.Split(left, "\n")
	rightLines := strings.Split(right, "\n")
	leftWidth := maxLineWidth(leftLines)
	lineCount := len(leftLines)
	if len(rightLines) > lineCount {
		lineCount = len(rightLines)
	}
	lines := make([]string, 0, lineCount)
	spacer := strings.Repeat(" ", gap)
	for i := 0; i < lineCount; i++ {
		leftLine := ""
		if i < len(leftLines) {
			leftLine = leftLines[i]
		}
		rightLine := ""
		if i < len(rightLines) {
			rightLine = rightLines[i]
		}
		lines = append(lines, padRight(leftLine, leftWidth)+spacer+"│ "+rightLine)
	}
	return strings.Join(lines, "\n")
}

func maxLineWidth(lines []string) int {
	width := 0
	for _, line := range lines {
		if lineWidth := lipgloss.Width(line); lineWidth > width {
			width = lineWidth
		}
	}
	return width
}

func padRight(value string, width int) string {
	valueWidth := lipgloss.Width(value)
	if valueWidth >= width {
		return value
	}
	return value + strings.Repeat(" ", width-valueWidth)
}

func (m Model) currentProject() (project.Project, bool) {
	visible := m.visibleProjects()
	if len(visible) == 0 || m.selected < 0 || m.selected >= len(visible) {
		return project.Project{}, false
	}
	return visible[m.selected], true
}

func (m Model) selectedProjectPath() string {
	project, ok := m.currentProject()
	if !ok {
		return ""
	}
	return project.Path
}

func (m Model) searchDisplay() string {
	if m.searching {
		return searchInputLine(m.search, "")
	}
	return m.search
}

func (m *Model) selectProjectPath(path string) {
	visible := m.visibleProjects()
	for index, project := range visible {
		if project.Path == path {
			m.selected = index
			return
		}
	}
	m.clampSelection()
}

func (m Model) visibleProjects() []project.Project {
	query := strings.TrimSpace(strings.ToLower(m.search))
	if query == "" {
		return m.projects
	}
	visible := make([]project.Project, 0, len(m.projects))
	for _, project := range m.projects {
		if projectMatchesSearch(project, query) {
			visible = append(visible, project)
		}
	}
	return visible
}

func projectMatchesSearch(project project.Project, query string) bool {
	values := []string{
		project.Name,
		project.Path,
		project.StackDisplay,
		strings.Join(project.Managers, " "),
		project.Status.Display,
		project.Note.Display,
	}
	for _, value := range values {
		if strings.Contains(strings.ToLower(value), query) {
			return true
		}
	}
	return false
}

func formatProjectCount(count int) string {
	if count == 1 {
		return "1 project"
	}
	return fmt.Sprintf("%d projects", count)
}
