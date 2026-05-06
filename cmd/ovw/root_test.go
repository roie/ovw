package ovw

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
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

func TestHelpTextDescriptions(t *testing.T) {
	cmd := NewRootCommand()
	if cmd.Short != "A terminal overview for your local projects" {
		t.Fatalf("Short = %q", cmd.Short)
	}
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--help"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	got := out.String()
	for _, want := range []string{
		"ovw scans your project folders and shows each project's stack, Git activity, status, and notes in one clean terminal view.",
		"add         Add a project manually",
		"cache       Manage ovw cache",
		"config      Manage ovw config",
		"hide        Hide a project from ovw",
		"remove      Hide a project from ovw without deleting files",
		"scan        Rescan configured roots",
		"set         Set project status or note",
		"show        Show project details",
		"unhide      Show a hidden project again",
		"unset       Clear project status or note",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("help output missing %q:\n%s", want, got)
		}
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

func TestSetUnsetShowCommands(t *testing.T) {
	home := t.TempDir()
	project := filepath.Join(t.TempDir(), "manual")
	if err := os.MkdirAll(project, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
	runCommand(t, []string{"add", project})

	runCommand(t, []string{"set", "manual", "--status", "active", "--note", "fix flow"})
	show := runCommand(t, []string{"show", "manual"})
	if !bytes.Contains([]byte(show), []byte("Status    active")) || !bytes.Contains([]byte(show), []byte("Note      fix flow")) {
		t.Fatalf("show output = %q", show)
	}

	runCommand(t, []string{"unset", "manual", "--note"})
	show = runCommand(t, []string{"show", "manual"})
	if bytes.Contains([]byte(show), []byte("fix flow")) {
		t.Fatalf("note was not unset: %q", show)
	}

	runCommand(t, []string{"unset", "manual", "--status"})
	show = runCommand(t, []string{"show", "manual"})
	if bytes.Contains([]byte(show), []byte("active")) {
		t.Fatalf("status was not unset: %q", show)
	}
}

func TestSetRejectsUnknownStatus(t *testing.T) {
	home := t.TempDir()
	project := filepath.Join(t.TempDir(), "manual")
	if err := os.MkdirAll(project, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
	runCommand(t, []string{"add", project})

	_, err := executeCommand([]string{"set", "manual", "--status", "building"})
	if err == nil {
		t.Fatal("expected unknown status error")
	}
	if !strings.Contains(err.Error(), "Unknown status: building") {
		t.Fatalf("error = %v", err)
	}
}

func TestSetCanTargetScannedProjectByName(t *testing.T) {
	home := t.TempDir()
	root := t.TempDir()
	t.Setenv("HOME", home)
	if err := os.MkdirAll(filepath.Join(root, "scanned"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "scanned", "go.mod"), []byte("module scanned"), 0o644); err != nil {
		t.Fatal(err)
	}
	configForTest(t, root)

	runCommand(t, []string{"set", "scanned", "--status", "active"})
	show := runCommand(t, []string{"show", "scanned"})
	if !bytes.Contains([]byte(show), []byte("Status    active")) {
		t.Fatalf("show output = %q", show)
	}
}

func runCommand(t *testing.T, args []string) string {
	t.Helper()
	out, err := executeCommand(args)
	if err != nil {
		t.Fatalf("Execute(%v) error = %v", args, err)
	}
	return out
}

func executeCommand(args []string) (string, error) {
	cmd := NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return out.String(), err
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
