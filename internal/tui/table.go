package tui

import (
	"fmt"
	"strings"

	ovwformat "ovw/internal/format"
	"ovw/internal/project"
)

type tableColumnWidths struct {
	Name     int
	Stack    int
	Activity int
	Status   int
	Note     int
}

type compactTableColumnWidths struct {
	Name     int
	Stack    int
	Activity int
	Status   int
}

func tableView(projects []project.Project, selected, width, height int) string {
	if len(projects) == 0 {
		return mutedStyle.Render("No projects found")
	}
	widths := fitTableColumns(width)
	noteHeader := "Note"
	if selected >= 0 && selected < len(projects) {
		noteHeader = fmt.Sprintf("Note %d/%d", selected+1, len(projects))
	}
	lines := []string{
		tableRow("Name", "Stack", "Activity", "Status", noteHeader, widths),
		strings.Repeat("-", tableLineWidth(widths)),
	}
	start, end := visibleRange(len(projects), selected, tableBodyHeight(height))
	for index := start; index < end; index++ {
		project := projects[index]
		line := tableRow(
			project.Name,
			project.StackDisplay,
			project.Activity.Display,
			ovwformat.TagDisplay(project.Tags),
			project.Note.Display,
			widths,
		)
		if index == selected {
			line = selectedStyle.Render(line)
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}

func compactTableView(projects []project.Project, selected, width, height int) string {
	if len(projects) == 0 {
		return mutedStyle.Render("No projects found")
	}
	widths := fitCompactTableColumns(width)
	lines := []string{
		compactTableRow("Name", "Stack", "Activity", "Status", widths),
		strings.Repeat("-", compactTableLineWidth(widths)),
	}
	start, end := visibleRange(len(projects), selected, tableBodyHeight(height))
	for index := start; index < end; index++ {
		project := projects[index]
		line := compactTableRow(
			project.Name,
			project.StackDisplay,
			project.Activity.Display,
			ovwformat.TagDisplay(project.Tags),
			widths,
		)
		if index == selected {
			line = selectedStyle.Render(line)
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}

func visibleRange(total, selected, rows int) (int, int) {
	if total <= 0 {
		return 0, 0
	}
	if rows <= 0 || rows > total {
		return 0, total
	}
	if selected < 0 {
		return 0, rows
	}
	if selected >= total {
		selected = total - 1
	}
	start := selected - rows/2
	if start < 0 {
		start = 0
	}
	if start+rows > total {
		start = total - rows
	}
	return start, start + rows
}

func tableBodyHeight(height int) int {
	if height <= 0 {
		return 0
	}
	rows := height - 2
	if rows < 1 {
		return 1
	}
	return rows
}

func tableRow(name, stack, activity, status, note string, widths tableColumnWidths) string {
	return fmt.Sprintf(
		"%-*s  %-*s  %-*s  %-*s  %s",
		widths.Name, truncateText(name, widths.Name),
		widths.Stack, truncateText(stack, widths.Stack),
		widths.Activity, truncateText(activity, widths.Activity),
		widths.Status, truncateText(status, widths.Status),
		truncateText(note, widths.Note),
	)
}

func compactTableRow(name, stack, activity, status string, widths compactTableColumnWidths) string {
	return fmt.Sprintf(
		"%-*s  %-*s  %-*s  %s",
		widths.Name, truncateText(name, widths.Name),
		widths.Stack, truncateText(stack, widths.Stack),
		widths.Activity, truncateText(activity, widths.Activity),
		truncateText(status, widths.Status),
	)
}

func fitTableColumns(width int) tableColumnWidths {
	if width <= 0 {
		width = 100
	}
	widths := tableColumnWidths{
		Name:     20,
		Stack:    16,
		Activity: 10,
		Status:   18,
	}
	widths.Note = width - widths.Name - widths.Stack - widths.Activity - widths.Status - tableGapWidth()
	if widths.Note >= 12 {
		return widths
	}
	widths.Name = 14
	widths.Stack = 12
	widths.Activity = 8
	widths.Status = 14
	widths.Note = width - widths.Name - widths.Stack - widths.Activity - widths.Status - tableGapWidth()
	if widths.Note < 8 {
		widths.Note = 8
	}
	return widths
}

func fitCompactTableColumns(width int) compactTableColumnWidths {
	if width <= 0 {
		width = 72
	}
	widths := compactTableColumnWidths{
		Name:     20,
		Stack:    18,
		Activity: 8,
	}
	widths.Status = width - widths.Name - widths.Stack - widths.Activity - compactTableGapWidth()
	if widths.Status > 24 {
		widths.Status = 24
	}
	if widths.Status >= 12 {
		return widths
	}
	widths.Name = 16
	widths.Stack = 14
	widths.Activity = 7
	widths.Status = width - widths.Name - widths.Stack - widths.Activity - compactTableGapWidth()
	if widths.Status > 18 {
		widths.Status = 18
	}
	if widths.Status < 8 {
		widths.Status = 8
	}
	return widths
}

func tableLineWidth(widths tableColumnWidths) int {
	return widths.Name + widths.Stack + widths.Activity + widths.Status + widths.Note + tableGapWidth()
}

func compactTableLineWidth(widths compactTableColumnWidths) int {
	return widths.Name + widths.Stack + widths.Activity + widths.Status + compactTableGapWidth()
}

func tableGapWidth() int {
	return 8
}

func compactTableGapWidth() int {
	return 6
}

func truncateText(value string, width int) string {
	if width <= 0 {
		return ""
	}
	runes := []rune(value)
	if len(runes) <= width {
		return value
	}
	if width <= 3 {
		return string(runes[:width])
	}
	return string(runes[:width-3]) + "..."
}
