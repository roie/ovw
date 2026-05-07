package stack

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"ovw/internal/config"
	"ovw/internal/workspace"
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
}

func Detect(path string, cfg config.StackConfig) (Result, error) {
	candidates, err := workspace.CandidateRoots(path)
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
	labels := suppressRedundantLabels(orderedLabels(matched))
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

func suppressRedundantLabels(labels []string) []string {
	present := map[string]bool{}
	for _, label := range labels {
		present[label] = true
	}
	out := []string{}
	for _, label := range labels {
		if label == "Svelte" && present["SvelteKit"] {
			continue
		}
		if label == "React" && present["React Native"] {
			continue
		}
		out = append(out, label)
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
