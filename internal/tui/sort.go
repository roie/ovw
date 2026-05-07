package tui

import "ovw/internal/app"

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
	m.request.Sort = option.Value
	m.activeSort = option.Value
}

func sortView(options []sortOption, selected int) string {
	labels := make([]string, 0, len(options))
	for _, option := range options {
		labels = append(labels, option.Label)
	}
	lines := modalOptionLines(labels, selected)
	lines = append(lines, "", actionHint("enter", "apply"))
	return modalView("Sort", lines, 42)
}

func sortFromRequest(request app.Options) string {
	if request.Sort != "" {
		return request.Sort
	}
	return "activity"
}
