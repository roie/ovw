package stack

import (
	"os"
	"path/filepath"
	"runtime"
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

func TestDetectDoesNotTreatBunAsStack(t *testing.T) {
	dir := t.TempDir()
	touch(t, filepath.Join(dir, "bun.lock"))

	result, err := Detect(dir, config.Default().Stack)
	if err != nil {
		t.Fatalf("Detect() error = %v", err)
	}
	if result.Display != "Unknown" {
		t.Fatalf("display = %q labels=%#v", result.Display, result.Labels)
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

func TestDetectWorkspaceSvelteKitCloudflare(t *testing.T) {
	dir := t.TempDir()
	writePackage(t, dir, `{"devDependencies":{"typescript":"latest","vitest":"latest"}}`)
	writePackage(t, filepath.Join(dir, "apps", "web"), `{"dependencies":{"@sveltejs/kit":"latest","svelte":"latest"},"devDependencies":{"wrangler":"latest"}}`)
	touch(t, filepath.Join(dir, "apps", "web", "wrangler.toml"))

	result, err := Detect(dir, config.Default().Stack)
	if err != nil {
		t.Fatalf("Detect() error = %v", err)
	}
	if result.Display != "SvelteKit+CF" {
		t.Fatalf("display = %q labels=%#v", result.Display, result.Labels)
	}
	if has(result.Labels, "Node") {
		t.Fatalf("Node should not be included with non-Node labels: %#v", result.Labels)
	}
}

func TestDetectPackageJSONWorkspaces(t *testing.T) {
	dir := t.TempDir()
	writePackage(t, dir, `{"workspaces":["apps/*"]}`)
	writePackage(t, filepath.Join(dir, "apps", "client"), `{"dependencies":{"react":"latest"}}`)

	result, err := Detect(dir, config.Default().Stack)
	if err != nil {
		t.Fatalf("Detect() error = %v", err)
	}
	if result.Display != "React" {
		t.Fatalf("display = %q labels=%#v", result.Display, result.Labels)
	}
}

func TestDetectSkipsUnreadablePackageJSON(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("chmod unreadable package.json is not portable on windows")
	}
	dir := t.TempDir()
	packagePath := filepath.Join(dir, "package.json")
	if err := os.WriteFile(packagePath, []byte(`{"dependencies":{"react":"latest"}}`), 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = os.Chmod(packagePath, 0o644)
	})
	touch(t, filepath.Join(dir, "go.mod"))

	result, err := Detect(dir, config.Default().Stack)
	if err != nil {
		t.Fatalf("Detect() error = %v", err)
	}
	if result.Display != "Go" {
		t.Fatalf("display = %q labels=%#v", result.Display, result.Labels)
	}
}

func TestDetectPackageJSONWorkspacesObject(t *testing.T) {
	dir := t.TempDir()
	writePackage(t, dir, `{"workspaces":{"packages":["packages/*"]}}`)
	writePackage(t, filepath.Join(dir, "packages", "api"), `{"dependencies":{"hono":"latest"}}`)

	result, err := Detect(dir, config.Default().Stack)
	if err != nil {
		t.Fatalf("Detect() error = %v", err)
	}
	if result.Display != "Hono" {
		t.Fatalf("display = %q labels=%#v", result.Display, result.Labels)
	}
}

func TestDetectPNPMWorkspaceSimpleGlobs(t *testing.T) {
	dir := t.TempDir()
	writePackage(t, dir, `{"devDependencies":{"typescript":"latest"}}`)
	if err := os.WriteFile(filepath.Join(dir, "pnpm-workspace.yaml"), []byte("packages:\n  - 'apps/*'\n  - \"packages/*\"\n  - '!**/test/**'\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	writePackage(t, filepath.Join(dir, "apps", "web"), `{"dependencies":{"vue":"latest"}}`)
	writePackage(t, filepath.Join(dir, "packages", "ignored-test"), `{"dependencies":{"react":"latest"}}`)

	result, err := Detect(dir, config.Default().Stack)
	if err != nil {
		t.Fatalf("Detect() error = %v", err)
	}
	if result.Display != "React+Vue" {
		t.Fatalf("display = %q labels=%#v", result.Display, result.Labels)
	}
}

func TestDetectFallbackCommonDirs(t *testing.T) {
	dir := t.TempDir()
	writePackage(t, dir, `{"devDependencies":{"typescript":"latest"}}`)
	writePackage(t, filepath.Join(dir, "extension", "popup"), `{"dependencies":{"wxt":"latest","svelte":"latest"}}`)

	result, err := Detect(dir, config.Default().Stack)
	if err != nil {
		t.Fatalf("Detect() error = %v", err)
	}
	if result.Display != "WXT+Svelte" {
		t.Fatalf("display = %q labels=%#v", result.Display, result.Labels)
	}
}

func TestDetectDedupesAndOrdersWorkspaceLabels(t *testing.T) {
	dir := t.TempDir()
	writePackage(t, dir, `{"dependencies":{"svelte":"latest"},"workspaces":["apps/*"]}`)
	writePackage(t, filepath.Join(dir, "apps", "web"), `{"dependencies":{"@sveltejs/kit":"latest","svelte":"latest","react":"latest"}}`)

	result, err := Detect(dir, config.Default().Stack)
	if err != nil {
		t.Fatalf("Detect() error = %v", err)
	}
	want := []string{"SvelteKit", "React"}
	if len(result.Labels) != len(want) {
		t.Fatalf("labels = %#v", result.Labels)
	}
	for i := range want {
		if result.Labels[i] != want[i] {
			t.Fatalf("labels = %#v, want %#v", result.Labels, want)
		}
	}
}

func TestDetectSuppressesRedundantReactNativeReact(t *testing.T) {
	dir := t.TempDir()
	writePackage(t, dir, `{"dependencies":{"react-native":"latest","react":"latest"}}`)

	result, err := Detect(dir, config.Default().Stack)
	if err != nil {
		t.Fatalf("Detect() error = %v", err)
	}
	if result.Display != "RN" {
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
