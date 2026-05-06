package ovw

import (
	"bytes"
	"path/filepath"
	"testing"
)

func TestHelpIncludesUsage(t *testing.T) {
	cmd := NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--help"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	got := out.String()
	if !bytes.Contains([]byte(got), []byte("Usage:")) {
		t.Fatalf("help output missing Usage: %q", got)
	}
	if !bytes.Contains([]byte(got), []byte("ovw")) {
		t.Fatalf("help output missing command name: %q", got)
	}
}

func TestConfigPathCommand(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", "")

	cmd := NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"config", "path"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	want := filepath.Join(home, ".config", "ovw", "config.toml") + "\n"
	if out.String() != want {
		t.Fatalf("config path = %q, want %q", out.String(), want)
	}
}
