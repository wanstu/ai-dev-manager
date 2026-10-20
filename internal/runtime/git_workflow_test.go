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
	runGitForCommitTest(t, git, root, "config", "core.autocrlf", "false")
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
	runGitForCommitTest(t, git, root, "config", "core.autocrlf", "false")
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

func TestControlledCherryPickContinueRechecksStagedAttributes(t *testing.T) {
	git, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git is not available")
	}
	root := t.TempDir()
	runGitForCommitTest(t, git, root, "init", "-b", "main")
	runGitForCommitTest(t, git, root, "config", "core.autocrlf", "false")
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
	runGitForCommitTest(t, git, root, "commit", "-am", "feature")
	featureCommit := runGitForCommitTest(t, git, root, "rev-parse", "HEAD")

	runGitForCommitTest(t, git, root, "checkout", "main")
	if err := os.WriteFile(filepath.Join(root, "tracked.txt"), []byte("main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGitForCommitTest(t, git, root, "commit", "-am", "main")

	rt, err := NewWithEnvironment(root, nil, os.Environ())
	if err != nil {
		t.Fatal(err)
	}
	result, err := rt.GitCherryPick(context.Background(), []string{featureCommit})
	if err != nil {
		t.Fatal(err)
	}
	if result.State != "conflict" || len(result.Conflicts) == 0 {
		t.Fatalf("expected cherry-pick conflict, got %+v", result)
	}

	if err := os.WriteFile(filepath.Join(root, "tracked.txt"), []byte("resolved\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".gitattributes"), []byte("*.txt filter=blocked\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGitForCommitTest(t, git, root, "add", "tracked.txt", ".gitattributes")

	if _, err := rt.GitCherryPickContinue(context.Background()); err == nil ||
		(!strings.Contains(err.Error(), ".gitattributes") && !strings.Contains(err.Error(), "may execute host code")) {
		t.Fatalf("expected staged attribute/filter safety refusal, got %v", err)
	}
	if !rt.gitPseudoRefExists(context.Background(), git, "CHERRY_PICK_HEAD") {
		t.Fatal("refused continuation unexpectedly ended cherry-pick")
	}
}

func TestControlledGitPushBranchAndExactTag(t *testing.T) {
	git, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git is not available")
	}
	root := t.TempDir()
	remote := filepath.Join(t.TempDir(), "remote.git")
	if err := os.MkdirAll(remote, 0o755); err != nil {
		t.Fatal(err)
	}
	runGitForCommitTest(t, git, remote, "init", "--bare")
	runGitForCommitTest(t, git, root, "init", "-b", "main")
	runGitForCommitTest(t, git, root, "config", "core.autocrlf", "false")
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

func TestControlledGitPushExactTagWithoutBranch(t *testing.T) {
	git, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git is not available")
	}
	root := t.TempDir()
	remote := filepath.Join(t.TempDir(), "remote.git")
	if err := os.MkdirAll(remote, 0o755); err != nil {
		t.Fatal(err)
	}
	runGitForCommitTest(t, git, remote, "init", "--bare")
	runGitForCommitTest(t, git, root, "init", "-b", "main")
	runGitForCommitTest(t, git, root, "config", "core.autocrlf", "false")
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
	if _, err := rt.GitTagCreate(context.Background(), "v-test-tag-only", "HEAD", "", false); err != nil {
		t.Fatal(err)
	}
	result, err := rt.GitPush(context.Background(), "origin", "", []string{"v-test-tag-only"}, false, true)
	if err != nil {
		t.Fatal(err)
	}
	if result.Branch != "" || len(result.Tags) != 1 || result.Tags[0] != "v-test-tag-only" {
		t.Fatalf("unexpected tags-only push result: %+v", result)
	}
	if got := runGitForCommitTest(t, git, remote, "rev-parse", "refs/tags/v-test-tag-only"); got == "" {
		t.Fatal("remote tag was not pushed")
	}
	branchCheck := exec.Command(git, "-C", remote, "show-ref", "--verify", "--quiet", "refs/heads/main")
	if err := branchCheck.Run(); err == nil {
		t.Fatal("tags-only push unexpectedly created remote main branch")
	}
}

func TestAgentSafeRemoteRejectsNonHTTPS(t *testing.T) {
	git, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git is not available")
	}
	root := t.TempDir()
	remote := filepath.Join(t.TempDir(), "remote.git")
	if err := os.MkdirAll(remote, 0o755); err != nil {
		t.Fatal(err)
	}
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



func TestGitRemoteListRedactsCredentialsAndLocalPaths(t *testing.T) {
	git, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git is not available")
	}
	root := t.TempDir()
	runGitForCommitTest(t, git, root, "init", "-b", "main")
	runGitForCommitTest(t, git, root, "remote", "add", "origin", "https://secret-token@example.com/org/repo.git?access_token=leak#fragment")
	runGitForCommitTest(t, git, root, "remote", "set-url", "--add", "--push", "origin", filepath.Join(t.TempDir(), "remote.git"))

	rt, err := NewWithEnvironment(root, nil, os.Environ())
	if err != nil {
		t.Fatal(err)
	}
	remotes, err := rt.GitRemoteList(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(remotes) != 1 {
		t.Fatalf("unexpected remotes: %+v", remotes)
	}
	got := strings.Join(append(remotes[0].FetchURLs, remotes[0].PushURLs...), "\n")
	for _, secret := range []string{"secret-token", "access_token", "leak"} {
		if strings.Contains(got, secret) {
			t.Fatalf("remote metadata leaked %q: %s", secret, got)
		}
	}
	if !strings.Contains(got, "https://example.com/org/repo.git") {
		t.Fatalf("sanitized HTTPS remote missing: %s", got)
	}
	if !strings.Contains(got, "<local-path>") {
		t.Fatalf("local remote path was not redacted: %s", got)
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
