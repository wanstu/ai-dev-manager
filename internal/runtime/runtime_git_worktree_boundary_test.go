package runtime

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestControlledGitPinsEnvironmentWorkTreeAndIgnoresExternalExcludeFile(t *testing.T) {
	git, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git is not available")
	}
	root := t.TempDir()
	outside := t.TempDir()
	runGitForCommitTest(t, git, root, "init", "-b", "main")
	runGitForCommitTest(t, git, root, "config", "user.name", "ADM Test")
	runGitForCommitTest(t, git, root, "config", "user.email", "adm-test@example.invalid")
	if err := os.WriteFile(filepath.Join(root, "tracked.txt"), []byte("base\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGitForCommitTest(t, git, root, "add", "tracked.txt")
	runGitForCommitTest(t, git, root, "commit", "-m", "base")

	excludes := filepath.Join(outside, "outside-excludes")
	if err := os.WriteFile(excludes, []byte("ignored.tmp\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGitForCommitTest(t, git, root, "config", "core.worktree", outside)
	runGitForCommitTest(t, git, root, "config", "core.excludesFile", excludes)

	if err := os.WriteFile(filepath.Join(root, "tracked.txt"), []byte("root changed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "ignored.tmp"), []byte("must remain visible\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(outside, "tracked.txt"), []byte("outside changed\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	rt, err := NewWithEnvironment(root, nil, os.Environ())
	if err != nil {
		t.Fatal(err)
	}
	status, err := rt.GitStatus(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, item := range status {
		seen[item.Path] = true
	}
	if !seen["tracked.txt"] || !seen["ignored.tmp"] {
		t.Fatalf("GitStatus followed repository core.worktree/core.excludesFile instead of Environment root: %+v", status)
	}

	diff, err := rt.GitDiff(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(diff, "root changed") || strings.Contains(diff, "outside changed") {
		t.Fatalf("GitDiff escaped Environment worktree: %s", diff)
	}

	result, err := rt.GitCommit(context.Background(), "test: pinned worktree")
	if err != nil {
		t.Fatal(err)
	}
	if result.Commit == "" {
		t.Fatal("controlled commit returned empty commit id")
	}
	cmd := exec.Command(git, "--git-dir="+filepath.Join(root, ".git"), "show", "HEAD:tracked.txt")
	output, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(output)) != "root changed" {
		t.Fatalf("commit captured wrong worktree content: %q", output)
	}
}
