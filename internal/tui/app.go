package tui

import (
	"fmt"
	"io"
	"strings"

	"ovw/internal/app"
	"ovw/internal/config"
	ovwformat "ovw/internal/format"
	"ovw/internal/project"

	tea "github.com/charmbracelet/bubbletea"
)

type overviewLoader func(app.Options) (app.OverviewResult, error)
type metadataUpdater func(string, app.MetadataUpdate) (app.MetadataUpdateResult, error)

type screenMode int

const (
	screenTable screenMode = iota
	screenDetail
	screenFilter
	screenSort
	screenNote
	screenStatus
	screenStatusInput
)

type Model struct {
	request app.Options
	loader  overviewLoader
	updater metadataUpdater

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
		activeFilter: optionsFromRequest(opts),
		activeSort:   sortFromRequest(opts),
		loading:      true,
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
		if isEscapeKey(msg.String()) && m.screen == screenDetail {
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
			m.noteInput = project.Note.Manual
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
		if isDownKey(msg.String()) {
			m.moveSelection(1)
			return m, nil
		}
		if isUpKey(msg.String()) {
			m.moveSelection(-1)
			return m, nil
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case overviewLoadedMsg:
		m.loading = false
		m.loadErr = nil
		m.config = msg.result.Config
		m.projects = msg.result.Projects
		if msg.message != "" {
			m.message = msg.message
		}
		if msg.preservePath != "" {
			m.selectProjectPath(msg.preservePath)
		} else if m.selected >= len(m.projects) {
			m.selected = 0
		}
	case overviewLoadFailedMsg:
		m.loading = false
		m.loadErr = msg.err
	case metadataSavedMsg:
		m.loading = false
		m.loadErr = nil
		m.config = msg.result.Config
		m.projects = msg.result.Projects
		m.message = msg.message
		m.clampSelection()
	case metadataFailedMsg:
		m.loading = false
		m.message = "Failed to write metadata: " + msg.err.Error()
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
	return m, nil
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
	case msg.Type == tea.KeyRunes:
		m.noteInput += string(msg.Runes)
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
	case msg.Type == tea.KeyRunes:
		m.statusInput += string(msg.Runes)
	}
	return m, nil
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
	program := tea.NewProgram(New(), tea.WithAltScreen())
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
	message string
	result  app.OverviewResult
}

type metadataFailedMsg struct {
	err error
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
		return metadataSavedMsg{message: "Note saved", result: result}
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
		return metadataSavedMsg{message: message, result: result}
	}
}

type errNoProjectSelected struct{}

func (errNoProjectSelected) Error() string {
	return "no project selected"
}

func renderShell(m Model) string {
	body := titleStyle.Render("ovw")
	if m.width > 0 && m.height > 0 {
		body += "\n" + mutedStyle.Render(fmt.Sprintf("%dx%d", m.width, m.height))
	}
	switch {
	case m.loading:
		body += "\n\n" + mutedStyle.Render("Loading projects...")
	case m.loadErr != nil:
		body += "\n\n" + errorStyle.Render("Failed to load projects: "+m.loadErr.Error())
	default:
		visible := m.visibleProjects()
		body = titleStyle.Render("ovw") + " " + mutedStyle.Render("- "+formatProjectCount(len(m.projects)))
		if m.activeFilter != "" && m.activeFilter != "all" {
			body += " " + mutedStyle.Render("filter: "+m.activeFilter)
		}
		if m.activeSort != "" {
			body += " " + mutedStyle.Render("sort: "+m.activeSort)
		}
		if m.message != "" {
			body += " " + mutedStyle.Render(m.message)
		}
		if m.search != "" || m.searching {
			body += " " + mutedStyle.Render("search: "+m.search)
		}
		if m.width > 0 && m.height > 0 {
			body += "\n" + mutedStyle.Render(fmt.Sprintf("%dx%d", m.width, m.height))
		}
		if m.screen == screenDetail {
			body += "\n\n" + detailView(m.currentProject())
		} else if m.screen == screenFilter {
			body += "\n\n" + filterView(m.filterOptions(), m.filterSelected)
		} else if m.screen == screenSort {
			body += "\n\n" + sortView(sortOptions(), m.sortSelected)
		} else if m.screen == screenNote {
			body += "\n\n" + noteView(m.noteInput)
		} else if m.screen == screenStatus {
			body += "\n\n" + statusView(m.statusOptions(), m.statusSelected)
		} else if m.screen == screenStatusInput {
			body += "\n\n" + statusInputView(m.statusInput)
		} else if len(visible) == 0 && m.search != "" {
			body += "\n\n" + mutedStyle.Render("No projects match search")
		} else {
			body += "\n\n" + tableView(visible, m.selected, m.width)
		}
	}
	body += "\n\n" + footerView()
	return appStyle.Render(body)
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
		ovwformat.TagDisplay(project.Tags),
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
