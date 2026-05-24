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

	"ovw/internal/columns"
	"ovw/internal/safefile"

	"github.com/pelletier/go-toml/v2"
)

type Config struct {
	Roots                   []string      `toml:"roots"`
	MaxDepth                int           `toml:"max_depth"`
	ScanNestedProjects      bool          `toml:"scan_nested_projects"`
	IgnoreDirs              []string      `toml:"ignore_dirs"`
	ProjectMarkers          []string      `toml:"project_markers"`
	StaleDays               int           `toml:"stale_days"`
	ShowUnpushed            bool          `toml:"show_unpushed"`
	NoteFallbackCommit      bool          `toml:"note_fallback_commit"`
	NoteFallbackDescription bool          `toml:"note_fallback_description"`
	NoteShowBranch          bool          `toml:"note_show_branch"`
	DefaultBranches         []string      `toml:"default_branches"`
	Statuses                []string      `toml:"statuses"`
	Fields                  []FieldConfig `toml:"fields,omitempty"`
	Columns                 []string      `toml:"columns"`
	ColumnOrder             []string      `toml:"column_order,omitempty"`
	SortBy                  string        `toml:"sort_by"`
	SortDir                 string        `toml:"sort_dir"`
	Stack                   StackConfig   `toml:"stack"`
	Keys                    KeyConfig     `toml:"keys"`
	Editor                  string        `toml:"editor"`
	Shell                   string        `toml:"shell"`
}

type FieldConfig struct {
	ID      string   `toml:"id"`
	Label   string   `toml:"label"`
	Type    string   `toml:"type"`
	Options []string `toml:"options,omitempty"`
	Column  bool     `toml:"column,omitempty"`
}

type StackConfig struct {
	ShowUnknown bool              `toml:"show_unknown"`
	Aliases     map[string]string `toml:"aliases"`
}

type KeyConfig struct {
	Actions ActionKeyConfig `toml:"actions"`
}

type ActionKeyConfig struct {
	Details  string `toml:"details"`
	Editor   string `toml:"editor"`
	Terminal string `toml:"terminal"`
	Runner   string `toml:"runner"`
	Note     string `toml:"note"`
	Status   string `toml:"status"`
	Pin      string `toml:"pin"`
	Hide     string `toml:"hide"`
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
		Keys: KeyConfig{
			Actions: ActionKeyConfig{
				Details:  "enter",
				Editor:   "o",
				Terminal: "t",
				Runner:   "r",
				Note:     "n",
				Status:   "m",
				Pin:      "p",
				Hide:     "x",
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
		return Config{}, fmt.Errorf("invalid config %s: %w", path, err)
	}
	cfg = ApplyFieldColumnDefaults(cfg)
	if err := Validate(cfg); err != nil {
		return Config{}, fmt.Errorf("invalid config %s: %w", path, err)
	}
	return cfg, nil
}

func ApplyFieldColumnDefaults(cfg Config) Config {
	for _, field := range cfg.Fields {
		if !field.Column {
			continue
		}
		column := columns.FieldColumn(field.ID)
		if !stringSliceContains(cfg.Columns, column) {
			cfg.Columns = append(cfg.Columns, column)
		}
		if len(cfg.ColumnOrder) > 0 && !stringSliceContains(cfg.ColumnOrder, column) {
			cfg.ColumnOrder = append(cfg.ColumnOrder, column)
		}
	}
	return cfg
}

func stringSliceContains(values []string, value string) bool {
	for _, candidate := range values {
		if candidate == value {
			return true
		}
	}
	return false
}

func Validate(cfg Config) error {
	if len(cfg.Roots) == 0 {
		return errors.New("invalid roots: expected at least one root")
	}
	for _, root := range cfg.Roots {
		if strings.TrimSpace(root) == "" {
			return errors.New("invalid roots: root paths cannot be empty")
		}
	}
	if cfg.MaxDepth < 0 {
		return fmt.Errorf("invalid max_depth %d: expected 0 or greater", cfg.MaxDepth)
	}
	if cfg.StaleDays <= 0 {
		return fmt.Errorf("invalid stale_days %d: expected 1 or greater", cfg.StaleDays)
	}
	if err := validateFields(cfg.Fields); err != nil {
		return err
	}
	for _, column := range cfg.Columns {
		if err := validateColumn(cfg, "column", column); err != nil {
			return err
		}
	}
	for _, column := range cfg.ColumnOrder {
		if err := validateColumn(cfg, "column_order", column); err != nil {
			return err
		}
	}
	if duplicate := duplicateString(cfg.ColumnOrder); duplicate != "" {
		return fmt.Errorf("invalid column_order %q: already used", duplicate)
	}
	if !validSortBy(cfg.SortBy) {
		return fmt.Errorf("invalid sort_by %q: expected activity, updated, name, or status", cfg.SortBy)
	}
	if cfg.SortDir != "asc" && cfg.SortDir != "desc" {
		return fmt.Errorf("invalid sort_dir %q: expected asc or desc", cfg.SortDir)
	}
	if err := validateActionKeys(cfg.Keys.Actions); err != nil {
		return err
	}
	return nil
}

func duplicateString(values []string) string {
	seen := map[string]bool{}
	for _, value := range values {
		if seen[value] {
			return value
		}
		seen[value] = true
	}
	return ""
}

func validateColumn(cfg Config, name, column string) error {
	if !columns.Valid(column) {
		return fmt.Errorf("invalid %s %q: expected %s", name, column, columns.OptionsString())
	}
	fieldID := columns.FieldID(column)
	if fieldID == "" {
		return nil
	}
	if FieldByID(cfg, fieldID) == nil {
		return fmt.Errorf("invalid %s %q: custom field is not defined", name, column)
	}
	return nil
}

func validateFields(fields []FieldConfig) error {
	seen := map[string]bool{}
	for index, field := range fields {
		id := strings.TrimSpace(field.ID)
		if !validFieldID(id) {
			return fmt.Errorf("invalid fields[%d].id: expected lowercase letters, numbers, underscores, or hyphens", index)
		}
		if seen[id] {
			return fmt.Errorf("invalid fields[%d].id %q: already used", index, id)
		}
		seen[id] = true
		if !validFieldType(field.Type) {
			return fmt.Errorf("invalid fields[%d].type %q: expected text, select, or checkbox", index, field.Type)
		}
		if field.Type == "select" && len(field.Options) == 0 {
			return fmt.Errorf("invalid fields[%d].options: select fields need at least one option", index)
		}
	}
	return nil
}

func validFieldID(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' || r == '-' {
			continue
		}
		return false
	}
	return true
}

func validFieldType(value string) bool {
	switch value {
	case "text", "select", "checkbox":
		return true
	default:
		return false
	}
}

func FieldByID(cfg Config, id string) *FieldConfig {
	for index := range cfg.Fields {
		if cfg.Fields[index].ID == id {
			return &cfg.Fields[index]
		}
	}
	return nil
}

func FieldLabel(cfg Config, column string) string {
	fieldID := columns.FieldID(column)
	if fieldID == "" {
		return columns.Label(column)
	}
	field := FieldByID(cfg, fieldID)
	if field == nil || strings.TrimSpace(field.Label) == "" {
		return columns.Label(column)
	}
	return field.Label
}

func FieldColumns(cfg Config) []string {
	out := make([]string, 0, len(cfg.Fields))
	for _, field := range cfg.Fields {
		out = append(out, columns.FieldColumn(field.ID))
	}
	return out
}

func validSortBy(sortBy string) bool {
	switch sortBy {
	case "activity", "updated", "name", "status":
		return true
	default:
		return false
	}
}

func validateActionKeys(keys ActionKeyConfig) error {
	values := []struct {
		name string
		key  string
	}{
		{"details", keys.Details},
		{"editor", keys.Editor},
		{"terminal", keys.Terminal},
		{"runner", keys.Runner},
		{"note", keys.Note},
		{"status", keys.Status},
		{"pin", keys.Pin},
		{"hide", keys.Hide},
	}
	seen := map[string]string{}
	for _, value := range values {
		key := strings.TrimSpace(value.key)
		if key == "" {
			return fmt.Errorf("invalid keys.actions.%s: expected a key", value.name)
		}
		if reservedActionKey(key) {
			return fmt.Errorf("invalid keys.actions.%s %q: key is reserved", value.name, key)
		}
		if previous, ok := seen[key]; ok {
			return fmt.Errorf("invalid keys.actions.%s %q: already used by %s", value.name, key, previous)
		}
		seen[key] = value.name
	}
	return nil
}

func reservedActionKey(key string) bool {
	switch strings.ToLower(strings.TrimSpace(key)) {
	case "q", "ctrl+c", "?", "esc", "/", " ", "space",
		"up", "down", "left", "right", "h", "j", "k", "l",
		"ctrl+p", ":", "ctrl+r", "f5",
		"a", "f", "s", "c",
		"backspace", "ctrl+h", "delete", "ctrl+d",
		"ctrl+a", "ctrl+e", "ctrl+u", "ctrl+k", "ctrl+w":
		return true
	default:
		return false
	}
}

func Write(path string, cfg Config) error {
	data, err := toml.Marshal(cfg)
	if err != nil {
		return err
	}
	return safefile.AtomicWriteFile(path, data, 0o644)
}

func WriteDefault(path string, roots []string) error {
	return safefile.AtomicWriteFile(path, []byte(DefaultTemplate(roots)), 0o644)
}

func DefaultTemplate(roots []string) string {
	if len(roots) == 0 {
		roots = Default().Roots
	}
	return strings.Replace(defaultConfigTemplate, "{{ROOTS}}", formatStringList(roots, "  "), 1)
}

func RootCandidates(cwd string) ([]string, error) {
	values := []string{}
	add := func(value string) {
		if value == "" {
			return
		}
		expanded, err := ExpandPath(value)
		if err != nil {
			return
		}
		info, err := os.Stat(expanded)
		if err != nil || !info.IsDir() {
			return
		}
		for _, existing := range values {
			if existing == value {
				return
			}
			if existingExpanded, err := ExpandPath(existing); err == nil && filepath.Clean(existingExpanded) == filepath.Clean(expanded) {
				return
			}
		}
		values = append(values, value)
	}
	detected, err := DetectRoot(cwd)
	if err != nil {
		return nil, err
	}
	add(detected)
	for _, candidate := range []string{"~/dev", "~/Projects", "~/Code", "~/Developer"} {
		add(candidate)
	}
	if len(values) == 0 {
		values = append(values, "~/Projects")
	}
	return values, nil
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
		root = "~/Projects"
	}
	fmt.Fprintf(out, "Project root to scan [%s]: ", root)
	line, readErr := bufio.NewReader(in).ReadString('\n')
	if readErr != nil && !errors.Is(readErr, io.EOF) {
		return Config{}, false, readErr
	}
	if value := strings.TrimSpace(line); value != "" {
		root = value
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
	if err := Validate(cfg); err != nil {
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
	if strings.TrimSpace(path) == "" {
		return "", errors.New("path cannot be empty")
	}
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
	if strings.HasPrefix(path, "~") {
		return "", fmt.Errorf("unsupported home path %q; use ~/path", path)
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

const defaultConfigTemplate = `# ovw — A terminal overview for your local projects.
# Config location:
#   Linux/macOS: ~/.config/ovw/config.toml
#   Windows:     %AppData%\ovw\config.toml

# ─────────────────────────────────────────
# Roots
# ─────────────────────────────────────────

# Folders ovw scans for projects.
# Edit this list or run ovw config setup to choose roots again.
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
# [branch ·] user note OR last commit msg OR project description OR ""
note_fallback_commit = true
note_fallback_description = true

# Prefix branch name when not on a default branch.
note_show_branch = true

# Branch names considered default. Branch prefix hidden for these.
default_branches = ["main", "master", "trunk"]

# ─────────────────────────────────────────
# Status
# ─────────────────────────────────────────

# Suggested statuses. Status is free-form.
statuses = ["active", "parked", "shipped", "idea"]

# ─────────────────────────────────────────
# Fields
# ─────────────────────────────────────────

# Custom project metadata fields.
# Values live in projects.json under each project's "fields" object.
# Example:
# [[fields]]
# id = "jira"
# label = "Jira"
# type = "text"
# column = true

# ─────────────────────────────────────────
# Display
# ─────────────────────────────────────────

# Columns to show and their order.
# Options: name, path, stack, manager, scripts, version, ports, branch, updated, activity, status, note, field:<id>
columns = ["name", "stack", "activity", "status", "note"]

# Full column picker order, including hidden columns.
# Leave empty to follow the default picker order.
column_order = []

# Default sort column. Options: activity, updated, name, status
sort_by = "activity"

# Sort direction. Options: asc, desc
sort_dir = "desc"

# ─────────────────────────────────────────
# Editor and Shell
# ─────────────────────────────────────────

# Commands used by TUI open and terminal actions.
editor = "code"
shell = ""

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

# ─────────────────────────────────────────
# Keyboard Shortcuts
# ─────────────────────────────────────────

[keys.actions]
details = "enter"
editor = "o"
terminal = "t"
runner = "r"
note = "n"
status = "m"
pin = "p"
hide = "x"
`
