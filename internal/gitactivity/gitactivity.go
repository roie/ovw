package gitactivity

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type Info struct {
	LastCommitAt      time.Time
	LastCommitMessage string
	Unpushed          int
	Dirty             bool
	Branch            string
	HasGit            bool
	HasCommits        bool
}

type CommitInfo struct {
	Hash    string
	Subject string
	At      time.Time
}

type runner func(path string, args ...string) (string, error)

type Detector struct {
	run   runner
	cache map[string]Info
}

const defaultGitTimeout = 2 * time.Second

func NewDetector() *Detector {
	return newDetectorWithRunner(runWithTimeout(defaultGitTimeout), defaultGitTimeout)
}

func newDetectorWithRunner(run runner, _ time.Duration) *Detector {
	return &Detector{
		run:   run,
		cache: map[string]Info{},
	}
}

func Detect(path string) Info {
	return NewDetector().Detect(path)
}

func (detector *Detector) Detect(path string) Info {
	if _, info, ok := detector.cached(path); ok {
		return info
	}
	info := Info{}
	root, err := detector.gitRoot(path)
	if err != nil {
		return info
	}
	if cached, ok := detector.cache[root]; ok {
		return cached
	}
	info.HasGit = true

	if branch, err := detector.run(root, "rev-parse", "--abbrev-ref", "HEAD"); err == nil {
		info.Branch = strings.TrimSpace(branch)
		if info.Branch == "HEAD" {
			info.Branch = "detached"
		}
	}
	if ts, err := detector.run(root, "log", "-1", "--format=%ct"); err == nil {
		trimmed := strings.TrimSpace(ts)
		if trimmed != "" {
			if seconds, parseErr := strconv.ParseInt(trimmed, 10, 64); parseErr == nil {
				info.LastCommitAt = time.Unix(seconds, 0)
				info.HasCommits = true
			}
		}
	}
	if message, err := detector.run(root, "log", "-1", "--format=%B"); err == nil {
		info.LastCommitMessage = strings.TrimSpace(message)
	}
	if status, err := detector.run(root, "status", "--porcelain", "--untracked-files=no"); err == nil {
		info.Dirty = strings.TrimSpace(status) != ""
	}
	if count, err := detector.run(root, "rev-list", "--count", "@{upstream}..HEAD"); err == nil {
		if n, parseErr := strconv.Atoi(strings.TrimSpace(count)); parseErr == nil {
			info.Unpushed = n
		}
	}
	detector.cache[root] = info
	return info
}

func (detector *Detector) cached(path string) (string, Info, bool) {
	bestRoot := ""
	var bestInfo Info
	for root, info := range detector.cache {
		if pathContains(root, path) {
			if hasOwnGitMarker(path) && filepath.Clean(path) != filepath.Clean(root) {
				continue
			}
			if len(root) > len(bestRoot) {
				bestRoot = root
				bestInfo = info
			}
		}
	}
	return bestRoot, bestInfo, bestRoot != ""
}

func (detector *Detector) gitRoot(path string) (string, error) {
	root, err := detector.run(path, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(root), nil
}

func Recent(path string, limit int) ([]CommitInfo, error) {
	if limit <= 0 {
		return nil, nil
	}
	commits, err := run(path, "log", "-"+strconv.Itoa(limit), "--format=%h%x09%ct%x09%s")
	if err != nil {
		return nil, err
	}
	return parseRecentCommits(commits), nil
}

func parseRecentCommits(value string) []CommitInfo {
	lines := strings.Split(strings.TrimSpace(value), "\n")
	commits := []CommitInfo{}
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 3)
		if len(parts) != 3 {
			continue
		}
		seconds, err := strconv.ParseInt(parts[1], 10, 64)
		if err != nil {
			continue
		}
		commits = append(commits, CommitInfo{
			Hash:    strings.TrimSpace(parts[0]),
			Subject: strings.TrimSpace(parts[2]),
			At:      time.Unix(seconds, 0),
		})
	}
	return commits
}

func run(path string, args ...string) (string, error) {
	return runWithTimeout(defaultGitTimeout)(path, args...)
}

func runWithTimeout(timeout time.Duration) runner {
	return func(path string, args ...string) (string, error) {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		cmd := exec.CommandContext(ctx, "git", append([]string{"-C", path}, args...)...)
		out, err := cmd.Output()
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		if err != nil {
			return "", err
		}
		return string(out), nil
	}
}

func pathContains(parent, child string) bool {
	rel, err := filepath.Rel(parent, child)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}

func hasOwnGitMarker(path string) bool {
	_, err := os.Stat(filepath.Join(path, ".git"))
	return err == nil
}
