package render

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/charmbracelet/x/ansi"

	"ovw/internal/config"
	"ovw/internal/format"
	"ovw/internal/project"
)

func TestTableContainsHeaderAndColumns(t *testing.T) {
	projects := []project.Project{{
		Name:         "eventca",
		StackDisplay: "SvelteKit+CF",
		Ports:        []int{3000},
		Activity:     format.ActivityInfo{Display: "2d ↑2"},
		Status:       format.StatusFromTags("shipped", []string{"dirty", "stale", "shipped"}),
		Note:         format.NoteInfo{Display: "feat/checkin · fix"},
	}}
	var out bytes.Buffer
	if err := Table(&out, projects, config.Default(), 200*time.Millisecond); err != nil {
		t.Fatalf("Table() error = %v", err)
	}
	got := out.String()
	for _, want := range []string{"ovw — 1 projects · scanned in 200ms", "Name", "Stack", "Activity", "Status", "Note", "----", "eventca", "SvelteKit+CF", "2d ↑2", "dirty · stale · shipped"} {
		if !strings.Contains(got, want) {
			t.Fatalf("table missing %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "Ports") || strings.Contains(got, "3000") {
		t.Fatalf("default table should not show ports:\n%s", got)
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
			StackDisplay: "Hono",
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
		StackDisplay: "WXT",
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
		if width := ansi.StringWidth(line); width > 72 {
			t.Fatalf("line width = %d, want <= 72:\n%s", width, out.String())
		}
	}
	if !strings.Contains(out.String(), "workspaces") {
		t.Fatalf("wrapped note should keep later words:\n%s", out.String())
	}
}

func TestTableTruncatesUnicodeWithoutBreakingUTF8(t *testing.T) {
	projects := []project.Project{{
		Name:         "emoji",
		Path:         "/tmp/emoji",
		StackDisplay: "Go",
		Activity:     format.ActivityInfo{Display: "1h"},
		Note:         format.NoteInfo{Display: "fix: keep emoji 🙂 intact while truncating"},
	}}
	var out bytes.Buffer
	if err := TableWithWidth(&out, projects, config.Default(), 100*time.Millisecond, 56); err != nil {
		t.Fatalf("TableWithWidth() error = %v", err)
	}
	got := out.String()
	if !utf8.ValidString(got) {
		t.Fatalf("table output is not valid UTF-8:\n%q", got)
	}
	if strings.ContainsRune(got, utf8.RuneError) {
		t.Fatalf("table output contains replacement rune after truncation:\n%s", got)
	}
	if !strings.Contains(got, "🙂") {
		t.Fatalf("table should keep valid unicode while wrapping:\n%s", got)
	}
}

func TestTableWidthConstrainsNonNoteColumns(t *testing.T) {
	cfg := config.Default()
	cfg.Columns = []string{"name", "path", "stack", "activity", "status", "note"}
	projects := []project.Project{{
		Name:         "very-long-project-name-that-will-not-fit",
		Path:         "/home/roie/dev/playground/very/deep/project/path/that/will/not/fit",
		StackDisplay: "SvelteKit+Cloudflare Workers+Tailwind+Vite",
		Activity:     format.ActivityInfo{Display: "12m"},
		Status:       format.StatusFromTags("", []string{"dirty"}),
		Note:         format.NoteInfo{Display: "small note"},
	}}
	var out bytes.Buffer
	if err := TableWithWidth(&out, projects, cfg, 100*time.Millisecond, 60); err != nil {
		t.Fatalf("TableWithWidth() error = %v", err)
	}
	got := out.String()
	for _, line := range strings.Split(strings.TrimRight(got, "\n"), "\n")[2:] {
		if width := ansi.StringWidth(line); width > 60 {
			t.Fatalf("line width = %d, want <= 60:\n%s", width, got)
		}
	}
	if !strings.Contains(got, "…") {
		t.Fatalf("table should truncate oversized non-note columns:\n%s", got)
	}
	if !strings.Contains(got, "dirty") {
		t.Fatalf("table should preserve short useful columns:\n%s", got)
	}
}

func TestTableCollapsesMultilineNotes(t *testing.T) {
	projects := []project.Project{{
		Name:         "instaview",
		Path:         "/tmp/instaview",
		StackDisplay: "WXT",
		Activity:     format.ActivityInfo{Display: "3w"},
		Status:       format.StatusFromTags("", []string{"dirty"}),
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

func TestTableWrapsPlainNoteColumn(t *testing.T) {
	projects := []project.Project{{
		Name:         "notes",
		StackDisplay: "Go",
		Activity:     format.ActivityInfo{Display: "1h"},
		Status:       format.StatusFromTags("", []string{"active"}),
		Note:         format.NoteInfo{Display: "this note should wrap across multiple rows instead of disappearing behind an ellipsis"},
	}}
	var out bytes.Buffer
	if err := TableWithWidth(&out, projects, config.Default(), 100*time.Millisecond, 52); err != nil {
		t.Fatalf("TableWithWidth() error = %v", err)
	}
	got := out.String()
	lines := strings.Split(strings.TrimRight(got, "\n"), "\n")
	if len(lines) < 6 {
		t.Fatalf("table should render note continuation rows:\n%s", got)
	}
	for _, line := range lines[2:] {
		if width := ansi.StringWidth(line); width > 52 {
			t.Fatalf("line width = %d, want <= 52:\n%s", width, got)
		}
	}
	if !strings.Contains(got, "instead") {
		t.Fatalf("wrapped note should keep later words:\n%s", got)
	}
	if strings.Contains(got, "instea\n") || strings.Contains(got, "disappea\n") {
		t.Fatalf("wrapped note should prefer word boundaries:\n%s", got)
	}
	if strings.Count(got, "notes") < 2 {
		t.Fatalf("note continuation rows should repeat project context:\n%s", got)
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

func TestTableSupportsCustomFieldColumns(t *testing.T) {
	cfg := config.Default()
	cfg.Fields = []config.FieldConfig{{ID: "jira", Label: "Jira", Type: "text"}}
	cfg.Columns = []string{"name", "field:jira"}
	projects := []project.Project{{
		Name:   "eventca",
		Path:   "/tmp/eventca",
		Fields: map[string]string{"jira": "OVW-123"},
	}}

	var out bytes.Buffer
	if err := TableWithWidth(&out, projects, cfg, 100*time.Millisecond, 120); err != nil {
		t.Fatalf("TableWithWidth() error = %v", err)
	}
	got := out.String()
	for _, want := range []string{"Name", "Jira", "eventca", "OVW-123"} {
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
		Scripts:      []string{"dev", "build"},
		Version:      "1.2.3",
		Ports:        []int{3000, 8787},
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
		Status:      format.StatusFromTags("active", []string{"dirty", "active"}),
		Description: "Project description",
		Note:        format.NoteInfo{Display: "note", Source: "user", Value: "note"},
		Fields:      map[string]string{"jira": "OVW-123"},
		Hidden:      true,
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
	for _, unwanted := range []string{"stack_display", "user", "hidden"} {
		if _, ok := item[unwanted]; ok {
			t.Fatalf("json includes internal field %q:\n%s", unwanted, out.String())
		}
	}
	if note, ok := item["note"].(string); !ok || note != "note" {
		t.Fatalf("note = %#v, want string note\n%s", item["note"], out.String())
	}
	fields, ok := item["fields"].(map[string]any)
	if !ok || fields["jira"] != "OVW-123" {
		t.Fatalf("fields = %#v, want jira field\n%s", item["fields"], out.String())
	}
	scripts, ok := item["scripts"].([]any)
	if !ok || len(scripts) != 2 || scripts[0] != "dev" || scripts[1] != "build" {
		t.Fatalf("scripts = %#v, want dev/build\n%s", item["scripts"], out.String())
	}
	if description, ok := item["description"].(string); !ok || description != "Project description" {
		t.Fatalf("description = %#v, want Project description\n%s", item["description"], out.String())
	}
	if version, ok := item["version"].(string); !ok || version != "1.2.3" {
		t.Fatalf("version = %#v, want 1.2.3\n%s", item["version"], out.String())
	}
	ports, ok := item["ports"].([]any)
	if !ok || len(ports) != 2 || ports[0] != float64(3000) || ports[1] != float64(8787) {
		t.Fatalf("ports = %#v, want 3000/8787\n%s", item["ports"], out.String())
	}
	managers, ok := item["managers"].([]any)
	if !ok || len(managers) != 1 || managers[0] != "go modules" {
		t.Fatalf("managers = %#v, want go modules\n%s", item["managers"], out.String())
	}
	tags, ok := item["tags"].([]any)
	if !ok || len(tags) != 2 || tags[0] != "dirty" || tags[1] != "active" {
		t.Fatalf("tags = %#v, want dirty/active\n%s", item["tags"], out.String())
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
	mustAppearInOrder(t, out.String(), []string{
		`"name":`,
		`"path":`,
		`"stack":`,
		`"managers":`,
		`"scripts":`,
		`"version":`,
		`"ports":`,
		`"activity":`,
		`"tags":`,
		`"status":`,
		`"description":`,
		`"note":`,
		`"fields":`,
	})
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
