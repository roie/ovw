package tui

import (
	"errors"
	"os/exec"

	"ovw/internal/commandline"
)

func OpenEditor(editor, path string) error {
	parts, err := commandline.Split(editor)
	if err != nil {
		return err
	}
	if len(parts) == 0 {
		return errors.New("editor is not configured")
	}
	args := append(parts[1:], path)
	return exec.Command(parts[0], args...).Run()
}
