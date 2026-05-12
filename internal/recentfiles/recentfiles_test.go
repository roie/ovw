package recentfiles

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDetectReturnsRecentlyModifiedFiles(t *testing.T) {
	root := t.TempDir()
	now := time.Date(2026, 5, 12, 12, 0, 0, 0, time.Local)
	writeFileAt(t, root, "README.md", now.Add(-5*time.Minute))
	writeFileAt(t, root, "internal/app.go", now.Add(-2*time.Minute))
	writeFileAt(t, root, "dist/bundle.js", now.Add(-1*time.Minute))
	writeFileAt(t, root, ".git/config", now)

	got, err := Detect(root, []string{"dist"}, 5)
	if err != nil {
		t.Fatalf("Detect() error = %v", err)
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

func TestDetectSkipsDotDirsButKeepsDotFiles(t *testing.T) {
	root := t.TempDir()
	now := time.Date(2026, 5, 12, 12, 0, 0, 0, time.Local)
	writeFileAt(t, root, ".gitignore", now.Add(-2*time.Minute))
	writeFileAt(t, root, ".vite/cache.json", now.Add(-1*time.Minute))

	got, err := Detect(root, nil, 5)
	if err != nil {
		t.Fatalf("Detect() error = %v", err)
	}
	if len(got) != 1 || got[0].Path != ".gitignore" {
		t.Fatalf("files = %#v, want .gitignore only", got)
	}
}

func TestDetectLimitsFiles(t *testing.T) {
	root := t.TempDir()
	now := time.Date(2026, 5, 12, 12, 0, 0, 0, time.Local)
	writeFileAt(t, root, "a.go", now.Add(-3*time.Minute))
	writeFileAt(t, root, "b.go", now.Add(-2*time.Minute))
	writeFileAt(t, root, "c.go", now.Add(-1*time.Minute))

	got, err := Detect(root, nil, 2)
	if err != nil {
		t.Fatalf("Detect() error = %v", err)
	}
	if len(got) != 2 || got[0].Path != "c.go" || got[1].Path != "b.go" {
		t.Fatalf("files = %#v, want c.go/b.go", got)
	}
}

func writeFileAt(t *testing.T, root, name string, at time.Time) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(name), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, at, at); err != nil {
		t.Fatal(err)
	}
}
