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
	if cfg.ShowDirty && info.Dirty {
		parts = append(parts, "!")
	}
	out.Display = strings.Join(parts, " ")
	return out
}

func RelativeAge(then, now time.Time) string {
	if then.IsZero() {
		return ""
	}
	if then.After(now) {
		then = now
	}
	days := int(now.Sub(then).Hours() / 24)
	switch {
	case days < 1:
		return "0d"
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
