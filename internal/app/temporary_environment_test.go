package app

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"ai-dev-manager-v2/internal/model"
	"ai-dev-manager-v2/internal/pathutil"
)

func TestCreateTemporaryEnvironmentExistingRootAndValidation(t *testing.T) {
	service := New(filepath.Join(t.TempDir(), "state.json"))
	workspaceRoot := t.TempDir()
	workspace, err := service.Workspaces.Add(workspaceRoot, "temporary-existing-root")
	if err != nil {
		t.Fatal(err)
	}

	created, err := service.CreateTemporaryEnvironment(context.Background(), "agent", model.TemporaryEnvironmentCreateRequest{
		WorkspaceID: workspace.ID,
		Name:        "task-default",
		OwnerID:     "owner-a",
		TTLSeconds:  3600,
		SessionID:   "session-a",
		RunID:       "run_external_a",
	})
	if err != nil {
		t.Fatal(err)
	}
	if created.Mode != model.TemporaryEnvironmentModeExistingRoot || created.ManagedWorktree != nil {
		t.Fatalf("unexpected creation result: %+v", created)
	}
	retention := created.Environment.Retention
	if retention.Persistence != model.PersistenceTemporary || retention.CreatorSurface != "agent" || retention.OwnerID != "owner-a" || retention.SessionID != "session-a" || retention.RunID != "run_external_a" || retention.ExpiresAt == nil {
		t.Fatalf("temporary retention = %+v", retention)
	}
	if !filepath.IsAbs(created.Environment.Root) || !sameTestPath(created.Environment.Root, workspaceRoot) {
		t.Fatalf("environment root = %q want %q", created.Environment.Root, workspaceRoot)
	}

	subdir := filepath.Join(workspaceRoot, "subdir")
	if err := os.MkdirAll(subdir, 0o755); err != nil {
		t.Fatal(err)
	}
	sub, err := service.CreateTemporaryEnvironment(context.Background(), "agent", model.TemporaryEnvironmentCreateRequest{
		WorkspaceID: workspace.ID,
		Name:        "task-subdir",
		OwnerID:     "owner-a",
		TTLSeconds:  60,
		Root:        subdir,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !sameTestPath(sub.Environment.Root, subdir) {
		t.Fatalf("subdir root = %q want %q", sub.Environment.Root, subdir)
	}

	before, err := service.Environments.List()
	if err != nil {
		t.Fatal(err)
	}
	invalid := []model.TemporaryEnvironmentCreateRequest{
		{WorkspaceID: workspace.ID, Name: "missing-owner", TTLSeconds: 60},
		{WorkspaceID: workspace.ID, Name: "zero-ttl", OwnerID: "owner-a", TTLSeconds: 0},
		{WorkspaceID: workspace.ID, Name: "bad-mode", OwnerID: "owner-a", TTLSeconds: 60, Mode: "copy"},
		{WorkspaceID: workspace.ID, Name: "missing-root", OwnerID: "owner-a", TTLSeconds: 60, Root: filepath.Join(workspaceRoot, "missing")},
		{WorkspaceID: workspace.ID, Name: "outside-root", OwnerID: "owner-a", TTLSeconds: 60, Root: t.TempDir()},
	}
	for _, request := range invalid {
		if _, err := service.CreateTemporaryEnvironment(context.Background(), "agent", request); err == nil {
			t.Fatalf("invalid request unexpectedly succeeded: %+v", request)
		}
	}
	after, err := service.Environments.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(after) != len(before) {
		t.Fatalf("invalid creation installed Environment records: before=%d after=%d", len(before), len(after))
	}
}

func TestCreateTemporaryEnvironmentRejectsDurableCollision(t *testing.T) {
	service := New(filepath.Join(t.TempDir(), "state.json"))
	workspaceRoot := t.TempDir()
	workspace, err := service.Workspaces.Add(workspaceRoot, "temporary-collision")
	if err != nil {
		t.Fatal(err)
	}
	durable, err := service.Environments.Create(workspace.ID, "same", "")
	if err != nil {
		t.Fatal(err)
	}
	if durable.Retention.Persistence != model.PersistenceDurable {
		t.Fatalf("durable retention = %+v", durable.Retention)
	}
	if _, err := service.CreateTemporaryEnvironment(context.Background(), "agent", model.TemporaryEnvironmentCreateRequest{
		WorkspaceID: workspace.ID,
		Name:        "same",
		OwnerID:     "owner-a",
		TTLSeconds:  60,
	}); err == nil || !strings.Contains(err.Error(), "conflicts with existing environment") {
		t.Fatalf("durable collision should fail clearly, err=%v", err)
	}
	stored, err := service.Environments.Get(durable.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Retention.Persistence != model.PersistenceDurable || stored.Retention.OwnerID != "" {
		t.Fatalf("collision mutated durable Environment: %+v", stored.Retention)
	}
}

func TestCreateTemporaryManagedWorktreePersistsRetentionAndKeepsSourceUnchanged(t *testing.T) {
	service := New(filepath.Join(t.TempDir(), "state.json"))
	workspaceRoot := t.TempDir()
	initTemporaryEnvironmentGitWorkspace(t, workspaceRoot)
	workspace, err := service.Workspaces.Add(workspaceRoot, "temporary-managed")
	if err != nil {
		t.Fatal(err)
	}
	branchBefore := gitTemporaryOutput(t, workspaceRoot, "branch", "--show-current")
	headBefore := gitTemporaryOutput(t, workspaceRoot, "rev-parse", "HEAD")

	created, err := service.CreateTemporaryEnvironment(context.Background(), "agent", model.TemporaryEnvironmentCreateRequest{
		WorkspaceID: workspace.ID,
		Name:        "task-managed",
		OwnerID:     "owner-managed",
		TTLSeconds:  3600,
		Mode:        model.TemporaryEnvironmentModeManagedWorktree,
		BranchName:  "test/temporary-managed",
		BaseRef:     "HEAD",
		RunID:       "run_external_managed",
	})
	if err != nil {
		t.Fatal(err)
	}
	if created.ManagedWorktree == nil {
		t.Fatalf("managed worktree metadata missing: %+v", created)
	}
	if created.Environment.Retention.Persistence != model.PersistenceTemporary || created.Environment.Retention.OwnerID != "owner-managed" || created.Environment.Retention.RunID != "run_external_managed" {
		t.Fatalf("managed Environment retention = %+v", created.Environment.Retention)
	}
	managed, ok, err := service.Isolation.GetByEnvironment(created.Environment.ID)
	if err != nil || !ok {
		t.Fatalf("managed metadata lookup: ok=%v err=%v", ok, err)
	}
	if managed.ID != created.ManagedWorktree.ID || !sameTestPath(managed.Root, created.Environment.Root) {
		t.Fatalf("managed identity mismatch: stored=%+v created=%+v", managed, created)
	}
	if got := gitTemporaryOutput(t, workspaceRoot, "branch", "--show-current"); got != branchBefore {
		t.Fatalf("source branch changed: got %q want %q", got, branchBefore)
	}
	if got := gitTemporaryOutput(t, workspaceRoot, "rev-parse", "HEAD"); got != headBefore {
		t.Fatalf("source HEAD changed: got %q want %q", got, headBefore)
	}

	if _, err := service.PromoteTemporaryEnvironment(created.Environment.ID, "owner-managed"); err != nil {
		t.Fatal(err)
	}
	managedAfterPromotion, ok, err := service.Isolation.GetByEnvironment(created.Environment.ID)
	if err != nil || !ok || managedAfterPromotion.ID != managed.ID || managedAfterPromotion.Branch != managed.Branch {
		t.Fatalf("promotion changed managed worktree metadata: before=%+v after=%+v ok=%v err=%v", managed, managedAfterPromotion, ok, err)
	}

	writerOwner := "test-cleanup-managed"
	if _, err := service.Environments.AcquireWriter(created.Environment.ID, writerOwner); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Isolation.Destroy(context.Background(), created.Environment.ID, writerOwner, true); err != nil {
		t.Fatal(err)
	}
}

func TestTemporaryEnvironmentPromotionRequiresOwnerAndPreservesContext(t *testing.T) {
	service := New(filepath.Join(t.TempDir(), "state.json"))
	workspaceRoot := t.TempDir()
	workspace, err := service.Workspaces.Add(workspaceRoot, "temporary-promote")
	if err != nil {
		t.Fatal(err)
	}
	created, err := service.CreateTemporaryEnvironment(context.Background(), "agent", model.TemporaryEnvironmentCreateRequest{
		WorkspaceID: workspace.ID,
		Name:        "promote-me",
		OwnerID:     "owner-promote",
		TTLSeconds:  3600,
		SessionID:   "session-promote",
		RunID:       "run-promote",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := service.Memory.EnvironmentWrite(created.Environment.ID, "private", "keep-me"); err != nil {
		t.Fatal(err)
	}
	if _, err := service.PromoteTemporaryEnvironment(created.Environment.ID, "wrong-owner"); err == nil {
		t.Fatal("wrong owner promotion unexpectedly succeeded")
	}
	promoted, err := service.PromoteTemporaryEnvironment(created.Environment.ID, "owner-promote")
	if err != nil {
		t.Fatal(err)
	}
	if promoted.Retention.Persistence != model.PersistenceDurable || promoted.Retention.OwnerID != "" || promoted.Retention.ExpiresAt != nil || promoted.Retention.RunID != "" || promoted.Retention.SessionID != "" {
		t.Fatalf("promoted retention = %+v", promoted.Retention)
	}
	stored, err := service.Environments.Get(created.Environment.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.ID != created.Environment.ID || stored.WorkspaceID != created.Environment.WorkspaceID || !sameTestPath(stored.Root, created.Environment.Root) {
		t.Fatalf("promotion changed Environment identity: before=%+v after=%+v", created.Environment, stored)
	}
	value, err := service.Memory.EnvironmentRead(created.Environment.ID, "private")
	if err != nil || value.Value != "keep-me" {
		t.Fatalf("promotion changed private Memory: value=%+v err=%v", value, err)
	}
}

func initTemporaryEnvironmentGitWorkspace(t *testing.T, root string) {
	t.Helper()
	for _, args := range [][]string{{"init"}, {"config", "user.email", "temporary-env@example.invalid"}, {"config", "user.name", "Temporary Env Test"}} {
		if out, err := exec.Command("git", append([]string{"-C", root}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %s failed: %v\n%s", strings.Join(args, " "), err, out)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("temporary environment\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"add", "README.md"}, {"commit", "-m", "init"}} {
		if out, err := exec.Command("git", append([]string{"-C", root}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %s failed: %v\n%s", strings.Join(args, " "), err, out)
		}
	}
}

func gitTemporaryOutput(t *testing.T, root string, args ...string) string {
	t.Helper()
	out, err := exec.Command("git", append([]string{"-C", root}, args...)...).CombinedOutput()
	if err != nil {
		t.Fatalf("git %s failed: %v\n%s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}

func sameTestPath(a, b string) bool {
	return pathutil.Same(a, b)
}

func TestTemporaryEnvironmentStatusShowsNotDueWithoutMutation(t *testing.T) {
	service := New(filepath.Join(t.TempDir(), "state.json"))
	workspace, err := service.Workspaces.Add(t.TempDir(), "temporary-status")
	if err != nil {
		t.Fatal(err)
	}
	created, err := service.CreateTemporaryEnvironment(context.Background(), "agent", model.TemporaryEnvironmentCreateRequest{
		WorkspaceID: workspace.ID,
		Name:        "not-due",
		OwnerID:     "owner-status",
		TTLSeconds:  3600,
	})
	if err != nil {
		t.Fatal(err)
	}
	status, err := service.TemporaryEnvironmentStatus(context.Background(), created.Environment.ID)
	if err != nil {
		t.Fatal(err)
	}
	if status.CleanupEligible || status.CleanupState != model.RetentionCleanupNotDue || !hasString(status.Blockers, "retention_not_expired") {
		t.Fatalf("not-due status = %+v", status)
	}
	stored, err := service.Environments.Get(created.Environment.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Retention.Persistence != model.PersistenceTemporary || stored.Retention.OwnerID != "owner-status" {
		t.Fatalf("status mutated Environment: %+v", stored.Retention)
	}
}

func TestTemporaryEnvironmentTTLIsFuture(t *testing.T) {
	service := New(filepath.Join(t.TempDir(), "state.json"))
	workspace, err := service.Workspaces.Add(t.TempDir(), "temporary-ttl")
	if err != nil {
		t.Fatal(err)
	}
	before := time.Now().UTC()
	created, err := service.CreateTemporaryEnvironment(context.Background(), "agent", model.TemporaryEnvironmentCreateRequest{
		WorkspaceID: workspace.ID,
		Name:        "ttl",
		OwnerID:     "owner-ttl",
		TTLSeconds:  2,
	})
	if err != nil {
		t.Fatal(err)
	}
	if created.Environment.Retention.ExpiresAt == nil || !created.Environment.Retention.ExpiresAt.After(before) {
		t.Fatalf("expires_at = %+v, want after %s", created.Environment.Retention.ExpiresAt, before)
	}
}
