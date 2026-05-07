package tui

import (
	"errors"
	"strings"
	"testing"
	"time"

	"ovw/internal/app"
	"ovw/internal/config"
	ovwformat "ovw/internal/format"
	"ovw/internal/project"

	tea "github.com/charmbracelet/bubbletea"
)

func TestNewModelRendersLoadingState(t *testing.T) {
	got := New().View()
	for _, want := range []string{"ovw", "Loading projects...", "quit"} {
		if !strings.Contains(got, want) {
			t.Fatalf("View() missing %q:\n%s", want, got)
		}
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

	view := stripANSI(model.View())
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

func TestModelKeepsTableVisibleDuringBackgroundLoading(t *testing.T) {
	model := Model{
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
	for _, want := range []string{"ovw  1 project  1/1  filter: all  sort: activity", "app", "Go", "12m", "dirty", "user note"} {
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
		activeFilter: "dirty",
		activeSort:   "name",
		projects: []project.Project{
			{Name: "app"},
			{Name: "api"},
		},
	}

	view := stripANSI(model.View())
	if !strings.Contains(view, "ovw  2 projects  1/2  filter: dirty  sort: name") {
		t.Fatalf("header missing filter and sort:\n%s", view)
	}
	if strings.Contains(view, "ovw -") {
		t.Fatalf("header should not use dash separator:\n%s", view)
	}
}

func TestModelHeaderShowsAllFilter(t *testing.T) {
	model := Model{
		activeFilter: "all",
		activeSort:   "activity",
		projects:     []project.Project{{Name: "app"}},
	}

	view := stripANSI(model.View())
	if !strings.Contains(view, "ovw  1 project  1/1  filter: all  sort: activity") {
		t.Fatalf("header missing all filter:\n%s", view)
	}
}

func TestModelHeaderSpacesSearchLikeOtherSegments(t *testing.T) {
	model := Model{
		activeFilter: "all",
		activeSort:   "activity",
		searching:    true,
		projects:     []project.Project{{Name: "app"}},
	}

	view := stripANSI(model.View())
	if !strings.Contains(view, "sort: activity  search: ▌") {
		t.Fatalf("header should separate sort and search consistently:\n%s", view)
	}
	if strings.Contains(view, "search:  ▌") {
		t.Fatalf("search label should not add extra spacing after colon:\n%s", view)
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
	for _, want := range []string{"Details", "one", "two", " │ ", "/tmp/two", "Go, Cobra", "go modules", "1.2.3", "dirty", "2 unpushed", "Value note"} {
		if !strings.Contains(view, want) {
			t.Fatalf("detail view missing %q:\n%s", want, view)
		}
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

func TestModelDetailEnterDoesNotToggleVisibility(t *testing.T) {
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
	if model.screen != screenDetail {
		t.Fatalf("screen = %v, want detail", model.screen)
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

func TestModelSearchEscClearsSearch(t *testing.T) {
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
		t.Fatal("search mode still active")
	}
	if model.search != "" {
		t.Fatalf("search = %q, want empty", model.search)
	}
	if len(model.visibleProjects()) != 2 {
		t.Fatalf("visible projects = %#v", model.visibleProjects())
	}
}

func TestModelSearchNoMatchesState(t *testing.T) {
	model := Model{
		projects: []project.Project{{Name: "api"}},
		search:   "zzz",
	}

	if !strings.Contains(model.View(), "No projects match search") {
		t.Fatalf("View() missing no matches state:\n%s", model.View())
	}
}

func TestModelFilterPickerAppliesDirtyFilter(t *testing.T) {
	var captured app.Options
	model := Model{
		width:  140,
		height: 24,
		loader: func(opts app.Options) (app.OverviewResult, error) {
			captured = opts
			return app.OverviewResult{
				Config:   config.Default(),
				Projects: []project.Project{{Name: "dirty-project"}},
			}, nil
		},
		config: config.Default(),
		projects: []project.Project{
			{Name: "dirty-project"},
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
	if cmd == nil {
		t.Fatal("expected filter apply to reload data")
	}
	model = updateMsg(t, model, cmd())
	if !captured.Dirty {
		t.Fatalf("captured options = %#v, want dirty", captured)
	}
	if model.activeFilter != "dirty" {
		t.Fatalf("activeFilter = %q, want dirty", model.activeFilter)
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

func TestModalPickersUseCaretSelection(t *testing.T) {
	cases := []struct {
		name string
		view string
		want string
	}{
		{name: "filter", view: filterView([]filterOption{{Label: "all"}, {Label: "dirty"}}, 1), want: "> dirty"},
		{name: "sort", view: sortView([]sortOption{{Label: "activity"}, {Label: "name"}}, 1), want: "> name"},
		{name: "status", view: statusView([]statusOption{{Label: "parked"}, {Label: "shipped"}}, 1), want: "> shipped"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := stripANSI(tc.view)
			if !strings.Contains(got, tc.want) {
				t.Fatalf("picker missing caret selection %q:\n%s", tc.want, got)
			}
			if strings.Contains(got, "\x1b[48;5;57m") {
				t.Fatalf("picker should not use selected background:\n%q", tc.view)
			}
			if !strings.Contains(tc.view, "\x1b[38;5;86;48;5;236m> "+strings.TrimPrefix(tc.want, "> ")) {
				t.Fatalf("picker selected text should be accented:\n%q", tc.view)
			}
			if !strings.Contains(tc.view, "\x1b[38;5;244;48;5;236m") {
				t.Fatalf("picker unselected text should be muted:\n%q", tc.view)
			}
		})
	}
}

func TestModelSortPickerAppliesNameSort(t *testing.T) {
	var captured app.Options
	model := Model{
		width:  140,
		height: 24,
		loader: func(opts app.Options) (app.OverviewResult, error) {
			captured = opts
			return app.OverviewResult{
				Config:   config.Default(),
				Projects: []project.Project{{Name: "a"}, {Name: "b"}},
			}, nil
		},
		activeSort: "activity",
		projects:   []project.Project{{Name: "a"}, {Name: "b"}},
	}

	model = updateKey(t, model, "s")
	if model.screen != screenSort {
		t.Fatalf("screen = %v, want sort", model.screen)
	}
	if !strings.Contains(model.View(), "name") {
		t.Fatalf("sort view missing name option:\n%s", model.View())
	}
	for _, want := range []string{"Name", "Sort", "activity", "esc"} {
		if !strings.Contains(model.View(), want) {
			t.Fatalf("sort modal view missing %q:\n%s", want, model.View())
		}
	}
	model = updateKey(t, model, "j")
	updated, cmd := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(Model)
	if cmd == nil {
		t.Fatal("expected sort apply to reload data")
	}
	model = updateMsg(t, model, cmd())
	if captured.Sort != "name" {
		t.Fatalf("captured sort = %q, want name", captured.Sort)
	}
	if model.activeSort != "name" {
		t.Fatalf("activeSort = %q, want name", model.activeSort)
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
	for _, want := range []string{"Name", "1/1", "app", "Note", "old▌", "enter", "save", "esc"} {
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
	for _, want := range []string{"empty clears note▌", "enter", "save"} {
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
	if !strings.Contains(view, "commit fallback note▌") {
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
	for _, want := range []string{"Name", "1/1", "app", "Status", "parked", "shipped", "enter select", "esc"} {
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
	if !strings.Contains(view, "empty clears status▌") {
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

func TestStatusInputPlaceholderKeepsCursorAfterRendering(t *testing.T) {
	got := stripANSI(statusInputView(""))

	if !strings.Contains(got, "empty clears status▌") {
		t.Fatalf("status input placeholder was truncated:\n%s", got)
	}
	if !strings.Contains(got, "enter save") {
		t.Fatalf("status input action hint missing:\n%s", got)
	}
}

func TestModalInputCursorBlinks(t *testing.T) {
	got := noteView("", "")

	if !strings.Contains(got, "\x1b[5;38;5;252;48;5;236m▌\x1b[25;22;39;48;5;236m") {
		t.Fatalf("modal cursor should use ANSI blink:\n%q", got)
	}
}

func TestNoteInputWrapsInsteadOfTruncating(t *testing.T) {
	got := stripANSI(noteView(strings.Repeat("f", 80), ""))

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
	got := stripANSI(noteView("", strings.Repeat("p", 80)))

	if strings.Contains(got, "...") {
		t.Fatalf("note placeholder should wrap instead of truncate:\n%s", got)
	}
	if !strings.Contains(got, strings.Repeat("p", 52)) {
		t.Fatalf("note placeholder missing first wrapped line:\n%s", got)
	}
	if !strings.Contains(got, strings.Repeat("p", 28)+"▌") {
		t.Fatalf("note placeholder missing second wrapped line with cursor:\n%s", got)
	}
}

func TestNotePlaceholderPreservesNewlinesAsModalRows(t *testing.T) {
	got := stripANSI(noteView("", "first line\n- second line wraps here\n\nthird line"))

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
	got := stripANSI(noteView("", "refactor: create content-agnostic segmenter module replacing Bible-specific splitter"))

	if strings.Contains(got, "r\neplacing") {
		t.Fatalf("note placeholder split word across lines:\n%s", got)
	}
	if !strings.Contains(got, "refactor: create content-agnostic segmenter module") || !strings.Contains(got, "replacing Bible-specific splitter") {
		t.Fatalf("note placeholder should wrap before replacing:\n%s", got)
	}
}

func TestStatusInputWrapsInsteadOfTruncating(t *testing.T) {
	got := stripANSI(statusInputView(strings.Repeat("f", 60)))

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
	model := Model{
		terminal: func(path, name string) tea.Cmd {
			gotPath = path
			gotName = name
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
	if model.message != "Opened terminal two" {
		t.Fatalf("message = %q, want Opened terminal two", model.message)
	}
}

func TestModelOpenTerminalShowsError(t *testing.T) {
	model := Model{
		terminal: func(path, name string) tea.Cmd {
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
	for _, want := range []string{"Name", "app", "Help", "search", "filter", "sort", "reload", "quit", "esc"} {
		if !strings.Contains(view, want) {
			t.Fatalf("help view missing %q:\n%s", want, view)
		}
	}
	model = updateSpecialKey(t, model, tea.KeyEsc)
	if model.screen != screenTable {
		t.Fatalf("screen = %v, want table", model.screen)
	}
}

func TestModelNoProjectsState(t *testing.T) {
	model := Model{}
	if !strings.Contains(model.View(), "No projects found") {
		t.Fatalf("View() missing no projects state:\n%s", model.View())
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
	if !strings.Contains(stripANSI(view), "ovw  2 projects  2/2  filter: all") {
		t.Fatalf("wide header should show selection position:\n%s", view)
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
			"\x1b[38;5;229;48;5;57mselected row\x1b[0m",
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
	if strings.Contains(view, "Path") || strings.Contains(view, "/tmp/one") {
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

func detailTestProject(name string) project.Project {
	return project.Project{
		Name:         name,
		Path:         "/tmp/" + name,
		Stack:        []string{"Go", "Cobra"},
		StackDisplay: "Go",
		Managers:     []string{"go modules"},
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
		Status:      ovwformat.StatusFromTags("parked", []string{"dirty", "unpushed", "parked"}),
		Note:        ovwformat.NoteInfo{Display: "Value note", Value: "Value note"},
		Description: "Project description",
	}
}
