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

func TestApplyAllowsFreeFormStatusFilter(t *testing.T) {
	projects := []project.Project{{Name: "a", Status: "needs review"}, {Name: "b", Status: "active"}}
	got, err := Apply(projects, Options{Status: "needs review"}, config.Default(), time.Now())
	if err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	if len(got) != 1 || got[0].Name != "a" {
		t.Fatalf("projects = %#v", got)
	}
}

func TestApplyHiddenFilter(t *testing.T) {
	projects := []project.Project{{Name: "visible"}, {Name: "hidden", Hidden: true}}
	got, err := Apply(projects, Options{Hidden: true}, config.Default(), time.Now())
	if err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	if len(got) != 1 || got[0].Name != "hidden" {
		t.Fatalf("projects = %#v", got)
	}
}

func TestSortModes(t *testing.T) {
	now := time.Date(2026, 5, 6, 0, 0, 0, 0, time.Local)
	cfg := config.Default()
	cfg.SortDir = "asc"
	projects := []project.Project{
		{Name: "beta", Status: "parked", Activity: format.ActivityInfo{LastCommitAt: now.AddDate(0, 0, -1), HasCommits: true}},
		{Name: "alpha", Status: "active", Activity: format.ActivityInfo{LastCommitAt: now.AddDate(0, 0, -3), HasCommits: true}},
	}

	got := Sort(projects, "name", cfg)
	if got[0].Name != "alpha" {
		t.Fatalf("name sort = %#v", got)
	}
	got = Sort(projects, "status", cfg)
	if got[0].Status != "active" {
		t.Fatalf("status sort = %#v", got)
	}
	got = Sort(projects, "activity", config.Default())
	if got[0].Name != "beta" {
		t.Fatalf("activity sort = %#v", got)
	}
}

func TestSortDirectionAppliesToNameAndStatus(t *testing.T) {
	cfg := config.Default()
	cfg.SortDir = "desc"
	projects := []project.Project{
		{Name: "alpha", Status: "active"},
		{Name: "beta", Status: "parked"},
	}

	got := Sort(projects, "name", cfg)
	if got[0].Name != "beta" {
		t.Fatalf("name desc sort = %#v", got)
	}
	got = Sort(projects, "status", cfg)
	if got[0].Status != "parked" {
		t.Fatalf("status desc sort = %#v", got)
	}
}
