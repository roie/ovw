package textwrap

import (
	"strings"

	"github.com/charmbracelet/x/ansi"
)

func Lines(value string, width int) []string {
	if width <= 0 {
		return []string{value}
	}
	paragraphs := strings.Split(value, "\n")
	lines := []string{}
	for _, paragraph := range paragraphs {
		lines = append(lines, line(paragraph, width)...)
	}
	return lines
}

func line(value string, width int) []string {
	if ansi.StringWidth(value) <= width {
		return []string{value}
	}
	words := strings.Fields(value)
	if len(words) == 0 {
		return []string{""}
	}
	lines := []string{}
	current := ""
	for _, word := range words {
		if current == "" {
			current = word
			continue
		}
		if ansi.StringWidth(current)+1+ansi.StringWidth(word) <= width {
			current += " " + word
			continue
		}
		lines = append(lines, current)
		current = word
	}
	if current != "" {
		lines = append(lines, current)
	}
	return lines
}
