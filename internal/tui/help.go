package tui

func helpView() string {
	entries := []struct {
		key    string
		action string
	}{
		{key: "↑↓ or j/k", action: "move"},
		{key: "←→ or h/l", action: "scroll"},
		{key: "/", action: "search"},
		{key: "a", action: "add"},
		{key: "f", action: "filter"},
		{key: "s", action: "sort"},
		{key: "c", action: "columns"},
		{key: "enter", action: "details"},
		{key: "n", action: "note"},
		{key: "m", action: "status"},
		{key: "r", action: "reload"},
		{key: "o", action: "open"},
		{key: "t", action: "terminal"},
		{key: "esc", action: "back"},
		{key: "q", action: "quit"},
	}
	lines := make([]string, 0, len(entries))
	for _, entry := range entries {
		lines = append(lines, keyActionLine(entry.key, entry.action, 14))
	}
	return modalView("Help", lines, 54)
}
