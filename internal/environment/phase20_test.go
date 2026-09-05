package environment

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ai-dev-manager/internal/controlplane"
	"ai-dev-manager/internal/model"
	"ai-dev-manager/internal/workspace"
)

func TestManagerIncludeChangesPreservesPartialStageAndForceDestroy(t *testing.T) {
	ctx := context.Background()
	configRoot := t.TempDir()
	repo, gitPath := initEnvironmentGitRepo(t)
	gitEnvRun(t, gitPath, repo, "config", "core.autocrlf", "false")
	if err := os.WriteFile(filepath.Join(repo, ".gitignore"), []byte("*.tmp\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitEnvRun(t, gitPath, repo, "add", ".gitignore")
	gitEnvRun(t, gitPath, repo, "commit", "-m", "ignore tmp")

	service, ws := setupEnvironmentService(t, configRoot, repo, gitPath)
	manager := newTestManager(t, configRoot, service)

	if err := os.WriteFile(filepath.Join(repo, "source.txt"), []byte("staged version\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitEnvRun(t, gitPath, repo, "add", "source.txt")
	if err := os.WriteFile(filepath.Join(repo, "source.txt"), []byte("staged version\nunstaged tail\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "note.txt"), []byte("untracked\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "secret.tmp"), []byte("ignored\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	created, err := manager.Create(ctx, CreateRequest{WorkspaceID: ws.ID, Name: "include-changes", IncludeChanges: true})
	if err != nil {
		t.Fatalf("Create(include changes) error = %v", err)
	}
	if created.Environment.State != StateReady || created.Environment.Metadata["changes_included"] != "true" {
		t.Fatalf("include-changes environment = %+v", created.Environment)
	}
	for _, warning := range created.Warnings {
		if warning.Code == "changes_not_included" {
			t.Fatalf("include-changes returned wrong warning: %+v", created.Warnings)
		}
	}
	foundFact := false
	for _, fact := range created.Facts {
		if fact.Code == "changes_included" {
			foundFact = true
		}
	}
	if !foundFact {
		t.Fatalf("changes_included fact missing: %+v", created.Facts)
	}

	staged := gitEnvOutput(t, gitPath, created.Environment.WorktreePath, "diff", "--cached", "--name-only")
	unstaged := gitEnvOutput(t, gitPath, created.Environment.WorktreePath, "diff", "--name-only")
	if !strings.Contains(staged, "source.txt") || !strings.Contains(unstaged, "source.txt") {
		t.Fatalf("partial staged state not preserved: staged=%q unstaged=%q", staged, unstaged)
	}
	if got := strings.TrimSpace(mustReadString(t, filepath.Join(created.Environment.WorktreePath, "note.txt"))); got != "untracked" {
		t.Fatalf("untracked file not copied: %q", got)
	}
	if _, err := os.Stat(filepath.Join(created.Environment.WorktreePath, "secret.tmp")); !os.IsNotExist(err) {
		t.Fatalf("ignored file copied: %v", err)
	}

	if _, err := manager.Destroy(ctx, created.Environment.ID, false); err == nil || !isEnvironmentCode(err, ErrUnsafeDestroy) {
		t.Fatalf("normal dirty destroy error = %v, want unsafe_destroy", err)
	}
	if _, err := manager.Destroy(ctx, created.Environment.ID, true); err != nil {
		t.Fatalf("force dirty destroy error = %v", err)
	}
	if _, err := os.Stat(created.Environment.WorktreePath); !os.IsNotExist(err) {
		t.Fatalf("force destroy left worktree: %v", err)
	}
	if branch := gitEnvOutput(t, gitPath, repo, "branch", "--list", created.Environment.Branch); !strings.Contains(branch, created.Environment.Branch) {
		t.Fatalf("force destroy deleted branch: %q", branch)
	}
}

func TestManagerIncludeChangesConflictLeavesErrorEnvironment(t *testing.T) {
	ctx := context.Background()
	configRoot := t.TempDir()
	repo, gitPath := initEnvironmentGitRepo(t)
	service, ws := setupEnvironmentService(t, configRoot, repo, gitPath)
	manager := newTestManager(t, configRoot, service)

	if err := os.WriteFile(filepath.Join(repo, "source.txt"), []byte("current-only dirty value\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := manager.Create(ctx, CreateRequest{WorkspaceID: ws.ID, Name: "conflicting-transfer", Base: "HEAD~1", IncludeChanges: true})
	if err == nil {
		t.Fatal("conflicting include-changes unexpectedly succeeded")
	}
	items, listErr := manager.List()
	if listErr != nil || len(items) != 1 {
		t.Fatalf("error environment list = %+v, %v", items, listErr)
	}
	if items[0].State != StateError || items[0].WorktreePath == "" || items[0].Error == nil || items[0].Error.Code != "change_transfer_failed" {
		t.Fatalf("failed transfer not preserved for diagnosis: %+v", items[0])
	}
	if _, statErr := os.Stat(items[0].WorktreePath); statErr != nil {
		t.Fatalf("failed transfer worktree not preserved: %v", statErr)
	}
}

func TestManagerDestroyRejectsUnpushedAndAllowsPushedCleanCommit(t *testing.T) {
	ctx := context.Background()
	configRoot := t.TempDir()
	repo, gitPath := initEnvironmentGitRepo(t)
	service, ws := setupEnvironmentService(t, configRoot, repo, gitPath)
	manager := newTestManager(t, configRoot, service)

	created, err := manager.Create(ctx, CreateRequest{WorkspaceID: ws.ID, Name: "pushed-work"})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(created.Environment.WorktreePath, "work.txt"), []byte("done\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitEnvRun(t, gitPath, created.Environment.WorktreePath, "add", "work.txt")
	gitEnvRun(t, gitPath, created.Environment.WorktreePath, "commit", "-m", "environment work")

	if _, err := manager.Destroy(ctx, created.Environment.ID, false); err == nil || !isEnvironmentCode(err, ErrUnsafeDestroy) {
		t.Fatalf("unpushed normal destroy error = %v, want unsafe_destroy", err)
	}

	remote := filepath.Join(t.TempDir(), "remote.git")
	gitEnvRun(t, gitPath, repo, "init", "--bare", remote)
	gitEnvRun(t, gitPath, repo, "remote", "add", "origin", remote)
	gitEnvRun(t, gitPath, created.Environment.WorktreePath, "push", "-u", "origin", created.Environment.Branch)

	if _, err := manager.Destroy(ctx, created.Environment.ID, false); err != nil {
		t.Fatalf("pushed clean normal destroy error = %v", err)
	}
	if _, err := os.Stat(created.Environment.WorktreePath); !os.IsNotExist(err) {
		t.Fatalf("pushed environment worktree still exists: %v", err)
	}
	if branch := gitEnvOutput(t, gitPath, repo, "branch", "--list", created.Environment.Branch); !strings.Contains(branch, created.Environment.Branch) {
		t.Fatalf("normal destroy deleted pushed branch: %q", branch)
	}
}

func setupEnvironmentService(t *testing.T, configRoot, repo, gitPath string) (*controlplane.Service, model.Workspace) {
	t.Helper()
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
	return service, ws
}
