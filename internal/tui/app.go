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

type screenMode int

const (
	screenTable screenMode = iota
	screenDetail
)

type Model struct {
	request app.Options
	loader  overviewLoader

	width     int
	height    int
	selected  int
	screen    screenMode
	search    string
	searching bool
	loading   bool
	loadErr   error
	config    config.Config
	projects  []project.Project
}

func New() Model {
	return NewWithOptions(app.Options{})
}

func NewWithOptions(opts app.Options) Model {
	if opts.Out == nil {
		opts.Out = io.Discard
	}
	return Model{
		request: opts,
		loader:  app.LoadOverview,
		loading: true,
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
		if m.selected >= len(m.projects) {
			m.selected = 0
		}
	case overviewLoadFailedMsg:
		m.loading = false
		m.loadErr = msg.err
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
	if m.screen == screenDetail {
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
	result app.OverviewResult
}

type overviewLoadFailedMsg struct {
	err error
}

func (m Model) loadOverview() tea.Cmd {
	return func() tea.Msg {
		result, err := m.loader(m.request)
		if err != nil {
			return overviewLoadFailedMsg{err: err}
		}
		return overviewLoadedMsg{result: result}
	}
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
		if m.search != "" || m.searching {
			body += " " + mutedStyle.Render("search: "+m.search)
		}
		if m.width > 0 && m.height > 0 {
			body += "\n" + mutedStyle.Render(fmt.Sprintf("%dx%d", m.width, m.height))
		}
		if m.screen == screenDetail {
			body += "\n\n" + detailView(m.currentProject())
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
