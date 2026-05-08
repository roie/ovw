package projectview

import (
	"strings"

	"ovw/internal/format"
	"ovw/internal/project"
)

func ColumnLabel(column string) string {
	if column == "" {
		return ""
	}
	return strings.ToUpper(column[:1]) + column[1:]
}

func ColumnValue(project project.Project, column, displayName string) string {
	switch column {
	case "name":
		return displayName
	case "path":
		return project.Path
	case "stack":
		return project.StackDisplay
	case "manager":
		return strings.Join(project.Managers, ", ")
	case "scripts":
		return scriptsDisplay(project.Scripts)
	case "version":
		return project.Version
	case "activity":
		return project.Activity.Display
	case "status":
		return project.Status.Display
	case "note":
		return format.SingleLine(project.Note.Display)
	default:
		return ""
	}
}
