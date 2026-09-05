package environment

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"ai-dev-manager/internal/adapter/runtimeadapter"
	"ai-dev-manager/internal/controlplane"
	"ai-dev-manager/internal/model"
	"ai-dev-manager/internal/workspace"
)

func TestManagerPersistentIsolatedEnvironmentsAndConservativeDestroy(t *testing.T) {
	ctx := context.Background()
	configRoot := t.TempDir()
	repo, gitPath := initEnvironmentGitRepo(t)

	service, err := controlplane.New(configRoot)
	if err != nil {
		t.Fatal(err)
	}
	ws, err := service.Registry().Add(workspace.Input{Path: repo})
	if err != nil {
		t.Fatal(err)
	}
	if err := service.Store().SaveProject(repo, model.ConfigLayer{
		Scope: model.ScopeProject,
		Policy: &model.Policy{
			Mode:               "standard",
			AllowedExecutables: []string{"git"},
			ToolPaths:          map[string]string{"git": gitPath},
		},
	}); err != nil {
		t.Fatal(err)
	}

	manager := newTestManager(t, configRoot, service)

	if err := os.WriteFile(filepath.Join(repo, "source.txt"), []byte("dirty-main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "local.tmp"), []byte("untracked-main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	mainHead := gitEnvOutput(t, gitPath, repo, "rev-parse", "HEAD")
	mainBranch := gitEnvOutput(t, gitPath, repo, "branch", "--show-current")
	oldHead := gitEnvOutput(t, gitPath, repo, "rev-parse", "HEAD~1")

	createdA, err := manager.Create(ctx, CreateRequest{WorkspaceID: ws.ID, Name: "task-a"})
	if err != nil {
		t.Fatalf("Create(task-a) error = %v", err)
	}
	createdB, err := manager.Create(ctx, CreateRequest{WorkspaceID: ws.ID, Name: "task-b", Base: "HEAD~1"})
	if err != nil {
		t.Fatalf("Create(task-b) error = %v", err)
	}
	if createdA.Environment.ID == createdB.Environment.ID || samePath(createdA.Environment.WorktreePath, createdB.Environment.WorktreePath) {
		t.Fatalf("environments not isolated: A=%+v B=%+v", createdA.Environment, createdB.Environment)
	}
	if createdA.Environment.Branch != "adm/task-a" || createdB.Environment.Branch != "adm/task-b" {
		t.Fatalf("unexpected branches: %q %q", createdA.Environment.Branch, createdB.Environment.Branch)
	}
	if createdA.Environment.BaseRef != "dev" || createdA.Environment.BaseCommit != mainHead {
		t.Fatalf("default base = %q %q, want dev %q", createdA.Environment.BaseRef, createdA.Environment.BaseCommit, mainHead)
	}
	if createdB.Environment.BaseCommit != oldHead {
		t.Fatalf("explicit base commit = %q, want %q", createdB.Environment.BaseCommit, oldHead)
	}
	if !hasWarning(createdA.Warnings, "changes_not_included") || !hasWarning(createdB.Warnings, "changes_not_included") {
		t.Fatalf("dirty base warning missing: A=%+v B=%+v", createdA.Warnings, createdB.Warnings)
	}
	if got := strings.TrimSpace(mustReadString(t, filepath.Join(createdA.Environment.WorktreePath, "source.txt"))); got != "committed-second" {
		t.Fatalf("task-a copied dirty main content: %q", got)
	}
	if _, err := os.Stat(filepath.Join(createdA.Environment.WorktreePath, "local.tmp")); !os.IsNotExist(err) {
		t.Fatalf("task-a copied untracked main file, stat err = %v", err)
	}
	if _, err := os.Stat(filepath.Join(createdB.Environment.WorktreePath, "second.txt")); !os.IsNotExist(err) {
		t.Fatalf("task-b explicit old base unexpectedly contains second.txt, stat err = %v", err)
	}
	if got := gitEnvOutput(t, gitPath, repo, "rev-parse", "HEAD"); got != mainHead {
		t.Fatalf("main HEAD changed: %q -> %q", mainHead, got)
	}
	if got := gitEnvOutput(t, gitPath, repo, "branch", "--show-current"); got != mainBranch {
		t.Fatalf("main branch changed: %q -> %q", mainBranch, got)
	}
	if got := strings.TrimSpace(mustReadString(t, filepath.Join(repo, "source.txt"))); got != "dirty-main" {
		t.Fatalf("main dirty content changed: %q", got)
	}

	// Simulate daemon restart: new Control Plane + new Environment Manager, with
	// only persisted Workspace/Environment state and actual Git worktrees.
	service2, err := controlplane.New(configRoot)
	if err != nil {
		t.Fatal(err)
	}
	manager2 := newTestManager(t, configRoot, service2)
	list, err := manager2.List()
	if err != nil || len(list) != 2 {
		t.Fatalf("List(after restart) = %+v, %v", list, err)
	}
	inspectedA, err := manager2.Inspect(ctx, createdA.Environment.ID)
	if err != nil {
		t.Fatalf("Inspect(after restart) error = %v", err)
	}
	if !samePath(inspectedA.Environment.WorktreePath, createdA.Environment.WorktreePath) {
		t.Fatalf("restart worktree path = %q, want %q", inspectedA.Environment.WorktreePath, createdA.Environment.WorktreePath)
	}

	derivedA, err := service2.BuildDerivedRuntime(ws.ID, createdA.Environment.ID, inspectedA.Environment.WorktreePath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := derivedA.Invoke(ctx, runtimeadapter.OpEdit, map[string]any{
		"path":                  "source.txt",
		"old_text":              "committed-second",
		"new_text":              "environment-a",
		"expected_replacements": 1,
	}); err != nil {
		t.Fatalf("edit environment A: %v", err)
	}
	if got := strings.TrimSpace(mustReadString(t, filepath.Join(createdA.Environment.WorktreePath, "source.txt"))); got != "environment-a" {
		t.Fatalf("environment A edit missing: %q", got)
	}
	if got := strings.TrimSpace(mustReadString(t, filepath.Join(repo, "source.txt"))); got != "dirty-main" {
		t.Fatalf("environment A leaked to main: %q", got)
	}
	if got := strings.TrimSpace(mustReadString(t, filepath.Join(createdB.Environment.WorktreePath, "source.txt"))); got != "committed-first" {
		t.Fatalf("environment A leaked to B: %q", got)
	}

	if _, err := manager2.Destroy(ctx, createdA.Environment.ID, false); err == nil || !isEnvironmentCode(err, ErrUnsafeDestroy) {
		t.Fatalf("dirty Destroy(A) error = %v, want unsafe_destroy", err)
	}
	removedB, err := manager2.Destroy(ctx, createdB.Environment.ID, false)
	if err != nil {
		t.Fatalf("Destroy(clean B) error = %v", err)
	}
	if _, err := os.Stat(removedB.WorktreePath); !os.IsNotExist(err) {
		t.Fatalf("B worktree still exists after destroy: %v", err)
	}
	if got := gitEnvOutput(t, gitPath, repo, "branch", "--list", "adm/task-b"); !strings.Contains(got, "adm/task-b") {
		t.Fatalf("destroy unexpectedly deleted branch: %q", got)
	}
	if _, err := manager2.Create(ctx, CreateRequest{WorkspaceID: ws.ID, Name: "task-b"}); err == nil || !isEnvironmentCode(err, ErrBranchExists) {
		t.Fatalf("recreate existing branch error = %v, want branch_exists", err)
	}

	gitEnvRun(t, gitPath, repo, "worktree", "remove", "--force", createdA.Environment.WorktreePath)
	if _, err := manager2.Inspect(ctx, createdA.Environment.ID); err == nil || !isEnvironmentCode(err, ErrWorktreeMissing) {
		t.Fatalf("Inspect(missing worktree) error = %v, want worktree_missing", err)
	}
	if items, err := manager2.List(); err != nil || len(items) != 1 || items[0].ID != createdA.Environment.ID {
		t.Fatalf("missing worktree registry record lost: %+v, %v", items, err)
	}
	if _, err := manager2.Destroy(ctx, createdA.Environment.ID, true); err != nil {
		t.Fatalf("force destroy missing worktree error = %v", err)
	}
	if items, err := manager2.List(); err != nil || len(items) != 0 {
		t.Fatalf("force destroy did not remove stale registry record: %+v, %v", items, err)
	}
}

func newTestManager(t *testing.T, root string, service *controlplane.Service) *Manager {
	t.Helper()
	manager, err := NewManager(
		NewStore(root),
		service.Registry().Get,
		func(workspaceID string) (runtimeadapter.Runtime, error) {
			return service.BuildRuntime(workspaceID, nil)
		},
		service.BuildDerivedRuntime,
	)
	if err != nil {
		t.Fatal(err)
	}
	return manager
}

func initEnvironmentGitRepo(t *testing.T) (string, string) {
	t.Helper()
	gitPath, err := exec.LookPath("git")
	if err != nil {
		t.Fatalf("git unavailable: %v", err)
	}
	gitPath, err = filepath.Abs(gitPath)
	if err != nil {
		t.Fatal(err)
	}
	repo := t.TempDir()
	gitEnvRun(t, gitPath, repo, "init")
	gitEnvRun(t, gitPath, repo, "config", "user.email", "adm-env@example.invalid")
	gitEnvRun(t, gitPath, repo, "config", "user.name", "ADM Environment Test")
	if err := os.WriteFile(filepath.Join(repo, "source.txt"), []byte("committed-first\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitEnvRun(t, gitPath, repo, "add", "source.txt")
	gitEnvRun(t, gitPath, repo, "commit", "-m", "first")
	gitEnvRun(t, gitPath, repo, "branch", "-M", "dev")
	if err := os.WriteFile(filepath.Join(repo, "source.txt"), []byte("committed-second\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "second.txt"), []byte("second\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitEnvRun(t, gitPath, repo, "add", "source.txt", "second.txt")
	gitEnvRun(t, gitPath, repo, "commit", "-m", "second")
	return repo, filepath.Clean(gitPath)
}

func gitEnvRun(t *testing.T, gitPath, cwd string, args ...string) {
	t.Helper()
	cmd := exec.Command(gitPath, args...)
	cmd.Dir = cwd
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s failed: %v\n%s", strings.Join(args, " "), err, output)
	}
}

func gitEnvOutput(t *testing.T, gitPath, cwd string, args ...string) string {
	t.Helper()
	cmd := exec.Command(gitPath, args...)
	cmd.Dir = cwd
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s failed: %v\n%s", strings.Join(args, " "), err, output)
	}
	return strings.TrimSpace(string(output))
}

func mustReadString(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func hasWarning(warnings []Warning, code string) bool {
	for _, warning := range warnings {
		if warning.Code == code {
			return true
		}
	}
	return false
}

func isEnvironmentCode(err error, code ErrorCode) bool {
	var target *Error
	return errors.As(err, &target) && target.Code == code
}
