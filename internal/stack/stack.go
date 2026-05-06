package stack

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"ovw/internal/config"
)

type Result struct {
	Labels  []string
	Display string
}

type Rule struct {
	Name     string
	Packages []string
	Files    []string
	Fallback bool
}

var Rules = []Rule{
	{Name: "WXT", Packages: []string{"wxt"}, Files: []string{"wxt.config.ts", "wxt.config.js"}},
	{Name: "Astro", Packages: []string{"astro"}, Files: []string{"astro.config.ts", "astro.config.js", "astro.config.mjs"}},
	{Name: "SvelteKit", Packages: []string{"@sveltejs/kit"}, Files: []string{"svelte.config.js", "svelte.config.ts"}},
	{Name: "Svelte", Packages: []string{"svelte"}},
	{Name: "React Native", Packages: []string{"react-native"}},
	{Name: "React", Packages: []string{"react"}},
	{Name: "Vue", Packages: []string{"vue"}},
	{Name: "Cloudflare Workers", Packages: []string{"wrangler", "@cloudflare/workers-types"}, Files: []string{"wrangler.toml"}},
	{Name: "Hono", Packages: []string{"hono"}},
	{Name: "Drizzle", Packages: []string{"drizzle-orm"}, Files: []string{"drizzle.config.ts", "drizzle.config.js"}},
	{Name: "Prisma", Packages: []string{"prisma", "@prisma/client"}, Files: []string{"prisma/schema.prisma"}},
	{Name: "Tailwind", Packages: []string{"tailwindcss"}, Files: []string{"tailwind.config.ts", "tailwind.config.js"}},
	{Name: "Node", Files: []string{"package.json"}, Fallback: true},
	{Name: "Rust", Files: []string{"Cargo.toml"}},
	{Name: "Go", Files: []string{"go.mod"}},
	{Name: "Python", Files: []string{"pyproject.toml", "requirements.txt"}},
	{Name: "Deno", Files: []string{"deno.json", "deno.jsonc"}},
	{Name: "Bun", Files: []string{"bun.lock"}},
}

func Detect(path string, cfg config.StackConfig) (Result, error) {
	candidates, err := candidateRoots(path)
	if err != nil {
		return Result{}, err
	}
	matched := map[string]bool{}
	nodeFallback := false
	for _, candidate := range candidates {
		labels, node, err := detectAtRoot(candidate)
		if err != nil {
			return Result{}, err
		}
		if node {
			nodeFallback = true
		}
		for _, label := range labels {
			matched[label] = true
		}
	}
	labels := orderedLabels(matched)
	if len(labels) == 0 && nodeFallback {
		labels = append(labels, "Node")
	}
	if len(labels) == 0 && cfg.ShowUnknown {
		return Result{Labels: []string{"Unknown"}, Display: "Unknown"}, nil
	}
	return Result{Labels: labels, Display: display(labels, cfg.Aliases)}, nil
}

func detectAtRoot(path string) ([]string, bool, error) {
	packages, err := readPackages(filepath.Join(path, "package.json"))
	if err != nil {
		return nil, false, err
	}
	labels := []string{}
	nodeFallback := false
	for _, rule := range Rules {
		if rule.Fallback {
			nodeFallback = matchesFiles(path, rule.Files)
			continue
		}
		if matchesPackages(packages, rule.Packages) || matchesFiles(path, rule.Files) {
			labels = append(labels, rule.Name)
		}
	}
	return labels, nodeFallback, nil
}

func candidateRoots(root string) ([]string, error) {
	seen := map[string]bool{}
	roots := []string{}
	add := func(path string) {
		clean := filepath.Clean(path)
		if seen[clean] || ignoredPath(root, clean) {
			return
		}
		if info, err := os.Stat(clean); err == nil && info.IsDir() {
			seen[clean] = true
			roots = append(roots, clean)
		}
	}
	add(root)

	globs, hasWorkspaceDeclaration, err := workspaceGlobs(root)
	if err != nil {
		return nil, err
	}
	if hasWorkspaceDeclaration {
		for _, glob := range globs {
			for _, match := range expandWorkspaceGlob(root, glob) {
				add(match)
			}
		}
		return roots, nil
	}

	for _, glob := range []string{"apps/*", "packages/*", "web/*", "extension/*", "extensions/*"} {
		for _, match := range expandWorkspaceGlob(root, glob) {
			add(match)
		}
	}
	return roots, nil
}

func readPackages(path string) (map[string]bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]bool{}, nil
		}
		return nil, err
	}
	var pkg struct {
		Dependencies    map[string]any `json:"dependencies"`
		DevDependencies map[string]any `json:"devDependencies"`
	}
	if err := json.Unmarshal(data, &pkg); err != nil {
		return map[string]bool{}, nil
	}
	out := map[string]bool{}
	for name := range pkg.Dependencies {
		out[name] = true
	}
	for name := range pkg.DevDependencies {
		out[name] = true
	}
	return out, nil
}

func workspaceGlobs(root string) ([]string, bool, error) {
	globs := []string{}
	hasDeclaration := false
	pnpmGlobs, ok, err := readPNPMWorkspace(filepath.Join(root, "pnpm-workspace.yaml"))
	if err != nil {
		return nil, false, err
	}
	if ok {
		hasDeclaration = true
		globs = append(globs, pnpmGlobs...)
	}
	pkgGlobs, ok, err := readPackageWorkspaces(filepath.Join(root, "package.json"))
	if err != nil {
		return nil, false, err
	}
	if ok {
		hasDeclaration = true
		globs = append(globs, pkgGlobs...)
	}
	return dedupeStrings(globs), hasDeclaration, nil
}

func readPNPMWorkspace(path string) ([]string, bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, false, nil
		}
		return nil, false, err
	}
	globs := []string{}
	inPackages := false
	for _, line := range strings.Split(string(data), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if !strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "\t") {
			inPackages = strings.HasPrefix(trimmed, "packages:")
			continue
		}
		if !inPackages || !strings.HasPrefix(trimmed, "-") {
			continue
		}
		value := cleanGlob(strings.TrimSpace(strings.TrimPrefix(trimmed, "-")))
		if value != "" && !strings.HasPrefix(value, "!") {
			globs = append(globs, value)
		}
	}
	return globs, true, nil
}

func readPackageWorkspaces(path string) ([]string, bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, false, nil
		}
		return nil, false, err
	}
	var pkg struct {
		Workspaces json.RawMessage `json:"workspaces"`
	}
	if err := json.Unmarshal(data, &pkg); err != nil {
		return nil, false, nil
	}
	if len(pkg.Workspaces) == 0 {
		return nil, false, nil
	}
	var array []string
	if err := json.Unmarshal(pkg.Workspaces, &array); err == nil {
		return cleanGlobs(array), true, nil
	}
	var object struct {
		Packages []string `json:"packages"`
	}
	if err := json.Unmarshal(pkg.Workspaces, &object); err == nil {
		return cleanGlobs(object.Packages), true, nil
	}
	return nil, true, nil
}

func expandWorkspaceGlob(root, pattern string) []string {
	pattern = cleanGlob(pattern)
	if pattern == "" || strings.HasPrefix(pattern, "!") || strings.Contains(pattern, "**") {
		return nil
	}
	fullPattern := filepath.Join(root, filepath.FromSlash(pattern))
	matches, err := filepath.Glob(fullPattern)
	if err != nil {
		return nil
	}
	out := []string{}
	for _, match := range matches {
		if ignoredPath(root, match) {
			continue
		}
		if info, err := os.Stat(match); err == nil && info.IsDir() {
			out = append(out, match)
		}
	}
	sort.Strings(out)
	return out
}

func cleanGlobs(values []string) []string {
	out := []string{}
	for _, value := range values {
		cleaned := cleanGlob(value)
		if cleaned != "" && !strings.HasPrefix(cleaned, "!") {
			out = append(out, cleaned)
		}
	}
	return out
}

func cleanGlob(value string) string {
	value = strings.TrimSpace(value)
	value = strings.Trim(value, `"'`)
	value = strings.TrimSpace(value)
	return value
}

func ignoredPath(root, path string) bool {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return true
	}
	if rel == "." {
		return false
	}
	for _, part := range strings.Split(filepath.ToSlash(rel), "/") {
		if ignoredDir(part) {
			return true
		}
	}
	return false
}

func ignoredDir(name string) bool {
	for _, ignored := range []string{
		"node_modules", "dist", "build", "target", ".next", ".nuxt",
		".svelte-kit", ".turbo", ".cache", "coverage", "vendor",
		".venv", "venv", "__pycache__",
	} {
		if name == ignored {
			return true
		}
	}
	return false
}

func orderedLabels(matched map[string]bool) []string {
	labels := []string{}
	for _, rule := range Rules {
		if rule.Fallback {
			continue
		}
		if matched[rule.Name] {
			labels = append(labels, rule.Name)
		}
	}
	return labels
}

func dedupeStrings(values []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, value := range values {
		if seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}

func matchesPackages(packages map[string]bool, names []string) bool {
	for _, name := range names {
		if packages[name] {
			return true
		}
	}
	return false
}

func matchesFiles(root string, files []string) bool {
	for _, file := range files {
		if _, err := os.Stat(filepath.Join(root, file)); err == nil {
			return true
		}
	}
	return false
}

func display(labels []string, aliases map[string]string) string {
	parts := make([]string, 0, len(labels))
	for _, label := range labels {
		if alias := aliases[label]; alias != "" {
			parts = append(parts, alias)
			continue
		}
		parts = append(parts, label)
	}
	return strings.Join(parts, "+")
}
