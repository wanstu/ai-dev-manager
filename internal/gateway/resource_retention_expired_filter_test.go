package gateway

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"ai-dev-manager-v2/internal/app"
	"ai-dev-manager-v2/internal/isolation"
	"ai-dev-manager-v2/internal/model"
)

// Only expired temporary Environments should receive expensive worktree checks.
// In particular, durable and unexpired Environments must not be added to the
// preview, even when all share one Workspace and Git repository.
func TestExpiredTemporaryCleanupFiltersBeforeWorktreeSafety(t *testing.T) {
	service := app.New(filepath.Join(t.TempDir(), "state.json"))
	root := t.TempDir()
	initRetentionGitWorkspace(t, root)
	ws, err := service.Workspaces.Add(root, "retention-filter")
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	expired, err := service.Isolation.Create(ctx, ws.ID, "expired", isolation.CreateOptions{BranchName: "test/expired-filter", BaseRef: "HEAD"})
	if err != nil {
		t.Fatal(err)
	}
	future, err := service.Isolation.Create(ctx, ws.ID, "future", isolation.CreateOptions{BranchName: "test/future-filter", BaseRef: "HEAD"})
	if err != nil {
		t.Fatal(err)
	}
	durable, err := service.Isolation.Create(ctx, ws.ID, "durable", isolation.CreateOptions{BranchName: "test/durable-filter", BaseRef: "HEAD"})
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range []struct {
		id, owner string
		expires   time.Time
	}{
		{expired.Environment.ID, "expired-owner", time.Now().UTC().Add(-time.Hour)},
		{future.Environment.ID, "future-owner", time.Now().UTC().Add(time.Hour)},
	} {
		_, err := service.MarkResourceTemporary(model.ResourceRetentionUpdateRequest{
			Kind: model.RetentionResourceEnvironment, ID: item.id, OwnerID: item.owner, ExpiresAt: &item.expires,
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(expired.ManagedWorktree.Root, "not_committed.txt"), []byte("keep"), 0644); err != nil {
		t.Fatal(err)
	}
	owner := newRuntimeOwner(service)
	defer owner.Close()
	preview, err := owner.ExpiredTemporaryEnvironmentsCleanup(ctx, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(preview.Report.Resources) != 1 {
		t.Fatalf("expected only expired resource; got %+v", preview.Report.Resources)
	}
	got := preview.Report.Resources[0]
	if got.ID != expired.Environment.ID {
		t.Fatalf("unexpected inspected resource %s", got.ID)
	}
	if !hasRetentionBlocker(got.Blockers, "managed_worktree_dirty") || got.CleanupEligible {
		t.Fatalf("dirty expired worktree must remain blocked: %+v", got)
	}
	executed, err := owner.ExpiredTemporaryEnvironmentsCleanup(ctx, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(executed.Removed) > 0 {
		t.Fatalf("dirty worktree must not be removed: %+v", executed.Removed)
	}
	for _, id := range []string{expired.Environment.ID, future.Environment.ID, durable.Environment.ID} {
		if _, err := service.Environments.Get(id); err != nil {
			t.Fatalf("environment %s unexpectedly removed: %v", id, err)
		}
	}
}
