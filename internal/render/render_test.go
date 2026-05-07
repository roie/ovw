package render

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"ovw/internal/config"
	"ovw/internal/format"
	"ovw/internal/project"
)

func TestTableContainsHeaderAndColumns(t *testing.T) {
	projects := []project.Project{{
		Name:         "eventca",
		StackDisplay: "SvelteKit+CF",
		Activity:     format.ActivityInfo{Display: "2d ↑2"},
		Tags:         []string{"dirty", "stale", "shipped"},
		Status:       "shipped",
		Note:         format.NoteInfo{Display: "feat/checkin · fix"},
	}}
	var out bytes.Buffer
	if err := Table(&out, projects, config.Default(), 200*time.Millisecond); err != nil {
		t.Fatalf("Table() error = %v", err)
	}
	got := out.String()
	for _, want := range []string{"ovw — 1 projects · scanned in 0.2s", "Name", "Stack", "Activity", "Status", "Note", "----", "eventca", "SvelteKit+CF", "2d ↑2", "dirty · stale · shipped"} {
		if !strings.Contains(got, want) {
			t.Fatalf("table missing %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "----  -----") {
		t.Fatalf("table uses disconnected column separators:\n%s", got)
	}
	for _, unwanted := range []string{"NAME", "STACK", "ACTIVITY", "STATUS", "NOTE", "2d ↑2 !"} {
		if strings.Contains(got, unwanted) {
			t.Fatalf("table contains uppercase header %q:\n%s", unwanted, got)
		}
	}
}

func TestTableDisambiguatesDuplicateNames(t *testing.T) {
	projects := []project.Project{
		{
			Name:         "vibe-oke",
			Path:         "/home/roie/dev/web/vibe-oke",
			StackDisplay: "Node",
			Activity:     format.ActivityInfo{Display: "1d"},
		},
		{
			Name:         "vibe-oke",
			Path:         "/home/roie/dev/playground/vibe-oke",
			StackDisplay: "Hono+Bun",
			Activity:     format.ActivityInfo{Display: "2d"},
		},
	}
	var out bytes.Buffer
	if err := Table(&out, projects, config.Default(), 100*time.Millisecond); err != nil {
		t.Fatalf("Table() error = %v", err)
	}
	got := out.String()
	for _, want := range []string{"web/vibe-oke", "playground/vibe-oke"} {
		if !strings.Contains(got, want) {
			t.Fatalf("table missing disambiguated name %q:\n%s", want, got)
		}
	}
}

func TestTableWidthTruncatesNoteAndSeparator(t *testing.T) {
	projects := []project.Project{{
		Name:         "imagio",
		Path:         "/tmp/imagio",
		StackDisplay: "WXT+Bun",
		Activity:     format.ActivityInfo{Display: "2w !"},
		Note:         format.NoteInfo{Display: "release/refactor · refactor: split repo into workspaces and packages"},
	}}
	var out bytes.Buffer
	if err := TableWithWidth(&out, projects, config.Default(), 100*time.Millisecond, 72); err != nil {
		t.Fatalf("TableWithWidth() error = %v", err)
	}
	lines := strings.Split(strings.TrimRight(out.String(), "\n"), "\n")
	if len(lines) < 5 {
		t.Fatalf("table output too short:\n%s", out.String())
	}
	separator := lines[3]
	if len(separator) != 72 {
		t.Fatalf("separator width = %d, want 72:\n%s", len(separator), out.String())
	}
	for _, line := range lines[2:] {
		if len(line) > 72 {
			t.Fatalf("line width = %d, want <= 72:\n%s", len(line), out.String())
		}
	}
	if !strings.Contains(out.String(), "…") {
		t.Fatalf("table did not truncate with ellipsis:\n%s", out.String())
	}
}

func TestTableCollapsesMultilineNotes(t *testing.T) {
	projects := []project.Project{{
		Name:         "instaview",
		Path:         "/tmp/instaview",
		StackDisplay: "WXT",
		Activity:     format.ActivityInfo{Display: "3w"},
		Tags:         []string{"dirty"},
		Note:         format.NoteInfo{Display: "fix: extract carousel\n- Add SJS script\n- Increase timeout"},
	}}
	var out bytes.Buffer
	if err := TableWithWidth(&out, projects, config.Default(), 100*time.Millisecond, 120); err != nil {
		t.Fatalf("TableWithWidth() error = %v", err)
	}
	got := out.String()
	if strings.Contains(got, "\n- Add") {
		t.Fatalf("table note should not contain embedded newlines:\n%s", got)
	}
	if !strings.Contains(got, "fix: extract carousel - Add SJS") {
		t.Fatalf("table note missing collapsed text:\n%s", got)
	}
}

func TestTableSupportsManagerColumn(t *testing.T) {
	cfg := config.Default()
	cfg.Columns = []string{"name", "stack", "manager", "activity", "status", "note"}
	projects := []project.Project{{
		Name:         "desktop",
		Path:         "/tmp/desktop",
		StackDisplay: "SvelteKit",
		Managers:     []string{"pnpm", "cargo"},
		Activity:     format.ActivityInfo{Display: "1h"},
	}}

	var out bytes.Buffer
	if err := TableWithWidth(&out, projects, cfg, 100*time.Millisecond, 120); err != nil {
		t.Fatalf("TableWithWidth() error = %v", err)
	}
	got := out.String()
	for _, want := range []string{"Manager", "desktop", "pnpm, cargo"} {
		if !strings.Contains(got, want) {
			t.Fatalf("table missing %q:\n%s", want, got)
		}
	}
}

func TestJSONOutputsPureArray(t *testing.T) {
	lastCommitAt := time.Date(2026, 5, 6, 18, 10, 35, 0, time.FixedZone("EDT", -4*60*60))
	projects := []project.Project{{
		Name:         "eventca",
		Path:         "/tmp/eventca",
		Stack:        []string{"Go"},
		StackDisplay: "Go",
		Managers:     []string{"go modules"},
		Activity: format.ActivityInfo{
			Display:           "1d !",
			LastCommitAge:     "1d",
			LastCommitAt:      lastCommitAt,
			LastCommitMessage: "feat: add overview",
			Branch:            "main",
			Dirty:             true,
			Unpushed:          2,
			HasGit:            true,
			HasCommits:        true,
		},
		Status: "active",
		Tags:   []string{"dirty", "unpushed", "active"},
		Note:   format.NoteInfo{Display: "note", Source: "manual", Manual: "note"},
		Manual: true,
		Hidden: true,
	}}
	var out bytes.Buffer
	if err := JSON(&out, projects); err != nil {
		t.Fatalf("JSON() error = %v", err)
	}
	got := strings.TrimSpace(out.String())
	if !strings.HasPrefix(got, "[") || !strings.HasSuffix(got, "]") {
		t.Fatalf("json is not a pure array: %q", got)
	}
	var decoded []map[string]any
	if err := json.Unmarshal(out.Bytes(), &decoded); err != nil {
		t.Fatalf("invalid json: %v\n%s", err, out.String())
	}
	if len(decoded) != 1 || decoded[0]["name"] != "eventca" {
		t.Fatalf("decoded = %#v", decoded)
	}
	item := decoded[0]
	for _, unwanted := range []string{"stack_display", "manual", "hidden"} {
		if _, ok := item[unwanted]; ok {
			t.Fatalf("json includes internal field %q:\n%s", unwanted, out.String())
		}
	}
	if note, ok := item["note"].(string); !ok || note != "note" {
		t.Fatalf("note = %#v, want string note\n%s", item["note"], out.String())
	}
	managers, ok := item["managers"].([]any)
	if !ok || len(managers) != 1 || managers[0] != "go modules" {
		t.Fatalf("managers = %#v, want go modules\n%s", item["managers"], out.String())
	}
	tags, ok := item["tags"].([]any)
	if !ok || len(tags) != 3 || tags[0] != "dirty" || tags[1] != "unpushed" || tags[2] != "active" {
		t.Fatalf("tags = %#v, want dirty/unpushed/active\n%s", item["tags"], out.String())
	}
	activity, ok := item["activity"].(map[string]any)
	if !ok {
		t.Fatalf("activity = %#v", item["activity"])
	}
	for _, unwanted := range []string{"display", "last_commit_age"} {
		if _, ok := activity[unwanted]; ok {
			t.Fatalf("json includes display field %q:\n%s", unwanted, out.String())
		}
	}
	for _, want := range []string{"last_commit_at", "last_commit_message", "branch", "dirty", "unpushed", "has_git", "has_commits"} {
		if _, ok := activity[want]; !ok {
			t.Fatalf("json missing activity field %q:\n%s", want, out.String())
		}
	}
	if activity["last_commit_message"] != "feat: add overview" || activity["branch"] != "main" || activity["dirty"] != true {
		t.Fatalf("activity = %#v", activity)
	}
}
