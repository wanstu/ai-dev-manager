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

func TestHardCleanupAfterGraceOnlyTargetsExplicitTTLEnvironments(t *testing.T) {
	service := app.New(filepath.Join(t.TempDir(), "state.json"))
	root := t.TempDir()
	initRetentionGitWorkspace(t, root)
	ws, err := service.Workspaces.Add(root, "retention-hard-cleanup")
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	create := func(name, branch string) isolation.CreateResult {
		result, err := service.Isolation.Create(ctx, ws.ID, name, isolation.CreateOptions{BranchName: branch, BaseRef: "HEAD"})
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(result.ManagedWorktree.Root, "dirty.txt"), []byte(name), 0644); err != nil {
			t.Fatal(err)
		}
		return result
	}
	oldTTL := create("old-explicit-ttl", "test/hard-old")
	recentTTL := create("recent-explicit-ttl", "test/hard-recent")
	legacy := create("legacy-policy", "test/hard-legacy")

	now := time.Now().UTC()
	oldExpiry := now.Add(-48 * time.Hour)
	recentExpiry := now.Add(-2 * time.Hour)
	for _, tc := range []struct {
		id, owner, policy string
		expires           time.Time
	}{
		{oldTTL.Environment.ID, "old-owner", temporaryEnvironmentTTLPolicy, oldExpiry},
		{recentTTL.Environment.ID, "recent-owner", temporaryEnvironmentTTLPolicy, recentExpiry},
		{legacy.Environment.ID, "legacy-owner", "managed_worktree_temporary", oldExpiry},
	} {
		if _, err := service.MarkResourceTemporary(model.ResourceRetentionUpdateRequest{
			Kind:      model.RetentionResourceEnvironment,
			ID:        tc.id,
			OwnerID:   tc.owner,
			ExpiresAt: &tc.expires,
			Policy:    tc.policy,
		}); err != nil {
			t.Fatal(err)
		}
	}

	owner := newRuntimeOwner(service)
	defer owner.Close()
	result, err := owner.expiredTemporaryEnvironmentsCleanup(
		ctx,
		true,
		true,
		now.Add(-expiredTemporaryHardCleanupGrace),
		temporaryEnvironmentTTLPolicy,
		false,
	)
	if err != nil {
		t.Fatal(err)
	}
	if !hasTemporaryRetentionMutation(result.Removed, model.RetentionResourceEnvironment, oldTTL.Environment.ID) {
		t.Fatalf("old explicit-TTL Environment was not hard-cleaned: %+v", result)
	}
	if _, err := service.Environments.Get(oldTTL.Environment.ID); err == nil {
		t.Fatal("old explicit-TTL Environment still exists")
	}
	for _, id := range []string{recentTTL.Environment.ID, legacy.Environment.ID} {
		if _, err := service.Environments.Get(id); err != nil {
			t.Fatalf("non-target Environment %s was removed: %v", id, err)
		}
	}
}


func TestForceExpiredCleanupRecoversBrokenManagedWorktrees(t *testing.T) {
	service := app.New(filepath.Join(t.TempDir(), "state.json"))
	root := t.TempDir()
	initRetentionGitWorkspace(t, root)
	ws, err := service.Workspaces.Add(root, "retention-broken-recovery")
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	branchDrift, err := service.Isolation.Create(ctx, ws.ID, "branch-drift", isolation.CreateOptions{BranchName: "test/recovery-expected", BaseRef: "HEAD"})
	if err != nil {
		t.Fatal(err)
	}
	nonGit, err := service.Isolation.Create(ctx, ws.ID, "non-git-root", isolation.CreateOptions{BranchName: "test/recovery-non-git", BaseRef: "HEAD"})
	if err != nil {
		t.Fatal(err)
	}
	expired := time.Now().UTC().Add(-48 * time.Hour)
	for _, item := range []isolation.CreateResult{branchDrift, nonGit} {
		if _, err := service.MarkResourceTemporary(model.ResourceRetentionUpdateRequest{
			Kind: model.RetentionResourceEnvironment, ID: item.Environment.ID, OwnerID: "broken-recovery",
			ExpiresAt: &expired, Policy: temporaryEnvironmentTTLPolicy,
		}); err != nil {
			t.Fatal(err)
		}
	}
	gatewayGitRun(t, branchDrift.ManagedWorktree.Root, "checkout", "-b", "test/recovery-actual")
	nonGitSentinel := filepath.Join(nonGit.ManagedWorktree.Root, "preserve-me.txt")
	if err := os.WriteFile(nonGitSentinel, []byte("preserve"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(nonGit.ManagedWorktree.Root, ".git")); err != nil {
		t.Fatal(err)
	}

	owner := newRuntimeOwner(service)
	defer owner.Close()
	normal, err := owner.ExpiredTemporaryEnvironmentsCleanup(ctx, false, false)
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range normal.Report.Resources {
		if !hasRetentionBlocker(item.Blockers, "managed_worktree_safety_check_failed") || item.CleanupEligible {
			t.Fatalf("broken worktree should be blocked without explicit force: %+v", item)
		}
	}

	forcedPreview, err := owner.ExpiredTemporaryEnvironmentsCleanup(ctx, false, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(forcedPreview.Report.Resources) != 2 {
		t.Fatalf("force preview resources=%+v", forcedPreview.Report.Resources)
	}
	for _, item := range forcedPreview.Report.Resources {
		if !item.CleanupEligible {
			t.Fatalf("recoverable broken worktree should be force eligible: %+v", item)
		}
		if !hasRetentionBlocker(item.Uncertainties, "force_cleanup_recovers_broken_managed_worktree") {
			t.Fatalf("force recovery must be disclosed: %+v", item)
		}
	}

	executed, err := owner.ExpiredTemporaryEnvironmentsCleanup(ctx, true, true)
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range []isolation.CreateResult{branchDrift, nonGit} {
		if !hasTemporaryRetentionMutation(executed.Removed, model.RetentionResourceEnvironment, item.Environment.ID) {
			t.Fatalf("broken Environment %s not removed: %+v", item.Environment.ID, executed)
		}
		if _, err := service.Environments.Get(item.Environment.ID); err == nil {
			t.Fatalf("broken Environment %s still exists", item.Environment.ID)
		}
	}
	if _, err := os.Stat(branchDrift.ManagedWorktree.Root); !os.IsNotExist(err) {
		t.Fatalf("branch-drift worktree root should be removed: %v", err)
	}
	if got := gatewayGitRun(t, root, "show-ref", "--verify", "--hash", "refs/heads/test/recovery-actual"); got == "" {
		t.Fatal("actual branch was not retained")
	}
	if got, err := os.ReadFile(nonGitSentinel); err != nil || string(got) != "preserve" {
		t.Fatalf("non-Git orphan directory contents must be preserved: got=%q err=%v", string(got), err)
	}
}
