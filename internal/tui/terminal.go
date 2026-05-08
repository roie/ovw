package tui

import (
	"errors"
	"os"
	"os/exec"
	"runtime"

	tea "github.com/charmbracelet/bubbletea"
)

func runTerminal(path, name, configuredShell string) tea.Cmd {
	shell := configuredShell
	if shell == "" {
		shell = os.Getenv("SHELL")
	}
	if shell == "" && runtime.GOOS == "windows" {
		shell = os.Getenv("COMSPEC")
	}
	if shell == "" {
		if runtime.GOOS == "windows" {
			return func() tea.Msg {
				return terminalFailedMsg{err: errors.New("COMSPEC is not configured")}
			}
		}
		shell = "/bin/sh"
	}
	cmd := exec.Command(shell)
	cmd.Dir = path
	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		if err != nil {
			return terminalFailedMsg{err: err}
		}
		return terminalOpenedMsg{message: "Opened terminal " + name}
	})
}
