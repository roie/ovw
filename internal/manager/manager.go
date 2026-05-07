package manager

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

type Rule struct {
	Name  string
	Files []string
}

var Rules = []Rule{
	{Name: "pnpm", Files: []string{"pnpm-lock.yaml"}},
	{Name: "bun", Files: []string{"bun.lock", "bun.lockb"}},
	{Name: "yarn", Files: []string{"yarn.lock"}},
	{Name: "npm", Files: []string{"package-lock.json"}},
	{Name: "go modules", Files: []string{"go.mod"}},
	{Name: "cargo", Files: []string{"Cargo.toml", "Cargo.lock"}},
	{Name: "uv", Files: []string{"uv.lock"}},
	{Name: "poetry", Files: []string{"poetry.lock"}},
	{Name: "pdm", Files: []string{"pdm.lock"}},
	{Name: "pipenv", Files: []string{"Pipfile", "Pipfile.lock"}},
	{Name: "pip", Files: []string{"requirements.txt"}},
	{Name: "python", Files: []string{"pyproject.toml"}},
	{Name: "deno", Files: []string{"deno.json", "deno.jsonc"}},
}

func Detect(path string) []string {
	matched := map[string]bool{}
	for _, rule := range Rules {
		if matchesFiles(path, rule.Files) {
			matched[rule.Name] = true
		}
	}
	if packageManager := packageJSONManager(filepath.Join(path, "package.json")); packageManager != "" {
		matched[packageManager] = true
	}
	out := []string{}
	for _, rule := range Rules {
		if matched[rule.Name] {
			out = append(out, rule.Name)
		}
	}
	return out
}

func packageJSONManager(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	var pkg struct {
		PackageManager string `json:"packageManager"`
	}
	if err := json.Unmarshal(data, &pkg); err != nil {
		return ""
	}
	name, _, _ := strings.Cut(pkg.PackageManager, "@")
	switch name {
	case "pnpm", "bun", "yarn", "npm":
		return name
	default:
		return ""
	}
}

func matchesFiles(root string, files []string) bool {
	for _, file := range files {
		if _, err := os.Stat(filepath.Join(root, file)); err == nil {
			return true
		}
	}
	return false
}
