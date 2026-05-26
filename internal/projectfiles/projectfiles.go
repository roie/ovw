package projectfiles

import (
	"bufio"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const gitFilesTimeout = 1200 * time.Millisecond

type Options struct {
	IgnoreDirs []string
}

type File struct {
	Path       string
	ModifiedAt time.Time
}

func Recent(root string, opts Options, limit int) ([]File, error) {
	if limit <= 0 {
		return nil, nil
	}
	if files, ok := gitRecent(root, opts, limit); ok {
		return files, nil
	}
	files, err := walk(root, opts)
	if err != nil {
		return nil, err
	}
	return newestFiles(files, limit), nil
}

func gitRecent(root string, opts Options, limit int) ([]File, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), gitFilesTimeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, "git", "-C", root, "ls-files", "--cached", "--others", "--exclude-standard", "-z").Output()
	if err != nil || ctx.Err() != nil {
		return nil, false
	}
	files := []File{}
	ignored := updatedIgnoredDirs(opts)
	for _, rel := range strings.Split(string(out), "\x00") {
		if rel == "" {
			continue
		}
		if hasIgnoredDir(rel, ignored) {
			continue
		}
		info, err := os.Stat(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil || info.IsDir() {
			continue
		}
		files = append(files, File{Path: filepath.ToSlash(rel), ModifiedAt: info.ModTime()})
	}
	return newestFiles(files, limit), true
}

func newestFiles(files []File, limit int) []File {
	sort.Slice(files, func(i, j int) bool {
		if files[i].ModifiedAt.Equal(files[j].ModifiedAt) {
			return files[i].Path < files[j].Path
		}
		return files[i].ModifiedAt.After(files[j].ModifiedAt)
	})
	if len(files) > limit {
		files = files[:limit]
	}
	return files
}

func NewestModified(root string, opts Options) (time.Time, bool, error) {
	files, err := walk(root, opts)
	if err != nil {
		return time.Time{}, false, err
	}
	var newest time.Time
	ok := false
	for _, file := range files {
		if !ok || file.ModifiedAt.After(newest) {
			newest = file.ModifiedAt
			ok = true
		}
	}
	return newest, ok, nil
}

func updatedIgnoredDirs(opts Options) map[string]bool {
	ignored := map[string]bool{".git": true}
	for _, dir := range opts.IgnoreDirs {
		ignored[dir] = true
	}
	return ignored
}

func hasIgnoredDir(rel string, ignored map[string]bool) bool {
	for _, part := range strings.Split(filepath.ToSlash(rel), "/") {
		if ignored[part] {
			return true
		}
	}
	return false
}

func walk(root string, opts Options) ([]File, error) {
	files := []File{}
	ignored := updatedIgnoredDirs(opts)
	var walkDir func(string, []ignoreRule) error
	walkDir = func(dir string, rules []ignoreRule) error {
		rules = append(rules, readIgnoreRules(root, dir)...)
		entries, err := os.ReadDir(dir)
		if err != nil {
			return err
		}
		for _, entry := range entries {
			name := entry.Name()
			path := filepath.Join(dir, name)
			rel, err := filepath.Rel(root, path)
			if err != nil {
				continue
			}
			rel = filepath.ToSlash(rel)
			isDir := entry.IsDir()
			if isDir {
				if ignored[name] || strings.HasPrefix(name, ".") || ignorePath(rel, true, rules) {
					continue
				}
				if err := walkDir(path, rules); err != nil {
					return err
				}
				continue
			}
			if ignorePath(rel, false, rules) {
				continue
			}
			info, err := entry.Info()
			if err != nil {
				continue
			}
			files = append(files, File{
				Path:       rel,
				ModifiedAt: info.ModTime(),
			})
		}
		return nil
	}
	if err := walkDir(root, nil); err != nil {
		return nil, err
	}
	return files, nil
}

type ignoreRule struct {
	pattern   string
	base      string
	negated   bool
	directory bool
	anchored  bool
	hasGlob   bool
}

func readIgnoreRules(root, dir string) []ignoreRule {
	file, err := os.Open(filepath.Join(dir, ".gitignore"))
	if err != nil {
		return nil
	}
	defer file.Close()
	base, err := filepath.Rel(root, dir)
	if err != nil {
		return nil
	}
	base = filepath.ToSlash(base)
	if base == "." {
		base = ""
	}
	rules := []ignoreRule{}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		rule, ok := parseIgnoreRule(scanner.Text(), base)
		if ok {
			rules = append(rules, rule)
		}
	}
	return rules
}

func parseIgnoreRule(line, base string) (ignoreRule, bool) {
	line = strings.TrimSpace(line)
	if line == "" || strings.HasPrefix(line, "#") {
		return ignoreRule{}, false
	}
	rule := ignoreRule{base: base}
	if strings.HasPrefix(line, "\\#") || strings.HasPrefix(line, "\\!") {
		line = line[1:]
	} else if strings.HasPrefix(line, "!") {
		rule.negated = true
		line = strings.TrimPrefix(line, "!")
	}
	line = strings.TrimSpace(line)
	if line == "" {
		return ignoreRule{}, false
	}
	if strings.HasSuffix(line, "/") {
		rule.directory = true
		line = strings.TrimSuffix(line, "/")
	}
	if strings.HasPrefix(line, "/") {
		rule.anchored = true
		line = strings.TrimPrefix(line, "/")
	}
	rule.pattern = filepath.ToSlash(line)
	rule.hasGlob = strings.ContainsAny(rule.pattern, "*?[")
	return rule, true
}

func ignorePath(rel string, isDir bool, rules []ignoreRule) bool {
	ignored := false
	for _, rule := range rules {
		if rule.matches(rel, isDir) {
			ignored = !rule.negated
		}
	}
	return ignored
}

func (rule ignoreRule) matches(rel string, isDir bool) bool {
	if rule.directory && !isDir {
		return false
	}
	candidate := rel
	if rule.base != "" {
		if rel != rule.base && !strings.HasPrefix(rel, rule.base+"/") {
			return false
		}
		candidate = strings.TrimPrefix(rel, rule.base+"/")
	}
	if rule.anchored || strings.Contains(rule.pattern, "/") {
		return patternMatch(rule, candidate)
	}
	for {
		if patternMatch(rule, candidate) {
			return true
		}
		index := strings.Index(candidate, "/")
		if index < 0 {
			break
		}
		candidate = candidate[index+1:]
	}
	return false
}

func patternMatch(rule ignoreRule, value string) bool {
	if !rule.hasGlob {
		return rule.pattern == value
	}
	matched, err := filepath.Match(rule.pattern, value)
	return err == nil && matched
}
