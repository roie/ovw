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

func sortOptions() []sortOption {
	return []sortOption{
		{Label: "activity", Value: "activity"},
		{Label: "name", Value: "name"},
		{Label: "status", Value: "status"},
	}
}

func (m Model) currentSortIndex() int {
	options := sortOptions()
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
