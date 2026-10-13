package app

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"ai-dev-manager-v2/internal/isolation"
	"ai-dev-manager-v2/internal/model"
)

func TestAdminManagedWorktreeAlwaysTemporaryAndPromotionRefused(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git unavailable")
	}
	src := appTestGitRepo(t)
	s := New(filepath.Join(t.TempDir(), "adm", "state.json"))
	ws, err := s.Workspaces.Add(src, "src")
	if err != nil {
		t.Fatal(err)
	}
	first, err := s.CreateManagedWorktree(context.Background(), ws.ID, "admin", isolation.CreateOptions{BranchName: "test/admin-temp", BaseRef: "HEAD"})
	if err != nil {
		t.Fatal(err)
	}
	defer appGitCommand(src, "worktree", "remove", "--force", first.ManagedWorktree.Root)
	second, err := s.CreateManagedWorktreeFromEnvironment(context.Background(), first.Environment.ID, "nested", isolation.CreateOptions{BranchName: "test/admin-nested-temp", BaseRef: "HEAD"})
	if err != nil {
		t.Fatal(err)
	}
	defer appGitCommand(src, "worktree", "remove", "--force", second.ManagedWorktree.Root)
	for _, created := range []isolation.CreateResult{first, second} {
		ret := created.Environment.Retention
		if ret.Persistence != model.PersistenceTemporary || ret.ExpiresAt == nil || !ret.ExpiresAt.After(time.Now()) || ret.OwnerID == "" {
			t.Fatalf("admin worktree should be temporary: %+v", ret)
		}
		if _, err := s.PromoteTemporaryEnvironment(created.Environment.ID, ret.OwnerID); err == nil || !strings.Contains(err.Error(), "cannot be promoted") {
			t.Fatalf("managed worktree promotion should fail: %v", err)
		}
		if _, err := s.PromoteResourceRetention(model.ResourceRetentionUpdateRequest{Kind: model.RetentionResourceEnvironment, ID: created.Environment.ID}); err == nil || !strings.Contains(err.Error(), "cannot be promoted") {
			t.Fatalf("generic durable promotion should fail: %v", err)
		}
	}
}

func TestManagedWorktreeMigrationChangesOnlyLegacyRetention(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git unavailable")
	}
	src := appTestGitRepo(t)
	s := New(filepath.Join(t.TempDir(), "adm", "state.json"))
	ws, err := s.Workspaces.Add(src, "src")
	if err != nil {
		t.Fatal(err)
	}
	ordinary, err := s.Environments.Create(ws.ID, "ordinary", "")
	if err != nil {
		t.Fatal(err)
	}
	legacy, err := s.Isolation.Create(context.Background(), ws.ID, "legacy", isolation.CreateOptions{BranchName: "test/legacy-migrate", BaseRef: "HEAD"})
	if err != nil {
		t.Fatal(err)
	}
	defer appGitCommand(src, "worktree", "remove", "--force", legacy.ManagedWorktree.Root)
	if legacy.Environment.Retention.Persistence != model.PersistenceDurable {
		t.Fatalf("fixture must start durable: %+v", legacy.Environment.Retention)
	}
	before, err := s.MigrateManagedWorktreeRetention(false)
	if err != nil || before.ManagedCount != 1 || len(before.Candidates) != 1 || before.Candidates[0] != legacy.Environment.ID || len(before.Converted) != 0 {
		t.Fatalf("preview should be metadata-only: %+v err=%v", before, err)
	}
	stored, err := s.Environments.Get(legacy.Environment.ID)
	if err != nil || stored.Retention.Persistence != model.PersistenceDurable {
		t.Fatalf("preview changed data: %+v err=%v", stored, err)
	}
	after, err := s.MigrateManagedWorktreeRetention(true)
	if err != nil || len(after.Converted) != 1 {
		t.Fatalf("migration failed: %+v %v", after, err)
	}
	stored, err = s.Environments.Get(legacy.Environment.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Retention.Persistence != model.PersistenceTemporary || stored.Retention.ExpiresAt == nil || stored.Retention.ExpiresAt.Before(time.Now().Add(29*24*time.Hour)) {
		t.Fatalf("missing full grace period: %+v", stored.Retention)
	}
	if _, err := os.Stat(legacy.ManagedWorktree.Root); err != nil {
		t.Fatalf("migration removed worktree: %v", err)
	}
	plain, err := s.Environments.Get(ordinary.ID)
	if err != nil || plain.Retention.Persistence != model.PersistenceDurable {
		t.Fatalf("ordinary Environment changed: %+v %v", plain, err)
	}
	again, err := s.MigrateManagedWorktreeRetention(true)
	if err != nil || len(again.Converted) != 0 {
		t.Fatalf("migration not idempotent: %+v %v", again, err)
	}
	if _, err := s.PromoteResourceRetention(model.ResourceRetentionUpdateRequest{Kind: model.RetentionResourceEnvironment, ID: legacy.Environment.ID}); err == nil {
		t.Fatal("migrated worktree must not be promoted durable")
	}
}
