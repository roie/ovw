package tui

import (
	"errors"
	"strings"

	"ovw/internal/app"
	"ovw/internal/config"

	tea "github.com/charmbracelet/bubbletea"
)

var ErrSetupCancelled = errors.New("setup cancelled")

type setupModel struct {
	rootPicker
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
		rootPicker: newRootPicker(candidates, roots),
		creator:    app.CreateConfigRoots,
		counter: func(paths []string) map[string]int {
			return app.CountProjectRootsWithConfig(paths, cfg)
		},
	}
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

func (m setupModel) Init() tea.Cmd {
	return m.countSetupPaths(m.visiblePaths())
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
	return onboardingSetupView(m.visibleRows(), m.selected, errText)
}

func (m setupModel) updatePicker(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	rows := m.visibleRows()
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
			children := m.expandPath(rows[m.selected].Path)
			return m, m.countSetupPaths(children)
		}
	case isLeftKey(value):
		if m.selected < len(rows) {
			row := rows[m.selected]
			if row.Depth > 0 {
				m.selectPath(row.Parent)
				break
			}
			if row.Expandable || row.Expanded {
				m.expanded[row.Path] = false
			}
		}
	case value == " ":
		if m.selected < len(rows) {
			m.toggleRow(rows[m.selected])
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

func (m setupModel) countSetupPaths(paths []string) tea.Cmd {
	missing := m.missingCountPaths(paths)
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
