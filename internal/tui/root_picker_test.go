package tui

import (
	"reflect"
	"testing"
)

func TestRootPickerRevealsCheckedNestedRoots(t *testing.T) {
	picker := rootPicker{
		options:  []string{"~/dev"},
		checked:  map[string]bool{"~/dev/web/eventca": true},
		expanded: map[string]bool{},
		children: map[string][]string{
			"~/dev":     {"~/dev/web", "~/dev/other"},
			"~/dev/web": {"~/dev/web/eventca", "~/dev/web/other"},
		},
	}
	picker.revealChecked()

	rows := picker.visibleRows()
	got := make([]string, 0, len(rows))
	for _, row := range rows {
		got = append(got, row.Path)
	}
	want := []string{"~/dev", "~/dev/web", "~/dev/web/eventca", "~/dev/web/other", "~/dev/other"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("visible rows = %#v, want %#v", got, want)
	}
	if !rows[0].Partial || !rows[1].Partial || !rows[2].Checked {
		t.Fatalf("nested checked state = %#v", rows)
	}
}

func TestRootPickerSelectedRootsExcludesUncheckedChild(t *testing.T) {
	picker := rootPicker{
		options:  []string{"~/dev"},
		checked:  map[string]bool{"~/dev": true},
		expanded: map[string]bool{"~/dev": true},
		children: map[string][]string{"~/dev": {"~/dev/web", "~/dev/extensions"}},
	}

	rows := picker.visibleRows()
	picker.toggleRow(rows[1])

	want := []string{"~/dev/extensions"}
	if got := picker.selectedRoots(); !reflect.DeepEqual(got, want) {
		t.Fatalf("selected roots = %#v, want %#v", got, want)
	}
}
