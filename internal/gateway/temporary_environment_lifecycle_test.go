package gateway

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"ai-dev-manager-v2/internal/app"
	"ai-dev-manager-v2/internal/isolation"
	"ai-dev-manager-v2/internal/model"
)

func TestTemporaryEnvironmentTargetedCleanupIsOwnerScopedAndDoesNotSweep(t *testing.T) {
	service := app.New(filepath.Join(t.TempDir(), "state.json"))
	workspaceRoot := t.TempDir()
	marker := filepath.Join(workspaceRoot, "keep.txt")
	if err := os.WriteFile(marker, []byte("keep\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	workspace, err := service.Workspaces.Add(workspaceRoot, "targeted-cleanup")
	if err != nil {
		t.Fatal(err)
	}
	expired := time.Now().UTC().Add(-time.Hour)
	first, err := service.Environments.CreateWithRetention(workspace.ID, "first", "", model.ResourceRetention{
		Persistence: model.PersistenceTemporary, OwnerID: "owner-first", ExpiresAt: &expired,
	})
	if err != nil {
		t.Fatal(err)
	}
	second, err := service.Environments.CreateWithRetention(workspace.ID, "second", "", model.ResourceRetention{
		Persistence: model.PersistenceTemporary, OwnerID: "owner-second", ExpiresAt: &expired,
	})
	if err != nil {
		t.Fatal(err)
	}

	owner := newRuntimeOwner(service)
	defer owner.Close()
	ctx := context.Background()
	preview, err := owner.TemporaryEnvironmentCleanup(ctx, first.ID, "ignored-for-preview", false)
	if err != nil {
		t.Fatal(err)
	}
	if !preview.DryRun || len(preview.Report.Resources) != 1 || preview.Report.Resources[0].ID != first.ID || !hasTemporaryRetentionMutation(preview.WouldRemove, model.RetentionResourceEnvironment, first.ID) {
		t.Fatalf("targeted preview = %+v", preview)
	}
	if hasTemporaryRetentionMutation(preview.WouldRemove, model.RetentionResourceEnvironment, second.ID) {
		t.Fatalf("targeted preview swept unrelated Environment: %+v", preview)
	}
	if _, err := owner.TemporaryEnvironmentCleanup(ctx, first.ID, "wrong-owner", true); err == nil {
		t.Fatal("wrong owner cleanup unexpectedly succeeded")
	}
	if _, err := service.Environments.Get(first.ID); err != nil {
		t.Fatalf("wrong owner cleanup mutated target: %v", err)
	}

	executed, err := owner.TemporaryEnvironmentCleanup(ctx, first.ID, "owner-first", true)
	if err != nil {
		t.Fatal(err)
	}
	if !hasTemporaryRetentionMutation(executed.Removed, model.RetentionResourceEnvironment, first.ID) {
		t.Fatalf("targeted cleanup did not remove target: %+v", executed)
	}
	if _, err := service.Environments.Get(first.ID); err == nil {
		t.Fatalf("target Environment still exists: %s", first.ID)
	}
	if _, err := service.Environments.Get(second.ID); err != nil {
		t.Fatalf("targeted cleanup removed unrelated Environment: %v", err)
	}
	if _, err := os.Stat(marker); err != nil {
		t.Fatalf("ordinary targeted cleanup touched project files: %v", err)
	}

	if _, err := service.PromoteTemporaryEnvironment(second.ID, "owner-second"); err != nil {
		t.Fatal(err)
	}
	if _, err := owner.TemporaryEnvironmentCleanup(ctx, second.ID, "owner-second", true); err == nil || !strings.Contains(err.Error(), "not temporary") {
		t.Fatalf("promoted durable Environment should refuse temporary cleanup, err=%v", err)
	}
	if stored, err := service.Environments.Get(second.ID); err != nil || stored.Retention.Persistence != model.PersistenceDurable {
		t.Fatalf("promoted Environment changed unexpectedly: env=%+v err=%v", stored, err)
	}
}

func TestTemporaryEnvironmentCleanupRuntimeBlockersIncludeWriterProcessRunAndVerifierRun(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "state.json")
	service := app.New(statePath)
	workspace, err := service.Workspaces.Add(t.TempDir(), "runtime-blockers")
	if err != nil {
		t.Fatal(err)
	}
	expired := time.Now().UTC().Add(-time.Hour)
	environment, err := service.Environments.CreateWithRetention(workspace.ID, "runtime-blocked", "", model.ResourceRetention{
		Persistence: model.PersistenceTemporary, OwnerID: "owner-runtime", ExpiresAt: &expired,
	})
	if err != nil {
		t.Fatal(err)
	}
	owner := newRuntimeOwner(service)
	defer owner.Close()
	ctx := context.Background()

	assertBlocker := func(want string) {
		t.Helper()
		status, err := owner.TemporaryEnvironmentStatus(ctx, environment.ID)
		if err != nil {
			t.Fatal(err)
		}
		if status.CleanupEligible || !hasRetentionBlocker(status.Blockers, want) {
			t.Fatalf("want blocker %q, status=%+v", want, status)
		}
	}

	if _, err := service.Environments.AcquireWriter(environment.ID, "writer-active"); err != nil {
		t.Fatal(err)
	}
	assertBlocker("active_writer")
	if _, err := service.Environments.ReleaseWriter(environment.ID, "writer-active", false); err != nil {
		t.Fatal(err)
	}

	owner.mu.Lock()
	owner.processes["proc_test"] = &ownedDevProcess{id: "proc_test", environmentID: environment.ID, state: devProcessRunning}
	owner.mu.Unlock()
	assertBlocker("active_process")
	owner.mu.Lock()
	delete(owner.processes, "proc_test")
	owner.mu.Unlock()

	owner.mu.Lock()
	owner.runs["run_test"] = &ownedAgentRun{id: "run_test", environmentID: environment.ID, state: agentRunRunning}
	owner.mu.Unlock()
	assertBlocker("active_run")
	owner.mu.Lock()
	delete(owner.runs, "run_test")
	owner.mu.Unlock()

	owner.mu.Lock()
	owner.verifierRuns["vfrun_test"] = &ownedVerifierRun{id: "vfrun_test", environmentID: environment.ID, state: verifierRunRunning}
	owner.mu.Unlock()
	assertBlocker("active_verifier_run")
	owner.mu.Lock()
	owner.verifierRuns["vfrun_test"].state = verifierRunSucceeded
	owner.mu.Unlock()
	status, err := owner.TemporaryEnvironmentStatus(ctx, environment.ID)
	if err != nil {
		t.Fatal(err)
	}
	if hasRetentionBlocker(status.Blockers, "active_verifier_run") || !status.CleanupEligible {
		t.Fatalf("terminal verifier observation should not block cleanup: %+v", status)
	}
	owner.mu.Lock()
	delete(owner.verifierRuns, "vfrun_test")
	owner.mu.Unlock()

	stateBytes, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(stateBytes), "vfrun_test") || strings.Contains(string(stateBytes), "active_verifier_run") {
		t.Fatalf("owner-local verifier blocker leaked into persisted state: %s", stateBytes)
	}
}

func TestTemporaryManagedWorktreeTargetedCleanupUsesDestroySafety(t *testing.T) {
	service := app.New(filepath.Join(t.TempDir(), "state.json"))
	workspaceRoot := t.TempDir()
	initRetentionGitWorkspace(t, workspaceRoot)
	workspace, err := service.Workspaces.Add(workspaceRoot, "targeted-managed-cleanup")
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	created, err := service.CreateTemporaryEnvironment(ctx, "agent", model.TemporaryEnvironmentCreateRequest{
		WorkspaceID: workspace.ID,
		Name:        "managed-cleanup",
		OwnerID:     "owner-managed",
		TTLSeconds:  1,
		Mode:        model.TemporaryEnvironmentModeManagedWorktree,
		BranchName:  "test/cleanup-managed",
	})
	if err != nil {
		t.Fatal(err)
	}
	if created.ManagedWorktree == nil {
		t.Fatal("managed worktree metadata missing")
	}
	time.Sleep(1100 * time.Millisecond)
	owner := newRuntimeOwner(service)
	defer owner.Close()
	executed, err := owner.TemporaryEnvironmentCleanup(ctx, created.Environment.ID, "owner-managed", true)
	if err != nil {
		t.Fatal(err)
	}
	if !hasTemporaryRetentionMutation(executed.Removed, model.RetentionResourceEnvironment, created.Environment.ID) {
		t.Fatalf("managed targeted cleanup did not remove Environment: %+v", executed)
	}
	if _, err := os.Stat(created.ManagedWorktree.Root); !os.IsNotExist(err) {
		t.Fatalf("managed worktree root still exists: %v", err)
	}
	if out, err := gitCommandOutput(workspaceRoot, "show-ref", "--verify", "refs/heads/"+created.ManagedWorktree.Branch); err != nil {
		t.Fatalf("managed branch should be retained: %v\n%s", err, out)
	}
}

func TestTemporaryManagedWorktreeTargetedCleanupBlocksDirtyWork(t *testing.T) {
	service := app.New(filepath.Join(t.TempDir(), "state.json"))
	workspaceRoot := t.TempDir()
	initRetentionGitWorkspace(t, workspaceRoot)
	workspace, err := service.Workspaces.Add(workspaceRoot, "targeted-managed-dirty")
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	created, err := service.CreateTemporaryEnvironment(ctx, "agent", model.TemporaryEnvironmentCreateRequest{
		WorkspaceID: workspace.ID,
		Name:        "managed-dirty",
		OwnerID:     "owner-dirty",
		TTLSeconds:  1,
		Mode:        model.TemporaryEnvironmentModeManagedWorktree,
		BranchName:  "test/dirty-managed",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(created.Environment.Root, "dirty.txt"), []byte("dirty\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	time.Sleep(1100 * time.Millisecond)
	owner := newRuntimeOwner(service)
	defer owner.Close()
	preview, err := owner.TemporaryEnvironmentCleanup(ctx, created.Environment.ID, "owner-dirty", false)
	if err != nil {
		t.Fatal(err)
	}
	if len(preview.Report.Resources) != 1 || !hasRetentionBlocker(preview.Report.Resources[0].Blockers, "managed_worktree_dirty") || preview.Report.Resources[0].CleanupEligible {
		t.Fatalf("dirty managed preview = %+v", preview)
	}
	if _, err := owner.TemporaryEnvironmentCleanup(ctx, created.Environment.ID, "owner-dirty", true); err == nil {
		t.Fatal("dirty managed cleanup unexpectedly succeeded")
	}
	if _, err := service.Environments.Get(created.Environment.ID); err != nil {
		t.Fatalf("dirty managed Environment was removed: %v", err)
	}

	writer := "test-force-cleanup"
	if _, err := service.Environments.AcquireWriter(created.Environment.ID, writer); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Isolation.Destroy(ctx, created.Environment.ID, writer, true); err != nil {
		t.Fatal(err)
	}
}

func TestTemporaryManagedWorktreeTargetedCleanupBlocksUnpublishedAndTamperedWork(t *testing.T) {
	t.Run("unpublished", func(t *testing.T) {
		service := app.New(filepath.Join(t.TempDir(), "state.json"))
		workspaceRoot := t.TempDir()
		initRetentionGitWorkspace(t, workspaceRoot)
		workspace, err := service.Workspaces.Add(workspaceRoot, "targeted-managed-unpublished")
		if err != nil {
			t.Fatal(err)
		}
		ctx := context.Background()
		created, err := service.Isolation.Create(ctx, workspace.ID, "managed-unpublished", isolation.CreateOptions{BranchName: "test/managed-unpublished", BaseRef: "HEAD"})
		if err != nil {
			t.Fatal(err)
		}
		expired := time.Now().UTC().Add(-time.Hour)
		if _, err := service.MarkResourceTemporary(model.ResourceRetentionUpdateRequest{Kind: model.RetentionResourceEnvironment, ID: created.Environment.ID, OwnerID: "owner-unpublished", ExpiresAt: &expired}); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(created.Environment.Root, "commit.txt"), []byte("commit\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		mustGitCommand(t, created.Environment.Root, "add", "commit.txt")
		mustGitCommand(t, created.Environment.Root, "commit", "-m", "local unpublished")

		owner := newRuntimeOwner(service)
		defer owner.Close()
		preview, err := owner.TemporaryEnvironmentCleanup(ctx, created.Environment.ID, "owner-unpublished", false)
		if err != nil {
			t.Fatal(err)
		}
		if len(preview.Report.Resources) != 1 || !hasRetentionBlocker(preview.Report.Resources[0].Blockers, "managed_worktree_unpublished") {
			t.Fatalf("unpublished blocker missing: %+v", preview)
		}
		if _, err := owner.TemporaryEnvironmentCleanup(ctx, created.Environment.ID, "owner-unpublished", true); err == nil {
			t.Fatal("unpublished managed cleanup unexpectedly succeeded")
		}
		writer := "test-force-unpublished"
		if _, err := service.Environments.AcquireWriter(created.Environment.ID, writer); err != nil {
			t.Fatal(err)
		}
		if _, err := service.Isolation.Destroy(ctx, created.Environment.ID, writer, true); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("tampered", func(t *testing.T) {
		service := app.New(filepath.Join(t.TempDir(), "state.json"))
		workspaceRoot := t.TempDir()
		initRetentionGitWorkspace(t, workspaceRoot)
		workspace, err := service.Workspaces.Add(workspaceRoot, "targeted-managed-tampered")
		if err != nil {
			t.Fatal(err)
		}
		ctx := context.Background()
		created, err := service.Isolation.Create(ctx, workspace.ID, "managed-tampered", isolation.CreateOptions{BranchName: "test/managed-tampered", BaseRef: "HEAD"})
		if err != nil {
			t.Fatal(err)
		}
		expired := time.Now().UTC().Add(-time.Hour)
		if _, err := service.MarkResourceTemporary(model.ResourceRetentionUpdateRequest{Kind: model.RetentionResourceEnvironment, ID: created.Environment.ID, OwnerID: "owner-tampered", ExpiresAt: &expired}); err != nil {
			t.Fatal(err)
		}
		mustGitCommand(t, created.Environment.Root, "checkout", "-b", "tampered-phase22")
		defer func() {
			_, _ = gitCommandOutput(workspaceRoot, "worktree", "remove", "--force", created.Environment.Root)
		}()

		owner := newRuntimeOwner(service)
		defer owner.Close()
		preview, err := owner.TemporaryEnvironmentCleanup(ctx, created.Environment.ID, "owner-tampered", false)
		if err != nil {
			t.Fatal(err)
		}
		if len(preview.Report.Resources) != 1 || !hasRetentionBlocker(preview.Report.Resources[0].Blockers, "managed_worktree_safety_check_failed") {
			t.Fatalf("tampered safety blocker missing: %+v", preview)
		}
		if _, err := owner.TemporaryEnvironmentCleanup(ctx, created.Environment.ID, "owner-tampered", true); err == nil {
			t.Fatal("tampered managed cleanup unexpectedly succeeded")
		}
	})
}

func mustGitCommand(t *testing.T, root string, args ...string) string {
	t.Helper()
	out, err := gitCommandOutput(root, args...)
	if err != nil {
		t.Fatalf("git %s failed: %v\n%s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(out)
}

func gitCommandOutput(root string, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func hasTemporaryRetentionMutation(items []model.ResourceRetentionCleanupMutation, kind, id string) bool {
	for _, item := range items {
		if item.Kind == kind && item.ID == id && item.Action == model.RetentionCleanupActionRemove {
			return true
		}
	}
	return false
}
