package tui

import (
	"fmt"
	"io"
	"os"
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
type configSetupLoader func(app.Options) (app.ConfigSetup, error)
type configRootsCreator func([]string) (config.FilePaths, config.Config, error)
type metadataUpdater func(string, app.MetadataUpdate) (app.MetadataUpdateResult, error)
type visibilityUpdater func(string, bool) (app.MetadataUpdateResult, error)
type projectAdder func(string) (app.AddProjectResult, error)
type editorRunner func(string, string) error
type terminalRunner func(string, string, string) tea.Cmd
type recentLoader func(string, time.Time) ([]ovwformat.RecentCommit, error)
type configWriter func(string, config.Config) error

var terminalTitleWriter io.Writer = os.Stdout

type screenMode int

const (
	screenTable screenMode = iota
	screenDetail
	screenAdd
	screenFilter
	screenSort
	screenNote
	screenStatus
	screenStatusInput
	screenHelp
	screenOnboarding
	screenOnboardingInput
	screenColumns
)

type Model struct {
	request      app.Options
	loader       overviewLoader
	setup        configSetupLoader
	roots        configRootsCreator
	updater      metadataUpdater
	visible      visibilityUpdater
	adder        projectAdder
	editor       editorRunner
	terminal     terminalRunner
	recent       recentLoader
	configWriter configWriter

	width           int
	height          int
	selected        int
	tableXOffset    int
	detailYOffset   int
	detailModalY    int
	detailsExpanded bool
	screen          screenMode
	search          string
	searchCursor    int
	searching       bool
	filterSelected  int
	activeFilter    string
	sortSelected    int
	activeSort      string
	activeSortDir   string
	noteInput       string
	noteCursor      int
	addInput        string
	addCursor       int
	addErr          string
	statusSelected  int
	statusInput     string
	statusCursor    int
	onboardOptions  []string
	onboardChecked  map[string]bool
	onboardSelected int
	onboardInput    string
	onboardCursor   int
	onboardErr      string
	columnSelected  int
	columnOrder     []string
	columnChecked   map[string]bool
	columnErr       string
	message         string
	loading         bool
	loadErr         error
	config          config.Config
	configPaths     config.FilePaths
	projects        []project.Project
	scanElapsed     time.Duration
	recentByPath    map[string][]ovwformat.RecentCommit
}

func New() Model {
	return NewWithOptions(app.Options{})
}

func NewWithOptions(opts app.Options) Model {
	if opts.Out == nil {
		opts.Out = io.Discard
	}
	return Model{
		request:       opts,
		loader:        app.LoadOverview,
		setup:         app.CheckConfig,
		roots:         app.CreateConfigRoots,
		updater:       app.UpdateProjectMetadata,
		visible:       app.SetProjectHidden,
		adder:         app.AddProject,
		editor:        OpenEditor,
		terminal:      runTerminal,
		recent:        loadRecentCommits,
		configWriter:  config.Write,
		activeFilter:  optionsFromRequest(opts),
		activeSort:    sortFromRequest(opts),
		activeSortDir: sortDirFromRequest(opts),
		loading:       true,
		recentByPath:  map[string][]ovwformat.RecentCommit{},
	}
}

func NewWithLoader(loader overviewLoader) Model {
	model := NewWithOptions(app.Options{})
	model.loader = loader
	model.setup = func(app.Options) (app.ConfigSetup, error) {
		return app.ConfigSetup{Exists: true}, nil
	}
	return model
}

func (m Model) Init() tea.Cmd {
	return m.startup()
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.screen == screenOnboarding {
			return m.updateOnboarding(msg)
		}
		if m.screen == screenOnboardingInput {
			return m.updateOnboardingInput(msg)
		}
		if m.screen == screenFilter {
			return m.updateFilter(msg)
		}
		if m.screen == screenSort {
			return m.updateSort(msg)
		}
		if m.screen == screenColumns {
			return m.updateColumns(msg)
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
		if m.screen == screenAdd {
			return m.updateAdd(msg)
		}
		if (isEscapeKey(msg.String()) || isEnterKey(msg.String())) && m.screen == screenHelp {
			m.screen = screenTable
			return m, nil
		}
		if isEscapeKey(msg.String()) && m.searching {
			m.searching = false
			return m, nil
		}
		if isEscapeKey(msg.String()) && m.search != "" {
			m.search = ""
			m.searchCursor = 0
			m.clampSelection()
			return m, m.loadSelectedRecent()
		}
		if isQuitKey(msg.String()) {
			return m, tea.Quit
		}
		if isEnterKey(msg.String()) && m.canOpenDetail() {
			m.screen = screenDetail
			m.detailModalY = 0
			m.detailsExpanded = false
			return m, nil
		}
		if m.screen == screenTable && m.showInlineDetail() && isDetailScrollKey(msg.String()) {
			m.scrollDetail(msg.String())
			return m, nil
		}
		if m.searching && isTextInputKey(msg) {
			return m.updateSearch(msg)
		}
		if isRightKey(msg.String()) {
			m.tableXOffset += 8
			return m, nil
		}
		if isLeftKey(msg.String()) {
			m.tableXOffset -= 8
			if m.tableXOffset < 0 {
				m.tableXOffset = 0
			}
			return m, nil
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
		if isSearchKey(msg.String()) {
			m.screen = screenTable
			m.searching = true
			m.searchCursor = textCursor(m.search, m.searchCursor)
			return m, nil
		}
		if isAddKey(msg.String()) && !m.loading {
			m.screen = screenAdd
			m.addInput = ""
			m.addCursor = 0
			m.addErr = ""
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
		if isColumnsKey(msg.String()) {
			m.openColumns()
			return m, nil
		}
		if isNoteKey(msg.String()) && m.canOpenDetail() {
			project, _ := m.currentProject()
			m.screen = screenNote
			m.noteInput = project.Note.Value
			m.noteCursor = len([]rune(m.noteInput))
			return m, nil
		}
		if isStatusKey(msg.String()) && m.canOpenDetail() {
			project, _ := m.currentProject()
			m.screen = screenStatus
			m.statusSelected = m.currentStatusIndex(project.Status.Value)
			return m, nil
		}
		if isPinKey(msg.String()) && m.canOpenDetail() {
			m.loading = true
			return m, m.togglePin()
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
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.clampDetailOffset()
		m.clampDetailModalOffset()
		return m, m.loadSelectedRecent()
	case overviewLoadedMsg:
		m.loading = false
		m.loadErr = nil
		m.screen = screenTable
		m.config = msg.result.Config
		m.configPaths = msg.result.Paths
		m.projects = msg.result.Projects
		m.scanElapsed = msg.result.Elapsed
		m.syncActiveSort()
		m.recentByPath = map[string][]ovwformat.RecentCommit{}
		if msg.message != "" {
			m.message = msg.message
		}
		if msg.preservePath != "" {
			m.selectProjectPath(msg.preservePath)
		} else if m.selected >= len(m.projects) {
			m.selected = 0
			m.detailYOffset = 0
		}
		return m, m.loadSelectedRecent()
	case overviewLoadFailedMsg:
		m.loading = false
		m.loadErr = msg.err
	case onboardingLoadedMsg:
		m.loading = false
		m.loadErr = nil
		m.screen = screenOnboarding
		m.onboardOptions = msg.candidates
		m.onboardChecked = checkedOnboardingOptions(msg.candidates)
		m.onboardSelected = 0
		m.detailYOffset = 0
		m.onboardInput = ""
		m.onboardErr = ""
	case onboardingFailedMsg:
		m.loading = false
		m.onboardErr = msg.err.Error()
	case onboardingCreatedMsg:
		m.loading = false
		m.loadErr = nil
		m.screen = screenTable
		m.config = msg.result.Config
		m.configPaths = msg.result.Paths
		m.projects = msg.result.Projects
		m.scanElapsed = msg.result.Elapsed
		m.syncActiveSort()
		m.recentByPath = map[string][]ovwformat.RecentCommit{}
		m.message = "Project root saved"
		m.detailYOffset = 0
		return m, m.loadSelectedRecent()
	case metadataSavedMsg:
		m.loading = false
		m.loadErr = nil
		m.config = msg.result.Config
		m.configPaths = msg.result.Paths
		m.projects = msg.result.Projects
		m.scanElapsed = msg.result.Elapsed
		m.syncActiveSort()
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
	case addProjectSavedMsg:
		m.loading = false
		m.loadErr = nil
		m.config = msg.result.Config
		m.configPaths = msg.result.Paths
		m.projects = msg.result.Projects
		m.scanElapsed = msg.result.Elapsed
		m.recentByPath = map[string][]ovwformat.RecentCommit{}
		m.screen = screenTable
		m.message = msg.message
		m.addInput = ""
		m.addErr = ""
		m.selectProjectPath(msg.preservePath)
		return m, m.loadSelectedRecent()
	case addProjectFailedMsg:
		m.loading = false
		m.screen = screenAdd
		m.addErr = msg.err.Error()
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
	case columnsSavedMsg:
		m.loading = false
		if msg.err != nil {
			m.message = "Failed to write columns: " + msg.err.Error()
			return m, nil
		}
		m.config = msg.config
		m.tableXOffset = 0
		m.message = "Columns saved"
	}
	return m, nil
}

func (m Model) updateDetail(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch value := msg.String(); {
	case isEscapeKey(value), isEnterKey(value):
		m.screen = screenTable
		m.detailModalY = 0
		m.detailsExpanded = false
	case isExpandKey(value):
		project, ok := m.currentProject()
		if ok && detailModalHasCompactContent(project) {
			m.detailsExpanded = !m.detailsExpanded
			m.detailModalY = 0
		}
	case isDetailScrollKey(value):
		m.scrollDetailModal(value)
	case isVisibilityKey(value):
		m.screen = screenTable
		m.detailModalY = 0
		m.detailsExpanded = false
		m.loading = true
		return m, m.toggleVisibility()
	case isPinKey(value):
		m.screen = screenTable
		m.detailModalY = 0
		m.detailsExpanded = false
		m.loading = true
		return m, m.togglePin()
	}
	return m, nil
}

func (m Model) updateAdd(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch value := msg.String(); {
	case isEscapeKey(value):
		m.screen = screenTable
		m.addInput = ""
		m.addCursor = 0
		m.addErr = ""
	case isEnterKey(value):
		m.loading = true
		m.addErr = ""
		return m, m.addProject()
	case value == "left":
		m.addCursor = textMoveLeft(m.addInput, m.addCursor)
	case value == "right":
		m.addCursor = textMoveRight(m.addInput, m.addCursor)
	case isMoveStartKey(value):
		m.addCursor = textMoveStart(m.addInput, m.addCursor)
	case isMoveEndKey(value):
		m.addCursor = textMoveEnd(m.addInput, m.addCursor)
	case isClearBeforeKey(value):
		m.addInput, m.addCursor = textClearBefore(m.addInput, m.addCursor)
	case isClearAfterKey(value):
		m.addInput, m.addCursor = textClearAfter(m.addInput, m.addCursor)
	case isDeletePreviousWordKey(value):
		m.addInput, m.addCursor = textDeletePreviousWord(m.addInput, m.addCursor)
	case isBackspaceKey(value):
		m.addInput, m.addCursor = textBackspace(m.addInput, m.addCursor)
	case isDeleteKey(value):
		m.addInput, m.addCursor = textDelete(m.addInput, m.addCursor)
	default:
		m.addInput, m.addCursor = textInsert(m.addInput, m.addCursor, inputText(msg))
	}
	return m, nil
}

func (m Model) updateSearch(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	value := msg.String()
	switch {
	case value == "left":
		m.searchCursor = textMoveLeft(m.search, m.searchCursor)
	case value == "right":
		m.searchCursor = textMoveRight(m.search, m.searchCursor)
	case isMoveStartKey(value):
		m.searchCursor = textMoveStart(m.search, m.searchCursor)
	case isMoveEndKey(value):
		m.searchCursor = textMoveEnd(m.search, m.searchCursor)
	case isClearBeforeKey(value):
		m.search, m.searchCursor = textClearBefore(m.search, m.searchCursor)
	case isClearAfterKey(value):
		m.search, m.searchCursor = textClearAfter(m.search, m.searchCursor)
	case isDeletePreviousWordKey(value):
		m.search, m.searchCursor = textDeletePreviousWord(m.search, m.searchCursor)
	case isBackspaceKey(value):
		m.search, m.searchCursor = textBackspace(m.search, m.searchCursor)
	case isDeleteKey(value):
		m.search, m.searchCursor = textDelete(m.search, m.searchCursor)
	case msg.Type == tea.KeyRunes:
		m.search, m.searchCursor = textInsert(m.search, m.searchCursor, string(msg.Runes))
	}
	m.clampSelection()
	m.clampDetailOffset()
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
	case isLeftKey(value), isRightKey(value):
		m.toggleSortDir()
	case isEnterKey(value):
		m.applySort(options[m.sortSelected])
		m.screen = screenTable
		m.loading = true
		m.selected = 0
		return m, m.loadOverview()
	}
	return m, nil
}

func (m Model) updateColumns(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch value := msg.String(); {
	case isEscapeKey(value):
		m.screen = screenTable
		m.columnErr = ""
	case isEnterKey(value):
		m.screen = screenTable
		m.loading = true
		m.columnErr = ""
		return m, m.saveColumns()
	case isDownKey(value):
		if m.columnSelected < len(m.columnOrder)-1 {
			m.columnSelected++
		}
	case isUpKey(value):
		if m.columnSelected > 0 {
			m.columnSelected--
		}
	case isLeftKey(value):
		m.moveColumn(-1)
	case isRightKey(value):
		m.moveColumn(1)
	case value == " ":
		m.toggleColumn()
	}
	return m, nil
}

func (m Model) updateOnboarding(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch value := msg.String(); {
	case isQuitKey(value):
		return m, tea.Quit
	case isDownKey(value):
		if m.onboardSelected < len(m.onboardOptions) {
			m.onboardSelected++
		}
	case isUpKey(value):
		if m.onboardSelected > 0 {
			m.onboardSelected--
		}
	case value == " ":
		if m.onboardSelected < len(m.onboardOptions) {
			m.toggleOnboardingOption(m.onboardOptions[m.onboardSelected])
		}
	case isEnterKey(value):
		if m.onboardSelected >= len(m.onboardOptions) {
			m.screen = screenOnboardingInput
			m.onboardInput = ""
			m.onboardCursor = 0
			m.onboardErr = ""
			return m, nil
		}
		m.loading = true
		m.onboardErr = ""
		return m, m.createOnboardingConfigRoots(m.selectedOnboardingRoots())
	}
	return m, nil
}

func (m Model) updateOnboardingInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch value := msg.String(); {
	case isEscapeKey(value):
		m.screen = screenOnboarding
		m.onboardErr = ""
	case isEnterKey(value):
		m.loading = true
		m.onboardErr = ""
		root := strings.TrimSpace(m.onboardInput)
		if root == "" {
			return m, m.createOnboardingConfigRoots(nil)
		}
		roots := m.selectedOnboardingRoots()
		roots = append(roots, root)
		return m, m.createOnboardingConfigRoots(roots)
	case value == "left":
		m.onboardCursor = textMoveLeft(m.onboardInput, m.onboardCursor)
	case value == "right":
		m.onboardCursor = textMoveRight(m.onboardInput, m.onboardCursor)
	case isMoveStartKey(value):
		m.onboardCursor = textMoveStart(m.onboardInput, m.onboardCursor)
	case isMoveEndKey(value):
		m.onboardCursor = textMoveEnd(m.onboardInput, m.onboardCursor)
	case isClearBeforeKey(value):
		m.onboardInput, m.onboardCursor = textClearBefore(m.onboardInput, m.onboardCursor)
	case isClearAfterKey(value):
		m.onboardInput, m.onboardCursor = textClearAfter(m.onboardInput, m.onboardCursor)
	case isDeletePreviousWordKey(value):
		m.onboardInput, m.onboardCursor = textDeletePreviousWord(m.onboardInput, m.onboardCursor)
	case isBackspaceKey(value):
		m.onboardInput, m.onboardCursor = textBackspace(m.onboardInput, m.onboardCursor)
	case isDeleteKey(value):
		m.onboardInput, m.onboardCursor = textDelete(m.onboardInput, m.onboardCursor)
	default:
		m.onboardInput, m.onboardCursor = textInsert(m.onboardInput, m.onboardCursor, inputText(msg))
	}
	return m, nil
}

func checkedOnboardingOptions(options []string) map[string]bool {
	checked := make(map[string]bool, len(options))
	if len(options) > 0 {
		checked[options[0]] = true
	}
	return checked
}

func (m *Model) toggleOnboardingOption(value string) {
	if m.onboardChecked == nil {
		m.onboardChecked = checkedOnboardingOptions(m.onboardOptions)
	}
	m.onboardChecked[value] = !m.onboardChecked[value]
}

func (m Model) selectedOnboardingRoots() []string {
	roots := make([]string, 0, len(m.onboardOptions))
	for _, option := range m.onboardOptions {
		if m.onboardChecked[option] {
			roots = append(roots, option)
		}
	}
	return roots
}

func (m Model) updateNote(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch value := msg.String(); {
	case isEscapeKey(value):
		m.screen = screenTable
	case isEnterKey(value):
		m.screen = screenTable
		m.loading = true
		return m, m.saveNote()
	case value == "left":
		m.noteCursor = textMoveLeft(m.noteInput, m.noteCursor)
	case value == "right":
		m.noteCursor = textMoveRight(m.noteInput, m.noteCursor)
	case isMoveStartKey(value):
		m.noteCursor = textMoveStart(m.noteInput, m.noteCursor)
	case isMoveEndKey(value):
		m.noteCursor = textMoveEnd(m.noteInput, m.noteCursor)
	case isClearBeforeKey(value):
		m.noteInput, m.noteCursor = textClearBefore(m.noteInput, m.noteCursor)
	case isClearAfterKey(value):
		m.noteInput, m.noteCursor = textClearAfter(m.noteInput, m.noteCursor)
	case isDeletePreviousWordKey(value):
		m.noteInput, m.noteCursor = textDeletePreviousWord(m.noteInput, m.noteCursor)
	case isBackspaceKey(value):
		m.noteInput, m.noteCursor = textBackspace(m.noteInput, m.noteCursor)
	case isDeleteKey(value):
		m.noteInput, m.noteCursor = textDelete(m.noteInput, m.noteCursor)
	default:
		m.noteInput, m.noteCursor = textInsert(m.noteInput, m.noteCursor, inputText(msg))
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
			project, _ := m.currentProject()
			m.screen = screenStatusInput
			m.statusInput = project.Status.Value
			m.statusCursor = len([]rune(m.statusInput))
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
	case value == "left":
		m.statusCursor = textMoveLeft(m.statusInput, m.statusCursor)
	case value == "right":
		m.statusCursor = textMoveRight(m.statusInput, m.statusCursor)
	case isMoveStartKey(value):
		m.statusCursor = textMoveStart(m.statusInput, m.statusCursor)
	case isMoveEndKey(value):
		m.statusCursor = textMoveEnd(m.statusInput, m.statusCursor)
	case isClearBeforeKey(value):
		m.statusInput, m.statusCursor = textClearBefore(m.statusInput, m.statusCursor)
	case isClearAfterKey(value):
		m.statusInput, m.statusCursor = textClearAfter(m.statusInput, m.statusCursor)
	case isDeletePreviousWordKey(value):
		m.statusInput, m.statusCursor = textDeletePreviousWord(m.statusInput, m.statusCursor)
	case isBackspaceKey(value):
		m.statusInput, m.statusCursor = textBackspace(m.statusInput, m.statusCursor)
	case isDeleteKey(value):
		m.statusInput, m.statusCursor = textDelete(m.statusInput, m.statusCursor)
	default:
		m.statusInput, m.statusCursor = textInsert(m.statusInput, m.statusCursor, inputText(msg))
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

func isTextInputKey(msg tea.KeyMsg) bool {
	value := msg.String()
	return value == "left" || value == "right" || isMoveStartKey(value) || isMoveEndKey(value) || isClearBeforeKey(value) || isClearAfterKey(value) || isDeletePreviousWordKey(value) || isBackspaceKey(value) || isDeleteKey(value) || msg.Type == tea.KeyRunes || msg.Type == tea.KeySpace
}

func isDetailScrollKey(value string) bool {
	switch value {
	case "pgup", "pgdown", "home", "end":
		return true
	default:
		return false
	}
}

func isExpandKey(value string) bool {
	return value == " " || value == "space"
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
		m.detailYOffset = 0
		return
	}
	before := m.selected
	m.selected += delta
	if m.selected < 0 {
		m.selected = 0
	}
	last := len(visible) - 1
	if m.selected > last {
		m.selected = last
	}
	if m.selected != before {
		m.detailYOffset = 0
	}
}

func (m *Model) clampSelection() {
	visible := m.visibleProjects()
	if len(visible) == 0 || m.selected < 0 {
		m.selected = 0
		m.detailYOffset = 0
		return
	}
	if m.selected >= len(visible) {
		m.selected = len(visible) - 1
		m.detailYOffset = 0
	}
}

func (m *Model) scrollDetail(value string) {
	_, maxOffset := m.currentScrollableDetail()
	if maxOffset <= 0 {
		m.detailYOffset = 0
		return
	}
	page := m.tableHeight() - 2
	if page < 1 {
		page = 1
	}
	switch value {
	case "pgup":
		m.detailYOffset -= page
	case "pgdown":
		m.detailYOffset += page
	case "home":
		m.detailYOffset = 0
	case "end":
		m.detailYOffset = maxOffset
	}
	if m.detailYOffset < 0 {
		m.detailYOffset = 0
	}
	if m.detailYOffset > maxOffset {
		m.detailYOffset = maxOffset
	}
}

func (m *Model) clampDetailOffset() {
	_, maxOffset := m.currentScrollableDetail()
	if maxOffset <= 0 || m.detailYOffset < 0 {
		m.detailYOffset = 0
		return
	}
	if m.detailYOffset > maxOffset {
		m.detailYOffset = maxOffset
	}
}

func (m *Model) scrollDetailModal(value string) {
	_, maxOffset := m.currentScrollableDetailModal()
	if maxOffset <= 0 {
		m.detailModalY = 0
		return
	}
	page := m.tableHeight() - 6
	if page < 1 {
		page = 1
	}
	switch value {
	case "pgup":
		m.detailModalY -= page
	case "pgdown":
		m.detailModalY += page
	case "home":
		m.detailModalY = 0
	case "end":
		m.detailModalY = maxOffset
	}
	if m.detailModalY < 0 {
		m.detailModalY = 0
	}
	if m.detailModalY > maxOffset {
		m.detailModalY = maxOffset
	}
}

func (m *Model) clampDetailModalOffset() {
	_, maxOffset := m.currentScrollableDetailModal()
	if maxOffset <= 0 || m.detailModalY < 0 {
		m.detailModalY = 0
		return
	}
	if m.detailModalY > maxOffset {
		m.detailModalY = maxOffset
	}
}

func (m Model) currentScrollableDetail() (string, int) {
	if !m.showInlineDetail() {
		return "", 0
	}
	_, detailWidth := splitPanelWidths(m.contentWidth(), 3)
	detail, ok := m.currentProject()
	if !ok {
		return "", 0
	}
	if commits, ok := m.recentByPath[detail.Path]; ok {
		detail.Activity.RecentCommits = commits
	}
	return scrollableDetailSummary(detail, detailWidth, m.tableHeight(), m.detailYOffset)
}

func (m Model) currentScrollableDetailModal() (string, int) {
	if m.screen != screenDetail {
		return "", 0
	}
	detail, ok := m.currentProject()
	if !ok {
		return "", 0
	}
	return detailModalViewWithScroll(detail, true, m.contentWidth(), m.tableHeight(), m.detailModalY, m.detailsExpanded)
}

func Run() error {
	return RunWithOptions(app.Options{})
}

func RunWithOptions(opts app.Options) error {
	setTerminalTitle(terminalTitleWriter, "ovw")
	program := tea.NewProgram(NewWithOptions(opts), tea.WithAltScreen())
	_, err := program.Run()
	return err
}

func setTerminalTitle(w io.Writer, title string) {
	if w == nil || title == "" {
		return
	}
	fmt.Fprintf(w, "\033]0;%s\007", title)
}

type overviewLoadedMsg struct {
	result       app.OverviewResult
	preservePath string
	message      string
}

type overviewLoadFailedMsg struct {
	err error
}

type onboardingLoadedMsg struct {
	candidates []string
}

type onboardingCreatedMsg struct {
	result app.OverviewResult
}

type onboardingFailedMsg struct {
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

type addProjectSavedMsg struct {
	message      string
	result       app.OverviewResult
	preservePath string
}

type addProjectFailedMsg struct {
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

type columnsSavedMsg struct {
	config config.Config
	err    error
}

func (m Model) loadOverview() tea.Cmd {
	return m.reloadOverview("", "")
}

func (m Model) startup() tea.Cmd {
	return func() tea.Msg {
		setup := m.setup
		if setup == nil {
			setup = app.CheckConfig
		}
		result, err := setup(m.request)
		if err != nil {
			return overviewLoadFailedMsg{err: err}
		}
		if !result.Exists {
			return onboardingLoadedMsg{candidates: result.Candidates}
		}
		loader := m.loader
		if loader == nil {
			loader = app.LoadOverview
		}
		overview, err := loader(m.request)
		if err != nil {
			return overviewLoadFailedMsg{err: err}
		}
		return overviewLoadedMsg{result: overview}
	}
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

func (m Model) createOnboardingConfigRoots(roots []string) tea.Cmd {
	return func() tea.Msg {
		if len(roots) == 0 {
			return onboardingFailedMsg{err: errEmptyProjectPath{}}
		}
		for _, root := range roots {
			if strings.TrimSpace(root) == "" {
				return onboardingFailedMsg{err: errEmptyProjectPath{}}
			}
		}
		creator := m.roots
		if creator == nil {
			creator = app.CreateConfigRoots
		}
		if _, _, err := creator(roots); err != nil {
			return onboardingFailedMsg{err: err}
		}
		loader := m.loader
		if loader == nil {
			loader = app.LoadOverview
		}
		result, err := loader(m.request)
		if err != nil {
			return overviewLoadFailedMsg{err: err}
		}
		return onboardingCreatedMsg{result: result}
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

func (m Model) togglePin() tea.Cmd {
	project, ok := m.currentProject()
	pinned := !project.Pinned
	return func() tea.Msg {
		if !ok {
			return metadataFailedMsg{err: errNoProjectSelected{}}
		}
		if _, err := m.updater(project.Path, app.MetadataUpdate{Pinned: &pinned}); err != nil {
			return metadataFailedMsg{err: err}
		}
		result, err := m.loader(m.request)
		if err != nil {
			return overviewLoadFailedMsg{err: err}
		}
		message := "Project pinned"
		if !pinned {
			message = "Project unpinned"
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

func (m Model) addProject() tea.Cmd {
	path := strings.TrimSpace(m.addInput)
	return func() tea.Msg {
		if path == "" {
			return addProjectFailedMsg{err: errEmptyProjectPath{}}
		}
		adder := m.adder
		if adder == nil {
			adder = app.AddProject
		}
		added, err := adder(path)
		if err != nil {
			return addProjectFailedMsg{err: err}
		}
		loader := m.loader
		if loader == nil {
			loader = app.LoadOverview
		}
		result, err := loader(m.request)
		if err != nil {
			return overviewLoadFailedMsg{err: err}
		}
		message := "Project added"
		if added.AlreadyTracked {
			message = "Project already tracked"
		}
		return addProjectSavedMsg{message: message, result: result, preservePath: added.Path}
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
	return runner(project.Path, project.Name, m.config.Shell)
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

type errEmptyProjectPath struct{}

func (errEmptyProjectPath) Error() string {
	return "enter a project path"
}

func renderShell(m Model) string {
	body := titleStyle.Render("ovw")
	switch {
	case m.loading && len(m.projects) == 0:
		body += "\n\n" + mutedStyle.Render("Loading projects...")
	case m.loadErr != nil:
		body += "\n\n" + errorStyle.Render("Failed to load projects: "+m.loadErr.Error())
	case m.screen == screenOnboarding:
		body = onboardingCheckedView(m.onboardOptions, m.onboardChecked, m.onboardSelected, m.onboardErr)
	case m.screen == screenOnboardingInput:
		body = onboardingInputView(m.onboardInput, m.onboardCursor, m.onboardErr)
	default:
		visible := m.visibleProjects()
		body = headerView(m)
		if m.message != "" {
			body += " " + mutedStyle.Render(m.message)
		}
		if len(visible) == 0 && m.search != "" {
			body += "\n\n" + mutedStyle.Render("No projects match search")
			body += "\n" + keyActionLine("esc", "clear search", 8)
		} else if len(visible) == 0 && m.pathScope() != "" {
			body += "\n\n" + mutedStyle.Render("No projects match path: "+m.pathScope())
		} else {
			content := m.tablePanel(visible)
			switch m.screen {
			case screenDetail:
				project, ok := m.currentProject()
				modal, maxOffset := detailModalViewWithScroll(project, ok, m.contentWidth(), m.tableHeight(), m.detailModalY, m.detailsExpanded)
				if m.detailModalY > maxOffset {
					m.detailModalY = maxOffset
					modal, _ = detailModalViewWithScroll(project, ok, m.contentWidth(), m.tableHeight(), m.detailModalY, m.detailsExpanded)
				}
				content = overlayModal(content, modal, m.contentWidth())
			case screenAdd:
				content = overlayModal(content, addProjectView(m.addInput, m.addCursor, m.addErr), m.contentWidth())
			case screenHelp:
				content = overlayModal(content, helpView(), m.contentWidth())
			case screenFilter:
				content = overlayModal(content, filterView(m.filterOptions(), m.filterSelected), m.contentWidth())
			case screenSort:
				content = overlayModal(content, sortView(sortOptions(), m.sortSelected, m.activeSortDir), m.contentWidth())
			case screenColumns:
				content = overlayModal(content, columnsView(m.columnOrder, m.columnChecked, m.columnSelected, m.columnErr), m.contentWidth())
			case screenNote:
				project, _ := m.currentProject()
				content = overlayModal(content, noteView(project.Name, m.noteInput, project.Note.Display, m.noteCursor), m.contentWidth())
			case screenStatus:
				project, _ := m.currentProject()
				content = overlayModal(content, statusView(project.Name, m.statusOptions(), m.statusSelected), m.contentWidth())
			case screenStatusInput:
				project, _ := m.currentProject()
				content = overlayModal(content, statusInputView(project.Name, m.statusInput, m.statusCursor), m.contentWidth())
			}
			body += "\n\n" + content
		}
	}
	if m.screen != screenOnboarding && m.screen != screenOnboardingInput {
		body = pinFooter(body, footerView(m.contentWidth()), m.height)
	}
	return body
}

func pinFooter(body, footer string, height int) string {
	if footer == "" {
		return body
	}
	if height <= 0 {
		return body + "\n\n" + footer
	}
	bodyLines := strings.Split(body, "\n")
	footerLines := strings.Split(footer, "\n")
	bodyLimit := height - len(footerLines)
	if bodyLimit <= 0 {
		return strings.Join(footerLines[len(footerLines)-height:], "\n")
	}
	if len(bodyLines) > bodyLimit {
		bodyLines = bodyLines[:bodyLimit]
	}
	for len(bodyLines) < bodyLimit {
		bodyLines = append(bodyLines, "")
	}
	return strings.Join(append(bodyLines, footerLines...), "\n")
}

func headerView(m Model) string {
	leftParts := []string{formatProjectCount(len(m.projects))}
	if scope := m.pathScope(); scope != "" {
		leftParts = append(leftParts, shortPath(scope))
	}
	if m.search != "" || m.searching {
		leftParts = append(leftParts, "search: "+m.searchDisplay())
	} else if elapsed := formatScanElapsed(m.scanElapsed); elapsed != "" {
		leftParts = append(leftParts, elapsed)
	}
	rightParts := []string{"filter: " + headerFilter(m.activeFilter)}
	if m.activeSort != "" && !tableColumnVisible(tableColumns(m.config), m.activeSort) {
		rightParts = append(rightParts, "sort: "+sortHeaderCompact(m.activeSort, m.activeSortDir))
	}
	rightParts = append(rightParts, selectedPosition(m.selected, len(m.visibleProjects())))

	fitHeaderParts(&leftParts, &rightParts, m.contentWidth())
	left := titleStyle.Render("ovw") + "  " + mutedStyle.Render(strings.Join(leftParts, "  "))
	right := mutedStyle.Render(strings.Join(rightParts, "  "))
	width := m.contentWidth()
	if width <= 0 || lipglossWidth(left)+lipglossWidth(right)+2 > width {
		return left + "  " + right
	}
	gap := width - lipglossWidth(left) - lipglossWidth(right)
	return left + strings.Repeat(" ", gap) + right
}

func (m Model) pathScope() string {
	return strings.TrimSpace(m.request.Path)
}

func tableColumnVisible(columns []string, column string) bool {
	for _, candidate := range columns {
		if candidate == column {
			return true
		}
	}
	return false
}

func fitHeaderParts(leftParts, rightParts *[]string, width int) {
	if width <= 0 {
		return
	}
	fits := func() bool {
		left := titleStyle.Render("ovw") + "  " + mutedStyle.Render(strings.Join(*leftParts, "  "))
		right := strings.Join(*rightParts, "  ")
		return lipglossWidth(left)+lipglossWidth(right)+2 <= width
	}
	for _, step := range []func(){
		func() {
			*leftParts = removeFirstMatching(*leftParts, func(part string) bool { return strings.HasPrefix(part, "scanned in ") })
		},
		func() {
			*rightParts = removeFirstMatching(*rightParts, func(part string) bool { return strings.HasPrefix(part, "sort: ") })
		},
		func() {
			*rightParts = removeFirstMatching(*rightParts, func(part string) bool { return strings.HasPrefix(part, "filter: ") })
		},
		func() {
			*leftParts = removeFirstMatching(*leftParts, func(part string) bool { return strings.HasPrefix(part, "search: ") })
		},
	} {
		if fits() {
			return
		}
		step()
	}
}

func removeFirstMatching(parts []string, match func(string) bool) []string {
	for index, part := range parts {
		if match(part) {
			return append(parts[:index], parts[index+1:]...)
		}
	}
	return parts
}

func formatScanElapsed(elapsed time.Duration) string {
	if elapsed <= 0 {
		return ""
	}
	return fmt.Sprintf("scanned in %.1fs", elapsed.Seconds())
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
			detailText, maxOffset := scrollableDetailSummary(detail, detailWidth, tableHeight, m.detailYOffset)
			if m.detailYOffset > maxOffset {
				m.detailYOffset = maxOffset
				detailText, _ = scrollableDetailSummary(detail, detailWidth, tableHeight, m.detailYOffset)
			}
			return joinColumns(
				tableView(visible, m.selected, tableWidth, tableHeight, m.tableXOffset, m.config, m.activeSort, m.activeSortDir),
				detailText,
				gap,
				tableHeight,
			)
		}
	}
	return tableView(visible, m.selected, contentWidth, tableHeight, m.tableXOffset, m.config, m.activeSort, m.activeSortDir)
}

func (m Model) showInlineDetail() bool {
	return m.isTableLayoutScreen() && m.contentWidth() >= 110 && len(m.visibleProjects()) > 0
}

func (m Model) isTableLayoutScreen() bool {
	switch m.screen {
	case screenTable, screenDetail, screenAdd, screenHelp, screenFilter, screenSort, screenColumns, screenNote, screenStatus, screenStatusInput:
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

func joinColumns(left, right string, gap int, height ...int) string {
	leftLines := strings.Split(left, "\n")
	rightLines := strings.Split(right, "\n")
	leftWidth := maxLineWidth(leftLines)
	lineCount := len(leftLines)
	if len(rightLines) > lineCount {
		lineCount = len(rightLines)
	}
	if len(height) > 0 && height[0] > lineCount {
		lineCount = height[0]
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

func scrollableDetailSummary(detail project.Project, width int, height int, offset int) (string, int) {
	lines := strings.Split(detailSummaryView(detail, width), "\n")
	if height <= 0 || len(lines) <= height {
		return strings.Join(lines, "\n"), 0
	}
	visibleHeight := height - 1
	if visibleHeight < 1 {
		visibleHeight = 1
	}
	maxOffset := len(lines) - visibleHeight
	if maxOffset < 0 {
		maxOffset = 0
	}
	if offset < 0 {
		offset = 0
	}
	if offset > maxOffset {
		offset = maxOffset
	}
	end := offset + visibleHeight
	if end > len(lines) {
		end = len(lines)
	}
	visible := append([]string{}, lines[offset:end]...)
	visible = append(visible, detailScrollHint(offset, maxOffset, width))
	return strings.Join(visible, "\n"), maxOffset
}

func detailScrollHint(offset, maxOffset, width int) string {
	return scrollHint(offset, maxOffset, width, func(value string) string {
		return inlineHintKey(value)
	}, func(value string) string {
		return mutedStyle.Render(value)
	})
}

func inlineHintKey(value string) string {
	return "\x1b[38;5;252m" + value + ansiReset
}

func scrollHint(offset, maxOffset, width int, markerStyle func(string) string, textStyle func(string) string) string {
	marker := "↓"
	switch {
	case offset > 0 && offset < maxOffset:
		marker = "↑↓"
	case offset >= maxOffset:
		marker = "↑"
	}
	text := " pgup/pgdn"
	if width > 0 {
		markerWidth := lipgloss.Width(marker)
		if width <= markerWidth {
			return markerStyle(truncateText(marker, width))
		}
		text = truncateText(text, width-markerWidth)
	}
	return markerStyle(marker) + textStyle(text)
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
		return searchInputLine(m.search, "", m.searchCursor)
	}
	return m.search
}

func (m *Model) selectProjectPath(path string) {
	visible := m.visibleProjects()
	for index, project := range visible {
		if project.Path == path {
			m.selected = index
			m.detailYOffset = 0
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
