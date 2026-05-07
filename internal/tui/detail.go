package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	ovwformat "ovw/internal/format"
	"ovw/internal/project"
	"ovw/internal/projectview"
)

func detailView(project project.Project, ok bool) string {
	if !ok {
		return mutedStyle.Render("No project selected")
	}
	lines := []string{titleStyle.Render(project.Name)}
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
	return modalView("Details", detailModalLines(project, ok), width)
}

func detailModalLines(project project.Project, ok bool) []string {
	if !ok {
		return []string{modalMuted("No project selected")}
	}
	lines := strings.Split(detailView(project, ok), "\n")
	if len(lines) > 0 {
		lines[0] = project.Name
	}
	return lines
}

func detailSummaryView(project project.Project, width int) string {
	lines := []string{titleStyle.Render(truncateText(project.Name, width))}
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
		lines = append(lines, truncateText(detailLine(label, value), width))
	}
	fields := projectview.Fields(project, projectview.Options{
		Path:     shortPath,
		Activity: projectview.DetailActivity,
	})
	for _, field := range fields {
		addSummaryLine(field)
	}
	addRecentCommits(&lines, project.Activity.RecentCommits, width)
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
	*lines = append(*lines, wrapText(note, width)...)
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

func wrapText(value string, width int) []string {
	if width <= 0 {
		return []string{value}
	}
	paragraphs := strings.Split(value, "\n")
	lines := []string{}
	for _, paragraph := range paragraphs {
		lines = append(lines, wrapTextLine(paragraph, width)...)
	}
	return lines
}

func wrapTextLine(value string, width int) []string {
	if len([]rune(value)) <= width {
		return []string{value}
	}
	words := strings.Fields(value)
	if len(words) == 0 {
		return []string{""}
	}
	lines := []string{}
	line := ""
	for _, word := range words {
		if line == "" {
			line = word
			continue
		}
		if len([]rune(line))+1+len([]rune(word)) <= width {
			line += " " + word
			continue
		}
		lines = append(lines, line)
		line = word
	}
	if line != "" {
		lines = append(lines, line)
	}
	return lines
}
