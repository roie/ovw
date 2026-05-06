package cache

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadMissingCache(t *testing.T) {
	store, err := Load(filepath.Join(t.TempDir(), "projects.json"))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(store.Projects) != 0 {
		t.Fatalf("Projects = %#v", store.Projects)
	}
}

func TestWriteReadCache(t *testing.T) {
	path := filepath.Join(t.TempDir(), "projects.json")
	now := time.Unix(100, 0)
	store := Store{Projects: map[string]Project{
		"/tmp/app": {
			Stack:             []string{"Go"},
			StackDisplay:      "Go",
			Description:       "tool",
			LastCommitAt:      now,
			LastCommitMessage: "init",
		},
	}}

	if err := Write(path, store); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	got := loaded.Projects["/tmp/app"]
	if got.StackDisplay != "Go" || got.LastCommitMessage != "init" || !got.LastCommitAt.Equal(now) {
		t.Fatalf("project = %#v", got)
	}
}

func TestLoadCorruptCacheFallsBack(t *testing.T) {
	path := filepath.Join(t.TempDir(), "projects.json")
	if err := os.WriteFile(path, []byte("{nope"), 0o644); err != nil {
		t.Fatal(err)
	}
	store, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(store.Projects) != 0 {
		t.Fatalf("Projects = %#v", store.Projects)
	}
}

func TestClearDoesNotTouchMetadata(t *testing.T) {
	dir := t.TempDir()
	cachePath := filepath.Join(dir, "cache", "projects.json")
	metadataPath := filepath.Join(dir, "data", "projects.json")
	if err := os.MkdirAll(filepath.Dir(cachePath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(metadataPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cachePath, []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(metadataPath, []byte(`{"projects":{}}`), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := Clear(cachePath); err != nil {
		t.Fatalf("Clear() error = %v", err)
	}
	if _, err := os.Stat(cachePath); !os.IsNotExist(err) {
		t.Fatalf("cache still exists or stat failed unexpectedly: %v", err)
	}
	if _, err := os.Stat(metadataPath); err != nil {
		t.Fatalf("metadata was touched: %v", err)
	}
}
