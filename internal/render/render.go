package render

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"ovw/internal/config"
	ovwformat "ovw/internal/format"
	"ovw/internal/project"
)

const fallbackTableWidth = 120

func Table(w io.Writer, projects []project.Project, cfg config.Config, elapsed time.Duration) error {
	return TableWithWidth(w, projects, cfg, elapsed, terminalWidth())
}

func TableWithWidth(w io.Writer, projects []project.Project, cfg config.Config, elapsed time.Duration, width int) error {
	if width <= 0 {
		width = fallbackTableWidth
	}
	fmt.Fprintf(w, "ovw — %d projects · scanned in %.1fs\n\n", len(projects), elapsed.Seconds())
	rows := tableRows(projects, cfg)
	applyWidth(rows, cfg.Columns, width)
	var table bytes.Buffer
	tw := tabwriter.NewWriter(&table, 0, 0, 2, ' ', 0)
	headers := make([]string, 0, len(cfg.Columns))
	for _, column := range cfg.Columns {
		headers = append(headers, headerLabel(column))
	}
	fmt.Fprintln(tw, strings.Join(headers, "\t"))
	for _, row := range rows {
		fmt.Fprintln(tw, strings.Join(row.values, "\t"))
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
	separatorWidth := width
	if separatorWidth > 0 {
		if _, err := fmt.Fprintln(w, strings.Repeat("-", separatorWidth)); err != nil {
			return err
		}
	}
	for _, line := range lines[1:] {
		line = trimLineWidth(line, width)
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
	out := make([]jsonProject, 0, len(projects))
	for _, project := range projects {
		out = append(out, newJSONProject(project))
	}
	return encoder.Encode(out)
}

func ProjectJSON(w io.Writer, project project.Project) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(newJSONProject(project))
}

type tableRow struct {
	values []string
}

type jsonProject struct {
	Name     string       `json:"name"`
	Path     string       `json:"path"`
	Stack    []string     `json:"stack"`
	Tags     []string     `json:"tags"`
	Status   string       `json:"status"`
	Note     string       `json:"note"`
	Activity jsonActivity `json:"activity"`
}

type jsonActivity struct {
	LastCommitAt      *time.Time `json:"last_commit_at,omitempty"`
	LastCommitMessage string     `json:"last_commit_message,omitempty"`
	Branch            string     `json:"branch,omitempty"`
	Dirty             bool       `json:"dirty"`
	Unpushed          int        `json:"unpushed"`
	HasGit            bool       `json:"has_git"`
	HasCommits        bool       `json:"has_commits"`
}

func newJSONProject(project project.Project) jsonProject {
	activity := jsonActivity{
		LastCommitMessage: project.Activity.LastCommitMessage,
		Branch:            project.Activity.Branch,
		Dirty:             project.Activity.Dirty,
		Unpushed:          project.Activity.Unpushed,
		HasGit:            project.Activity.HasGit,
		HasCommits:        project.Activity.HasCommits,
	}
	if !project.Activity.LastCommitAt.IsZero() {
		activity.LastCommitAt = &project.Activity.LastCommitAt
	}
	return jsonProject{
		Name:     project.Name,
		Path:     project.Path,
		Stack:    project.Stack,
		Tags:     project.Tags,
		Status:   project.Status,
		Note:     project.Note.Display,
		Activity: activity,
	}
}

func tableRows(projects []project.Project, cfg config.Config) []tableRow {
	displayNames := disambiguatedNames(projects)
	rows := make([]tableRow, 0, len(projects))
	for _, project := range projects {
		values := make([]string, 0, len(cfg.Columns))
		for _, column := range cfg.Columns {
			values = append(values, value(project, column, displayNames[project.Path]))
		}
		rows = append(rows, tableRow{values: values})
	}
	return rows
}

func value(p project.Project, column, displayName string) string {
	switch column {
	case "name":
		return displayName
	case "stack":
		return p.StackDisplay
	case "activity":
		return p.Activity.Display
	case "status":
		return ovwformat.TagDisplay(p.Tags)
	case "note":
		return p.Note.Display
	default:
		return ""
	}
}

func applyWidth(rows []tableRow, columns []string, width int) {
	noteIndex := -1
	for i, column := range columns {
		if column == "note" {
			noteIndex = i
			break
		}
	}
	if noteIndex < 0 {
		return
	}
	maxNoteWidth := width - nonNoteWidth(rows, columns, noteIndex)
	if maxNoteWidth < 8 {
		maxNoteWidth = 8
	}
	for i := range rows {
		if noteIndex < len(rows[i].values) {
			rows[i].values[noteIndex] = truncate(rows[i].values[noteIndex], maxNoteWidth)
		}
	}
}

func nonNoteWidth(rows []tableRow, columns []string, noteIndex int) int {
	widths := map[int]int{}
	for i, column := range columns {
		widths[i] = len(headerLabel(column))
	}
	for _, row := range rows {
		for i, value := range row.values {
			if i == noteIndex {
				continue
			}
			if len(value) > widths[i] {
				widths[i] = len(value)
			}
		}
	}
	total := 0
	for i, width := range widths {
		if i == noteIndex {
			continue
		}
		total += width
	}
	total += 2 * (len(widths) - 1)
	return total
}

func truncate(value string, maxWidth int) string {
	if len(value) <= maxWidth {
		return value
	}
	if maxWidth <= len("…") {
		return ""
	}
	return value[:maxWidth-len("…")] + "…"
}

func trimLineWidth(line string, width int) string {
	hasNewline := strings.HasSuffix(line, "\n")
	line = strings.TrimRight(line, "\n")
	if len(line) > width {
		line = truncate(line, width)
	}
	if hasNewline {
		return line + "\n"
	}
	return line
}

func terminalWidth() int {
	if columns := os.Getenv("COLUMNS"); columns != "" {
		if width, err := strconv.Atoi(columns); err == nil && width > 0 {
			return width
		}
	}
	cmd := exec.Command("sh", "-c", "stty size 2>/dev/null | awk '{print $2}'")
	cmd.Stdin = os.Stdin
	out, err := cmd.Output()
	if err == nil {
		if width, parseErr := strconv.Atoi(strings.TrimSpace(string(out))); parseErr == nil && width > 0 {
			return width
		}
	}
	return fallbackTableWidth
}

func disambiguatedNames(projects []project.Project) map[string]string {
	counts := map[string]int{}
	for _, project := range projects {
		counts[project.Name]++
	}
	names := map[string]string{}
	for _, project := range projects {
		if counts[project.Name] < 2 {
			names[project.Path] = project.Name
			continue
		}
		parent := filepath.Base(filepath.Dir(project.Path))
		if parent == "." || parent == string(filepath.Separator) || parent == "" {
			names[project.Path] = project.Name
			continue
		}
		names[project.Path] = filepath.ToSlash(filepath.Join(parent, project.Name))
	}
	return names
}
