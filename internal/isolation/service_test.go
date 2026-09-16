package isolation

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"ai-dev-manager-v2/internal/environment"
	"ai-dev-manager-v2/internal/model"
	"ai-dev-manager-v2/internal/pathutil"
	"ai-dev-manager-v2/internal/store"
	"ai-dev-manager-v2/internal/workspace"
)

func TestNonGitEnvironmentRemainsValidWhenWorktreeIsolationUnavailableForWorkspace(t *testing.T) {
	stateStore := store.New(filepath.Join(t.TempDir(), "adm", "state.json"))
	workspaces := workspace.New(stateStore)
	environments := environment.New(stateStore, workspaces)
	root := t.TempDir()
	ws, err := workspaces.Add(root, "plain")
	if err != nil {
		t.Fatal(err)
	}
	env, err := environments.Create(ws.ID, "plain", "")
	if err != nil {
		t.Fatalf("ordinary non-Git Environment creation must remain valid: %v", err)
	}
	if !pathutil.Same(env.Root, root) {
		t.Fatalf("ordinary Environment root=%q want %q", env.Root, root)
	}
	service := New(stateStore, workspaces, environments)
	if _, err := service.Create(context.Background(), ws.ID, "isolated", "HEAD"); err == nil || !strings.Contains(err.Error(), "not a usable Git worktree") {
		t.Fatalf("Git-specific create should fail locally for non-Git Workspace, got %v", err)
	}
	if err := service.ValidateEnvironment(context.Background(), env); err != nil {
		t.Fatalf("normal in-Workspace Environment must not require managed metadata: %v", err)
	}
}

func TestCreateTwoManagedWorktreesKeepsSourceCheckoutUnchanged(t *testing.T) {
	service, environments, ws, source := newGitIsolationService(t)
	ctx := context.Background()
	beforeBranch := gitRun(t, source, "branch", "--show-current")
	beforeHead := gitRun(t, source, "rev-parse", "HEAD")
	beforeFile, err := os.ReadFile(filepath.Join(source, "tracked.txt"))
	if err != nil {
		t.Fatal(err)
	}

	first, err := service.Create(ctx, ws.ID, "lane-a", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	second, err := service.Create(ctx, ws.ID, "lane-b", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	defer removeManagedWorktreeForTest(t, source, first.ManagedWorktree)
	defer removeManagedWorktreeForTest(t, source, second.ManagedWorktree)

	if first.ManagedWorktree.ID == second.ManagedWorktree.ID || first.Environment.ID == second.Environment.ID {
		t.Fatalf("managed identities must be distinct: first=%+v second=%+v", first, second)
	}
	if samePath(first.ManagedWorktree.Root, second.ManagedWorktree.Root) || first.ManagedWorktree.Branch == second.ManagedWorktree.Branch {
		t.Fatalf("managed roots/branches must be isolated: first=%+v second=%+v", first.ManagedWorktree, second.ManagedWorktree)
	}
	ownedRoot := service.ownedRoot()
	for _, item := range []model.ManagedWorktree{first.ManagedWorktree, second.ManagedWorktree} {
		if !within(ownedRoot, item.Root) {
			t.Fatalf("managed root %s escaped ADM-owned root %s", item.Root, ownedRoot)
		}
		if err := service.ValidateEnvironment(ctx, mustEnvironment(t, environments, item.EnvironmentID)); err != nil {
			t.Fatalf("managed worktree %s failed validation: %v", item.ID, err)
		}
	}
	isolatedMarker := filepath.Join(first.ManagedWorktree.Root, "lane-a-only.txt")
	if err := os.WriteFile(isolatedMarker, []byte("lane-a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(second.ManagedWorktree.Root, "lane-a-only.txt")); !os.IsNotExist(err) {
		t.Fatalf("write in first managed root leaked into second root: %v", err)
	}
	if _, err := os.Stat(filepath.Join(source, "lane-a-only.txt")); !os.IsNotExist(err) {
		t.Fatalf("write in managed root leaked into source checkout: %v", err)
	}
	if got := gitRun(t, source, "branch", "--show-current"); got != beforeBranch {
		t.Fatalf("source branch changed: got %q want %q", got, beforeBranch)
	}
	if got := gitRun(t, source, "rev-parse", "HEAD"); got != beforeHead {
		t.Fatalf("source HEAD changed: got %q want %q", got, beforeHead)
	}
	afterFile, err := os.ReadFile(filepath.Join(source, "tracked.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(afterFile) != string(beforeFile) {
		t.Fatalf("source tracked file changed: before=%q after=%q", beforeFile, afterFile)
	}
	if _, err := environments.Remove(first.Environment.ID); err == nil || !strings.Contains(err.Error(), "managed worktree") {
		t.Fatalf("generic Environment removal must refuse managed roots, got %v", err)
	}
}

func TestCreateFromEnvironmentFetchesUpstreamWithoutTouchingDirtySource(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not available")
	}
	workspaceRoot := t.TempDir()
	remote := filepath.Join(t.TempDir(), "remote.git")
	if err := os.MkdirAll(remote, 0o755); err != nil {
		t.Fatal(err)
	}
	gitRun(t, remote, "init", "--bare")

	seed := filepath.Join(t.TempDir(), "seed")
	if err := os.MkdirAll(seed, 0o755); err != nil {
		t.Fatal(err)
	}
	gitRun(t, seed, "init", "-b", "main")
	gitRun(t, seed, "config", "user.email", "adm-test@example.invalid")
	gitRun(t, seed, "config", "user.name", "ADM Test")
	if err := os.WriteFile(filepath.Join(seed, "tracked.txt"), []byte("remote-v1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, seed, "add", "tracked.txt")
	gitRun(t, seed, "commit", "-m", "remote v1")
	gitRun(t, seed, "remote", "add", "origin", remote)
	gitRun(t, seed, "push", "-u", "origin", "main")

	source := filepath.Join(workspaceRoot, "work", "front_spa")
	if err := os.MkdirAll(filepath.Dir(source), 0o755); err != nil {
		t.Fatal(err)
	}
	gitRun(t, workspaceRoot, "clone", "--branch", "main", remote, source)
	gitRun(t, source, "config", "user.email", "adm-test@example.invalid")
	gitRun(t, source, "config", "user.name", "ADM Test")
	sourceHeadBefore := gitRun(t, source, "rev-parse", "HEAD")

	updater := filepath.Join(t.TempDir(), "updater")
	gitRun(t, filepath.Dir(updater), "clone", "--branch", "main", remote, updater)
	gitRun(t, updater, "config", "user.email", "adm-test@example.invalid")
	gitRun(t, updater, "config", "user.name", "ADM Test")
	if err := os.WriteFile(filepath.Join(updater, "tracked.txt"), []byte("remote-v2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, updater, "add", "tracked.txt")
	gitRun(t, updater, "commit", "-m", "remote v2")
	remoteHead := gitRun(t, updater, "rev-parse", "HEAD")
	gitRun(t, updater, "push", "origin", "main")

	if err := os.WriteFile(filepath.Join(source, "tracked.txt"), []byte("local-dirty\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	stateStore := store.New(filepath.Join(t.TempDir(), "adm", "state.json"))
	workspaces := workspace.New(stateStore)
	ws, err := workspaces.Add(workspaceRoot, "projects")
	if err != nil {
		t.Fatal(err)
	}
	environments := environment.New(stateStore, workspaces)
	sourceEnvironment, err := environments.Create(ws.ID, "front_spa", source)
	if err != nil {
		t.Fatal(err)
	}
	service := New(stateStore, workspaces, environments)
	created, err := service.CreateFromEnvironment(context.Background(), sourceEnvironment.ID, "agent-task", "")
	if err != nil {
		t.Fatal(err)
	}
	defer removeManagedWorktreeForTest(t, source, created.ManagedWorktree)

	if created.ManagedWorktree.SourceEnvironmentID != sourceEnvironment.ID || !samePath(created.ManagedWorktree.SourceRoot, source) || created.ManagedWorktree.WorkspaceID != ws.ID {
		t.Fatalf("managed source lineage = %+v", created.ManagedWorktree)
	}
	if created.ManagedWorktree.BaseCommit != remoteHead {
		t.Fatalf("managed base = %s; want fetched upstream %s", created.ManagedWorktree.BaseCommit, remoteHead)
	}
	if got := gitRun(t, source, "rev-parse", "HEAD"); got != sourceHeadBefore {
		t.Fatalf("source HEAD changed: got %s want %s", got, sourceHeadBefore)
	}
	if data, err := os.ReadFile(filepath.Join(source, "tracked.txt")); err != nil || string(data) != "local-dirty\n" {
		t.Fatalf("source dirty work changed: data=%q err=%v", data, err)
	}
	if data, err := os.ReadFile(filepath.Join(created.ManagedWorktree.Root, "tracked.txt")); err != nil || strings.TrimSpace(string(data)) != "remote-v2" {
		t.Fatalf("managed worktree did not use fetched upstream: data=%q err=%v", data, err)
	}
}

func TestCreateWithRetentionRollsBackWorktreeWhenEnvironmentPersistenceFails(t *testing.T) {
	service, environments, ws, source := newGitIsolationService(t)
	ctx := context.Background()
	branchesBefore := gitRun(t, source, "branch", "--list", "adm/*")
	worktreesBefore := gitRun(t, source, "worktree", "list", "--porcelain")
	service.createManagedEnvironment = func(string, string, string, model.ManagedWorktree, model.ResourceRetention) (model.Environment, model.ManagedWorktree, error) {
		return model.Environment{}, model.ManagedWorktree{}, errors.New("injected managed Environment persistence failure")
	}
	expiresAt := time.Now().UTC().Add(time.Hour)
	_, err := service.CreateWithRetention(ctx, ws.ID, "rollback", "HEAD", model.ResourceRetention{
		Persistence: model.PersistenceTemporary,
		OwnerID:     "owner-rollback",
		ExpiresAt:   &expiresAt,
	})
	if err == nil || !strings.Contains(err.Error(), "injected managed Environment persistence failure") {
		t.Fatalf("expected injected persistence failure, got %v", err)
	}
	environmentItems, err := environments.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(environmentItems) != 0 {
		t.Fatalf("failed managed create left Environment state: %+v", environmentItems)
	}
	managedItems, err := service.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(managedItems) != 0 {
		t.Fatalf("failed managed create left managed metadata: %+v", managedItems)
	}
	if got := gitRun(t, source, "branch", "--list", "adm/*"); got != branchesBefore {
		t.Fatalf("failed managed create left branch: before=%q after=%q", branchesBefore, got)
	}
	if got := gitRun(t, source, "worktree", "list", "--porcelain"); got != worktreesBefore {
		t.Fatalf("failed managed create left worktree registration:\nbefore:\n%s\nafter:\n%s", worktreesBefore, got)
	}
}

func TestDestroyRefusesDirtyByDefaultAndForceRetainsBranch(t *testing.T) {
	service, environments, ws, source := newGitIsolationService(t)
	ctx := context.Background()
	created, err := service.Create(ctx, ws.ID, "dirty", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := environments.AcquireWriter(created.Environment.ID, "writer"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(created.ManagedWorktree.Root, "dirty.txt"), []byte("dirty"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Destroy(ctx, created.Environment.ID, "writer", false); err == nil || !strings.Contains(err.Error(), "dirty worktree") {
		t.Fatalf("dirty destroy must be refused without force, got %v", err)
	}
	result, err := service.Destroy(ctx, created.Environment.ID, "writer", true)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Forced || result.RetainedBranch != created.ManagedWorktree.Branch {
		t.Fatalf("unexpected force destroy result: %+v", result)
	}
	if _, err := os.Stat(created.ManagedWorktree.Root); !os.IsNotExist(err) {
		t.Fatalf("managed root still exists after force destroy: %v", err)
	}
	if _, err := environments.Get(created.Environment.ID); err == nil {
		t.Fatal("managed Environment metadata still exists after destroy")
	}
	if got := gitRun(t, source, "show-ref", "--verify", "--hash", "refs/heads/"+created.ManagedWorktree.Branch); got == "" {
		t.Fatal("managed branch must be retained after force destroy")
	}
}

func TestDestroyRefusesUnpublishedCommitAndRetainsCommittedWork(t *testing.T) {
	service, environments, ws, source := newGitIsolationService(t)
	ctx := context.Background()
	created, err := service.Create(ctx, ws.ID, "committed", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	root := created.ManagedWorktree.Root
	if err := os.WriteFile(filepath.Join(root, "tracked.txt"), []byte("managed commit\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, root, "add", "tracked.txt")
	gitRun(t, root, "commit", "-m", "managed local commit")
	localHead := gitRun(t, root, "rev-parse", "HEAD")
	if localHead == created.ManagedWorktree.BaseCommit {
		t.Fatal("test did not advance managed HEAD")
	}
	if _, err := environments.AcquireWriter(created.Environment.ID, "writer"); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Destroy(ctx, created.Environment.ID, "writer", false); err == nil || !strings.Contains(err.Error(), "not present in any remote-tracking ref") {
		t.Fatalf("unpublished local commit must block default destroy, got %v", err)
	}
	if _, err := service.Destroy(ctx, created.Environment.ID, "writer", true); err != nil {
		t.Fatal(err)
	}
	if got := gitRun(t, source, "rev-parse", created.ManagedWorktree.Branch); got != localHead {
		t.Fatalf("retained branch lost local commit: got %s want %s", got, localHead)
	}
}

func TestCleanDestroyNeedsWriterAndRetainsBranch(t *testing.T) {
	service, environments, ws, source := newGitIsolationService(t)
	ctx := context.Background()
	created, err := service.Create(ctx, ws.ID, "clean", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Destroy(ctx, created.Environment.ID, "wrong", false); err == nil {
		t.Fatal("destroy without matching writer must fail")
	}
	if _, err := environments.AcquireWriter(created.Environment.ID, "writer"); err != nil {
		t.Fatal(err)
	}
	result, err := service.Destroy(ctx, created.Environment.ID, "writer", false)
	if err != nil {
		t.Fatal(err)
	}
	if result.Forced {
		t.Fatalf("clean destroy unexpectedly reported force: %+v", result)
	}
	gitRun(t, source, "show-ref", "--verify", "refs/heads/"+created.ManagedWorktree.Branch)
}

func TestValidationDetectsManagedBranchTamper(t *testing.T) {
	service, _, ws, source := newGitIsolationService(t)
	ctx := context.Background()
	created, err := service.Create(ctx, ws.ID, "tamper", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		_, _ = gitCommand(source, "worktree", "remove", "--force", created.ManagedWorktree.Root)
	}()
	gitRun(t, created.ManagedWorktree.Root, "checkout", "-b", "tampered-branch")
	if err := service.ValidateEnvironment(ctx, created.Environment); err == nil || !strings.Contains(err.Error(), "branch changed") {
		t.Fatalf("managed branch tamper must be detected, got %v", err)
	}
}

func TestSettingsRegistersProtectedWorkspaceAndFollowsRoot(t *testing.T) {
	stateStore := store.New(filepath.Join(t.TempDir(), "adm", "state.json"))
	workspaces := workspace.New(stateStore)
	environments := environment.New(stateStore, workspaces)
	service := New(stateStore, workspaces, environments)

	settings, err := service.UpdateSettings("", "")
	if err != nil {
		t.Fatal(err)
	}
	if settings.WorkspaceID == "" || settings.BranchPrefix != "adm/" {
		t.Fatalf("default settings = %+v", settings)
	}
	if info, err := os.Stat(settings.Root); err != nil || !info.IsDir() {
		t.Fatalf("default worktree root missing: info=%v err=%v", info, err)
	}
	managedWorkspace, err := workspaces.Get(settings.WorkspaceID)
	if err != nil {
		t.Fatal(err)
	}
	if managedWorkspace.SystemKind != model.WorkspaceSystemKindManagedWorktreeRoot || !samePath(managedWorkspace.Path, settings.Root) {
		t.Fatalf("system workspace = %+v; settings=%+v", managedWorkspace, settings)
	}
	if _, err := workspaces.Remove(settings.WorkspaceID); err == nil || !strings.Contains(err.Error(), "system-managed") {
		t.Fatalf("system workspace removal should be refused, got %v", err)
	}

	customRoot := filepath.Join(t.TempDir(), "custom-worktrees")
	updated, err := service.UpdateSettings(customRoot, "agent/")
	if err != nil {
		t.Fatal(err)
	}
	if updated.WorkspaceID != settings.WorkspaceID || updated.BranchPrefix != "agent/" {
		t.Fatalf("updated settings = %+v; want same workspace %s and agent/ prefix", updated, settings.WorkspaceID)
	}
	managedWorkspace, err = workspaces.Get(settings.WorkspaceID)
	if err != nil {
		t.Fatal(err)
	}
	if !samePath(managedWorkspace.Path, updated.Root) || managedWorkspace.SystemKind != model.WorkspaceSystemKindManagedWorktreeRoot {
		t.Fatalf("system workspace did not follow configured root: workspace=%+v settings=%+v", managedWorkspace, updated)
	}
}

func TestCreateUsesConfiguredRootAndBranchPrefixAndRootChangeIsBlockedWhileActive(t *testing.T) {
	service, _, ws, source := newGitIsolationService(t)
	customRoot := filepath.Join(t.TempDir(), "worktrees")
	settings, err := service.UpdateSettings(customRoot, "agent/")
	if err != nil {
		t.Fatal(err)
	}
	created, err := service.Create(context.Background(), ws.ID, "configured", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	defer removeManagedWorktreeForTest(t, source, created.ManagedWorktree)
	if !within(settings.Root, created.ManagedWorktree.Root) {
		t.Fatalf("managed root %s escaped configured root %s", created.ManagedWorktree.Root, settings.Root)
	}
	if !strings.HasPrefix(created.ManagedWorktree.Branch, "agent/") {
		t.Fatalf("managed branch %q did not use configured prefix", created.ManagedWorktree.Branch)
	}
	if _, err := service.UpdateSettings(filepath.Join(t.TempDir(), "other-root"), "agent/"); err == nil || !strings.Contains(err.Error(), "managed worktree") {
		t.Fatalf("root change with active managed worktree should fail, got %v", err)
	}
	if updated, err := service.UpdateSettings(settings.Root, "next/"); err != nil || updated.BranchPrefix != "next/" {
		t.Fatalf("branch-only settings change failed: settings=%+v err=%v", updated, err)
	}
}

func newGitIsolationService(t *testing.T) (*Service, *environment.Service, model.Workspace, string) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not available")
	}
	source := filepath.Join(t.TempDir(), "source")
	if err := os.MkdirAll(source, 0o755); err != nil {
		t.Fatal(err)
	}
	gitRun(t, source, "init", "-b", "main")
	gitRun(t, source, "config", "user.email", "adm-test@example.invalid")
	gitRun(t, source, "config", "user.name", "ADM Test")
	if err := os.WriteFile(filepath.Join(source, "tracked.txt"), []byte("base\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, source, "add", "tracked.txt")
	gitRun(t, source, "commit", "-m", "base")

	stateStore := store.New(filepath.Join(t.TempDir(), "adm", "state.json"))
	workspaces := workspace.New(stateStore)
	ws, err := workspaces.Add(source, "git-source")
	if err != nil {
		t.Fatal(err)
	}
	environments := environment.New(stateStore, workspaces)
	return New(stateStore, workspaces, environments), environments, ws, source
}

func mustEnvironment(t *testing.T, environments *environment.Service, id string) model.Environment {
	t.Helper()
	env, err := environments.Get(id)
	if err != nil {
		t.Fatal(err)
	}
	return env
}

func removeManagedWorktreeForTest(t *testing.T, source string, managed model.ManagedWorktree) {
	t.Helper()
	_, _ = gitCommand(source, "worktree", "remove", "--force", managed.Root)
	_, _ = gitCommand(source, "branch", "-D", managed.Branch)
}

func gitRun(t *testing.T, dir string, args ...string) string {
	t.Helper()
	out, err := gitCommand(dir, args...)
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(out)
}

func gitCommand(dir string, args ...string) (string, error) {
	git, err := exec.LookPath("git")
	if err != nil {
		return "", err
	}
	fullArgs := append([]string{"-C", dir}, args...)
	cmd := exec.Command(git, fullArgs...)
	out, err := cmd.CombinedOutput()
	return string(out), err
}
