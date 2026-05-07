package tui

import "strings"

func noteView(value string) string {
	if value == "" {
		return titleStyle.Render("Note") + "\n" + mutedStyle.Render("empty clears manual note")
	}
	return titleStyle.Render("Note") + "\n" + value
}

type statusOptionKind int

const (
	statusOptionValue statusOptionKind = iota
	statusOptionCustom
	statusOptionClear
)

type statusOption struct {
	Label string
	Value string
	Kind  statusOptionKind
}

func (m Model) statusOptions() []statusOption {
	options := make([]statusOption, 0, len(m.config.Statuses)+2)
	for _, status := range m.config.Statuses {
		options = append(options, statusOption{Label: status, Value: status})
	}
	options = append(options,
		statusOption{Label: "custom", Kind: statusOptionCustom},
		statusOption{Label: "clear", Kind: statusOptionClear},
	)
	return options
}

func statusView(options []statusOption, selected int) string {
	lines := []string{titleStyle.Render("Status")}
	for index, option := range options {
		line := option.Label
		if index == selected {
			line = selectedStyle.Render(line)
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}

func statusInputView(value string) string {
	if value == "" {
		return titleStyle.Render("Custom status") + "\n" + mutedStyle.Render("empty clears manual status")
	}
	return titleStyle.Render("Custom status") + "\n" + value
}
