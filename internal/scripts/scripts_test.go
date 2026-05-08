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

func TestDetectIgnoresPackageWithoutScripts(t *testing.T) {
	dir := t.TempDir()
	writeScriptsPackage(t, dir, `{"name":"app"}`)

	if got := Detect(dir); len(got) != 0 {
		t.Fatalf("Detect() = %#v, want empty", got)
	}
}

func writeScriptsPackage(t *testing.T, dir, data string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
}
