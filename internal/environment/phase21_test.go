package environment

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"ai-dev-manager/internal/controlplane"
)

func TestManagerInspectBaseFactsWriterAndStaleState(t *testing.T) {
	ctx := context.Background()
	configRoot := t.TempDir()
	repo, gitPath := initEnvironmentGitRepo(t)
	service, ws := setupEnvironmentService(t, configRoot, repo, gitPath)
	manager := newTestManager(t, configRoot, service)

	created, err := manager.Create(ctx, CreateRequest{WorkspaceID: ws.ID, Name: "phase21-state"})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	firstWriter, err := manager.AcquireWriter(ctx, created.Environment.ID, "owner-a")
	if err != nil {
		t.Fatalf("AcquireWriter(owner-a) error = %v", err)
	}
	if firstWriter.Writer == nil || firstWriter.Writer.Owner != "owner-a" {
		t.Fatalf("writer lease = %+v", firstWriter.Writer)
	}
	acquiredAt := firstWriter.Writer.AcquiredAt
	firstSeen := firstWriter.Writer.LastSeenAt
	manager.now = func() time.Time { return firstSeen.Add(time.Minute) }
	renewed, err := manager.AcquireWriter(ctx, created.Environment.ID, "owner-a")
	if err != nil {
		t.Fatalf("AcquireWriter(same owner) error = %v", err)
	}
	if renewed.Writer == nil || !renewed.Writer.AcquiredAt.Equal(acquiredAt) || !renewed.Writer.LastSeenAt.After(firstSeen) {
		t.Fatalf("writer renew = %+v", renewed.Writer)
	}
	if _, err := manager.AcquireWriter(ctx, created.Environment.ID, "owner-b"); err == nil || !isEnvironmentCode(err, ErrWriterConflict) {
		t.Fatalf("second owner acquire error = %v, want writer_conflict", err)
	}

	// Reconstruct the Control Plane/Manager to prove the writer is persisted,
	// not daemon-process memory.
	service2, err := serviceForEnvironmentRoot(configRoot)
	if err != nil {
		t.Fatal(err)
	}
	manager2 := newTestManager(t, configRoot, service2)
	persisted, err := manager2.store.Get(created.Environment.ID)
	if err != nil {
		t.Fatal(err)
	}
	if persisted.Writer == nil || persisted.Writer.Owner != "owner-a" {
		t.Fatalf("persisted writer = %+v", persisted.Writer)
	}

	// Advance base by one commit: fact only, no synchronization hint yet.
	if err := os.WriteFile(filepath.Join(repo, "base-01.txt"), []byte("1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitEnvRun(t, gitPath, repo, "add", "base-01.txt")
	gitEnvRun(t, gitPath, repo, "commit", "-m", "base 01")
	activityBeforeInspect := persisted.LastActivityAt
	manager2.now = func() time.Time { return activityBeforeInspect.Add(time.Hour) }
	inspected, err := manager2.Inspect(ctx, created.Environment.ID)
	if err != nil {
		t.Fatalf("Inspect(behind one) error = %v", err)
	}
	if got := factInt(t, inspected.Facts, "behind"); got != 1 {
		t.Fatalf("behind = %d, want 1; facts=%+v", got, inspected.Facts)
	}
	if got := factInt(t, inspected.Facts, "ahead"); got != 0 {
		t.Fatalf("ahead = %d, want 0", got)
	}
	if len(inspected.Hints) != 0 {
		t.Fatalf("small behind unexpectedly produced hint: %+v", inspected.Hints)
	}
	persistedAfterInspect, err := manager2.store.Get(created.Environment.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !persistedAfterInspect.LastActivityAt.Equal(activityBeforeInspect) {
		t.Fatalf("Inspect refreshed activity: before=%v after=%v", activityBeforeInspect, persistedAfterInspect.LastActivityAt)
	}

	// Reach the significant-behind threshold without changing the Environment.
	for i := 2; i <= 10; i++ {
		name := filepath.Join(repo, "base-"+twoDigits(i)+".txt")
		if err := os.WriteFile(name, []byte("base\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		gitEnvRun(t, gitPath, repo, "add", filepath.Base(name))
		gitEnvRun(t, gitPath, repo, "commit", "-m", "base advances")
	}
	inspected, err = manager2.Inspect(ctx, created.Environment.ID)
	if err != nil {
		t.Fatalf("Inspect(behind ten) error = %v", err)
	}
	if got := factInt(t, inspected.Facts, "behind"); got != 10 {
		t.Fatalf("behind = %d, want 10", got)
	}
	if len(inspected.Hints) != 1 || inspected.Hints[0].Code != "base_sync_may_need_confirmation" {
		t.Fatalf("significant behind hints = %+v", inspected.Hints)
	}

	// Add a unique Environment commit too: relation is now divergent, still at
	// most one soft synchronization hint.
	if err := os.WriteFile(filepath.Join(created.Environment.WorktreePath, "env-only.txt"), []byte("env\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitEnvRun(t, gitPath, created.Environment.WorktreePath, "add", "env-only.txt")
	gitEnvRun(t, gitPath, created.Environment.WorktreePath, "commit", "-m", "env advances")
	inspected, err = manager2.Inspect(ctx, created.Environment.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !factBool(t, inspected.Facts, "diverged") || factInt(t, inspected.Facts, "ahead") != 1 {
		t.Fatalf("divergence facts = %+v", inspected.Facts)
	}
	if len(inspected.Hints) != 1 {
		t.Fatalf("divergence should produce one hint, got %+v", inspected.Hints)
	}

	// Dirty is also observational fact.
	if err := os.WriteFile(filepath.Join(created.Environment.WorktreePath, "dirty.txt"), []byte("dirty\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	inspected, err = manager2.Inspect(ctx, created.Environment.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !factBool(t, inspected.Facts, "dirty") {
		t.Fatalf("dirty fact missing: %+v", inspected.Facts)
	}

	// Make the stored activity old while keeping the writer lease. Inspect must
	// report stale but must not release the writer or delete the Environment.
	old := manager2.now().Add(-8 * 24 * time.Hour)
	staleEnv, err := manager2.store.Get(created.Environment.ID)
	if err != nil {
		t.Fatal(err)
	}
	staleEnv.LastActivityAt = old
	staleEnv.Writer.LastSeenAt = old
	if err := manager2.store.Put(staleEnv); err != nil {
		t.Fatal(err)
	}
	inspected, err = manager2.Inspect(ctx, created.Environment.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !factBool(t, inspected.Facts, "stale") || inspected.Environment.Writer == nil || inspected.Environment.Writer.Owner != "owner-a" {
		t.Fatalf("stale/writer result = %+v facts=%+v", inspected.Environment.Writer, inspected.Facts)
	}

	if _, err := manager2.ReleaseWriter(created.Environment.ID, "owner-b", false); err == nil || !isEnvironmentCode(err, ErrWriterNotOwner) {
		t.Fatalf("wrong owner release error = %v, want writer_not_owner", err)
	}
	released, err := manager2.ReleaseWriter(created.Environment.ID, "", true)
	if err != nil {
		t.Fatalf("force ReleaseWriter() error = %v", err)
	}
	if released.Writer != nil {
		t.Fatalf("force release left writer: %+v", released.Writer)
	}
}

func TestManagerDestroyRejectsActiveWriterUnlessForced(t *testing.T) {
	ctx := context.Background()
	configRoot := t.TempDir()
	repo, gitPath := initEnvironmentGitRepo(t)
	service, ws := setupEnvironmentService(t, configRoot, repo, gitPath)
	manager := newTestManager(t, configRoot, service)

	created, err := manager.Create(ctx, CreateRequest{WorkspaceID: ws.ID, Name: "writer-protected-destroy"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := manager.AcquireWriter(ctx, created.Environment.ID, "owner-a"); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.Destroy(ctx, created.Environment.ID, false); err == nil || !isEnvironmentCode(err, ErrUnsafeDestroy) {
		t.Fatalf("active-writer destroy error = %v, want unsafe_destroy", err)
	}
	if _, err := os.Stat(created.Environment.WorktreePath); err != nil {
		t.Fatalf("normal destroy removed active-writer worktree: %v", err)
	}
	if _, err := manager.Destroy(ctx, created.Environment.ID, true); err != nil {
		t.Fatalf("forced active-writer destroy error = %v", err)
	}
	if _, err := os.Stat(created.Environment.WorktreePath); !os.IsNotExist(err) {
		t.Fatalf("forced destroy left active-writer worktree: %v", err)
	}
}

func TestManagerInspectMissingBaseRefIsNonFatal(t *testing.T) {
	ctx := context.Background()
	configRoot := t.TempDir()
	repo, gitPath := initEnvironmentGitRepo(t)
	service, ws := setupEnvironmentService(t, configRoot, repo, gitPath)
	manager := newTestManager(t, configRoot, service)

	gitEnvRun(t, gitPath, repo, "branch", "temporary-base")
	created, err := manager.Create(ctx, CreateRequest{WorkspaceID: ws.ID, Name: "missing-base", Base: "temporary-base"})
	if err != nil {
		t.Fatal(err)
	}
	gitEnvRun(t, gitPath, repo, "branch", "-D", "temporary-base")
	inspected, err := manager.Inspect(ctx, created.Environment.ID)
	if err != nil {
		t.Fatalf("Inspect(missing base) should remain usable: %v", err)
	}
	if !hasWarningCode(inspected.Warnings, "base_ref_unavailable") || factBool(t, inspected.Facts, "current_base_available") {
		t.Fatalf("missing base result warnings=%+v facts=%+v", inspected.Warnings, inspected.Facts)
	}
	if inspected.Environment.BaseRef != "temporary-base" || inspected.Environment.BaseCommit != created.Environment.BaseCommit {
		t.Fatalf("Inspect rewrote recorded base: before=%+v after=%+v", created.Environment, inspected.Environment)
	}
}

func serviceForEnvironmentRoot(root string) (*controlplane.Service, error) {
	return controlplane.New(root)
}

func twoDigits(value int) string {
	if value < 10 {
		return "0" + string(rune('0'+value))
	}
	return "10"
}

func factInt(t *testing.T, facts []Fact, code string) int {
	t.Helper()
	for _, fact := range facts {
		if fact.Code != code {
			continue
		}
		switch value := fact.Value.(type) {
		case int:
			return value
		case int64:
			return int(value)
		case float64:
			return int(value)
		}
		t.Fatalf("fact %s has non-numeric value %#v", code, fact.Value)
	}
	t.Fatalf("fact %s not found in %+v", code, facts)
	return 0
}

func factBool(t *testing.T, facts []Fact, code string) bool {
	t.Helper()
	for _, fact := range facts {
		if fact.Code == code {
			value, ok := fact.Value.(bool)
			if !ok {
				t.Fatalf("fact %s has non-bool value %#v", code, fact.Value)
			}
			return value
		}
	}
	t.Fatalf("fact %s not found in %+v", code, facts)
	return false
}

func hasWarningCode(warnings []Warning, code string) bool {
	for _, warning := range warnings {
		if warning.Code == code {
			return true
		}
	}
	return false
}
