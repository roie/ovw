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
	modal, _ := detailModalViewWithScroll(project, ok, width, 0, 0, false)
	return modal
}

func detailModalViewWithScroll(project project.Project, ok bool, width int, height int, offset int, expanded bool) (string, int) {
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
	lines := detailModalLinesWithWidth(project, ok, width-4, expanded)
	lines, maxOffset := scrollDetailModalLines(lines, height, offset, width-4)
	return modalView(title, lines, width), maxOffset
}

func detailModalLines(project project.Project, ok bool) []string {
	return detailModalLinesWithWidth(project, ok, 68, false)
}

func detailModalLinesWithWidth(project project.Project, ok bool, width int, expanded bool) []string {
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
	hasCompactContent := detailModalHasCompactContent(project)
	for _, field := range projectview.Fields(project, projectview.Options{Activity: projectview.DetailActivity}) {
		if field.Label == "Scripts" && !expanded {
			field.Value, _ = compactScripts(project.Scripts)
		}
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
	actions := []string{actionHint("x", visibilityAction(project))}
	if hasCompactContent {
		if expanded {
			actions = append(actions, actionHint("space", "collapse"))
		} else {
			actions = append(actions, actionHint("space", "expand"))
		}
	}
	lines = append(lines, "", strings.Join(actions, " · "))
	return lines
}

func detailModalHasCompactContent(project project.Project) bool {
	_, compacted := compactScripts(project.Scripts)
	return compacted
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
		if label == "Scripts" {
			value, _ = compactScriptsForWidth(project.Scripts, contentWidth-detailLinePrefixWidth)
		}
		if label == "Note" {
			addSummaryNote(&lines, value, width, project.Note.Source)
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

const maxFallbackSummaryNoteLines = 4
const compactScriptsLimit = 6
const detailLinePrefixWidth = 9

var compactScriptPriority = []string{
	"dev",
	"start",
	"build",
	"test",
	"lint",
	"check",
	"typecheck",
	"preview",
	"serve",
}

func compactScripts(scripts []string) (string, bool) {
	return compactScriptsForWidth(scripts, 0)
}

func compactScriptsForWidth(scripts []string, width int) (string, bool) {
	if len(scripts) <= compactScriptsLimit {
		return strings.Join(scripts, ", "), false
	}
	selected := make([]string, 0, compactScriptsLimit)
	seen := make(map[string]bool, compactScriptsLimit)
	add := func(script string) {
		if len(selected) >= compactScriptsLimit || seen[script] {
			return
		}
		for _, candidate := range scripts {
			if candidate == script {
				selected = append(selected, script)
				seen[script] = true
				return
			}
		}
	}
	for _, script := range compactScriptPriority {
		add(script)
	}
	for _, script := range scripts {
		if len(selected) >= compactScriptsLimit {
			break
		}
		if !seen[script] {
			selected = append(selected, script)
			seen[script] = true
		}
	}
	marker := fmt.Sprintf("+%d more", len(scripts)-len(selected))
	if width <= 0 {
		return strings.Join(selected, ", ") + " " + marker, true
	}
	for len(selected) > 0 {
		value := strings.Join(selected, ", ") + " " + marker
		if len([]rune(value)) <= width {
			return value, true
		}
		selected = selected[:len(selected)-1]
	}
	return marker, true
}

func addSummaryNote(lines *[]string, note string, width int, source string) {
	if note == "" {
		return
	}
	if len(*lines) > 1 {
		*lines = append(*lines, "")
	}
	*lines = append(*lines, truncateText("Note", width))
	noteLines := textwrap.Lines(note, width)
	if isFallbackNoteSource(source) && len(noteLines) > maxFallbackSummaryNoteLines {
		noteLines = append(noteLines[:maxFallbackSummaryNoteLines], truncateText("...", width))
	}
	*lines = append(*lines, noteLines...)
}

func isFallbackNoteSource(source string) bool {
	return source == "commit" || source == "description"
}

func scrollDetailModalLines(lines []string, height int, offset int, width int) ([]string, int) {
	if height <= 0 || len(lines)+4 <= height {
		return lines, 0
	}
	bodyHeight := height - 4
	if bodyHeight < 1 {
		bodyHeight = 1
	}
	visibleHeight := bodyHeight - 1
	if visibleHeight < 1 {
		visibleHeight = 1
	}
	maxOffset := len(lines) - visibleHeight
	if maxOffset < 0 {
		maxOffset = 0
	}
	if offset < 0 {
		offset = 0
	}
	if offset > maxOffset {
		offset = maxOffset
	}
	end := offset + visibleHeight
	if end > len(lines) {
		end = len(lines)
	}
	visible := append([]string{}, lines[offset:end]...)
	visible = append(visible, modalDetailScrollHint(offset, maxOffset, width))
	return visible, maxOffset
}

func modalDetailScrollHint(offset, maxOffset, width int) string {
	return scrollHint(offset, maxOffset, width, modalHintKey, modalMuted)
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
