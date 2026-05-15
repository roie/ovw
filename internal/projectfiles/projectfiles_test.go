package projectfiles

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestRecentReturnsRecentlyModifiedFiles(t *testing.T) {
	root := t.TempDir()
	now := time.Date(2026, 5, 12, 12, 0, 0, 0, time.Local)
	writeFileAt(t, root, "README.md", now.Add(-5*time.Minute))
	writeFileAt(t, root, "internal/app.go", now.Add(-2*time.Minute))
	writeFileAt(t, root, "dist/bundle.js", now.Add(-1*time.Minute))
	writeFileAt(t, root, ".git/config", now)

	got, err := Recent(root, Options{IgnoreDirs: []string{"dist"}}, 5)
	if err != nil {
		t.Fatalf("Recent() error = %v", err)
	}
	want := []string{"internal/app.go", "README.md"}
	if len(got) != len(want) {
		t.Fatalf("files = %#v, want %v", got, want)
	}
	for index := range want {
		if got[index].Path != want[index] {
			t.Fatalf("files = %#v, want %v", got, want)
		}
	}
}

func TestRecentSkipsDotDirsButKeepsDotFiles(t *testing.T) {
	root := t.TempDir()
	now := time.Date(2026, 5, 12, 12, 0, 0, 0, time.Local)
	writeFileContentAt(t, root, ".gitignore", "# keep dot file\n", now.Add(-2*time.Minute))
	writeFileAt(t, root, ".vite/cache.json", now.Add(-1*time.Minute))

	got, err := Recent(root, Options{}, 5)
	if err != nil {
		t.Fatalf("Recent() error = %v", err)
	}
	if len(got) != 1 || got[0].Path != ".gitignore" {
		t.Fatalf("files = %#v, want .gitignore only", got)
	}
}

func TestRecentLimitsFiles(t *testing.T) {
	root := t.TempDir()
	now := time.Date(2026, 5, 12, 12, 0, 0, 0, time.Local)
	writeFileAt(t, root, "a.go", now.Add(-3*time.Minute))
	writeFileAt(t, root, "b.go", now.Add(-2*time.Minute))
	writeFileAt(t, root, "c.go", now.Add(-1*time.Minute))

	got, err := Recent(root, Options{}, 2)
	if err != nil {
		t.Fatalf("Recent() error = %v", err)
	}
	if len(got) != 2 || got[0].Path != "c.go" || got[1].Path != "b.go" {
		t.Fatalf("files = %#v, want c.go/b.go", got)
	}
}

func TestRecentRespectsGitignoreRules(t *testing.T) {
	root := t.TempDir()
	now := time.Date(2026, 5, 12, 12, 0, 0, 0, time.Local)
	writeFileContentAt(t, root, ".gitignore", "node_modules/\n", now.Add(-4*time.Minute))
	writeFileAt(t, root, "src/app.ts", now.Add(-2*time.Minute))
	writeFileAt(t, root, "node_modules/pkg/index.js", now)

	got, err := Recent(root, Options{}, 5)
	if err != nil {
		t.Fatalf("Recent() error = %v", err)
	}
	if len(got) == 0 || got[0].Path != "src/app.ts" {
		t.Fatalf("files = %#v, want src/app.ts first", got)
	}
	for _, file := range got {
		if file.Path == "node_modules/pkg/index.js" {
			t.Fatalf("Recent() should ignore node_modules from .gitignore: %#v", got)
		}
	}
}

func TestRecentRespectsNestedGitignoreRules(t *testing.T) {
	root := t.TempDir()
	now := time.Date(2026, 5, 12, 12, 0, 0, 0, time.Local)
	writeFileContentAt(t, root, "apps/web/.gitignore", "generated/\n", now.Add(-4*time.Minute))
	writeFileAt(t, root, "apps/web/src/page.ts", now.Add(-2*time.Minute))
	writeFileAt(t, root, "apps/web/generated/client.ts", now)

	got, err := Recent(root, Options{}, 5)
	if err != nil {
		t.Fatalf("Recent() error = %v", err)
	}
	if len(got) == 0 || got[0].Path != "apps/web/src/page.ts" {
		t.Fatalf("files = %#v, want apps/web/src/page.ts first", got)
	}
	for _, file := range got {
		if file.Path == "apps/web/generated/client.ts" {
			t.Fatalf("Recent() should ignore nested .gitignore match: %#v", got)
		}
	}
}

func TestRecentRespectsGitignoreNegationRules(t *testing.T) {
	root := t.TempDir()
	now := time.Date(2026, 5, 12, 12, 0, 0, 0, time.Local)
	writeFileContentAt(t, root, ".gitignore", "*.log\n!important.log\n", now.Add(-4*time.Minute))
	writeFileAt(t, root, "debug.log", now)
	writeFileAt(t, root, "important.log", now.Add(-1*time.Minute))

	got, err := Recent(root, Options{}, 5)
	if err != nil {
		t.Fatalf("Recent() error = %v", err)
	}
	if len(got) == 0 || got[0].Path != "important.log" {
		t.Fatalf("files = %#v, want important.log first", got)
	}
	for _, file := range got {
		if file.Path == "debug.log" {
			t.Fatalf("Recent() should ignore debug.log: %#v", got)
		}
	}
}

func TestRecentRespectsAnchoredGitignoreRules(t *testing.T) {
	root := t.TempDir()
	now := time.Date(2026, 5, 12, 12, 0, 0, 0, time.Local)
	writeFileContentAt(t, root, ".gitignore", "/build/\n", now.Add(-4*time.Minute))
	writeFileAt(t, root, "build/output.js", now)
	writeFileAt(t, root, "apps/web/build/output.js", now.Add(-1*time.Minute))

	got, err := Recent(root, Options{}, 5)
	if err != nil {
		t.Fatalf("Recent() error = %v", err)
	}
	if len(got) == 0 || got[0].Path != "apps/web/build/output.js" {
		t.Fatalf("files = %#v, want nested build output first", got)
	}
	for _, file := range got {
		if file.Path == "build/output.js" {
			t.Fatalf("Recent() should ignore root build output: %#v", got)
		}
	}
}

func TestRecentUsesGitIgnoreRulesInGitRepo(t *testing.T) {
	root := gitRepo(t)
	now := time.Date(2026, 5, 12, 12, 0, 0, 0, time.Local)
	writeFileContentAt(t, root, ".gitignore", "node_modules/\n", now.Add(-4*time.Minute))
	writeFileAt(t, root, "src/app.ts", now.Add(-2*time.Minute))
	writeFileAt(t, root, "node_modules/pkg/index.js", now)

	got, err := Recent(root, Options{}, 5)
	if err != nil {
		t.Fatalf("Recent() error = %v", err)
	}
	if len(got) == 0 || got[0].Path != "src/app.ts" {
		t.Fatalf("files = %#v, want src/app.ts first", got)
	}
	for _, file := range got {
		if file.Path == "node_modules/pkg/index.js" {
			t.Fatalf("Recent() should ignore node_modules from .gitignore: %#v", got)
		}
	}
}

func TestRecentGitPathSkipsConfiguredIgnoredDirs(t *testing.T) {
	root := gitRepo(t)
	now := time.Date(2026, 5, 12, 12, 0, 0, 0, time.Local)
	writeFileAt(t, root, "src/app.ts", now.Add(-2*time.Minute))
	writeFileAt(t, root, "dist/output.js", now)
	gitGlobal(t, "-C", root, "add", "src/app.ts", "dist/output.js")

	got, err := Recent(root, Options{IgnoreDirs: []string{"dist"}}, 5)
	if err != nil {
		t.Fatalf("Recent() error = %v", err)
	}
	if len(got) == 0 || got[0].Path != "src/app.ts" {
		t.Fatalf("files = %#v, want src/app.ts first", got)
	}
	for _, file := range got {
		if file.Path == "dist/output.js" {
			t.Fatalf("Recent() should ignore configured dir in git path: %#v", got)
		}
	}
}

func TestNewestModifiedSkipsIgnoredDirs(t *testing.T) {
	root := t.TempDir()
	now := time.Date(2026, 5, 12, 12, 0, 0, 0, time.Local)
	writeFileAt(t, root, "src/app.ts", now.Add(-2*time.Minute))
	writeFileAt(t, root, "node_modules/pkg/index.js", now)

	got, ok, err := NewestModified(root, Options{IgnoreDirs: []string{"node_modules"}})
	if err != nil {
		t.Fatalf("NewestModified() error = %v", err)
	}
	if !ok || !got.Equal(now.Add(-2*time.Minute)) {
		t.Fatalf("NewestModified() = %s, %v; want src/app.ts mtime", got, ok)
	}
}

func TestNewestModifiedSkipsConfiguredIgnoredDirs(t *testing.T) {
	root := t.TempDir()
	now := time.Date(2026, 5, 12, 12, 0, 0, 0, time.Local)
	writeFileAt(t, root, "src/app.ts", now.Add(-2*time.Minute))
	writeFileAt(t, root, "generated/client.ts", now)

	got, ok, err := NewestModified(root, Options{IgnoreDirs: []string{"generated"}})
	if err != nil {
		t.Fatalf("NewestModified() error = %v", err)
	}
	if !ok || !got.Equal(now.Add(-2*time.Minute)) {
		t.Fatalf("NewestModified() = %s, %v; want src/app.ts mtime", got, ok)
	}
}

func TestNewestModifiedSkipsDotDirsButKeepsDotFiles(t *testing.T) {
	root := t.TempDir()
	now := time.Date(2026, 5, 12, 12, 0, 0, 0, time.Local)
	writeFileAt(t, root, ".hidden/cache.json", now)
	writeFileContentAt(t, root, ".gitignore", "# keep dot file\n", now.Add(-1*time.Minute))
	writeFileAt(t, root, "src/app.ts", now.Add(-2*time.Minute))

	got, ok, err := NewestModified(root, Options{})
	if err != nil {
		t.Fatalf("NewestModified() error = %v", err)
	}
	if !ok || !got.Equal(now.Add(-1*time.Minute)) {
		t.Fatalf("NewestModified() = %s, %v; want .gitignore mtime", got, ok)
	}
}

func TestNewestModifiedReturnsFalseForEmptyProject(t *testing.T) {
	got, ok, err := NewestModified(t.TempDir(), Options{})
	if err != nil {
		t.Fatalf("NewestModified() error = %v", err)
	}
	if ok || !got.IsZero() {
		t.Fatalf("NewestModified() = %s, %v; want zero time and false", got, ok)
	}
}

func writeFileAt(t *testing.T, root, name string, at time.Time) {
	t.Helper()
	writeFileContentAt(t, root, name, name, at)
}

func writeFileContentAt(t *testing.T, root, name, content string, at time.Time) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, at, at); err != nil {
		t.Fatal(err)
	}
}

func gitRepo(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	dir := t.TempDir()
	gitGlobal(t, "init", dir)
	return dir
}

func gitGlobal(t *testing.T, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, string(out))
	}
}
