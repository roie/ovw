package scanner

import (
	"os"
	"path/filepath"
	"testing"

	"ovw/internal/config"
	"ovw/internal/metadata"
)

func TestScanDetectsConfiguredMarkers(t *testing.T) {
	root := t.TempDir()
	touch(t, filepath.Join(root, "nodeapp", "package.json"))
	touch(t, filepath.Join(root, "rustapp", "Cargo.toml"))
	touch(t, filepath.Join(root, "goapp", "go.mod"))
	touch(t, filepath.Join(root, "pyapp", "pyproject.toml"))
	touch(t, filepath.Join(root, "readme", "README.md"))

	projects, err := Scan(configForRoot(root), metadata.New())
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}

	names := projectNames(projects)
	for _, want := range []string{"goapp", "nodeapp", "pyapp", "rustapp"} {
		if !names[want] {
			t.Fatalf("missing project %q in %#v", want, names)
		}
	}
	if names["readme"] {
		t.Fatalf("README-only folder detected as project")
	}
}

func TestScanIgnoresConfiguredDirectories(t *testing.T) {
	root := t.TempDir()
	touch(t, filepath.Join(root, "node_modules", "dep", "package.json"))

	projects, err := Scan(configForRoot(root), metadata.New())
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}
	if len(projects) != 0 {
		t.Fatalf("projects = %#v", projects)
	}
}

func TestScanStopsAtProjectWhenNestedDisabled(t *testing.T) {
	root := t.TempDir()
	touch(t, filepath.Join(root, "mono", "package.json"))
	touch(t, filepath.Join(root, "mono", "child", "go.mod"))

	cfg := configForRoot(root)
	cfg.ScanNestedProjects = false
	projects, err := Scan(cfg, metadata.New())
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}
	names := projectNames(projects)
	if !names["mono"] || names["child"] {
		t.Fatalf("names = %#v", names)
	}
}

func TestScanNestedFalseDoesNotCreateWorkspaceChildRows(t *testing.T) {
	root := t.TempDir()
	touch(t, filepath.Join(root, "mono", "package.json"))
	touch(t, filepath.Join(root, "mono", "apps", "web", "package.json"))

	cfg := configForRoot(root)
	cfg.ScanNestedProjects = false
	projects, err := Scan(cfg, metadata.New())
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}
	names := projectNames(projects)
	if !names["mono"] || names["web"] {
		t.Fatalf("names = %#v", names)
	}
}

func TestScanSupportsMaxDepth(t *testing.T) {
	root := t.TempDir()
	touch(t, filepath.Join(root, "level1", "level2", "package.json"))
	cfg := configForRoot(root)
	cfg.MaxDepth = 1

	projects, err := Scan(cfg, metadata.New())
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}
	if len(projects) != 0 {
		t.Fatalf("projects = %#v", projects)
	}
}

func TestScanIncludesManualOutsideRoots(t *testing.T) {
	root := t.TempDir()
	outside := filepath.Join(t.TempDir(), "manual")
	if err := os.MkdirAll(outside, 0o755); err != nil {
		t.Fatal(err)
	}
	store := metadata.New()
	canonical, _, err := store.Set(outside, metadata.Entry{Manual: true})
	if err != nil {
		t.Fatal(err)
	}

	projects, err := Scan(configForRoot(root), store)
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}
	if len(projects) != 1 {
		t.Fatalf("projects = %#v", projects)
	}
	if projects[0].Path != canonical || !projects[0].Manual {
		t.Fatalf("project = %#v", projects[0])
	}
}

func TestDetectRootRequiresTwoProjectChildren(t *testing.T) {
	root := t.TempDir()
	touch(t, filepath.Join(root, "one", "package.json"))
	if DetectCurrentRoot(root, config.Default().ProjectMarkers) {
		t.Fatal("detected root with only one project child")
	}
	touch(t, filepath.Join(root, "two", "go.mod"))
	if !DetectCurrentRoot(root, config.Default().ProjectMarkers) {
		t.Fatal("did not detect root with two project children")
	}
}

func configForRoot(root string) config.Config {
	cfg := config.Default()
	cfg.Roots = []string{root}
	cfg.IgnoreDirs = append(cfg.IgnoreDirs, "node_modules")
	return cfg
}

func touch(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func projectNames(projects []Project) map[string]bool {
	out := map[string]bool{}
	for _, project := range projects {
		out[project.Name] = true
	}
	return out
}
