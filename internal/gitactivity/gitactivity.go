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

func Detect(path string) Info {
	info := Info{}
	if _, err := run(path, "rev-parse", "--git-dir"); err != nil {
		return info
	}
	info.HasGit = true

	if branch, err := run(path, "rev-parse", "--abbrev-ref", "HEAD"); err == nil {
		info.Branch = strings.TrimSpace(branch)
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
	if message, err := run(path, "log", "-1", "--format=%s"); err == nil {
		info.LastCommitMessage = strings.TrimSpace(message)
	}
	if status, err := run(path, "status", "--porcelain"); err == nil {
		info.Dirty = strings.TrimSpace(status) != ""
	}
	if count, err := run(path, "rev-list", "--count", "@{upstream}..HEAD"); err == nil {
		if n, parseErr := strconv.Atoi(strings.TrimSpace(count)); parseErr == nil {
			info.Unpushed = n
		}
	}
	return info
}

func run(path string, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", path}, args...)...)
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
}
