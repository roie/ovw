package stack

import (
	"os"
	"path/filepath"
	"testing"

	"ovw/internal/config"
)

func TestDetectSvelteKitCloudflareWithAliases(t *testing.T) {
	dir := t.TempDir()
	writePackage(t, dir, `{"dependencies":{"@sveltejs/kit":"latest","wrangler":"latest"}}`)
	cfg := config.Default()

	result, err := Detect(dir, cfg.Stack)
	if err != nil {
		t.Fatalf("Detect() error = %v", err)
	}
	if !has(result.Labels, "SvelteKit") || !has(result.Labels, "Cloudflare Workers") {
		t.Fatalf("labels = %#v", result.Labels)
	}
	if result.Display != "SvelteKit+CF" {
		t.Fatalf("display = %q", result.Display)
	}
}

func TestDetectWXTSvelte(t *testing.T) {
	dir := t.TempDir()
	writePackage(t, dir, `{"devDependencies":{"wxt":"latest","svelte":"latest"}}`)

	result, err := Detect(dir, config.Default().Stack)
	if err != nil {
		t.Fatalf("Detect() error = %v", err)
	}
	if result.Display != "WXT+Svelte" {
		t.Fatalf("display = %q labels=%#v", result.Display, result.Labels)
	}
}

func TestDetectLanguageFiles(t *testing.T) {
	cases := map[string]string{
		"Cargo.toml":     "Rust",
		"go.mod":         "Go",
		"pyproject.toml": "Python",
	}
	for file, want := range cases {
		t.Run(want, func(t *testing.T) {
			dir := t.TempDir()
			touch(t, filepath.Join(dir, file))
			result, err := Detect(dir, config.Default().Stack)
			if err != nil {
				t.Fatalf("Detect() error = %v", err)
			}
			if result.Display != want {
				t.Fatalf("display = %q", result.Display)
			}
		})
	}
}

func TestDetectNodeFallback(t *testing.T) {
	dir := t.TempDir()
	writePackage(t, dir, `{"dependencies":{"left-pad":"latest"}}`)

	result, err := Detect(dir, config.Default().Stack)
	if err != nil {
		t.Fatalf("Detect() error = %v", err)
	}
	if result.Display != "Node" {
		t.Fatalf("display = %q labels=%#v", result.Display, result.Labels)
	}
}

func TestUnknownDisplay(t *testing.T) {
	dir := t.TempDir()
	result, err := Detect(dir, config.Default().Stack)
	if err != nil {
		t.Fatalf("Detect() error = %v", err)
	}
	if result.Display != "Unknown" {
		t.Fatalf("display = %q", result.Display)
	}
}

func writePackage(t *testing.T, dir, data string) {
	t.Helper()
	touch(t, filepath.Join(dir, "package.json"))
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
}

func touch(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
}

func has(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
