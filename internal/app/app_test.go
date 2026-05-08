package app

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ovw/internal/config"
	"ovw/internal/metadata"
)

func TestOverviewJSONScansAndRendersProject(t *testing.T) {
	home := t.TempDir()
	root := t.TempDir()
	t.Setenv("HOME", home)
	writePackage(t, filepath.Join(root, "app"), `{"dependencies":{"@sveltejs/kit":"latest","wrangler":"latest"}}`)
	paths, err := config.Paths()
	if err != nil {
		t.Fatal(err)
	}
	cfg := config.Default()
	cfg.Roots = []string{root}
	if err := config.Write(paths.Config, cfg); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	err = Run(Options{JSON: true, Cwd: root, Out: &out, In: strings.NewReader("\n")})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	got := out.String()
	if !strings.HasPrefix(strings.TrimSpace(got), "[") {
		t.Fatalf("json output has prefix/logs: %q", got)
	}
	for _, want := range []string{`"name": "app"`, `"SvelteKit"`, `"Cloudflare Workers"`} {
		if !strings.Contains(got, want) {
			t.Fatalf("json output missing %q = %s", want, got)
		}
	}
	if strings.Contains(got, "stack_display") {
		t.Fatalf("json output = %s", got)
	}
}

func TestRunRejectsConflictingOutputModes(t *testing.T) {
	var out bytes.Buffer
	err := Run(Options{Plain: true, JSON: true, Out: &out, In: strings.NewReader("\n")})

	if err == nil || err.Error() != "choose only one output mode: --plain or --json" {
		t.Fatalf("Run() error = %v", err)
	}
	if out.Len() != 0 {
		t.Fatalf("Run() wrote output on invalid modes: %q", out.String())
	}
}

func TestOverviewPlainAppliesStatusFilter(t *testing.T) {
	home := t.TempDir()
	root := t.TempDir()
	t.Setenv("HOME", home)
	writePackage(t, filepath.Join(root, "app"), `{}`)
	writePackage(t, filepath.Join(root, "other"), `{}`)
	paths, err := config.Paths()
	if err != nil {
		t.Fatal(err)
	}
	cfg := config.Default()
	cfg.Roots = []string{root}
	if err := config.Write(paths.Config, cfg); err != nil {
		t.Fatal(err)
	}
	metadataPath := filepath.Join(home, ".local", "share", "ovw", "projects.json")
	appPath, err := filepath.Abs(filepath.Join(root, "app"))
	if err != nil {
		t.Fatal(err)
	}
	meta := `{"projects":{` + quote(appPath) + `:{"status":"active"}}}`
	if err := os.MkdirAll(filepath.Dir(metadataPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(metadataPath, []byte(meta), 0o644); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	err = Run(Options{Status: "active", Plain: true, Cwd: root, Out: &out, In: strings.NewReader("\n")})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	got := out.String()
	if !strings.Contains(got, "app") || strings.Contains(got, "other") {
		t.Fatalf("table output = %s", got)
	}
}

func TestOverviewCanFilterHiddenProjects(t *testing.T) {
	home := t.TempDir()
	root := t.TempDir()
	t.Setenv("HOME", home)
	writePackage(t, filepath.Join(root, "app"), `{}`)
	writePackage(t, filepath.Join(root, "hidden"), `{}`)
	paths, err := config.Paths()
	if err != nil {
		t.Fatal(err)
	}
	cfg := config.Default()
	cfg.Roots = []string{root}
	if err := config.Write(paths.Config, cfg); err != nil {
		t.Fatal(err)
	}
	metadataPath := filepath.Join(home, ".local", "share", "ovw", "projects.json")
	hiddenPath, err := filepath.Abs(filepath.Join(root, "hidden"))
	if err != nil {
		t.Fatal(err)
	}
	meta := `{"projects":{` + quote(hiddenPath) + `:{"hidden":true}}}`
	if err := os.MkdirAll(filepath.Dir(metadataPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(metadataPath, []byte(meta), 0o644); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	err = Run(Options{Hidden: true, Plain: true, Cwd: root, Out: &out, In: strings.NewReader("\n")})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	got := out.String()
	if !strings.Contains(got, "hidden") || strings.Contains(got, "app") {
		t.Fatalf("table output = %s", got)
	}
}

func TestLoadOverviewReturnsFilteredProjects(t *testing.T) {
	home := t.TempDir()
	root := t.TempDir()
	t.Setenv("HOME", home)
	writePackage(t, filepath.Join(root, "app"), `{}`)
	writePackage(t, filepath.Join(root, "other"), `{}`)
	paths, err := config.Paths()
	if err != nil {
		t.Fatal(err)
	}
	cfg := config.Default()
	cfg.Roots = []string{root}
	if err := config.Write(paths.Config, cfg); err != nil {
		t.Fatal(err)
	}
	appPath, err := filepath.Abs(filepath.Join(root, "app"))
	if err != nil {
		t.Fatal(err)
	}
	if err := metadata.Write(paths.Metadata, metadata.Store{
		Projects: map[string]metadata.Entry{
			appPath: {Status: "active"},
		},
	}); err != nil {
		t.Fatal(err)
	}

	result, err := LoadOverview(Options{Status: "active", Cwd: root, In: strings.NewReader("\n")})
	if err != nil {
		t.Fatalf("LoadOverview() error = %v", err)
	}
	if len(result.Projects) != 1 || result.Projects[0].Name != "app" {
		t.Fatalf("projects = %#v", result.Projects)
	}
	if result.Config.Roots[0] != root {
		t.Fatalf("config roots = %#v", result.Config.Roots)
	}
}

func TestLoadOverviewRejectsInvalidSort(t *testing.T) {
	home := t.TempDir()
	root := t.TempDir()
	t.Setenv("HOME", home)
	writePackage(t, filepath.Join(root, "app"), `{}`)
	paths, err := config.Paths()
	if err != nil {
		t.Fatal(err)
	}
	cfg := config.Default()
	cfg.Roots = []string{root}
	if err := config.Write(paths.Config, cfg); err != nil {
		t.Fatal(err)
	}

	_, err = LoadOverview(Options{Sort: "recent", Cwd: root, In: strings.NewReader("\n")})
	if err == nil || err.Error() != `invalid sort "recent": expected activity, name, or status` {
		t.Fatalf("LoadOverview() error = %v", err)
	}
}

func TestLoadOverviewAcceptsInlineSortDirection(t *testing.T) {
	home := t.TempDir()
	root := t.TempDir()
	t.Setenv("HOME", home)
	writePackage(t, filepath.Join(root, "alpha"), `{}`)
	writePackage(t, filepath.Join(root, "beta"), `{}`)
	paths, err := config.Paths()
	if err != nil {
		t.Fatal(err)
	}
	cfg := config.Default()
	cfg.Roots = []string{root}
	cfg.SortDir = "asc"
	if err := config.Write(paths.Config, cfg); err != nil {
		t.Fatal(err)
	}

	result, err := LoadOverview(Options{Sort: "name:desc", Cwd: root, In: strings.NewReader("\n")})
	if err != nil {
		t.Fatalf("LoadOverview() error = %v", err)
	}
	if len(result.Projects) != 2 || result.Projects[0].Name != "beta" {
		t.Fatalf("projects = %#v", result.Projects)
	}
}

func TestCheckConfigReturnsCandidatesWhenMissing(t *testing.T) {
	home := t.TempDir()
	root := filepath.Join(home, "dev")
	t.Setenv("HOME", home)
	writePackage(t, filepath.Join(root, "app"), `{}`)
	writePackage(t, filepath.Join(root, "api"), `{}`)

	setup, err := CheckConfig(Options{Cwd: root})
	if err != nil {
		t.Fatalf("CheckConfig() error = %v", err)
	}
	if setup.Exists {
		t.Fatal("Exists = true")
	}
	if len(setup.Candidates) == 0 || setup.Candidates[0] != root {
		t.Fatalf("Candidates = %#v, want first %q", setup.Candidates, root)
	}
}

func TestCreateConfigWritesDefaultConfig(t *testing.T) {
	home := t.TempDir()
	root := t.TempDir()
	t.Setenv("HOME", home)

	paths, cfg, err := CreateConfig(root)
	if err != nil {
		t.Fatalf("CreateConfig() error = %v", err)
	}
	if len(cfg.Roots) != 1 || cfg.Roots[0] != root {
		t.Fatalf("Roots = %#v, want %q", cfg.Roots, root)
	}
	data, err := os.ReadFile(paths.Config)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "# ovw — A terminal overview for your local projects.") || !strings.Contains(string(data), quote(root)) {
		t.Fatalf("config not written from default template:\n%s", string(data))
	}
}

func TestLoadOverviewHandlesLargeProjectSet(t *testing.T) {
	home := t.TempDir()
	root := t.TempDir()
	t.Setenv("HOME", home)
	for i := range 120 {
		writePackage(t, filepath.Join(root, fmt.Sprintf("project-%03d", i)), `{}`)
	}
	paths, err := config.Paths()
	if err != nil {
		t.Fatal(err)
	}
	cfg := config.Default()
	cfg.Roots = []string{root}
	if err := config.Write(paths.Config, cfg); err != nil {
		t.Fatal(err)
	}

	result, err := LoadOverview(Options{Sort: "name:asc", Cwd: root, In: strings.NewReader("\n")})
	if err != nil {
		t.Fatalf("LoadOverview() error = %v", err)
	}
	if len(result.Projects) != 120 {
		t.Fatalf("projects = %d, want 120", len(result.Projects))
	}
	if result.Projects[0].Name != "project-000" || result.Projects[119].Name != "project-119" {
		t.Fatalf("projects not sorted by name: first=%q last=%q", result.Projects[0].Name, result.Projects[119].Name)
	}
}

func TestResolveProjectFindsScannedProjectByName(t *testing.T) {
	root := t.TempDir()
	writePackage(t, filepath.Join(root, "app"), `{}`)
	cfg := config.Default()
	cfg.Roots = []string{root}

	got, err := ResolveProject("app", cfg, metadata.New())
	if err != nil {
		t.Fatalf("ResolveProject() error = %v", err)
	}
	want, err := metadata.CanonicalPath(filepath.Join(root, "app"))
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("ResolveProject() = %q, want %q", got, want)
	}
}

func TestUpdateProjectMetadataWritesStore(t *testing.T) {
	home := t.TempDir()
	root := t.TempDir()
	t.Setenv("HOME", home)
	writePackage(t, filepath.Join(root, "app"), `{}`)
	paths, err := config.Paths()
	if err != nil {
		t.Fatal(err)
	}
	cfg := config.Default()
	cfg.Roots = []string{root}
	if err := config.Write(paths.Config, cfg); err != nil {
		t.Fatal(err)
	}

	status := "parked"
	note := "user note"
	result, err := UpdateProjectMetadata("app", MetadataUpdate{Status: &status, Note: &note})
	if err != nil {
		t.Fatalf("UpdateProjectMetadata() error = %v", err)
	}
	if result.Entry.Status != status || result.Entry.Note != note {
		t.Fatalf("entry = %#v", result.Entry)
	}
	store, err := metadata.Load(paths.Metadata)
	if err != nil {
		t.Fatal(err)
	}
	got := store.Projects[result.Path]
	if got.Status != status || got.Note != note {
		t.Fatalf("stored entry = %#v", got)
	}
	data, err := os.ReadFile(paths.Metadata)
	if err != nil {
		t.Fatal(err)
	}
	for _, generated := range []string{
		"stack",
		"stack_display",
		"managers",
		"version",
		"activity",
		"last_commit_at",
		"last_commit_message",
		"branch",
		"dirty",
		"unpushed",
		"has_git",
		"has_commits",
		"description",
	} {
		if strings.Contains(string(data), generated) {
			t.Fatalf("metadata stored generated field %q: %s", generated, data)
		}
	}
}

func TestAddProjectWritesConfigRoot(t *testing.T) {
	home := t.TempDir()
	project := filepath.Join(t.TempDir(), "custom")
	if err := os.MkdirAll(project, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)

	result, err := AddProject(project)
	if err != nil {
		t.Fatalf("AddProject() error = %v", err)
	}
	if result.AlreadyTracked {
		t.Fatal("AlreadyTracked = true, want false")
	}
	cfg, err := config.Load(filepath.Join(home, ".config", "ovw", "config.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Roots) != 1 || cfg.Roots[0] != project {
		t.Fatalf("roots = %#v, want %q", cfg.Roots, project)
	}
}

func TestAddProjectReportsAlreadyTrackedRoot(t *testing.T) {
	home := t.TempDir()
	project := filepath.Join(t.TempDir(), "custom")
	if err := os.MkdirAll(project, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)

	if _, err := AddProject(project); err != nil {
		t.Fatalf("AddProject() first error = %v", err)
	}
	result, err := AddProject(project)
	if err != nil {
		t.Fatalf("AddProject() second error = %v", err)
	}
	if !result.AlreadyTracked {
		t.Fatal("AlreadyTracked = false, want true")
	}
}

func writePackage(t *testing.T, dir, data string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
}

func quote(s string) string {
	return `"` + strings.ReplaceAll(s, `\`, `\\`) + `"`
}
