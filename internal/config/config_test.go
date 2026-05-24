package config

import (
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestDefaultConfigValues(t *testing.T) {
	cfg := Default()

	if !reflect.DeepEqual(cfg.Roots, []string{"~/Code", "~/Projects"}) {
		t.Fatalf("Roots = %#v", cfg.Roots)
	}
	if cfg.MaxDepth != 0 {
		t.Fatalf("MaxDepth = %d", cfg.MaxDepth)
	}
	if cfg.ScanNestedProjects {
		t.Fatal("ScanNestedProjects = true")
	}
	if cfg.StaleDays != 30 {
		t.Fatalf("StaleDays = %d", cfg.StaleDays)
	}
	if !reflect.DeepEqual(cfg.Statuses, []string{"active", "parked", "shipped", "idea"}) {
		t.Fatalf("Statuses = %#v", cfg.Statuses)
	}
	if !reflect.DeepEqual(cfg.Columns, []string{"name", "stack", "activity", "status", "note"}) {
		t.Fatalf("Columns = %#v", cfg.Columns)
	}
	if cfg.SortBy != "activity" || cfg.SortDir != "desc" {
		t.Fatalf("sort = %q %q", cfg.SortBy, cfg.SortDir)
	}
	if cfg.Stack.Aliases["Cloudflare Workers"] != "CF" {
		t.Fatalf("Cloudflare alias = %q", cfg.Stack.Aliases["Cloudflare Workers"])
	}
	if cfg.Keys.Actions.Details != "enter" || cfg.Keys.Actions.Editor != "o" || cfg.Keys.Actions.Terminal != "t" || cfg.Keys.Actions.Hide != "x" {
		t.Fatalf("action keys = %#v", cfg.Keys.Actions)
	}
}

func TestPathHelpersUseHome(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("XDG_DATA_HOME", "")
	t.Setenv("XDG_CACHE_HOME", "")

	paths, err := Paths()
	if err != nil {
		t.Fatalf("Paths() error = %v", err)
	}

	if paths.Config != filepath.Join(home, ".config", "ovw", "config.toml") {
		t.Fatalf("Config path = %q", paths.Config)
	}
	if paths.Metadata != filepath.Join(home, ".local", "share", "ovw", "projects.json") {
		t.Fatalf("Metadata path = %q", paths.Metadata)
	}
}

func TestLoadWriteRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	cfg := Default()
	cfg.Roots = []string{"~/dev"}
	cfg.StaleDays = 7
	cfg.ColumnOrder = []string{"name", "path", "stack"}
	cfg.Fields = []FieldConfig{
		{ID: "jira", Label: "Jira", Type: "text"},
		{ID: "priority", Label: "Priority", Type: "select", Options: []string{"high", "medium", "low"}},
	}
	cfg.Keys.Actions.Editor = "e"
	cfg.Stack.Aliases["Cloudflare Workers"] = "Workers"

	if err := Write(path, cfg); err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if !reflect.DeepEqual(loaded.Roots, cfg.Roots) {
		t.Fatalf("Roots = %#v", loaded.Roots)
	}
	if loaded.StaleDays != 7 {
		t.Fatalf("StaleDays = %d", loaded.StaleDays)
	}
	if !reflect.DeepEqual(loaded.ColumnOrder, cfg.ColumnOrder) {
		t.Fatalf("ColumnOrder = %#v", loaded.ColumnOrder)
	}
	if !reflect.DeepEqual(loaded.Fields, cfg.Fields) {
		t.Fatalf("Fields = %#v", loaded.Fields)
	}
	if loaded.Keys.Actions.Editor != "e" {
		t.Fatalf("Editor key = %q", loaded.Keys.Actions.Editor)
	}
	if loaded.Stack.Aliases["Cloudflare Workers"] != "Workers" {
		t.Fatalf("Alias = %q", loaded.Stack.Aliases["Cloudflare Workers"])
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("config not written: %v", err)
	}
}

func TestLoadExistingConfigDefaultsMissingDetailsShortcut(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	text := `
roots = ["~/dev"]

[keys.actions]
editor = "o"
terminal = "t"
runner = "r"
note = "n"
status = "m"
pin = "p"
hide = "x"
`
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if loaded.Keys.Actions.Details != "enter" {
		t.Fatalf("Details key = %q, want enter", loaded.Keys.Actions.Details)
	}
}

func TestLoadRejectsInvalidConfigValues(t *testing.T) {
	tests := []struct {
		name string
		edit func(Config) Config
		want string
	}{
		{
			name: "empty roots",
			edit: func(cfg Config) Config {
				cfg.Roots = nil
				return cfg
			},
			want: "invalid roots: expected at least one root",
		},
		{
			name: "unknown column",
			edit: func(cfg Config) Config {
				cfg.Columns = []string{"name", "url"}
				return cfg
			},
			want: `invalid column "url": expected name, path, stack, manager, scripts, version, ports, branch, updated, activity, status, note, or field:<id>`,
		},
		{
			name: "known custom field column",
			edit: func(cfg Config) Config {
				cfg.Fields = []FieldConfig{{ID: "jira", Label: "Jira", Type: "text"}}
				cfg.Columns = []string{"name", "field:jira"}
				return cfg
			},
			want: "",
		},
		{
			name: "unknown column order",
			edit: func(cfg Config) Config {
				cfg.ColumnOrder = []string{"name", "url"}
				return cfg
			},
			want: `invalid column_order "url": expected name, path, stack, manager, scripts, version, ports, branch, updated, activity, status, note, or field:<id>`,
		},
		{
			name: "duplicate column order",
			edit: func(cfg Config) Config {
				cfg.ColumnOrder = []string{"name", "status", "name"}
				return cfg
			},
			want: `invalid column_order "name": already used`,
		},
		{
			name: "unknown custom field column",
			edit: func(cfg Config) Config {
				cfg.Columns = []string{"name", "field:jira"}
				return cfg
			},
			want: `invalid column "field:jira": custom field is not defined`,
		},
		{
			name: "empty custom field id",
			edit: func(cfg Config) Config {
				cfg.Fields = []FieldConfig{{ID: "", Label: "Jira", Type: "text"}}
				return cfg
			},
			want: `invalid fields[0].id: expected lowercase letters, numbers, underscores, or hyphens`,
		},
		{
			name: "duplicate custom field id",
			edit: func(cfg Config) Config {
				cfg.Fields = []FieldConfig{{ID: "jira", Label: "Jira", Type: "text"}, {ID: "jira", Label: "Issue", Type: "text"}}
				return cfg
			},
			want: `invalid fields[1].id "jira": already used`,
		},
		{
			name: "unknown custom field type",
			edit: func(cfg Config) Config {
				cfg.Fields = []FieldConfig{{ID: "jira", Label: "Jira", Type: "number"}}
				return cfg
			},
			want: `invalid fields[0].type "number": expected text, select, or checkbox`,
		},
		{
			name: "missing custom field type",
			edit: func(cfg Config) Config {
				cfg.Fields = []FieldConfig{{ID: "jira", Label: "Jira"}}
				return cfg
			},
			want: `invalid fields[0].type "": expected text, select, or checkbox`,
		},
		{
			name: "unknown sort",
			edit: func(cfg Config) Config {
				cfg.SortBy = "recent"
				return cfg
			},
			want: `invalid sort_by "recent": expected activity, updated, name, or status`,
		},
		{
			name: "unknown sort dir",
			edit: func(cfg Config) Config {
				cfg.SortDir = "down"
				return cfg
			},
			want: `invalid sort_dir "down": expected asc or desc`,
		},
		{
			name: "empty action key",
			edit: func(cfg Config) Config {
				cfg.Keys.Actions.Editor = ""
				return cfg
			},
			want: `invalid keys.actions.editor: expected a key`,
		},
		{
			name: "reserved action key",
			edit: func(cfg Config) Config {
				cfg.Keys.Actions.Editor = "?"
				return cfg
			},
			want: `invalid keys.actions.editor "?": key is reserved`,
		},
		{
			name: "duplicate action key",
			edit: func(cfg Config) Config {
				cfg.Keys.Actions.Terminal = "o"
				return cfg
			},
			want: `invalid keys.actions.terminal "o": already used by editor`,
		},
		{
			name: "enter allowed for action key",
			edit: func(cfg Config) Config {
				cfg.Keys.Actions.Details = "d"
				cfg.Keys.Actions.Terminal = "enter"
				return cfg
			},
		},
		{
			name: "duplicate enter action key",
			edit: func(cfg Config) Config {
				cfg.Keys.Actions.Terminal = "enter"
				return cfg
			},
			want: `invalid keys.actions.terminal "enter": already used by details`,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.toml")
			if err := Write(path, tc.edit(Default())); err != nil {
				t.Fatalf("Write() error = %v", err)
			}
			_, err := Load(path)
			if tc.want == "" {
				if err != nil {
					t.Fatalf("Load() error = %v", err)
				}
				return
			}
			want := "invalid config " + path + ": " + tc.want
			if err == nil || err.Error() != want {
				t.Fatalf("Load() error = %v, want %q", err, want)
			}
		})
	}
}

func TestExpandPathRejectsAmbiguousHomePaths(t *testing.T) {
	for _, path := range []string{"", "~roie/projects"} {
		if got, err := ExpandPath(path); err == nil {
			t.Fatalf("ExpandPath(%q) = %q, nil error; want error", path, got)
		}
	}
}

func TestLoadIncludesPathForMalformedConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte("roots = ["), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := Load(path)
	if err == nil {
		t.Fatal("Load() error = nil")
	}
	if !strings.Contains(err.Error(), "invalid config "+path+":") {
		t.Fatalf("Load() error should include config path: %v", err)
	}
}

func TestEnsureWritesCommentedDefaultConfigThatParses(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	root := t.TempDir()
	path := filepath.Join(t.TempDir(), "config.toml")
	var out strings.Builder

	_, created, err := Ensure(path, "", strings.NewReader(root+"\n"), &out)
	if err != nil {
		t.Fatalf("Ensure() error = %v", err)
	}
	if !created {
		t.Fatal("created = false")
	}
	if !strings.Contains(out.String(), "Project root to scan [~/Projects]: ") {
		t.Fatalf("first run prompt = %q", out.String())
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	text := string(data)
	if !strings.Contains(text, "# ovw — A terminal overview for your local projects.") {
		t.Fatalf("default config missing comments:\n%s", text)
	}
	if !strings.Contains(text, `"Cloudflare Workers" = "CF"`) {
		t.Fatalf("stack alias key is not a quoted string:\n%s", text)
	}
	if !strings.Contains(text, `columns = ["name", "stack", "activity", "status", "note"]`) {
		t.Fatalf("default config missing status column:\n%s", text)
	}
	if !strings.Contains(text, "# Options: name, path, stack, manager, scripts, version, ports, branch, updated, activity, status, note, field:<id>") {
		t.Fatalf("default config missing column options comment:\n%s", text)
	}
	if !strings.Contains(text, "[keys.actions]") || !strings.Contains(text, `details = "enter"`) || !strings.Contains(text, `editor = "o"`) || !strings.Contains(text, `hide = "x"`) {
		t.Fatalf("default config missing keyboard shortcuts:\n%s", text)
	}
	if strings.Contains(text, "show_untagged") || strings.Contains(text, "relative_dates") {
		t.Fatalf("default config contains stale display fields:\n%s", text)
	}
	if strings.Contains(text, "Phase 2") || strings.Contains(text, "future TUI actions") {
		t.Fatalf("default config contains future placeholder text:\n%s", text)
	}
	if strings.Contains(text, "[cache]") {
		t.Fatalf("default config contains cache section:\n%s", text)
	}
	if !strings.Contains(text, "# Suggested statuses. Status is free-form.") {
		t.Fatalf("default config missing suggested status comment:\n%s", text)
	}
	if !strings.Contains(text, "# Commands used by TUI open and terminal actions.") {
		t.Fatalf("default config missing editor/shell action comment:\n%s", text)
	}
	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load() generated config error = %v\n%s", err, text)
	}
	if !reflect.DeepEqual(loaded.Roots, []string{root}) {
		t.Fatalf("Roots = %#v", loaded.Roots)
	}
}

func TestLoadAppliesFieldColumnDefaults(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	text := `
roots = ["~/dev"]
columns = ["name"]
sort_by = "activity"
sort_dir = "desc"

[[fields]]
id = "jira"
label = "Jira"
type = "text"
column = true
`
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if !reflect.DeepEqual(cfg.Columns, []string{"name", "field:jira"}) {
		t.Fatalf("Columns = %#v", cfg.Columns)
	}
}

func TestEnsurePromptsWithDetectedRootDefault(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	dev := filepath.Join(home, "dev")
	for _, child := range []string{"one", "two"} {
		if err := os.MkdirAll(filepath.Join(dev, child), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dev, child, "go.mod"), []byte("module "+child+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	path := filepath.Join(t.TempDir(), "config.toml")
	var out strings.Builder

	cfg, created, err := Ensure(path, dev, strings.NewReader("\n"), &out)
	if err != nil {
		t.Fatalf("Ensure() error = %v", err)
	}
	if !created {
		t.Fatal("created = false")
	}
	if !strings.Contains(out.String(), "Project root to scan ["+dev+"]: ") {
		t.Fatalf("first run prompt = %q", out.String())
	}
	if !reflect.DeepEqual(cfg.Roots, []string{dev}) {
		t.Fatalf("Roots = %#v, want %q", cfg.Roots, dev)
	}
}

func TestEnsureExistingConfigDoesNotRewriteComments(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	original := strings.Replace(DefaultTemplate([]string{"~/Projects"}), "# ovw — A terminal overview for your local projects.", "# custom user comment", 1)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}

	_, created, err := Ensure(path, t.TempDir(), strings.NewReader("\n"), io.Discard)
	if err != nil {
		t.Fatalf("Ensure() error = %v", err)
	}
	if created {
		t.Fatal("created = true")
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != original {
		t.Fatalf("existing config was rewritten:\n%s", string(after))
	}
}
