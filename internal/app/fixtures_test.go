package app

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"ovw/internal/config"
	"ovw/internal/metadata"
	"ovw/internal/scanner"
)

func TestPhaseOneFixturesScanStackMetadataAndJSON(t *testing.T) {
	fixtureRoot, err := filepath.Abs(filepath.Join("..", "..", "testdata", "fixtures", "projects"))
	if err != nil {
		t.Fatal(err)
	}
	hiddenPath := filepath.Join(fixtureRoot, "hidden-go")
	meta := metadata.New()
	canonicalHidden, _, err := meta.Set(hiddenPath, metadata.Entry{Hidden: true})
	if err != nil {
		t.Fatal(err)
	}
	cfg := config.Default()
	cfg.Roots = []string{fixtureRoot}

	scanned, err := scanner.Scan(cfg, meta)
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}
	for _, project := range scanned {
		if project.Path == canonicalHidden {
			t.Fatalf("hidden project included: %#v", scanned)
		}
	}

	home := t.TempDir()
	t.Setenv("HOME", home)
	paths, err := config.Paths()
	if err != nil {
		t.Fatal(err)
	}
	if err := config.Write(paths.Config, cfg); err != nil {
		t.Fatal(err)
	}
	if err := metadata.Write(paths.Metadata, meta); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	if err := Run(Options{JSON: true, Cwd: fixtureRoot, Out: &out, In: strings.NewReader("\n")}); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	got := out.String()
	for _, want := range []string{`"SvelteKit"`, `"Cloudflare Workers"`} {
		if !strings.Contains(got, want) {
			t.Fatalf("json missing stack value %q: %s", want, got)
		}
	}
	if strings.Contains(got, "stack_display") {
		t.Fatalf("json included display-only stack field: %s", got)
	}
	if strings.Contains(got, "hidden-go") {
		t.Fatalf("json included hidden project: %s", got)
	}
	if strings.Contains(got, "readme-only") {
		t.Fatalf("json included README-only fixture: %s", got)
	}
}
