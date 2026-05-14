package tui

import (
	"bytes"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"ovw/internal/app"
	"ovw/internal/config"
	ovwformat "ovw/internal/format"
	"ovw/internal/project"
	"ovw/internal/scanner"

	tea "github.com/charmbracelet/bubbletea"
)

func TestNewModelRendersLoadingState(t *testing.T) {
	got := New().View()
	for _, want := range []string{"ovw", "Loading projects...", "ctrl+p command"} {
		if !strings.Contains(got, want) {
			t.Fatalf("View() missing %q:\n%s", want, got)
		}
	}
}

func TestSetTerminalTitleWritesOSCSequence(t *testing.T) {
	var out bytes.Buffer
	setTerminalTitle(&out, "ovw")

	if got, want := out.String(), "\033]0;ovw\007"; got != want {
		t.Fatalf("setTerminalTitle() = %q, want %q", got, want)
	}
}

func TestModelStoresWindowSize(t *testing.T) {
	model, _ := New().Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	got := model.(Model)
	width, height := got.Size()
	if width != 100 || height != 30 {
		t.Fatalf("Size() = %dx%d, want 100x30", width, height)
	}
}

func TestModelUsesFullTerminalFrame(t *testing.T) {
	model := Model{
		width:  100,
		height: 30,
		projects: []project.Project{
			{Name: "app", StackDisplay: "Go"},
		},
	}

	view := model.View()
	if strings.HasPrefix(view, " ") || strings.HasPrefix(view, "\n") {
		t.Fatalf("View() should start at terminal origin:\n%q", view[:min(len(view), 20)])
	}
	if strings.Contains(view, "100x30") {
		t.Fatalf("View() should not render debug dimensions:\n%s", view)
	}
	if model.contentWidth() != 100 {
		t.Fatalf("contentWidth() = %d, want 100", model.contentWidth())
	}
	if model.tableHeight() != 26 {
		t.Fatalf("tableHeight() = %d, want 26", model.tableHeight())
	}
}

func TestModelPinsFooterToBottom(t *testing.T) {
	model := Model{
		width:  100,
		height: 12,
		projects: []project.Project{
			{Name: "app", StackDisplay: "Go"},
		},
	}

	lines := strings.Split(stripANSI(model.View()), "\n")
	if len(lines) != model.height {
		t.Fatalf("View() rendered %d lines, want %d:\n%s", len(lines), model.height, strings.Join(lines, "\n"))
	}
	if !strings.Contains(lines[len(lines)-1], "ctrl+p command") {
		t.Fatalf("footer should stay on last row:\n%s", strings.Join(lines, "\n"))
	}
	if !strings.Contains(lines[0], "ovw") {
		t.Fatalf("header should stay on first row:\n%s", strings.Join(lines, "\n"))
	}
}

func TestModelClipsContentBeforeFooter(t *testing.T) {
	model := Model{
		width:  100,
		height: 5,
		projects: []project.Project{
			{Name: "app", StackDisplay: "Go"},
			{Name: "api", StackDisplay: "Go"},
			{Name: "web", StackDisplay: "Node"},
		},
	}

	lines := strings.Split(stripANSI(model.View()), "\n")
	if len(lines) != model.height {
		t.Fatalf("View() rendered %d lines, want %d:\n%s", len(lines), model.height, strings.Join(lines, "\n"))
	}
	if !strings.Contains(lines[len(lines)-1], "ctrl+p command") {
		t.Fatalf("footer should remain visible when content is clipped:\n%s", strings.Join(lines, "\n"))
	}
}

func TestModelLoadsOverviewData(t *testing.T) {
	model := NewWithLoader(func(opts app.Options) (app.OverviewResult, error) {
		return app.OverviewResult{
			Config: config.Default(),
			Projects: []project.Project{
				{
					Name:         "app",
					StackDisplay: "Go",
					Activity:     ovwformat.ActivityInfo{Display: "12m"},
					Status:       ovwformat.StatusFromTags("", []string{"dirty"}),
					Note:         ovwformat.NoteInfo{Display: "user note"},
				},
			},
		}, nil
	})

	cmd := model.Init()
	updated, _ := model.Update(cmd())
	got := updated.(Model)
	if got.loading {
		t.Fatal("model is still loading")
	}
	if got.loadErr != nil {
		t.Fatalf("loadErr = %v", got.loadErr)
	}
	if len(got.projects) != 1 || got.projects[0].Name != "app" {
		t.Fatalf("projects = %#v", got.projects)
	}
	view := got.View()
	for _, want := range []string{"ovw", "1 project", "Name", "Stack", "Activity", "Status", "Note", "app", "Go", "12m", "dirty", "user note"} {
		if !strings.Contains(view, want) {
			t.Fatalf("View() missing %q:\n%s", want, view)
		}
	}
}

func TestModelStartsFromDiscoveryBeforeFullLoad(t *testing.T) {
	calledFullLoader := false
	calledDiscover := false
	model := NewWithOptions(app.Options{})
	model.setup = func(app.Options) (app.ConfigSetup, error) {
		return app.ConfigSetup{Exists: true}, nil
	}
	model.loader = func(app.Options) (app.OverviewResult, error) {
		calledFullLoader = true
		return app.OverviewResult{}, nil
	}
	model.discover = func(app.Options) (app.OverviewResult, error) {
		calledDiscover = true
		return app.OverviewResult{
			Config:   config.Default(),
			Projects: []project.Project{{Name: "app", Path: "/tmp/app"}},
		}, nil
	}

	cmd := model.Init()
	updated, _ := model.Update(cmd())
	got := updated.(Model)
	if !calledDiscover {
		t.Fatal("discovery loader was not called")
	}
	if calledFullLoader {
		t.Fatal("full loader should not block initial TUI startup")
	}
	if got.loading || got.enriching {
		t.Fatalf("loading=%v enriching=%v", got.loading, got.enriching)
	}
	if len(got.projects) != 1 || got.projects[0].Name != "app" {
		t.Fatalf("projects = %#v", got.projects)
	}
}

func TestModelKeepsSelectionIndexStableDuringEnrichment(t *testing.T) {
	model := Model{
		config:        config.Default(),
		projects:      []project.Project{{Name: "old", Path: "/tmp/old"}, {Name: "new", Path: "/tmp/new"}},
		activeSort:    "activity",
		activeSortDir: "desc",
		selected:      1,
		enriching:     true,
	}
	update := enrichmentUpdate{
		project: project.Project{
			Name:     "new",
			Path:     "/tmp/new",
			Activity: ovwformat.ActivityInfo{LastCommitAt: time.Now(), Display: "1m"},
		},
		done:  1,
		total: 2,
	}

	updated, _ := model.Update(projectEnrichedMsg{update: update})
	got := updated.(Model)
	if got.selected != 1 {
		t.Fatalf("selected = %d, want cursor index to stay 1", got.selected)
	}
	if got.projects[0].Path != "/tmp/new" {
		t.Fatalf("expected enriched project to sort first: %#v", got.projects)
	}
	selected, ok := got.currentProject()
	if !ok || selected.Path != "/tmp/old" {
		t.Fatalf("selected row should remain the same row after resort, got projects=%#v selected=%d", got.projects, got.selected)
	}
}

func TestModelStartsOnboardingWhenConfigIsMissing(t *testing.T) {
	model := NewWithOptions(app.Options{})
	model.width = 100
	model.setup = func(app.Options) (app.ConfigSetup, error) {
		return app.ConfigSetup{Candidates: []string{"/tmp/dev", "~/Projects"}}, nil
	}

	cmd := model.Init()
	updated, _ := model.Update(cmd())
	got := updated.(Model)
	if got.screen != screenOnboarding {
		t.Fatalf("screen = %v, want onboarding", got.screen)
	}
	view := stripANSI(got.View())
	for _, want := range []string{"ovw", "A terminal overview for your local projects.", "Select project folders to scan", "> [x] /tmp/dev", "[ ] ~/Projects", "custom path", "space toggle", "enter continue"} {
		if !strings.Contains(view, want) {
			t.Fatalf("onboarding view missing %q:\n%s", want, view)
		}
	}
}

func TestModelOnboardingSelectionCreatesConfigAndLoadsOverview(t *testing.T) {
	var createdRoot string
	model := Model{
		width:          100,
		screen:         screenOnboarding,
		onboardOptions: []string{"/tmp/dev", "~/Projects"},
		onboardChecked: map[string]bool{"/tmp/dev": true},
		roots: func(roots []string) (config.FilePaths, config.Config, error) {
			if len(roots) > 0 {
				createdRoot = roots[0]
			}
			cfg := config.Default()
			cfg.Roots = roots
			return config.FilePaths{}, cfg, nil
		},
		loader: func(app.Options) (app.OverviewResult, error) {
			return app.OverviewResult{
				Config:   config.Default(),
				Projects: []project.Project{{Name: "app", Path: "/tmp/dev/app"}},
			}, nil
		},
	}

	updated, cmd := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(Model)
	if cmd == nil {
		t.Fatal("expected onboarding create command")
	}
	model = updateMsg(t, model, cmd())
	if createdRoot != "/tmp/dev" {
		t.Fatalf("createdRoot = %q, want /tmp/dev", createdRoot)
	}
	if model.screen != screenTable {
		t.Fatalf("screen = %v, want table", model.screen)
	}
	if len(model.projects) != 1 || model.projects[0].Name != "app" {
		t.Fatalf("projects = %#v", model.projects)
	}
}

func TestModelOnboardingCustomPathValidationStaysInInput(t *testing.T) {
	model := Model{
		width:          100,
		screen:         screenOnboarding,
		onboardOptions: []string{"/tmp/dev"},
		onboardChecked: map[string]bool{"/tmp/dev": true},
		roots: func(roots []string) (config.FilePaths, config.Config, error) {
			return config.FilePaths{}, config.Config{}, errors.New("path does not exist")
		},
	}
	model.onboardSelected = 1
	model = updateSpecialKey(t, model, tea.KeyEnter)
	if model.screen != screenOnboardingInput {
		t.Fatalf("screen = %v, want onboarding input", model.screen)
	}
	for _, value := range []string{"/", "n", "o", "p", "e"} {
		model = updateKey(t, model, value)
	}
	updated, cmd := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(Model)
	if cmd == nil {
		t.Fatal("expected onboarding create command")
	}
	model = updateMsg(t, model, cmd())
	if model.screen != screenOnboardingInput {
		t.Fatalf("screen = %v, want onboarding input", model.screen)
	}
	if !strings.Contains(stripANSI(model.View()), "path does not exist") {
		t.Fatalf("onboarding input should show validation error:\n%s", stripANSI(model.View()))
	}
}

func TestModelOnboardingCustomPathSupportsCursorEditing(t *testing.T) {
	model := Model{
		screen:         screenOnboarding,
		onboardOptions: []string{"/tmp/dev"},
		onboardChecked: map[string]bool{},
	}
	model.onboardSelected = 1
	model = updateSpecialKey(t, model, tea.KeyEnter)
	for _, value := range []string{"/", "t", "m", "p", "/", "a", "p"} {
		model = updateKey(t, model, value)
	}
	model = updateSpecialKey(t, model, tea.KeyLeft)
	model = updateKey(t, model, "p")

	if model.onboardInput != "/tmp/app" {
		t.Fatalf("onboardInput = %q, want /tmp/app", model.onboardInput)
	}
	if model.onboardCursor != len([]rune("/tmp/app"))-1 {
		t.Fatalf("onboardCursor = %d, want before last p", model.onboardCursor)
	}
	if !strings.Contains(stripANSI(model.View()), "/tmp/ap▌p") {
		t.Fatalf("onboarding input cursor not rendered in place:\n%s", stripANSI(model.View()))
	}

	model = updateSpecialKey(t, model, tea.KeyDelete)
	if model.onboardInput != "/tmp/ap" {
		t.Fatalf("onboardInput after delete = %q, want /tmp/ap", model.onboardInput)
	}
}

func TestSetupExpandsRootAndSelectsChildFolder(t *testing.T) {
	var created []string
	model := setupModel{
		options:  []string{"~/dev"},
		checked:  map[string]bool{"~/dev": true},
		expanded: map[string]bool{},
		children: map[string][]string{
			"~/dev": {"~/dev/web", "~/dev/extensions"},
		},
		creator: func(roots []string) (config.FilePaths, config.Config, error) {
			created = append([]string{}, roots...)
			return config.FilePaths{}, config.Default(), nil
		},
	}

	model = updateSetupSpecialKey(t, model, tea.KeyRight)
	view := stripANSI(model.View())
	if !strings.Contains(view, "▾") || !strings.Contains(view, "web") || !strings.Contains(view, "extensions") {
		t.Fatalf("expanded setup view missing child folders:\n%s", view)
	}
	if !strings.Contains(view, "[x] web") || !strings.Contains(view, "[x] extensions") {
		t.Fatalf("children should show included when parent is checked:\n%s", view)
	}
	model = updateSetupSpecialKey(t, model, tea.KeyDown)
	model = updateSetupKey(t, model, " ")
	if model.checked["~/dev"] {
		t.Fatal("parent root should be unchecked when child is excluded")
	}
	if model.checked["~/dev/web"] {
		t.Fatal("excluded child root should be unchecked")
	}
	if !model.checked["~/dev/extensions"] {
		t.Fatal("sibling child root should remain checked")
	}
	model = updateSetupSpecialKey(t, model, tea.KeyLeft)
	model = updateSetupSpecialKey(t, model, tea.KeyLeft)
	view = stripANSI(model.View())
	if !strings.Contains(view, "[-] ~/dev") {
		t.Fatalf("collapsed parent should show partial child exclusion:\n%s", view)
	}
	model = updateSetupSpecialKey(t, model, tea.KeyRight)
	model = updateSetupSpecialKey(t, model, tea.KeyDown)

	updated, cmd := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(setupModel)
	if cmd == nil {
		t.Fatal("expected setup create command")
	}
	model = updateSetupMsg(t, model, cmd())
	if len(created) != 1 || created[0] != "~/dev/extensions" {
		t.Fatalf("created roots = %#v, want ~/dev/extensions", created)
	}
}

func TestSetupLoadsProjectCountsInBackground(t *testing.T) {
	var counted []string
	model := setupModel{
		options:  []string{"~/dev", "~/Projects"},
		checked:  map[string]bool{"~/dev": true},
		expanded: map[string]bool{},
		children: map[string][]string{},
		counts:   map[string]int{},
		counter: func(paths []string) map[string]int {
			counted = append([]string{}, paths...)
			return map[string]int{"~/dev": 5, "~/Projects": 0}
		},
	}

	view := stripANSI(model.View())
	if strings.Contains(view, "counting") || strings.Contains(view, " -") {
		t.Fatalf("setup view should not show count placeholders:\n%s", view)
	}
	cmd := model.Init()
	if cmd == nil {
		t.Fatal("expected setup count command")
	}
	model = updateSetupMsg(t, model, cmd())
	view = stripANSI(model.View())
	for _, want := range []string{"~/dev       5", "~/Projects  0"} {
		if !strings.Contains(view, want) {
			t.Fatalf("setup view missing count %q:\n%s", want, view)
		}
	}
	if !reflect.DeepEqual(counted, []string{"~/dev", "~/Projects"}) {
		t.Fatalf("counted paths = %#v", counted)
	}
}

func TestSetupCountsExpandedChildrenOnlyAfterExpand(t *testing.T) {
	model := setupModel{
		options:  []string{"~/dev"},
		checked:  map[string]bool{"~/dev": true},
		expanded: map[string]bool{},
		children: map[string][]string{
			"~/dev": {"~/dev/web", "~/dev/extensions"},
		},
		counts: map[string]int{"~/dev": 3},
		counter: func(paths []string) map[string]int {
			return map[string]int{"~/dev/web": 2, "~/dev/extensions": 1}
		},
	}

	updated, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRight})
	model = updated.(setupModel)
	if cmd == nil {
		t.Fatal("expected child count command")
	}
	view := stripANSI(model.View())
	if strings.Contains(view, "web  2") || strings.Contains(view, "extensions  1") {
		t.Fatalf("child counts should not appear before count command completes:\n%s", view)
	}
	model = updateSetupMsg(t, model, cmd())
	view = stripANSI(model.View())
	for _, want := range []string{"web         2", "extensions  1"} {
		if !strings.Contains(view, want) {
			t.Fatalf("expanded setup view missing child count %q:\n%s", want, view)
		}
	}
}

func TestSetupCustomPathSupportsCursorEditing(t *testing.T) {
	model := setupModel{
		options:   []string{"/tmp/dev"},
		checked:   map[string]bool{},
		expanded:  map[string]bool{},
		children:  map[string][]string{},
		selected:  1,
		inputting: true,
	}
	for _, value := range []string{"/", "t", "m", "p", "/", "a", "p"} {
		model = updateSetupKey(t, model, value)
	}
	model = updateSetupSpecialKey(t, model, tea.KeyLeft)
	model = updateSetupKey(t, model, "p")

	if model.input != "/tmp/app" {
		t.Fatalf("input = %q, want /tmp/app", model.input)
	}
	if model.cursor != len([]rune("/tmp/app"))-1 {
		t.Fatalf("cursor = %d, want before last p", model.cursor)
	}
	if !strings.Contains(stripANSI(model.View()), "/tmp/ap▌p") {
		t.Fatalf("setup input cursor not rendered in place:\n%s", stripANSI(model.View()))
	}

	model = updateSetupSpecialKey(t, model, tea.KeyDelete)
	if model.input != "/tmp/ap" {
		t.Fatalf("input after delete = %q, want /tmp/ap", model.input)
	}
}

func TestInputCursorRendersOnceAtWrapBoundary(t *testing.T) {
	got := strings.Count(stripANSI(addProjectView(strings.Repeat("a", 60), 51, "")), "▌")
	if got != 1 {
		t.Fatalf("cursor count = %d, want 1", got)
	}
}

func TestSetupExpandsNestedFoldersLazily(t *testing.T) {
	var created []string
	model := setupModel{
		options:  []string{"~/dev"},
		checked:  map[string]bool{"~/dev": true},
		expanded: map[string]bool{},
		children: map[string][]string{
			"~/dev":     {"~/dev/web", "~/dev/extensions"},
			"~/dev/web": {"~/dev/web/eventca", "~/dev/web/other"},
		},
		creator: func(roots []string) (config.FilePaths, config.Config, error) {
			created = append([]string{}, roots...)
			return config.FilePaths{}, config.Default(), nil
		},
	}

	model = updateSetupSpecialKey(t, model, tea.KeyRight)
	model = updateSetupSpecialKey(t, model, tea.KeyDown)
	model = updateSetupSpecialKey(t, model, tea.KeyRight)
	view := stripANSI(model.View())
	if !strings.Contains(view, "eventca") {
		t.Fatalf("nested setup view missing grandchild folder:\n%s", view)
	}
	model = updateSetupSpecialKey(t, model, tea.KeyDown)
	model = updateSetupKey(t, model, " ")
	if model.checked["~/dev"] || model.checked["~/dev/web"] {
		t.Fatal("ancestors should be unchecked when nested child is excluded")
	}
	if model.checked["~/dev/web/eventca"] {
		t.Fatal("nested child should be unchecked")
	}
	if !model.checked["~/dev/extensions"] || !model.checked["~/dev/web/other"] {
		t.Fatalf("sibling branches should stay checked: %#v", model.checked)
	}
	model = updateSetupSpecialKey(t, model, tea.KeyLeft)
	model = updateSetupSpecialKey(t, model, tea.KeyLeft)
	view = stripANSI(model.View())
	if !strings.Contains(view, "[-] web") {
		t.Fatalf("collapsed child should show partial selection:\n%s", view)
	}
	model = updateSetupSpecialKey(t, model, tea.KeyLeft)
	model = updateSetupSpecialKey(t, model, tea.KeyLeft)
	view = stripANSI(model.View())
	if !strings.Contains(view, "[-] ~/dev") {
		t.Fatalf("collapsed root should show partial selection:\n%s", view)
	}

	model = updateSetupSpecialKey(t, model, tea.KeyRight)
	model = updateSetupSpecialKey(t, model, tea.KeyDown)
	model = updateSetupSpecialKey(t, model, tea.KeyRight)
	model = updateSetupSpecialKey(t, model, tea.KeyDown)
	updated, cmd := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(setupModel)
	if cmd == nil {
		t.Fatal("expected setup create command")
	}
	model = updateSetupMsg(t, model, cmd())
	want := []string{"~/dev/web/other", "~/dev/extensions"}
	if !reflect.DeepEqual(created, want) {
		t.Fatalf("created roots = %#v, want %#v", created, want)
	}
}

func TestSetupNestsCheckedRootsUnderAncestorCandidate(t *testing.T) {
	model := setupModel{
		options:  []string{"~/dev"},
		checked:  map[string]bool{"~/dev/extensions": true},
		expanded: map[string]bool{},
		children: map[string][]string{},
	}

	view := stripANSI(model.View())
	if strings.Contains(view, "~/dev/extensions") {
		t.Fatalf("descendant root should not render as top-level row:\n%s", view)
	}
	if !strings.Contains(view, "[-] ~/dev") {
		t.Fatalf("ancestor should show partial selection:\n%s", view)
	}

	model = updateSetupSpecialKey(t, model, tea.KeyRight)
	view = stripANSI(model.View())
	if !strings.Contains(view, "[x] extensions") {
		t.Fatalf("checked descendant should render as nested child:\n%s", view)
	}
}

func TestSetupShowsParentCheckedWhenEveryChildIsChecked(t *testing.T) {
	model := setupModel{
		options: []string{"~/dev"},
		checked: map[string]bool{
			"~/dev/extensions":     true,
			"~/dev/fork":           true,
			"~/dev/ovw-screenshot": true,
			"~/dev/playground":     true,
			"~/dev/web":            true,
		},
		expanded: map[string]bool{"~/dev": true},
		children: map[string][]string{
			"~/dev": {
				"~/dev/extensions",
				"~/dev/fork",
				"~/dev/ovw-screenshot",
				"~/dev/playground",
				"~/dev/web",
			},
		},
	}

	view := stripANSI(model.View())
	if !strings.Contains(view, "▾ [x] ~/dev") {
		t.Fatalf("parent should render checked when every child is checked:\n%s", view)
	}
	if strings.Contains(view, "[-] ~/dev") {
		t.Fatalf("parent should not render partial when every child is checked:\n%s", view)
	}

	model = updateSetupKey(t, model, " ")
	for path, checked := range model.checked {
		if checked {
			t.Fatalf("checking all children then toggling parent should clear descendants, but %s is still checked: %#v", path, model.checked)
		}
	}
}

func TestSetupRevealsCheckedNestedRoots(t *testing.T) {
	model := setupModel{
		options:  []string{"~/dev"},
		checked:  map[string]bool{"~/dev/web/eventca": true},
		expanded: map[string]bool{},
		children: map[string][]string{},
	}
	model.revealCheckedRoots()

	view := stripANSI(model.View())
	for _, want := range []string{"▾ [-] ~/dev", "▾ [-] web", "[x] eventca"} {
		if !strings.Contains(view, want) {
			t.Fatalf("revealed setup view missing %q:\n%s", want, view)
		}
	}
}

func TestModelKeepsTableVisibleDuringBackgroundLoading(t *testing.T) {
	model := Model{
		width:        120,
		loading:      true,
		activeFilter: "all",
		activeSort:   "activity",
		projects: []project.Project{
			{
				Name:         "app",
				StackDisplay: "Go",
				Activity:     ovwformat.ActivityInfo{Display: "12m"},
				Status:       ovwformat.StatusFromTags("", []string{"dirty"}),
				Note:         ovwformat.NoteInfo{Display: "user note"},
			},
		},
	}

	view := stripANSI(model.View())
	for _, want := range []string{"ovw  1 project", "filter: all", "1/1", "Activity ↓", "app", "Go", "12m", "dirty", "user note"} {
		if !strings.Contains(view, want) {
			t.Fatalf("background loading view missing %q:\n%s", want, view)
		}
	}
	for _, notWant := range []string{"Loading projects...", "Loading...", "Saving note...", "Saving status...", "Filtering...", "Sorting..."} {
		if strings.Contains(view, notWant) {
			t.Fatalf("background loading should not show %q:\n%s", notWant, view)
		}
	}
}

func TestModelHeaderShowsFilterAndSort(t *testing.T) {
	model := Model{
		width:         80,
		activeFilter:  "dirty",
		activeSort:    "name",
		activeSortDir: "asc",
		projects: []project.Project{
			{Name: "app"},
			{Name: "api"},
		},
	}

	view := stripANSI(model.View())
	header := strings.Split(view, "\n")[0]
	for _, want := range []string{"ovw  2 projects", "filter: dirty"} {
		if !strings.Contains(header, want) {
			t.Fatalf("header missing %q:\n%s", want, header)
		}
	}
	if strings.Index(header, "filter: dirty") < strings.Index(header, "2 projects") {
		t.Fatalf("header should keep filter on the right:\n%s", view)
	}
	if strings.Contains(header, "sort:") {
		t.Fatalf("header should not repeat visible sort column:\n%s", header)
	}
	if !strings.HasSuffix(header, "1/2") {
		t.Fatalf("header should put position on the right:\n%s", header)
	}
	if strings.Contains(view, "ovw -") {
		t.Fatalf("header should not use dash separator:\n%s", view)
	}
}

func TestModelHeaderShowsAllFilter(t *testing.T) {
	model := Model{
		width:         80,
		activeFilter:  "all",
		activeSort:    "activity",
		activeSortDir: "desc",
		projects:      []project.Project{{Name: "app"}},
	}

	header := strings.Split(stripANSI(model.View()), "\n")[0]
	for _, want := range []string{"ovw  1 project", "filter: all"} {
		if !strings.Contains(header, want) {
			t.Fatalf("header missing %q:\n%s", want, header)
		}
	}
	if !strings.HasSuffix(header, "1/1") {
		t.Fatalf("header should put position on the right:\n%s", header)
	}
	if strings.Contains(header, "sort:") {
		t.Fatalf("header should not repeat visible sort column:\n%s", header)
	}
}

func TestModelFooterShowsPathScope(t *testing.T) {
	t.Setenv("HOME", "/home/roie")

	model := Model{
		width:         100,
		height:        12,
		request:       app.Options{Path: "/home/roie/dev/extensions"},
		activeFilter:  "all",
		activeSort:    "activity",
		activeSortDir: "desc",
		projects:      []project.Project{{Name: "api"}},
	}

	lines := strings.Split(stripANSI(model.View()), "\n")
	header := lines[0]
	footer := lines[len(lines)-1]
	if strings.Contains(header, "~/dev/extensions") {
		t.Fatalf("header should not show path scope:\n%s", header)
	}
	if !strings.Contains(footer, "~/dev/extensions") {
		t.Fatalf("footer missing path scope:\n%s", footer)
	}
}

func TestModelHeaderShowsSortWhenSortColumnIsHidden(t *testing.T) {
	cfg := config.Default()
	cfg.Columns = []string{"name", "status", "note"}
	model := Model{
		width:         80,
		activeFilter:  "all",
		activeSort:    "activity",
		activeSortDir: "desc",
		config:        cfg,
		projects:      []project.Project{{Name: "app"}},
	}

	header := strings.Split(stripANSI(model.View()), "\n")[0]
	if !strings.Contains(header, "sort: activity ↓") {
		t.Fatalf("header should show hidden sort column:\n%s", header)
	}
}

func TestModelHeaderShowsScanElapsedWhenWide(t *testing.T) {
	model := Model{
		width:           120,
		activeFilter:    "all",
		activeSort:      "activity",
		activeSortDir:   "desc",
		scanElapsed:     200 * time.Millisecond,
		showScanElapsed: true,
		projects:        []project.Project{{Name: "app"}},
	}

	header := strings.Split(stripANSI(model.View()), "\n")[0]
	for _, want := range []string{"ovw  1 project", "scanned in 0.2s", "filter: all", "1/1"} {
		if !strings.Contains(header, want) {
			t.Fatalf("header missing %q:\n%s", want, header)
		}
	}
}

func TestModelHeaderDropsScanElapsedBeforeFilterSort(t *testing.T) {
	model := Model{
		width:           44,
		activeFilter:    "all",
		activeSort:      "activity",
		activeSortDir:   "desc",
		scanElapsed:     200 * time.Millisecond,
		showScanElapsed: true,
		projects:        []project.Project{{Name: "app"}},
	}

	header := strings.Split(stripANSI(model.View()), "\n")[0]
	if strings.Contains(header, "scanned in") {
		t.Fatalf("header should drop elapsed before filter/sort:\n%s", header)
	}
	for _, want := range []string{"ovw  1 project", "filter: all", "1/1"} {
		if !strings.Contains(header, want) {
			t.Fatalf("header missing %q:\n%s", want, header)
		}
	}
}

func TestModelHeaderSpacesSearchLikeOtherSegments(t *testing.T) {
	model := Model{
		activeFilter:    "all",
		activeSort:      "activity",
		activeSortDir:   "desc",
		scanElapsed:     200 * time.Millisecond,
		showScanElapsed: true,
		searching:       true,
		projects:        []project.Project{{Name: "app"}},
	}

	view := stripANSI(model.View())
	if !strings.Contains(view, "1 project  search: ▌") {
		t.Fatalf("header should separate sort and search consistently:\n%s", view)
	}
	if strings.Contains(view, "search:  ▌") {
		t.Fatalf("search label should not add extra spacing after colon:\n%s", view)
	}
	if strings.Contains(view, "scanned in") {
		t.Fatalf("search should replace scan time:\n%s", view)
	}
}

func TestModelHeaderHidesScanElapsedAfterInteraction(t *testing.T) {
	model := Model{
		width:           120,
		activeFilter:    "all",
		activeSort:      "activity",
		activeSortDir:   "desc",
		scanElapsed:     200 * time.Millisecond,
		showScanElapsed: true,
		projects:        []project.Project{{Name: "app"}, {Name: "api"}},
	}

	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyDown})
	next := updated.(Model)
	header := strings.Split(stripANSI(next.View()), "\n")[0]
	if strings.Contains(header, "scanned in") {
		t.Fatalf("header should hide elapsed after user interaction:\n%s", header)
	}
	for _, want := range []string{"ovw  2 projects", "filter: all", "2/2"} {
		if !strings.Contains(header, want) {
			t.Fatalf("header missing %q:\n%s", want, header)
		}
	}
}

func TestModelFooterShowsScopeAndCommandHint(t *testing.T) {
	t.Setenv("HOME", "/home/roie")
	model := Model{
		width:   80,
		request: app.Options{Path: "/home/roie/dev/web"},
	}

	got := stripANSI(model.footerView())
	if !strings.Contains(got, "~/dev/web") || !strings.HasSuffix(got, "ctrl+p command") {
		t.Fatalf("footer should show scoped path and command hint:\n%s", got)
	}
}

func TestModelFooterResolvesRelativeScope(t *testing.T) {
	t.Setenv("HOME", "/home/roie")
	model := Model{
		width:   80,
		request: app.Options{Path: "extensions", Cwd: "/home/roie/dev"},
	}

	got := stripANSI(model.footerView())
	if !strings.Contains(got, "~/dev/extensions") {
		t.Fatalf("footer should show complete resolved scope path:\n%s", got)
	}
}

func TestModelFooterShowsSessionRoot(t *testing.T) {
	t.Setenv("HOME", "/home/roie")
	model := Model{
		width:   80,
		request: app.Options{SessionRoot: "/home/roie/dev/web"},
	}

	got := stripANSI(model.footerView())
	if !strings.Contains(got, "~/dev/web") {
		t.Fatalf("footer should show session root:\n%s", got)
	}
}

func TestModelFooterShowsSingleConfigRoot(t *testing.T) {
	t.Setenv("HOME", "/home/roie")
	cfg := config.Default()
	cfg.Roots = []string{"~/dev/web"}
	model := Model{
		width:  80,
		config: cfg,
	}

	got := stripANSI(model.footerView())
	if !strings.Contains(got, "~/dev/web") {
		t.Fatalf("footer should show single configured root:\n%s", got)
	}
}

func TestModelFooterShowsMultipleConfigRoots(t *testing.T) {
	t.Setenv("HOME", "/home/roie")
	cfg := config.Default()
	cfg.Roots = []string{"~/dev/web", "~/dev/extensions"}
	model := Model{
		width:  100,
		config: cfg,
	}

	got := stripANSI(model.footerView())
	for _, want := range []string{"~/dev", "ctrl+p command"} {
		if !strings.Contains(got, want) {
			t.Fatalf("footer missing %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "~/dev/web") || strings.Contains(got, "~/dev/extensions") {
		t.Fatalf("footer should collapse sibling roots to shared parent:\n%s", got)
	}
}

func TestModelFooterShowsUnrelatedRootsWithMoreCount(t *testing.T) {
	t.Setenv("HOME", "/home/roie")
	cfg := config.Default()
	cfg.Roots = []string{"~/work/client-a", "~/personal/tools", "~/sandbox/lab", "~/src/lib", "~/tmp/demo"}
	model := Model{
		width:  120,
		config: cfg,
	}

	got := stripANSI(model.footerView())
	for _, want := range []string{"~/work/client-a", "~/personal/tools", "+3 more", "ctrl+p command"} {
		if !strings.Contains(got, want) {
			t.Fatalf("footer missing %q:\n%s", want, got)
		}
	}
}

func TestModelFooterDoesNotShowSelectedProjectPath(t *testing.T) {
	t.Setenv("HOME", "/home/roie")
	model := Model{
		width:    80,
		projects: []project.Project{{Name: "eventca", Path: "/home/roie/dev/web/eventca"}},
	}

	got := stripANSI(model.footerView())
	if strings.Contains(got, "eventca") {
		t.Fatalf("footer should not change with selected project path:\n%s", got)
	}
	if !strings.HasSuffix(got, "ctrl+p command") {
		t.Fatalf("footer should preserve command hint:\n%s", got)
	}
}

func TestModelFooterTruncatesLongScope(t *testing.T) {
	model := Model{
		width:   24,
		request: app.Options{Path: "/very/long/path/to/eventca"},
	}

	got := stripANSI(model.footerView())
	if lipglossWidth(got) > model.width {
		t.Fatalf("footer width = %d, want <= %d: %q", lipglossWidth(got), model.width, got)
	}
	if !strings.HasSuffix(got, "ctrl+p command") {
		t.Fatalf("footer should preserve command hint:\n%s", got)
	}
}

func TestModelHorizontalScrollKeysMoveTableViewport(t *testing.T) {
	model := Model{
		projects: []project.Project{{Name: "app", Path: "/tmp/app"}},
	}

	model = updateSpecialKey(t, model, tea.KeyRight)
	if model.tableXOffset != 8 {
		t.Fatalf("tableXOffset = %d, want 8", model.tableXOffset)
	}
	model = updateKey(t, model, "l")
	if model.tableXOffset != 16 {
		t.Fatalf("tableXOffset = %d, want 16", model.tableXOffset)
	}
	model = updateKey(t, model, "h")
	if model.tableXOffset != 8 {
		t.Fatalf("tableXOffset = %d, want 8", model.tableXOffset)
	}
	model = updateSpecialKey(t, model, tea.KeyLeft)
	model = updateSpecialKey(t, model, tea.KeyLeft)
	if model.tableXOffset != 0 {
		t.Fatalf("tableXOffset = %d, want 0", model.tableXOffset)
	}
}

func TestModelActionsDoNotSetTransientProgressMessages(t *testing.T) {
	model := Model{
		config: config.Default(),
		loader: func(opts app.Options) (app.OverviewResult, error) {
			return app.OverviewResult{Config: config.Default()}, nil
		},
		updater: func(path string, update app.MetadataUpdate) (app.MetadataUpdateResult, error) {
			return app.MetadataUpdateResult{Path: path}, nil
		},
		projects: []project.Project{{Name: "app", Path: "/tmp/app"}},
	}

	cases := []struct {
		name  string
		model Model
		key   string
	}{
		{name: "note", model: func() Model { m := model; m.screen = screenNote; return m }(), key: "enter"},
		{name: "status", model: func() Model { m := model; m.screen = screenStatus; return m }(), key: "enter"},
		{name: "filter", model: func() Model { m := model; m.screen = screenFilter; return m }(), key: "enter"},
		{name: "sort", model: func() Model { m := model; m.screen = screenSort; return m }(), key: "enter"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			updated := updateKey(t, tc.model, tc.key)
			for _, notWant := range []string{"Saving note...", "Saving status...", "Filtering...", "Sorting..."} {
				if updated.message == notWant {
					t.Fatalf("message = %q", updated.message)
				}
			}
		})
	}
}

func TestModelInitialLoadStillShowsLoadingState(t *testing.T) {
	model := Model{loading: true}

	view := stripANSI(model.View())
	if !strings.Contains(view, "Loading projects...") {
		t.Fatalf("background loading should not replace table:\n%s", view)
	}
}

func TestModelLoadsRecentCommitsOnlyForWideSidepane(t *testing.T) {
	model := NewWithLoader(func(opts app.Options) (app.OverviewResult, error) {
		return app.OverviewResult{
			Config: config.Default(),
			Projects: []project.Project{
				{
					Name:         "app",
					Path:         "/tmp/app",
					StackDisplay: "Go",
					Activity: ovwformat.ActivityInfo{
						Display:    "12m",
						HasGit:     true,
						HasCommits: true,
					},
				},
			},
		}, nil
	})
	model.width = 140
	model.height = 24
	model.recent = func(path string, now time.Time) ([]ovwformat.RecentCommit, error) {
		if path != "/tmp/app" {
			t.Fatalf("recent path = %q, want /tmp/app", path)
		}
		return []ovwformat.RecentCommit{{Hash: "abc1234", Subject: "fix lazy recent", Age: "2m"}}, nil
	}

	updated, cmd := model.Update(model.Init()())
	model = updated.(Model)
	if cmd == nil {
		t.Fatal("expected lazy recent commit command for wide sidepane")
	}
	updated, _ = model.Update(cmd())
	model = updated.(Model)

	view := stripANSI(model.View())
	for _, want := range []string{"Recent", "abc1234", "fix lazy recent", "2m"} {
		if !strings.Contains(view, want) {
			t.Fatalf("wide sidepane missing %q:\n%s", want, view)
		}
	}
}

func TestModelLoadsRecentFilesForWideSidepaneWithoutGit(t *testing.T) {
	model := NewWithLoader(func(opts app.Options) (app.OverviewResult, error) {
		return app.OverviewResult{
			Config: config.Default(),
			Projects: []project.Project{
				{Name: "app", Path: "/tmp/app", StackDisplay: "Go"},
			},
		}, nil
	})
	model.width = 140
	model.height = 24
	model.recentFiles = func(path string, ignoreDirs []string, now time.Time) ([]ovwformat.RecentFile, error) {
		if path != "/tmp/app" {
			t.Fatalf("recent files path = %q, want /tmp/app", path)
		}
		return []ovwformat.RecentFile{{Path: "internal/tui/app.go", Age: "4m"}}, nil
	}

	updated, cmd := model.Update(model.Init()())
	model = updated.(Model)
	if cmd == nil {
		t.Fatal("expected lazy recent files command for wide sidepane")
	}
	updated, _ = model.Update(cmd())
	model = updated.(Model)

	view := stripANSI(model.View())
	for _, want := range []string{"Recent files", "internal/tui/app.go", "4m"} {
		if !strings.Contains(view, want) {
			t.Fatalf("wide sidepane missing %q:\n%s", want, view)
		}
	}
}

func TestModelDoesNotLoadRecentCommitsForNarrowLayout(t *testing.T) {
	model := NewWithLoader(func(opts app.Options) (app.OverviewResult, error) {
		return app.OverviewResult{
			Config: config.Default(),
			Projects: []project.Project{
				{
					Name: "app",
					Path: "/tmp/app",
					Activity: ovwformat.ActivityInfo{
						Display:    "12m",
						HasGit:     true,
						HasCommits: true,
					},
				},
			},
		}, nil
	})
	model.width = 80
	model.height = 24
	model.recent = func(path string, now time.Time) ([]ovwformat.RecentCommit, error) {
		t.Fatalf("recent loader should not run for narrow layout")
		return nil, nil
	}

	_, cmd := model.Update(model.Init()())
	if cmd != nil {
		t.Fatal("expected no lazy recent command for narrow layout")
	}
}

func TestModelScrollsLongSidePaneDetails(t *testing.T) {
	model := Model{
		width:        140,
		height:       12,
		activeFilter: "all",
		activeSort:   "activity",
		screen:       screenTable,
		config:       config.Default(),
		projects: []project.Project{
			{
				Name:         "app",
				Path:         "/tmp/app",
				StackDisplay: "Go",
				Activity:     ovwformat.ActivityInfo{Display: "12m"},
				Status:       ovwformat.StatusFromTags("active", []string{"active"}),
				Note: ovwformat.NoteInfo{
					Display: strings.Join([]string{
						"line one",
						"line two",
						"line three",
						"line four",
						"line five",
						"line six",
						"line seven",
						"line eight",
						"line nine",
						"line ten",
					}, "\n"),
					Source: "user",
					Value:  "user note",
				},
			},
		},
	}

	view := stripANSI(model.View())
	if !strings.Contains(view, "↓ pgup/pgdn") {
		t.Fatalf("long sidepane should show scroll hint:\n%s", view)
	}
	if strings.Contains(view, "line ten") {
		t.Fatalf("long sidepane should start clipped:\n%s", view)
	}

	model = updateSpecialKey(t, model, tea.KeyPgDown)
	view = stripANSI(model.View())
	if model.detailYOffset == 0 || !strings.Contains(view, "line four") {
		t.Fatalf("sidepane should scroll down:\n%s", view)
	}
	if !strings.Contains(view, "↑ pgup/pgdn") && !strings.Contains(view, "↑↓ pgup/pgdn") {
		t.Fatalf("scrolled sidepane should show upward scroll hint:\n%s", view)
	}

	model = updateSpecialKey(t, model, tea.KeyEnd)
	view = stripANSI(model.View())
	if !strings.Contains(view, "line ten") {
		t.Fatalf("sidepane should jump to end:\n%s", view)
	}
}

func TestDetailScrollHintUsesKeyMarkerStyle(t *testing.T) {
	got := detailScrollHint(0, 3, 20)

	if !strings.Contains(got, "\x1b[38;5;252m↓") {
		t.Fatalf("scroll hint marker should use key style:\n%q", got)
	}
	if strings.Contains(stripANSI(got), "detail") {
		t.Fatalf("scroll hint should not include redundant detail label:\n%q", got)
	}
}

func TestModelDoesNotScrollShortSidePaneDetails(t *testing.T) {
	model := Model{
		width:        140,
		height:       18,
		activeFilter: "all",
		activeSort:   "activity",
		screen:       screenTable,
		config:       config.Default(),
		projects: []project.Project{
			{
				Name:         "app",
				Path:         "/tmp/app",
				StackDisplay: "Go",
				Activity:     ovwformat.ActivityInfo{Display: "12m"},
				Note:         ovwformat.NoteInfo{Display: "short note", Source: "user", Value: "short note"},
			},
		},
	}

	view := stripANSI(model.View())
	if strings.Contains(view, "pgup/pgdn") {
		t.Fatalf("short sidepane should not show scroll hint:\n%s", view)
	}
	model = updateSpecialKey(t, model, tea.KeyPgDown)
	if model.detailYOffset != 0 {
		t.Fatalf("detailYOffset = %d, want 0", model.detailYOffset)
	}
}

func TestModelScrollsLongDetailModal(t *testing.T) {
	detailProject := project.Project{
		Name:         "openclaw",
		Path:         "/tmp/openclaw",
		StackDisplay: "Node",
		Managers:     []string{"pnpm"},
		Scripts: []string{
			"android:assemble", "android:test", "build", "build:docker",
			"check", "check:docs", "dev", "docs:dev", "lint", "lint:docs",
			"test", "test:all", "test:docker:live-gateway", "test:docker:live-models",
			"test:perf:hotspots", "test:startup:memory", "ui:build", "ui:dev",
		},
		Activity: ovwformat.ActivityInfo{Display: "6h"},
		Status:   ovwformat.StatusFromTags("active", []string{"active"}),
	}
	model := Model{
		width:        140,
		height:       12,
		activeFilter: "all",
		activeSort:   "activity",
		screen:       screenDetail,
		config:       config.Default(),
		projects:     []project.Project{detailProject},
	}

	view := stripANSI(model.View())
	if !strings.Contains(view, "↓ pgup/pgdn") {
		t.Fatalf("long detail modal should show scroll hint:\n%s", view)
	}
	if strings.Contains(view, "x hide") {
		t.Fatalf("long detail modal should start clipped:\n%s", view)
	}

	model = updateSpecialKey(t, model, tea.KeyEnd)
	view = stripANSI(model.View())
	if !strings.Contains(view, "x hide") {
		t.Fatalf("detail modal should jump to bottom:\n%s", view)
	}
	if !strings.Contains(view, "↑ pgup/pgdn") {
		t.Fatalf("detail modal should show upward scroll hint:\n%s", view)
	}
}

func TestModelTogglesExpandedDetailModal(t *testing.T) {
	detailProject := detailTestProject("openclaw")
	detailProject.Scripts = []string{
		"android:assemble", "android:test", "build", "build:docker",
		"check", "check:docs", "dev", "docs:dev", "lint", "lint:docs",
		"test", "test:all", "test:docker:live-gateway", "test:docker:live-models",
		"test:perf:hotspots", "test:startup:memory", "ui:build", "ui:dev",
	}
	model := Model{
		width:        140,
		height:       24,
		activeFilter: "all",
		activeSort:   "activity",
		screen:       screenDetail,
		config:       config.Default(),
		projects:     []project.Project{detailProject},
		detailModalY: 3,
	}

	view := stripANSI(model.View())
	if !strings.Contains(view, "space expand") {
		t.Fatalf("compact detail modal missing expand hint:\n%s", view)
	}
	if strings.Contains(view, "ui:dev") {
		t.Fatalf("compact detail modal should hide later scripts:\n%s", view)
	}

	model = updateSpecialKey(t, model, tea.KeySpace)
	if !model.detailsExpanded {
		t.Fatal("detailsExpanded = false, want true")
	}
	if model.detailModalY != 0 {
		t.Fatalf("detailModalY = %d, want 0", model.detailModalY)
	}
	view = stripANSI(model.View())
	if !strings.Contains(view, "ui:dev") {
		t.Fatalf("expanded detail modal should show later scripts:\n%s", view)
	}
	model = updateSpecialKey(t, model, tea.KeyEnd)
	view = stripANSI(model.View())
	if !strings.Contains(view, "space collapse") {
		t.Fatalf("expanded detail modal missing collapse hint:\n%s", view)
	}

	model = updateSpecialKey(t, model, tea.KeySpace)
	if model.detailsExpanded {
		t.Fatal("detailsExpanded = true, want false")
	}
	view = stripANSI(model.View())
	if !strings.Contains(view, "space expand") || strings.Contains(view, "ui:dev") {
		t.Fatalf("collapsed detail modal should compact again:\n%s", view)
	}
}

func TestModelStoresLoadError(t *testing.T) {
	model := NewWithLoader(func(opts app.Options) (app.OverviewResult, error) {
		return app.OverviewResult{}, errors.New("boom")
	})

	cmd := model.Init()
	updated, _ := model.Update(cmd())
	got := updated.(Model)
	if got.loading {
		t.Fatal("model is still loading")
	}
	if got.loadErr == nil || got.loadErr.Error() != "boom" {
		t.Fatalf("loadErr = %v", got.loadErr)
	}
	if !strings.Contains(got.View(), "Failed to load projects: boom") {
		t.Fatalf("View() missing error state:\n%s", got.View())
	}
}

func TestModelMovesSelectionWithArrowAndVimKeys(t *testing.T) {
	model := Model{
		projects: []project.Project{
			{Name: "one"},
			{Name: "two"},
			{Name: "three"},
		},
	}

	model = updateKey(t, model, "j")
	if model.selected != 1 {
		t.Fatalf("selected after j = %d, want 1", model.selected)
	}
	model = updateSpecialKey(t, model, tea.KeyDown)
	if model.selected != 2 {
		t.Fatalf("selected after down = %d, want 2", model.selected)
	}
	model = updateKey(t, model, "j")
	if model.selected != 2 {
		t.Fatalf("selected after bottom j = %d, want 2", model.selected)
	}
	model = updateKey(t, model, "k")
	if model.selected != 1 {
		t.Fatalf("selected after k = %d, want 1", model.selected)
	}
	model = updateSpecialKey(t, model, tea.KeyUp)
	if model.selected != 0 {
		t.Fatalf("selected after up = %d, want 0", model.selected)
	}
	model = updateKey(t, model, "k")
	if model.selected != 0 {
		t.Fatalf("selected after top k = %d, want 0", model.selected)
	}
}

func TestModelNavigationHandlesEmptyProjects(t *testing.T) {
	model := updateKey(t, Model{}, "j")
	if model.selected != 0 {
		t.Fatalf("selected = %d, want 0", model.selected)
	}
}

func TestModelOpensAndClosesDetailView(t *testing.T) {
	updated := updateSpecialKey(t, Model{
		width:  140,
		height: 24,
		projects: []project.Project{
			detailTestProject("one"),
			detailTestProject("two"),
		},
		selected: 1,
	}, tea.KeyEnter)

	if updated.screen != screenDetail {
		t.Fatalf("screen = %v, want detail", updated.screen)
	}
	if updated.selected != 1 {
		t.Fatalf("selected = %d, want 1", updated.selected)
	}
	view := updated.View()
	for _, want := range []string{"one", "two", " │ ", "/tmp/two", "Go, Cobra", "go modules", "1.2.3", "dirty", "2 unpushed", "Value note"} {
		if !strings.Contains(view, want) {
			t.Fatalf("detail view missing %q:\n%s", want, view)
		}
	}
	if strings.Contains(view, "Details") {
		t.Fatalf("detail view should use project name as modal title:\n%s", view)
	}
	if strings.Contains(view, "NoteSource") {
		t.Fatalf("detail view contains noisy internal field:\n%s", view)
	}
	if !strings.Contains(stripANSI(view), "x hide") {
		t.Fatalf("detail view missing hide action:\n%s", view)
	}

	updated = updateSpecialKey(t, updated, tea.KeyEsc)
	if updated.screen != screenTable {
		t.Fatalf("screen = %v, want table", updated.screen)
	}
	if updated.selected != 1 {
		t.Fatalf("selected after esc = %d, want 1", updated.selected)
	}
}

func TestModelDetailVisibilityKeyHidesProject(t *testing.T) {
	var hiddenPath string
	var hiddenValue bool
	model := Model{
		loader: func(opts app.Options) (app.OverviewResult, error) {
			return app.OverviewResult{Config: config.Default(), Projects: []project.Project{{Name: "other", Path: "/tmp/other"}}}, nil
		},
		visible: func(path string, hidden bool) (app.MetadataUpdateResult, error) {
			hiddenPath = path
			hiddenValue = hidden
			return app.MetadataUpdateResult{Path: path}, nil
		},
		projects: []project.Project{{Name: "app", Path: "/tmp/app"}},
		screen:   screenDetail,
	}

	updated, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")})
	model = updated.(Model)
	if cmd == nil {
		t.Fatal("expected hide command")
	}
	model = updateMsg(t, model, cmd())
	if hiddenPath != "/tmp/app" || !hiddenValue {
		t.Fatalf("hidden update = %s %v, want /tmp/app true", hiddenPath, hiddenValue)
	}
	if model.screen != screenTable {
		t.Fatalf("screen = %v, want table", model.screen)
	}
	if model.message != "Project hidden" {
		t.Fatalf("message = %q, want Project hidden", model.message)
	}
	if len(model.projects) != 1 || model.projects[0].Name != "other" {
		t.Fatalf("projects = %#v", model.projects)
	}
}

func TestModelDetailEnterClosesWithoutTogglingVisibility(t *testing.T) {
	called := false
	model := Model{
		visible: func(path string, hidden bool) (app.MetadataUpdateResult, error) {
			called = true
			return app.MetadataUpdateResult{Path: path}, nil
		},
		projects: []project.Project{{Name: "app", Path: "/tmp/app"}},
		screen:   screenDetail,
	}

	updated, cmd := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(Model)
	if cmd != nil {
		t.Fatal("enter should not return a visibility command")
	}
	if called {
		t.Fatal("enter should not toggle visibility")
	}
	if model.screen != screenTable {
		t.Fatalf("screen = %v, want table", model.screen)
	}
}

func TestModelDetailVisibilityKeyUnhidesProject(t *testing.T) {
	var hiddenValue bool
	model := Model{
		loader: func(opts app.Options) (app.OverviewResult, error) {
			return app.OverviewResult{Config: config.Default(), Projects: []project.Project{{Name: "app", Path: "/tmp/app"}}}, nil
		},
		visible: func(path string, hidden bool) (app.MetadataUpdateResult, error) {
			hiddenValue = hidden
			return app.MetadataUpdateResult{Path: path}, nil
		},
		projects: []project.Project{{Name: "app", Path: "/tmp/app", Hidden: true}},
		screen:   screenDetail,
	}

	updated, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")})
	model = updated.(Model)
	if cmd == nil {
		t.Fatal("expected unhide command")
	}
	model = updateMsg(t, model, cmd())
	if hiddenValue {
		t.Fatal("hiddenValue = true, want false")
	}
	if model.message != "Project unhidden" {
		t.Fatalf("message = %q, want Project unhidden", model.message)
	}
}

func TestModelDoesNotOpenDetailWithoutProjects(t *testing.T) {
	updated := updateSpecialKey(t, Model{}, tea.KeyEnter)
	if updated.screen != screenTable {
		t.Fatalf("screen = %v, want table", updated.screen)
	}
}

func TestModelSearchFiltersVisibleProjects(t *testing.T) {
	model := Model{
		projects: []project.Project{
			{Name: "api", Path: "/tmp/api", StackDisplay: "Go"},
			{Name: "web", Path: "/tmp/web", StackDisplay: "SvelteKit", Note: ovwformat.NoteInfo{Display: "frontend"}},
		},
	}

	model = updateKey(t, model, "/")
	if !model.searching {
		t.Fatal("expected search mode")
	}
	model = updateKey(t, model, "w")
	model = updateKey(t, model, "e")
	model = updateKey(t, model, "b")
	if model.search != "web" {
		t.Fatalf("search = %q, want web", model.search)
	}
	visible := model.visibleProjects()
	if len(visible) != 1 || visible[0].Name != "web" {
		t.Fatalf("visible projects = %#v", visible)
	}
	view := stripANSI(model.View())
	if !strings.Contains(view, "search: web▌") || !strings.Contains(view, "web") || strings.Contains(view, "api") {
		t.Fatalf("search view = %s", view)
	}
}

func TestModelCommandPaletteOpensFromColonAndCtrlP(t *testing.T) {
	base := Model{
		width:    100,
		height:   24,
		projects: []project.Project{{Name: "app", Path: "/tmp/app"}},
		config:   config.Default(),
	}

	model := updateKey(t, base, ":")
	if model.screen != screenCommand {
		t.Fatalf("screen = %v, want command", model.screen)
	}
	view := stripANSI(model.View())
	for _, want := range []string{"Commands", "type a command", "Search projects", "Open in editor", "Set status", "Filter projects", "Choose columns", "Reload projects"} {
		if !strings.Contains(view, want) {
			t.Fatalf("command palette missing %q:\n%s", want, view)
		}
	}
	for _, want := range []string{"/", "o", "m", "f", "c", "r", "q"} {
		if !strings.Contains(view, want) {
			t.Fatalf("command palette missing shortcut %q:\n%s", want, view)
		}
	}

	model = updateSpecialKey(t, base, tea.KeyCtrlP)
	if model.screen != screenCommand {
		t.Fatalf("screen = %v, want command from ctrl+p", model.screen)
	}
}

func TestModelCommandPaletteFiltersAndRunsAction(t *testing.T) {
	model := Model{
		width:    100,
		height:   24,
		projects: []project.Project{{Name: "app", Path: "/tmp/app"}},
		config:   config.Default(),
	}

	model = updateKey(t, model, ":")
	for _, value := range []string{"f", "i", "l", "t"} {
		model = updateKey(t, model, value)
	}
	view := stripANSI(model.View())
	actions := model.filteredCommandActions()
	if len(actions) != 1 || actions[0].Label != "Filter projects" || !strings.Contains(view, "Filter projects") {
		t.Fatalf("command palette should filter commands:\n%s", view)
	}

	updated, cmd := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(Model)
	if cmd != nil {
		t.Fatalf("filter command should not return async command")
	}
	if model.screen != screenFilter {
		t.Fatalf("screen = %v, want filter", model.screen)
	}
}

func TestModelCommandPaletteShowsConditionalPinLabel(t *testing.T) {
	model := Model{
		projects: []project.Project{{Name: "app", Path: "/tmp/app", Pinned: true}},
		config:   config.Default(),
	}

	actions := model.commandActions()
	if !hasCommandLabel(actions, "Unpin project") || hasCommandLabel(actions, "Pin project") {
		t.Fatalf("pinned command labels = %#v", commandLabels(actions))
	}
}

func TestCommandOptionLineRightAlignsShortcut(t *testing.T) {
	line := commandOptionLine(commandAction{Label: "Open in editor", Shortcut: "o"}, 30)
	if !strings.HasPrefix(line, "Open in editor") || !strings.HasSuffix(line, "o") {
		t.Fatalf("command option line = %q", line)
	}
	if lipglossWidth(line) != 30 {
		t.Fatalf("command option line width = %d, want 30: %q", lipglossWidth(line), line)
	}
}

func TestModelCommandPaletteSupportsEditingAndEscape(t *testing.T) {
	model := Model{
		width:    100,
		height:   24,
		projects: []project.Project{{Name: "app", Path: "/tmp/app"}},
		config:   config.Default(),
	}

	model = updateKey(t, model, ":")
	for _, value := range []string{"s", "o", "r"} {
		model = updateKey(t, model, value)
	}
	model = updateSpecialKey(t, model, tea.KeyLeft)
	model = updateKey(t, model, "t")
	if model.commandInput != "sotr" {
		t.Fatalf("commandInput = %q, want sotr", model.commandInput)
	}
	model = updateSpecialKey(t, model, tea.KeyBackspace)
	if model.commandInput != "sor" {
		t.Fatalf("commandInput after backspace = %q, want sor", model.commandInput)
	}
	model = updateSpecialKey(t, model, tea.KeyEsc)
	if model.screen != screenTable {
		t.Fatalf("screen = %v, want table", model.screen)
	}
}

func TestModelSearchEmptyShowsCursor(t *testing.T) {
	model := Model{
		projects:  []project.Project{{Name: "api"}},
		searching: true,
	}

	view := stripANSI(model.View())
	if !strings.Contains(view, "search: ▌") {
		t.Fatalf("search view missing cursor:\n%s", view)
	}
}

func TestModelSearchCursorBlinks(t *testing.T) {
	model := Model{
		projects:  []project.Project{{Name: "api"}},
		searching: true,
	}

	view := model.View()
	if !strings.Contains(view, "\x1b[5m▌\x1b[25m") {
		t.Fatalf("search cursor should use ANSI blink:\n%q", view)
	}
}

func TestModelIgnoresStaleCursorBlink(t *testing.T) {
	model := Model{
		projects:      []project.Project{{Name: "api"}},
		searching:     true,
		cursorBlinkID: 2,
	}

	updated, cmd := model.Update(inputCursorBlinkMsg{id: 1})
	got := updated.(Model)
	if got.cursorHidden {
		t.Fatal("stale cursor blink should not toggle cursor state")
	}
	if cmd != nil {
		t.Fatal("stale cursor blink should not schedule another blink")
	}
}

func TestModelIgnoresCursorBlinkWhenInputInactive(t *testing.T) {
	model := Model{
		projects:      []project.Project{{Name: "api"}},
		cursorBlinkID: 1,
	}

	updated, cmd := model.Update(inputCursorBlinkMsg{id: 1})
	got := updated.(Model)
	if got.cursorHidden {
		t.Fatal("inactive cursor blink should not toggle cursor state")
	}
	if cmd != nil {
		t.Fatal("inactive cursor blink should not schedule another blink")
	}
}

func TestModelSearchEscBlursThenClearsSearch(t *testing.T) {
	model := Model{
		projects: []project.Project{
			{Name: "api"},
			{Name: "web"},
		},
		searching: true,
		search:    "web",
	}

	model = updateSpecialKey(t, model, tea.KeyEsc)
	if model.searching {
		t.Fatal("search focus still active")
	}
	if model.search != "web" {
		t.Fatalf("search = %q, want web", model.search)
	}
	if len(model.visibleProjects()) != 1 {
		t.Fatalf("visible projects after blur = %#v", model.visibleProjects())
	}

	model = updateSpecialKey(t, model, tea.KeyEsc)
	if model.searching {
		t.Fatal("search focus active after clear")
	}
	if model.search != "" {
		t.Fatalf("search = %q, want empty", model.search)
	}
	if len(model.visibleProjects()) != 2 {
		t.Fatalf("visible projects = %#v", model.visibleProjects())
	}
}

func TestModelSearchAllowsSelectionAndDetails(t *testing.T) {
	model := Model{
		projects: []project.Project{
			{Name: "web-api", Path: "/tmp/web-api"},
			{Name: "web-app", Path: "/tmp/web-app"},
			{Name: "docs", Path: "/tmp/docs"},
		},
	}

	model = updateKey(t, model, "/")
	model = updateKey(t, model, "w")
	model = updateKey(t, model, "e")
	model = updateKey(t, model, "b")
	if !model.searching {
		t.Fatal("expected search focus")
	}
	model = updateSpecialKey(t, model, tea.KeyDown)
	if !model.searching {
		t.Fatal("search focus should stay active after moving")
	}
	if model.selected != 1 {
		t.Fatalf("selected = %d, want 1", model.selected)
	}

	model = updateSpecialKey(t, model, tea.KeyEnter)
	if model.screen != screenDetail {
		t.Fatalf("screen = %v, want detail", model.screen)
	}
	if model.search != "web" {
		t.Fatalf("search = %q, want web", model.search)
	}
}

func TestModelSearchKeepsLetterKeysInSearchInput(t *testing.T) {
	model := Model{
		projects: []project.Project{
			{Name: "web", Path: "/tmp/web", Note: ovwformat.NoteInfo{Value: "note"}},
		},
		searching:    true,
		search:       "web",
		searchCursor: 3,
	}

	model = updateKey(t, model, "n")
	if model.screen != screenTable {
		t.Fatalf("screen = %v, want table", model.screen)
	}
	if model.search != "webn" {
		t.Fatalf("search = %q, want webn", model.search)
	}
}

func TestModelSearchSupportsCursorEditing(t *testing.T) {
	model := Model{
		projects: []project.Project{
			{Name: "web-api", Path: "/tmp/web-api"},
			{Name: "web-app", Path: "/tmp/web-app"},
		},
		searching:    true,
		search:       "web",
		searchCursor: 3,
	}

	model = updateSpecialKey(t, model, tea.KeyLeft)
	model = updateKey(t, model, "-")
	if model.search != "we-b" {
		t.Fatalf("search = %q, want we-b", model.search)
	}
	if !strings.Contains(stripANSI(model.View()), "search: we-▌b") {
		t.Fatalf("search cursor not rendered in place:\n%s", stripANSI(model.View()))
	}

	model = updateSpecialKey(t, model, tea.KeyBackspace)
	if model.search != "web" {
		t.Fatalf("search after backspace = %q, want web", model.search)
	}
	model = updateSpecialKey(t, model, tea.KeyDelete)
	if model.search != "we" {
		t.Fatalf("search after delete = %q, want we", model.search)
	}
}

func TestModelSearchSupportsReadlineEditing(t *testing.T) {
	model := Model{
		projects:     []project.Project{{Name: "web-api", Path: "/tmp/web-api"}},
		searching:    true,
		search:       "web api",
		searchCursor: len([]rune("web api")),
	}

	model = updateSpecialKey(t, model, tea.KeyCtrlA)
	if model.searchCursor != 0 {
		t.Fatalf("searchCursor after ctrl+a = %d, want 0", model.searchCursor)
	}
	model = updateSpecialKey(t, model, tea.KeyCtrlE)
	if model.searchCursor != len([]rune(model.search)) {
		t.Fatalf("searchCursor after ctrl+e = %d, want end", model.searchCursor)
	}
	model = updateSpecialKey(t, model, tea.KeyCtrlW)
	if model.search != "web " || model.searchCursor != len([]rune("web ")) {
		t.Fatalf("search after ctrl+w = %q cursor %d, want web and cursor after space", model.search, model.searchCursor)
	}
	model = updateSpecialKey(t, model, tea.KeyCtrlU)
	if model.search != "" || model.searchCursor != 0 {
		t.Fatalf("search after ctrl+u = %q cursor %d, want empty", model.search, model.searchCursor)
	}
}

func TestModelSearchBlurAllowsActions(t *testing.T) {
	model := Model{
		projects: []project.Project{
			{Name: "web", Path: "/tmp/web", Note: ovwformat.NoteInfo{Value: "note"}},
		},
		searching:    true,
		search:       "web",
		searchCursor: 3,
	}

	model = updateSpecialKey(t, model, tea.KeyEsc)
	model = updateKey(t, model, "n")
	if model.screen != screenNote {
		t.Fatalf("screen = %v, want note", model.screen)
	}
	if model.noteInput != "note" {
		t.Fatalf("noteInput = %q, want note", model.noteInput)
	}
}

func TestModelSearchNoMatchesState(t *testing.T) {
	model := Model{
		width:    100,
		height:   24,
		projects: []project.Project{{Name: "api"}},
		search:   "zzz",
	}

	raw := model.View()
	view := stripANSI(raw)
	for _, want := range []string{"No projects match search", "esc", "clear search"} {
		if !strings.Contains(view, want) {
			t.Fatalf("View() missing %q:\n%s", want, view)
		}
	}
	if strings.Contains(raw, "48;5;236") {
		t.Fatalf("search empty state should not use modal background styling:\n%q", raw)
	}
}

func TestModelFilterPickerAppliesDirtyFilter(t *testing.T) {
	calledLoader := false
	model := Model{
		width:  80,
		height: 24,
		loader: func(opts app.Options) (app.OverviewResult, error) {
			calledLoader = true
			return app.OverviewResult{
				Config:   config.Default(),
				Projects: []project.Project{{Name: "dirty-project"}},
			}, nil
		},
		config: config.Default(),
		projects: []project.Project{
			{Name: "clean-project"},
			{Name: "dirty-project", Activity: ovwformat.ActivityInfo{Dirty: true}},
		},
	}

	model = updateKey(t, model, "f")
	if model.screen != screenFilter {
		t.Fatalf("screen = %v, want filter", model.screen)
	}
	if !strings.Contains(model.View(), "dirty") {
		t.Fatalf("filter view missing dirty option:\n%s", model.View())
	}
	for _, want := range []string{"Name", "Filter", "dirty-project", "esc"} {
		if !strings.Contains(model.View(), want) {
			t.Fatalf("filter modal view missing %q:\n%s", want, model.View())
		}
	}
	model = updateKey(t, model, "j")
	updated, cmd := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(Model)
	if cmd != nil {
		model = updateMsg(t, model, cmd())
	}
	if calledLoader {
		t.Fatal("filter should not reload overview")
	}
	if model.activeFilter != "dirty" {
		t.Fatalf("activeFilter = %q, want dirty", model.activeFilter)
	}
	visible := model.visibleProjects()
	if len(visible) != 1 || visible[0].Name != "dirty-project" {
		t.Fatalf("visible projects = %#v, want only dirty-project", visible)
	}
	if model.loading {
		t.Fatal("model is still loading")
	}
}

func TestModelFilterPickerIncludesConfiguredStatuses(t *testing.T) {
	cfg := config.Default()
	cfg.Statuses = []string{"parked", "shipped"}
	model := Model{config: cfg}

	model = updateKey(t, model, "f")
	view := stripANSI(model.View())
	for _, want := range []string{"all", "dirty", "stale", "untagged", "hidden", "parked", "shipped"} {
		if !strings.Contains(view, want) {
			t.Fatalf("filter view missing %q:\n%s", want, view)
		}
	}
}

func TestHeaderProjectCountUsesVisibleProjects(t *testing.T) {
	model := Model{
		width:        120,
		height:       24,
		config:       config.Default(),
		activeFilter: "shipped",
		request:      app.Options{Status: "shipped"},
		projects: []project.Project{
			{Name: "active", Status: ovwformat.StatusInfo{Value: "active", Display: "active"}},
			{Name: "parked", Status: ovwformat.StatusInfo{Value: "parked", Display: "parked"}},
		},
	}

	header := strings.Split(stripANSI(model.View()), "\n")[0]
	if !strings.Contains(header, "0 projects") {
		t.Fatalf("header should count visible projects:\n%s", header)
	}
	if strings.Contains(header, "2 projects") {
		t.Fatalf("header should not count all projects while filtered:\n%s", header)
	}
}

func TestModelColumnPickerTogglesAndSavesColumns(t *testing.T) {
	cfg := config.Default()
	var saved config.Config
	model := Model{
		width:    140,
		height:   24,
		config:   cfg,
		projects: []project.Project{{Name: "app", Path: "/tmp/app"}},
		configWriter: func(path string, cfg config.Config) error {
			saved = cfg
			return nil
		},
	}

	model = updateKey(t, model, "c")
	if model.screen != screenColumns {
		t.Fatalf("screen = %v, want columns", model.screen)
	}
	view := stripANSI(model.View())
	for _, want := range []string{"Columns", "[x] name", "[ ] path", "[ ] branch", "[ ] updated", "space toggle", "←→ reorder", "enter save"} {
		if !strings.Contains(view, want) {
			t.Fatalf("columns modal missing %q:\n%s", want, view)
		}
	}

	for !strings.Contains(stripANSI(model.View()), "> [ ] path") {
		model = updateKey(t, model, "j")
	}
	model = updateKey(t, model, " ")
	updated, cmd := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(Model)
	if cmd == nil {
		t.Fatal("expected columns save command")
	}
	model = updateMsg(t, model, cmd())

	want := []string{"name", "stack", "activity", "status", "note", "path"}
	if !reflect.DeepEqual(saved.Columns, want) {
		t.Fatalf("saved columns = %#v, want %#v", saved.Columns, want)
	}
	if !reflect.DeepEqual(model.config.Columns, want) {
		t.Fatalf("model columns = %#v, want %#v", model.config.Columns, want)
	}
	if model.message != "Columns saved" {
		t.Fatalf("message = %q, want Columns saved", model.message)
	}
}

func TestModelColumnPickerDetectsPortsWhenEnabled(t *testing.T) {
	cfg := config.Default()
	cfg.Columns = []string{"name", "ports"}
	appPath := "/tmp/app"
	var detectedPaths []string
	model := Model{
		config:   config.Default(),
		projects: []project.Project{{Name: "app", Path: appPath}},
		portDetector: func(paths []string) map[string][]int {
			detectedPaths = append([]string{}, paths...)
			return map[string][]int{appPath: []int{5173}}
		},
	}

	updated, cmd := model.Update(columnsSavedMsg{config: cfg})
	model = updated.(Model)
	if cmd == nil {
		t.Fatal("expected port detection command")
	}
	model = updateMsg(t, model, cmd())

	if !reflect.DeepEqual(detectedPaths, []string{appPath}) {
		t.Fatalf("detected paths = %#v, want app path", detectedPaths)
	}
	if !reflect.DeepEqual(model.projects[0].Ports, []int{5173}) {
		t.Fatalf("ports = %#v, want 5173", model.projects[0].Ports)
	}
}

func TestStartEnrichmentDetectsPortsWhenColumnVisible(t *testing.T) {
	cfg := config.Default()
	cfg.Columns = []string{"name", "ports"}
	appPath := "/tmp/app"
	var detectedPaths []string
	updates := startEnrichment([]scanner.Project{{Name: "app", Path: appPath}}, cfg, func(paths []string) map[string][]int {
		detectedPaths = append([]string{}, paths...)
		return map[string][]int{appPath: []int{3000}}
	})

	update := <-updates
	if !reflect.DeepEqual(detectedPaths, []string{appPath}) {
		t.Fatalf("detected paths = %#v, want app path", detectedPaths)
	}
	if !reflect.DeepEqual(update.project.Ports, []int{3000}) {
		t.Fatalf("ports = %#v, want 3000", update.project.Ports)
	}
	if _, ok := <-updates; ok {
		t.Fatal("expected enrichment channel to close")
	}
}

func TestModelColumnPickerReordersTableColumns(t *testing.T) {
	cfg := config.Default()
	cfg.Columns = []string{"name", "stack"}
	model := Model{
		width:    100,
		height:   20,
		config:   cfg,
		projects: []project.Project{{Name: "app", StackDisplay: "Go"}},
		configWriter: func(path string, cfg config.Config) error {
			return nil
		},
	}

	model = updateKey(t, model, "c")
	model = updateKey(t, model, "j")
	model = updateSpecialKey(t, model, tea.KeyLeft)
	updated, cmd := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(Model)
	model = updateMsg(t, model, cmd())

	header := strings.Split(stripANSI(model.View()), "\n")[2]
	if !strings.HasPrefix(header, "Stack  Name") {
		t.Fatalf("header = %q, want Stack before Name\n%s", header, stripANSI(model.View()))
	}
}

func TestModelColumnPickerKeepsOneVisibleColumn(t *testing.T) {
	cfg := config.Default()
	cfg.Columns = []string{"name"}
	model := Model{
		config:   cfg,
		projects: []project.Project{{Name: "app"}},
	}

	model = updateKey(t, model, "c")
	model = updateKey(t, model, " ")

	if !reflect.DeepEqual(model.config.Columns, []string{"name"}) {
		t.Fatalf("columns = %#v, want name still visible", model.config.Columns)
	}
	if !strings.Contains(stripANSI(model.View()), "keep at least one column") {
		t.Fatalf("columns modal missing guard message:\n%s", stripANSI(model.View()))
	}
}

func TestModalPickersUseCaretSelection(t *testing.T) {
	cases := []struct {
		name string
		view string
		want string
	}{
		{name: "filter", view: filterView([]filterOption{{Label: "all"}, {Label: "dirty"}}, 1), want: "> dirty"},
		{name: "sort", view: sortView([]sortOption{{Label: "activity"}, {Label: "name"}}, 1, "desc"), want: "> name   desc"},
		{name: "status", view: statusView("", []statusOption{{Label: "parked"}, {Label: "shipped"}}, 1), want: "> shipped"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := stripANSI(tc.view)
			if !strings.Contains(got, tc.want) {
				t.Fatalf("picker missing caret selection %q:\n%s", tc.want, got)
			}
			if strings.Contains(got, "\x1b[48;5;"+selectionBackgroundColor+"m") {
				t.Fatalf("picker should not use selected background:\n%q", tc.view)
			}
			if !strings.Contains(tc.view, "\x1b[38;5;"+accentColor+";48;5;"+modalSurfaceColor+"m> "+strings.TrimPrefix(tc.want, "> ")) {
				t.Fatalf("picker selected text should be accented:\n%q", tc.view)
			}
			if !strings.Contains(tc.view, "\x1b[38;5;"+mutedColor+";48;5;"+modalSurfaceColor+"m") {
				t.Fatalf("picker unselected text should be muted:\n%q", tc.view)
			}
		})
	}
}

func TestWrapPickerSelection(t *testing.T) {
	cases := []struct {
		name     string
		selected int
		total    int
		delta    int
		want     int
	}{
		{name: "down wraps at end", selected: 2, total: 3, delta: 1, want: 0},
		{name: "up wraps at start", selected: 0, total: 3, delta: -1, want: 2},
		{name: "moves inside list", selected: 1, total: 3, delta: 1, want: 2},
		{name: "empty list", selected: 1, total: 0, delta: 1, want: 0},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := wrapPickerSelection(tc.selected, tc.total, tc.delta); got != tc.want {
				t.Fatalf("wrapPickerSelection() = %d, want %d", got, tc.want)
			}
		})
	}
}

func TestModelPickerNavigationWraps(t *testing.T) {
	cfg := config.Default()

	filterModel := Model{screen: screenFilter, config: cfg}
	filterModel.filterSelected = len(filterModel.filterOptions()) - 1
	filterModel = updateKey(t, filterModel, "j")
	if filterModel.filterSelected != 0 {
		t.Fatalf("filter selected = %d, want wrapped to 0", filterModel.filterSelected)
	}
	filterModel = updateKey(t, filterModel, "k")
	if filterModel.filterSelected != len(filterModel.filterOptions())-1 {
		t.Fatalf("filter selected = %d, want wrapped to end", filterModel.filterSelected)
	}

	sortModel := Model{screen: screenSort}
	sortModel.sortSelected = len(sortOptions()) - 1
	sortModel = updateKey(t, sortModel, "j")
	if sortModel.sortSelected != 0 {
		t.Fatalf("sort selected = %d, want wrapped to 0", sortModel.sortSelected)
	}

	columnModel := Model{screen: screenColumns, columnOrder: []string{"name", "stack"}}
	columnModel.columnSelected = len(columnModel.columnOrder) - 1
	columnModel = updateKey(t, columnModel, "j")
	if columnModel.columnSelected != 0 {
		t.Fatalf("column selected = %d, want wrapped to 0", columnModel.columnSelected)
	}

	statusModel := Model{screen: screenStatus, config: cfg, projects: []project.Project{{Name: "app"}}}
	statusModel.statusSelected = len(statusModel.statusOptions()) - 1
	statusModel = updateKey(t, statusModel, "j")
	if statusModel.statusSelected != 0 {
		t.Fatalf("status selected = %d, want wrapped to 0", statusModel.statusSelected)
	}

	commandModel := Model{}
	commandModel.openCommandPalette()
	commandModel.commandSelected = len(commandModel.filteredCommandActions()) - 1
	commandModel = updateKey(t, commandModel, "j")
	if commandModel.commandSelected != 0 {
		t.Fatalf("command selected = %d, want wrapped to 0", commandModel.commandSelected)
	}

	onboardingModel := Model{screen: screenOnboarding, onboardOptions: []string{"/tmp/a", "/tmp/b"}}
	onboardingModel.onboardSelected = len(onboardingModel.onboardOptions)
	onboardingModel = updateKey(t, onboardingModel, "j")
	if onboardingModel.onboardSelected != 0 {
		t.Fatalf("onboarding selected = %d, want wrapped to 0", onboardingModel.onboardSelected)
	}
	onboardingModel = updateKey(t, onboardingModel, "k")
	if onboardingModel.onboardSelected != len(onboardingModel.onboardOptions) {
		t.Fatalf("onboarding selected = %d, want wrapped to custom path", onboardingModel.onboardSelected)
	}

	setupPicker := setupModel{
		options:  []string{"/tmp/a", "/tmp/b"},
		checked:  map[string]bool{},
		expanded: map[string]bool{},
		children: map[string][]string{},
		counts:   map[string]int{},
		selected: 2,
	}
	updated, _ := setupPicker.updatePicker(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	setupPicker = updated.(setupModel)
	if setupPicker.selected != 0 {
		t.Fatalf("setup selected = %d, want wrapped to 0", setupPicker.selected)
	}
}

func TestModelSortPickerAppliesNameSort(t *testing.T) {
	calledLoader := false
	model := Model{
		width:  80,
		height: 24,
		loader: func(opts app.Options) (app.OverviewResult, error) {
			calledLoader = true
			return app.OverviewResult{
				Config:   config.Default(),
				Projects: []project.Project{{Name: "a"}, {Name: "b"}},
			}, nil
		},
		activeSort:    "activity",
		activeSortDir: "desc",
		config:        config.Default(),
		projects:      []project.Project{{Name: "b"}, {Name: "a"}},
	}

	model = updateKey(t, model, "s")
	if model.screen != screenSort {
		t.Fatalf("screen = %v, want sort", model.screen)
	}
	if !strings.Contains(model.View(), "name") {
		t.Fatalf("sort view missing name option:\n%s", model.View())
	}
	view := stripANSI(model.View())
	for _, want := range []string{"Name", "Sort", "> activity   desc", "updated", "name", "enter apply", "<-> direction", "esc"} {
		if !strings.Contains(view, want) {
			t.Fatalf("sort modal view missing %q:\n%s", want, view)
		}
	}
	model = updateKey(t, model, "j")
	model = updateKey(t, model, "j")
	if !strings.Contains(stripANSI(model.View()), "> name   desc") {
		t.Fatalf("selected sort row should show direction:\n%s", stripANSI(model.View()))
	}
	model = updateSpecialKey(t, model, tea.KeyLeft)
	if !strings.Contains(stripANSI(model.View()), "> name   asc") {
		t.Fatalf("selected sort row should toggle direction:\n%s", stripANSI(model.View()))
	}
	updated, cmd := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(Model)
	if cmd != nil {
		model = updateMsg(t, model, cmd())
	}
	if calledLoader {
		t.Fatal("sort should not reload overview")
	}
	if model.activeSort != "name" {
		t.Fatalf("activeSort = %q, want name", model.activeSort)
	}
	if model.activeSortDir != "asc" {
		t.Fatalf("activeSortDir = %q, want asc", model.activeSortDir)
	}
	if model.request.Sort != "name:asc" {
		t.Fatalf("request.Sort = %q, want name:asc", model.request.Sort)
	}
	if got := []string{model.projects[0].Name, model.projects[1].Name}; !reflect.DeepEqual(got, []string{"a", "b"}) {
		t.Fatalf("projects order = %#v, want a,b", got)
	}
	if model.loading {
		t.Fatal("model is still loading")
	}
}

func TestSortPickerEscCloses(t *testing.T) {
	model := Model{screen: screenSort}
	model = updateSpecialKey(t, model, tea.KeyEsc)
	if model.screen != screenTable {
		t.Fatalf("screen = %v, want table", model.screen)
	}
}

func TestModelNoteEditorSavesAndReloads(t *testing.T) {
	var target string
	var savedNote string
	model := Model{
		width:  140,
		height: 24,
		loader: func(opts app.Options) (app.OverviewResult, error) {
			return app.OverviewResult{
				Config: config.Default(),
				Projects: []project.Project{
					{
						Name: "app",
						Path: "/tmp/app",
						Note: ovwformat.NoteInfo{Display: savedNote, Value: savedNote},
					},
				},
			}, nil
		},
		updater: func(path string, update app.MetadataUpdate) (app.MetadataUpdateResult, error) {
			target = path
			if update.Note == nil {
				t.Fatal("note update was nil")
			}
			savedNote = *update.Note
			return app.MetadataUpdateResult{Path: path}, nil
		},
		projects: []project.Project{
			{
				Name: "app",
				Path: "/tmp/app",
				Note: ovwformat.NoteInfo{Display: "old", Value: "old"},
			},
		},
	}

	model = updateKey(t, model, "n")
	if model.screen != screenNote {
		t.Fatalf("screen = %v, want note", model.screen)
	}
	if model.noteInput != "old" {
		t.Fatalf("noteInput = %q, want old", model.noteInput)
	}
	view := stripANSI(model.View())
	for _, want := range []string{"Name", "1/1", "app", "Note · app", "old▌", "enter", "save", "esc"} {
		if !strings.Contains(view, want) {
			t.Fatalf("note modal view missing %q:\n%s", want, view)
		}
	}
	model = updateKey(t, model, "!")
	updated, cmd := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(Model)
	if cmd == nil {
		t.Fatal("expected note save command")
	}
	model = updateMsg(t, model, cmd())
	if target != "/tmp/app" {
		t.Fatalf("target = %q, want /tmp/app", target)
	}
	if savedNote != "old!" {
		t.Fatalf("savedNote = %q, want old!", savedNote)
	}
	if model.message != "Note saved" {
		t.Fatalf("message = %q, want Note saved", model.message)
	}
	if model.projects[0].Note.Value != "old!" {
		t.Fatalf("project note = %#v", model.projects[0].Note)
	}
}

func TestModelNoteEditorAcceptsSpaces(t *testing.T) {
	model := Model{screen: screenNote}

	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeySpace})
	model = updated.(Model)

	if model.noteInput != " " {
		t.Fatalf("noteInput = %q, want space", model.noteInput)
	}
}

func TestModelNoteEditorSupportsCursorEditing(t *testing.T) {
	model := Model{
		screen:     screenNote,
		noteInput:  "old",
		noteCursor: 3,
	}

	model = updateSpecialKey(t, model, tea.KeyLeft)
	model = updateKey(t, model, "e")
	if model.noteInput != "oled" {
		t.Fatalf("noteInput = %q, want oled", model.noteInput)
	}
	if !strings.Contains(stripANSI(model.View()), "ole▌d") {
		t.Fatalf("note cursor not rendered in place:\n%s", stripANSI(model.View()))
	}

	model = updateSpecialKey(t, model, tea.KeyBackspace)
	if model.noteInput != "old" {
		t.Fatalf("noteInput after backspace = %q, want old", model.noteInput)
	}
	model = updateSpecialKey(t, model, tea.KeyDelete)
	if model.noteInput != "ol" {
		t.Fatalf("noteInput after delete = %q, want ol", model.noteInput)
	}
}

func TestModelNoteEditorSupportsReadlineEditing(t *testing.T) {
	model := Model{
		screen:     screenNote,
		noteInput:  "waiting for api",
		noteCursor: len([]rune("waiting for api")),
	}

	model = updateSpecialKey(t, model, tea.KeyCtrlW)
	if model.noteInput != "waiting for " {
		t.Fatalf("noteInput after ctrl+w = %q, want waiting for space", model.noteInput)
	}
	model = updateSpecialKey(t, model, tea.KeyCtrlA)
	model = updateSpecialKey(t, model, tea.KeyCtrlK)
	if model.noteInput != "" || model.noteCursor != 0 {
		t.Fatalf("noteInput after ctrl+a ctrl+k = %q cursor %d, want empty", model.noteInput, model.noteCursor)
	}
}

func TestModelNoteEditorEmptyClearsManualNote(t *testing.T) {
	var savedNote *string
	model := Model{
		loader: func(opts app.Options) (app.OverviewResult, error) {
			return app.OverviewResult{Config: config.Default()}, nil
		},
		updater: func(path string, update app.MetadataUpdate) (app.MetadataUpdateResult, error) {
			savedNote = update.Note
			return app.MetadataUpdateResult{Path: path}, nil
		},
		projects: []project.Project{{Name: "app", Path: "/tmp/app"}},
		screen:   screenNote,
	}

	view := stripANSI(model.View())
	for _, want := range []string{"empty clears note", "enter", "save"} {
		if !strings.Contains(view, want) {
			t.Fatalf("empty note modal missing %q:\n%s", want, view)
		}
	}
	updated, cmd := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(Model)
	if cmd == nil {
		t.Fatal("expected note clear command")
	}
	model = updateMsg(t, model, cmd())
	if savedNote == nil || *savedNote != "" {
		t.Fatalf("savedNote = %v, want empty string pointer", savedNote)
	}
}

func TestModelNoteEditorUsesCurrentDisplayNoteAsPlaceholder(t *testing.T) {
	model := Model{
		projects: []project.Project{{
			Name: "app",
			Path: "/tmp/app",
			Note: ovwformat.NoteInfo{Display: "commit fallback note"},
		}},
		screen: screenNote,
	}

	view := stripANSI(model.View())
	if !strings.Contains(view, "commit fallback note") {
		t.Fatalf("note modal should show current note as placeholder:\n%s", view)
	}
	if strings.Contains(view, "empty clears note") {
		t.Fatalf("note modal should not show clear hint when a current note exists:\n%s", view)
	}
}

func TestModelMetadataWriteErrorIsVisible(t *testing.T) {
	model := Model{
		loader: func(opts app.Options) (app.OverviewResult, error) {
			return app.OverviewResult{}, nil
		},
		updater: func(path string, update app.MetadataUpdate) (app.MetadataUpdateResult, error) {
			return app.MetadataUpdateResult{}, errors.New("disk full")
		},
		projects: []project.Project{{Name: "app", Path: "/tmp/app"}},
		screen:   screenNote,
	}

	updated, cmd := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(Model)
	if cmd == nil {
		t.Fatal("expected metadata command")
	}
	model = updateMsg(t, model, cmd())
	if !strings.Contains(model.message, "Failed to write metadata: disk full") {
		t.Fatalf("message = %q", model.message)
	}
}

func TestModelStatusPickerSavesConfiguredStatus(t *testing.T) {
	var savedStatus string
	cfg := config.Default()
	cfg.Statuses = []string{"parked", "shipped"}
	model := Model{
		width:  140,
		height: 24,
		config: cfg,
		loader: func(opts app.Options) (app.OverviewResult, error) {
			return app.OverviewResult{Config: cfg, Projects: []project.Project{{Name: "app", Path: "/tmp/app", Status: ovwformat.StatusFromTags(savedStatus, []string{savedStatus})}}}, nil
		},
		updater: func(path string, update app.MetadataUpdate) (app.MetadataUpdateResult, error) {
			if update.Status == nil {
				t.Fatal("status update was nil")
			}
			savedStatus = *update.Status
			return app.MetadataUpdateResult{Path: path}, nil
		},
		projects: []project.Project{{Name: "app", Path: "/tmp/app"}},
	}

	model = updateKey(t, model, "m")
	if model.screen != screenStatus {
		t.Fatalf("screen = %v, want status", model.screen)
	}
	view := stripANSI(model.View())
	for _, want := range []string{"Name", "1/1", "app", "Status · app", "parked", "shipped", "enter select", "esc"} {
		if !strings.Contains(view, want) {
			t.Fatalf("status modal view missing %q:\n%s", want, view)
		}
	}
	updated, cmd := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(Model)
	if cmd == nil {
		t.Fatal("expected status save command")
	}
	model = updateMsg(t, model, cmd())
	if savedStatus != "parked" {
		t.Fatalf("savedStatus = %q, want parked", savedStatus)
	}
	if model.message != "Status saved" {
		t.Fatalf("message = %q, want Status saved", model.message)
	}
}

func TestModelStatusPickerSupportsCustomInput(t *testing.T) {
	var savedStatus string
	model := Model{
		config: config.Default(),
		loader: func(opts app.Options) (app.OverviewResult, error) {
			return app.OverviewResult{Config: config.Default()}, nil
		},
		updater: func(path string, update app.MetadataUpdate) (app.MetadataUpdateResult, error) {
			savedStatus = *update.Status
			return app.MetadataUpdateResult{Path: path}, nil
		},
		projects:       []project.Project{{Name: "app", Path: "/tmp/app"}},
		screen:         screenStatus,
		statusSelected: len(config.Default().Statuses),
	}

	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(Model)
	if model.screen != screenStatusInput {
		t.Fatalf("screen = %v, want status input", model.screen)
	}
	view := stripANSI(model.View())
	if !strings.Contains(view, "Custom status · app") {
		t.Fatalf("custom status modal missing project name:\n%s", view)
	}
	if !strings.Contains(view, "empty clears status") {
		t.Fatalf("empty custom status modal missing cursor placeholder:\n%s", view)
	}
	for _, value := range []string{"b", "l", "o", "c", "k", "e", "d"} {
		model = updateKey(t, model, value)
	}
	view = stripANSI(model.View())
	if !strings.Contains(view, "blocked▌") {
		t.Fatalf("custom status modal missing cursor:\n%s", view)
	}
	updated, cmd := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(Model)
	if cmd == nil {
		t.Fatal("expected custom status save command")
	}
	model = updateMsg(t, model, cmd())
	if savedStatus != "blocked" {
		t.Fatalf("savedStatus = %q, want blocked", savedStatus)
	}
}

func TestModelStatusInputSupportsCursorEditing(t *testing.T) {
	model := Model{
		screen:       screenStatusInput,
		statusInput:  "todo",
		statusCursor: 4,
		projects:     []project.Project{{Name: "app", Path: "/tmp/app"}},
	}

	model = updateSpecialKey(t, model, tea.KeyLeft)
	model = updateKey(t, model, "-")
	if model.statusInput != "tod-o" {
		t.Fatalf("statusInput = %q, want tod-o", model.statusInput)
	}
	if !strings.Contains(stripANSI(model.View()), "tod-▌o") {
		t.Fatalf("status cursor not rendered in place:\n%s", stripANSI(model.View()))
	}

	model = updateSpecialKey(t, model, tea.KeyBackspace)
	if model.statusInput != "todo" {
		t.Fatalf("statusInput after backspace = %q, want todo", model.statusInput)
	}
	model = updateSpecialKey(t, model, tea.KeyDelete)
	if model.statusInput != "tod" {
		t.Fatalf("statusInput after delete = %q, want tod", model.statusInput)
	}
}

func TestModelStatusInputSupportsReadlineEditing(t *testing.T) {
	model := Model{
		screen:       screenStatusInput,
		statusInput:  "needs review",
		statusCursor: len([]rune("needs review")),
		projects:     []project.Project{{Name: "app", Path: "/tmp/app"}},
	}

	model = updateSpecialKey(t, model, tea.KeyCtrlA)
	model = updateSpecialKey(t, model, tea.KeyCtrlE)
	if model.statusCursor != len([]rune("needs review")) {
		t.Fatalf("statusCursor after ctrl+a ctrl+e = %d, want end", model.statusCursor)
	}
	model = updateSpecialKey(t, model, tea.KeyCtrlU)
	if model.statusInput != "" || model.statusCursor != 0 {
		t.Fatalf("statusInput after ctrl+u = %q cursor %d, want empty", model.statusInput, model.statusCursor)
	}
}

func TestModelStatusPickerPrefillsCurrentCustomStatus(t *testing.T) {
	cfg := config.Default()
	model := Model{
		config: cfg,
		projects: []project.Project{
			{Name: "app", Path: "/tmp/app", Status: ovwformat.StatusFromTags("needs review", []string{"needs review"})},
		},
		screen:         screenStatus,
		statusSelected: len(cfg.Statuses),
	}

	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(Model)
	if model.screen != screenStatusInput {
		t.Fatalf("screen = %v, want status input", model.screen)
	}
	if model.statusInput != "needs review" {
		t.Fatalf("statusInput = %q, want needs review", model.statusInput)
	}
	if !strings.Contains(stripANSI(model.View()), "needs review▌") {
		t.Fatalf("custom status input should show existing value:\n%s", stripANSI(model.View()))
	}
}

func TestModelStatusPickerSelectsCurrentStatus(t *testing.T) {
	cfg := config.Default()
	cfg.Statuses = []string{"active", "parked", "shipped"}
	model := Model{
		config: cfg,
		projects: []project.Project{
			{Name: "configured", Path: "/tmp/configured", Status: ovwformat.StatusFromTags("parked", []string{"parked"})},
			{Name: "custom", Path: "/tmp/custom", Status: ovwformat.StatusFromTags("blocked", []string{"blocked"})},
			{Name: "clear", Path: "/tmp/clear"},
		},
	}

	model.selected = 0
	model = updateKey(t, model, "m")
	if model.statusSelected != 1 {
		t.Fatalf("configured statusSelected = %d, want 1", model.statusSelected)
	}
	if !strings.Contains(stripANSI(model.View()), "> parked") {
		t.Fatalf("configured status row not selected:\n%s", stripANSI(model.View()))
	}

	model.screen = screenTable
	model.selected = 1
	model = updateKey(t, model, "m")
	if model.statusSelected != len(cfg.Statuses) {
		t.Fatalf("custom statusSelected = %d, want custom index", model.statusSelected)
	}
	if !strings.Contains(stripANSI(model.View()), "> custom") {
		t.Fatalf("custom row not selected:\n%s", stripANSI(model.View()))
	}

	model.screen = screenTable
	model.selected = 2
	model = updateKey(t, model, "m")
	if model.statusSelected != 0 {
		t.Fatalf("empty statusSelected = %d, want 0", model.statusSelected)
	}
	if !strings.Contains(stripANSI(model.View()), "> active") {
		t.Fatalf("first configured status row not selected:\n%s", stripANSI(model.View()))
	}
}

func TestStatusInputPlaceholderKeepsCursorAfterRendering(t *testing.T) {
	got := stripANSI(statusInputView("", "", 0))

	if !strings.Contains(got, "empty clears status") {
		t.Fatalf("status input placeholder was truncated:\n%s", got)
	}
	if !strings.Contains(got, "enter save") {
		t.Fatalf("status input action hint missing:\n%s", got)
	}
}

func TestModalInputPlaceholderUsesBlockCursor(t *testing.T) {
	got := noteView("", "", "", 0)

	if !strings.Contains(got, "\x1b[38;5;236;48;5;252me\x1b[22;39;48;5;236m") {
		t.Fatalf("modal placeholder should draw a cursor block over the first character:\n%q", got)
	}
	if strings.Contains(got, "\x1b[5;") {
		t.Fatalf("modal placeholder should not blink the first character itself:\n%q", got)
	}
}

func TestModalInputPlaceholderCanHideCursorWithoutHidingText(t *testing.T) {
	got := noteView("", "", "", 0, inputCursorState{Visible: false})
	plain := stripANSI(got)

	if !strings.Contains(plain, "empty clears note") {
		t.Fatalf("modal placeholder should remain visible while cursor is hidden:\n%s", plain)
	}
	if strings.Contains(got, "48;5;252") {
		t.Fatalf("modal placeholder should not draw cursor block while hidden:\n%q", got)
	}
}

func TestNoteInputWrapsInsteadOfTruncating(t *testing.T) {
	value := strings.Repeat("f", 80)
	got := stripANSI(noteView("", value, "", len([]rune(value))))

	if strings.Contains(got, "...") {
		t.Fatalf("note input should wrap instead of truncate:\n%s", got)
	}
	if !strings.Contains(got, strings.Repeat("f", 52)) {
		t.Fatalf("note input missing first wrapped line:\n%s", got)
	}
	if !strings.Contains(got, strings.Repeat("f", 28)+"▌") {
		t.Fatalf("note input missing second wrapped line with cursor:\n%s", got)
	}
}

func TestNotePlaceholderWrapsInsteadOfTruncating(t *testing.T) {
	got := stripANSI(noteView("", "", strings.Repeat("p", 80), 0))

	if strings.Contains(got, "...") {
		t.Fatalf("note placeholder should wrap instead of truncate:\n%s", got)
	}
	if !strings.Contains(got, strings.Repeat("p", 52)) {
		t.Fatalf("note placeholder missing first wrapped line:\n%s", got)
	}
	if !strings.Contains(got, strings.Repeat("p", 28)) {
		t.Fatalf("note placeholder missing second wrapped line:\n%s", got)
	}
}

func TestNotePlaceholderPreservesNewlinesAsModalRows(t *testing.T) {
	got := stripANSI(noteView("", "", "first line\n- second line wraps here\n\nthird line", 0))

	for _, want := range []string{"first line", "- second line wraps here", "third line"} {
		if !strings.Contains(got, want) {
			t.Fatalf("note placeholder missing %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "first line\n- second line wraps here") {
		t.Fatalf("note placeholder embedded newline inside one modal row:\n%s", got)
	}
}

func TestNotePlaceholderWrapsAtWordBoundaries(t *testing.T) {
	got := stripANSI(noteView("", "", "refactor: create content-agnostic segmenter module replacing Bible-specific splitter", 0))

	if strings.Contains(got, "r\neplacing") {
		t.Fatalf("note placeholder split word across lines:\n%s", got)
	}
	if !strings.Contains(got, "refactor: create content-agnostic segmenter module") || !strings.Contains(got, "replacing Bible-specific splitter") {
		t.Fatalf("note placeholder should wrap before replacing:\n%s", got)
	}
}

func TestStatusInputWrapsInsteadOfTruncating(t *testing.T) {
	value := strings.Repeat("f", 60)
	got := stripANSI(statusInputView("", value, len([]rune(value))))

	if strings.Contains(got, "...") {
		t.Fatalf("status input should wrap instead of truncate:\n%s", got)
	}
	if !strings.Contains(got, strings.Repeat("f", 38)) {
		t.Fatalf("status input missing first wrapped line:\n%s", got)
	}
	if !strings.Contains(got, strings.Repeat("f", 22)+"▌") {
		t.Fatalf("status input missing second wrapped line with cursor:\n%s", got)
	}
}

func TestModelStatusPickerClearSavesEmptyStatus(t *testing.T) {
	var savedStatus *string
	cfg := config.Default()
	model := Model{
		config: cfg,
		loader: func(opts app.Options) (app.OverviewResult, error) {
			return app.OverviewResult{Config: cfg}, nil
		},
		updater: func(path string, update app.MetadataUpdate) (app.MetadataUpdateResult, error) {
			savedStatus = update.Status
			return app.MetadataUpdateResult{Path: path}, nil
		},
		projects:       []project.Project{{Name: "app", Path: "/tmp/app"}},
		screen:         screenStatus,
		statusSelected: len(cfg.Statuses) + 1,
	}

	updated, cmd := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(Model)
	if cmd == nil {
		t.Fatal("expected clear status command")
	}
	model = updateMsg(t, model, cmd())
	if savedStatus == nil || *savedStatus != "" {
		t.Fatalf("savedStatus = %v, want empty string pointer", savedStatus)
	}
	if model.message != "Status cleared" {
		t.Fatalf("message = %q, want Status cleared", model.message)
	}
}

func TestModelPinKeyTogglesSelectedProject(t *testing.T) {
	var savedPinned *bool
	cfg := config.Default()
	model := Model{
		config: cfg,
		loader: func(opts app.Options) (app.OverviewResult, error) {
			return app.OverviewResult{
				Config:   cfg,
				Projects: []project.Project{{Name: "app", Path: "/tmp/app", Pinned: true}},
			}, nil
		},
		updater: func(path string, update app.MetadataUpdate) (app.MetadataUpdateResult, error) {
			savedPinned = update.Pinned
			return app.MetadataUpdateResult{Path: path}, nil
		},
		projects: []project.Project{{Name: "app", Path: "/tmp/app"}},
	}

	updated, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}})
	model = updated.(Model)
	if cmd == nil {
		t.Fatal("expected pin command")
	}
	model = updateMsg(t, model, cmd())
	if savedPinned == nil || !*savedPinned {
		t.Fatalf("savedPinned = %v, want true pointer", savedPinned)
	}
	if model.message != "Project pinned" {
		t.Fatalf("message = %q, want Project pinned", model.message)
	}
	if len(model.projects) != 1 || !model.projects[0].Pinned {
		t.Fatalf("projects = %#v", model.projects)
	}
}

func TestModelDetailPinKeyTogglesSelectedProject(t *testing.T) {
	var savedPinned *bool
	cfg := config.Default()
	model := Model{
		config: cfg,
		screen: screenDetail,
		loader: func(opts app.Options) (app.OverviewResult, error) {
			return app.OverviewResult{
				Config:   cfg,
				Projects: []project.Project{{Name: "app", Path: "/tmp/app"}},
			}, nil
		},
		updater: func(path string, update app.MetadataUpdate) (app.MetadataUpdateResult, error) {
			savedPinned = update.Pinned
			return app.MetadataUpdateResult{Path: path}, nil
		},
		projects: []project.Project{{Name: "app", Path: "/tmp/app", Pinned: true}},
	}

	updated, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}})
	model = updated.(Model)
	if cmd == nil {
		t.Fatal("expected detail pin command")
	}
	model = updateMsg(t, model, cmd())
	if savedPinned == nil || *savedPinned {
		t.Fatalf("savedPinned = %v, want false pointer", savedPinned)
	}
	if model.message != "Project unpinned" {
		t.Fatalf("message = %q, want Project unpinned", model.message)
	}
}

func TestModelAddProjectModalSavesPath(t *testing.T) {
	var addedPath string
	model := Model{
		loader: func(opts app.Options) (app.OverviewResult, error) {
			return app.OverviewResult{
				Config:   config.Default(),
				Projects: []project.Project{{Name: "custom", Path: "/tmp/custom"}},
			}, nil
		},
		adder: func(path string) (app.AddProjectResult, error) {
			addedPath = path
			return app.AddProjectResult{Path: "/tmp/custom"}, nil
		},
		projects: []project.Project{{Name: "app", Path: "/tmp/app"}},
	}

	model = updateKey(t, model, "a")
	if model.screen != screenAdd {
		t.Fatalf("screen = %v, want add", model.screen)
	}
	view := stripANSI(model.View())
	for _, want := range []string{"Add project", "~/dev/my-project", "enter save", "esc"} {
		if !strings.Contains(view, want) {
			t.Fatalf("add modal missing %q:\n%s", want, view)
		}
	}
	for _, value := range []string{"/", "t", "m", "p", "/", "c", "u", "s", "t", "o", "m"} {
		model = updateKey(t, model, value)
	}
	updated, cmd := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(Model)
	if cmd == nil {
		t.Fatal("expected add command")
	}
	model = updateMsg(t, model, cmd())
	if addedPath != "/tmp/custom" {
		t.Fatalf("addedPath = %q, want /tmp/custom", addedPath)
	}
	if model.screen != screenTable {
		t.Fatalf("screen = %v, want table", model.screen)
	}
	if model.message != "Project added" {
		t.Fatalf("message = %q, want Project added", model.message)
	}
	if len(model.projects) != 1 || model.projects[0].Path != "/tmp/custom" {
		t.Fatalf("projects = %#v", model.projects)
	}
}

func TestModelAddProjectModalSupportsCursorEditing(t *testing.T) {
	var addedPath string
	model := Model{
		loader: func(opts app.Options) (app.OverviewResult, error) {
			return app.OverviewResult{Config: config.Default()}, nil
		},
		adder: func(path string) (app.AddProjectResult, error) {
			addedPath = path
			return app.AddProjectResult{Path: path}, nil
		},
		screen: screenAdd,
	}
	for _, value := range []string{"/", "t", "m", "p", "/", "a", "p"} {
		model = updateKey(t, model, value)
	}
	model = updateSpecialKey(t, model, tea.KeyLeft)
	model = updateKey(t, model, "p")
	if model.addInput != "/tmp/app" {
		t.Fatalf("addInput = %q, want /tmp/app", model.addInput)
	}
	if !strings.Contains(stripANSI(model.View()), "/tmp/ap▌p") {
		t.Fatalf("add input cursor not rendered in place:\n%s", stripANSI(model.View()))
	}
	model = updateSpecialKey(t, model, tea.KeyDelete)
	if model.addInput != "/tmp/ap" {
		t.Fatalf("addInput after delete = %q, want /tmp/ap", model.addInput)
	}
	model = updateKey(t, model, "p")

	updated, cmd := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(Model)
	if cmd == nil {
		t.Fatal("expected add command")
	}
	model = updateMsg(t, model, cmd())
	if addedPath != "/tmp/app" {
		t.Fatalf("addedPath = %q, want /tmp/app", addedPath)
	}
}

func TestModelAddProjectModalSupportsReadlineEditing(t *testing.T) {
	model := Model{
		screen:    screenAdd,
		addInput:  "/tmp/old-path",
		addCursor: len([]rune("/tmp/old-path")),
	}

	model = updateSpecialKey(t, model, tea.KeyCtrlA)
	model = updateSpecialKey(t, model, tea.KeyCtrlK)
	if model.addInput != "" || model.addCursor != 0 {
		t.Fatalf("addInput after ctrl+a ctrl+k = %q cursor %d, want empty", model.addInput, model.addCursor)
	}
}

func TestModelAddProjectModalShowsErrorsInline(t *testing.T) {
	model := Model{
		adder: func(path string) (app.AddProjectResult, error) {
			return app.AddProjectResult{}, errors.New("missing directory")
		},
		screen:   screenAdd,
		addInput: "/tmp/missing",
	}

	updated, cmd := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(Model)
	if cmd == nil {
		t.Fatal("expected add command")
	}
	model = updateMsg(t, model, cmd())
	if model.screen != screenAdd {
		t.Fatalf("screen = %v, want add", model.screen)
	}
	if model.loading {
		t.Fatal("model should not stay loading")
	}
	view := stripANSI(model.View())
	if !strings.Contains(view, "missing directory") {
		t.Fatalf("add modal missing inline error:\n%s", view)
	}
}

func TestModelReloadPreservesSelectionByPath(t *testing.T) {
	reloaded := false
	model := Model{
		loader: func(opts app.Options) (app.OverviewResult, error) {
			reloaded = true
			return app.OverviewResult{
				Config: config.Default(),
				Projects: []project.Project{
					{Name: "c", Path: "/tmp/c"},
					{Name: "b", Path: "/tmp/b"},
					{Name: "a", Path: "/tmp/a"},
				},
			}, nil
		},
		projects: []project.Project{
			{Name: "a", Path: "/tmp/a"},
			{Name: "b", Path: "/tmp/b"},
			{Name: "c", Path: "/tmp/c"},
		},
		selected: 1,
	}

	updated, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("r")})
	model = updated.(Model)
	if cmd == nil {
		t.Fatal("expected reload command")
	}
	if !model.loading {
		t.Fatal("model should be loading")
	}
	model = updateMsg(t, model, cmd())
	if !reloaded {
		t.Fatal("loader was not called")
	}
	if model.selected != 1 {
		t.Fatalf("selected = %d, want index of /tmp/b", model.selected)
	}
	if model.projects[model.selected].Path != "/tmp/b" {
		t.Fatalf("selected project = %#v", model.projects[model.selected])
	}
	if model.message != "Reloaded" {
		t.Fatalf("message = %q, want Reloaded", model.message)
	}
}

func TestModelOpenEditorUsesSelectedProject(t *testing.T) {
	cfg := config.Default()
	cfg.Editor = "code --reuse-window"
	var gotEditor string
	var gotPath string
	model := Model{
		config: cfg,
		editor: func(editor, path string) error {
			gotEditor = editor
			gotPath = path
			return nil
		},
		projects: []project.Project{
			{Name: "one", Path: "/tmp/one"},
			{Name: "two", Path: "/tmp/two"},
		},
		selected: 1,
	}

	updated, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("o")})
	model = updated.(Model)
	if cmd == nil {
		t.Fatal("expected editor command")
	}
	model = updateMsg(t, model, cmd())
	if gotEditor != "code --reuse-window" {
		t.Fatalf("editor = %q", gotEditor)
	}
	if gotPath != "/tmp/two" {
		t.Fatalf("path = %q, want /tmp/two", gotPath)
	}
	if model.message != "Opened two" {
		t.Fatalf("message = %q, want Opened two", model.message)
	}
}

func TestModelOpenEditorShowsError(t *testing.T) {
	model := Model{
		config: config.Default(),
		editor: func(editor, path string) error {
			return errors.New("no editor")
		},
		projects: []project.Project{{Name: "one", Path: "/tmp/one"}},
	}

	updated, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("o")})
	model = updated.(Model)
	if cmd == nil {
		t.Fatal("expected editor command")
	}
	model = updateMsg(t, model, cmd())
	if !strings.Contains(model.message, "Editor failed: no editor") {
		t.Fatalf("message = %q", model.message)
	}
}

func TestModelOpenTerminalUsesSelectedProject(t *testing.T) {
	var gotPath string
	var gotName string
	var gotShell string
	cfg := config.Default()
	cfg.Shell = "zsh"
	model := Model{
		config: cfg,
		terminal: func(path, name, shell string) tea.Cmd {
			gotPath = path
			gotName = name
			gotShell = shell
			return func() tea.Msg {
				return terminalOpenedMsg{message: "Opened terminal two"}
			}
		},
		projects: []project.Project{
			{Name: "one", Path: "/tmp/one"},
			{Name: "two", Path: "/tmp/two"},
		},
		selected: 1,
	}

	updated, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("t")})
	model = updated.(Model)
	if cmd == nil {
		t.Fatal("expected terminal command")
	}
	model = updateMsg(t, model, cmd())
	if gotPath != "/tmp/two" {
		t.Fatalf("path = %q, want /tmp/two", gotPath)
	}
	if gotName != "two" {
		t.Fatalf("name = %q, want two", gotName)
	}
	if gotShell != "zsh" {
		t.Fatalf("shell = %q, want zsh", gotShell)
	}
	if model.message != "Opened terminal two" {
		t.Fatalf("message = %q, want Opened terminal two", model.message)
	}
}

func TestModelOpenTerminalShowsError(t *testing.T) {
	model := Model{
		terminal: func(path, name, shell string) tea.Cmd {
			return func() tea.Msg {
				return terminalFailedMsg{err: errors.New("no shell")}
			}
		},
		projects: []project.Project{{Name: "one", Path: "/tmp/one"}},
	}

	updated, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("t")})
	model = updated.(Model)
	if cmd == nil {
		t.Fatal("expected terminal command")
	}
	model = updateMsg(t, model, cmd())
	if !strings.Contains(model.message, "Terminal failed: no shell") {
		t.Fatalf("message = %q", model.message)
	}
}

func TestModelHelpOpensAndCloses(t *testing.T) {
	model := updateKey(t, Model{
		width:    140,
		height:   24,
		projects: []project.Project{{Name: "app", Path: "/tmp/app"}},
	}, "?")
	if model.screen != screenHelp {
		t.Fatalf("screen = %v, want help", model.screen)
	}
	view := model.View()
	for _, want := range []string{"Name", "app", "ovw", "1.1.1", "A terminal overview for your local projects.", "https://github.com/roie/ovw", "search", "add", "filter", "sort", "command", "open editor", "terminal", "note", "status", "pin", "reload", "quit", "esc"} {
		if !strings.Contains(view, want) {
			t.Fatalf("help view missing %q:\n%s", want, view)
		}
	}
	for _, notWant := range []string{"Help", "Navigation", "Actions", "MIT"} {
		if strings.Contains(view, notWant) {
			t.Fatalf("help view should not include %q:\n%s", notWant, view)
		}
	}
	model = updateSpecialKey(t, model, tea.KeyEsc)
	if model.screen != screenTable {
		t.Fatalf("screen = %v, want table", model.screen)
	}

	model = updateKey(t, model, "?")
	model = updateSpecialKey(t, model, tea.KeyEnter)
	if model.screen != screenTable {
		t.Fatalf("screen = %v, want table after enter", model.screen)
	}
}

func TestModelNoProjectsState(t *testing.T) {
	model := Model{width: 100, height: 24}
	raw := model.View()
	view := stripANSI(raw)
	for _, want := range []string{"No projects found", "a", "add project", "r", "reload"} {
		if !strings.Contains(view, want) {
			t.Fatalf("View() missing %q:\n%s", want, view)
		}
	}
	if strings.Contains(raw, "48;5;236") {
		t.Fatalf("empty table should not use modal background styling:\n%q", raw)
	}
}

func TestModelNoProjectsStateShowsPathScope(t *testing.T) {
	model := Model{
		width:   100,
		height:  24,
		request: app.Options{Path: "library1"},
	}

	raw := model.View()
	view := stripANSI(raw)
	if !strings.Contains(view, "No projects match path: library1") {
		t.Fatalf("empty path scope view missing message:\n%s", view)
	}
	if strings.Contains(raw, "48;5;236") {
		t.Fatalf("path empty state should not use modal background styling:\n%q", raw)
	}
}

func TestModelWideViewShowsInlineDetailPane(t *testing.T) {
	model := Model{
		width:  140,
		height: 24,
		projects: []project.Project{
			detailTestProject("one"),
			detailTestProject("two"),
		},
		selected: 1,
	}

	view := model.View()
	for _, want := range []string{"one", "two", "Path", "/tmp/two", "Stack", "Go, Cobra"} {
		if !strings.Contains(view, want) {
			t.Fatalf("wide view missing %q:\n%s", want, view)
		}
	}
	if !strings.Contains(view, "Note") {
		t.Fatalf("wide table should keep note column:\n%s", view)
	}
	header := strings.Split(stripANSI(view), "\n")[0]
	for _, want := range []string{"ovw  2 projects", "filter: all", "2/2"} {
		if !strings.Contains(header, want) {
			t.Fatalf("wide header missing %q:\n%s", want, header)
		}
	}
	if !strings.HasSuffix(header, "2/2") {
		t.Fatalf("wide header should show selection position on the right:\n%s", view)
	}
	if !strings.Contains(view, " │ ") {
		t.Fatalf("wide view missing split divider:\n%s", view)
	}
	lines := strings.Split(stripANSI(view), "\n")
	for _, line := range lines {
		if strings.Contains(line, "Name") && strings.Contains(line, "│") {
			if index := strings.Index(line, "│"); index < 80 {
				t.Fatalf("divider starts too early at %d:\n%s", index, view)
			}
			return
		}
	}
	t.Fatalf("wide view missing table header divider:\n%s", view)
}

func TestModelWideViewFitsTerminalWidth(t *testing.T) {
	selection := detailTestProject("instaview")
	selection.Note.Display = "fix: extract carousel from SJS for long-format shortcodes\n- Add SJS script extraction for carousel posts"
	model := Model{
		width:    157,
		height:   31,
		projects: []project.Project{selection},
	}

	view := model.View()
	lines := strings.Split(stripANSI(view), "\n")
	for _, line := range lines {
		if len([]rune(line)) > model.width {
			t.Fatalf("line width = %d, want <= %d:\n%s", len([]rune(line)), model.width, view)
		}
	}
	if !strings.Contains(view, "long-format") {
		t.Fatalf("wide view should preserve full word in detail note:\n%s", view)
	}
}

func TestSplitDividerAlignsOnSelectedRows(t *testing.T) {
	got := joinColumns(
		strings.Join([]string{
			"Name",
			selectedStyle.Render("selected row"),
			"next row",
		}, "\n"),
		strings.Join([]string{
			"detail",
			"Stack Go",
			"Status dirty",
		}, "\n"),
		3,
	)

	lines := strings.Split(stripANSI(got), "\n")
	want := strings.Index(lines[0], "│")
	if want < 0 {
		t.Fatalf("missing divider:\n%s", got)
	}
	for _, line := range lines[1:] {
		if index := strings.Index(line, "│"); index != want {
			t.Fatalf("divider index = %d, want %d:\n%s", index, want, got)
		}
	}
}

func TestSplitDividerUsesFixedHeight(t *testing.T) {
	got := joinColumns("Name\napp", "detail", 3, 5)
	lines := strings.Split(stripANSI(got), "\n")

	if len(lines) != 5 {
		t.Fatalf("joinColumns rendered %d lines, want 5:\n%s", len(lines), got)
	}
	for index, line := range lines {
		if !strings.Contains(line, "│") {
			t.Fatalf("line %d missing divider:\n%s", index, got)
		}
	}
}

func TestOverlayModalPreservesBackgroundAroundModal(t *testing.T) {
	base := strings.Join([]string{
		"left row keeps visible right side",
		"prefix background middle suffix",
		"bottom row remains visible",
		"final row remains visible",
	}, "\n")
	modal := strings.Join([]string{
		"modal",
		"body",
	}, "\n")

	got := overlayModal(base, modal, 40)

	for _, want := range []string{"prefix", "suffix", "final row remains visible", "modal", "body"} {
		if !strings.Contains(got, want) {
			t.Fatalf("overlay missing %q:\n%s", want, got)
		}
	}
	for _, notWant := range []string{"background middle"} {
		if strings.Contains(got, notWant) {
			t.Fatalf("overlay did not clear covered background %q:\n%s", notWant, got)
		}
	}
}

func TestOverlayLinePreservesBackgroundOutsideModal(t *testing.T) {
	got := stripANSI(overlayLine("abcdefghijklmnopqrstuvwxyz", "MODAL", 10, 30))

	if !strings.Contains(got, "abcdefghijMODALpqrstuvwxyz") {
		t.Fatalf("overlay line did not preserve outside background:\n%q", got)
	}
	if strings.Contains(got, "klmno") {
		t.Fatalf("overlay line preserved covered background:\n%q", got)
	}
}

func TestModelNarrowViewKeepsDetailPaneHidden(t *testing.T) {
	model := Model{
		width:  80,
		height: 24,
		projects: []project.Project{
			detailTestProject("one"),
		},
	}

	view := model.View()
	lines := strings.Split(view, "\n")
	body := strings.Join(lines[:len(lines)-1], "\n")
	if strings.Contains(body, "Path") || strings.Contains(body, "/tmp/one") {
		t.Fatalf("narrow table view should not render inline detail:\n%s", view)
	}
}

func updateKey(t *testing.T, model Model, value string) Model {
	t.Helper()
	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(value)})
	return updated.(Model)
}

func updateSpecialKey(t *testing.T, model Model, key tea.KeyType) Model {
	t.Helper()
	updated, _ := model.Update(tea.KeyMsg{Type: key})
	return updated.(Model)
}

func updateMsg(t *testing.T, model Model, msg tea.Msg) Model {
	t.Helper()
	updated, _ := model.Update(msg)
	return updated.(Model)
}

func hasCommandLabel(actions []commandAction, label string) bool {
	for _, action := range actions {
		if action.Label == label {
			return true
		}
	}
	return false
}

func commandLabels(actions []commandAction) []string {
	labels := make([]string, 0, len(actions))
	for _, action := range actions {
		labels = append(labels, action.Label)
	}
	return labels
}

func updateSetupKey(t *testing.T, model setupModel, value string) setupModel {
	t.Helper()
	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(value)})
	return updated.(setupModel)
}

func updateSetupSpecialKey(t *testing.T, model setupModel, key tea.KeyType) setupModel {
	t.Helper()
	updated, _ := model.Update(tea.KeyMsg{Type: key})
	return updated.(setupModel)
}

func updateSetupMsg(t *testing.T, model setupModel, msg tea.Msg) setupModel {
	t.Helper()
	updated, _ := model.Update(msg)
	return updated.(setupModel)
}

func detailTestProject(name string) project.Project {
	return project.Project{
		Name:         name,
		Path:         "/tmp/" + name,
		Stack:        []string{"Go", "Cobra"},
		StackDisplay: "Go",
		Managers:     []string{"go modules"},
		Scripts:      []string{"dev", "build", "check"},
		Version:      "1.2.3",
		Activity: ovwformat.ActivityInfo{
			Display:           "12m ↑2",
			LastCommitAge:     "12m",
			LastCommitAt:      time.Date(2026, 5, 7, 12, 30, 0, 0, time.UTC),
			LastCommitMessage: "feat: add detail",
			Unpushed:          2,
			Dirty:             true,
			Branch:            "main",
			HasGit:            true,
			HasCommits:        true,
		},
		Status:      ovwformat.StatusFromTags("parked", []string{"dirty", "parked"}),
		Note:        ovwformat.NoteInfo{Display: "Value note", Value: "Value note"},
		Description: "Project description",
	}
}
