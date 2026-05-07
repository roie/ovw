package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestNewModelRendersPlaceholder(t *testing.T) {
	got := New().View()
	for _, want := range []string{"ovw", "TUI loading...", "quit"} {
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
