package tui

import (
	"errors"
	"os/exec"
	"strings"
)

func OpenEditor(editor, path string) error {
	parts := strings.Fields(editor)
	if len(parts) == 0 {
		return errors.New("editor is not configured")
	}
	args := append(parts[1:], path)
	return exec.Command(parts[0], args...).Run()
}
