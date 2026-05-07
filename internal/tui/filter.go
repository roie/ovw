package tui

import "ovw/internal/app"

type filterOption struct {
	Label    string
	Status   string
	Dirty    bool
	Stale    bool
	Untagged bool
	Hidden   bool
}

func (m Model) filterOptions() []filterOption {
	options := []filterOption{
		{Label: "all"},
		{Label: "dirty", Dirty: true},
		{Label: "stale", Stale: true},
		{Label: "untagged", Untagged: true},
		{Label: "hidden", Hidden: true},
	}
	for _, status := range m.config.Statuses {
		options = append(options, filterOption{Label: status, Status: status})
	}
	return options
}

func (m Model) currentFilterIndex() int {
	options := m.filterOptions()
	for index, option := range options {
		if option.Label == m.activeFilter {
			return index
		}
	}
	return 0
}

func (m *Model) applyFilter(option filterOption) {
	m.request.Status = option.Status
	m.request.Dirty = option.Dirty
	m.request.Stale = option.Stale
	m.request.Untagged = option.Untagged
	m.request.Hidden = option.Hidden
	m.activeFilter = option.Label
}

func filterView(options []filterOption, selected int) string {
	if len(options) == 0 {
		return modalView("Filter", []string{modalMuted("No filters available")}, 42)
	}
	lines := []string{}
	for index, option := range options {
		line := option.Label
		if index == selected {
			line = modalSelected(line)
		}
		lines = append(lines, line)
	}
	lines = append(lines, "", actionHint("enter", "apply"))
	return modalView("Filter", lines, 42)
}

func optionsFromRequest(request app.Options) string {
	switch {
	case request.Dirty:
		return "dirty"
	case request.Stale:
		return "stale"
	case request.Untagged:
		return "untagged"
	case request.Hidden:
		return "hidden"
	case request.Status != "":
		return request.Status
	default:
		return "all"
	}
}
