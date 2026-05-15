package format

import (
	"fmt"
	"strings"
	"time"

	"ovw/internal/config"
	"ovw/internal/gitactivity"
	"ovw/internal/projectfiles"
)

type ActivityInfo struct {
	Display           string         `json:"display"`
	LastCommitAge     string         `json:"last_commit_age"`
	LastCommitAt      time.Time      `json:"last_commit_at"`
	LastCommitMessage string         `json:"last_commit_message"`
	RecentCommits     []RecentCommit `json:"-"`
	Unpushed          int            `json:"unpushed"`
	Dirty             bool           `json:"dirty"`
	Branch            string         `json:"branch"`
	HasGit            bool           `json:"has_git"`
	HasCommits        bool           `json:"has_commits"`
}

type RecentCommit struct {
	Hash    string
	Subject string
	Age     string
}

type RecentFile struct {
	Path string
	Age  string
}

type NoteInfo struct {
	Display string `json:"display"`
	Value   string `json:"value"`
	Source  string `json:"source"`
}

type StatusInfo struct {
	Display string   `json:"display"`
	Value   string   `json:"value"`
	Tags    []string `json:"tags"`
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

func RecentCommits(commits []gitactivity.CommitInfo, now time.Time) []RecentCommit {
	out := make([]RecentCommit, 0, len(commits))
	for _, commit := range commits {
		out = append(out, RecentCommit{
			Hash:    commit.Hash,
			Subject: commit.Subject,
			Age:     RelativeAge(commit.At, now),
		})
	}
	return out
}

func RecentFiles(files []projectfiles.File, now time.Time) []RecentFile {
	out := make([]RecentFile, 0, len(files))
	for _, file := range files {
		out = append(out, RecentFile{
			Path: file.Path,
			Age:  RelativeAge(file.ModifiedAt, now),
		})
	}
	return out
}

func isStale(activity ActivityInfo, cfg config.Config, now time.Time) bool {
	if !activity.HasCommits || activity.LastCommitAt.IsZero() {
		return false
	}
	return now.Sub(activity.LastCommitAt) > time.Duration(cfg.StaleDays)*24*time.Hour
}

func Tags(activity ActivityInfo, status string, cfg config.Config, now time.Time) []string {
	if status != "" {
		return []string{status}
	}
	if !activity.HasGit {
		return []string{}
	}
	tags := []string{}
	if !activity.HasCommits {
		tags = append(tags, "no commits")
		return tags
	}
	if activity.Dirty {
		tags = append(tags, "dirty")
	}
	if isStale(activity, cfg, now) {
		tags = append(tags, "stale")
	} else if len(tags) == 0 {
		tags = append(tags, "active")
	}
	return tags
}

func TagDisplay(tags []string) string {
	return strings.Join(tags, " · ")
}

func Status(activity ActivityInfo, value string, cfg config.Config, now time.Time) StatusInfo {
	tags := Tags(activity, value, cfg, now)
	return StatusFromTags(value, tags)
}

func StatusFromTags(value string, tags []string) StatusInfo {
	return StatusInfo{
		Display: TagDisplay(tags),
		Value:   value,
		Tags:    tags,
	}
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

func Note(userNote, description string, activity gitactivity.Info, cfg config.Config) NoteInfo {
	note := NoteInfo{Value: userNote, Source: "none"}
	text := ""
	switch {
	case userNote != "":
		text = userNote
		note.Source = "user"
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
	if note.Source != "user" && cfg.NoteShowBranch && activity.Branch != "" && !isDefaultBranch(activity.Branch, cfg.DefaultBranches) {
		text = activity.Branch + " · " + text
	}
	note.Display = text
	return note
}

func SingleLine(value string) string {
	return strings.Join(strings.Fields(value), " ")
}

func isDefaultBranch(branch string, defaults []string) bool {
	for _, value := range defaults {
		if branch == value {
			return true
		}
	}
	return false
}
