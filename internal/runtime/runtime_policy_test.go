package runtime

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStrictModeRejectsCommandProxyEvenWhenAllowlisted(t *testing.T) {
	rt, err := NewWithPolicy(t.TempDir(), []string{"powershell"}, os.Environ(), false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := rt.allowedExecutable("powershell"); err == nil || !strings.Contains(err.Error(), "strict mode") {
		t.Fatalf("expected strict-mode proxy rejection, got %v", err)
	}
	if rt.IsExplicitlyAllowed("powershell") {
		t.Fatal("command proxy must not count as explicitly allowed in strict policy")
	}
}

func TestFullAuthorizationResolvesUnlistedExecutable(t *testing.T) {
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	rt, err := NewWithPolicy(t.TempDir(), nil, os.Environ(), true)
	if err != nil {
		t.Fatal(err)
	}
	resolved, err := rt.allowedExecutable(executable)
	if err != nil {
		t.Fatalf("full authorization should resolve current executable: %v", err)
	}
	if !samePath(filepath.Clean(executable), filepath.Clean(resolved)) {
		t.Fatalf("resolved executable mismatch: want %q got %q", executable, resolved)
	}
	if rt.IsExplicitlyAllowed(executable) {
		t.Fatal("unlisted executable must remain distinguishable for bypass auditing")
	}
}

func TestCommandBlacklistOverridesAllowlistAndFullAuthorization(t *testing.T) {
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	base := filepath.Base(executable)
	rt, err := NewWithExecutionPolicy(t.TempDir(), []string{base}, []string{base}, os.Environ(), true)
	if err != nil {
		t.Fatal(err)
	}
	if !rt.IsBlockedExecutable(base) {
		t.Fatalf("expected %q to be blacklisted", base)
	}
	if _, err := rt.allowedExecutable(base); err == nil || !strings.Contains(err.Error(), "command blacklist") {
		t.Fatalf("blacklist must override allowlist/full authorization, got %v", err)
	}
}

func TestCommandBlacklistTreatsWindowsExeSuffixAsSameCommandName(t *testing.T) {
	rt, err := NewWithExecutionPolicy(t.TempDir(), nil, []string{"pwsh"}, os.Environ(), true)
	if err != nil {
		t.Fatal(err)
	}
	for _, executable := range []string{"pwsh", "PWSH.EXE", filepath.Join(t.TempDir(), "pwsh.exe")} {
		if !rt.IsBlockedExecutable(executable) {
			t.Fatalf("blacklist entry pwsh should block %q", executable)
		}
	}
}

func TestCommandBlacklistAbsolutePathDoesNotOverblockSameBasename(t *testing.T) {
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	rt, err := NewWithExecutionPolicy(t.TempDir(), nil, []string{executable}, os.Environ(), true)
	if err != nil {
		t.Fatal(err)
	}
	if !rt.IsBlockedExecutable(executable) {
		t.Fatalf("absolute path %q should be blacklisted", executable)
	}
	if rt.IsBlockedExecutable(filepath.Base(executable)) {
		t.Fatalf("absolute blacklist entry must not block basename-only requests")
	}
}

func TestGitWorktreeIsReservedEvenWithFullAuthorization(t *testing.T) {
	rt, err := NewWithPolicy(t.TempDir(), []string{"git"}, os.Environ(), true)
	if err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"worktree", "add", "x"},
		{"--no-pager", "worktree", "list"},
		{"-C", ".", "worktree", "remove", "x"},
	} {
		if _, err := rt.PrepareCommand(context.Background(), "git", args, ""); err == nil || !strings.Contains(err.Error(), "environment_worktree_create") {
			t.Fatalf("git %v should be reserved by ADM, got %v", args, err)
		}
	}
	if _, err := rt.PrepareCommand(context.Background(), "git", []string{"status", "--short"}, ""); err != nil {
		t.Fatalf("ordinary git command should remain available: %v", err)
	}
}
