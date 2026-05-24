package workspace

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"ovw/internal/config"
)

func CandidateRoots(root string) ([]string, error) {
	return CandidateRootsWithIgnore(root, config.Default().IgnoreDirs)
}

func CandidateRootsWithIgnore(root string, ignoreDirs []string) ([]string, error) {
	ignoreDirs = defaultIgnoreDirs(ignoreDirs)
	seen := map[string]bool{}
	roots := []string{}
	add := func(path string) {
		clean := filepath.Clean(path)
		if seen[clean] || IgnoredPathWithIgnore(root, clean, ignoreDirs) {
			return
		}
		if info, err := os.Stat(clean); err == nil && info.IsDir() {
			seen[clean] = true
			roots = append(roots, clean)
		}
	}
	add(root)

	globs, hasWorkspaceDeclaration, err := WorkspaceGlobs(root)
	if err != nil {
		return nil, err
	}
	if hasWorkspaceDeclaration {
		for _, glob := range globs {
			for _, match := range ExpandGlobWithIgnore(root, glob, ignoreDirs) {
				add(match)
			}
		}
		return roots, nil
	}

	for _, glob := range []string{"apps/*", "packages/*", "web/*", "extension/*", "extensions/*"} {
		for _, match := range ExpandGlobWithIgnore(root, glob, ignoreDirs) {
			add(match)
		}
	}
	return roots, nil
}

func WorkspaceGlobs(root string) ([]string, bool, error) {
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
		if inPackages && strings.HasPrefix(trimmed, "-") {
			value := cleanGlob(strings.TrimSpace(strings.TrimPrefix(trimmed, "-")))
			if value != "" && !strings.HasPrefix(value, "!") {
				globs = append(globs, value)
			}
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

func ExpandGlob(root, pattern string) []string {
	return ExpandGlobWithIgnore(root, pattern, config.Default().IgnoreDirs)
}

func ExpandGlobWithIgnore(root, pattern string, ignoreDirs []string) []string {
	ignoreDirs = defaultIgnoreDirs(ignoreDirs)
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
		if IgnoredPathWithIgnore(root, match, ignoreDirs) {
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

func IgnoredPath(root, path string) bool {
	return IgnoredPathWithIgnore(root, path, config.Default().IgnoreDirs)
}

func IgnoredPathWithIgnore(root, path string, ignoreDirs []string) bool {
	ignoreDirs = defaultIgnoreDirs(ignoreDirs)
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return true
	}
	if rel == "." {
		return false
	}
	for _, part := range strings.Split(filepath.ToSlash(rel), "/") {
		if ignoredDir(part, ignoreDirs) {
			return true
		}
	}
	return false
}

func ignoredDir(name string, ignoreDirs []string) bool {
	for _, ignored := range ignoreDirs {
		if name == ignored {
			return true
		}
	}
	return false
}

func defaultIgnoreDirs(ignoreDirs []string) []string {
	if ignoreDirs != nil {
		return ignoreDirs
	}
	return config.Default().IgnoreDirs
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
