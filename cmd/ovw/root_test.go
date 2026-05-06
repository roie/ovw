package ovw

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"ovw/internal/config"
)

func TestHelpIncludesUsage(t *testing.T) {
	cmd := NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--help"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	got := out.String()
	if !bytes.Contains([]byte(got), []byte("Usage:")) {
		t.Fatalf("help output missing Usage: %q", got)
	}
	if !bytes.Contains([]byte(got), []byte("ovw")) {
		t.Fatalf("help output missing command name: %q", got)
	}
}

func TestConfigPathCommand(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", "")

	cmd := NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"config", "path"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	want := filepath.Join(home, ".config", "ovw", "config.toml") + "\n"
	if out.String() != want {
		t.Fatalf("config path = %q, want %q", out.String(), want)
	}
}

func TestCacheClearCommand(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CACHE_HOME", "")
	cachePath := filepath.Join(home, ".cache", "ovw", "projects.json")
	if err := os.MkdirAll(filepath.Dir(cachePath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cachePath, []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}

	cmd := NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"cache", "clear"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if out.String() != "Cache cleared.\n" {
		t.Fatalf("output = %q", out.String())
	}
	if _, err := os.Stat(cachePath); !os.IsNotExist(err) {
		t.Fatalf("cache still exists or stat failed unexpectedly: %v", err)
	}
}

func TestAddHideUnhideRemoveCommands(t *testing.T) {
	home := t.TempDir()
	project := filepath.Join(t.TempDir(), "manual")
	if err := os.MkdirAll(project, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)

	runCommand(t, []string{"add", project})
	metaPath := filepath.Join(home, ".local", "share", "ovw", "projects.json")
	data, err := os.ReadFile(metaPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(data, []byte(`"manual": true`)) {
		t.Fatalf("metadata after add = %s", string(data))
	}

	out := runCommand(t, []string{"hide", project})
	if !bytes.Contains([]byte(out), []byte("No files were deleted.")) {
		t.Fatalf("hide output = %q", out)
	}
	if _, err := os.Stat(project); err != nil {
		t.Fatalf("project was deleted: %v", err)
	}

	runCommand(t, []string{"unhide", project})
	data, err = os.ReadFile(metaPath)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(data, []byte(`"hidden": true`)) {
		t.Fatalf("metadata after unhide = %s", string(data))
	}

	runCommand(t, []string{"remove", project})
	data, err = os.ReadFile(metaPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(data, []byte(`"hidden": true`)) {
		t.Fatalf("metadata after remove = %s", string(data))
	}
}

func TestScanCommand(t *testing.T) {
	home := t.TempDir()
	root := t.TempDir()
	t.Setenv("HOME", home)
	if err := os.MkdirAll(filepath.Join(root, "app"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "app", "go.mod"), []byte("module app"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := configForTest(t, root)
	_ = cfg

	out := runCommand(t, []string{"scan"})
	if !bytes.Contains([]byte(out), []byte("Scanning")) || !bytes.Contains([]byte(out), []byte("Found 1 projects")) {
		t.Fatalf("scan output = %q", out)
	}
}

func runCommand(t *testing.T, args []string) string {
	t.Helper()
	cmd := NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs(args)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute(%v) error = %v", args, err)
	}
	return out.String()
}

func configForTest(t *testing.T, root string) string {
	t.Helper()
	paths, err := config.Paths()
	if err != nil {
		t.Fatal(err)
	}
	cfg := config.Default()
	cfg.Roots = []string{root}
	if err := config.Write(paths.Config, cfg); err != nil {
		t.Fatal(err)
	}
	return paths.Config
}
