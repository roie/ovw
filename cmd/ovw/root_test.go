package ovw

import (
	"bytes"
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
