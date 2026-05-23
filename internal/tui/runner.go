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
	Custom  bool
}

func runnerView(project project.Project, selected int, input string, cursor int, cursorState ...inputCursorState) string {
	modal, _ := runnerViewWithScroll(project, selected, input, cursor, 0, 0, "", false, false, cursorState...)
	return modal
}

func runnerViewWithScroll(project project.Project, selected int, input string, cursor int, height int, offset int, addName string, adding bool, showInfo bool, cursorState ...inputCursorState) (string, int) {
	scripts := runnerScriptOptions(project, input)
	placeholder := "filter or add scripts..."
	if len(runnerScriptOptions(project, "")) == 0 {
		placeholder = "add script..."
	}
	if adding {
		placeholder = addName + " = command"
	}
	lines := inputModalLines(input, placeholder, 38, cursor, cursorState...)
	if adding && addName != "" {
		lines[0] = modalMuted(addName+" = ") + lines[0]
	}
	lines = append(lines, "")
	optionLines := runnerOptionLines(project, scripts, selected, input, adding, showInfo)
	optionLines, maxOffset := scrollRunnerOptionLines(optionLines, height, offset)
	lines = append(lines, optionLines...)
	action := runnerActionHint(scripts, selected, strings.TrimSpace(input), adding, showInfo)
	if adding {
		action = actionHint("enter", "save")
	}
	if action != "" {
		lines = append(lines, "", action)
	}
	return modalView(modalTitleWithProject("Runner", project.Name), lines, 42), maxOffset
}

func runnerOptionLines(project project.Project, scripts []runnerScript, selected int, input string, adding bool, showInfo bool) []string {
	if adding {
		return nil
	}
	if len(scripts) == 0 {
		if strings.TrimSpace(input) != "" {
			return []string{modalAccent("+ add script")}
		}
		return []string{modalMuted("No scripts yet")}
	}
	lines := make([]string, 0, len(scripts)*2)
	for index, script := range scripts {
		if index == selected {
			lines = append(lines, modalAccent("> "+script.Name))
		} else {
			lines = append(lines, "  "+modalMuted(script.Name))
		}
		if showInfo {
			lines = append(lines, "    "+modalMuted(runnerScriptDetail(script, project)))
		}
	}
	return lines
}

func runnerActionHint(scripts []runnerScript, selected int, input string, adding bool, showInfo bool) string {
	if adding {
		return actionHint("enter", "save")
	}
	scriptCount := len(scripts)
	if scriptCount == 0 && input == "" {
		return ""
	}
	if scriptCount == 0 {
		return actionHint("enter", "add")
	}
	actions := []string{actionHint("enter", "run")}
	if showInfo {
		actions = append(actions, actionHint("space", "collapse"))
	} else {
		actions = append(actions, actionHint("space", "details"))
	}
	if selected >= 0 && selected < len(scripts) && scripts[selected].Custom {
		actions = append(actions, actionHint("del", "delete"))
	}
	return strings.Join(actions, " · ")
}

func runnerScriptDetail(script runnerScript, project project.Project) string {
	if script.Command != "" {
		return script.Command
	}
	return scriptManager(project.Managers) + " run " + script.Name
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

func runnerSelectedLineRange(selected int, showInfo bool) (int, int) {
	if selected < 0 {
		selected = 0
	}
	if showInfo {
		start := selected * 2
		return start, start + 1
	}
	return selected, selected
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
		ordered = append(ordered, runnerScript{Name: name, Command: project.CustomScripts[name], Custom: true})
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
