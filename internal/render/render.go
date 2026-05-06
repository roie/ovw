package render

import (
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
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	headers := make([]string, 0, len(cfg.Columns))
	for _, column := range cfg.Columns {
		headers = append(headers, strings.ToUpper(column))
	}
	fmt.Fprintln(tw, strings.Join(headers, "\t"))
	for _, project := range projects {
		values := make([]string, 0, len(cfg.Columns))
		for _, column := range cfg.Columns {
			values = append(values, value(project, column))
		}
		fmt.Fprintln(tw, strings.Join(values, "\t"))
	}
	return tw.Flush()
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
