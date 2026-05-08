package metadata

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadMissingMetadata(t *testing.T) {
	store, err := Load(filepath.Join(t.TempDir(), "projects.json"))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(store.Projects) != 0 {
		t.Fatalf("Projects = %#v", store.Projects)
	}
}

func TestWriteReadMetadata(t *testing.T) {
	path := filepath.Join(t.TempDir(), "projects.json")
	projectPath := filepath.Join(t.TempDir(), "app")
	if err := os.MkdirAll(projectPath, 0o755); err != nil {
		t.Fatal(err)
	}
	store := Store{Projects: map[string]Entry{}}
	canonical, err := CanonicalPath(projectPath)
	if err != nil {
		t.Fatal(err)
	}
	store.Projects[canonical] = Entry{
		Status: "active",
		Note:   "ship it",
		Hidden: true,
	}

	if err := Write(path, store); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	got := loaded.Projects[canonical]
	if got.Status != "active" || got.Note != "ship it" || !got.Hidden {
		t.Fatalf("Entry = %#v", got)
	}
}

func TestSetUsesCanonicalPath(t *testing.T) {
	root := t.TempDir()
	projectPath := filepath.Join(root, "app")
	if err := os.MkdirAll(projectPath, 0o755); err != nil {
		t.Fatal(err)
	}
	store := New()
	canonical, _, err := store.Set(filepath.Join(projectPath, "."), Entry{Status: "active"})
	if err != nil {
		t.Fatalf("Set() error = %v", err)
	}
	if !filepath.IsAbs(canonical) {
		t.Fatalf("canonical path is not absolute: %q", canonical)
	}
	if _, ok := store.Projects[canonical]; !ok {
		t.Fatalf("canonical key missing from %#v", store.Projects)
	}
}

func TestOmitEmptyFields(t *testing.T) {
	data, err := json.Marshal(Store{Projects: map[string]Entry{
		"/tmp/app": {Hidden: true},
	}})
	if err != nil {
		t.Fatal(err)
	}
	got := string(data)
	if got != `{"projects":{"/tmp/app":{"hidden":true}}}` {
		t.Fatalf("json = %s", got)
	}
}
