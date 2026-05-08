package tui

import (
	"fmt"
	"regexp"
	"strings"
	"testing"

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
	}, -1, 100, 0, 0, config.Default())

	for _, want := range []string{"Name", "Stack", "Activity", "Status", "Note", "eventca", "SvelteKit+CF", "40m", "dirty", "stale", "partial check-in"} {
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
			Status:       ovwformat.StatusFromTags("", []string{"dirty", "unpushed", "stale"}),
			Note:         ovwformat.NoteInfo{Display: "this is a long note that should not overflow the table width"},
		},
	}, -1, 72, 0, 0, config.Default())

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
	}, 0, 120, 8, 0, config.Default())

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

	got := tableView(projects, 15, 80, 8, 0, config.Default())

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
	cfg.Columns = []string{"name", "path", "manager", "version", "status"}
	got := tableView([]project.Project{
		{
			Name:     "eventca",
			Path:     "/tmp/eventca",
			Managers: []string{"pnpm"},
			Version:  "1.2.3",
			Status:   ovwformat.StatusFromTags("", []string{"active"}),
		},
	}, -1, 100, 0, 0, cfg)

	for _, want := range []string{"Name", "Path", "Manager", "Version", "Status", "eventca", "/tmp/eventca", "pnpm", "1.2.3", "active"} {
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

	left := tableView([]project.Project{item}, -1, 40, 0, 0, cfg)
	right := tableView([]project.Project{item}, -1, 40, 0, 32, cfg)

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

func stripANSI(value string) string {
	return ansiPattern.ReplaceAllString(value, "")
}

var ansiPattern = regexp.MustCompile(`\x1b\[[0-9;]*m`)
