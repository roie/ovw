package tui

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestOpenEditorAllowsQuotedCommandPathAndArgs(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake editor uses a POSIX shell")
	}
	editorPath, recordPath := writeEditorRecorder(t)
	target := filepath.Join(t.TempDir(), "project")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatal(err)
	}

	err := OpenEditor(quoteCommandPart(editorPath)+" --reuse-window", target)
	if err != nil {
		t.Fatalf("OpenEditor() error = %v", err)
	}
	data, err := os.ReadFile(recordPath)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(data), "--reuse-window\n"+target+"\n"; got != want {
		t.Fatalf("editor args = %q, want %q", got, want)
	}
}

func writeEditorRecorder(t *testing.T) (string, string) {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "Editor App")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	recordPath := filepath.Join(t.TempDir(), "opened.txt")
	editorPath := filepath.Join(dir, "editor")
	script := "#!/bin/sh\nprintf '%s\\n' \"$@\" > \"$OVW_EDITOR_RECORD\"\n"
	if err := os.WriteFile(editorPath, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("OVW_EDITOR_RECORD", recordPath)
	return editorPath, recordPath
}

func quoteCommandPart(value string) string {
	return `"` + strings.ReplaceAll(value, `"`, `\"`) + `"`
}
