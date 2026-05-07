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

func TestShortPathUsesHomePrefix(t *testing.T) {
	t.Setenv("HOME", "/home/roie")

	if got := shortPath("/home/roie/dev/web/eventca"); got != "~/dev/web/eventca" {
		t.Fatalf("shortPath() = %q, want ~/dev/web/eventca", got)
	}
}
