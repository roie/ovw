package projectversion

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDetectPackageJSONVersion(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "package.json"), `{"version":"1.2.3"}`)

	got := Detect(dir)
	if got != "1.2.3" {
		t.Fatalf("Detect() = %q", got)
	}
}

func TestDetectCargoVersion(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "Cargo.toml"), "[package]\nname = \"app\"\nversion = \"0.4.0\"\n")

	got := Detect(dir)
	if got != "0.4.0" {
		t.Fatalf("Detect() = %q", got)
	}
}

func TestDetectPyprojectVersion(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "pyproject.toml"), "[project]\nname = \"app\"\nversion = \"2.0.1\"\n")

	got := Detect(dir)
	if got != "2.0.1" {
		t.Fatalf("Detect() = %q", got)
	}
}

func TestDetectManifestVersion(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "manifest.json"), `{"manifest_version":3,"version":"1.0.0"}`)

	got := Detect(dir)
	if got != "1.0.0" {
		t.Fatalf("Detect() = %q", got)
	}
}

func TestDetectWorkspaceVersion(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "package.json"), `{"workspaces":["apps/*"]}`)
	write(t, filepath.Join(dir, "apps", "web", "package.json"), `{"version":"0.8.0"}`)

	got := Detect(dir)
	if got != "0.8.0" {
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
