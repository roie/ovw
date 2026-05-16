package tui

import (
	columnmeta "ovw/internal/columns"
	"ovw/internal/config"

	tea "github.com/charmbracelet/bubbletea"
)

func (m *Model) openColumns() {
	m.screen = screenColumns
	m.columnSelected = 0
	m.columnOrder = orderedColumnIDs(m.config.Columns, m.config.ColumnOrder)
	m.columnChecked = checkedColumns(m.config.Columns)
	m.columnErr = ""
}

func orderedColumnIDs(configured, configuredOrder []string) []string {
	seen := map[string]bool{}
	order := []string{}
	for _, column := range configuredOrder {
		if columnmeta.Valid(column) && !seen[column] {
			order = append(order, column)
			seen[column] = true
		}
	}
	for _, column := range configured {
		if columnmeta.Valid(column) && !seen[column] {
			order = append(order, column)
			seen[column] = true
		}
	}
	for _, column := range columnmeta.IDs() {
		if !seen[column] {
			order = append(order, column)
		}
	}
	return order
}

func checkedColumns(configured []string) map[string]bool {
	checked := map[string]bool{}
	for _, column := range configured {
		if columnmeta.Valid(column) {
			checked[column] = true
		}
	}
	if len(checked) == 0 {
		for _, column := range config.Default().Columns {
			checked[column] = true
		}
	}
	return checked
}

func (m *Model) toggleColumn() {
	if len(m.columnOrder) == 0 || m.columnSelected >= len(m.columnOrder) {
		return
	}
	column := m.columnOrder[m.columnSelected]
	if m.columnChecked[column] && m.visibleColumnCount() == 1 {
		m.columnErr = "keep at least one column"
		return
	}
	m.columnChecked[column] = !m.columnChecked[column]
	m.columnErr = ""
}

func (m *Model) moveColumn(delta int) {
	if len(m.columnOrder) == 0 {
		return
	}
	next := m.columnSelected + delta
	if next < 0 || next >= len(m.columnOrder) {
		return
	}
	m.columnOrder[m.columnSelected], m.columnOrder[next] = m.columnOrder[next], m.columnOrder[m.columnSelected]
	m.columnSelected = next
	m.columnErr = ""
}

func (m Model) visibleColumnCount() int {
	count := 0
	for _, column := range m.columnOrder {
		if m.columnChecked[column] {
			count++
		}
	}
	return count
}

func (m Model) selectedColumns() []string {
	columns := []string{}
	for _, column := range m.columnOrder {
		if m.columnChecked[column] {
			columns = append(columns, column)
		}
	}
	return columns
}

func (m Model) saveColumns() tea.Cmd {
	cfg := m.config
	cfg.Columns = m.selectedColumns()
	cfg.ColumnOrder = append([]string(nil), m.columnOrder...)
	path := m.configPaths.Config
	writer := m.configWriter
	if writer == nil {
		writer = config.Write
	}
	return func() tea.Msg {
		if len(cfg.Columns) == 0 {
			return columnsSavedMsg{err: errNoVisibleColumns{}}
		}
		if path == "" {
			paths, err := config.Paths()
			if err != nil {
				return columnsSavedMsg{err: err}
			}
			path = paths.Config
		}
		if err := writer(path, cfg); err != nil {
			return columnsSavedMsg{err: err}
		}
		return columnsSavedMsg{config: cfg}
	}
}

func columnsView(order []string, checked map[string]bool, selected int, errText string) string {
	if len(order) == 0 {
		return modalView("Columns", []string{modalMuted("No columns available")}, 48)
	}
	lines := modalCheckboxLines(order, checked, selected)
	if errText != "" {
		lines = append(lines, "", modalMuted(errText))
	}
	lines = append(lines, "", actionHint("space", "toggle")+" · "+actionHint("←→", "reorder")+" · "+actionHint("enter", "save"))
	return modalView("Columns", lines, 48)
}

type errNoVisibleColumns struct{}

func (errNoVisibleColumns) Error() string {
	return "keep at least one column"
}
