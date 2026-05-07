package tui

import (
	"fmt"
	"io"

	"ovw/internal/app"
	"ovw/internal/config"
	"ovw/internal/project"

	tea "github.com/charmbracelet/bubbletea"
)

type overviewLoader func(app.Options) (app.OverviewResult, error)

type Model struct {
	request app.Options
	loader  overviewLoader

	width    int
	height   int
	selected int
	loading  bool
	loadErr  error
	config   config.Config
	projects []project.Project
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
		if isQuitKey(msg.String()) {
			return m, tea.Quit
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

func (m Model) View() string {
	return renderShell(m)
}

func (m Model) Size() (int, int) {
	return m.width, m.height
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
		body = titleStyle.Render("ovw") + " " + mutedStyle.Render("- "+formatProjectCount(len(m.projects)))
		if m.width > 0 && m.height > 0 {
			body += "\n" + mutedStyle.Render(fmt.Sprintf("%dx%d", m.width, m.height))
		}
		body += "\n\n" + tableView(m.projects, m.selected, m.width)
	}
	body += "\n\n" + footerView()
	return appStyle.Render(body)
}

func formatProjectCount(count int) string {
	if count == 1 {
		return "1 project"
	}
	return fmt.Sprintf("%d projects", count)
}
