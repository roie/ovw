package gitactivity

import (
	"os"
	"os/exec"
	"path/filepath"
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

func gitGlobal(t *testing.T, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, string(out))
	}
}
