package workspace

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestReadPNPMWorkspaceAllowsIndentlessLists(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "pnpm-workspace.yaml")
	if err := os.WriteFile(path, []byte(`packages:
- apps/*
- packages/*
`), 0o644); err != nil {
		t.Fatal(err)
	}

	got, ok, err := readPNPMWorkspace(path)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected pnpm workspace declaration")
	}
	want := []string{"apps/*", "packages/*"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("globs = %#v, want %#v", got, want)
	}
}

func TestReadPNPMWorkspaceAllowsInlineArrays(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "pnpm-workspace.yaml")
	if err := os.WriteFile(path, []byte(`packages: ["apps/*", "packages/*"]
`), 0o644); err != nil {
		t.Fatal(err)
	}

	got, ok, err := readPNPMWorkspace(path)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected pnpm workspace declaration")
	}
	want := []string{"apps/*", "packages/*"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("globs = %#v, want %#v", got, want)
	}
}
