package app

import (
	"path/filepath"
	"testing"
	"time"

	"ovw/internal/config"
	"ovw/internal/scanner"
)

func TestEnrichUsesProjectDescriptionFallback(t *testing.T) {
	dir := t.TempDir()
	writePackage(t, dir, `{"description":"Package description"}`)

	enriched := Enrich(scanner.Project{Name: "app", Path: dir}, config.Default(), time.Now())
	if enriched.Note.Display != "Package description" || enriched.Note.Source != "description" {
		t.Fatalf("note = %#v", enriched.Note)
	}
	if enriched.Description != "Package description" {
		t.Fatalf("Description = %q", enriched.Description)
	}
}

func TestEnrichDescriptionFallbackUsesWorkspaceChild(t *testing.T) {
	dir := t.TempDir()
	writePackage(t, dir, `{"workspaces":["apps/*"]}`)
	writePackage(t, filepath.Join(dir, "apps", "web"), `{"description":"Workspace app"}`)

	enriched := Enrich(scanner.Project{Name: "app", Path: dir}, config.Default(), time.Now())
	if enriched.Note.Display != "Workspace app" || enriched.Note.Source != "description" {
		t.Fatalf("note = %#v", enriched.Note)
	}
}

func TestEnrichDetectsProjectVersion(t *testing.T) {
	dir := t.TempDir()
	writePackage(t, dir, `{"version":"1.2.3"}`)

	enriched := Enrich(scanner.Project{Name: "app", Path: dir}, config.Default(), time.Now())
	if enriched.Version != "1.2.3" {
		t.Fatalf("Version = %q", enriched.Version)
	}
}

func TestEnrichDetectsProjectScripts(t *testing.T) {
	dir := t.TempDir()
	writePackage(t, dir, `{"scripts":{"dev":"vite","build":"vite build","check":"go test ./..."}}`)

	enriched := Enrich(scanner.Project{Name: "app", Path: dir}, config.Default(), time.Now())
	want := []string{"dev", "build", "check"}
	if len(enriched.Scripts) != len(want) {
		t.Fatalf("Scripts = %#v, want %#v", enriched.Scripts, want)
	}
	for index, script := range want {
		if enriched.Scripts[index] != script {
			t.Fatalf("Scripts = %#v, want %#v", enriched.Scripts, want)
		}
	}
}
