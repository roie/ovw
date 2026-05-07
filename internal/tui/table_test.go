package tui

import (
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
	}, -1, 100)

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
	}, -1, 72)

	for _, line := range strings.Split(got, "\n") {
		if len([]rune(line)) > 72 {
			t.Fatalf("line width = %d, want <= 72:\n%s", len([]rune(line)), got)
		}
	}
	if !strings.Contains(got, "...") {
		t.Fatalf("table did not truncate long content:\n%s", got)
	}
}
