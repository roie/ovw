package textwrap

import (
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func TestLinesUsesDisplayWidth(t *testing.T) {
	got := Lines("你好 世界", 5)
	want := []string{"你好", "世界"}
	if len(got) != len(want) {
		t.Fatalf("Lines() = %#v, want %#v", got, want)
	}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("Lines() = %#v, want %#v", got, want)
		}
		if width := ansi.StringWidth(got[index]); width > 5 {
			t.Fatalf("line width = %d, want <= 5: %#v", width, got)
		}
	}
}
