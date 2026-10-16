package runtime

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestGitCommitControlledDisablesHooksAndReturnsCommit(t *testing.T) {
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

	hooks := filepath.Join(root, ".git", "hooks")
	if err := os.MkdirAll(hooks, 0o755); err != nil {
		t.Fatal(err)
	}
	hook := filepath.Join(hooks, "pre-commit")
	if err := os.WriteFile(hook, []byte("#!/bin/sh\nexit 97\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "tracked.txt"), []byte("changed\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	rt, err := NewWithEnvironment(root, nil, os.Environ())
	if err != nil {
		t.Fatal(err)
	}
	result, err := rt.GitCommit(context.Background(), "test: controlled commit")
	if err != nil {
		t.Fatalf("controlled commit unexpectedly ran hook or failed: %v", err)
	}
	if result.Commit == "" || result.Branch != "main" {
		t.Fatalf("unexpected commit result: %+v", result)
	}
	if got := runGitForCommitTest(t, git, root, "show", "-s", "--format=%s", "HEAD"); got != "test: controlled commit" {
		t.Fatalf("commit subject = %q", got)
	}
}

func TestGitCommitControlledUsesEffectiveGlobalIdentityWithoutLoadingGlobalConfigIntoCommit(t *testing.T) {
	git, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git is not available")
	}
	root := t.TempDir()
	runGitForCommitTest(t, git, root, "init", "-b", "main")
	if err := os.WriteFile(filepath.Join(root, "tracked.txt"), []byte("base\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGitForCommitTest(t, git, root, "add", "tracked.txt")
	runGitForCommitTest(t, git, root, "-c", "user.name=Bootstrap", "-c", "user.email=bootstrap@example.invalid", "commit", "-m", "base")

	home := t.TempDir()
	globalConfig := "[user]\n\tname = Safe Global User\n\temail = safe-global@example.invalid\n[core]\n\thooksPath = definitely-must-not-be-used\n"
	if err := os.WriteFile(filepath.Join(home, ".gitconfig"), []byte(globalConfig), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "tracked.txt"), []byte("changed\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	environment := append([]string(nil), os.Environ()...)
	environment = replaceEnvironmentValue(environment, "HOME", home)
	environment = replaceEnvironmentValue(environment, "USERPROFILE", home)
	rt, err := NewWithEnvironment(root, nil, environment)
	if err != nil {
		t.Fatal(err)
	}
	result, err := rt.GitCommit(context.Background(), "test: global identity")
	if err != nil {
		t.Fatal(err)
	}
	if result.Commit == "" {
		t.Fatal("controlled commit returned empty commit id")
	}
	if got := runGitForCommitTest(t, git, root, "show", "-s", "--format=%an|%ae", "HEAD"); got != "Safe Global User|safe-global@example.invalid" {
		t.Fatalf("commit identity = %q", got)
	}
}

func TestGitCommitControlledRefusesGitFilters(t *testing.T) {
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
	if err := os.WriteFile(filepath.Join(root, ".gitattributes"), []byte("*.txt filter=blocked\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "tracked.txt"), []byte("changed\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	rt, err := NewWithEnvironment(root, nil, os.Environ())
	if err != nil {
		t.Fatal(err)
	}
	_, err = rt.GitCommit(context.Background(), "must refuse filter")
	if err == nil || !strings.Contains(err.Error(), "may execute host code") {
		t.Fatalf("expected Git filter refusal, got %v", err)
	}
	if got := runGitForCommitTest(t, git, root, "show", "-s", "--format=%s", "HEAD"); got != "base" {
		t.Fatalf("refused commit changed HEAD: %q", got)
	}
}

func TestReadOnlyGitIgnoresInheritedGitDir(t *testing.T) {
	git, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git is not available")
	}
	root := t.TempDir()
	other := t.TempDir()
	for _, dir := range []string{root, other} {
		runGitForCommitTest(t, git, dir, "init", "-b", "main")
		runGitForCommitTest(t, git, dir, "config", "user.name", "ADM Test")
		runGitForCommitTest(t, git, dir, "config", "user.email", "adm-test@example.invalid")
		if err := os.WriteFile(filepath.Join(dir, "tracked.txt"), []byte("base\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		runGitForCommitTest(t, git, dir, "add", "tracked.txt")
		runGitForCommitTest(t, git, dir, "commit", "-m", "base")
	}
	if err := os.WriteFile(filepath.Join(root, "tracked.txt"), []byte("root changed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	environment := append(os.Environ(), "GIT_DIR="+filepath.Join(other, ".git"))
	rt, err := NewWithEnvironment(root, nil, environment)
	if err != nil {
		t.Fatal(err)
	}
	status, err := rt.GitStatus(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(status) != 1 || status[0].Path != "tracked.txt" {
		t.Fatalf("GitStatus followed inherited GIT_DIR outside Environment: %+v", status)
	}
}

func runGitForCommitTest(t *testing.T, git, dir string, args ...string) string {
	t.Helper()
	full := append([]string{"-C", dir}, args...)
	cmd := exec.Command(git, full...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, output)
	}
	return strings.TrimSpace(string(output))
}
