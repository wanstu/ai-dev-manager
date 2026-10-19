package sandbox

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateExecutionRequestRejectsCwdOutsideWorktree(t *testing.T) {
	worktree := t.TempDir()
	outside := t.TempDir()
	executable := filepath.Join(worktree, "tool.exe")
	if err := writeExecutableFixture(executable); err != nil {
		t.Fatal(err)
	}
	_, _, _, err := validateExecutionRequest(ExecutionRequest{
		Scope:        "test",
		WorktreeRoot: worktree,
		Executable:   executable,
		Cwd:          outside,
	})
	if err == nil || !strings.Contains(err.Error(), "escapes worktree") {
		t.Fatalf("expected cwd escape rejection, got %v", err)
	}
}

func TestValidateExecutionRequestCanonicalizesContainedPaths(t *testing.T) {
	worktree := t.TempDir()
	sub := filepath.Join(worktree, "sub")
	if err := mkdirFixture(sub); err != nil {
		t.Fatal(err)
	}
	executable := filepath.Join(worktree, "tool.exe")
	if err := writeExecutableFixture(executable); err != nil {
		t.Fatal(err)
	}
	root, exe, cwd, err := validateExecutionRequest(ExecutionRequest{
		Scope:        "test",
		WorktreeRoot: worktree,
		Executable:   executable,
		Cwd:          sub,
	})
	if err != nil {
		t.Fatal(err)
	}
	if root == "" || exe == "" || cwd == "" {
		t.Fatalf("canonical execution paths missing: root=%q exe=%q cwd=%q", root, exe, cwd)
	}
}
