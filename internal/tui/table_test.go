package tui

import (
	"fmt"
	"regexp"
	"strings"
	"testing"

	ovwformat "ovw/internal/format"
	"ovw/internal/project"
)

func TestTableViewRendersProjectColumns(t *testing.T) {
	got := tableView([]project.Project{
		{
			Name:         "eventca",
			StackDisplay: "SvelteKit+CF",
			Activity:     ovwformat.ActivityInfo{Display: "40m"},
			Tags:         []string{"dirty", "stale"},
			Note:         ovwformat.NoteInfo{Display: "partial check-in"},
		},
	}, -1, 100, 0)

	for _, want := range []string{"Name", "Stack", "Activity", "Status", "Note", "eventca", "SvelteKit+CF", "40m", "dirty", "stale", "partial check-in"} {
		if !strings.Contains(got, want) {
			t.Fatalf("table missing %q:\n%s", want, got)
		}
	}
}

func TestTableViewTruncatesToWidth(t *testing.T) {
	got := tableView([]project.Project{
		{
			Name:         "very-long-project-name",
			StackDisplay: "SvelteKit+Cloudflare Workers+Tailwind",
			Activity:     ovwformat.ActivityInfo{Display: "2w"},
			Tags:         []string{"dirty", "unpushed", "stale"},
			Note:         ovwformat.NoteInfo{Display: "this is a long note that should not overflow the table width"},
		},
	}, -1, 72, 0)

	for _, line := range strings.Split(got, "\n") {
		if len([]rune(line)) > 72 {
			t.Fatalf("line width = %d, want <= 72:\n%s", len([]rune(line)), got)
		}
	}
	if !strings.Contains(got, "...") {
		t.Fatalf("table did not truncate long content:\n%s", got)
	}
}

func TestTableViewScrollsToSelectedRowWithinHeight(t *testing.T) {
	projects := make([]project.Project, 0, 20)
	for i := range 20 {
		projects = append(projects, project.Project{Name: fmt.Sprintf("project-%02d", i)})
	}

	got := tableView(projects, 15, 80, 8)

	if !strings.Contains(got, "project-15") {
		t.Fatalf("selected row is not visible:\n%s", got)
	}
	if strings.Contains(got, "project-00") {
		t.Fatalf("table did not scroll away from first row:\n%s", got)
	}
	if strings.Contains(got, "project-19") {
		t.Fatalf("table rendered rows past viewport:\n%s", got)
	}
	if !strings.Contains(got, "16/20") {
		t.Fatalf("table missing scroll position:\n%s", got)
	}
}

func TestCompactTableViewOmitsNoteColumnForInlineDetail(t *testing.T) {
	got := compactTableView([]project.Project{
		{
			Name:         "eventca",
			StackDisplay: "SvelteKit+CF",
			Activity:     ovwformat.ActivityInfo{Display: "18m"},
			Tags:         []string{"dirty", "active"},
			Note:         ovwformat.NoteInfo{Display: "long note belongs in detail pane"},
		},
	}, 0, 72, 8)

	if strings.Contains(got, "Note") || strings.Contains(got, "long note") {
		t.Fatalf("compact table should omit note column:\n%s", got)
	}
	for _, want := range []string{"Name", "Stack", "Activity", "Status", "eventca", "SvelteKit+CF", "dirty"} {
		if !strings.Contains(got, want) {
			t.Fatalf("compact table missing %q:\n%s", want, got)
		}
	}
}

func TestCompactTableViewDoesNotConsumeAllAvailableWidth(t *testing.T) {
	got := compactTableView([]project.Project{
		{
			Name:         "eventca",
			StackDisplay: "SvelteKit+CF",
			Activity:     ovwformat.ActivityInfo{Display: "18m"},
			Tags:         []string{"dirty", "active"},
		},
	}, 0, 100, 8)

	lines := strings.Split(got, "\n")
	if len([]rune(lines[1])) > 78 {
		t.Fatalf("compact separator width = %d, want <= 78:\n%s", len([]rune(lines[1])), got)
	}
}

func stripANSI(value string) string {
	return ansiPattern.ReplaceAllString(value, "")
}

var ansiPattern = regexp.MustCompile(`\x1b\[[0-9;]*m`)
