package tui

import (
	"sort"
	"strings"

	"ovw/internal/project"
)

var runnerScriptPriority = []string{"dev", "start", "build", "test", "check", "lint"}

type runnerScript struct {
	Name    string
	Command string
}

func runnerView(project project.Project, selected int, input string, cursor int, cursorState ...inputCursorState) string {
	modal, _ := runnerViewWithScroll(project, selected, input, cursor, 0, 0, cursorState...)
	return modal
}

func runnerViewWithScroll(project project.Project, selected int, input string, cursor int, height int, offset int, cursorState ...inputCursorState) (string, int) {
	scripts := runnerScriptOptions(project, input)
	lines := inputModalLines(input, "filter scripts...", 38, cursor, cursorState...)
	lines = append(lines, "")
	optionLines := runnerOptionLines(scripts, selected)
	optionLines, maxOffset := scrollRunnerOptionLines(optionLines, height, offset)
	lines = append(lines, optionLines...)
	lines = append(lines, "", actionHint("enter", "run"))
	return modalView(modalTitleWithProject("Runner", project.Name), lines, 42), maxOffset
}

func runnerOptionLines(scripts []runnerScript, selected int) []string {
	if len(scripts) == 0 {
		return []string{modalMuted("No matching scripts")}
	}
	labels := make([]string, 0, len(scripts))
	for _, script := range scripts {
		labels = append(labels, script.Name)
	}
	return modalOptionLines(labels, selected)
}

func scrollRunnerOptionLines(lines []string, height int, offset int) ([]string, int) {
	visibleHeight := runnerVisibleOptionHeight(height)
	if height <= 0 || len(lines) <= visibleHeight {
		return lines, 0
	}
	maxOffset := len(lines) - visibleHeight
	if offset < 0 {
		offset = 0
	}
	if offset > maxOffset {
		offset = maxOffset
	}
	end := offset + visibleHeight
	if end > len(lines) {
		end = len(lines)
	}
	visible := append([]string{}, lines[offset:end]...)
	visible = append(visible, modalDetailScrollHint(offset, maxOffset, 38))
	return visible, maxOffset
}

func runnerVisibleOptionHeight(height int) int {
	if height <= 0 {
		return 1 << 30
	}
	bodyHeight := height - 4
	if bodyHeight < 1 {
		bodyHeight = 1
	}
	// Input line, blank separator, blank before action, action line, scroll hint.
	visibleHeight := bodyHeight - 5
	if visibleHeight < 1 {
		visibleHeight = 1
	}
	return visibleHeight
}

func runnerScriptOptions(project project.Project, query string) []runnerScript {
	ordered := prioritizedScripts(project)
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return ordered
	}
	filtered := make([]runnerScript, 0, len(ordered))
	for _, script := range ordered {
		if strings.Contains(strings.ToLower(script.Name), query) {
			filtered = append(filtered, script)
		}
	}
	return filtered
}

func prioritizedScripts(project project.Project) []runnerScript {
	scripts := project.Scripts
	ordered := make([]runnerScript, 0, len(scripts)+len(project.CustomScripts))
	seen := make(map[string]bool, len(scripts)+len(project.CustomScripts))
	customNames := make([]string, 0, len(project.CustomScripts))
	for name := range project.CustomScripts {
		customNames = append(customNames, name)
	}
	sort.Strings(customNames)
	for _, name := range customNames {
		ordered = append(ordered, runnerScript{Name: name, Command: project.CustomScripts[name]})
		seen[name] = true
	}
	add := func(want string) {
		for _, script := range scripts {
			if script == want && !seen[script] {
				ordered = append(ordered, runnerScript{Name: script})
				seen[script] = true
				return
			}
		}
	}
	for _, script := range runnerScriptPriority {
		add(script)
	}
	for _, script := range scripts {
		if !seen[script] {
			ordered = append(ordered, runnerScript{Name: script})
			seen[script] = true
		}
	}
	return ordered
}
