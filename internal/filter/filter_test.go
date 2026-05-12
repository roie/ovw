package filter

import (
	"testing"
	"time"

	"ovw/internal/config"
	"ovw/internal/format"
	"ovw/internal/project"
)

func testStatus(value string) format.StatusInfo {
	return format.StatusFromTags(value, []string{value})
}

func TestApplyCombinedFilters(t *testing.T) {
	now := time.Date(2026, 5, 6, 0, 0, 0, 0, time.Local)
	projects := []project.Project{
		{Name: "a", Status: testStatus("active"), Activity: format.ActivityInfo{Dirty: true, HasGit: true, HasCommits: true, LastCommitAt: now.AddDate(0, 0, -40)}},
		{Name: "b", Status: testStatus("active"), Activity: format.ActivityInfo{Dirty: false, HasGit: true, HasCommits: true, LastCommitAt: now.AddDate(0, 0, -40)}},
		{Name: "c", Status: testStatus("parked"), Activity: format.ActivityInfo{Dirty: true, HasGit: true, HasCommits: true, LastCommitAt: now.AddDate(0, 0, -40)}},
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
	projects := []project.Project{{Name: "a"}, {Name: "b", Status: testStatus("active")}}
	got, err := Apply(projects, Options{Untagged: true}, config.Default(), time.Now())
	if err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	if len(got) != 1 || got[0].Name != "a" {
		t.Fatalf("projects = %#v", got)
	}
}

func TestApplyAllowsFreeFormStatusFilter(t *testing.T) {
	projects := []project.Project{{Name: "a", Status: testStatus("needs review")}, {Name: "b", Status: testStatus("active")}}
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

func TestApplyPathFilterMatchesProjectPathOnly(t *testing.T) {
	projects := []project.Project{
		{Name: "library1-api", Path: "/home/me/work/client-a/api"},
		{Name: "api", Path: "/home/me/work/library1/api"},
		{Name: "other", Path: "/home/me/work/library2/other"},
	}
	got, err := Apply(projects, Options{Path: "library1"}, config.Default(), time.Now())
	if err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	if len(got) != 1 || got[0].Name != "api" {
		t.Fatalf("projects = %#v", got)
	}
}

func TestApplyPathFilterCanMatchFullPath(t *testing.T) {
	projects := []project.Project{
		{Name: "api", Path: "/home/me/work/library1/api"},
		{Name: "web", Path: "/home/me/work/library2/web"},
	}
	got, err := Apply(projects, Options{Path: "library1/api"}, config.Default(), time.Now())
	if err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	if len(got) != 1 || got[0].Name != "api" {
		t.Fatalf("projects = %#v", got)
	}
}

func TestSortModes(t *testing.T) {
	now := time.Date(2026, 5, 6, 0, 0, 0, 0, time.Local)
	cfg := config.Default()
	cfg.SortDir = "asc"
	projects := []project.Project{
		{Name: "beta", Status: testStatus("parked"), Activity: format.ActivityInfo{LastCommitAt: now.AddDate(0, 0, -1), HasCommits: true}},
		{Name: "alpha", Status: testStatus("active"), Activity: format.ActivityInfo{LastCommitAt: now.AddDate(0, 0, -3), HasCommits: true}},
	}

	got := Sort(projects, "name", cfg)
	if got[0].Name != "alpha" {
		t.Fatalf("name sort = %#v", got)
	}
	got = Sort(projects, "status", cfg)
	if got[0].Status.Value != "active" {
		t.Fatalf("status sort = %#v", got)
	}
	got = Sort(projects, "activity", config.Default())
	if got[0].Name != "beta" {
		t.Fatalf("activity sort = %#v", got)
	}
	got = Sort(projects, "name:desc", config.Default())
	if got[0].Name != "beta" {
		t.Fatalf("name desc inline sort = %#v", got)
	}
}

func TestSortKeepsPinnedProjectsFirst(t *testing.T) {
	now := time.Date(2026, 5, 6, 0, 0, 0, 0, time.Local)
	projects := []project.Project{
		{Name: "alpha", Activity: format.ActivityInfo{LastCommitAt: now.AddDate(0, 0, -1), HasCommits: true}},
		{Name: "zulu", Pinned: true, Activity: format.ActivityInfo{LastCommitAt: now.AddDate(0, 0, -10), HasCommits: true}},
		{Name: "beta", Pinned: true, Activity: format.ActivityInfo{LastCommitAt: now.AddDate(0, 0, -3), HasCommits: true}},
	}

	got := Sort(projects, "name:asc", config.Default())
	if got[0].Name != "beta" || got[1].Name != "zulu" || got[2].Name != "alpha" {
		t.Fatalf("name sort with pinned projects = %#v", got)
	}
	got = Sort(projects, "activity:desc", config.Default())
	if got[0].Name != "beta" || got[1].Name != "zulu" || got[2].Name != "alpha" {
		t.Fatalf("activity sort with pinned projects = %#v", got)
	}
}

func TestSortDirectionAppliesToNameAndStatus(t *testing.T) {
	cfg := config.Default()
	cfg.SortDir = "desc"
	projects := []project.Project{
		{Name: "alpha", Status: testStatus("active")},
		{Name: "beta", Status: testStatus("parked")},
	}

	got := Sort(projects, "name", cfg)
	if got[0].Name != "beta" {
		t.Fatalf("name desc sort = %#v", got)
	}
	got = Sort(projects, "status", cfg)
	if got[0].Status.Value != "parked" {
		t.Fatalf("status desc sort = %#v", got)
	}
}

func TestValidateSortRejectsUnknownValue(t *testing.T) {
	err := ValidateSort("recent")
	if err == nil || err.Error() != `invalid sort "recent": expected activity, name, or status` {
		t.Fatalf("ValidateSort() error = %v", err)
	}
	err = ValidateSort("name:up")
	if err == nil || err.Error() != `invalid sort "name:up": expected direction asc or desc` {
		t.Fatalf("ValidateSort() error = %v", err)
	}
	err = ValidateSort("name:")
	if err == nil || err.Error() != `invalid sort "name:": expected direction asc or desc` {
		t.Fatalf("ValidateSort() error = %v", err)
	}
	for _, value := range []string{"", "activity", "name", "status", "name:asc", "activity:desc"} {
		if err := ValidateSort(value); err != nil {
			t.Fatalf("ValidateSort(%q) error = %v", value, err)
		}
	}
}

func TestParseSortUsesConfigDirection(t *testing.T) {
	cfg := config.Default()
	cfg.SortBy = "name"
	cfg.SortDir = "asc"

	spec, err := ParseSort("", cfg)
	if err != nil {
		t.Fatalf("ParseSort() error = %v", err)
	}
	if spec.By != "name" || spec.Dir != "asc" {
		t.Fatalf("ParseSort() = %#v", spec)
	}
	spec, err = ParseSort("status:desc", cfg)
	if err != nil {
		t.Fatalf("ParseSort() error = %v", err)
	}
	if spec.By != "status" || spec.Dir != "desc" {
		t.Fatalf("ParseSort() = %#v", spec)
	}
}
