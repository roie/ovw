package app

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ovw/internal/config"
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

func TestRefreshClearsCacheBeforeRendering(t *testing.T) {
	home := t.TempDir()
	root := t.TempDir()
	t.Setenv("HOME", home)
	writePackage(t, filepath.Join(root, "app"), `{"description":"fresh description"}`)
	paths, err := config.Paths()
	if err != nil {
		t.Fatal(err)
	}
	cfg := config.Default()
	cfg.Roots = []string{root}
	if err := config.Write(paths.Config, cfg); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(paths.Cache), 0o755); err != nil {
		t.Fatal(err)
	}
	staleCache := `{"projects":{` + quote(filepath.Join(root, "app")) + `:{"description":"stale description"}}}`
	if err := os.WriteFile(paths.Cache, []byte(staleCache), 0o644); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	err = Run(Options{Refresh: true, Plain: true, Cwd: root, Out: &out, In: strings.NewReader("\n")})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	got := out.String()
	if !strings.Contains(got, "fresh description") || strings.Contains(got, "stale description") {
		t.Fatalf("refresh output = %s", got)
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
