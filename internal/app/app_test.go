package app

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"ovw/internal/config"
	"ovw/internal/gitactivity"
	"ovw/internal/metadata"
	"ovw/internal/scanner"
)

func TestOverviewJSONScansAndRendersProject(t *testing.T) {
	home := t.TempDir()
	root := t.TempDir()
	t.Setenv("HOME", home)
	appPath := filepath.Join(root, "app")
	writePackage(t, appPath, `{"dependencies":{"@sveltejs/kit":"latest","wrangler":"latest"}}`)
	paths, err := config.Paths()
	if err != nil {
		t.Fatal(err)
	}
	cfg := config.Default()
	cfg.Roots = []string{root}
	if err := config.Write(paths.Config, cfg); err != nil {
		t.Fatal(err)
	}
	withPortDetector(t, func(paths []string) map[string][]int {
		t.Fatalf("port detector should not run for JSON unless ports column is enabled: %#v", paths)
		return nil
	})

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
	if strings.Contains(got, `"ports": [`) {
		t.Fatalf("json output should not include detected ports unless ports column is enabled: %s", got)
	}
	if strings.Contains(got, "stack_display") {
		t.Fatalf("json output = %s", got)
	}
}

func TestDiscoverOverviewReturnsPlaceholderProjects(t *testing.T) {
	home := t.TempDir()
	root := t.TempDir()
	t.Setenv("HOME", home)
	writePackage(t, filepath.Join(root, "app"), `{"dependencies":{"@sveltejs/kit":"latest"}}`)
	paths, err := config.Paths()
	if err != nil {
		t.Fatal(err)
	}
	cfg := config.Default()
	cfg.Roots = []string{root}
	if err := config.Write(paths.Config, cfg); err != nil {
		t.Fatal(err)
	}

	result, err := DiscoverOverview(Options{Cwd: root, In: strings.NewReader("\n")})
	if err != nil {
		t.Fatalf("DiscoverOverview() error = %v", err)
	}
	if len(result.Projects) != 1 || len(result.Scanned) != 1 {
		t.Fatalf("discovered projects=%#v scanned=%#v", result.Projects, result.Scanned)
	}
	project := result.Projects[0]
	if project.Name != "app" || project.Path == "" {
		t.Fatalf("placeholder project = %#v", project)
	}
	if project.StackDisplay != "" || project.Activity.Display != "" || project.Status.Display != "" || project.Note.Display != "" {
		t.Fatalf("placeholder should not be enriched yet: %#v", project)
	}
}

func TestSessionRootScansNestedProjectsWithoutConfig(t *testing.T) {
	home := t.TempDir()
	root := t.TempDir()
	t.Setenv("HOME", home)
	writePackage(t, root, `{}`)
	writePackage(t, filepath.Join(root, "services", "api"), `{}`)

	result, err := LoadOverview(Options{SessionRoot: root, Plain: true, Cwd: root, In: strings.NewReader("\n")})
	if err != nil {
		t.Fatalf("LoadOverview() error = %v", err)
	}
	if len(result.Projects) != 2 {
		t.Fatalf("projects = %#v, want root and nested service", result.Projects)
	}
	names := []string{result.Projects[0].Name, result.Projects[1].Name}
	if !containsString(names, filepath.Base(root)) || !containsString(names, "api") {
		t.Fatalf("project names = %#v, want root and api", names)
	}
	paths, err := config.Paths()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(paths.Config); !os.IsNotExist(err) {
		t.Fatalf("session root should not create config, stat err = %v", err)
	}
	if !result.Config.ScanNestedProjects {
		t.Fatal("session root should enable nested project scanning")
	}
}

func TestSessionRootSkipsFirstRunSetup(t *testing.T) {
	home := t.TempDir()
	root := t.TempDir()
	t.Setenv("HOME", home)

	setup, err := CheckConfig(Options{SessionRoot: root, Cwd: root})
	if err != nil {
		t.Fatalf("CheckConfig() error = %v", err)
	}
	if !setup.Exists {
		t.Fatal("session root should behave like config exists")
	}
}

func TestCountProjectRootsUsesDiscoveryOnly(t *testing.T) {
	root := t.TempDir()
	writePackage(t, filepath.Join(root, "app"), `{}`)
	writePackage(t, filepath.Join(root, "node_modules", "ignored"), `{}`)

	counts := CountProjectRoots([]string{root})
	if counts[root] != 1 {
		t.Fatalf("count = %d, want 1", counts[root])
	}
}

func TestCountProjectRootsWithConfigUsesScanRules(t *testing.T) {
	root := t.TempDir()
	writePackage(t, filepath.Join(root, "app"), `{}`)
	writePackage(t, filepath.Join(root, "nested", "app"), `{}`)

	cfg := config.Default()
	cfg.MaxDepth = 1
	counts := CountProjectRootsWithConfig([]string{root}, cfg)
	if counts[root] != 1 {
		t.Fatalf("count = %d, want 1", counts[root])
	}
}

func TestOverviewJSONTimingWritesOnlyToErr(t *testing.T) {
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

	var out bytes.Buffer
	var errOut bytes.Buffer
	err = Run(Options{JSON: true, Timing: true, Cwd: root, Out: &out, Err: &errOut, In: strings.NewReader("\n")})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if !strings.HasPrefix(strings.TrimSpace(out.String()), "[") {
		t.Fatalf("json stdout is not pure JSON: %q", out.String())
	}
	for _, want := range []string{"timing: total", "timing: config", "timing: discover", "timing: enrich", "timing: ports skipped", "timing: filter/sort"} {
		if !strings.Contains(errOut.String(), want) {
			t.Fatalf("timing stderr missing %q:\n%s", want, errOut.String())
		}
	}
}

func TestLoadOverviewTimingDoesNotWrite(t *testing.T) {
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

	var errOut bytes.Buffer
	result, err := LoadOverview(Options{Plain: true, Timing: true, Cwd: root, Err: &errOut, In: strings.NewReader("\n")})
	if err != nil {
		t.Fatalf("LoadOverview() error = %v", err)
	}
	if errOut.Len() != 0 {
		t.Fatalf("LoadOverview() wrote timing before render: %q", errOut.String())
	}
	if result.Timing.Total <= 0 {
		t.Fatalf("LoadOverview() did not record total timing: %s", result.Timing.Total)
	}
}

func TestOverviewJSONDetectsPortsWhenColumnVisible(t *testing.T) {
	home := t.TempDir()
	root := t.TempDir()
	t.Setenv("HOME", home)
	appPath := filepath.Join(root, "app")
	writePackage(t, appPath, `{}`)
	paths, err := config.Paths()
	if err != nil {
		t.Fatal(err)
	}
	cfg := config.Default()
	cfg.Roots = []string{root}
	cfg.Columns = []string{"name", "ports"}
	if err := config.Write(paths.Config, cfg); err != nil {
		t.Fatal(err)
	}
	withPortDetector(t, func(paths []string) map[string][]int {
		return map[string][]int{appPath: []int{3000}}
	})

	var out bytes.Buffer
	if err := Run(Options{JSON: true, Cwd: root, Out: &out, In: strings.NewReader("\n")}); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if !strings.Contains(out.String(), `"ports": [`) || !strings.Contains(out.String(), "3000") {
		t.Fatalf("json output missing ports when ports column enabled: %s", out.String())
	}
}

func TestProjectFromPathDetectsPorts(t *testing.T) {
	path := filepath.Join(t.TempDir(), "app")
	store := metadata.New()
	store.Projects[path] = metadata.Entry{Fields: map[string]string{"jira": "OVW-123"}}
	withPortDetector(t, func(paths []string) map[string][]int {
		return map[string][]int{path: []int{5173}}
	})

	project := ProjectFromPath(path, config.Default(), store, time.Now())
	if len(project.Ports) != 1 || project.Ports[0] != 5173 {
		t.Fatalf("ports = %#v, want 5173", project.Ports)
	}
	if project.Fields["jira"] != "OVW-123" {
		t.Fatalf("fields = %#v, want jira field", project.Fields)
	}
}

func TestLoadOverviewSkipsPortDetectionWhenColumnHidden(t *testing.T) {
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
	cfg.Columns = []string{"name", "stack", "activity", "status", "note"}
	if err := config.Write(paths.Config, cfg); err != nil {
		t.Fatal(err)
	}
	withPortDetector(t, func(paths []string) map[string][]int {
		t.Fatalf("port detector should not run when ports column is hidden: %#v", paths)
		return nil
	})

	if _, err := LoadOverview(Options{Plain: true, Cwd: root, In: strings.NewReader("\n")}); err != nil {
		t.Fatalf("LoadOverview() error = %v", err)
	}
}

func TestLoadOverviewDetectsPortsWhenColumnVisible(t *testing.T) {
	home := t.TempDir()
	root := t.TempDir()
	t.Setenv("HOME", home)
	appPath := filepath.Join(root, "app")
	writePackage(t, appPath, `{}`)
	paths, err := config.Paths()
	if err != nil {
		t.Fatal(err)
	}
	cfg := config.Default()
	cfg.Roots = []string{root}
	cfg.Columns = []string{"name", "ports"}
	if err := config.Write(paths.Config, cfg); err != nil {
		t.Fatal(err)
	}
	called := false
	withPortDetector(t, func(paths []string) map[string][]int {
		called = true
		return map[string][]int{appPath: []int{5173}}
	})

	result, err := LoadOverview(Options{Plain: true, Cwd: root, In: strings.NewReader("\n")})
	if err != nil {
		t.Fatalf("LoadOverview() error = %v", err)
	}
	if !called {
		t.Fatal("port detector was not called")
	}
	if len(result.Projects) != 1 || len(result.Projects[0].Ports) != 1 || result.Projects[0].Ports[0] != 5173 {
		t.Fatalf("ports = %#v", result.Projects)
	}
}

func TestEnrichSetsUpdatedAtFromRecentlyModifiedFiles(t *testing.T) {
	root := t.TempDir()
	appPath := filepath.Join(root, "app")
	writePackage(t, appPath, `{}`)
	if err := os.MkdirAll(filepath.Join(appPath, "src"), 0o755); err != nil {
		t.Fatal(err)
	}
	newer := time.Date(2026, 5, 12, 9, 30, 0, 0, time.Local)
	olderCommit := newer.Add(-48 * time.Hour)
	sourcePath := filepath.Join(appPath, "src", "main.go")
	if err := os.WriteFile(sourcePath, []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(filepath.Join(appPath, "package.json"), olderCommit, olderCommit); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(sourcePath, newer, newer); err != nil {
		t.Fatal(err)
	}
	cfg := config.Default()

	got := EnrichWithGit(scanner.Project{Name: "app", Path: appPath}, cfg, newer, gitactivity.Info{
		HasGit:       true,
		HasCommits:   true,
		LastCommitAt: olderCommit,
	})

	if !got.UpdatedAt.Equal(newer) {
		t.Fatalf("UpdatedAt = %s, want %s", got.UpdatedAt, newer)
	}
	if !got.Activity.LastCommitAt.Equal(olderCommit) {
		t.Fatalf("Activity.LastCommitAt = %s, want %s", got.Activity.LastCommitAt, olderCommit)
	}
}

func TestLoadOverviewSkipsUpdatedWhenColumnHidden(t *testing.T) {
	home := t.TempDir()
	root := t.TempDir()
	t.Setenv("HOME", home)
	appPath := filepath.Join(root, "app")
	writePackage(t, appPath, `{}`)
	if err := os.WriteFile(filepath.Join(appPath, "main.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	paths, err := config.Paths()
	if err != nil {
		t.Fatal(err)
	}
	cfg := config.Default()
	cfg.Roots = []string{root}
	cfg.Columns = []string{"name", "activity"}
	cfg.SortBy = "activity"
	if err := config.Write(paths.Config, cfg); err != nil {
		t.Fatal(err)
	}

	result, err := LoadOverview(Options{Plain: true, Cwd: root, In: strings.NewReader("\n")})
	if err != nil {
		t.Fatalf("LoadOverview() error = %v", err)
	}
	if len(result.Projects) != 1 {
		t.Fatalf("projects = %#v", result.Projects)
	}
	if !result.Projects[0].UpdatedAt.IsZero() {
		t.Fatalf("UpdatedAt = %s, want zero when updated is hidden", result.Projects[0].UpdatedAt)
	}
}

func TestLoadOverviewDetectsUpdatedWhenColumnVisible(t *testing.T) {
	home := t.TempDir()
	root := t.TempDir()
	t.Setenv("HOME", home)
	appPath := filepath.Join(root, "app")
	writePackage(t, appPath, `{}`)
	if err := os.WriteFile(filepath.Join(appPath, "main.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	paths, err := config.Paths()
	if err != nil {
		t.Fatal(err)
	}
	cfg := config.Default()
	cfg.Roots = []string{root}
	cfg.Columns = []string{"name", "updated"}
	cfg.SortBy = "activity"
	if err := config.Write(paths.Config, cfg); err != nil {
		t.Fatal(err)
	}

	result, err := LoadOverview(Options{Plain: true, Cwd: root, In: strings.NewReader("\n")})
	if err != nil {
		t.Fatalf("LoadOverview() error = %v", err)
	}
	if len(result.Projects) != 1 {
		t.Fatalf("projects = %#v", result.Projects)
	}
	if result.Projects[0].UpdatedAt.IsZero() {
		t.Fatal("UpdatedAt is zero when updated column is visible")
	}
}

func TestLoadOverviewDetectsUpdatedWhenSortedByUpdated(t *testing.T) {
	home := t.TempDir()
	root := t.TempDir()
	t.Setenv("HOME", home)
	appPath := filepath.Join(root, "app")
	writePackage(t, appPath, `{}`)
	if err := os.WriteFile(filepath.Join(appPath, "main.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	paths, err := config.Paths()
	if err != nil {
		t.Fatal(err)
	}
	cfg := config.Default()
	cfg.Roots = []string{root}
	cfg.Columns = []string{"name", "activity"}
	cfg.SortBy = "activity"
	if err := config.Write(paths.Config, cfg); err != nil {
		t.Fatal(err)
	}

	result, err := LoadOverview(Options{Plain: true, Sort: "updated", Cwd: root, In: strings.NewReader("\n")})
	if err != nil {
		t.Fatalf("LoadOverview() error = %v", err)
	}
	if len(result.Projects) != 1 {
		t.Fatalf("projects = %#v", result.Projects)
	}
	if result.Projects[0].UpdatedAt.IsZero() {
		t.Fatal("UpdatedAt is zero when sorting by updated")
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

func TestRunRejectsTimingWithoutPlainOrJSON(t *testing.T) {
	var out bytes.Buffer
	var errOut bytes.Buffer
	err := Run(Options{Timing: true, Out: &out, Err: &errOut, In: strings.NewReader("\n")})

	if err == nil || err.Error() != "--timing requires --plain or --json" {
		t.Fatalf("Run() error = %v", err)
	}
	if out.Len() != 0 || errOut.Len() != 0 {
		t.Fatalf("Run() wrote output on invalid timing: out=%q err=%q", out.String(), errOut.String())
	}
}

func TestFormatDurationShowsSubMillisecondValues(t *testing.T) {
	if got := formatDuration(time.Nanosecond); got != "<1ms" {
		t.Fatalf("formatDuration() = %q, want <1ms", got)
	}
	if got := formatDuration(0); got != "0ms" {
		t.Fatalf("formatDuration(0) = %q, want 0ms", got)
	}
	if got := formatDuration(1500 * time.Millisecond); got != "1500ms" {
		t.Fatalf("formatDuration(1500ms) = %q, want 1500ms", got)
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

func TestOverviewPlainAppliesPathFilter(t *testing.T) {
	home := t.TempDir()
	root := t.TempDir()
	t.Setenv("HOME", home)
	writePackage(t, filepath.Join(root, "client-a", "app"), `{}`)
	writePackage(t, filepath.Join(root, "client-b", "library1-name"), `{}`)
	writePackage(t, filepath.Join(root, "library1", "api"), `{}`)
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
	err = Run(Options{Path: "library1", Plain: true, Cwd: root, Out: &out, In: strings.NewReader("\n")})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	got := out.String()
	if !strings.Contains(got, "api") {
		t.Fatalf("table output missing path match: %s", got)
	}
	if strings.Contains(got, "app") || strings.Contains(got, "library1-name") {
		t.Fatalf("path filter should not match non-path projects by name: %s", got)
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
	if err == nil || err.Error() != `invalid sort "recent": expected activity, updated, name, or status` {
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

func TestResolveProjectReturnsScannerError(t *testing.T) {
	cfg := config.Default()
	cfg.Roots = []string{"~other/projects"}

	_, err := ResolveProject("missing", cfg, metadata.New())
	if err == nil {
		t.Fatal("ResolveProject() error = nil")
	}
	if !strings.Contains(err.Error(), "unsupported home path") {
		t.Fatalf("ResolveProject() error = %v, want scanner error", err)
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
	pinned := true
	script := "go run ."
	jira := "OVW-123"
	result, err := UpdateProjectMetadata("app", MetadataUpdate{
		Status:  &status,
		Note:    &note,
		Pinned:  &pinned,
		Scripts: map[string]*string{"run": &script},
		Fields:  map[string]*string{"jira": &jira},
	})
	if err != nil {
		t.Fatalf("UpdateProjectMetadata() error = %v", err)
	}
	if result.Entry.Status != status || result.Entry.Note != note || !result.Entry.Pinned || result.Entry.Scripts["run"] != script || result.Entry.Fields["jira"] != jira {
		t.Fatalf("entry = %#v", result.Entry)
	}
	store, err := metadata.Load(paths.Metadata)
	if err != nil {
		t.Fatal(err)
	}
	got := store.Projects[result.Path]
	if got.Status != status || got.Note != note || !got.Pinned || got.Scripts["run"] != script || got.Fields["jira"] != jira {
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

func TestUpdateProjectMetadataRefusesCorruptedMetadata(t *testing.T) {
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
	corrupt := []byte(`{"projects":`)
	if err := os.MkdirAll(filepath.Dir(paths.Metadata), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(paths.Metadata, corrupt, 0o644); err != nil {
		t.Fatal(err)
	}

	status := "parked"
	_, err = UpdateProjectMetadata("app", MetadataUpdate{Status: &status})
	if err == nil {
		t.Fatal("UpdateProjectMetadata() error = nil")
	}
	data, readErr := os.ReadFile(paths.Metadata)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(data) != string(corrupt) {
		t.Fatalf("corrupted metadata was overwritten: %s", data)
	}
}

func TestSetProjectHiddenRefusesCorruptedMetadata(t *testing.T) {
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
	corrupt := []byte(`{"projects":`)
	if err := os.MkdirAll(filepath.Dir(paths.Metadata), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(paths.Metadata, corrupt, 0o644); err != nil {
		t.Fatal(err)
	}

	_, err = SetProjectHidden("app", true)
	if err == nil {
		t.Fatal("SetProjectHidden() error = nil")
	}
	data, readErr := os.ReadFile(paths.Metadata)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(data) != string(corrupt) {
		t.Fatalf("corrupted metadata was overwritten: %s", data)
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

func TestAddProjectStoresCanonicalRootForRelativePath(t *testing.T) {
	home := t.TempDir()
	parent := t.TempDir()
	project := filepath.Join(parent, "custom")
	if err := os.MkdirAll(project, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
	t.Chdir(parent)

	result, err := AddProject("./custom")
	if err != nil {
		t.Fatalf("AddProject() error = %v", err)
	}
	cfg, err := config.Load(filepath.Join(home, ".config", "ovw", "config.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Roots) != 1 || cfg.Roots[0] != result.Path {
		t.Fatalf("roots = %#v, want canonical %q", cfg.Roots, result.Path)
	}
}

func TestAddProjectPreservesExistingConfig(t *testing.T) {
	home := t.TempDir()
	project := filepath.Join(t.TempDir(), "custom")
	if err := os.MkdirAll(project, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)

	paths, err := config.Paths()
	if err != nil {
		t.Fatal(err)
	}
	cfg := config.Default()
	cfg.Roots = []string{"~/dev"}
	cfg.Columns = []string{"name", "path", "status"}
	cfg.ColumnOrder = []string{"path", "name", "status"}
	cfg.IgnoreDirs = []string{"node_modules", "generated"}
	cfg.RecentCommitsLimit = 7
	cfg.RecentFilesLimit = 8
	cfg.Keys.Actions.Sidepane = "ctrl+g"
	cfg.Fields = []config.FieldConfig{{ID: "owner", Label: "Owner", Type: "text"}}
	if err := config.Write(paths.Config, cfg); err != nil {
		t.Fatal(err)
	}

	result, err := AddProject(project)
	if err != nil {
		t.Fatalf("AddProject() error = %v", err)
	}
	if result.AlreadyTracked {
		t.Fatal("AlreadyTracked = true, want false")
	}
	loaded, err := config.Load(paths.Config)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.Roots) != 2 || loaded.Roots[0] != "~/dev" || loaded.Roots[1] != result.Path {
		t.Fatalf("Roots = %#v, want existing root plus %q", loaded.Roots, result.Path)
	}
	if !reflect.DeepEqual(loaded.Columns, cfg.Columns) {
		t.Fatalf("Columns = %#v, want %#v", loaded.Columns, cfg.Columns)
	}
	if !reflect.DeepEqual(loaded.ColumnOrder, cfg.ColumnOrder) {
		t.Fatalf("ColumnOrder = %#v, want %#v", loaded.ColumnOrder, cfg.ColumnOrder)
	}
	if !reflect.DeepEqual(loaded.IgnoreDirs, cfg.IgnoreDirs) {
		t.Fatalf("IgnoreDirs = %#v, want %#v", loaded.IgnoreDirs, cfg.IgnoreDirs)
	}
	if loaded.RecentCommitsLimit != cfg.RecentCommitsLimit {
		t.Fatalf("RecentCommitsLimit = %d, want %d", loaded.RecentCommitsLimit, cfg.RecentCommitsLimit)
	}
	if loaded.RecentFilesLimit != cfg.RecentFilesLimit {
		t.Fatalf("RecentFilesLimit = %d, want %d", loaded.RecentFilesLimit, cfg.RecentFilesLimit)
	}
	if loaded.Keys.Actions.Sidepane != cfg.Keys.Actions.Sidepane {
		t.Fatalf("Sidepane key = %q, want %q", loaded.Keys.Actions.Sidepane, cfg.Keys.Actions.Sidepane)
	}
	if !reflect.DeepEqual(loaded.Fields, cfg.Fields) {
		t.Fatalf("Fields = %#v, want %#v", loaded.Fields, cfg.Fields)
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

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func withPortDetector(t *testing.T, detector func([]string) map[string][]int) {
	t.Helper()
	previous := detectPorts
	detectPorts = detector
	t.Cleanup(func() {
		detectPorts = previous
	})
}

func quote(s string) string {
	return `"` + strings.ReplaceAll(s, `\`, `\\`) + `"`
}
