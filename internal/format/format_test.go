package format

import (
	"testing"
	"time"

	"ovw/internal/config"
	"ovw/internal/gitactivity"
)

func TestActivityDisplayIncludesAgeAndUnpushed(t *testing.T) {
	now := time.Date(2026, 5, 6, 12, 0, 0, 0, time.Local)
	info := gitactivity.Info{
		HasGit:       true,
		HasCommits:   true,
		LastCommitAt: now.Add(-48 * time.Hour),
		Unpushed:     2,
		Dirty:        true,
	}
	got := Activity(info, config.Default(), now)
	if got.Display != "2d ↑2" || got.LastCommitAge != "2d" {
		t.Fatalf("activity = %#v", got)
	}
}

func TestActivityDisplayHonorsDisabledFlags(t *testing.T) {
	now := time.Date(2026, 5, 6, 12, 0, 0, 0, time.Local)
	cfg := config.Default()
	cfg.ShowUnpushed = false
	info := gitactivity.Info{
		HasGit:       true,
		HasCommits:   true,
		LastCommitAt: now.Add(-24 * time.Hour),
		Unpushed:     3,
		Dirty:        true,
	}
	if got := Activity(info, cfg, now); got.Display != "1d" {
		t.Fatalf("display = %q", got.Display)
	}
}

func TestRelativeAgeUsesMinutesAndHours(t *testing.T) {
	now := time.Date(2026, 5, 6, 12, 0, 0, 0, time.Local)
	cases := []struct {
		name string
		then time.Time
		want string
	}{
		{name: "now", then: now.Add(-30 * time.Second), want: "now"},
		{name: "minutes", then: now.Add(-40 * time.Minute), want: "40m"},
		{name: "hours", then: now.Add(-3 * time.Hour), want: "3h"},
		{name: "days", then: now.Add(-49 * time.Hour), want: "2d"},
		{name: "weeks", then: now.AddDate(0, 0, -14), want: "2w"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := RelativeAge(tc.then, now); got != tc.want {
				t.Fatalf("RelativeAge() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestActivityNoGitAndNoCommits(t *testing.T) {
	now := time.Now()
	if got := Activity(gitactivity.Info{}, config.Default(), now); got.Display != "—" {
		t.Fatalf("no git display = %q", got.Display)
	}
	if got := Activity(gitactivity.Info{HasGit: true}, config.Default(), now); got.Display != "no commits" {
		t.Fatalf("no commits display = %q", got.Display)
	}
}

func TestTagsBuildAutomaticAndManualStatusTags(t *testing.T) {
	now := time.Date(2026, 5, 6, 12, 0, 0, 0, time.Local)
	cfg := config.Default()
	cases := []struct {
		name   string
		info   ActivityInfo
		status string
		want   []string
	}{
		{name: "no git", info: ActivityInfo{}, status: "parked", want: []string{"no git"}},
		{name: "no commits", info: ActivityInfo{HasGit: true}, status: "parked", want: []string{"no commits", "parked"}},
		{name: "dirty stale manual", info: ActivityInfo{HasGit: true, HasCommits: true, Dirty: true, LastCommitAt: now.AddDate(0, 0, -60)}, status: "parked", want: []string{"dirty", "stale", "parked"}},
		{name: "dirty unpushed stale manual", info: ActivityInfo{HasGit: true, HasCommits: true, Dirty: true, Unpushed: 2, LastCommitAt: now.AddDate(0, 0, -60)}, status: "parked", want: []string{"dirty", "unpushed", "stale", "parked"}},
		{name: "unpushed active", info: ActivityInfo{HasGit: true, HasCommits: true, Unpushed: 2, LastCommitAt: now.AddDate(0, 0, -2)}, want: []string{"unpushed", "active"}},
		{name: "stale", info: ActivityInfo{HasGit: true, HasCommits: true, LastCommitAt: now.AddDate(0, 0, -60)}, want: []string{"stale"}},
		{name: "active dedupes manual active", info: ActivityInfo{HasGit: true, HasCommits: true, LastCommitAt: now.AddDate(0, 0, -2)}, status: "active", want: []string{"active"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := Tags(tc.info, tc.status, cfg, now)
			if len(got) != len(tc.want) {
				t.Fatalf("Tags() = %#v, want %#v", got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("Tags() = %#v, want %#v", got, tc.want)
				}
			}
		})
	}
}

func TestTagDisplay(t *testing.T) {
	if got := TagDisplay([]string{"dirty", "unpushed", "stale", "parked"}); got != "dirty · unpushed · stale · parked" {
		t.Fatalf("TagDisplay() = %q", got)
	}
}

func TestNoteFallbackChain(t *testing.T) {
	cfg := config.Default()
	info := gitactivity.Info{Branch: "feat/checkin", LastCommitMessage: "commit msg"}

	got := Note("manual note", "description", info, cfg)
	if got.Display != "feat/checkin · manual note" || got.Source != "manual" || got.Manual != "manual note" {
		t.Fatalf("manual note = %#v", got)
	}

	got = Note("", "description", info, cfg)
	if got.Display != "feat/checkin · commit msg" || got.Source != "commit" {
		t.Fatalf("commit note = %#v", got)
	}

	info.LastCommitMessage = ""
	got = Note("", "description", info, cfg)
	if got.Display != "feat/checkin · description" || got.Source != "description" {
		t.Fatalf("description note = %#v", got)
	}
}

func TestNoteHidesDefaultBranchAndDisabledFallbacks(t *testing.T) {
	cfg := config.Default()
	info := gitactivity.Info{Branch: "main", LastCommitMessage: "commit msg"}
	got := Note("", "description", info, cfg)
	if got.Display != "commit msg" {
		t.Fatalf("display = %q", got.Display)
	}

	cfg.NoteFallbackCommit = false
	cfg.NoteFallbackDescription = false
	got = Note("", "description", info, cfg)
	if got.Display != "" || got.Source != "none" {
		t.Fatalf("note = %#v", got)
	}
}
