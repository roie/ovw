package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	ovwformat "ovw/internal/format"
	"ovw/internal/project"
	"ovw/internal/projectview"
	"ovw/internal/textwrap"
)

func detailView(project project.Project, ok bool) string {
	if !ok {
		return mutedStyle.Render("No project selected")
	}
	lines := []string{titleStyle.Render(project.Name)}
	if subtitle := projectview.Subtitle(project); subtitle != "" {
		lines = append(lines, subtitle, "")
	}
	for _, field := range projectview.Fields(project, projectview.Options{Activity: projectview.DetailActivity}) {
		lines = append(lines, detailLine(field.Label, field.Value))
	}
	return strings.Join(lines, "\n")
}

func detailModalView(project project.Project, ok bool, width int) string {
	if width <= 0 || width > 72 {
		width = 72
	}
	if width < 32 {
		width = 32
	}
	title := "Details"
	if ok && project.Name != "" {
		title = project.Name
	}
	return modalView(title, detailModalLinesWithWidth(project, ok, width-4), width)
}

func detailModalLines(project project.Project, ok bool) []string {
	return detailModalLinesWithWidth(project, ok, 68)
}

func detailModalLinesWithWidth(project project.Project, ok bool, width int) []string {
	if !ok {
		return []string{modalMuted("No project selected")}
	}
	valueWidth := width - 11
	if valueWidth < 8 {
		valueWidth = width
	}
	var lines []string
	if subtitle := projectview.Subtitle(project); subtitle != "" {
		lines = append(lines, textwrap.Lines(subtitle, width)...)
		lines = append(lines, "")
	}
	for _, field := range projectview.Fields(project, projectview.Options{Activity: projectview.DetailActivity}) {
		if field.Value == "" {
			continue
		}
		wrapped := textwrap.Lines(field.Value, valueWidth)
		if len(wrapped) == 0 {
			continue
		}
		lines = append(lines, detailLine(field.Label, wrapped[0]))
		for _, line := range wrapped[1:] {
			lines = append(lines, detailLine("", line))
		}
	}
	lines = append(lines, "", actionHint("x", visibilityAction(project)))
	return lines
}

func visibilityAction(project project.Project) string {
	if project.Hidden {
		return "unhide"
	}
	return "hide"
}

func detailSummaryView(project project.Project, width int) string {
	contentWidth := width - 2
	if contentWidth < 1 {
		contentWidth = width
	}
	lines := []string{titleStyle.Render(truncateText(project.Name, width))}
	if subtitle := projectview.Subtitle(project); subtitle != "" {
		lines = append(lines, textwrap.Lines(subtitle, contentWidth)...)
	}
	addSummaryLine := func(field projectview.Field) {
		label := field.Label
		value := field.Value
		if value == "" {
			return
		}
		if label == "Note" {
			addSummaryNote(&lines, value, width)
			return
		}
		lines = append(lines, truncateText(detailLine(label, value), contentWidth))
	}
	fields := projectview.Fields(project, projectview.Options{
		Path:     shortPath,
		Activity: projectview.DetailActivity,
	})
	for _, field := range fields {
		addSummaryLine(field)
	}
	addRecentCommits(&lines, project.Activity.RecentCommits, contentWidth)
	return strings.Join(lines, "\n")
}

func addSummaryNote(lines *[]string, note string, width int) {
	if note == "" {
		return
	}
	if len(*lines) > 1 {
		*lines = append(*lines, "")
	}
	*lines = append(*lines, truncateText("Note", width))
	*lines = append(*lines, textwrap.Lines(note, width)...)
}

func addRecentCommits(lines *[]string, commits []ovwformat.RecentCommit, width int) {
	if len(commits) == 0 {
		return
	}
	if len(*lines) > 1 {
		*lines = append(*lines, "")
	}
	*lines = append(*lines, truncateText("Recent", width))
	for _, commit := range commits {
		*lines = append(*lines, recentCommitLine(commit, width))
	}
}

func recentCommitLine(commit ovwformat.RecentCommit, width int) string {
	if width <= 0 {
		return commit.Hash + "  " + commit.Subject + "  " + commit.Age
	}
	fixedWidth := len([]rune(commit.Hash)) + len([]rune(commit.Age)) + 4
	subjectWidth := width - fixedWidth
	if subjectWidth < 4 {
		return truncateText(commit.Hash+"  "+commit.Subject+"  "+commit.Age, width)
	}
	subject := truncateText(commit.Subject, subjectWidth)
	return fmt.Sprintf("%s  %-*s  %s", commit.Hash, subjectWidth, subject, commit.Age)
}

func shortPath(path string) string {
	if path == "" {
		return ""
	}
	home, err := os.UserHomeDir()
	if err == nil && home != "" {
		if rel, relErr := filepath.Rel(home, path); relErr == nil && rel != "." && !strings.HasPrefix(rel, "..") {
			return filepath.Join("~", rel)
		}
		if path == home {
			return "~"
		}
	}
	return path
}

func detailLine(label, value string) string {
	return fmt.Sprintf("%-8s %s", label, value)
}
