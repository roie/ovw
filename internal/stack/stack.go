package stack

import (
	"encoding/json"
	"os"
	"path/filepath"
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
	packages, err := readPackages(filepath.Join(path, "package.json"))
	if err != nil {
		return Result{}, err
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
	if len(labels) == 0 && nodeFallback {
		labels = append(labels, "Node")
	}
	if len(labels) == 0 && cfg.ShowUnknown {
		return Result{Labels: []string{"Unknown"}, Display: "Unknown"}, nil
	}
	return Result{Labels: labels, Display: display(labels, cfg.Aliases)}, nil
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
