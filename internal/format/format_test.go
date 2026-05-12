package format

import (
	"testing"
	"time"

	"ovw/internal/config"
	"ovw/internal/gitactivity"
	"ovw/internal/recentfiles"
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

func TestActivityFormatsRecentCommits(t *testing.T) {
	now := time.Date(2026, 5, 6, 12, 0, 0, 0, time.Local)
	commits := []gitactivity.CommitInfo{
		{Hash: "abc1234", Subject: "fix modal", At: now.Add(-10 * time.Minute)},
		{Hash: "def5678", Subject: "add sidepane", At: now.Add(-2 * time.Hour)},
	}

	got := RecentCommits(commits, now)
	if len(got) != 2 {
		t.Fatalf("RecentCommits len = %d", len(got))
	}
	if got[0].Hash != "abc1234" || got[0].Subject != "fix modal" || got[0].Age != "10m" {
		t.Fatalf("RecentCommits[0] = %#v", got[0])
	}
	if got[1].Age != "2h" {
		t.Fatalf("RecentCommits[1] = %#v", got[1])
	}
}

func TestRecentFilesFormatsModifiedAge(t *testing.T) {
	now := time.Date(2026, 5, 12, 12, 0, 0, 0, time.Local)
	files := []recentfiles.File{
		{Path: "internal/tui/app.go", ModifiedAt: now.Add(-4 * time.Minute)},
	}

	got := RecentFiles(files, now)
	if len(got) != 1 || got[0].Path != "internal/tui/app.go" || got[0].Age != "4m" {
		t.Fatalf("RecentFiles() = %#v", got)
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
		{name: "no git", info: ActivityInfo{}, want: []string{}},
		{name: "no git user", info: ActivityInfo{}, status: "parked", want: []string{"parked"}},
		{name: "no commits user", info: ActivityInfo{HasGit: true}, status: "parked", want: []string{"parked"}},
		{name: "dirty stale user", info: ActivityInfo{HasGit: true, HasCommits: true, Dirty: true, LastCommitAt: now.AddDate(0, 0, -60)}, status: "parked", want: []string{"parked"}},
		{name: "dirty unpushed stale user", info: ActivityInfo{HasGit: true, HasCommits: true, Dirty: true, Unpushed: 2, LastCommitAt: now.AddDate(0, 0, -60)}, status: "parked", want: []string{"parked"}},
		{name: "dirty recent", info: ActivityInfo{HasGit: true, HasCommits: true, Dirty: true, LastCommitAt: now.AddDate(0, 0, -2)}, want: []string{"dirty"}},
		{name: "unpushed recent", info: ActivityInfo{HasGit: true, HasCommits: true, Unpushed: 2, LastCommitAt: now.AddDate(0, 0, -2)}, want: []string{"active"}},
		{name: "dirty unpushed recent", info: ActivityInfo{HasGit: true, HasCommits: true, Dirty: true, Unpushed: 2, LastCommitAt: now.AddDate(0, 0, -2)}, want: []string{"dirty"}},
		{name: "stale", info: ActivityInfo{HasGit: true, HasCommits: true, LastCommitAt: now.AddDate(0, 0, -60)}, want: []string{"stale"}},
		{name: "active dedupes user active", info: ActivityInfo{HasGit: true, HasCommits: true, LastCommitAt: now.AddDate(0, 0, -2)}, status: "active", want: []string{"active"}},
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

func TestUserStatusDisplayDoesNotIncludeGeneratedTags(t *testing.T) {
	now := time.Date(2026, 5, 6, 12, 0, 0, 0, time.Local)
	activity := ActivityInfo{
		HasGit:       true,
		HasCommits:   true,
		Dirty:        true,
		Unpushed:     9,
		LastCommitAt: now.AddDate(0, 0, -60),
	}

	got := Status(activity, "blocked", config.Default(), now)
	if got.Display != "blocked" || got.Value != "blocked" {
		t.Fatalf("Status() = %#v, want clean user status", got)
	}
	if len(got.Tags) != 1 || got.Tags[0] != "blocked" {
		t.Fatalf("Status tags = %#v, want blocked only", got.Tags)
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

	got := Note("user note", "description", info, cfg)
	if got.Display != "user note" || got.Source != "user" || got.Value != "user note" {
		t.Fatalf("user note = %#v", got)
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

func TestSingleLineCollapsesWhitespace(t *testing.T) {
	got := SingleLine("fix: extract carousel\n- Add SJS script\n\n- Add fallback")
	want := "fix: extract carousel - Add SJS script - Add fallback"
	if got != want {
		t.Fatalf("SingleLine() = %q, want %q", got, want)
	}
}
