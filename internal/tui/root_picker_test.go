package tui

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
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

func TestRootPickerLeftCollapsesExpandedNestedRowBeforeSelectingParent(t *testing.T) {
	picker := rootPicker{
		options: []string{"~/dev"},
		expanded: map[string]bool{
			"~/dev":     true,
			"~/dev/web": true,
		},
		children: map[string][]string{
			"~/dev":     {"~/dev/web"},
			"~/dev/web": {"~/dev/web/app"},
		},
		selected: 1,
	}

	rows := picker.visibleRows()
	picker.collapseOrSelectParent(rows[picker.selected])

	if picker.expanded["~/dev/web"] {
		t.Fatal("expected expanded nested row to collapse")
	}
	if picker.selected != 1 {
		t.Fatalf("selected = %d, want row to stay selected while collapsing", picker.selected)
	}
	rows = picker.visibleRows()
	if got, want := len(rows), 2; got != want {
		t.Fatalf("visible rows = %d, want %d after collapse: %#v", got, want, rows)
	}

	picker.collapseOrSelectParent(rows[picker.selected])
	if picker.selected != 0 {
		t.Fatalf("selected = %d, want parent selected after second left", picker.selected)
	}
}

func TestRootPickerDoesNotShowCaretForVisibleLeafFolder(t *testing.T) {
	root := t.TempDir()
	branch := filepath.Join(root, "branch")
	leaf := filepath.Join(root, "leaf")
	if err := os.MkdirAll(filepath.Join(branch, "child"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(leaf, 0o755); err != nil {
		t.Fatal(err)
	}
	picker := rootPicker{
		options:  []string{branch, leaf},
		checked:  map[string]bool{branch: true, leaf: true},
		expanded: map[string]bool{},
		children: map[string][]string{},
		counts:   map[string]int{branch: 1, leaf: 0},
	}

	rows := picker.visibleRows()
	if len(rows) != 2 {
		t.Fatalf("visible rows = %#v, want branch and leaf", rows)
	}
	if !rows[0].Expandable {
		t.Fatalf("branch row is not expandable: %#v", rows[0])
	}
	if rows[1].Expandable {
		t.Fatalf("leaf row is expandable: %#v", rows[1])
	}
	view := stripANSI(strings.Join(modalSetupRowLines(rows, -1), "\n"))
	branchLine := lineContaining(view, branch)
	if !strings.ContainsAny(branchLine, "▸▾") {
		t.Fatalf("branch line should show a caret: %q", branchLine)
	}
	leafLine := lineContaining(view, leaf)
	if leafLine == "" {
		t.Fatalf("leaf line missing from view:\n%s", view)
	}
	if strings.ContainsAny(leafLine, "▸▾") {
		t.Fatalf("leaf line should not show a caret: %q", leafLine)
	}
}

func lineContaining(text, value string) string {
	for _, line := range strings.Split(text, "\n") {
		if strings.Contains(line, value) {
			return line
		}
	}
	return ""
}
