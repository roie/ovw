package tui

import (
	"fmt"
	"regexp"
	"strings"
	"testing"
	"time"

	"ovw/internal/config"
	ovwformat "ovw/internal/format"
	"ovw/internal/project"
)

func TestTableViewRendersProjectColumns(t *testing.T) {
	got := tableView([]project.Project{
		{
			Name:         "eventca",
			StackDisplay: "SvelteKit+CF",
			Activity:     ovwformat.ActivityInfo{Display: "40m"},
			Status:       ovwformat.StatusFromTags("", []string{"dirty", "stale"}),
			Note:         ovwformat.NoteInfo{Display: "partial check-in"},
		},
	}, -1, 100, 0, 0, config.Default(), "activity", "desc")

	for _, want := range []string{"Name", "Stack", "Activity ↓", "Status", "Note", "eventca", "SvelteKit+CF", "40m", "dirty", "stale", "partial check-in"} {
		if !strings.Contains(got, want) {
			t.Fatalf("table missing %q:\n%s", want, got)
		}
	}
}

func TestTableViewUsesHorizontalViewport(t *testing.T) {
	got := tableView([]project.Project{
		{
			Name:         "very-long-project-name",
			StackDisplay: "SvelteKit+Cloudflare Workers+Tailwind",
			Activity:     ovwformat.ActivityInfo{Display: "2w"},
			Status:       ovwformat.StatusFromTags("", []string{"dirty", "stale"}),
			Note:         ovwformat.NoteInfo{Display: "this is a long note that should not overflow the table width"},
		},
	}, -1, 72, 0, 0, config.Default(), "activity", "desc")

	for _, line := range strings.Split(got, "\n") {
		if len([]rune(line)) > 72 {
			t.Fatalf("line width = %d, want <= 72:\n%s", len([]rune(line)), got)
		}
	}
	if !strings.Contains(got, "SvelteKit+Cloudflare Workers+Tailwind") {
		t.Fatalf("non-note columns should keep natural content before viewport clipping:\n%s", got)
	}
}

func TestTableViewCollapsesMultilineNotes(t *testing.T) {
	got := tableView([]project.Project{
		{
			Name:         "instaview",
			StackDisplay: "WXT",
			Activity:     ovwformat.ActivityInfo{Display: "3w"},
			Status:       ovwformat.StatusFromTags("", []string{"dirty"}),
			Note:         ovwformat.NoteInfo{Display: "fix: extract carousel\n- Add SJS script\n- Increase timeout"},
		},
	}, 0, 120, 8, 0, config.Default(), "activity", "desc")

	lines := strings.Split(stripANSI(got), "\n")
	if len(lines) != 3 {
		t.Fatalf("table should render one row per project, got %d lines:\n%s", len(lines), got)
	}
	if strings.Contains(got, "\n- Add") {
		t.Fatalf("table note should not contain embedded newlines:\n%s", got)
	}
	if !strings.Contains(got, "fix: extract carousel - Add SJS") {
		t.Fatalf("table note missing collapsed text:\n%s", got)
	}
}

func TestTableViewScrollsToSelectedRowWithinHeight(t *testing.T) {
	projects := make([]project.Project, 0, 20)
	for i := range 20 {
		projects = append(projects, project.Project{Name: fmt.Sprintf("project-%02d", i)})
	}

	got := tableView(projects, 15, 80, 8, 0, config.Default(), "activity", "desc")

	if !strings.Contains(got, "project-15") {
		t.Fatalf("selected row is not visible:\n%s", got)
	}
	if strings.Contains(got, "project-00") {
		t.Fatalf("table did not scroll away from first row:\n%s", got)
	}
	if strings.Contains(got, "project-19") {
		t.Fatalf("table rendered rows past viewport:\n%s", got)
	}
	if !strings.Contains(got, "Note") {
		t.Fatalf("table should keep note header plain:\n%s", got)
	}
}

func TestTableViewFollowsConfiguredColumns(t *testing.T) {
	cfg := config.Default()
	cfg.Columns = []string{"name", "path", "manager", "scripts", "version", "branch", "updated", "status"}
	updated := time.Date(2026, 5, 12, 9, 30, 0, 0, time.Local)
	got := tableView([]project.Project{
		{
			Name:      "eventca",
			Path:      "/tmp/eventca",
			Managers:  []string{"pnpm"},
			Scripts:   []string{"dev", "build"},
			Version:   "1.2.3",
			UpdatedAt: updated,
			Activity:  ovwformat.ActivityInfo{Branch: "feat/pins"},
			Status:    ovwformat.StatusFromTags("", []string{"active"}),
		},
	}, -1, 100, 0, 0, cfg, "name", "asc")

	for _, want := range []string{"Name ↑", "Path", "Manager", "Scripts", "Version", "Branch", "Updated", "Status", "eventca", "/tmp/eventca", "pnpm", "dev, build", "1.2.3", "feat/pins", "2026-05-12 09:30", "active"} {
		if !strings.Contains(got, want) {
			t.Fatalf("table missing configured column value %q:\n%s", want, got)
		}
	}
	for _, notWant := range []string{"Stack", "Activity", "Note"} {
		if strings.Contains(got, notWant) {
			t.Fatalf("table should not include unconfigured column %q:\n%s", notWant, got)
		}
	}
}

func TestTableViewKeepsGitActivityWhenUpdatedVisible(t *testing.T) {
	cfg := config.Default()
	cfg.Columns = []string{"name", "updated", "activity"}
	updated := time.Date(2026, 5, 12, 9, 30, 0, 0, time.Local)
	item := project.Project{
		Name:      "ahead",
		UpdatedAt: updated,
		Activity: ovwformat.ActivityInfo{
			Display:       "3d ↑4",
			LastCommitAge: "3d",
			Unpushed:      4,
			HasGit:        true,
			HasCommits:    true,
		},
	}

	got := tableView([]project.Project{item}, -1, 120, 0, 0, cfg, "activity", "desc")

	for _, want := range []string{"Updated", "Activity", "2026-05-12 09:30", "3d ↑4"} {
		if !strings.Contains(got, want) {
			t.Fatalf("table missing %q:\n%s", want, got)
		}
	}
}

func TestTableViewHorizontallyScrollsConfiguredColumns(t *testing.T) {
	cfg := config.Default()
	cfg.Columns = []string{"name", "path", "manager", "version", "activity", "status", "note"}
	item := project.Project{
		Name:     "eventca",
		Path:     "/tmp/projects/web/eventca",
		Managers: []string{"pnpm"},
		Version:  "1.2.3",
		Activity: ovwformat.ActivityInfo{Display: "40m"},
		Status:   ovwformat.StatusFromTags("", []string{"active"}),
		Note:     ovwformat.NoteInfo{Display: strings.Repeat("n", 80)},
	}

	left := tableView([]project.Project{item}, -1, 40, 0, 0, cfg, "activity", "desc")
	right := tableView([]project.Project{item}, -1, 40, 0, 32, cfg, "activity", "desc")

	if !strings.Contains(left, "eventca") {
		t.Fatalf("left viewport should show early columns:\n%s", left)
	}
	if strings.Contains(left, "1.2.3") {
		t.Fatalf("left viewport should hide later columns before horizontal scroll:\n%s", left)
	}
	if !strings.Contains(right, "1.2.3") && !strings.Contains(right, "pnpm") {
		t.Fatalf("right viewport should reveal later columns:\n%s", right)
	}
	for _, line := range strings.Split(right, "\n") {
		if len([]rune(line)) > 40 {
			t.Fatalf("line width = %d, want <= 40:\n%s", len([]rune(line)), right)
		}
	}
	if strings.Contains(right, strings.Repeat("n", 60)) {
		t.Fatalf("note should remain capped in table:\n%s", right)
	}
}

func TestTableViewExpandsToViewport(t *testing.T) {
	got := tableView([]project.Project{
		{
			Name:         "speaklines",
			StackDisplay: "Node",
			Activity:     ovwformat.ActivityInfo{Display: "no commits"},
			Status:       ovwformat.StatusFromTags("parked", nil),
			Note:         ovwformat.NoteInfo{Display: "testing notes here"},
		},
	}, -1, 120, 8, 0, config.Default(), "activity", "desc")

	lines := strings.Split(stripANSI(got), "\n")
	if len(lines) < 3 {
		t.Fatalf("table missing rows:\n%s", got)
	}
	for _, line := range lines[:3] {
		if width := lipglossWidth(line); width != 120 {
			t.Fatalf("line width = %d, want 120: %q\n%s", width, line, got)
		}
	}
}

func TestExpandTableRowsDistributesExtraWidth(t *testing.T) {
	rows := []tableRow{{
		Cells: []tableCell{
			{Value: "name", Width: 4},
			{Value: "activity", Width: 8},
			{Value: "status", Width: 6},
		},
	}}

	expandTableRows(rows, 30)

	got := []int{rows[0].Cells[0].Width, rows[0].Cells[1].Width, rows[0].Cells[2].Width}
	if got[0] != 4 || got[1] <= 8 || got[2] <= 6 {
		t.Fatalf("extra width should be spread across trailing columns, got %#v", got)
	}
	if got[2]-6 == 30-tableLineWidthFromWidths([]int{4, 8, 6}) {
		t.Fatalf("extra width should not all go to the last column, got %#v", got)
	}
}

func stripANSI(value string) string {
	return ansiPattern.ReplaceAllString(value, "")
}

var ansiPattern = regexp.MustCompile(`\x1b\[[0-9;]*m`)
