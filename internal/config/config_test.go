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
	if loaded.Stack.Aliases["Cloudflare Workers"] != "Workers" {
		t.Fatalf("Alias = %q", loaded.Stack.Aliases["Cloudflare Workers"])
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("config not written: %v", err)
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
			want: `invalid column "url": expected name, path, stack, manager, scripts, version, ports, branch, updated, activity, status, or note`,
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
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.toml")
			if err := Write(path, tc.edit(Default())); err != nil {
				t.Fatalf("Write() error = %v", err)
			}
			_, err := Load(path)
			want := "invalid config " + path + ": " + tc.want
			if err == nil || err.Error() != want {
				t.Fatalf("Load() error = %v, want %q", err, want)
			}
		})
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
	if !strings.Contains(text, "# Options: name, path, stack, manager, scripts, version, ports, branch, updated, activity, status, note") {
		t.Fatalf("default config missing column options comment:\n%s", text)
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
