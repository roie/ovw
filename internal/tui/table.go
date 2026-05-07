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

func tableView(projects []project.Project, selected, width int) string {
	if len(projects) == 0 {
		return mutedStyle.Render("No projects found")
	}
	widths := fitTableColumns(width)
	lines := []string{
		tableRow("Name", "Stack", "Activity", "Status", "Note", widths),
		strings.Repeat("-", tableLineWidth(widths)),
	}
	for index, project := range projects {
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

func tableLineWidth(widths tableColumnWidths) int {
	return widths.Name + widths.Stack + widths.Activity + widths.Status + widths.Note + tableGapWidth()
}

func tableGapWidth() int {
	return 8
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
