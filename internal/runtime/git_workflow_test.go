package runtime

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestControlledGitWorkflowCherryPickTagLogAndShow(t *testing.T) {
	git, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git is not available")
	}
	root := t.TempDir()
	runGitForCommitTest(t, git, root, "init", "-b", "main")
	runGitForCommitTest(t, git, root, "config", "user.name", "ADM Test")
	runGitForCommitTest(t, git, root, "config", "user.email", "adm-test@example.invalid")
	if err := os.WriteFile(filepath.Join(root, "tracked.txt"), []byte("base\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGitForCommitTest(t, git, root, "add", "tracked.txt")
	runGitForCommitTest(t, git, root, "commit", "-m", "base")

	runGitForCommitTest(t, git, root, "checkout", "-b", "feature")
	if err := os.WriteFile(filepath.Join(root, "tracked.txt"), []byte("feature\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGitForCommitTest(t, git, root, "commit", "-am", "feature change")
	featureCommit := runGitForCommitTest(t, git, root, "rev-parse", "HEAD")
	runGitForCommitTest(t, git, root, "checkout", "main")

	rt, err := NewWithEnvironment(root, nil, os.Environ())
	if err != nil {
		t.Fatal(err)
	}
	result, err := rt.GitCherryPick(context.Background(), []string{featureCommit})
	if err != nil {
		t.Fatal(err)
	}
	if result.State != "completed" || len(result.Conflicts) != 0 {
		t.Fatalf("unexpected cherry-pick result: %+v", result)
	}
	if got := strings.TrimSpace(string(mustReadFile(t, filepath.Join(root, "tracked.txt")))); got != "feature" {
		t.Fatalf("cherry-pick content = %q", got)
	}

	tag, err := rt.GitTagCreate(context.Background(), "v-test", "HEAD", "test release", true)
	if err != nil {
		t.Fatal(err)
	}
	if tag.Name != "v-test" || tag.Commit == "" {
		t.Fatalf("unexpected tag result: %+v", tag)
	}
	tags, err := rt.GitTagList(context.Background(), 20)
	if err != nil {
		t.Fatal(err)
	}
	if len(tags) != 1 || tags[0].Name != "v-test" {
		t.Fatalf("unexpected tag list: %+v", tags)
	}

	log, err := rt.GitLog(context.Background(), "HEAD", 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(log) < 2 || log[0].Subject != "feature change" {
		t.Fatalf("unexpected log: %+v", log)
	}
	show, err := rt.GitShow(context.Background(), "HEAD", 128*1024)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(show, "feature change") || !strings.Contains(show, "feature") {
		t.Fatalf("git show missing expected content:\n%s", show)
	}
}

func TestControlledCherryPickRefusesIncomingGitAttributes(t *testing.T) {
	git, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git is not available")
	}
	root := t.TempDir()
	runGitForCommitTest(t, git, root, "init", "-b", "main")
	runGitForCommitTest(t, git, root, "config", "user.name", "ADM Test")
	runGitForCommitTest(t, git, root, "config", "user.email", "adm-test@example.invalid")
	if err := os.WriteFile(filepath.Join(root, "tracked.txt"), []byte("base\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGitForCommitTest(t, git, root, "add", "tracked.txt")
	runGitForCommitTest(t, git, root, "commit", "-m", "base")

	runGitForCommitTest(t, git, root, "checkout", "-b", "unsafe")
	if err := os.WriteFile(filepath.Join(root, ".gitattributes"), []byte("*.txt filter=evil\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGitForCommitTest(t, git, root, "add", ".gitattributes")
	runGitForCommitTest(t, git, root, "commit", "-m", "unsafe attributes")
	unsafeCommit := runGitForCommitTest(t, git, root, "rev-parse", "HEAD")
	runGitForCommitTest(t, git, root, "checkout", "main")

	rt, err := NewWithEnvironment(root, nil, os.Environ())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := rt.GitCherryPick(context.Background(), []string{unsafeCommit}); err == nil || !strings.Contains(err.Error(), ".gitattributes") {
		t.Fatalf("expected .gitattributes refusal, got %v", err)
	}
	if got := runGitForCommitTest(t, git, root, "show", "-s", "--format=%s", "HEAD"); got != "base" {
		t.Fatalf("refused cherry-pick changed HEAD: %q", got)
	}
}

func TestControlledGitPushBranchAndExactTag(t *testing.T) {
	git, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git is not available")
	}
	root := t.TempDir()
	remote := filepath.Join(t.TempDir(), "remote.git")
	runGitForCommitTest(t, git, remote, "init", "--bare")
	runGitForCommitTest(t, git, root, "init", "-b", "main")
	runGitForCommitTest(t, git, root, "config", "user.name", "ADM Test")
	runGitForCommitTest(t, git, root, "config", "user.email", "adm-test@example.invalid")
	if err := os.WriteFile(filepath.Join(root, "tracked.txt"), []byte("base\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGitForCommitTest(t, git, root, "add", "tracked.txt")
	runGitForCommitTest(t, git, root, "commit", "-m", "base")
	runGitForCommitTest(t, git, root, "remote", "add", "origin", remote)

	rt, err := NewWithEnvironment(root, nil, os.Environ())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := rt.GitTagCreate(context.Background(), "v-test", "HEAD", "", false); err != nil {
		t.Fatal(err)
	}
	result, err := rt.GitPush(context.Background(), "origin", "main", []string{"v-test"}, true, true)
	if err != nil {
		t.Fatal(err)
	}
	if result.Branch != "main" || len(result.Tags) != 1 || result.Tags[0] != "v-test" {
		t.Fatalf("unexpected push result: %+v", result)
	}
	if got := runGitForCommitTest(t, git, remote, "rev-parse", "refs/heads/main"); got == "" {
		t.Fatal("remote main was not pushed")
	}
	if got := runGitForCommitTest(t, git, remote, "rev-parse", "refs/tags/v-test"); got == "" {
		t.Fatal("remote tag was not pushed")
	}
}

func TestAgentSafeRemoteRejectsNonHTTPS(t *testing.T) {
	git, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git is not available")
	}
	root := t.TempDir()
	remote := filepath.Join(t.TempDir(), "remote.git")
	runGitForCommitTest(t, git, remote, "init", "--bare")
	runGitForCommitTest(t, git, root, "init", "-b", "main")
	runGitForCommitTest(t, git, root, "remote", "add", "origin", remote)

	rt, err := NewWithEnvironment(root, nil, os.Environ())
	if err != nil {
		t.Fatal(err)
	}
	_, err = rt.GitFetch(context.Background(), "origin", false, false, false)
	if err == nil || !strings.Contains(err.Error(), "requires HTTPS") {
		t.Fatalf("expected Agent-safe remote refusal, got %v", err)
	}
}

func mustReadFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}
