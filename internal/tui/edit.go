package tui

import (
	"strconv"
	"strings"

	"github.com/charmbracelet/x/ansi"
)

func noteView(projectName, value, placeholder string, cursor int) string {
	if placeholder == "" {
		placeholder = "empty clears note"
	}
	return inputModalView(modalTitleWithProject("Note", projectName), value, placeholder, 56, cursor)
}

func addProjectView(value string, cursor int, err string) string {
	inputWidth := 52
	lines := inputModalLines(value, "~/dev/my-project", inputWidth, cursor)
	if err != "" {
		lines = append(lines, "", errorStyle.Render(err))
	}
	lines = append(lines, "", actionHint("enter", "save"))
	return modalView("Add project", lines, 56)
}

func onboardingView(options []string, selected int, err string) string {
	if len(options) == 0 {
		options = []string{"~/Projects"}
	}
	checked := checkedOnboardingOptions(options)
	return onboardingCheckedView(options, checked, selected, err)
}

func onboardingCheckedView(options []string, checked map[string]bool, selected int, err string) string {
	if len(options) == 0 {
		options = []string{"~/Projects"}
	}
	lines := onboardingHeaderLines("Select project folders to scan")
	lines = append(lines, inlineCheckboxLines(options, checked, selected)...)
	lines = append(lines, onboardingCustomPathLine(selected == len(options)))
	if err != "" {
		lines = append(lines, "", errorStyle.Render(err))
	}
	lines = append(lines, "", inlineActionHint("space", "toggle")+" · "+inlineActionHint("enter", "continue")+" · "+inlineActionHint("q", "quit"))
	return strings.Join(lines, "\n")
}

type setupRow struct {
	Path       string
	Label      string
	Parent     string
	Depth      int
	Checked    bool
	Partial    bool
	Expandable bool
	Expanded   bool
	Count      *int
}

func onboardingSetupView(rows []setupRow, selected int, err string) string {
	lines := onboardingHeaderLines("Select project folders to scan")
	lines = append(lines, inlineSetupRowLines(rows, selected)...)
	lines = append(lines, onboardingCustomPathLine(selected == len(rows)))
	if err != "" {
		lines = append(lines, "", errorStyle.Render(err))
	}
	lines = append(lines, "", inlineActionHint("space", "toggle")+" · "+inlineActionHint("←→", "expand/collapse")+" · "+inlineActionHint("enter", "continue")+" · "+inlineActionHint("q", "quit"))
	return strings.Join(lines, "\n")
}

func onboardingInputView(value string, cursor int, err string) string {
	lines := onboardingHeaderLines("Enter a custom project folder")
	lines = append(lines, inlineInputLines(value, "~/Projects", 52, cursor)...)
	if err != "" {
		lines = append(lines, "", errorStyle.Render(err))
	}
	lines = append(lines, "", inlineActionHint("enter", "continue")+" · "+inlineActionHint("esc", "back"))
	return strings.Join(lines, "\n")
}

func onboardingHeaderLines(prompt string) []string {
	return []string{
		titleStyle.Render("ovw"),
		mutedStyle.Render("A terminal overview for your local projects."),
		"",
		mutedStyle.Render(prompt),
		"",
	}
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

func (m Model) currentStatusIndex(value string) int {
	options := m.statusOptions()
	customIndex := 0
	for index, option := range options {
		if option.Kind == statusOptionCustom {
			customIndex = index
		}
		if value != "" && option.Value == value {
			return index
		}
	}
	if value == "" {
		return 0
	}
	return customIndex
}

func statusView(projectName string, options []statusOption, selected int) string {
	labels := make([]string, 0, len(options))
	for _, option := range options {
		labels = append(labels, option.Label)
	}
	lines := modalOptionLines(labels, selected)
	lines = append(lines, "", actionHint("enter", "select"))
	return modalView(modalTitleWithProject("Status", projectName), lines, 42)
}

func statusInputView(projectName, value string, cursor int) string {
	return inputModalView(modalTitleWithProject("Custom status", projectName), value, "empty clears status", 42, cursor)
}

func modalTitleWithProject(title, projectName string) string {
	if projectName == "" {
		return title
	}
	return title + " · " + projectName
}

func inputModalView(title, value, placeholder string, width int, cursor int) string {
	inputWidth := width - 4
	lines := inputModalLines(value, placeholder, inputWidth, cursor)
	lines = append(lines, "", actionHint("enter", "save"))
	return modalView(title, lines, width)
}

func searchInputLine(value, placeholder string, cursor int) string {
	if value == "" {
		return placeholder + activeCursor()
	}
	return stringWithCursor(value, cursor, activeCursor())
}

func inputModalLines(value, placeholder string, width int, cursor int) []string {
	if value == "" {
		lines := wrapInputPlaceholderLines(placeholder, width)
		for index := range lines {
			lines[index] = modalMuted(lines[index])
		}
		last := len(lines) - 1
		lines[last] += modalCursor()
		return lines
	}
	if textCursor(value, cursor) == len([]rune(value)) {
		lines := wrapInputLines(value, width)
		last := len(lines) - 1
		lines[last] += modalCursor()
		return lines
	}
	return wrapInputLinesWithCursor(value, width, cursor, modalCursor())
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
	paragraphs := strings.Split(value, "\n")
	lines := []string{}
	for _, paragraph := range paragraphs {
		lines = append(lines, wrapInputLine(paragraph, width)...)
	}
	return lines
}

func wrapInputPlaceholderLines(value string, width int) []string {
	if width <= 0 {
		return []string{value}
	}
	paragraphs := strings.Split(value, "\n")
	lines := []string{}
	for _, paragraph := range paragraphs {
		lines = append(lines, wrapInputPlaceholderLine(paragraph, width)...)
	}
	return lines
}

func wrapInputPlaceholderLine(value string, width int) []string {
	words := strings.Fields(value)
	if len(words) == 0 {
		return []string{""}
	}
	lines := []string{}
	line := ""
	for _, word := range words {
		if lipglossWidth(word)+1 > width {
			if line != "" {
				lines = append(lines, line)
				line = ""
			}
			lines = append(lines, wrapInputLine(word, width)...)
			continue
		}
		if line == "" {
			line = word
			continue
		}
		if lipglossWidth(line)+1+lipglossWidth(word)+1 <= width {
			line += " " + word
			continue
		}
		lines = append(lines, line)
		line = word
	}
	if line != "" {
		lines = append(lines, line)
	}
	return lines
}

func wrapInputLine(value string, width int) []string {
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

func wrapInputLinesWithCursor(value string, width int, cursor int, cursorText string) []string {
	runes := []rune(value)
	cursor = textCursor(value, cursor)
	if width <= 1 {
		return []string{string(runes[:cursor]) + cursorText + string(runes[cursor:])}
	}
	lineWidth := width - 1
	lines := []string{}
	for start := 0; start <= len(runes); start += lineWidth {
		end := start + lineWidth
		if end > len(runes) {
			end = len(runes)
		}
		line := string(runes[start:end])
		if cursor >= start && (cursor < end || end == len(runes)) {
			offset := cursor - start
			lineRunes := []rune(line)
			line = string(lineRunes[:offset]) + cursorText + string(lineRunes[offset:])
		}
		lines = append(lines, line)
		if end == len(runes) {
			break
		}
	}
	return lines
}

func stringWithCursor(value string, cursor int, cursorText string) string {
	runes := []rune(value)
	cursor = textCursor(value, cursor)
	return string(runes[:cursor]) + cursorText + string(runes[cursor:])
}

func modalLine(value string, width int) string {
	return modalSurface("│ " + padRight(value, width) + " │")
}

func modalSurface(value string) string {
	return "\x1b[48;5;" + modalSurfaceColor + "m" + value + ansiReset
}

func modalTitle(value string) string {
	return modalANSI("1;38;5;"+accentColor, value)
}

func modalMuted(value string) string {
	return modalANSI("38;5;"+mutedColor, value)
}

func modalHintKey(value string) string {
	return modalANSI("38;5;252", value)
}

func modalAccent(value string) string {
	return modalANSI("38;5;"+accentColor, value)
}

func modalCursor() string {
	return modalANSI("5;38;5;252", "▌")
}

func activeCursor() string {
	return "\x1b[5m▌\x1b[25m"
}

func modalOptionLines(labels []string, selected int) []string {
	lines := make([]string, 0, len(labels))
	for index, label := range labels {
		if index == selected {
			lines = append(lines, modalAccent("> "+label))
			continue
		}
		lines = append(lines, "  "+modalMuted(label))
	}
	return lines
}

func modalCheckboxLines(labels []string, checked map[string]bool, selected int) []string {
	lines := make([]string, 0, len(labels))
	for index, label := range labels {
		box := "[ ]"
		if checked[label] {
			box = "[x]"
		}
		line := box + " " + label
		if index == selected {
			lines = append(lines, modalAccent("> "+line))
			continue
		}
		lines = append(lines, "  "+modalMuted(line))
	}
	return lines
}

func inlineCheckboxLines(labels []string, checked map[string]bool, selected int) []string {
	lines := make([]string, 0, len(labels))
	for index, label := range labels {
		box := "[ ]"
		if checked[label] {
			box = "[x]"
		}
		line := box + " " + label
		if index == selected {
			lines = append(lines, titleStyle.Render("> "+line))
			continue
		}
		lines = append(lines, "  "+mutedStyle.Render(line))
	}
	return lines
}

func inlineSetupRowLines(rows []setupRow, selected int) []string {
	lines := make([]string, 0, len(rows))
	rowTexts := make([]string, len(rows))
	maxWidth := 0
	maxCountWidth := 0
	for index, row := range rows {
		box := "[ ]"
		if row.Checked {
			box = "[x]"
		} else if row.Partial {
			box = "[-]"
		}
		prefix := "  "
		if row.Expandable {
			if row.Expanded {
				prefix = "▾ "
			} else {
				prefix = "▸ "
			}
		}
		indent := strings.Repeat("  ", row.Depth)
		line := indent + prefix + box + " " + row.Label
		rowTexts[index] = line
		if width := lipglossWidth(line); width > maxWidth {
			maxWidth = width
		}
		if row.Count != nil {
			if width := lipglossWidth(strconv.Itoa(*row.Count)); width > maxCountWidth {
				maxCountWidth = width
			}
		}
	}
	for index, row := range rows {
		line := rowTexts[index]
		count := ""
		if row.Count != nil {
			count = strconv.Itoa(*row.Count)
			gap := maxWidth - lipglossWidth(line) + 2
			if gap < 2 {
				gap = 2
			}
			countGap := maxCountWidth - lipglossWidth(count)
			line += strings.Repeat(" ", gap+countGap)
		}
		if index == selected {
			rendered := titleStyle.Render("> " + line)
			if count != "" {
				rendered += mutedStyle.Render(count)
			}
			lines = append(lines, rendered)
			continue
		}
		if count != "" {
			line += count
		}
		lines = append(lines, "  "+mutedStyle.Render(line))
	}
	return lines
}

func onboardingCustomPathLine(selected bool) string {
	label := "custom path..."
	if selected {
		return titleStyle.Render("> " + label)
	}
	return "  " + mutedStyle.Render(label)
}

func inlineInputLines(value, placeholder string, width int, cursor int) []string {
	if value == "" {
		lines := wrapInputPlaceholderLines(placeholder, width)
		for index := range lines {
			lines[index] = mutedStyle.Render(lines[index])
		}
		last := len(lines) - 1
		lines[last] += activeCursor()
		return lines
	}
	if textCursor(value, cursor) == len([]rune(value)) {
		lines := wrapInputLines(value, width)
		last := len(lines) - 1
		lines[last] += activeCursor()
		return lines
	}
	return wrapInputLinesWithCursor(value, width, cursor, activeCursor())
}

func inlineActionHint(key, action string) string {
	return hintKeyStyle.Render(key) + " " + mutedStyle.Render(action)
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
