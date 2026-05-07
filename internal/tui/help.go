package tui

import "strings"

func helpView() string {
	lines := []string{
		titleStyle.Render("Help"),
		"up/down or j/k  move",
		"/               search",
		"f               filter",
		"s               sort",
		"enter           details",
		"n               note",
		"m               status",
		"r               reload",
		"o               open",
		"esc             back",
		"q               quit",
	}
	return strings.Join(lines, "\n")
}
