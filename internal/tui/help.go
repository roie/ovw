package tui

import (
	"strings"

	"ovw/internal/buildinfo"
	"ovw/internal/config"
)

const helpDescription = "A terminal overview for your local projects."
const helpURL = "https://github.com/roie/ovw"

func helpView(actionKeys ...config.ActionKeyConfig) string {
	keys := config.Default().Keys.Actions
	if len(actionKeys) > 0 {
		keys = actionKeys[0]
	}
	entries := []helpEntry{
		{Key: "↑↓ or j/k", Action: "move"},
		{Key: "←→ or h/l", Action: "scroll columns"},
		{Key: keys.Details, Action: "details"},
		{Key: "esc or q", Action: "back / quit"},
		{Key: "ctrl+p", Action: "command"},
		{Key: "/", Action: "search"},
		{Key: "a", Action: "add"},
		{Key: keys.Sidepane, Action: "sidepane"},
		{Key: "f, s", Action: "filter, sort"},
		{Key: "c", Action: "columns"},
		{Key: keys.Editor + ", " + keys.Terminal, Action: "open editor, terminal"},
		{Key: keys.Runner, Action: "runner"},
		{Key: keys.Note + ", " + keys.Status, Action: "note, status"},
		{Key: keys.Pin, Action: "pin"},
		{Key: "ctrl+r or F5", Action: "reload"},
	}
	lines := helpEntryLines(entries)
	return helpModal(lines, 56)
}

type helpEntry struct {
	Key    string
	Action string
}

func helpEntryLines(entries []helpEntry) []string {
	lines := make([]string, 0, len(entries))
	for _, entry := range entries {
		lines = append(lines, "  "+helpActionCell(entry.Key, entry.Action))
	}
	return lines
}

func helpActionCell(key, action string) string {
	keyWidth := 13
	gap := keyWidth - lipglossWidth(key)
	if gap < 2 {
		gap = 2
	}
	return modalHintKey(key) + strings.Repeat(" ", gap) + modalMuted(action)
}

func helpModal(lines []string, width int) string {
	if width < 24 {
		width = 24
	}
	innerWidth := width - 4
	title := modalTitle("ovw") + " " + modalMuted(buildinfo.Version)
	esc := modalMuted("esc")
	titleGap := innerWidth - lipglossWidth(title) - lipglossWidth(esc)
	if titleGap < 1 {
		titleGap = 1
	}
	out := []string{
		modalSurface("┌" + strings.Repeat("─", width-2) + "┐"),
		modalLine(title+strings.Repeat(" ", titleGap)+esc, innerWidth),
		modalLine(modalMuted(helpDescription), innerWidth),
		modalSurface("├" + strings.Repeat("─", width-2) + "┤"),
	}
	for _, line := range lines {
		out = append(out, modalLine(fitModalText(line, innerWidth), innerWidth))
	}
	out = append(out,
		modalLine(helpFooter(innerWidth), innerWidth),
		modalSurface("└"+strings.Repeat("─", width-2)+"┘"),
	)
	return strings.Join(out, "\n")
}

func helpFooter(width int) string {
	right := modalMuted(helpURL)
	gap := width - lipglossWidth(right)
	if gap < 0 {
		gap = 0
	}
	return strings.Repeat(" ", gap) + right
}
