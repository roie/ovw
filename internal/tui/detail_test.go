package tui

import (
	"strings"
	"testing"
)

func TestDetailSummaryShowsUsefulFieldsAndNoteBlock(t *testing.T) {
	project := detailTestProject("eventca")

	got := stripANSI(detailSummaryView(project, 60))
	for _, want := range []string{
		"eventca",
		"Path     /tmp/eventca",
		"Stack    Go, Cobra",
		"Activity 12m",
		"Status   dirty · unpushed",
		"Branch   main",
		"Manager  go modules",
		"Note",
		"Manual note",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("detail summary missing %q:\n%s", want, got)
		}
	}
}

func TestDetailSummaryWrapsNote(t *testing.T) {
	project := detailTestProject("eventca")
	project.Note.Display = "chore: add biome and apply repository-wide formatting"

	got := stripANSI(detailSummaryView(project, 28))

	if strings.Contains(got, "...") {
		t.Fatalf("detail summary note should wrap instead of truncate:\n%s", got)
	}
	for _, want := range []string{"chore: add biome and apply", "repository-wide formatting"} {
		if !strings.Contains(got, want) {
			t.Fatalf("detail summary note missing %q:\n%s", want, got)
		}
	}
}

func TestShortPathUsesHomePrefix(t *testing.T) {
	t.Setenv("HOME", "/home/roie")

	if got := shortPath("/home/roie/dev/web/eventca"); got != "~/dev/web/eventca" {
		t.Fatalf("shortPath() = %q, want ~/dev/web/eventca", got)
	}
}
