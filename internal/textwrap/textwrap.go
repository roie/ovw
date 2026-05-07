package textwrap

import "strings"

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
	if len([]rune(value)) <= width {
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
		if len([]rune(current))+1+len([]rune(word)) <= width {
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
