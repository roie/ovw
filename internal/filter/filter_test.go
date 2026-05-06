package filter

import (
	"testing"
	"time"

	"ovw/internal/config"
	"ovw/internal/format"
	"ovw/internal/project"
)

func TestApplyCombinedFilters(t *testing.T) {
	now := time.Date(2026, 5, 6, 0, 0, 0, 0, time.Local)
	projects := []project.Project{
		{Name: "a", Status: "active", Activity: format.ActivityInfo{Dirty: true, HasGit: true, HasCommits: true, LastCommitAt: now.AddDate(0, 0, -40)}},
		{Name: "b", Status: "active", Activity: format.ActivityInfo{Dirty: false, HasGit: true, HasCommits: true, LastCommitAt: now.AddDate(0, 0, -40)}},
		{Name: "c", Status: "parked", Activity: format.ActivityInfo{Dirty: true, HasGit: true, HasCommits: true, LastCommitAt: now.AddDate(0, 0, -40)}},
	}
	opts := Options{Status: "active", Dirty: true, Stale: true}

	got, err := Apply(projects, opts, config.Default(), now)
	if err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	if len(got) != 1 || got[0].Name != "a" {
		t.Fatalf("projects = %#v", got)
	}
}

func TestApplyUntaggedFilter(t *testing.T) {
	projects := []project.Project{{Name: "a"}, {Name: "b", Status: "active"}}
	got, err := Apply(projects, Options{Untagged: true}, config.Default(), time.Now())
	if err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	if len(got) != 1 || got[0].Name != "a" {
		t.Fatalf("projects = %#v", got)
	}
}

func TestApplyRejectsUnknownStatus(t *testing.T) {
	_, err := Apply(nil, Options{Status: "building"}, config.Default(), time.Now())
	if err == nil {
		t.Fatal("expected unknown status error")
	}
}

func TestSortModes(t *testing.T) {
	now := time.Date(2026, 5, 6, 0, 0, 0, 0, time.Local)
	projects := []project.Project{
		{Name: "beta", Status: "parked", Activity: format.ActivityInfo{LastCommitAt: now.AddDate(0, 0, -1), HasCommits: true}},
		{Name: "alpha", Status: "active", Activity: format.ActivityInfo{LastCommitAt: now.AddDate(0, 0, -3), HasCommits: true}},
	}

	got := Sort(projects, "name", config.Default())
	if got[0].Name != "alpha" {
		t.Fatalf("name sort = %#v", got)
	}
	got = Sort(projects, "status", config.Default())
	if got[0].Status != "active" {
		t.Fatalf("status sort = %#v", got)
	}
	got = Sort(projects, "activity", config.Default())
	if got[0].Name != "beta" {
		t.Fatalf("activity sort = %#v", got)
	}
}
