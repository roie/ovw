package gitactivity

import (
	"os/exec"
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

func Detect(path string) Info {
	info := Info{}
	if _, err := run(path, "rev-parse", "--git-dir"); err != nil {
		return info
	}
	info.HasGit = true

	if branch, err := run(path, "rev-parse", "--abbrev-ref", "HEAD"); err == nil {
		info.Branch = strings.TrimSpace(branch)
		if info.Branch == "HEAD" {
			info.Branch = "detached"
		}
	}
	if ts, err := run(path, "log", "-1", "--format=%ct"); err == nil {
		trimmed := strings.TrimSpace(ts)
		if trimmed != "" {
			if seconds, parseErr := strconv.ParseInt(trimmed, 10, 64); parseErr == nil {
				info.LastCommitAt = time.Unix(seconds, 0)
				info.HasCommits = true
			}
		}
	}
	if message, err := run(path, "log", "-1", "--format=%B"); err == nil {
		info.LastCommitMessage = strings.TrimSpace(message)
	}
	if status, err := run(path, "status", "--porcelain", "--untracked-files=no"); err == nil {
		info.Dirty = strings.TrimSpace(status) != ""
	}
	if count, err := run(path, "rev-list", "--count", "@{upstream}..HEAD"); err == nil {
		if n, parseErr := strconv.Atoi(strings.TrimSpace(count)); parseErr == nil {
			info.Unpushed = n
		}
	}
	return info
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
	cmd := exec.Command("git", append([]string{"-C", path}, args...)...)
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
}
