package management_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"ai-dev-manager-v2/internal/app"
	"ai-dev-manager-v2/internal/isolation"
	"ai-dev-manager-v2/internal/management"
)

func TestSnapshotExposesManagedWorktreeIDsWithoutClassifyingOrdinaryEnvironments(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git unavailable")
	}
	root := filepath.Join(t.TempDir(), "source")
	if err := os.MkdirAll(root, 0755); err != nil {
		t.Fatal(err)
	}
	git := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}
	git("init", "-b", "main")
	git("config", "user.name", "ADM Test")
	git("config", "user.email", "adm@example.invalid")
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("source\n"), 0644); err != nil {
		t.Fatal(err)
	}
	git("add", "README.md")
	git("commit", "-m", "base")
	application := app.New(filepath.Join(t.TempDir(), "adm", "state.json"))
	ws, err := application.Workspaces.Add(root, "Source")
	if err != nil {
		t.Fatal(err)
	}
	ordinary, err := application.Environments.Create(ws.ID, "Ordinary", "")
	if err != nil {
		t.Fatal(err)
	}
	created, err := application.CreateManagedWorktree(context.Background(), ws.ID, "Managed", isolation.CreateOptions{
		BranchName: "test/snapshot-managed", BaseRef: "HEAD",
	})
	if err != nil {
		t.Fatal(err)
	}
	defer git("worktree", "remove", "--force", created.ManagedWorktree.Root)
	snapshot, err := management.New(application).Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.ManagedWorktreeEnvironmentIDs) != 1 || snapshot.ManagedWorktreeEnvironmentIDs[0] != created.Environment.ID {
		t.Fatalf("wrong managed worktree IDs: %v (ordinary %s)", snapshot.ManagedWorktreeEnvironmentIDs, ordinary.ID)
	}
}
