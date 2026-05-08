package tui

import (
	"strings"

	"ovw/internal/config"
	"ovw/internal/project"
	"ovw/internal/projectview"
)

type tableCell struct {
	Value string
	Width int
	Flex  bool
}

type tableRow struct {
	Cells []tableCell
}

func tableView(projects []project.Project, selected, width, height int, cfg config.Config) string {
	if len(projects) == 0 {
		return mutedStyle.Render("No projects found")
	}
	columns := tableColumns(cfg)
	rows := tableRows(projects, columns, width)
	lines := []string{
		tableLine(tableHeader(columns, rows)),
		strings.Repeat("-", tableLineWidth(tableHeader(columns, rows))),
	}
	start, end := visibleRange(len(projects), selected, tableBodyHeight(height))
	for index := start; index < end; index++ {
		line := tableLine(rows[index])
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

func tableRows(projects []project.Project, columns []string, width int) []tableRow {
	displayNames := projectview.DisambiguatedNames(projects)
	rows := make([]tableRow, 0, len(projects))
	for index, project := range projects {
		row := make([]tableCell, 0, len(columns))
		for _, column := range columns {
			row = append(row, tableCell{
				Value: projectview.ColumnValue(project, column, displayNames[projectview.ProjectKey(project, index)]),
				Flex:  tableColumnIsFlex(column),
			})
		}
		rows = append(rows, tableRow{Cells: row})
	}
	fitTableRows(rows, columns, width)
	return rows
}

func tableHeader(columns []string, rows []tableRow) tableRow {
	cells := make([]tableCell, 0, len(columns))
	for index, column := range columns {
		width := len([]rune(projectview.ColumnLabel(column)))
		if len(rows) > 0 && index < len(rows[0].Cells) {
			width = rows[0].Cells[index].Width
		}
		cells = append(cells, tableCell{Value: projectview.ColumnLabel(column), Width: width})
	}
	return tableRow{Cells: cells}
}

func tableColumnIsFlex(column string) bool {
	return column == "note" || column == "path"
}

func fitTableRows(rows []tableRow, columns []string, width int) {
	if width <= 0 {
		width = 100
	}
	widths := make([]int, len(columns))
	flexIndexes := []int{}
	for index, column := range columns {
		widths[index] = len([]rune(projectview.ColumnLabel(column)))
		if tableColumnIsFlex(column) {
			flexIndexes = append(flexIndexes, index)
		}
	}
	for _, row := range rows {
		for index, cell := range row.Cells {
			if cellWidth := len([]rune(cell.Value)); cellWidth > widths[index] {
				widths[index] = cellWidth
			}
		}
	}
	total := tableLineWidthFromWidths(widths)
	if total > width {
		indexes := flexIndexes
		if len(indexes) == 0 {
			indexes = longestColumnIndexes(widths)
		}
		shrinkColumns(widths, indexes, total-width)
		total = tableLineWidthFromWidths(widths)
		if total > width {
			shrinkColumns(widths, longestColumnIndexes(widths), total-width)
		}
	}
	total = tableLineWidthFromWidths(widths)
	if total < width && len(widths) > 0 {
		widths[len(widths)-1] += width - total
	}
	for rowIndex := range rows {
		for cellIndex := range rows[rowIndex].Cells {
			rows[rowIndex].Cells[cellIndex].Width = widths[cellIndex]
		}
	}
}

func longestColumnIndexes(widths []int) []int {
	indexes := make([]int, len(widths))
	used := make([]bool, len(widths))
	for rank := range widths {
		best := -1
		for index, width := range widths {
			if used[index] {
				continue
			}
			if best < 0 || width > widths[best] {
				best = index
			}
		}
		if best < 0 {
			return indexes[:rank]
		}
		used[best] = true
		indexes[rank] = best
	}
	return indexes
}

func shrinkColumns(widths []int, indexes []int, overflow int) {
	for overflow > 0 {
		shrank := false
		for _, index := range indexes {
			if overflow <= 0 {
				return
			}
			minWidth := 8
			if len(widths) > 5 {
				minWidth = 6
			}
			if widths[index] <= minWidth {
				continue
			}
			widths[index]--
			overflow--
			shrank = true
		}
		if !shrank {
			return
		}
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
	runes := []rune(value)
	if len(runes) <= width {
		return value
	}
	if width <= 3 {
		return string(runes[:width])
	}
	return string(runes[:width-3]) + "..."
}
