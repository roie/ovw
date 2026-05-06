package app

import (
	"path/filepath"
	"testing"
	"time"

	"ovw/internal/cache"
	"ovw/internal/config"
	"ovw/internal/scanner"
)

func TestEnrichUsesProjectDescriptionFallback(t *testing.T) {
	dir := t.TempDir()
	writePackage(t, dir, `{"description":"Package description"}`)

	enriched, cached := Enrich(scanner.Project{Name: "app", Path: dir}, config.Default(), cache.New(), time.Now())
	if enriched.Note.Display != "Package description" || enriched.Note.Source != "description" {
		t.Fatalf("note = %#v", enriched.Note)
	}
	if enriched.Description != "Package description" {
		t.Fatalf("Description = %q", enriched.Description)
	}
	if cached.Description != "Package description" {
		t.Fatalf("cached Description = %q", cached.Description)
	}
}

func TestEnrichDescriptionFallbackUsesWorkspaceChild(t *testing.T) {
	dir := t.TempDir()
	writePackage(t, dir, `{"workspaces":["apps/*"]}`)
	writePackage(t, filepath.Join(dir, "apps", "web"), `{"description":"Workspace app"}`)

	enriched, _ := Enrich(scanner.Project{Name: "app", Path: dir}, config.Default(), cache.New(), time.Now())
	if enriched.Note.Display != "Workspace app" || enriched.Note.Source != "description" {
		t.Fatalf("note = %#v", enriched.Note)
	}
}
