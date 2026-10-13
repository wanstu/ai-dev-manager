package isolation

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSafetyProtectsIgnoredLocalFiles(t *testing.T) {
	s, _, ws, source := newGitIsolationService(t)
	if err := os.WriteFile(filepath.Join(source, ".gitignore"), []byte("cache/\n"), 0644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, source, "add", ".gitignore")
	gitRun(t, source, "commit", "-m", "ignore local cache")
	created, err := s.Create(context.Background(), ws.ID, "ignored-data", CreateOptions{BranchName: "test/safety-ignored", BaseRef: "HEAD"})
	if err != nil {
		t.Fatal(err)
	}
	defer removeManagedWorktreeForTest(t, source, created.ManagedWorktree)
	cache := filepath.Join(created.ManagedWorktree.Root, "cache")
	if err := os.MkdirAll(cache, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cache, "private.bin"), []byte("important local contents"), 0644); err != nil {
		t.Fatal(err)
	}
	if result := gitRun(t, created.ManagedWorktree.Root, "status", "--porcelain=v1", "--untracked-files=all"); result != "" {
		t.Fatalf("test ignored data is visible to ordinary git status: %q", result)
	}
	safety, err := s.Safety(context.Background(), created.Environment.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !safety.Dirty || !safety.Ignored {
		t.Fatalf("ignored files must block cleanup even with clean git status: %+v", safety)
	}
}

func TestSafetyVerifiesRemotePublicationAndPrunesDeletedBranch(t *testing.T) {
	s, _, ws, source := newGitIsolationService(t)
	remote := filepath.Join(t.TempDir(), "origin.git")
	gitRun(t, source, "init", "--bare", remote)
	gitRun(t, source, "remote", "add", "origin", remote)
	gitRun(t, source, "push", "origin", "HEAD:refs/heads/main")
	created, err := s.Create(context.Background(), ws.ID, "remote-status", CreateOptions{BranchName: "test/safety-remote", BaseRef: "HEAD"})
	if err != nil {
		t.Fatal(err)
	}
	defer removeManagedWorktreeForTest(t, source, created.ManagedWorktree)
	root := created.ManagedWorktree.Root
	if err := os.WriteFile(filepath.Join(root, "tracked.txt"), []byte("new remote work\n"), 0644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, root, "add", "tracked.txt")
	gitRun(t, root, "commit", "-m", "only local at first")
	before, err := s.Safety(context.Background(), created.Environment.ID)
	if err != nil {
		t.Fatal(err)
	}
	if before.Dirty || !before.Unpublished {
		t.Fatalf("unpublished commit must block cleanup: %+v", before)
	}
	gitRun(t, root, "push", "origin", "HEAD:refs/heads/published")
	afterPush, err := s.Safety(context.Background(), created.Environment.ID)
	if err != nil {
		t.Fatal(err)
	}
	if afterPush.Dirty || afterPush.Unpublished {
		t.Fatalf("remote confirmed published commit should pass: %+v", afterPush)
	}
	// The locally cached origin/published ref still exists immediately after
	// remote deletion. Only a fresh remote fetch can reject it.
	gitRun(t, root, "push", "origin", "--delete", "published")
	// A cached ref for an old, no-longer-configured remote must not count.
	gitRun(t, root, "update-ref", "refs/remotes/obsolete/published", gitRun(t, root, "rev-parse", "HEAD"))
	gitRun(t, root, "config", "remote.origin.skipFetchAll", "true")
	afterDelete, err := s.Safety(context.Background(), created.Environment.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !afterDelete.Unpublished {
		t.Fatalf("stale remote-tracking refs must not bypass safety: %+v", afterDelete)
	}
}

func TestSafetyRefusesCleanupWhenRemoteVerificationFails(t *testing.T) {
	s, envs, ws, source := newGitIsolationService(t)
	remote := filepath.Join(t.TempDir(), "origin.git")
	gitRun(t, source, "init", "--bare", remote)
	gitRun(t, source, "remote", "add", "origin", remote)
	gitRun(t, source, "push", "origin", "HEAD:refs/heads/main")
	created, err := s.Create(context.Background(), ws.ID, "offline", CreateOptions{BranchName: "test/safety-offline", BaseRef: "HEAD"})
	if err != nil {
		t.Fatal(err)
	}
	defer removeManagedWorktreeForTest(t, source, created.ManagedWorktree)
	root := created.ManagedWorktree.Root
	if err := os.WriteFile(filepath.Join(root, "tracked.txt"), []byte("new commit\n"), 0644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, root, "add", "tracked.txt")
	gitRun(t, root, "commit", "-m", "published earlier")
	gitRun(t, root, "push", "origin", "HEAD:refs/heads/published")
	// Keep the old tracking ref but make the remote unreachable.
	gitRun(t, source, "remote", "set-url", "origin", filepath.Join(t.TempDir(), "missing-repository.git"))
	if _, err := s.Safety(context.Background(), created.Environment.ID); err == nil || !strings.Contains(err.Error(), "cannot verify published commits") {
		t.Fatalf("failed remote verification must block safety eligibility, got %v", err)
	}
	if _, err := envs.AcquireWriter(created.Environment.ID, "cleanup"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Destroy(context.Background(), created.Environment.ID, "cleanup", false); err == nil {
		t.Fatal("offline remote must not allow ordinary worktree removal")
	}
	if _, err := os.Stat(root); err != nil {
		t.Fatalf("blocked cleanup should leave worktree intact: %v", err)
	}
}
