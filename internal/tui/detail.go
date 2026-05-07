package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	ovwformat "ovw/internal/format"
	"ovw/internal/project"
)

func detailView(project project.Project, ok bool) string {
	if !ok {
		return mutedStyle.Render("No project selected")
	}
	lines := []string{
		titleStyle.Render(project.Name),
		detailLine("Path", project.Path),
	}
	if len(project.Stack) > 0 {
		lines = append(lines, detailLine("Stack", strings.Join(project.Stack, ", ")))
	}
	if len(project.Managers) > 0 {
		lines = append(lines, detailLine("Manager", strings.Join(project.Managers, ", ")))
	}
	if project.Activity.Display != "" {
		lines = append(lines, detailLine("Activity", project.Activity.Display))
	}
	if status := ovwformat.TagDisplay(project.Tags); status != "" {
		lines = append(lines, detailLine("Status", status))
	}
	if project.Status != "" {
		lines = append(lines, detailLine("Manual status", project.Status))
	}
	if project.Activity.Branch != "" {
		lines = append(lines, detailLine("Branch", project.Activity.Branch))
	}
	if !project.Activity.LastCommitAt.IsZero() {
		lines = append(lines, detailLine("Updated", project.Activity.LastCommitAt.Format("2006-01-02 15:04")))
	}
	if project.Activity.LastCommitMessage != "" {
		lines = append(lines, detailLine("Last commit", project.Activity.LastCommitMessage))
	}
	if dirty := dirtyDetail(project); dirty != "" {
		lines = append(lines, detailLine("Git", dirty))
	}
	if project.Note.Display != "" {
		lines = append(lines, detailLine("Note", project.Note.Display))
	}
	if project.Note.Manual != "" {
		lines = append(lines, detailLine("Manual note", project.Note.Manual))
	}
	if project.Description != "" {
		lines = append(lines, detailLine("Description", project.Description))
	}
	return strings.Join(lines, "\n")
}

func detailSummaryView(project project.Project, width int) string {
	lines := []string{titleStyle.Render(truncateText(project.Name, width))}
	addSummaryLine := func(label, value string) {
		if value == "" {
			return
		}
		lines = append(lines, truncateText(detailLine(label, value), width))
	}
	addSummaryLine("Path", shortPath(project.Path))
	if len(project.Stack) > 0 {
		addSummaryLine("Stack", strings.Join(project.Stack, ", "))
	}
	addSummaryLine("Activity", project.Activity.Display)
	if status := ovwformat.TagDisplay(project.Tags); status != "" {
		addSummaryLine("Status", status)
	}
	addSummaryLine("Branch", project.Activity.Branch)
	if len(project.Managers) > 0 {
		addSummaryLine("Manager", strings.Join(project.Managers, ", "))
	}
	addSummaryNote(&lines, project.Note.Display, width)
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
	*lines = append(*lines, truncateText(note, width))
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

func dirtyDetail(project project.Project) string {
	parts := []string{}
	if project.Activity.Dirty {
		parts = append(parts, "dirty")
	}
	if project.Activity.Unpushed > 0 {
		parts = append(parts, fmt.Sprintf("%d unpushed", project.Activity.Unpushed))
	}
	return strings.Join(parts, " · ")
}
