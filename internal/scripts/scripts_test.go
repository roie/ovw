package scripts

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDetectPackageJSONScriptsPreservesOrder(t *testing.T) {
	dir := t.TempDir()
	writeScriptsPackage(t, dir, `{
		"scripts": {
			"dev": "vite",
			"build": "vite build",
			"check": "svelte-check"
		}
	}`)

	got := Detect(dir)
	want := []string{"dev", "build", "check"}
	if len(got) != len(want) {
		t.Fatalf("Detect() = %#v, want %#v", got, want)
	}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("Detect() = %#v, want %#v", got, want)
		}
	}
}

func TestDetectCargoAliases(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, ".cargo", "config.toml"), `[alias]
xtask = "run --package xtask --"
ci = ["test", "--workspace"]
`)

	got := Detect(dir)
	want := []string{"xtask", "ci"}
	if len(got) != len(want) {
		t.Fatalf("Detect() = %#v, want %#v", got, want)
	}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("Detect() = %#v, want %#v", got, want)
		}
	}
}

func TestDetectPyprojectToolTasks(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "pyproject.toml"), `[tool.poe.tasks]
dev = "uvicorn app:app"
test = "pytest"

[tool.taskipy.tasks]
lint = "ruff check ."
`)

	got := Detect(dir)
	want := []string{"dev", "test", "lint"}
	if len(got) != len(want) {
		t.Fatalf("Detect() = %#v, want %#v", got, want)
	}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("Detect() = %#v, want %#v", got, want)
		}
	}
}

func TestDetectPyprojectSubtableTasksPreservesOrder(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "pyproject.toml"), `[tool.poe.tasks]
test = "pytest"

[tool.poe.tasks.serve]
cmd = "uvicorn app:app"

[tool.poe.tasks.lint]
cmd = "ruff check ."
`)

	got := Detect(dir)
	want := []string{"test", "serve", "lint"}
	if len(got) != len(want) {
		t.Fatalf("Detect() = %#v, want %#v", got, want)
	}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("Detect() = %#v, want %#v", got, want)
		}
	}
}

func TestDetectIgnoresPackageWithoutScripts(t *testing.T) {
	dir := t.TempDir()
	writeScriptsPackage(t, dir, `{"name":"app"}`)

	if got := Detect(dir); len(got) != 0 {
		t.Fatalf("Detect() = %#v, want empty", got)
	}
}

func TestDetectWithIgnoreSkipsIgnoredWorkspaceRoots(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "package.json"), `{"workspaces":["node_modules/*","packages/*"]}`)
	writeScriptsPackage(t, filepath.Join(dir, "node_modules", "ignored"), `{"scripts":{"bad":"bad"}}`)
	writeScriptsPackage(t, filepath.Join(dir, "packages", "app"), `{"scripts":{"dev":"vite"}}`)

	got := DetectWithIgnore(dir, []string{"node_modules"})
	want := []string{"dev"}
	if len(got) != len(want) {
		t.Fatalf("DetectWithIgnore() = %#v, want %#v", got, want)
	}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("DetectWithIgnore() = %#v, want %#v", got, want)
		}
	}
}

func writeScriptsPackage(t *testing.T, dir, data string) {
	t.Helper()
	writeFile(t, filepath.Join(dir, "package.json"), data)
}

func writeFile(t *testing.T, path, data string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
}
