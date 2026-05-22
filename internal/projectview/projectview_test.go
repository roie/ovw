package projectview

import (
	"testing"
	"time"

	"ovw/internal/format"
	"ovw/internal/project"
)

func TestFieldsUseCanonicalDetailOrder(t *testing.T) {
	project := project.Project{
		Name:        "eventca",
		Path:        "/tmp/eventca",
		Stack:       []string{"Go", "Cobra"},
		Managers:    []string{"go modules"},
		Scripts:     []string{"dev", "build", "check"},
		Version:     "1.2.3",
		Ports:       []int{3000, 8787},
		Status:      format.StatusFromTags("parked", []string{"dirty", "parked"}),
		Description: "Project description",
		Note:        format.NoteInfo{Display: "user note"},
		Fields: map[string]string{
			"jira":     "OVW-123",
			"priority": "high",
		},
		UpdatedAt: time.Date(2026, 5, 7, 12, 30, 0, 0, time.UTC),
		Activity: format.ActivityInfo{
			Display:    "12m",
			Branch:     "main",
			HasGit:     true,
			HasCommits: true,
		},
	}

	fields := Fields(project, Options{
		Path: func(path string) string { return "~" + path },
		Time: func(time.Time) string { return "2026-05-07 12:30" },
		Fields: []OptionsField{
			{ID: "priority", Label: "Priority"},
			{ID: "jira", Label: "Jira"},
		},
	})

	got := make([]string, 0, len(fields))
	for _, field := range fields {
		got = append(got, field.Label)
	}
	want := []string{"Path", "Stack", "Manager", "Scripts", "Version", "Ports", "Branch", "Activity", "Updated", "Status", "Note", "Priority", "Jira"}
	if len(got) != len(want) {
		t.Fatalf("labels = %#v, want %#v", got, want)
	}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("labels = %#v, want %#v", got, want)
		}
	}
	if subtitle := Subtitle(project); subtitle != "Project description" {
		t.Fatalf("Subtitle() = %q, want Project description", subtitle)
	}
}

func TestFieldsCanHideEmptyDisplayFields(t *testing.T) {
	fields := Fields(project.Project{Path: "/tmp/app"}, Options{HideEmptyDisplayFields: true})

	got := make([]string, 0, len(fields))
	for _, field := range fields {
		got = append(got, field.Label)
		if field.Value == "—" {
			t.Fatalf("Fields() should hide placeholder field = %#v", field)
		}
	}
	want := []string{"Path"}
	if len(got) != len(want) {
		t.Fatalf("labels = %#v, want %#v", got, want)
	}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("labels = %#v, want %#v", got, want)
		}
	}
}

func TestColumnValueMarksPinnedProjectName(t *testing.T) {
	proj := project.Project{Pinned: true}

	if got := ColumnValue(proj, "name", "app"); got != "★ app" {
		t.Fatalf("ColumnValue(name) = %q, want ★ app", got)
	}
}

func TestColumnValueShowsPorts(t *testing.T) {
	proj := project.Project{Ports: []int{3000, 8787}}

	if got := ColumnValue(proj, "ports", "app"); got != "3000, 8787" {
		t.Fatalf("ColumnValue(ports) = %q, want 3000, 8787", got)
	}
	if got := ColumnValue(project.Project{}, "ports", "app"); got != "—" {
		t.Fatalf("ColumnValue(empty ports) = %q, want —", got)
	}
}

func TestColumnValueShowsBranch(t *testing.T) {
	proj := project.Project{Activity: format.ActivityInfo{Branch: "feat/pins"}}

	if got := ColumnValue(proj, "branch", "app"); got != "feat/pins" {
		t.Fatalf("ColumnValue(branch) = %q, want feat/pins", got)
	}
	if got := ColumnValue(project.Project{}, "branch", "app"); got != "—" {
		t.Fatalf("ColumnValue(empty branch) = %q, want —", got)
	}
}

func TestColumnValueShowsUpdatedTimestamp(t *testing.T) {
	proj := project.Project{UpdatedAt: time.Date(2026, 5, 12, 9, 30, 0, 0, time.UTC)}

	if got := ColumnValue(proj, "updated", "app"); got != "2026-05-12 09:30" {
		t.Fatalf("ColumnValue(updated) = %q, want 2026-05-12 09:30", got)
	}
	if got := ColumnValue(project.Project{}, "updated", "app"); got != "—" {
		t.Fatalf("ColumnValue(empty updated) = %q, want —", got)
	}
}

func TestColumnValueShowsCustomField(t *testing.T) {
	proj := project.Project{Fields: map[string]string{"jira": "OVW-123"}}

	if got := ColumnValue(proj, "field:jira", "app"); got != "OVW-123" {
		t.Fatalf("ColumnValue(field:jira) = %q, want OVW-123", got)
	}
	if got := ColumnValue(project.Project{}, "field:jira", "app"); got != "—" {
		t.Fatalf("ColumnValue(empty custom field) = %q, want —", got)
	}
}

func TestColumnValueDisplaysCheckboxCustomFieldAsState(t *testing.T) {
	proj := project.Project{
		Fields:    map[string]string{"reviewed": "true", "blocked": "false"},
		FieldDefs: []project.FieldDef{{ID: "reviewed", Type: "checkbox"}, {ID: "blocked", Type: "checkbox"}},
	}

	if got := ColumnValue(proj, "field:reviewed", "app"); got != "[x]" {
		t.Fatalf("ColumnValue(field:reviewed) = %q, want [x]", got)
	}
	if got := ColumnValue(proj, "field:blocked", "app"); got != "[ ]" {
		t.Fatalf("ColumnValue(field:blocked) = %q, want [ ]", got)
	}
}

func TestFieldsUseProjectFieldDefinitionsByDefault(t *testing.T) {
	project := project.Project{
		Path:      "/tmp/app",
		Fields:    map[string]string{"jira": "OVW-123"},
		FieldDefs: []project.FieldDef{{ID: "jira", Label: "Jira"}},
	}

	fields := Fields(project, Options{HideEmptyDisplayFields: true})

	got := make([]string, 0, len(fields))
	for _, field := range fields {
		got = append(got, field.Label)
	}
	want := []string{"Path", "Jira"}
	if len(got) != len(want) {
		t.Fatalf("labels = %#v, want %#v", got, want)
	}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("labels = %#v, want %#v", got, want)
		}
	}
}

func TestFieldsDisplayCheckboxValuesAsState(t *testing.T) {
	project := project.Project{
		Path: "/tmp/app",
		Fields: map[string]string{
			"reviewed": "true",
			"blocked":  "false",
			"empty":    "",
		},
		FieldDefs: []project.FieldDef{
			{ID: "reviewed", Label: "Reviewed", Type: "checkbox"},
			{ID: "blocked", Label: "Blocked", Type: "checkbox"},
			{ID: "empty", Label: "Empty", Type: "checkbox"},
		},
	}

	fields := Fields(project, Options{HideEmptyDisplayFields: true})
	values := map[string]string{}
	for _, field := range fields {
		values[field.Label] = field.Value
	}

	if values["Reviewed"] != "[x]" {
		t.Fatalf("Reviewed = %q, want [x]", values["Reviewed"])
	}
	if values["Blocked"] != "[ ]" {
		t.Fatalf("Blocked = %q, want [ ]", values["Blocked"])
	}
	if values["Empty"] != "" {
		t.Fatalf("Empty = %q, want blank", values["Empty"])
	}
}

func TestFieldsDoNotExposeRedundantGitOrLastCommitFields(t *testing.T) {
	project := project.Project{
		Path:   "/tmp/eventca",
		Stack:  []string{"Go"},
		Status: format.StatusFromTags("", []string{"dirty"}),
		Activity: format.ActivityInfo{
			Display:           "12m ↑2",
			LastCommitAge:     "12m",
			LastCommitMessage: "feat: add overview",
			Dirty:             true,
			Unpushed:          2,
			HasGit:            true,
			HasCommits:        true,
		},
	}

	for _, field := range Fields(project, Options{Activity: DetailActivity}) {
		if field.Label == "Git" || field.Label == "Last commit" {
			t.Fatalf("unexpected detail field = %#v", field)
		}
	}
}
