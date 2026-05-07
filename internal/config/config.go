package config

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

type Config struct {
	Roots                   []string    `toml:"roots"`
	MaxDepth                int         `toml:"max_depth"`
	ScanNestedProjects      bool        `toml:"scan_nested_projects"`
	IgnoreDirs              []string    `toml:"ignore_dirs"`
	ProjectMarkers          []string    `toml:"project_markers"`
	StaleDays               int         `toml:"stale_days"`
	ShowUnpushed            bool        `toml:"show_unpushed"`
	NoteFallbackCommit      bool        `toml:"note_fallback_commit"`
	NoteFallbackDescription bool        `toml:"note_fallback_description"`
	NoteShowBranch          bool        `toml:"note_show_branch"`
	DefaultBranches         []string    `toml:"default_branches"`
	Statuses                []string    `toml:"statuses"`
	Columns                 []string    `toml:"columns"`
	SortBy                  string      `toml:"sort_by"`
	SortDir                 string      `toml:"sort_dir"`
	Stack                   StackConfig `toml:"stack"`
	Editor                  string      `toml:"editor"`
	Shell                   string      `toml:"shell"`
}

type StackConfig struct {
	ShowUnknown bool              `toml:"show_unknown"`
	Aliases     map[string]string `toml:"aliases"`
}

type FilePaths struct {
	Config   string
	Metadata string
}

func Default() Config {
	return Config{
		Roots:              []string{"~/Code", "~/Projects"},
		MaxDepth:           0,
		ScanNestedProjects: false,
		IgnoreDirs: []string{
			"node_modules", ".git", "dist", "build", "target", ".next", ".nuxt",
			".svelte-kit", ".turbo", ".cache", "coverage", "vendor", ".venv",
			"venv", "__pycache__",
		},
		ProjectMarkers: []string{
			".git", "package.json", "Cargo.toml", "go.mod", "pyproject.toml", "deno.json", "bun.lock", "bun.lockb",
		},
		StaleDays:               30,
		ShowUnpushed:            true,
		NoteFallbackCommit:      true,
		NoteFallbackDescription: true,
		NoteShowBranch:          true,
		DefaultBranches:         []string{"main", "master", "trunk"},
		Statuses:                []string{"active", "parked", "shipped", "idea"},
		Columns:                 []string{"name", "stack", "activity", "status", "note"},
		SortBy:                  "activity",
		SortDir:                 "desc",
		Stack: StackConfig{
			ShowUnknown: true,
			Aliases: map[string]string{
				"Cloudflare Workers": "CF",
				"React Native":       "RN",
				"TypeScript":         "TS",
				"JavaScript":         "JS",
			},
		},
		Editor: "code",
		Shell:  "",
	}
}

func Paths() (FilePaths, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return FilePaths{}, err
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return FilePaths{}, err
	}
	if runtime.GOOS == "windows" {
		local := os.Getenv("LocalAppData")
		if local == "" {
			local = filepath.Join(home, "AppData", "Local")
		}
		return FilePaths{
			Config:   filepath.Join(configDir, "ovw", "config.toml"),
			Metadata: filepath.Join(local, "ovw", "projects.json"),
		}, nil
	}
	dataDir := os.Getenv("XDG_DATA_HOME")
	if dataDir == "" {
		dataDir = filepath.Join(home, ".local", "share")
	}
	return FilePaths{
		Config:   filepath.Join(configDir, "ovw", "config.toml"),
		Metadata: filepath.Join(dataDir, "ovw", "projects.json"),
	}, nil
}

func Load(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}
	cfg := Default()
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func Write(path string, cfg Config) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := toml.Marshal(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func WriteDefault(path string, roots []string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(DefaultTemplate(roots)), 0o644)
}

func DefaultTemplate(roots []string) string {
	if len(roots) == 0 {
		roots = Default().Roots
	}
	return strings.Replace(defaultConfigTemplate, "{{ROOTS}}", formatStringList(roots, "  "), 1)
}

func Ensure(path, cwd string, in io.Reader, out io.Writer) (Config, bool, error) {
	cfg, err := Load(path)
	if err == nil {
		return cfg, false, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return Config{}, false, err
	}
	cfg = Default()
	root, err := DetectRoot(cwd)
	if err != nil {
		return Config{}, false, err
	}
	if root == "" {
		fmt.Fprintln(out, "No config found.")
		fmt.Fprint(out, "Where are your projects? [~/Projects]: ")
		line, readErr := bufio.NewReader(in).ReadString('\n')
		if readErr != nil && !errors.Is(readErr, io.EOF) {
			return Config{}, false, readErr
		}
		root = strings.TrimSpace(line)
		if root == "" {
			root = "~/Projects"
		}
	}
	expanded, err := ExpandPath(root)
	if err != nil {
		return Config{}, false, err
	}
	info, err := os.Stat(expanded)
	if err != nil {
		return Config{}, false, err
	}
	if !info.IsDir() {
		return Config{}, false, fmt.Errorf("%s is not a directory", root)
	}
	cfg.Roots = []string{root}
	if err := WriteDefault(path, cfg.Roots); err != nil {
		return Config{}, false, err
	}
	return cfg, true, nil
}

func DetectRoot(cwd string) (string, error) {
	if cwd != "" && cwdHasTwoProjectChildren(cwd, Default().ProjectMarkers) {
		return cwd, nil
	}
	for _, candidate := range []string{"~/Code", "~/Projects", "~/Developer", "~/dev"} {
		expanded, err := ExpandPath(candidate)
		if err == nil {
			if info, statErr := os.Stat(expanded); statErr == nil && info.IsDir() {
				return candidate, nil
			}
		}
	}
	return "", nil
}

func Edit(path string) error {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vi"
	}
	cmd := exec.Command(editor, path)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func ExpandPath(path string) (string, error) {
	if path == "~" || strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		if path == "~" {
			return home, nil
		}
		return filepath.Join(home, path[2:]), nil
	}
	return filepath.Abs(path)
}

func cwdHasTwoProjectChildren(cwd string, markers []string) bool {
	entries, err := os.ReadDir(cwd)
	if err != nil {
		return false
	}
	count := 0
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		child := filepath.Join(cwd, entry.Name())
		for _, marker := range markers {
			if _, err := os.Stat(filepath.Join(child, marker)); err == nil {
				count++
				break
			}
		}
		if count >= 2 {
			return true
		}
	}
	return false
}

func formatStringList(values []string, indent string) string {
	lines := make([]string, 0, len(values))
	for _, value := range values {
		lines = append(lines, indent+strconv.Quote(value))
	}
	return strings.Join(lines, ",\n")
}

const defaultConfigTemplate = `# ovw — local project overview
# Config location:
#   Linux/macOS: ~/.config/ovw/config.toml
#   Windows:     %AppData%\ovw\config.toml

# ─────────────────────────────────────────
# Roots
# ─────────────────────────────────────────

# Folders ovw will scan for projects.
# Auto-detected on first run if not set.
roots = [
{{ROOTS}}
]

# ─────────────────────────────────────────
# Scanning
# ─────────────────────────────────────────

# Maximum folder depth. 0 = no limit.
max_depth = 0

# Stop scanning inside a folder once it is detected as a project.
# Prevents monorepo packages from exploding into separate rows.
scan_nested_projects = false

# Folders to never scan inside.
ignore_dirs = [
  "node_modules",
  ".git",
  "dist",
  "build",
  "target",
  ".next",
  ".nuxt",
  ".svelte-kit",
  ".turbo",
  ".cache",
  "coverage",
  "vendor",
  ".venv",
  "venv",
  "__pycache__"
]

# Files or folders that mark a directory as a project.
project_markers = [
  ".git",
  "package.json",
  "Cargo.toml",
  "go.mod",
  "pyproject.toml",
  "deno.json",
  "bun.lock",
  "bun.lockb"
]

# ─────────────────────────────────────────
# Activity
# ─────────────────────────────────────────

# Days before a project is considered stale.
stale_days = 30

# Show unpushed commit count in activity column.
show_unpushed = true

# ─────────────────────────────────────────
# Note column
# ─────────────────────────────────────────

# Fallback chain for note column:
# [branch ·] manual note OR last commit msg OR project description OR ""
note_fallback_commit = true
note_fallback_description = true

# Prefix branch name when not on a default branch.
note_show_branch = true

# Branch names considered default. Branch prefix hidden for these.
default_branches = ["main", "master", "trunk"]

# ─────────────────────────────────────────
# Status
# ─────────────────────────────────────────

# Suggested manual statuses. Status is free-form.
statuses = ["active", "parked", "shipped", "idea"]

# ─────────────────────────────────────────
# Display
# ─────────────────────────────────────────

# Columns to show and their order.
columns = ["name", "stack", "activity", "status", "note"]

# Default sort column. Options: activity, name, status
sort_by = "activity"

# Sort direction: asc or desc
sort_dir = "desc"

# ─────────────────────────────────────────
# Stack
# ─────────────────────────────────────────

[stack]
show_unknown = true

[stack.aliases]
"Cloudflare Workers" = "CF"
"React Native"       = "RN"
"TypeScript"         = "TS"
"JavaScript"         = "JS"

# Phase 2: user-defined stack rules
# [[stack.rules]]
# name = "SvelteKit"
# packages = ["@sveltejs/kit"]
# files = ["svelte.config.js", "svelte.config.ts"]

# ─────────────────────────────────────────
# Editor and Shell
# ─────────────────────────────────────────

# Used by future TUI actions.
editor = "code"
shell = ""
`
