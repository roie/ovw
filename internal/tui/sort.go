package tui

import (
	"ovw/internal/app"
	"ovw/internal/config"
	"ovw/internal/filter"
)

type sortOption struct {
	Label string
	Value string
}

func sortOptions(cfg config.Config) []sortOption {
	columns := cfg.Columns
	if len(columns) == 0 {
		columns = config.Default().Columns
	}
	visible := map[string]bool{}
	for _, column := range columns {
		visible[column] = true
	}
	all := []sortOption{
		{Label: "activity", Value: "activity"},
		{Label: "updated", Value: "updated"},
		{Label: "name", Value: "name"},
		{Label: "status", Value: "status"},
	}
	options := make([]sortOption, 0, len(all))
	for _, option := range all {
		if visible[option.Value] {
			options = append(options, option)
		}
	}
	return options
}

func (m Model) currentSortIndex() int {
	options := sortOptions(m.config)
	for index, option := range options {
		if option.Value == m.activeSort {
			return index
		}
	}
	return 0
}

func (m *Model) applySort(option sortOption) {
	m.request.Sort = filter.FormatSort(option.Value, m.activeSortDir)
	m.activeSort = option.Value
	m.activeSortDir = sortDir(m.activeSortDir)
}

func sortView(options []sortOption, selected int, dir string) string {
	labels := make([]string, 0, len(options))
	dir = sortDir(dir)
	for index, option := range options {
		label := option.Label
		if index == selected {
			label += "   " + dir
		}
		labels = append(labels, label)
	}
	lines := modalOptionLines(labels, selected)
	lines = append(lines, "", actionHint("enter", "apply")+" · "+actionHint("<->", "direction"))
	return modalView("Sort", lines, 42)
}

func sortFromRequest(request app.Options) string {
	spec, err := filter.ParseSort(request.Sort, defaultSortConfig())
	if err != nil {
		return "activity"
	}
	return spec.By
}

func sortDirFromRequest(request app.Options) string {
	spec, err := filter.ParseSort(request.Sort, defaultSortConfig())
	if err != nil {
		return "desc"
	}
	return spec.Dir
}

func sortHeader(by, dir string) string {
	if by == "" {
		by = "activity"
	}
	return by + " " + sortDir(dir)
}

func sortHeaderCompact(by, dir string) string {
	if by == "" {
		by = "activity"
	}
	switch sortDir(dir) {
	case "asc":
		return by + " ↑"
	default:
		return by + " ↓"
	}
}

func (m *Model) toggleSortDir() {
	if sortDir(m.activeSortDir) == "asc" {
		m.activeSortDir = "desc"
		return
	}
	m.activeSortDir = "asc"
}

func (m *Model) syncActiveSort() {
	spec, err := filter.ParseSort(m.request.Sort, m.config)
	if err != nil {
		return
	}
	m.activeSort = spec.By
	m.activeSortDir = spec.Dir
}

func sortDir(dir string) string {
	if dir == "asc" {
		return "asc"
	}
	return "desc"
}

func defaultSortConfig() config.Config {
	cfg := config.Default()
	return cfg
}
