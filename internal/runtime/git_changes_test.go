package runtime

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ai-dev-manager/internal/model"
)

func TestGitChangeSetPreservesIndexWorkingTreeUntrackedAndBinary(t *testing.T) {
	root, gitPath := initGitRepository(t)
	gitTestRun(t, gitPath, root, "config", "core.autocrlf", "false")
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte("*.tmp\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	originalBinary := []byte{0, 1, 2, 3, 0, 4}
	if err := os.WriteFile(filepath.Join(root, "bin.dat"), originalBinary, 0o644); err != nil {
		t.Fatal(err)
	}
	gitTestRun(t, gitPath, root, "add", ".gitignore", "bin.dat")
	gitTestRun(t, gitPath, root, "commit", "-m", "change-set baseline")

	policy := model.Policy{Mode: string(ModeStandard), AllowedExecutables: []string{"git"}, ToolPaths: map[string]string{"git": gitPath}}
	source := mustNativeWithID(t, "ws_changes_source", root, policy, nil)

	if err := os.WriteFile(filepath.Join(root, "source.txt"), []byte("hello staged\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitTestRun(t, gitPath, root, "add", "source.txt")
	if err := os.WriteFile(filepath.Join(root, "source.txt"), []byte("hello staged\nunstaged tail\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	changedBinary := []byte{0, 9, 8, 7, 0, 6, 5}
	if err := os.WriteFile(filepath.Join(root, "bin.dat"), changedBinary, 0o644); err != nil {
		t.Fatal(err)
	}
	gitTestRun(t, gitPath, root, "add", "bin.dat")
	if err := os.WriteFile(filepath.Join(root, "note.txt"), []byte("untracked\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "secret.tmp"), []byte("ignored\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	set, err := source.GitExportChanges()
	if err != nil {
		t.Fatalf("GitExportChanges() error = %v", err)
	}
	if len(set.StagedPatch) == 0 || len(set.UnstagedPatch) == 0 {
		t.Fatalf("expected staged+unstaged patches, got staged=%d unstaged=%d", len(set.StagedPatch), len(set.UnstagedPatch))
	}
	if len(set.Untracked) != 1 || filepath.Clean(set.Untracked[0].Path) != "note.txt" {
		t.Fatalf("unexpected untracked snapshot: %+v", set.Untracked)
	}
	if strings.Contains(string(set.StagedPatch), "secret.tmp") || strings.Contains(string(set.UnstagedPatch), "secret.tmp") {
		t.Fatal("ignored file leaked into tracked patch")
	}

	created, err := source.GitWorktreeCreate("changes-target", "changes-target")
	if err != nil {
		t.Fatalf("GitWorktreeCreate() error = %v", err)
	}
	target := mustNativeWithID(t, "ws_changes_target", created.Path, policy, nil)
	if err := target.GitApplyChanges(set); err != nil {
		t.Fatalf("GitApplyChanges() error = %v", err)
	}

	status, err := target.GitStatus()
	if err != nil {
		t.Fatal(err)
	}
	byPath := map[string]GitStatusEntry{}
	for _, entry := range status {
		byPath[filepath.Clean(entry.Path)] = entry
	}
	if entry := byPath["source.txt"]; entry.X != "M" || entry.Y != "M" {
		t.Fatalf("partially staged source status = %+v, want MM", entry)
	}
	if entry := byPath["bin.dat"]; entry.X != "M" || entry.Y != " " {
		t.Fatalf("binary staged status = %+v, want M<space>", entry)
	}
	if entry := byPath["note.txt"]; entry.X != "?" || entry.Y != "?" {
		t.Fatalf("untracked status = %+v, want ??", entry)
	}
	if _, ok := byPath["secret.tmp"]; ok {
		t.Fatal("ignored file appeared in target status")
	}
	if _, err := os.Stat(filepath.Join(created.Path, "secret.tmp")); !os.IsNotExist(err) {
		t.Fatalf("ignored file copied to target: %v", err)
	}
	gotBinary, err := os.ReadFile(filepath.Join(created.Path, "bin.dat"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(gotBinary, changedBinary) {
		t.Fatalf("binary data mismatch: got=%v want=%v", gotBinary, changedBinary)
	}
	gotSource, err := os.ReadFile(filepath.Join(created.Path, "source.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(gotSource), "unstaged tail") {
		t.Fatalf("unstaged content missing: %q", gotSource)
	}
}

func TestGitExportChangesRejectsUntrackedSymlink(t *testing.T) {
	root, gitPath := initGitRepository(t)
	policy := model.Policy{Mode: string(ModeStandard), AllowedExecutables: []string{"git"}, ToolPaths: map[string]string{"git": gitPath}}
	runtime := mustNativeWithID(t, "ws_changes_symlink", root, policy, nil)

	if err := os.WriteFile(filepath.Join(root, "target.txt"), []byte("target\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	linkPath := filepath.Join(root, "link.txt")
	if err := os.Symlink("target.txt", linkPath); err != nil {
		t.Skipf("symlink unavailable on this host: %v", err)
	}
	if _, err := runtime.GitExportChanges(); err == nil {
		t.Fatal("untracked symlink unexpectedly exported as a regular file")
	} else {
		assertRuntimeErrorKind(t, err, ErrInvalidPath)
	}
}

func TestGitPushStatusAndForcedWorktreeRemoval(t *testing.T) {
	root, gitPath := initGitRepository(t)
	policy := model.Policy{Mode: string(ModeStandard), AllowedExecutables: []string{"git"}, ToolPaths: map[string]string{"git": gitPath}}
	runtime := mustNativeWithID(t, "ws_push_status", root, policy, nil)

	status, err := runtime.GitPushStatus()
	if err != nil {
		t.Fatal(err)
	}
	if status.HasUpstream || status.Ahead != 0 {
		t.Fatalf("unexpected initial push status: %+v", status)
	}

	remote := filepath.Join(t.TempDir(), "remote.git")
	gitTestRun(t, gitPath, root, "init", "--bare", remote)
	gitTestRun(t, gitPath, root, "remote", "add", "origin", remote)
	gitTestRun(t, gitPath, root, "push", "-u", "origin", "main")
	status, err = runtime.GitPushStatus()
	if err != nil {
		t.Fatal(err)
	}
	if !status.HasUpstream || status.Ahead != 0 || status.Upstream == "" {
		t.Fatalf("pushed status = %+v", status)
	}

	if err := os.WriteFile(filepath.Join(root, "ahead.txt"), []byte("ahead\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitTestRun(t, gitPath, root, "add", "ahead.txt")
	gitTestRun(t, gitPath, root, "commit", "-m", "ahead")
	status, err = runtime.GitPushStatus()
	if err != nil {
		t.Fatal(err)
	}
	if !status.HasUpstream || status.Ahead != 1 {
		t.Fatalf("ahead status = %+v", status)
	}

	created, err := runtime.GitWorktreeCreate("force-target", "force-target")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(created.Path, "dirty.txt"), []byte("dirty\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := runtime.GitWorktreeRemove("force-target"); err == nil {
		t.Fatal("normal remove unexpectedly removed dirty worktree")
	}
	if err := runtime.GitWorktreeRemoveWithOptions("force-target", true); err != nil {
		t.Fatalf("forced remove error = %v", err)
	}
	if _, err := os.Stat(created.Path); !os.IsNotExist(err) {
		t.Fatalf("forced worktree still exists: %v", err)
	}
}
