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
		Version:     "1.2.3",
		Status:      format.StatusFromTags("parked", []string{"dirty", "parked"}),
		Description: "Project description",
		Note:        format.NoteInfo{Display: "user note"},
		Activity: format.ActivityInfo{
			Display:      "12m",
			LastCommitAt: time.Date(2026, 5, 7, 12, 30, 0, 0, time.UTC),
			Branch:       "main",
			HasGit:       true,
			HasCommits:   true,
		},
	}

	fields := Fields(project, Options{
		Path: func(path string) string { return "~" + path },
		Time: func(time.Time) string { return "2026-05-07 12:30" },
	})

	got := make([]string, 0, len(fields))
	for _, field := range fields {
		got = append(got, field.Label)
	}
	want := []string{"Path", "Stack", "Manager", "Version", "Branch", "Activity", "Updated", "Status", "Note"}
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

func TestFieldsDoNotExposeRedundantGitOrLastCommitFields(t *testing.T) {
	project := project.Project{
		Path:   "/tmp/eventca",
		Stack:  []string{"Go"},
		Status: format.StatusFromTags("", []string{"dirty", "unpushed"}),
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
