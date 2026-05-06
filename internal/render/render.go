package render

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"
	"time"

	"ovw/internal/config"
	"ovw/internal/project"
)

func Table(w io.Writer, projects []project.Project, cfg config.Config, elapsed time.Duration) error {
	fmt.Fprintf(w, "ovw — %d projects · scanned in %.1fs\n\n", len(projects), elapsed.Seconds())
	var table bytes.Buffer
	tw := tabwriter.NewWriter(&table, 0, 0, 2, ' ', 0)
	headers := make([]string, 0, len(cfg.Columns))
	for _, column := range cfg.Columns {
		headers = append(headers, headerLabel(column))
	}
	fmt.Fprintln(tw, strings.Join(headers, "\t"))
	for _, project := range projects {
		values := make([]string, 0, len(cfg.Columns))
		for _, column := range cfg.Columns {
			values = append(values, value(project, column))
		}
		fmt.Fprintln(tw, strings.Join(values, "\t"))
	}
	if err := tw.Flush(); err != nil {
		return err
	}
	lines := strings.SplitAfter(table.String(), "\n")
	if len(lines) == 0 {
		return nil
	}
	if _, err := io.WriteString(w, lines[0]); err != nil {
		return err
	}
	separatorWidth := len(strings.TrimRight(lines[0], "\n"))
	if separatorWidth > 0 {
		if _, err := fmt.Fprintln(w, strings.Repeat("-", separatorWidth)); err != nil {
			return err
		}
	}
	for _, line := range lines[1:] {
		if _, err := io.WriteString(w, line); err != nil {
			return err
		}
	}
	return nil
}

func headerLabel(column string) string {
	if column == "" {
		return ""
	}
	return strings.ToUpper(column[:1]) + column[1:]
}

func JSON(w io.Writer, projects []project.Project) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(projects)
}

func value(p project.Project, column string) string {
	switch column {
	case "name":
		return p.Name
	case "stack":
		return p.StackDisplay
	case "activity":
		return p.Activity.Display
	case "status":
		return p.Status
	case "note":
		return p.Note.Display
	default:
		return ""
	}
}
