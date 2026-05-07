package tui

import (
	"errors"
	"strings"
	"testing"

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

func TestModelLoadsOverviewData(t *testing.T) {
	model := NewWithLoader(func(opts app.Options) (app.OverviewResult, error) {
		return app.OverviewResult{
			Config: config.Default(),
			Projects: []project.Project{
				{
					Name:         "app",
					StackDisplay: "Go",
					Activity:     ovwformat.ActivityInfo{Display: "12m"},
					Tags:         []string{"dirty"},
					Note:         ovwformat.NoteInfo{Display: "manual note"},
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
	for _, want := range []string{"ovw", "1 project", "Name", "Stack", "Activity", "Status", "Note", "app", "Go", "12m", "dirty", "manual note"} {
		if !strings.Contains(view, want) {
			t.Fatalf("View() missing %q:\n%s", want, view)
		}
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
