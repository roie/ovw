package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"ovw/internal/config"
	ovwformat "ovw/internal/format"
	"ovw/internal/project"
	"ovw/internal/projectview"
	"ovw/internal/textwrap"
)

func detailView(project project.Project, ok bool) string {
	if !ok {
		return mutedStyle.Render("No project selected")
	}
	lines := []string{titleStyle.Render(projectTitle(project))}
	if subtitle := projectview.Subtitle(project); subtitle != "" {
		lines = append(lines, subtitle, "")
	}
	for _, field := range detailFields(project, projectview.DetailActivity) {
		lines = append(lines, detailLine(field.Label, field.Value))
	}
	return strings.Join(lines, "\n")
}

func detailModalView(project project.Project, ok bool, width int) string {
	modal, _ := detailModalViewWithScroll(project, ok, width, 0, 0, false)
	return modal
}

func detailModalViewWithScroll(project project.Project, ok bool, width int, height int, offset int, expanded bool, actionKeys ...config.ActionKeyConfig) (string, int) {
	return detailModalViewWithSelectionScroll(project, ok, width, height, offset, expanded, -1, actionKeys...)
}

func detailModalViewWithSelectionScroll(project project.Project, ok bool, width int, height int, offset int, expanded bool, selected int, actionKeys ...config.ActionKeyConfig) (string, int) {
	if width <= 0 {
		width = 72
	}
	if width > 100 {
		width = 100
	}
	if width < 32 {
		width = 32
	}
	title := "Details"
	if ok && project.Name != "" {
		title = projectTitle(project)
	}
	keys := config.Default().Keys.Actions
	if len(actionKeys) > 0 {
		keys = actionKeys[0]
	}
	lines := detailModalLinesWithSelectionRows(project, ok, width-4, expanded, selected, keys)
	lines, maxOffset := scrollDetailModalLines(lines, height, offset, width-4)
	return modalView(title, lines, width), maxOffset
}

func detailModalLines(project project.Project, ok bool) []string {
	return detailModalLinesWithWidth(project, ok, 68, false)
}

func detailModalLinesWithWidth(project project.Project, ok bool, width int, expanded bool, actionKeys ...config.ActionKeyConfig) []string {
	keys := config.Default().Keys.Actions
	if len(actionKeys) > 0 {
		keys = actionKeys[0]
	}
	return detailModalLinesWithSelectionRows(project, ok, width, expanded, -1, keys)
}

func detailModalLinesWithSelection(project project.Project, ok bool, width int, expanded bool, selected int, keys config.ActionKeyConfig) string {
	return strings.Join(detailModalLinesWithSelectionRows(project, ok, width, expanded, selected, keys), "\n")
}

func detailModalLinesWithSelectionRows(project project.Project, ok bool, width int, expanded bool, selected int, keys config.ActionKeyConfig) []string {
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
	rows := detailRows(project, projectview.DetailActivity)
	selectedRow := selectedDetailRowIndex(selected, rows)
	for index, row := range rows {
		if row.Label == "Scripts" && !expanded {
			row.Value, _ = compactScripts(project.Scripts)
		}
		if row.Value == "" && row.EditKind == detailEditNone {
			continue
		}
		wrapped := textwrap.Lines(row.Value, valueWidth)
		if row.Value == "" && row.EditKind != detailEditNone {
			wrapped = []string{""}
		}
		if len(wrapped) == 0 {
			continue
		}
		line := detailLine(row.Label, wrapped[0])
		if index == selectedRow {
			line = modalAccent("> " + line)
		}
		lines = append(lines, line)
		for _, line := range wrapped[1:] {
			lines = append(lines, detailLine("", line))
		}
	}
	primaryActions := []string{
		actionHint(keys.Editor, "open"),
		actionHint(keys.Terminal, "terminal"),
	}
	if enterHint := detailEnterHint(rows, selected); enterHint != "" {
		primaryActions = append(primaryActions, enterHint)
	} else {
		primaryActions = append(primaryActions, actionHint(keys.Note, "note"), actionHint(keys.Status, "status"))
	}
	secondaryActions := []string{actionHint(keys.Hide, visibilityAction(project)), actionHint(keys.Pin, pinAction(project))}
	if hasCompactContent {
		if expanded {
			secondaryActions = append(secondaryActions, actionHint("space", "collapse"))
		} else {
			secondaryActions = append(secondaryActions, actionHint("space", "expand"))
		}
	}
	lines = append(lines, "", strings.Join(primaryActions, " · "), strings.Join(secondaryActions, " · "))
	return lines
}

func editableDetailRowIndexes(rows []detailRow) []int {
	indexes := []int{}
	for index, row := range rows {
		if row.EditKind != detailEditNone {
			indexes = append(indexes, index)
		}
	}
	return indexes
}

func selectedDetailRowIndex(selected int, rows []detailRow) int {
	editable := editableDetailRowIndexes(rows)
	if len(editable) == 0 || selected < 0 {
		return -1
	}
	return editable[clampIndex(selected, len(editable))]
}

func detailEnterHint(rows []detailRow, selected int) string {
	rowIndex := selectedDetailRowIndex(selected, rows)
	if rowIndex < 0 {
		return ""
	}
	switch rows[rowIndex].EditKind {
	case detailEditStatus, detailEditCustomSelect, detailEditCustomCheckbox:
		return actionHint("←→", "change")
	default:
		return actionHint("enter", "edit")
	}
}

func detailModalHasCompactContent(project project.Project) bool {
	_, compacted := compactScripts(project.Scripts)
	return compacted
}

type detailEditKind int

const (
	detailEditNone detailEditKind = iota
	detailEditNote
	detailEditStatus
	detailEditCustomText
	detailEditCustomSelect
	detailEditCustomCheckbox
)

type detailRow struct {
	Label    string
	Value    string
	FieldID  string
	Options  []string
	EditKind detailEditKind
}

func detailRows(project project.Project, activity func(project.Project) string) []detailRow {
	fields := detailFields(project, activity)
	rows := make([]detailRow, 0, len(fields))
	for _, field := range fields {
		rows = append(rows, detailRow{
			Label:    field.Label,
			Value:    field.Value,
			FieldID:  field.ID,
			Options:  append([]string(nil), field.Options...),
			EditKind: detailEditKindForField(field),
		})
	}
	return rows
}

func detailEditKindForField(field projectview.Field) detailEditKind {
	if field.ID != "" {
		return detailCustomEditKind(field.Type)
	}
	switch field.Label {
	case "Note":
		return detailEditNote
	case "Status":
		return detailEditStatus
	default:
		return detailEditNone
	}
}

func detailCustomEditKind(fieldType string) detailEditKind {
	switch fieldType {
	case "text":
		return detailEditCustomText
	case "select":
		return detailEditCustomSelect
	case "checkbox":
		return detailEditCustomCheckbox
	default:
		return detailEditNone
	}
}

func detailFields(project project.Project, activity func(project.Project) string) []projectview.Field {
	return projectview.Fields(project, detailOptions(nil, activity))
}

func detailOptions(path func(string) string, activity func(project.Project) string) projectview.Options {
	return projectview.Options{
		Path:                   path,
		Activity:               activity,
		HideEmptyDisplayFields: true,
	}
}

func visibilityAction(project project.Project) string {
	if project.Hidden {
		return "unhide"
	}
	return "hide"
}

func pinAction(project project.Project) string {
	if project.Pinned {
		return "unpin"
	}
	return "pin"
}

func projectTitle(project project.Project) string {
	return projectview.Title(project, project.Name)
}

func detailSummaryView(project project.Project, width int) string {
	contentWidth := width - 2
	if contentWidth < 1 {
		contentWidth = width
	}
	lines := []string{titleStyle.Render(truncateText(projectTitle(project), width))}
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
	fields := projectview.Fields(project, detailOptions(shortPath, projectview.DetailActivity))
	for _, field := range fields {
		addSummaryLine(field)
	}
	addRecentFiles(&lines, project.RecentFiles, contentWidth)
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

func addRecentFiles(lines *[]string, files []ovwformat.RecentFile, width int) {
	if len(files) == 0 {
		return
	}
	if len(*lines) > 1 {
		*lines = append(*lines, "")
	}
	*lines = append(*lines, truncateText("Recent files", width))
	for _, file := range files {
		*lines = append(*lines, recentFileLine(file, width))
	}
}

func recentFileLine(file ovwformat.RecentFile, width int) string {
	ageWidth := len([]rune(file.Age))
	gap := 2
	pathWidth := width - ageWidth - gap
	if pathWidth < 8 {
		return truncateText(file.Path, width)
	}
	path := truncateText(file.Path, pathWidth)
	padding := width - len([]rune(path)) - ageWidth
	if padding < 1 {
		padding = 1
	}
	return path + strings.Repeat(" ", padding) + file.Age
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
