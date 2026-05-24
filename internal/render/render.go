package render

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/charmbracelet/x/ansi"

	"ovw/internal/config"
	ovwformat "ovw/internal/format"
	"ovw/internal/project"
	"ovw/internal/projectview"
)

const fallbackTableWidth = 120

func Table(w io.Writer, projects []project.Project, cfg config.Config, elapsed time.Duration) error {
	return TableWithWidth(w, projects, cfg, elapsed, terminalWidth())
}

func TableWithWidth(w io.Writer, projects []project.Project, cfg config.Config, elapsed time.Duration, width int) error {
	if width <= 0 {
		width = fallbackTableWidth
	}
	fmt.Fprintf(w, "ovw — %d projects · scanned in %s\n\n", len(projects), ovwformat.Elapsed(elapsed))
	rows := tableRows(projects, cfg)
	headers := make([]string, 0, len(cfg.Columns))
	for _, column := range cfg.Columns {
		headers = append(headers, config.FieldLabel(cfg, column))
	}
	widths := applyWidth(headers, rows, cfg, width)
	rows = wrapNoteRows(rows, cfg, widths)
	var table bytes.Buffer
	tw := tabwriter.NewWriter(&table, 0, 0, 2, ' ', 0)
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
	if _, err := io.WriteString(w, trimLineWidth(lines[0], width)); err != nil {
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
	Name        string            `json:"name"`
	Path        string            `json:"path"`
	Stack       []string          `json:"stack"`
	Managers    []string          `json:"managers"`
	Scripts     []string          `json:"scripts"`
	Version     string            `json:"version,omitempty"`
	Ports       []int             `json:"ports"`
	Activity    jsonActivity      `json:"activity"`
	Tags        []string          `json:"tags"`
	Status      string            `json:"status"`
	Description string            `json:"description,omitempty"`
	Note        string            `json:"note"`
	Fields      map[string]string `json:"fields,omitempty"`
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
		Name:        project.Name,
		Path:        project.Path,
		Stack:       project.Stack,
		Managers:    project.Managers,
		Scripts:     project.Scripts,
		Version:     project.Version,
		Ports:       project.Ports,
		Activity:    activity,
		Tags:        project.Status.Tags,
		Status:      project.Status.Value,
		Description: project.Description,
		Note:        project.Note.Display,
		Fields:      project.Fields,
	}
}

func tableRows(projects []project.Project, cfg config.Config) []tableRow {
	displayNames := projectview.DisambiguatedNames(projects)
	rows := make([]tableRow, 0, len(projects))
	for index, project := range projects {
		values := make([]string, 0, len(cfg.Columns))
		for _, column := range cfg.Columns {
			values = append(values, projectview.ColumnValue(project, column, displayNames[projectview.ProjectKey(project, index)]))
		}
		rows = append(rows, tableRow{values: values})
	}
	return rows
}

func applyWidth(headers []string, rows []tableRow, cfg config.Config, width int) []int {
	if width <= 0 || len(headers) == 0 {
		return nil
	}
	widths := naturalColumnWidths(headers, rows)
	available := width - 2*(len(widths)-1)
	if available < 0 {
		available = 0
	}
	widths = fitColumnWidths(widths, minimumColumnWidths(headers, cfg), available, noteColumnIndex(cfg))
	for i := range headers {
		headers[i] = truncate(headers[i], widths[i])
	}
	noteIndex := noteColumnIndex(cfg)
	for i := range rows {
		for column := range rows[i].values {
			if column < len(widths) {
				if column == noteIndex {
					continue
				}
				rows[i].values[column] = truncate(rows[i].values[column], widths[column])
			}
		}
	}
	return widths
}

func wrapNoteRows(rows []tableRow, cfg config.Config, widths []int) []tableRow {
	noteIndex := noteColumnIndex(cfg)
	if noteIndex < 0 || noteIndex >= len(widths) || widths[noteIndex] <= 0 {
		return rows
	}
	out := make([]tableRow, 0, len(rows))
	for _, row := range rows {
		if noteIndex >= len(row.values) {
			out = append(out, row)
			continue
		}
		parts := wrapCell(row.values[noteIndex], widths[noteIndex])
		if len(parts) == 0 {
			out = append(out, row)
			continue
		}
		row.values[noteIndex] = parts[0]
		out = append(out, row)
		for _, part := range parts[1:] {
			continuation := tableRow{values: append([]string(nil), row.values...)}
			continuation.values[noteIndex] = part
			out = append(out, continuation)
		}
	}
	return out
}

func wrapCell(value string, width int) []string {
	if value == "" || width <= 0 {
		return nil
	}
	if strings.Contains(value, "\x1b") {
		return wrapCellByWidth(value, width)
	}
	parts := []string{}
	words := strings.Fields(value)
	if len(words) == 0 {
		return nil
	}
	line := ""
	for _, word := range words {
		if line == "" {
			for ansi.StringWidth(word) > width {
				part := ansi.Cut(word, 0, width)
				if part == "" {
					break
				}
				parts = append(parts, part)
				word = ansi.Cut(word, ansi.StringWidth(part), ansi.StringWidth(word))
			}
			line = word
			continue
		}
		next := line + " " + word
		if ansi.StringWidth(next) <= width {
			line = next
			continue
		}
		parts = append(parts, line)
		line = ""
		for ansi.StringWidth(word) > width {
			part := ansi.Cut(word, 0, width)
			if part == "" {
				break
			}
			parts = append(parts, part)
			word = ansi.Cut(word, ansi.StringWidth(part), ansi.StringWidth(word))
		}
		line = word
	}
	if line != "" {
		parts = append(parts, line)
	}
	return parts
}

func wrapCellByWidth(value string, width int) []string {
	parts := []string{}
	for ansi.StringWidth(value) > width {
		part := ansi.Cut(value, 0, width)
		if part == "" {
			break
		}
		parts = append(parts, part)
		value = trimLeadingSpacesANSI(ansi.Cut(value, ansi.StringWidth(part), ansi.StringWidth(value)))
	}
	if value != "" {
		parts = append(parts, value)
	}
	return parts
}

func trimLeadingSpacesANSI(value string) string {
	prefix := ""
	for strings.HasPrefix(value, "\x1b[") {
		end := strings.IndexByte(value, 'm')
		if end < 0 {
			break
		}
		prefix += value[:end+1]
		value = value[end+1:]
	}
	return prefix + strings.TrimLeft(value, " ")
}

func naturalColumnWidths(headers []string, rows []tableRow) []int {
	widths := make([]int, len(headers))
	for i, header := range headers {
		widths[i] = ansi.StringWidth(header)
	}
	for _, row := range rows {
		for i, value := range row.values {
			if valueWidth := ansi.StringWidth(value); i < len(widths) && valueWidth > widths[i] {
				widths[i] = valueWidth
			}
		}
	}
	return widths
}

func minimumColumnWidths(headers []string, cfg config.Config) []int {
	minimums := make([]int, len(headers))
	for i, header := range headers {
		minimums[i] = ansi.StringWidth(header)
		if minimums[i] > 4 {
			minimums[i] = 4
		}
		if i < len(cfg.Columns) && cfg.Columns[i] == "note" && minimums[i] < 8 {
			minimums[i] = 8
		}
	}
	return minimums
}

func noteColumnIndex(cfg config.Config) int {
	for i, column := range cfg.Columns {
		if column == "note" {
			return i
		}
	}
	return -1
}

func fitColumnWidths(widths []int, minimums []int, available int, noteIndex int) []int {
	fitted := append([]int(nil), widths...)
	mins := append([]int(nil), minimums...)
	for sumWidths(mins) > available {
		shrunk := false
		for i := len(mins) - 1; i >= 0 && sumWidths(mins) > available; i-- {
			if mins[i] > 0 {
				mins[i]--
				shrunk = true
			}
		}
		if !shrunk {
			break
		}
	}
	shrinkColumnWidths(fitted, mins, available, noteIndex)
	return fitted
}

func shrinkColumnWidths(widths []int, minimums []int, available int, noteIndex int) {
	if noteIndex >= 0 && noteIndex < len(widths) {
		for sumWidths(widths) > available && widths[noteIndex] > minimums[noteIndex] {
			widths[noteIndex]--
		}
	}
	for sumWidths(widths) > available {
		index := widestShrinkableColumn(widths, minimums)
		if index < 0 {
			return
		}
		widths[index]--
	}
}

func widestShrinkableColumn(widths []int, minimums []int) int {
	index := -1
	for i, width := range widths {
		if width <= minimums[i] {
			continue
		}
		if index < 0 || width > widths[index] {
			index = i
		}
	}
	return index
}

func sumWidths(widths []int) int {
	total := 0
	for _, width := range widths {
		total += width
	}
	return total
}

func truncate(value string, maxWidth int) string {
	if ansi.StringWidth(value) <= maxWidth {
		return value
	}
	ellipsis := "…"
	ellipsisWidth := ansi.StringWidth(ellipsis)
	if maxWidth <= ellipsisWidth {
		return ansi.Cut(value, 0, maxWidth)
	}
	return ansi.Cut(value, 0, maxWidth-ellipsisWidth) + ellipsis
}

func trimLineWidth(line string, width int) string {
	hasNewline := strings.HasSuffix(line, "\n")
	line = strings.TrimRight(line, "\n")
	if ansi.StringWidth(line) > width {
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
