package tui

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"ovw/internal/app"
	"ovw/internal/config"
	"ovw/internal/filter"
	ovwformat "ovw/internal/format"
	"ovw/internal/gitactivity"
	"ovw/internal/ports"
	"ovw/internal/project"
	"ovw/internal/projectfiles"
	"ovw/internal/scanner"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const inputCursorBlinkSpeed = 750 * time.Millisecond

type overviewLoader func(app.Options) (app.OverviewResult, error)
type configSetupLoader func(app.Options) (app.ConfigSetup, error)
type configRootsCreator func([]string) (config.FilePaths, config.Config, error)
type metadataUpdater func(string, app.MetadataUpdate) (app.MetadataUpdateResult, error)
type visibilityUpdater func(string, bool) (app.MetadataUpdateResult, error)
type projectAdder func(string) (app.AddProjectResult, error)
type editorRunner func(string, string) error
type terminalRunner func(string, string, string) tea.Cmd
type scriptRunner func(string, string, string, string) tea.Cmd
type recentLoader func(string, time.Time) ([]ovwformat.RecentCommit, error)
type recentFilesLoader func(string, []string, time.Time) ([]ovwformat.RecentFile, error)
type configWriter func(string, config.Config) error
type portDetector func([]string) map[string][]int

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
	screenCommand
	screenRunner
	screenOnboarding
	screenOnboardingInput
	screenColumns
	screenConfig
	screenConfigList
	screenConfigInput
	screenConfigRoots
	screenConfigRootsInput
	screenConfigNote
	screenConfigKeys
)

type Model struct {
	request      app.Options
	loader       overviewLoader
	discover     overviewLoader
	setup        configSetupLoader
	roots        configRootsCreator
	updater      metadataUpdater
	visible      visibilityUpdater
	adder        projectAdder
	editor       editorRunner
	terminal     terminalRunner
	runner       scriptRunner
	recent       recentLoader
	recentFiles  recentFilesLoader
	configWriter configWriter
	portDetector portDetector

	width             int
	height            int
	selected          int
	tableXOffset      int
	detailYOffset     int
	detailModalY      int
	detailsExpanded   bool
	screen            screenMode
	search            string
	searchCursor      int
	searching         bool
	filterSelected    int
	activeFilter      string
	sortSelected      int
	activeSort        string
	activeSortDir     string
	noteInput         string
	noteCursor        int
	addInput          string
	addCursor         int
	addErr            string
	statusSelected    int
	statusInput       string
	statusCursor      int
	runnerSelected    int
	runnerInput       string
	runnerCursor      int
	runnerYOffset     int
	runnerAddName     string
	runnerAdding      bool
	runnerShowInfo    bool
	commandInput      string
	commandCursor     int
	commandSelected   int
	cursorHidden      bool
	cursorBlinkID     int
	onboardOptions    []string
	onboardChecked    map[string]bool
	onboardSelected   int
	onboardInput      string
	onboardCursor     int
	onboardErr        string
	columnSelected    int
	columnOrder       []string
	columnChecked     map[string]bool
	columnErr         string
	configSelected    int
	configDraft       config.Config
	configErr         string
	configListField   configListField
	configListSel     int
	configListInput   string
	configListCursor  int
	configListEditing bool
	configInputKind   configInputKind
	configInput       string
	configCursor      int
	configRootPicker  rootPicker
	configRootInput   string
	configRootCursor  int
	configNoteSel     int
	configKeySel      int
	message           string
	loading           bool
	loadErr           error
	enriching         bool
	enrichedCount     int
	enrichTotal       int
	config            config.Config
	configPaths       config.FilePaths
	projects          []project.Project
	scanElapsed       time.Duration
	showScanElapsed   bool
	recentByPath      map[string][]ovwformat.RecentCommit
	filesByPath       map[string][]ovwformat.RecentFile
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
		discover:      app.DiscoverOverview,
		setup:         app.CheckConfig,
		roots:         app.CreateConfigRoots,
		updater:       app.UpdateProjectMetadata,
		visible:       app.SetProjectHidden,
		adder:         app.AddProject,
		editor:        OpenEditor,
		terminal:      runTerminal,
		runner:        runScript,
		recent:        loadRecentCommits,
		recentFiles:   loadRecentFiles,
		configWriter:  config.Write,
		portDetector:  ports.Detect,
		activeFilter:  optionsFromRequest(opts),
		activeSort:    sortFromRequest(opts),
		activeSortDir: sortDirFromRequest(opts),
		loading:       true,
		recentByPath:  map[string][]ovwformat.RecentCommit{},
		filesByPath:   map[string][]ovwformat.RecentFile{},
	}
}

func NewWithLoader(loader overviewLoader) Model {
	model := NewWithOptions(app.Options{})
	model.loader = loader
	model.discover = nil
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
	case inputCursorBlinkMsg:
		if msg.id != m.cursorBlinkID || !m.cursorBlinkActive() {
			return m, nil
		}
		m.cursorHidden = !m.cursorHidden
		return m, inputCursorBlink(m.cursorBlinkID)
	case tea.MouseMsg:
		return m.updateMouse(msg)
	case tea.KeyMsg:
		m.cursorHidden = false
		m.showScanElapsed = false
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
		if m.screen == screenConfig {
			return m.updateConfig(msg)
		}
		if m.screen == screenConfigList {
			return m.updateConfigList(msg)
		}
		if m.screen == screenConfigInput {
			return m.updateConfigInput(msg)
		}
		if m.screen == screenConfigRoots {
			return m.updateConfigRoots(msg)
		}
		if m.screen == screenConfigRootsInput {
			return m.updateConfigRootsInput(msg)
		}
		if m.screen == screenConfigNote {
			return m.updateConfigNote(msg)
		}
		if m.screen == screenConfigKeys {
			return m.updateConfigKeys(msg)
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
		if m.screen == screenRunner {
			return m.updateRunner(msg)
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
		if m.screen == screenCommand {
			return m.updateCommand(msg)
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
		if isCommandKey(msg.String()) {
			m.openCommandPalette()
			return m, m.startInputCursorBlink()
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
			return m, m.startInputCursorBlink()
		}
		if isAddKey(msg.String()) && !m.loading {
			m.screen = screenAdd
			m.addInput = ""
			m.addCursor = 0
			m.addErr = ""
			return m, m.startInputCursorBlink()
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
		if m.isNoteKey(msg.String()) && m.canOpenDetail() {
			project, _ := m.currentProject()
			m.screen = screenNote
			m.noteInput = project.Note.Value
			m.noteCursor = len([]rune(m.noteInput))
			return m, m.startInputCursorBlink()
		}
		if m.isStatusKey(msg.String()) && m.canOpenDetail() {
			project, _ := m.currentProject()
			m.screen = screenStatus
			m.statusSelected = m.currentStatusIndex(project.Status.Value)
			return m, nil
		}
		if m.isPinKey(msg.String()) && m.canOpenDetail() {
			m.loading = true
			return m, m.togglePin()
		}
		if m.isRunnerKey(msg.String()) && m.canOpenDetail() {
			return m.openRunner()
		}
		if isReloadKey(msg.String()) {
			m.screen = screenTable
			m.loading = true
			return m, m.reloadOverview("", "Reloaded")
		}
		if m.isOpenKey(msg.String()) && m.canOpenDetail() {
			return m, m.openSelectedProject()
		}
		if m.isTerminalKey(msg.String()) && m.canOpenDetail() {
			return m, m.openSelectedTerminal()
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.clampDetailOffset()
		m.clampDetailModalOffset()
		m.clampRunnerOffset()
		return m, m.loadSelectedRecent()
	case overviewLoadedMsg:
		m.loading = false
		m.enriching = false
		m.enrichedCount = 0
		m.enrichTotal = 0
		m.loadErr = nil
		m.screen = screenTable
		m.config = msg.result.Config
		m.configPaths = msg.result.Paths
		m.projects = msg.result.Projects
		m.scanElapsed = msg.result.Elapsed
		m.showScanElapsed = true
		m.syncActiveSort()
		m.recentByPath = map[string][]ovwformat.RecentCommit{}
		m.filesByPath = map[string][]ovwformat.RecentFile{}
		if msg.message != "" {
			m.message = msg.message
		}
		if msg.preservePath != "" {
			m.selectProjectPath(msg.preservePath)
		} else if msg.selected >= 0 {
			m.selected = msg.selected
			m.clampSelection()
		} else if m.selected >= len(m.projects) {
			m.selected = 0
			m.detailYOffset = 0
		}
		return m, m.loadSelectedRecent()
	case overviewDiscoveredMsg:
		m.loading = false
		m.enriching = len(msg.result.Scanned) > 0
		m.enrichedCount = 0
		m.enrichTotal = len(msg.result.Scanned)
		m.loadErr = nil
		m.screen = screenTable
		m.config = msg.result.Config
		m.configPaths = msg.result.Paths
		m.projects = msg.result.Projects
		m.scanElapsed = msg.result.Elapsed
		m.showScanElapsed = true
		m.syncActiveSort()
		m.recentByPath = map[string][]ovwformat.RecentCommit{}
		m.filesByPath = map[string][]ovwformat.RecentFile{}
		if msg.message != "" {
			m.message = msg.message
		}
		if msg.preservePath != "" {
			m.selectProjectPath(msg.preservePath)
		} else if msg.selected >= 0 {
			m.selected = msg.selected
			m.clampSelection()
		} else if m.selected >= len(m.projects) {
			m.selected = 0
			m.detailYOffset = 0
		}
		if msg.updates == nil || !m.enriching {
			m.enriching = false
			return m, m.loadSelectedRecent()
		}
		return m, waitForEnrichment(msg.updates)
	case overviewLoadFailedMsg:
		m.loading = false
		m.enriching = false
		m.loadErr = msg.err
	case projectEnrichedMsg:
		selectedIndex := m.selected
		m.enrichedCount = msg.update.done
		m.enrichTotal = msg.update.total
		m.scanElapsed = msg.update.elapsed
		m.replaceProject(msg.update.project)
		m.projects = filter.Sort(m.projects, filter.FormatSort(m.activeSort, m.activeSortDir), m.config)
		m.selected = selectedIndex
		m.clampSelection()
		return m, waitForEnrichment(msg.updates)
	case enrichmentDoneMsg:
		m.loading = false
		m.enriching = false
		m.enrichedCount = m.enrichTotal
		return m, m.loadSelectedRecent()
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
		m.showScanElapsed = true
		m.syncActiveSort()
		m.recentByPath = map[string][]ovwformat.RecentCommit{}
		m.filesByPath = map[string][]ovwformat.RecentFile{}
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
		m.showScanElapsed = false
		m.syncActiveSort()
		m.recentByPath = map[string][]ovwformat.RecentCommit{}
		m.filesByPath = map[string][]ovwformat.RecentFile{}
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
		m.showScanElapsed = false
		m.recentByPath = map[string][]ovwformat.RecentCommit{}
		m.filesByPath = map[string][]ovwformat.RecentFile{}
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
		return m, m.startInputCursorBlink()
	case editorOpenedMsg:
		m.message = msg.message
	case editorFailedMsg:
		m.message = "Editor failed: " + msg.err.Error()
	case terminalOpenedMsg:
		m.message = msg.message
	case terminalFailedMsg:
		m.message = "Terminal failed: " + msg.err.Error()
	case runnerFinishedMsg:
		m.message = msg.message
	case runnerFailedMsg:
		m.message = "Script failed: " + msg.err.Error()
	case recentLoadedMsg:
		if m.recentByPath == nil {
			m.recentByPath = map[string][]ovwformat.RecentCommit{}
		}
		if m.filesByPath == nil {
			m.filesByPath = map[string][]ovwformat.RecentFile{}
		}
		if msg.commitErr == nil {
			m.recentByPath[msg.path] = msg.commits
		}
		if msg.fileErr == nil {
			m.filesByPath[msg.path] = msg.files
		}
	case columnsSavedMsg:
		m.loading = false
		if msg.err != nil {
			m.message = "Failed to write columns: " + msg.err.Error()
			m.columnErr = msg.err.Error()
			return m, nil
		}
		previousConfig := m.config
		preservePath := m.selectedProjectPath()
		m.screen = screenTable
		m.config = msg.config
		m.tableXOffset = 0
		m.message = "Columns saved"
		if !app.NeedsUpdated(previousConfig, "") && app.NeedsUpdated(m.config, "") {
			m.loading = true
			return m, m.reloadOverview(preservePath, "Columns saved")
		}
		if shouldDetectPortsForColumns(m.config.Columns) {
			return m, m.detectProjectPorts()
		}
	case portsDetectedMsg:
		m.attachPorts(msg.portsByPath)
	case configRootCandidatesMsg:
		m.applyConfigRootCandidates(msg.candidates)
		return m, m.countConfigRootPaths(m.configRootPicker.visiblePaths())
	case configRootCountsMsg:
		if m.configRootPicker.counts == nil {
			m.configRootPicker.counts = map[string]int{}
		}
		for path, count := range msg.counts {
			m.configRootPicker.counts[path] = count
		}
	case configSavedMsg:
		m.loading = false
		if msg.err != nil {
			m.screen = screenConfig
			m.message = "Failed to write config: " + msg.err.Error()
			m.configErr = msg.err.Error()
			return m, nil
		}
		m.screen = screenTable
		m.config = msg.result.Config
		m.configPaths = msg.result.Paths
		m.projects = msg.result.Projects
		m.scanElapsed = msg.result.Elapsed
		m.showScanElapsed = false
		m.syncActiveSort()
		m.recentByPath = map[string][]ovwformat.RecentCommit{}
		m.filesByPath = map[string][]ovwformat.RecentFile{}
		m.message = "Config saved"
		if msg.preservePath != "" {
			m.selectProjectPath(msg.preservePath)
		} else {
			m.clampSelection()
		}
		return m, m.loadSelectedRecent()
	case configOpenedMsg:
		m.loading = false
		if msg.err != nil {
			m.screen = screenConfig
			m.message = "Failed to open config: " + msg.err.Error()
			m.configErr = msg.err.Error()
			return m, nil
		}
		m.screen = screenTable
		m.config = msg.result.Config
		m.configPaths = msg.result.Paths
		m.projects = msg.result.Projects
		m.scanElapsed = msg.result.Elapsed
		m.showScanElapsed = false
		m.syncActiveSort()
		m.recentByPath = map[string][]ovwformat.RecentCommit{}
		m.filesByPath = map[string][]ovwformat.RecentFile{}
		m.message = "Config opened"
		return m, m.loadSelectedRecent()
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
	case m.isNoteKey(value):
		project, ok := m.currentProject()
		if ok {
			m.screen = screenNote
			m.noteInput = project.Note.Value
			m.noteCursor = len([]rune(m.noteInput))
			return m, m.startInputCursorBlink()
		}
	case m.isStatusKey(value):
		project, ok := m.currentProject()
		if ok {
			m.screen = screenStatus
			m.statusSelected = m.currentStatusIndex(project.Status.Value)
		}
	case m.isOpenKey(value):
		return m, m.openSelectedProject()
	case m.isTerminalKey(value):
		return m, m.openSelectedTerminal()
	case m.isVisibilityKey(value):
		m.screen = screenTable
		m.detailModalY = 0
		m.detailsExpanded = false
		m.loading = true
		return m, m.toggleVisibility()
	case m.isPinKey(value):
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
		m.filterSelected = wrapPickerSelection(m.filterSelected, len(options), 1)
	case isUpKey(value):
		m.filterSelected = wrapPickerSelection(m.filterSelected, len(options), -1)
	case isEnterKey(value):
		if len(options) == 0 {
			m.screen = screenTable
			return m, nil
		}
		m.applyFilter(options[m.filterSelected])
		m.screen = screenTable
		m.selected = 0
		m.clampSelection()
		return m, m.loadSelectedRecent()
	}
	return m, nil
}

func (m Model) updateSort(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	options := sortOptions(m.config)
	switch value := msg.String(); {
	case isEscapeKey(value):
		m.screen = screenTable
	case isDownKey(value):
		m.sortSelected = wrapPickerSelection(m.sortSelected, len(options), 1)
	case isUpKey(value):
		m.sortSelected = wrapPickerSelection(m.sortSelected, len(options), -1)
	case isLeftKey(value), isRightKey(value):
		m.toggleSortDir()
	case isEnterKey(value):
		if len(options) == 0 {
			m.screen = screenTable
			return m, nil
		}
		m.applySort(options[m.sortSelected])
		m.screen = screenTable
		m.projects = filter.Sort(m.projects, filter.FormatSort(m.activeSort, m.activeSortDir), m.config)
		m.selected = 0
		m.clampSelection()
		return m, m.loadSelectedRecent()
	}
	return m, nil
}

func (m Model) updateRunner(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	project, ok := m.currentProject()
	if !ok {
		m.screen = screenTable
		return m, nil
	}
	if m.runnerAdding {
		return m.updateRunnerCommand(msg)
	}
	scripts := runnerScriptOptions(project, m.runnerInput)
	switch value := msg.String(); {
	case isEscapeKey(value):
		m.screen = screenTable
		m.runnerInput = ""
		m.runnerCursor = 0
		m.runnerYOffset = 0
		m.runnerAddName = ""
		m.runnerAdding = false
		m.runnerShowInfo = false
	case isDownKey(value):
		m.runnerSelected = wrapPickerSelection(m.runnerSelected, len(scripts), 1)
		m.keepRunnerSelectionVisible()
	case isUpKey(value):
		m.runnerSelected = wrapPickerSelection(m.runnerSelected, len(scripts), -1)
		m.keepRunnerSelectionVisible()
	case isDetailScrollKey(value):
		m.scrollRunner(value)
	case isExpandKey(value):
		m.runnerShowInfo = !m.runnerShowInfo
	case isEnterKey(value):
		if len(scripts) == 0 {
			name := strings.TrimSpace(m.runnerInput)
			if name != "" {
				m.runnerAddName = name
				m.runnerInput = ""
				m.runnerCursor = 0
				m.runnerAdding = true
				return m, m.startInputCursorBlink()
			}
			return m, nil
		}
		return m.runRunnerScript(project, scripts)
	case value == "left":
		m.runnerCursor = textMoveLeft(m.runnerInput, m.runnerCursor)
	case value == "right":
		m.runnerCursor = textMoveRight(m.runnerInput, m.runnerCursor)
	case isMoveStartKey(value):
		m.runnerCursor = textMoveStart(m.runnerInput, m.runnerCursor)
	case isMoveEndKey(value):
		m.runnerCursor = textMoveEnd(m.runnerInput, m.runnerCursor)
	case isClearBeforeKey(value):
		m.runnerInput, m.runnerCursor = textClearBefore(m.runnerInput, m.runnerCursor)
		m.runnerSelected = 0
		m.runnerYOffset = 0
		m.runnerShowInfo = false
	case isClearAfterKey(value):
		m.runnerInput, m.runnerCursor = textClearAfter(m.runnerInput, m.runnerCursor)
		m.runnerSelected = 0
		m.runnerYOffset = 0
		m.runnerShowInfo = false
	case isDeletePreviousWordKey(value):
		m.runnerInput, m.runnerCursor = textDeletePreviousWord(m.runnerInput, m.runnerCursor)
		m.runnerSelected = 0
		m.runnerYOffset = 0
		m.runnerShowInfo = false
	case isBackspaceKey(value):
		m.runnerInput, m.runnerCursor = textBackspace(m.runnerInput, m.runnerCursor)
		m.runnerSelected = 0
		m.runnerYOffset = 0
		m.runnerShowInfo = false
	case isDeleteKey(value):
		m.runnerInput, m.runnerCursor = textDelete(m.runnerInput, m.runnerCursor)
		m.runnerSelected = 0
		m.runnerYOffset = 0
		m.runnerShowInfo = false
	default:
		m.runnerInput, m.runnerCursor = textInsert(m.runnerInput, m.runnerCursor, inputText(msg))
		m.runnerSelected = 0
		m.runnerYOffset = 0
		m.runnerShowInfo = false
	}
	return m, nil
}

func (m Model) updateRunnerCommand(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch value := msg.String(); {
	case isEscapeKey(value):
		m.runnerAdding = false
		m.runnerInput = m.runnerAddName
		m.runnerCursor = len([]rune(m.runnerInput))
		m.runnerAddName = ""
	case isEnterKey(value):
		command := strings.TrimSpace(m.runnerInput)
		if command == "" {
			return m, nil
		}
		m.screen = screenTable
		m.runnerAdding = false
		return m, m.saveRunnerScript(m.runnerAddName, command)
	case value == "left":
		m.runnerCursor = textMoveLeft(m.runnerInput, m.runnerCursor)
	case value == "right":
		m.runnerCursor = textMoveRight(m.runnerInput, m.runnerCursor)
	case isMoveStartKey(value):
		m.runnerCursor = textMoveStart(m.runnerInput, m.runnerCursor)
	case isMoveEndKey(value):
		m.runnerCursor = textMoveEnd(m.runnerInput, m.runnerCursor)
	case isClearBeforeKey(value):
		m.runnerInput, m.runnerCursor = textClearBefore(m.runnerInput, m.runnerCursor)
	case isClearAfterKey(value):
		m.runnerInput, m.runnerCursor = textClearAfter(m.runnerInput, m.runnerCursor)
	case isDeletePreviousWordKey(value):
		m.runnerInput, m.runnerCursor = textDeletePreviousWord(m.runnerInput, m.runnerCursor)
	case isBackspaceKey(value):
		m.runnerInput, m.runnerCursor = textBackspace(m.runnerInput, m.runnerCursor)
	case isDeleteKey(value):
		m.runnerInput, m.runnerCursor = textDelete(m.runnerInput, m.runnerCursor)
	default:
		m.runnerInput, m.runnerCursor = textInsert(m.runnerInput, m.runnerCursor, inputText(msg))
	}
	return m, nil
}

func (m Model) runRunnerScript(project project.Project, scripts []runnerScript) (Model, tea.Cmd) {
	if m.runnerSelected >= len(scripts) {
		m.runnerSelected = len(scripts) - 1
	}
	if m.runnerSelected < 0 {
		m.runnerSelected = 0
	}
	script := scripts[m.runnerSelected]
	m.screen = screenTable
	return m, m.runSelectedScript(project, script)
}

func (m Model) updateColumns(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.loading {
		return m, nil
	}
	switch value := msg.String(); {
	case isEscapeKey(value):
		m.screen = screenTable
		m.columnErr = ""
	case isEnterKey(value):
		m.loading = true
		m.columnErr = ""
		return m, m.saveColumns()
	case isDownKey(value):
		m.columnSelected = wrapPickerSelection(m.columnSelected, len(m.columnOrder), 1)
	case isUpKey(value):
		m.columnSelected = wrapPickerSelection(m.columnSelected, len(m.columnOrder), -1)
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
		m.onboardSelected = wrapPickerSelection(m.onboardSelected, len(m.onboardOptions)+1, 1)
	case isUpKey(value):
		m.onboardSelected = wrapPickerSelection(m.onboardSelected, len(m.onboardOptions)+1, -1)
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
			return m, m.startInputCursorBlink()
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
		m.statusSelected = wrapPickerSelection(m.statusSelected, len(options), 1)
	case isUpKey(value):
		m.statusSelected = wrapPickerSelection(m.statusSelected, len(options), -1)
	case isEnterKey(value):
		option := options[m.statusSelected]
		switch option.Kind {
		case statusOptionCustom:
			project, _ := m.currentProject()
			m.screen = screenStatusInput
			m.statusInput = project.Status.Value
			m.statusCursor = len([]rune(m.statusInput))
			return m, m.startInputCursorBlink()
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

func wrapPickerSelection(selected, total, delta int) int {
	if total <= 0 {
		return 0
	}
	next := (selected + delta) % total
	if next < 0 {
		next += total
	}
	return next
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

func (m Model) updateMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	if m.loading || m.screen != screenTable {
		return m, nil
	}
	before := m.selected
	switch msg.Button {
	case tea.MouseButtonWheelDown:
		m.moveSelection(1)
	case tea.MouseButtonWheelUp:
		m.moveSelection(-1)
	default:
		return m, nil
	}
	if m.selected == before {
		return m, nil
	}
	return m, m.loadSelectedRecent()
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

func (m *Model) scrollRunner(value string) {
	_, maxOffset := m.currentScrollableRunner()
	if maxOffset <= 0 {
		m.runnerYOffset = 0
		return
	}
	page := m.tableHeight() - 6
	if page < 1 {
		page = 1
	}
	switch value {
	case "pgup":
		m.runnerYOffset -= page
	case "pgdown":
		m.runnerYOffset += page
	case "home":
		m.runnerYOffset = 0
	case "end":
		m.runnerYOffset = maxOffset
	}
	m.clampRunnerOffset()
}

func (m *Model) clampRunnerOffset() {
	_, maxOffset := m.currentScrollableRunner()
	if maxOffset <= 0 || m.runnerYOffset < 0 {
		m.runnerYOffset = 0
		return
	}
	if m.runnerYOffset > maxOffset {
		m.runnerYOffset = maxOffset
	}
}

func (m *Model) keepRunnerSelectionVisible() {
	_, maxOffset := m.currentScrollableRunner()
	if maxOffset <= 0 {
		m.runnerYOffset = 0
		return
	}
	visibleHeight := runnerVisibleOptionHeight(m.tableHeight())
	selectedStart, selectedEnd := runnerSelectedLineRange(m.runnerSelected, m.runnerShowInfo)
	if selectedEnd-selectedStart+1 > visibleHeight {
		m.runnerYOffset = selectedStart
	} else if selectedStart < m.runnerYOffset {
		m.runnerYOffset = selectedStart
	} else if selectedEnd >= m.runnerYOffset+visibleHeight {
		m.runnerYOffset = selectedEnd - visibleHeight + 1
	}
	m.clampRunnerOffset()
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
	if files, ok := m.filesByPath[detail.Path]; ok {
		detail.RecentFiles = files
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

func (m Model) currentScrollableRunner() (string, int) {
	if m.screen != screenRunner {
		return "", 0
	}
	project, ok := m.currentProject()
	if !ok {
		return "", 0
	}
	return runnerViewWithScroll(project, m.runnerSelected, m.runnerInput, m.runnerCursor, m.tableHeight(), m.runnerYOffset, m.runnerAddName, m.runnerAdding, m.runnerShowInfo, m.inputCursorState())
}

func Run() error {
	return RunWithOptions(app.Options{})
}

func RunWithOptions(opts app.Options) error {
	setTerminalTitle(terminalTitleWriter, "ovw")
	program := tea.NewProgram(NewWithOptions(opts), tea.WithAltScreen(), tea.WithMouseCellMotion())
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
	selected     int
	message      string
}

type overviewDiscoveredMsg struct {
	result       app.OverviewResult
	updates      <-chan enrichmentUpdate
	preservePath string
	selected     int
	message      string
}

type enrichmentUpdate struct {
	project project.Project
	done    int
	total   int
	elapsed time.Duration
}

type projectEnrichedMsg struct {
	update  enrichmentUpdate
	updates <-chan enrichmentUpdate
}

type enrichmentDoneMsg struct{}

type portsDetectedMsg struct {
	portsByPath map[string][]int
}

type inputCursorBlinkMsg struct {
	id int
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

type runnerFinishedMsg struct {
	message string
}

type runnerFailedMsg struct {
	err error
}

type recentLoadedMsg struct {
	path      string
	commits   []ovwformat.RecentCommit
	files     []ovwformat.RecentFile
	commitErr error
	fileErr   error
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
		return m.loadOverviewMessage("", "")
	}
}

func (m Model) reloadOverview(preservePath, message string) tea.Cmd {
	selected := m.selected
	return func() tea.Msg {
		msg := m.loadOverviewMessage(preservePath, message)
		if preservePath != "" {
			return msg
		}
		switch loaded := msg.(type) {
		case overviewLoadedMsg:
			loaded.selected = selected
			return loaded
		case overviewDiscoveredMsg:
			loaded.selected = selected
			return loaded
		default:
			return msg
		}
	}
}

func (m Model) loadOverviewMessage(preservePath, message string) tea.Msg {
	discover := m.discover
	if discover == nil {
		loader := m.loader
		if loader == nil {
			loader = app.LoadOverview
		}
		result, err := loader(m.request)
		if err != nil {
			return overviewLoadFailedMsg{err: err}
		}
		return overviewLoadedMsg{result: result, preservePath: preservePath, message: message}
	}
	result, err := discover(m.request)
	if err != nil {
		return overviewLoadFailedMsg{err: err}
	}
	updates := startEnrichment(result.Scanned, result.Config, m.portDetector)
	return overviewDiscoveredMsg{result: result, updates: updates, preservePath: preservePath, message: message}
}

func startEnrichment(scanned []scanner.Project, cfg config.Config, detect portDetector) <-chan enrichmentUpdate {
	out := make(chan enrichmentUpdate, len(scanned))
	go func() {
		defer close(out)
		if len(scanned) == 0 {
			return
		}
		start := time.Now()
		now := time.Now()
		detector := gitactivity.NewDetector()
		portsByPath := map[string][]int{}
		if shouldDetectPortsForColumns(cfg.Columns) {
			if detect == nil {
				detect = ports.Detect
			}
			portsByPath = detect(scannedProjectPaths(scanned))
		}
		jobs := make(chan scanner.Project)
		results := make(chan project.Project)
		workers := app.DefaultEnrichmentWorkers
		if len(scanned) < workers {
			workers = len(scanned)
		}
		enrichOpts := app.EnrichOptions{DetectUpdated: app.NeedsUpdated(cfg, "")}
		for i := 0; i < workers; i++ {
			go func() {
				for scannedProject := range jobs {
					enriched := app.EnrichWithGitOptions(scannedProject, cfg, now, detector.Detect(scannedProject.Path), enrichOpts)
					enriched.Ports = portsByPath[scannedProject.Path]
					results <- enriched
				}
			}()
		}
		go func() {
			for _, scannedProject := range scanned {
				jobs <- scannedProject
			}
			close(jobs)
		}()
		for done := 1; done <= len(scanned); done++ {
			out <- enrichmentUpdate{
				project: <-results,
				done:    done,
				total:   len(scanned),
				elapsed: time.Since(start),
			}
		}
	}()
	return out
}

func waitForEnrichment(updates <-chan enrichmentUpdate) tea.Cmd {
	return func() tea.Msg {
		update, ok := <-updates
		if !ok {
			return enrichmentDoneMsg{}
		}
		return projectEnrichedMsg{update: update, updates: updates}
	}
}

func (m Model) detectProjectPorts() tea.Cmd {
	paths := projectPathsForPorts(m.projects)
	detect := m.portDetector
	if detect == nil {
		detect = ports.Detect
	}
	return func() tea.Msg {
		return portsDetectedMsg{portsByPath: detect(paths)}
	}
}

func (m *Model) attachPorts(portsByPath map[string][]int) {
	for index := range m.projects {
		m.projects[index].Ports = portsByPath[m.projects[index].Path]
	}
}

func shouldDetectPortsForColumns(columns []string) bool {
	for _, column := range columns {
		if column == "ports" {
			return true
		}
	}
	return false
}

func scannedProjectPaths(scanned []scanner.Project) []string {
	paths := make([]string, 0, len(scanned))
	for _, scannedProject := range scanned {
		paths = append(paths, scannedProject.Path)
	}
	return paths
}

func projectPathsForPorts(projects []project.Project) []string {
	paths := make([]string, 0, len(projects))
	for _, project := range projects {
		paths = append(paths, project.Path)
	}
	return paths
}

func (m *Model) startInputCursorBlink() tea.Cmd {
	m.cursorHidden = false
	m.cursorBlinkID++
	return inputCursorBlink(m.cursorBlinkID)
}

func (m Model) cursorBlinkActive() bool {
	if m.searching {
		return true
	}
	switch m.screen {
	case screenAdd, screenNote, screenStatusInput, screenCommand, screenOnboardingInput, screenConfigInput, screenConfigRootsInput:
		return true
	case screenRunner:
		return true
	case screenConfigList:
		return true
	default:
		return false
	}
}

func (m Model) inputCursorState() inputCursorState {
	return inputCursorState{Visible: !m.cursorHidden}
}

func inputCursorBlink(id int) tea.Cmd {
	return tea.Tick(inputCursorBlinkSpeed, func(time.Time) tea.Msg {
		return inputCursorBlinkMsg{id: id}
	})
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

func (m Model) saveRunnerScript(name, command string) tea.Cmd {
	project, ok := m.currentProject()
	return func() tea.Msg {
		if !ok {
			return metadataFailedMsg{err: errNoProjectSelected{}}
		}
		scriptCommand := command
		if _, err := m.updater(project.Path, app.MetadataUpdate{Scripts: map[string]*string{name: &scriptCommand}}); err != nil {
			return metadataFailedMsg{err: err}
		}
		result, err := m.loader(m.request)
		if err != nil {
			return overviewLoadFailedMsg{err: err}
		}
		return metadataSavedMsg{message: "Script saved", result: result, preservePath: project.Path}
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

func (m Model) openRunner() (Model, tea.Cmd) {
	_, ok := m.currentProject()
	if !ok {
		m.message = "No scripts found"
		return m, nil
	}
	m.screen = screenRunner
	m.runnerSelected = 0
	m.runnerInput = ""
	m.runnerCursor = 0
	m.runnerYOffset = 0
	m.runnerAddName = ""
	m.runnerAdding = false
	m.runnerShowInfo = false
	return m, m.startInputCursorBlink()
}

func (m Model) runSelectedScript(project project.Project, script runnerScript) tea.Cmd {
	runner := m.runner
	if runner == nil {
		runner = runScript
	}
	manager := scriptManager(project.Managers)
	if script.Command != "" {
		manager = ""
	}
	return runner(project.Path, manager, script.Name, script.Command)
}

func scriptManager(managers []string) string {
	for _, manager := range managers {
		switch strings.ToLower(manager) {
		case "pnpm", "bun", "yarn", "npm":
			return strings.ToLower(manager)
		}
	}
	return "npm"
}

func (m Model) loadSelectedRecent() tea.Cmd {
	if !m.showInlineDetail() {
		return nil
	}
	project, ok := m.currentProject()
	if !ok {
		return nil
	}
	commitsCached := true
	if project.Activity.HasGit && project.Activity.HasCommits {
		_, commitsCached = m.recentByPath[project.Path]
	}
	_, filesCached := m.filesByPath[project.Path]
	if commitsCached && filesCached {
		return nil
	}
	path := project.Path
	loader := m.recent
	if loader == nil {
		loader = loadRecentCommits
	}
	fileLoader := m.recentFiles
	if fileLoader == nil {
		fileLoader = loadRecentFiles
	}
	ignoreDirs := append([]string{}, m.config.IgnoreDirs...)
	loadCommits := project.Activity.HasGit && project.Activity.HasCommits && !commitsCached
	loadFiles := !filesCached
	return func() tea.Msg {
		now := time.Now()
		var commits []ovwformat.RecentCommit
		var commitErr error
		if loadCommits {
			commits, commitErr = loader(path, now)
		}
		var files []ovwformat.RecentFile
		var fileErr error
		if loadFiles {
			files, fileErr = fileLoader(path, ignoreDirs, now)
		}
		return recentLoadedMsg{path: path, commits: commits, files: files, commitErr: commitErr, fileErr: fileErr}
	}
}

func loadRecentCommits(path string, now time.Time) ([]ovwformat.RecentCommit, error) {
	commits, err := gitactivity.Recent(path, 3)
	if err != nil {
		return nil, err
	}
	return ovwformat.RecentCommits(commits, now), nil
}

func loadRecentFiles(path string, ignoreDirs []string, now time.Time) ([]ovwformat.RecentFile, error) {
	files, err := projectfiles.Recent(path, projectfiles.Options{IgnoreDirs: ignoreDirs}, 5)
	if err != nil {
		return nil, err
	}
	return ovwformat.RecentFiles(files, now), nil
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
		body = onboardingInputView(m.onboardInput, m.onboardCursor, m.onboardErr, m.inputCursorState())
	default:
		visible := m.visibleProjects()
		body = headerView(m)
		if m.message != "" {
			body += " " + mutedStyle.Render(m.message)
		}
		if len(visible) == 0 && m.search != "" {
			body += "\n\n" + mutedStyle.Render("No projects match search")
			body += "\n" + inlineActionHint("esc", "clear search")
		} else if len(visible) == 0 && m.pathScope() != "" {
			body += "\n\n" + mutedStyle.Render("No projects match path: "+m.pathScope())
		} else {
			content := m.tablePanel(visible)
			switch m.screen {
			case screenDetail:
				project, ok := m.currentProject()
				modal, maxOffset := detailModalViewWithScroll(project, ok, m.contentWidth(), m.tableHeight(), m.detailModalY, m.detailsExpanded, m.actionKeys())
				if m.detailModalY > maxOffset {
					m.detailModalY = maxOffset
					modal, _ = detailModalViewWithScroll(project, ok, m.contentWidth(), m.tableHeight(), m.detailModalY, m.detailsExpanded, m.actionKeys())
				}
				content = overlayModal(content, modal, m.contentWidth())
			case screenAdd:
				content = overlayModal(content, addProjectView(m.addInput, m.addCursor, m.addErr, m.inputCursorState()), m.contentWidth())
			case screenHelp:
				content = overlayModal(content, helpView(m.actionKeys()), m.contentWidth())
			case screenCommand:
				content = overlayModal(content, commandView(m.commandInput, m.commandCursor, m.filteredCommandActions(), m.commandSelected, m.inputCursorState()), m.contentWidth())
			case screenRunner:
				project, _ := m.currentProject()
				modal, maxOffset := runnerViewWithScroll(project, m.runnerSelected, m.runnerInput, m.runnerCursor, m.tableHeight(), m.runnerYOffset, m.runnerAddName, m.runnerAdding, m.runnerShowInfo, m.inputCursorState())
				if m.runnerYOffset > maxOffset {
					m.runnerYOffset = maxOffset
					modal, _ = runnerViewWithScroll(project, m.runnerSelected, m.runnerInput, m.runnerCursor, m.tableHeight(), m.runnerYOffset, m.runnerAddName, m.runnerAdding, m.runnerShowInfo, m.inputCursorState())
				}
				content = overlayModal(content, modal, m.contentWidth())
			case screenFilter:
				content = overlayModal(content, filterView(m.filterOptions(), m.filterSelected), m.contentWidth())
			case screenSort:
				content = overlayModal(content, sortView(sortOptions(m.config), m.sortSelected, m.activeSortDir), m.contentWidth())
			case screenColumns:
				content = overlayModal(content, columnsView(m.columnOrder, m.columnChecked, m.columnSelected, m.columnErr), m.contentWidth())
			case screenConfig:
				content = overlayModal(content, configView(m.configRows(), m.configSelected, m.configErr), m.contentWidth())
			case screenConfigList:
				content = overlayModal(content, configListView(m.configListTitle(), m.configListValues(), m.configListSelected(), m.configListInput, m.configListCursor, m.configListEditing, m.configErr, m.inputCursorState()), m.contentWidth())
			case screenConfigInput:
				content = overlayModal(content, configInputView(m.configInputTitle(), m.configInput, m.configInputPlaceholder(), m.configCursor, m.configErr, m.inputCursorState()), m.contentWidth())
			case screenConfigRoots:
				content = overlayModal(content, configRootsView(m.configRootPicker.visibleRows(), m.configRootPicker.selected, m.configErr), m.contentWidth())
			case screenConfigRootsInput:
				content = overlayModal(content, configRootInputView(m.configRootInput, m.configRootCursor, m.configErr, m.inputCursorState()), m.contentWidth())
			case screenConfigNote:
				content = overlayModal(content, configNoteView(m.configNoteRows(), m.configNoteSel, m.configErr), m.contentWidth())
			case screenConfigKeys:
				content = overlayModal(content, configKeysView(m.configKeyRows(), m.configKeySel, m.configErr), m.contentWidth())
			case screenNote:
				project, _ := m.currentProject()
				content = overlayModal(content, noteView(project.Name, m.noteInput, project.Note.Display, m.noteCursor, m.inputCursorState()), m.contentWidth())
			case screenStatus:
				project, _ := m.currentProject()
				content = overlayModal(content, statusView(project.Name, m.statusOptions(), m.statusSelected), m.contentWidth())
			case screenStatusInput:
				project, _ := m.currentProject()
				content = overlayModal(content, statusInputView(project.Name, m.statusInput, m.statusCursor, m.inputCursorState()), m.contentWidth())
			}
			body += "\n\n" + content
		}
	}
	if m.screen != screenOnboarding && m.screen != screenOnboardingInput {
		body = pinFooter(body, m.footerView(), m.height)
	}
	return body
}

func (m Model) footerView() string {
	left := m.footerPath()
	right := "ctrl+p command"
	width := m.contentWidth()
	if width <= 0 {
		return mutedStyle.Render(right)
	}
	if left == "" {
		return mutedStyle.Render(right)
	}
	if lipglossWidth(left)+lipglossWidth(right)+2 > width {
		leftWidth := width - lipglossWidth(right) - 2
		if leftWidth <= 0 {
			return mutedStyle.Render(truncateText(right, width))
		}
		left = truncateText(left, leftWidth)
	}
	gap := width - lipglossWidth(left) - lipglossWidth(right)
	if gap < 1 {
		gap = 1
	}
	return mutedStyle.Render(left + strings.Repeat(" ", gap) + right)
}

func (m Model) footerPath() string {
	if scope := m.pathScope(); scope != "" {
		return shortPath(m.resolveScopePath(scope))
	}
	if root := strings.TrimSpace(m.request.SessionRoot); root != "" {
		return shortPath(m.resolveScopePath(root))
	}
	if len(m.config.Roots) == 1 {
		return shortPath(m.resolveScopePath(m.config.Roots[0]))
	}
	if len(m.config.Roots) > 1 {
		roots := make([]string, 0, len(m.config.Roots))
		for _, root := range m.config.Roots {
			if root = strings.TrimSpace(root); root != "" {
				roots = append(roots, m.resolveScopePath(root))
			}
		}
		return formatRootScope(roots)
	}
	return ""
}

func (m Model) resolveScopePath(scope string) string {
	if scope == "~" || strings.HasPrefix(scope, "~/") {
		if expanded, err := config.ExpandPath(scope); err == nil {
			return expanded
		}
		return scope
	}
	if filepath.IsAbs(scope) {
		if abs, err := filepath.Abs(scope); err == nil {
			return abs
		}
		return scope
	}
	cwd := m.request.Cwd
	if cwd == "" {
		if value, err := os.Getwd(); err == nil {
			cwd = value
		}
	}
	if cwd == "" {
		return scope
	}
	return filepath.Join(cwd, scope)
}

func formatRootScope(roots []string) string {
	roots = cleanRootPaths(roots)
	if len(roots) == 0 {
		return ""
	}
	if len(roots) == 1 {
		return shortPath(roots[0])
	}
	if parent, ok := commonRootParent(roots); ok {
		return shortPath(parent)
	}
	display := make([]string, 0, 2)
	for index := 0; index < len(roots) && index < 2; index++ {
		display = append(display, shortPath(roots[index]))
	}
	if remaining := len(roots) - len(display); remaining > 0 {
		display = append(display, fmt.Sprintf("+%d more", remaining))
	}
	return strings.Join(display, ", ")
}

func cleanRootPaths(roots []string) []string {
	seen := map[string]bool{}
	cleaned := make([]string, 0, len(roots))
	for _, root := range roots {
		root = strings.TrimSpace(root)
		if root == "" {
			continue
		}
		root = filepath.Clean(root)
		if seen[root] {
			continue
		}
		seen[root] = true
		cleaned = append(cleaned, root)
	}
	return cleaned
}

func commonRootParent(roots []string) (string, bool) {
	if len(roots) < 2 {
		return "", false
	}
	parent := filepath.Dir(roots[0])
	if parent == "." || parent == string(filepath.Separator) {
		return "", false
	}
	for _, root := range roots[1:] {
		if filepath.Dir(root) != parent {
			return "", false
		}
	}
	return parent, true
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
	visible := m.visibleProjects()
	leftParts := []string{formatProjectCount(len(visible))}
	if m.search != "" || m.searching {
		leftParts = append(leftParts, "search: "+m.searchDisplay())
	} else if m.enriching && m.enrichTotal > 0 {
		leftParts = append(leftParts, fmt.Sprintf("enriching %d/%d", m.enrichedCount, m.enrichTotal))
	} else if elapsed := formatScanElapsed(m.scanElapsed); m.showScanElapsed && elapsed != "" {
		leftParts = append(leftParts, elapsed)
	}
	rightParts := []string{"filter: " + headerFilter(m.activeFilter)}
	if m.activeSort != "" && !tableColumnVisible(tableColumns(m.config), m.activeSort) {
		rightParts = append(rightParts, "sort: "+sortHeaderCompact(m.activeSort, m.activeSortDir))
	}
	rightParts = append(rightParts, selectedPosition(m.selected, len(visible)))

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
	return fmt.Sprintf("scanned in %s", ovwformat.Elapsed(elapsed))
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
			if files, ok := m.filesByPath[detail.Path]; ok {
				detail.RecentFiles = files
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
	case screenTable, screenDetail, screenAdd, screenHelp, screenCommand, screenRunner, screenFilter, screenSort, screenColumns, screenConfig, screenConfigList, screenConfigInput, screenConfigRoots, screenConfigRootsInput, screenConfigNote, screenConfigKeys, screenNote, screenStatus, screenStatusInput:
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

func (m *Model) replaceProject(updated project.Project) {
	for index, project := range m.projects {
		if project.Path == updated.Path {
			m.projects[index] = updated
			return
		}
	}
	m.projects = append(m.projects, updated)
}

func (m Model) searchDisplay() string {
	if m.searching {
		return searchInputLine(m.search, "", m.searchCursor, m.inputCursorState())
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
	projects, err := filter.Apply(m.projects, filter.Options{
		Status:   m.request.Status,
		Dirty:    m.request.Dirty,
		Stale:    m.request.Stale,
		Untagged: m.request.Untagged,
		Hidden:   m.request.Hidden,
	}, m.config, time.Now())
	if err != nil {
		projects = m.projects
	}
	query := strings.TrimSpace(strings.ToLower(m.search))
	if query == "" {
		return projects
	}
	visible := make([]project.Project, 0, len(projects))
	for _, project := range projects {
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
