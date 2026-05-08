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
		"Manager  go modules",
		"Scripts  dev, build, check",
		"Version  1.2.3",
		"Branch   main",
		"Activity 12m",
		"Status   dirty · unpushed",
		"Project description",
		"Note",
		"Value note",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("detail summary missing %q:\n%s", want, got)
		}
	}
	mustAppearInOrder(t, got, []string{
		"Path",
		"Stack",
		"Manager",
		"Scripts",
		"Version",
		"Branch",
		"Activity",
		"Updated",
		"Status",
		"Note",
	})
	mustAppearInOrder(t, got, []string{"eventca", "Project description", "Path"})
	for _, unwanted := range []string{"Last commit", "Git"} {
		if strings.Contains(got, unwanted) {
			t.Fatalf("detail summary contains redundant field %q:\n%s", unwanted, got)
		}
	}
}

func TestDetailModalShowsUnhideActionForHiddenProject(t *testing.T) {
	project := detailTestProject("eventca")
	project.Hidden = true

	got := stripANSI(detailModalView(project, true, 60))
	if !strings.Contains(got, "eventca") {
		t.Fatalf("detail modal missing project title:\n%s", got)
	}
	if strings.Contains(got, "Details") {
		t.Fatalf("detail modal should use project name as title:\n%s", got)
	}
	if !strings.Contains(got, "x unhide") {
		t.Fatalf("detail modal missing unhide action:\n%s", got)
	}
}

func TestDetailModalUsesFallbackTitleWithoutProject(t *testing.T) {
	got := stripANSI(detailModalView(detailTestProject("eventca"), false, 60))

	if !strings.Contains(got, "Details") {
		t.Fatalf("detail modal missing fallback title:\n%s", got)
	}
	if !strings.Contains(got, "No project selected") {
		t.Fatalf("detail modal missing empty state:\n%s", got)
	}
}

func TestDetailSummaryWrapsNote(t *testing.T) {
	project := detailTestProject("eventca")
	project.Note.Display = "chore: add biome and apply repository-wide formatting"

	got := stripANSI(detailSummaryView(project, 28))

	for _, want := range []string{"chore: add biome and apply", "repository-wide formatting"} {
		if !strings.Contains(got, want) {
			t.Fatalf("detail summary note missing %q:\n%s", want, got)
		}
	}
	noteBlock := got[strings.Index(got, "Note"):]
	if strings.Contains(noteBlock, "...") {
		t.Fatalf("detail summary note should wrap instead of truncate:\n%s", got)
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

func TestDetailModalWrapsDescriptionAndNote(t *testing.T) {
	project := detailTestProject("imagio")
	project.Description = "Imagio - View Image Properties. The image inspector that actually helps."
	project.Note.Display = "release/refactor · refactor: split repo into workspaces and packages"

	got := stripANSI(detailModalView(project, true, 56))

	for _, want := range []string{
		"Imagio - View Image Properties.",
		"that actually helps.",
		"Note",
		"release/refactor · refactor:",
		"into workspaces and packages",
		"x hide",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("detail modal missing wrapped text %q:\n%s", want, got)
		}
	}
	noteBlock := got[strings.Index(got, "Note"):]
	if strings.Contains(noteBlock, "...") {
		t.Fatalf("detail modal note should wrap instead of truncate:\n%s", got)
	}
	if strings.Contains(got, "actually...") || strings.Contains(got, "packages...") {
		t.Fatalf("detail modal description/note was truncated:\n%s", got)
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
	mustAppearInOrder(t, got, []string{"Note", "Value note", "Recent"})
}

func TestDetailSummaryLimitsFallbackNotePreview(t *testing.T) {
	project := detailTestProject("ripgrep")
	project.Note = ovwformat.NoteInfo{
		Display: strings.Join([]string{
			"doc: clarify half-boundary syntax for the -w flag",
			"",
			"In regex 1.10.0, the half-boundary assertions were introduced.",
			"While the CLI help text was updated, the rationale was omitted.",
			"Furthermore, GUIDE.md was still outdated.",
			"This final sentence should not take over the side pane.",
		}, "\n"),
		Source: "commit",
	}
	project.Activity.RecentCommits = []ovwformat.RecentCommit{
		{Hash: "abc1234", Subject: "doc: clarify half-boundary syntax", Age: "2mo"},
	}

	got := stripANSI(detailSummaryView(project, 48))
	noteBlock := got[strings.Index(got, "Note"):]

	if !strings.Contains(noteBlock, "...") {
		t.Fatalf("fallback note should show a preview marker:\n%s", got)
	}
	if strings.Contains(noteBlock, "This final sentence should not take over") {
		t.Fatalf("fallback note should be capped in side pane:\n%s", got)
	}
	if !strings.Contains(got, "Recent") {
		t.Fatalf("recent commits should remain visible after fallback note:\n%s", got)
	}
}

func TestDetailSummaryDoesNotLimitUserNote(t *testing.T) {
	project := detailTestProject("eventca")
	project.Note = ovwformat.NoteInfo{
		Display: "line one\nline two\nline three\nline four\nline five",
		Source:  "user",
		Value:   "line one\nline two\nline three\nline four\nline five",
	}

	got := stripANSI(detailSummaryView(project, 40))

	if strings.Contains(got, "...") {
		t.Fatalf("user note should not be capped:\n%s", got)
	}
	if !strings.Contains(got, "line five") {
		t.Fatalf("user note should render all lines:\n%s", got)
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

func mustAppearInOrder(t *testing.T, text string, values []string) {
	t.Helper()
	offset := 0
	for _, value := range values {
		index := strings.Index(text[offset:], value)
		if index < 0 {
			t.Fatalf("value %q not found after offset %d:\n%s", value, offset, text)
		}
		offset += index + len(value)
	}
}
