package gateway

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"ai-dev-manager-v2/internal/app"
	"ai-dev-manager-v2/internal/isolation"
	"ai-dev-manager-v2/internal/model"
)

func TestStaleManagedWorktreeCleanupPreviewClassifiesCandidatesConservatively(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not available")
	}
	ctx := context.Background()
	service := app.New(filepath.Join(t.TempDir(), "adm", "state.json"))
	source := t.TempDir()
	initRetentionGitWorkspace(t, source)
	workspace, err := service.Workspaces.Add(source, "cleanup-preview")
	if err != nil {
		t.Fatal(err)
	}

	clean := createCleanupManagedWorktree(t, ctx, service, workspace.ID, "clean-stale", "cleanup/clean-stale")
	recent := createCleanupManagedWorktree(t, ctx, service, workspace.ID, "recent", "cleanup/recent")
	dirty := createCleanupManagedWorktree(t, ctx, service, workspace.ID, "dirty-stale", "cleanup/dirty-stale")
	unpublished := createCleanupManagedWorktree(t, ctx, service, workspace.ID, "unpublished-stale", "cleanup/unpublished-stale")
	temporary := createCleanupManagedWorktree(t, ctx, service, workspace.ID, "temporary-stale", "cleanup/temporary-stale")

	for _, item := range []string{clean.Environment.ID, dirty.Environment.ID, unpublished.Environment.ID, temporary.Environment.ID} {
		ageCleanupEnvironment(t, service, item, 48*time.Hour)
	}
	if err := os.WriteFile(filepath.Join(dirty.ManagedWorktree.Root, "dirty.txt"), []byte("dirty\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(unpublished.ManagedWorktree.Root, "local.txt"), []byte("local\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gatewayGitRun(t, unpublished.ManagedWorktree.Root, "add", "local.txt")
	gatewayGitRun(t, unpublished.ManagedWorktree.Root, "commit", "-m", "local unpublished")
	expiresAt := time.Now().UTC().Add(time.Hour)
	if _, err := service.MarkResourceTemporary(model.ResourceRetentionUpdateRequest{
		Kind:      model.RetentionResourceEnvironment,
		ID:        temporary.Environment.ID,
		OwnerID:   "temp-owner",
		ExpiresAt: &expiresAt,
	}); err != nil {
		t.Fatal(err)
	}

	owner := newRuntimeOwner(service)
	defer owner.Close()
	result, err := owner.StaleManagedWorktreeCleanup(ctx, 24*60*60, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	if !result.DryRun {
		t.Fatal("preview must be dry-run")
	}
	candidates := cleanupCandidateMap(result.Report.Candidates)
	if got := candidates[clean.Environment.ID]; !got.CleanupEligible || len(got.Blockers) != 0 {
		t.Fatalf("clean stale candidate=%+v", got)
	}
	if got := candidates[recent.Environment.ID]; got.CleanupEligible || !containsGatewayString(got.Blockers, "recent_activity") {
		t.Fatalf("recent candidate=%+v", got)
	}
	if got := candidates[dirty.Environment.ID]; got.CleanupEligible || !got.Dirty || !containsGatewayString(got.Blockers, "managed_worktree_dirty") {
		t.Fatalf("dirty candidate=%+v", got)
	}
	if got := candidates[unpublished.Environment.ID]; got.CleanupEligible || !got.Unpublished || !containsGatewayString(got.Blockers, "managed_worktree_unpublished") {
		t.Fatalf("unpublished candidate=%+v", got)
	}
	if _, ok := candidates[temporary.Environment.ID]; ok {
		t.Fatalf("temporary managed worktree must use temporary cleanup, got %+v", candidates[temporary.Environment.ID])
	}
	for _, item := range []isolation.CreateResult{clean, recent, dirty, unpublished, temporary} {
		if _, err := os.Stat(item.ManagedWorktree.Root); err != nil {
			t.Fatalf("preview mutated managed root %s: %v", item.ManagedWorktree.Root, err)
		}
	}
}

func TestStaleManagedWorktreeCleanupExecuteRequiresExplicitSafeSelection(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not available")
	}
	ctx := context.Background()
	service := app.New(filepath.Join(t.TempDir(), "adm", "state.json"))
	source := t.TempDir()
	initRetentionGitWorkspace(t, source)
	workspace, err := service.Workspaces.Add(source, "cleanup-execute")
	if err != nil {
		t.Fatal(err)
	}

	clean := createCleanupManagedWorktree(t, ctx, service, workspace.ID, "clean-stale", "cleanup/execute-clean")
	dirty := createCleanupManagedWorktree(t, ctx, service, workspace.ID, "dirty-stale", "cleanup/execute-dirty")
	ageCleanupEnvironment(t, service, clean.Environment.ID, 48*time.Hour)
	ageCleanupEnvironment(t, service, dirty.Environment.ID, 48*time.Hour)
	if err := os.WriteFile(filepath.Join(dirty.ManagedWorktree.Root, "dirty.txt"), []byte("dirty\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	owner := newRuntimeOwner(service)
	defer owner.Close()
	if _, err := owner.StaleManagedWorktreeCleanup(ctx, 24*60*60, nil, true); err == nil {
		t.Fatal("execute without explicit environment_ids must fail")
	}

	result, err := owner.StaleManagedWorktreeCleanup(ctx, 24*60*60, []string{clean.Environment.ID, dirty.Environment.ID}, true)
	if err != nil {
		t.Fatal(err)
	}
	if result.DryRun {
		t.Fatal("execute result reported dry-run")
	}
	if len(result.Removed) != 1 || result.Removed[0].EnvironmentID != clean.Environment.ID {
		t.Fatalf("removed=%+v", result.Removed)
	}
	if len(result.Skipped) != 1 || result.Skipped[0].EnvironmentID != dirty.Environment.ID || !containsGatewayString(result.Skipped[0].Blockers, "managed_worktree_dirty") {
		t.Fatalf("skipped=%+v", result.Skipped)
	}
	if _, err := service.Environments.Get(clean.Environment.ID); err == nil {
		t.Fatal("clean stale Environment state still exists")
	}
	if _, err := os.Stat(clean.ManagedWorktree.Root); !os.IsNotExist(err) {
		t.Fatalf("clean stale worktree root still exists: %v", err)
	}
	if got := gatewayGitRun(t, source, "show-ref", "--verify", "--hash", "refs/heads/"+clean.ManagedWorktree.Branch); got == "" {
		t.Fatal("cleanup deleted retained branch")
	}
	if _, err := service.Environments.Get(dirty.Environment.ID); err != nil {
		t.Fatalf("dirty Environment should remain: %v", err)
	}
	if _, err := os.Stat(dirty.ManagedWorktree.Root); err != nil {
		t.Fatalf("dirty managed root should remain: %v", err)
	}
}

func createCleanupManagedWorktree(t *testing.T, ctx context.Context, service *app.Service, workspaceID, name, branch string) isolation.CreateResult {
	t.Helper()
	created, err := service.Isolation.Create(ctx, workspaceID, name, isolation.CreateOptions{BranchName: branch, BaseRef: "HEAD"})
	if err != nil {
		t.Fatal(err)
	}
	return created
}

func ageCleanupEnvironment(t *testing.T, service *app.Service, environmentID string, age time.Duration) {
	t.Helper()
	when := time.Now().UTC().Add(-age)
	if err := service.Store.Update(func(state *model.State) error {
		for i := range state.Environments {
			if state.Environments[i].ID == environmentID {
				state.Environments[i].LastActivityAt = when
				state.Environments[i].UpdatedAt = when
				return nil
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func cleanupCandidateMap(items []model.ManagedWorktreeCleanupCandidate) map[string]model.ManagedWorktreeCleanupCandidate {
	out := make(map[string]model.ManagedWorktreeCleanupCandidate, len(items))
	for _, item := range items {
		out[item.EnvironmentID] = item
	}
	return out
}
