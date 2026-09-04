package runtime

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"ai-dev-manager/internal/model"
)

func TestGitStatusDiffBranchWithRealRepository(t *testing.T) {
	root, gitPath := initGitRepository(t)
	runtime := mustNativeWithID(t, "ws_git_status", root, model.Policy{
		Mode:               string(ModeStandard),
		AllowedExecutables: []string{"git"},
		ToolPaths:          map[string]string{"git": gitPath},
	}, nil)

	if _, err := runtime.Edit("source.txt", "hello", "hello changed", 1); err != nil {
		t.Fatalf("Edit() error = %v", err)
	}
	status, err := runtime.GitStatus()
	if err != nil {
		t.Fatalf("GitStatus() error = %v", err)
	}
	if len(status) != 1 || status[0].Path != "source.txt" || (status[0].X != "M" && status[0].Y != "M") {
		t.Fatalf("unexpected status: %+v", status)
	}

	diff, err := runtime.GitDiff()
	if err != nil {
		t.Fatalf("GitDiff() error = %v", err)
	}
	if len(diff.Files) != 1 || diff.Files[0] != "source.txt" || !strings.Contains(diff.Patch, "hello changed") {
		t.Fatalf("unexpected diff: files=%+v patch=%q", diff.Files, diff.Patch)
	}

	branch, err := runtime.GitBranch()
	if err != nil {
		t.Fatalf("GitBranch() error = %v", err)
	}
	if branch != "main" {
		t.Fatalf("branch = %q, want main", branch)
	}
}

func TestGitAPIDeniedWhenGitNotAllowed(t *testing.T) {
	root, _ := initGitRepository(t)
	runtime := mustNativeWithID(t, "ws_git_denied", root, model.Policy{
		Mode:               string(ModeStandard),
		AllowedExecutables: []string{"other"},
	}, nil)
	_, err := runtime.GitStatus()
	assertRuntimeErrorKind(t, err, ErrToolNotAllowed)
}

func TestManagedGitWorktreeCreateListRemoveDoesNotSwitchMainCheckout(t *testing.T) {
	root, gitPath := initGitRepository(t)
	runtime := mustNativeWithID(t, "ws_git_worktree", root, model.Policy{
		Mode:               string(ModeStandard),
		AllowedExecutables: []string{"git"},
		ToolPaths:          map[string]string{"git": gitPath},
	}, nil)
	beforeHead := gitTestOutput(t, gitPath, root, "rev-parse", "HEAD")
	beforeBranch := gitTestOutput(t, gitPath, root, "branch", "--show-current")

	created, err := runtime.GitWorktreeCreate("feature-one", "feature-one")
	if err != nil {
		t.Fatalf("GitWorktreeCreate() error = %v", err)
	}
	managedRoot, err := runtime.managedWorktreeRoot()
	if err != nil {
		t.Fatalf("managedWorktreeRoot() error = %v", err)
	}
	wantTarget := filepath.Join(managedRoot, "feature-one")
	if !samePath(created.Path, wantTarget) {
		t.Fatalf("created worktree path = %q, want %q", created.Path, wantTarget)
	}
	if _, err := os.Stat(filepath.Join(wantTarget, "source.txt")); err != nil {
		t.Fatalf("managed worktree missing checkout: %v", err)
	}

	worktrees, err := runtime.GitWorktrees()
	if err != nil {
		t.Fatalf("GitWorktrees() error = %v", err)
	}
	found := false
	for _, worktree := range worktrees {
		if samePath(worktree.Path, wantTarget) {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("created worktree missing from list: %+v", worktrees)
	}

	afterHead := gitTestOutput(t, gitPath, root, "rev-parse", "HEAD")
	afterBranch := gitTestOutput(t, gitPath, root, "branch", "--show-current")
	if afterHead != beforeHead || afterBranch != beforeBranch {
		t.Fatalf("main checkout switched: head %q -> %q, branch %q -> %q", beforeHead, afterHead, beforeBranch, afterBranch)
	}

	if err := runtime.GitWorktreeRemove("feature-one"); err != nil {
		t.Fatalf("GitWorktreeRemove() error = %v", err)
	}
	if _, err := os.Stat(wantTarget); !os.IsNotExist(err) {
		t.Fatalf("managed target still exists after remove, stat error = %v", err)
	}

	if _, err := runtime.GitWorktreeCreate("../escape", "bad"); err == nil {
		t.Fatal("unsafe worktree name unexpectedly accepted")
	} else {
		assertRuntimeErrorKind(t, err, ErrInvalidPath)
	}
}

func initGitRepository(t *testing.T) (string, string) {
	t.Helper()
	gitPath, err := exec.LookPath("git")
	if err != nil {
		t.Fatalf("git executable not available for Phase 5 acceptance: %v", err)
	}
	gitPath, err = filepath.Abs(gitPath)
	if err != nil {
		t.Fatalf("filepath.Abs(git) error = %v", err)
	}
	root := t.TempDir()
	gitTestRun(t, gitPath, root, "init")
	gitTestRun(t, gitPath, root, "config", "user.email", "adm-test@example.invalid")
	gitTestRun(t, gitPath, root, "config", "user.name", "ADM Test")
	if err := os.WriteFile(filepath.Join(root, "source.txt"), []byte("hello\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(source.txt) error = %v", err)
	}
	gitTestRun(t, gitPath, root, "add", "source.txt")
	gitTestRun(t, gitPath, root, "commit", "-m", "baseline")
	gitTestRun(t, gitPath, root, "branch", "-M", "main")
	return root, filepath.Clean(gitPath)
}

func gitTestRun(t *testing.T, gitPath, cwd string, args ...string) {
	t.Helper()
	cmd := exec.Command(gitPath, args...)
	cmd.Dir = cwd
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s failed: %v\n%s", strings.Join(args, " "), err, output)
	}
}

func gitTestOutput(t *testing.T, gitPath, cwd string, args ...string) string {
	t.Helper()
	cmd := exec.Command(gitPath, args...)
	cmd.Dir = cwd
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s failed: %v\n%s", strings.Join(args, " "), err, output)
	}
	return strings.TrimSpace(string(output))
}

func mustNativeWithID(t *testing.T, id, root string, policy model.Policy, verifiers map[string]model.ResolvedVerifier) *Native {
	t.Helper()
	runtime, err := NewNative(model.Workspace{ID: id, Path: root}, model.EffectiveConfig{
		Policy:    &model.ResolvedPolicy{Policy: policy, Source: model.ScopeProject},
		Verifiers: verifiers,
	})
	if err != nil {
		t.Fatalf("NewNative() error = %v", err)
	}
	return runtime
}
