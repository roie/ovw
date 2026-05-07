package format

import (
	"fmt"
	"strings"
	"time"

	"ovw/internal/config"
	"ovw/internal/gitactivity"
)

type ActivityInfo struct {
	Display           string    `json:"display"`
	LastCommitAge     string    `json:"last_commit_age"`
	LastCommitAt      time.Time `json:"last_commit_at"`
	LastCommitMessage string    `json:"last_commit_message"`
	Unpushed          int       `json:"unpushed"`
	Dirty             bool      `json:"dirty"`
	Branch            string    `json:"branch"`
	HasGit            bool      `json:"has_git"`
	HasCommits        bool      `json:"has_commits"`
}

type NoteInfo struct {
	Display string `json:"display"`
	Manual  string `json:"manual"`
	Source  string `json:"source"`
}

func Activity(info gitactivity.Info, cfg config.Config, now time.Time) ActivityInfo {
	out := ActivityInfo{
		LastCommitAt:      info.LastCommitAt,
		LastCommitMessage: info.LastCommitMessage,
		Unpushed:          info.Unpushed,
		Dirty:             info.Dirty,
		Branch:            info.Branch,
		HasGit:            info.HasGit,
		HasCommits:        info.HasCommits,
	}
	if !info.HasGit {
		out.Display = "—"
		return out
	}
	if !info.HasCommits {
		out.Display = "no commits"
		return out
	}
	out.LastCommitAge = RelativeAge(info.LastCommitAt, now)
	parts := []string{out.LastCommitAge}
	if cfg.ShowUnpushed && info.Unpushed > 0 {
		parts = append(parts, fmt.Sprintf("↑%d", info.Unpushed))
	}
	out.Display = strings.Join(parts, " ")
	return out
}

func isStale(activity ActivityInfo, cfg config.Config, now time.Time) bool {
	if !activity.HasCommits || activity.LastCommitAt.IsZero() {
		return false
	}
	return now.Sub(activity.LastCommitAt) > time.Duration(cfg.StaleDays)*24*time.Hour
}

func Tags(activity ActivityInfo, status string, cfg config.Config, now time.Time) []string {
	if !activity.HasGit {
		return appendManualTag([]string{}, status)
	}
	tags := []string{}
	if !activity.HasCommits {
		tags = append(tags, "no commits")
		return appendManualTag(tags, status)
	}
	if activity.Dirty {
		tags = append(tags, "dirty")
	}
	if activity.Unpushed > 0 {
		tags = append(tags, "unpushed")
	}
	if isStale(activity, cfg, now) {
		tags = append(tags, "stale")
	} else {
		tags = append(tags, "active")
	}
	return appendManualTag(tags, status)
}

func appendManualTag(tags []string, status string) []string {
	if status == "" {
		return tags
	}
	for _, tag := range tags {
		if tag == status {
			return tags
		}
	}
	return append(tags, status)
}

func TagDisplay(tags []string) string {
	return strings.Join(tags, " · ")
}

func RelativeAge(then, now time.Time) string {
	if then.IsZero() {
		return ""
	}
	if then.After(now) {
		then = now
	}
	elapsed := now.Sub(then)
	minutes := int(elapsed.Minutes())
	hours := int(elapsed.Hours())
	days := hours / 24
	switch {
	case minutes < 1:
		return "now"
	case minutes < 60:
		return fmt.Sprintf("%dm", minutes)
	case hours < 24:
		return fmt.Sprintf("%dh", hours)
	case days < 7:
		return fmt.Sprintf("%dd", days)
	case days < 30:
		return fmt.Sprintf("%dw", days/7)
	case days < 365:
		return fmt.Sprintf("%dmo", days/30)
	default:
		return fmt.Sprintf("%dy", days/365)
	}
}

func Note(manual, description string, activity gitactivity.Info, cfg config.Config) NoteInfo {
	note := NoteInfo{Manual: manual, Source: "none"}
	text := ""
	switch {
	case manual != "":
		text = manual
		note.Source = "manual"
	case cfg.NoteFallbackCommit && activity.LastCommitMessage != "":
		text = activity.LastCommitMessage
		note.Source = "commit"
	case cfg.NoteFallbackDescription && description != "":
		text = description
		note.Source = "description"
	}
	if text == "" {
		return note
	}
	if cfg.NoteShowBranch && activity.Branch != "" && !isDefaultBranch(activity.Branch, cfg.DefaultBranches) {
		text = activity.Branch + " · " + text
	}
	note.Display = text
	return note
}

func isDefaultBranch(branch string, defaults []string) bool {
	for _, value := range defaults {
		if branch == value {
			return true
		}
	}
	return false
}
