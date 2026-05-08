package description

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDetectPackageJSONDescription(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "package.json"), `{"description":"Browser event platform"}`)

	got := Detect(dir)
	if got != "Browser event platform" {
		t.Fatalf("Detect() = %q", got)
	}
}

func TestDetectCargoDescription(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "Cargo.toml"), "[package]\nname = \"app\"\ndescription = \"Rust helper\"\n")

	got := Detect(dir)
	if got != "Rust helper" {
		t.Fatalf("Detect() = %q", got)
	}
}

func TestDetectPyprojectDescription(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "pyproject.toml"), "[project]\nname = \"app\"\ndescription = \"Python tool\"\n")

	got := Detect(dir)
	if got != "Python tool" {
		t.Fatalf("Detect() = %q", got)
	}
}

func TestDetectManifestDescription(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "manifest.json"), `{"manifest_version":3,"description":"Domain diagnostics when things vanish"}`)

	got := Detect(dir)
	if got != "Domain diagnostics when things vanish" {
		t.Fatalf("Detect() = %q", got)
	}
}

func TestDetectReturnsEmptyWhenMissing(t *testing.T) {
	if got := Detect(t.TempDir()); got != "" {
		t.Fatalf("Detect() = %q", got)
	}
}

func write(t *testing.T, path, data string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
}
