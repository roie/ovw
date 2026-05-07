package manager

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDetectJavaScriptLockfiles(t *testing.T) {
	cases := []struct {
		file string
		want string
	}{
		{"pnpm-lock.yaml", "pnpm"},
		{"bun.lock", "bun"},
		{"bun.lockb", "bun"},
		{"yarn.lock", "yarn"},
		{"package-lock.json", "npm"},
	}
	for _, tc := range cases {
		t.Run(tc.want+" "+tc.file, func(t *testing.T) {
			dir := t.TempDir()
			touchManagerFile(t, filepath.Join(dir, tc.file))

			got := Detect(dir)
			if len(got) != 1 || got[0] != tc.want {
				t.Fatalf("Detect() = %#v, want %#v", got, []string{tc.want})
			}
		})
	}
}

func TestDetectPackageJSONPackageManagerFallback(t *testing.T) {
	dir := t.TempDir()
	writeManagerPackage(t, dir, `{"packageManager":"pnpm@9.0.0"}`)

	got := Detect(dir)
	if len(got) != 1 || got[0] != "pnpm" {
		t.Fatalf("Detect() = %#v, want pnpm", got)
	}
}

func TestDetectLanguageManagers(t *testing.T) {
	cases := []struct {
		file string
		want string
	}{
		{"go.mod", "go modules"},
		{"Cargo.toml", "cargo"},
		{"Cargo.lock", "cargo"},
		{"uv.lock", "uv"},
		{"poetry.lock", "poetry"},
		{"pdm.lock", "pdm"},
		{"Pipfile", "pipenv"},
		{"Pipfile.lock", "pipenv"},
		{"requirements.txt", "pip"},
		{"pyproject.toml", "python"},
		{"deno.json", "deno"},
		{"deno.jsonc", "deno"},
	}
	for _, tc := range cases {
		t.Run(tc.want+" "+tc.file, func(t *testing.T) {
			dir := t.TempDir()
			touchManagerFile(t, filepath.Join(dir, tc.file))

			got := Detect(dir)
			if len(got) != 1 || got[0] != tc.want {
				t.Fatalf("Detect() = %#v, want %#v", got, []string{tc.want})
			}
		})
	}
}

func TestDetectMixedManagersInStableOrder(t *testing.T) {
	dir := t.TempDir()
	touchManagerFile(t, filepath.Join(dir, "pnpm-lock.yaml"))
	touchManagerFile(t, filepath.Join(dir, "Cargo.toml"))

	got := Detect(dir)
	want := []string{"pnpm", "cargo"}
	if len(got) != len(want) {
		t.Fatalf("Detect() = %#v, want %#v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("Detect() = %#v, want %#v", got, want)
		}
	}
}

func writeManagerPackage(t *testing.T, dir, data string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
}

func touchManagerFile(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
}
