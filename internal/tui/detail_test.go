package tui

import (
	"strings"
	"testing"

	ovwformat "ovw/internal/format"
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

func TestDetailSummaryPreservesNoteNewlines(t *testing.T) {
	project := detailTestProject("eventca")
	project.Note.Display = "blocked by API auth\ncheck after deploy\n\nsecond paragraph wraps here"

	got := stripANSI(detailSummaryView(project, 24))

	for _, want := range []string{
		"blocked by API auth",
		"check after deploy",
		"second paragraph wraps",
		"here",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("detail summary note missing %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "auth check") {
		t.Fatalf("detail summary collapsed explicit newline:\n%s", got)
	}
	if !strings.Contains(got, "check after deploy\n\nsecond paragraph") {
		t.Fatalf("detail summary did not preserve blank line:\n%s", got)
	}
}

func TestDetailSummaryShowsRecentCommits(t *testing.T) {
	project := detailTestProject("eventca")
	project.Activity.RecentCommits = []ovwformat.RecentCommit{
		{Hash: "abc1234", Subject: "fix modal surface", Age: "12m"},
		{Hash: "def5678", Subject: "add recent sidepane", Age: "2h"},
	}

	got := stripANSI(detailSummaryView(project, 60))
	for _, want := range []string{
		"Recent",
		"abc1234  fix modal surface",
		"12m",
		"def5678  add recent sidepane",
		"2h",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("detail summary missing recent commit %q:\n%s", want, got)
		}
	}
}

func TestDetailSummaryHidesRecentCommitsWhenEmpty(t *testing.T) {
	project := detailTestProject("eventca")
	project.Activity.RecentCommits = nil

	got := stripANSI(detailSummaryView(project, 60))
	if strings.Contains(got, "Recent") {
		t.Fatalf("detail summary should hide empty recent commits:\n%s", got)
	}
}

func TestShortPathUsesHomePrefix(t *testing.T) {
	t.Setenv("HOME", "/home/roie")

	if got := shortPath("/home/roie/dev/web/eventca"); got != "~/dev/web/eventca" {
		t.Fatalf("shortPath() = %q, want ~/dev/web/eventca", got)
	}
}
