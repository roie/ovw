package gitactivity

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
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
