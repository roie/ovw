package render

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"ovw/internal/config"
	"ovw/internal/format"
	"ovw/internal/project"
)

func TestTableContainsHeaderAndColumns(t *testing.T) {
	projects := []project.Project{{
		Name:         "eventca",
		StackDisplay: "SvelteKit+CF",
		Activity:     format.ActivityInfo{Display: "2d ↑2 !"},
		Status:       "active",
		Note:         format.NoteInfo{Display: "feat/checkin · fix"},
	}}
	var out bytes.Buffer
	if err := Table(&out, projects, config.Default(), 200*time.Millisecond); err != nil {
		t.Fatalf("Table() error = %v", err)
	}
	got := out.String()
	for _, want := range []string{"ovw — 1 projects · scanned in 0.2s", "Name", "Stack", "Activity", "Status", "Note", "----", "eventca", "SvelteKit+CF"} {
		if !strings.Contains(got, want) {
			t.Fatalf("table missing %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "----  -----") {
		t.Fatalf("table uses disconnected column separators:\n%s", got)
	}
	for _, unwanted := range []string{"NAME", "STACK", "ACTIVITY", "STATUS", "NOTE"} {
		if strings.Contains(got, unwanted) {
			t.Fatalf("table contains uppercase header %q:\n%s", unwanted, got)
		}
	}
}

func TestTableDisambiguatesDuplicateNames(t *testing.T) {
	projects := []project.Project{
		{
			Name:         "vibe-oke",
			Path:         "/home/roie/dev/web/vibe-oke",
			StackDisplay: "Node",
			Activity:     format.ActivityInfo{Display: "1d"},
		},
		{
			Name:         "vibe-oke",
			Path:         "/home/roie/dev/playground/vibe-oke",
			StackDisplay: "Hono+Bun",
			Activity:     format.ActivityInfo{Display: "2d"},
		},
	}
	var out bytes.Buffer
	if err := Table(&out, projects, config.Default(), 100*time.Millisecond); err != nil {
		t.Fatalf("Table() error = %v", err)
	}
	got := out.String()
	for _, want := range []string{"web/vibe-oke", "playground/vibe-oke"} {
		if !strings.Contains(got, want) {
			t.Fatalf("table missing disambiguated name %q:\n%s", want, got)
		}
	}
}

func TestJSONOutputsPureArray(t *testing.T) {
	projects := []project.Project{{
		Name:         "eventca",
		Path:         "/tmp/eventca",
		Stack:        []string{"Go"},
		StackDisplay: "Go",
		Activity:     format.ActivityInfo{Display: "1d"},
		Status:       "active",
		Note:         format.NoteInfo{Display: "note", Source: "manual", Manual: "note"},
		Manual:       true,
	}}
	var out bytes.Buffer
	if err := JSON(&out, projects); err != nil {
		t.Fatalf("JSON() error = %v", err)
	}
	got := strings.TrimSpace(out.String())
	if !strings.HasPrefix(got, "[") || !strings.HasSuffix(got, "]") {
		t.Fatalf("json is not a pure array: %q", got)
	}
	var decoded []project.Project
	if err := json.Unmarshal(out.Bytes(), &decoded); err != nil {
		t.Fatalf("invalid json: %v\n%s", err, out.String())
	}
	if len(decoded) != 1 || decoded[0].Name != "eventca" {
		t.Fatalf("decoded = %#v", decoded)
	}
}
