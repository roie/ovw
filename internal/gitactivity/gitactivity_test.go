package gitactivity

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestDetectNoGit(t *testing.T) {
	info := Detect(t.TempDir())
	if info.HasGit {
		t.Fatalf("HasGit = true: %#v", info)
	}
}

func TestDetectNoCommits(t *testing.T) {
	dir := gitRepo(t)
	info := Detect(dir)
	if !info.HasGit {
		t.Fatalf("HasGit = false: %#v", info)
	}
	if info.HasCommits {
		t.Fatalf("HasCommits = true: %#v", info)
	}
}

func TestDetectCommitBranchAndDirty(t *testing.T) {
	dir := gitRepo(t)
	git(t, dir, "checkout", "-b", "feat/test")
	if err := os.WriteFile(filepath.Join(dir, "file.txt"), []byte("one"), 0o644); err != nil {
		t.Fatal(err)
	}
	git(t, dir, "add", "file.txt")
	git(t, dir, "commit", "-m", "initial commit", "-m", "- Add setup\n- Add tests")
	if err := os.WriteFile(filepath.Join(dir, "file.txt"), []byte("two"), 0o644); err != nil {
		t.Fatal(err)
	}

	info := Detect(dir)
	if !info.HasGit || !info.HasCommits {
		t.Fatalf("git flags = %#v", info)
	}
	if info.Branch != "feat/test" {
		t.Fatalf("Branch = %q", info.Branch)
	}
	wantMessage := "initial commit\n\n- Add setup\n- Add tests"
	if info.LastCommitMessage != wantMessage {
		t.Fatalf("LastCommitMessage = %q", info.LastCommitMessage)
	}
	if info.LastCommitAt.IsZero() {
		t.Fatal("LastCommitAt is zero")
	}
	if !info.Dirty {
		t.Fatal("Dirty = false")
	}
}

func TestDetectDirtyIgnoresUntrackedFiles(t *testing.T) {
	dir := gitRepo(t)
	if err := os.WriteFile(filepath.Join(dir, "tracked.txt"), []byte("one"), 0o644); err != nil {
		t.Fatal(err)
	}
	git(t, dir, "add", "tracked.txt")
	git(t, dir, "commit", "-m", "initial commit")
	if err := os.WriteFile(filepath.Join(dir, "untracked.txt"), []byte("two"), 0o644); err != nil {
		t.Fatal(err)
	}

	info := Detect(dir)
	if info.Dirty {
		t.Fatalf("Dirty = true for untracked-only repo: %#v", info)
	}
}

func TestDetectDetachedHeadBranch(t *testing.T) {
	dir := gitRepo(t)
	if err := os.WriteFile(filepath.Join(dir, "file.txt"), []byte("one"), 0o644); err != nil {
		t.Fatal(err)
	}
	git(t, dir, "add", "file.txt")
	git(t, dir, "commit", "-m", "initial commit")
	hash := strings.TrimSpace(gitOutput(t, dir, "rev-parse", "HEAD"))
	git(t, dir, "checkout", hash)

	info := Detect(dir)
	if info.Branch != "detached" {
		t.Fatalf("Branch = %q, want detached", info.Branch)
	}
}

func TestDetectUnpushed(t *testing.T) {
	remote := gitRepo(t)
	if err := os.WriteFile(filepath.Join(remote, "base.txt"), []byte("base"), 0o644); err != nil {
		t.Fatal(err)
	}
	git(t, remote, "add", "base.txt")
	git(t, remote, "commit", "-m", "base commit")
	git(t, remote, "config", "receive.denyCurrentBranch", "updateInstead")
	work := filepath.Join(t.TempDir(), "work")
	gitGlobal(t, "clone", remote, work)
	git(t, work, "config", "user.email", "test@example.com")
	git(t, work, "config", "user.name", "Test User")
	if err := os.WriteFile(filepath.Join(work, "file.txt"), []byte("one"), 0o644); err != nil {
		t.Fatal(err)
	}
	git(t, work, "add", "file.txt")
	git(t, work, "commit", "-m", "ahead commit")

	info := Detect(work)
	if info.Unpushed != 1 {
		t.Fatalf("Unpushed = %d info=%#v", info.Unpushed, info)
	}
}

func TestDetectUsesFastDirtyAndCombinedLogCommands(t *testing.T) {
	calls := []string{}
	detector := newDetectorWithRunner(func(path string, args ...string) (string, error) {
		command := strings.Join(args, " ")
		calls = append(calls, command)
		switch command {
		case "rev-parse --show-toplevel":
			return "/repo\n", nil
		case "rev-parse --abbrev-ref HEAD":
			return "main\n", nil
		case "log -1 --format=%ct%x00%B":
			return "1762000000\x00initial commit\n\nbody\n", nil
		case "status --porcelain --untracked-files=no":
			return " M file.txt\n", nil
		case "rev-list --count @{upstream}..HEAD":
			return "2\n", nil
		default:
			t.Fatalf("unexpected git command for %s: %v", path, args)
			return "", nil
		}
	}, time.Second)

	info := detector.Detect("/repo")
	if !info.HasGit || !info.HasCommits {
		t.Fatalf("git info not detected: %#v", info)
	}
	if info.Branch != "main" {
		t.Fatalf("Branch = %q", info.Branch)
	}
	if info.Unpushed != 2 {
		t.Fatalf("Unpushed = %d", info.Unpushed)
	}
	if !info.Dirty {
		t.Fatal("Dirty = false")
	}
	if info.LastCommitMessage != "initial commit\n\nbody" {
		t.Fatalf("LastCommitMessage = %q", info.LastCommitMessage)
	}
	wantCalls := []string{
		"rev-parse --show-toplevel",
		"rev-parse --abbrev-ref HEAD",
		"log -1 --format=%ct%x00%B",
		"status --porcelain --untracked-files=no",
		"rev-list --count @{upstream}..HEAD",
	}
	if strings.Join(calls, "|") != strings.Join(wantCalls, "|") {
		t.Fatalf("calls = %#v, want %#v", calls, wantCalls)
	}
}

func TestDetectRecentCommitsCapsAtThree(t *testing.T) {
	dir := gitRepo(t)
	for _, subject := range []string{"first", "second", "third", "fourth"} {
		if err := os.WriteFile(filepath.Join(dir, subject+".txt"), []byte(subject), 0o644); err != nil {
			t.Fatal(err)
		}
		git(t, dir, "add", subject+".txt")
		git(t, dir, "commit", "-m", subject)
	}

	commits, err := Recent(dir, 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(commits) != 3 {
		t.Fatalf("Recent len = %d commits=%#v", len(commits), commits)
	}
	for index, want := range []string{"fourth", "third", "second"} {
		got := commits[index]
		if got.Subject != want {
			t.Fatalf("RecentCommits[%d].Subject = %q, want %q", index, got.Subject, want)
		}
		if got.Hash == "" {
			t.Fatalf("RecentCommits[%d].Hash is empty", index)
		}
		if got.At.IsZero() {
			t.Fatalf("RecentCommits[%d].At is zero", index)
		}
	}
}

func TestDetectorReusesGitInfoForNestedPathsInSameWorktree(t *testing.T) {
	calls := 0
	detector := newDetectorWithRunner(func(path string, args ...string) (string, error) {
		calls++
		command := strings.Join(args, " ")
		switch command {
		case "rev-parse --show-toplevel":
			if strings.HasPrefix(path, "/repo") {
				return "/repo\n", nil
			}
			return "", errors.New("not a git repo")
		case "rev-parse --abbrev-ref HEAD":
			return "main\n", nil
		case "log -1 --format=%ct%x00%B":
			return "1762000000\x00initial commit\n", nil
		case "status --porcelain --untracked-files=no":
			return "", nil
		case "rev-list --count @{upstream}..HEAD":
			return "0\n", nil
		default:
			t.Fatalf("unexpected git command for %s: %v", path, args)
			return "", nil
		}
	}, time.Second)

	first := detector.Detect("/repo/node_modules/a")
	firstCalls := calls
	second := detector.Detect("/repo/node_modules/b")

	if !first.HasGit || !second.HasGit {
		t.Fatalf("git info not detected: first=%#v second=%#v", first, second)
	}
	if firstCalls == 0 {
		t.Fatal("expected first detection to call git")
	}
	if calls != firstCalls {
		t.Fatalf("second nested path called git again: first calls=%d total=%d", firstCalls, calls)
	}
}

func TestDetectorReusesInFlightGitInfoForNestedPathsInSameWorktree(t *testing.T) {
	var mu sync.Mutex
	branchCalls := 0
	detector := newDetectorWithRunner(func(path string, args ...string) (string, error) {
		command := strings.Join(args, " ")
		switch command {
		case "rev-parse --show-toplevel":
			if strings.HasPrefix(path, "/repo") {
				return "/repo\n", nil
			}
			return "", errors.New("not a git repo")
		case "rev-parse --abbrev-ref HEAD":
			mu.Lock()
			branchCalls++
			mu.Unlock()
			time.Sleep(10 * time.Millisecond)
			return "main\n", nil
		case "log -1 --format=%ct%x00%B":
			return "1762000000\x00initial commit\n", nil
		case "status --porcelain --untracked-files=no":
			return "", nil
		case "rev-list --count @{upstream}..HEAD":
			return "0\n", nil
		default:
			t.Fatalf("unexpected git command for %s: %v", path, args)
			return "", nil
		}
	}, time.Second)

	var wg sync.WaitGroup
	for _, path := range []string{"/repo/node_modules/a", "/repo/node_modules/b"} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			info := detector.Detect(path)
			if !info.HasGit {
				t.Errorf("git info not detected for %s: %#v", path, info)
			}
		}()
	}
	wg.Wait()

	if branchCalls != 1 {
		t.Fatalf("branch calls = %d, want 1", branchCalls)
	}
}

func TestDetectorDoesNotReuseParentGitInfoForNestedGitProject(t *testing.T) {
	parent := filepath.Join(t.TempDir(), "repo")
	child := filepath.Join(parent, "nested")
	if err := os.MkdirAll(filepath.Join(parent, "pkg"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(parent, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(child, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}

	detector := newDetectorWithRunner(func(path string, args ...string) (string, error) {
		command := strings.Join(args, " ")
		if command == "rev-parse --show-toplevel" {
			if strings.HasPrefix(path, child) {
				return child + "\n", nil
			}
			return parent + "\n", nil
		}
		if command == "rev-parse --abbrev-ref HEAD" {
			if path == child {
				return "nested\n", nil
			}
			return "parent\n", nil
		}
		switch command {
		case "log -1 --format=%ct%x00%B":
			return "1762000000\x00initial commit\n", nil
		case "status --porcelain --untracked-files=no":
			return "", nil
		case "rev-list --count @{upstream}..HEAD":
			return "0\n", nil
		default:
			t.Fatalf("unexpected git command for %s: %v", path, args)
			return "", nil
		}
	}, time.Second)

	parentInfo := detector.Detect(filepath.Join(parent, "pkg"))
	childInfo := detector.Detect(child)

	if parentInfo.Branch != "parent" {
		t.Fatalf("parent branch = %q", parentInfo.Branch)
	}
	if childInfo.Branch != "nested" {
		t.Fatalf("child branch = %q, want nested", childInfo.Branch)
	}
}

func gitRepo(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	dir := t.TempDir()
	gitGlobal(t, "init", dir)
	git(t, dir, "config", "user.email", "test@example.com")
	git(t, dir, "config", "user.name", "Test User")
	return dir
}

func git(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, string(out))
	}
}

func gitOutput(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, string(out))
	}
	return string(out)
}

func gitGlobal(t *testing.T, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, string(out))
	}
}
