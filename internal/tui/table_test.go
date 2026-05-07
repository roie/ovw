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
			Status:       ovwformat.StatusFromTags("", []string{"dirty", "stale"}),
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
			Status:       ovwformat.StatusFromTags("", []string{"dirty", "unpushed", "stale"}),
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

func TestTableViewCollapsesMultilineNotes(t *testing.T) {
	got := tableView([]project.Project{
		{
			Name:         "instaview",
			StackDisplay: "WXT",
			Activity:     ovwformat.ActivityInfo{Display: "3w"},
			Status:       ovwformat.StatusFromTags("", []string{"dirty"}),
			Note:         ovwformat.NoteInfo{Display: "fix: extract carousel\n- Add SJS script\n- Increase timeout"},
		},
	}, 0, 120, 8)

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
	if !strings.Contains(got, "Note") {
		t.Fatalf("table should keep note header plain:\n%s", got)
	}
}

func stripANSI(value string) string {
	return ansiPattern.ReplaceAllString(value, "")
}

var ansiPattern = regexp.MustCompile(`\x1b\[[0-9;]*m`)
