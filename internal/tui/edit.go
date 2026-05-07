package tui

func noteView(value string) string {
	if value == "" {
		return titleStyle.Render("Note") + "\n" + mutedStyle.Render("empty clears manual note")
	}
	return titleStyle.Render("Note") + "\n" + value
}
