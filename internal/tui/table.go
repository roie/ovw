package tui

import (
	"strings"

	"ovw/internal/config"
	"ovw/internal/project"
	"ovw/internal/projectview"

	"github.com/charmbracelet/x/ansi"
)

type tableCell struct {
	Value string
	Width int
}

type tableRow struct {
	Cells []tableCell
}

const maxTableNoteWidth = 48

func tableView(projects []project.Project, selected, width, height, xOffset int, cfg config.Config, sortBy, sortDir string) string {
	if len(projects) == 0 {
		lines := []string{
			mutedStyle.Render("No projects found"),
			"",
			inlineActionHint("a", "add project"),
			inlineActionHint("ctrl+r", "reload"),
		}
		return strings.Join(lines, "\n")
	}
	columns := tableColumns(cfg)
	rows := tableRows(projects, columns, cfg, sortBy, sortDir)
	expandTableRows(rows, width)
	header := tableHeader(columns, rows, cfg, sortBy, sortDir)
	xOffset = clampTableXOffset(xOffset, width, tableLineWidth(header))
	lines := []string{
		tableViewportLine(tableLine(header), xOffset, width),
		tableViewportLine(strings.Repeat("-", tableLineWidth(header)), xOffset, width),
	}
	start, end := visibleRange(len(projects), selected, tableBodyHeight(height))
	for index := start; index < end; index++ {
		line := tableViewportLine(tableLine(rows[index]), xOffset, width)
		if index == selected {
			line = selectedStyle.Render(line)
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}

func tableColumns(cfg config.Config) []string {
	if len(cfg.Columns) == 0 {
		return config.Default().Columns
	}
	return cfg.Columns
}

func tableRows(projects []project.Project, columns []string, cfg config.Config, sortBy, sortDir string) []tableRow {
	displayNames := projectview.DisambiguatedNames(projects)
	rows := make([]tableRow, 0, len(projects))
	for index, project := range projects {
		row := make([]tableCell, 0, len(columns))
		for _, column := range columns {
			row = append(row, tableCell{
				Value: projectview.ColumnValue(project, column, displayNames[projectview.ProjectKey(project, index)]),
			})
		}
		rows = append(rows, tableRow{Cells: row})
	}
	fitTableRows(rows, columns, cfg, sortBy, sortDir)
	return rows
}

func tableHeader(columns []string, rows []tableRow, cfg config.Config, sortBy, sortDir string) tableRow {
	cells := make([]tableCell, 0, len(columns))
	for index, column := range columns {
		label := tableColumnLabel(column, cfg, sortBy, sortDir)
		width := len([]rune(label))
		if len(rows) > 0 && index < len(rows[0].Cells) {
			width = rows[0].Cells[index].Width
		}
		cells = append(cells, tableCell{Value: label, Width: width})
	}
	return tableRow{Cells: cells}
}

func fitTableRows(rows []tableRow, columns []string, cfg config.Config, sortBy, sortDir string) {
	widths := make([]int, len(columns))
	for index, column := range columns {
		widths[index] = len([]rune(tableColumnLabel(column, cfg, sortBy, sortDir)))
	}
	for _, row := range rows {
		for index, cell := range row.Cells {
			if cellWidth := len([]rune(cell.Value)); cellWidth > widths[index] {
				widths[index] = cellWidth
			}
		}
	}
	for index, column := range columns {
		if column == "note" && widths[index] > maxTableNoteWidth {
			widths[index] = maxTableNoteWidth
		}
	}
	for rowIndex := range rows {
		for cellIndex := range rows[rowIndex].Cells {
			rows[rowIndex].Cells[cellIndex].Width = widths[cellIndex]
		}
	}
}

func expandTableRows(rows []tableRow, viewportWidth int) {
	if viewportWidth <= 0 || len(rows) == 0 || len(rows[0].Cells) == 0 {
		return
	}
	currentWidth := tableLineWidth(rows[0])
	if currentWidth >= viewportWidth {
		return
	}
	lastIndex := len(rows[0].Cells) - 1
	extra := viewportWidth - currentWidth
	for rowIndex := range rows {
		if lastIndex < len(rows[rowIndex].Cells) {
			rows[rowIndex].Cells[lastIndex].Width += extra
		}
	}
}

func tableColumnLabel(column string, cfg config.Config, sortBy, dir string) string {
	label := config.FieldLabel(cfg, column)
	if column != sortBy {
		return label
	}
	switch sortDir(dir) {
	case "asc":
		return label + " ↑"
	default:
		return label + " ↓"
	}
}

func tableLine(row tableRow) string {
	parts := make([]string, 0, len(row.Cells))
	for _, cell := range row.Cells {
		parts = append(parts, tablePadRight(truncateText(cell.Value, cell.Width), cell.Width))
	}
	return strings.Join(parts, "  ")
}

func tableLineWidth(row tableRow) int {
	widths := make([]int, 0, len(row.Cells))
	for _, cell := range row.Cells {
		widths = append(widths, cell.Width)
	}
	return tableLineWidthFromWidths(widths)
}

func tableLineWidthFromWidths(widths []int) int {
	total := 0
	for _, width := range widths {
		total += width
	}
	if len(widths) > 1 {
		total += 2 * (len(widths) - 1)
	}
	return total
}

func tablePadRight(value string, width int) string {
	padding := width - len([]rune(value))
	if padding <= 0 {
		return value
	}
	return value + strings.Repeat(" ", padding)
}

func tableViewportLine(value string, offset, width int) string {
	if width <= 0 {
		return value
	}
	runes := []rune(value)
	if offset < 0 {
		offset = 0
	}
	if offset > len(runes) {
		offset = len(runes)
	}
	end := offset + width
	if end > len(runes) {
		end = len(runes)
	}
	out := string(runes[offset:end])
	return tablePadRight(out, width)
}

func clampTableXOffset(offset, viewportWidth, contentWidth int) int {
	if offset < 0 || viewportWidth <= 0 || contentWidth <= viewportWidth {
		return 0
	}
	maxOffset := contentWidth - viewportWidth
	if offset > maxOffset {
		return maxOffset
	}
	return offset
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

func truncateText(value string, width int) string {
	if width <= 0 {
		return ""
	}
	if lipglossWidth(value) <= width {
		return value
	}
	if width <= 3 {
		return ansi.Truncate(value, width, "")
	}
	return ansi.Truncate(value, width, "...")
}
