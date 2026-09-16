package app

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"ai-dev-manager-v2/internal/model"
)

func TestManagedWorktreeTamperBlocksRoutedMutationBeforeFileChange(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not available")
	}
	source := appTestGitRepo(t)
	service := New(filepath.Join(t.TempDir(), "adm", "state.json"))
	ws, err := service.Workspaces.Add(source, "source")
	if err != nil {
		t.Fatal(err)
	}
	created, err := service.CreateManagedWorktree(context.Background(), ws.ID, "managed", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	root := created.ManagedWorktree.Root
	defer func() {
		_, _ = appGitCommand(source, "worktree", "remove", "--force", root)
	}()
	if _, err := service.Environments.AcquireWriter(created.Environment.ID, "writer"); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Write(created.Environment.ID, "writer", "before.txt", "ok\n", false); err != nil {
		t.Fatalf("valid managed mutation failed: %v", err)
	}
	appGitRun(t, root, "checkout", "-b", "tampered-branch")
	if _, err := service.Write(created.Environment.ID, "writer", "blocked.txt", "must-not-write\n", false); err == nil || !strings.Contains(err.Error(), "branch changed") {
		t.Fatalf("tampered managed worktree must fail before mutation, got %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "blocked.txt")); !os.IsNotExist(err) {
		t.Fatalf("blocked mutation touched file: %v", err)
	}
}

func TestOutOfWorkspaceEnvironmentWithoutManagedRecordIsRejectedByRuntime(t *testing.T) {
	workspaceRoot := t.TempDir()
	outsideRoot := t.TempDir()
	service := New(filepath.Join(t.TempDir(), "adm", "state.json"))
	ws, err := service.Workspaces.Add(workspaceRoot, "plain")
	if err != nil {
		t.Fatal(err)
	}
	env, err := service.Environments.Create(ws.ID, "plain", "")
	if err != nil {
		t.Fatal(err)
	}
	if err := service.Store.Update(func(state *model.State) error {
		for i := range state.Environments {
			if state.Environments[i].ID == env.ID {
				state.Environments[i].Root = outsideRoot
				break
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Environments.AcquireWriter(env.ID, "writer"); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Write(env.ID, "writer", "blocked.txt", "no\n", false); err == nil || !strings.Contains(err.Error(), "outside workspace") {
		t.Fatalf("out-of-Workspace unmanaged root must be rejected, got %v", err)
	}
	if _, err := os.Stat(filepath.Join(outsideRoot, "blocked.txt")); !os.IsNotExist(err) {
		t.Fatalf("rejected unmanaged root mutation touched file: %v", err)
	}
}

func appTestGitRepo(t *testing.T) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "source")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	appGitRun(t, root, "init", "-b", "main")
	appGitRun(t, root, "config", "user.email", "adm-test@example.invalid")
	appGitRun(t, root, "config", "user.name", "ADM Test")
	if err := os.WriteFile(filepath.Join(root, "tracked.txt"), []byte("base\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	appGitRun(t, root, "add", "tracked.txt")
	appGitRun(t, root, "commit", "-m", "base")
	return root
}

func appGitRun(t *testing.T, dir string, args ...string) string {
	t.Helper()
	out, err := appGitCommand(dir, args...)
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(out)
}

func appGitCommand(dir string, args ...string) (string, error) {
	git, err := exec.LookPath("git")
	if err != nil {
		return "", err
	}
	cmd := exec.Command(git, append([]string{"-C", dir}, args...)...)
	out, err := cmd.CombinedOutput()
	return string(out), err
}
