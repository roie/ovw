package projectview

import (
	"strconv"
	"strings"

	"ovw/internal/columns"
	"ovw/internal/format"
	"ovw/internal/project"
)

func ColumnLabel(column string) string {
	return columns.Label(column)
}

func ColumnValue(project project.Project, column, displayName string) string {
	switch column {
	case "name":
		return Title(project, displayName)
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
	case "ports":
		return portsDisplay(project.Ports)
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

func portsDisplay(ports []int) string {
	if len(ports) == 0 {
		return "—"
	}
	values := make([]string, 0, len(ports))
	for _, port := range ports {
		values = append(values, strconv.Itoa(port))
	}
	return strings.Join(values, ", ")
}
