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
	preview, err := owner.ExpiredTemporaryEnvironmentsCleanup(ctx, false, false)
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
	executed, err := owner.ExpiredTemporaryEnvironmentsCleanup(ctx, true, false)
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

	forcePreview, err := owner.ExpiredTemporaryEnvironmentsCleanup(ctx, false, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(forcePreview.Report.Resources) != 1 || !forcePreview.Report.Resources[0].CleanupEligible {
		t.Fatalf("force preview should make expired dirty managed worktree eligible: %+v", forcePreview)
	}
	if !hasRetentionBlocker(forcePreview.Report.Resources[0].Uncertainties, "force_cleanup_discards_uncommitted_or_ignored_work") {
		t.Fatalf("force preview must disclose destructive cleanup: %+v", forcePreview.Report.Resources[0])
	}
	forced, err := owner.ExpiredTemporaryEnvironmentsCleanup(ctx, true, true)
	if err != nil {
		t.Fatal(err)
	}
	if !hasTemporaryRetentionMutation(forced.Removed, model.RetentionResourceEnvironment, expired.Environment.ID) {
		t.Fatalf("force cleanup did not remove expired dirty managed worktree: %+v", forced)
	}
	if _, err := service.Environments.Get(expired.Environment.ID); err == nil {
		t.Fatal("expired dirty Environment still exists after force cleanup")
	}
	if _, err := os.Stat(expired.ManagedWorktree.Root); !os.IsNotExist(err) {
		t.Fatalf("expired dirty managed root still exists after force cleanup: %v", err)
	}
	for _, id := range []string{future.Environment.ID, durable.Environment.ID} {
		if _, err := service.Environments.Get(id); err != nil {
			t.Fatalf("non-expired/durable environment %s unexpectedly removed: %v", id, err)
		}
	}
}
