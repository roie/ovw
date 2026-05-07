package tui

import (
	"strings"

	"github.com/charmbracelet/x/ansi"
)

func noteView(value string) string {
	return inputModalView("Note", value, "empty clears manual note", 56)
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
	lines := []string{}
	for index, option := range options {
		line := option.Label
		if index == selected {
			line = modalSelected(line)
		}
		lines = append(lines, line)
	}
	lines = append(lines, "", actionHint("enter", "select"))
	return modalView("Status", lines, 42)
}

func statusInputView(value string) string {
	return inputModalView("Custom status", value, "empty clears manual status", 42)
}

func inputModalView(title, value, placeholder string, width int) string {
	inputWidth := width - 4
	lines := inputModalLines(value, placeholder, inputWidth)
	lines = append(lines, "", actionHint("enter", "save"))
	return modalView(title, lines, width)
}

func searchInputLine(value, placeholder string) string {
	if value == "" {
		return placeholder + activeCursor()
	}
	return value + activeCursor()
}

func inputModalLines(value, placeholder string, width int) []string {
	if value == "" {
		return []string{modalMuted(placeholder) + modalCursor()}
	}
	lines := wrapInputLines(value, width)
	last := len(lines) - 1
	lines[last] += modalCursor()
	return lines
}

func actionHint(key, action string) string {
	return modalHintKey(key) + " " + modalMuted(action)
}

func keyActionLine(key, action string, keyWidth int) string {
	gap := keyWidth - lipglossWidth(key)
	if gap < 2 {
		gap = 2
	}
	return modalHintKey(key) + strings.Repeat(" ", gap) + modalMuted(action)
}

func modalView(title string, lines []string, width int) string {
	if width < 24 {
		width = 24
	}
	innerWidth := width - 4
	titleLine := modalTitle(title)
	esc := modalMuted("esc")
	titleGap := innerWidth - lipglossWidth(titleLine) - lipglossWidth(esc)
	if titleGap < 1 {
		titleGap = 1
	}
	out := []string{
		modalSurface("┌" + strings.Repeat("─", width-2) + "┐"),
		modalLine(titleLine+strings.Repeat(" ", titleGap)+esc, innerWidth),
		modalSurface("├" + strings.Repeat("─", width-2) + "┤"),
	}
	for _, line := range lines {
		out = append(out, modalLine(fitModalText(line, innerWidth), innerWidth))
	}
	out = append(out, modalSurface("└"+strings.Repeat("─", width-2)+"┘"))
	return strings.Join(out, "\n")
}

func fitModalText(value string, width int) string {
	if lipglossWidth(value) <= width {
		return value
	}
	return truncateText(value, width)
}

func wrapInputLines(value string, width int) []string {
	if width <= 0 || lipglossWidth(value)+1 <= width {
		return []string{value}
	}
	runes := []rune(value)
	lines := make([]string, 0, len(runes)/width+1)
	for len(runes)+1 > width {
		lines = append(lines, string(runes[:width]))
		runes = runes[width:]
	}
	lines = append(lines, string(runes))
	return lines
}

func modalLine(value string, width int) string {
	return modalSurface("│ " + padRight(value, width) + " │")
}

func modalSurface(value string) string {
	return "\x1b[48;5;" + modalSurfaceColor + "m" + value + ansiReset
}

func modalTitle(value string) string {
	return modalANSI("1;38;5;86", value)
}

func modalMuted(value string) string {
	return modalANSI("38;5;244", value)
}

func modalHintKey(value string) string {
	return modalANSI("38;5;252", value)
}

func modalCursor() string {
	return modalANSI("5;38;5;252", "▌")
}

func activeCursor() string {
	return "\x1b[5m▌\x1b[25m"
}

func modalSelected(value string) string {
	return "\x1b[38;5;229;48;5;57m" + value + "\x1b[39;48;5;" + modalSurfaceColor + "m"
}

func modalANSI(code, value string) string {
	return "\x1b[" + code + ";48;5;" + modalSurfaceColor + "m" + value + "\x1b[25;22;39;48;5;" + modalSurfaceColor + "m"
}

func overlayModal(base, modal string, width int) string {
	baseLines := strings.Split(base, "\n")
	modalLines := strings.Split(modal, "\n")
	if width <= 0 {
		width = maxLineWidth(baseLines)
	}
	if width <= 0 {
		return modal
	}
	modalWidth := maxLineWidth(modalLines)
	left := (width - modalWidth) / 2
	if left < 0 {
		left = 0
	}
	top := (len(baseLines) - len(modalLines)) / 2
	if top < 1 {
		top = 1
	}
	for len(baseLines) < top+len(modalLines) {
		baseLines = append(baseLines, "")
	}
	for index, line := range modalLines {
		baseLines[top+index] = overlayLine(baseLines[top+index], line, left, width)
	}
	return strings.Join(baseLines, "\n")
}

func overlayLine(base, overlay string, left, width int) string {
	overlayWidth := lipglossWidth(overlay)
	if left < 0 {
		left = 0
	}
	right := left + overlayWidth
	prefix := displayPrefix(base, left)
	suffix := displaySuffix(base, right)
	line := padRight(prefix, left) + ansiReset + overlay + ansiReset + suffix
	if lipglossWidth(line) < width {
		line = padRight(line, width)
	}
	return line
}

const ansiReset = "\x1b[0m"

func displayPrefix(value string, width int) string {
	if width <= 0 {
		return ""
	}
	return ansi.Cut(value, 0, width)
}

func displaySuffix(value string, start int) string {
	if start <= 0 {
		return value
	}
	return ansi.Cut(value, start, lipglossWidth(value))
}

func lipglossWidth(value string) int {
	return maxLineWidth([]string{value})
}
