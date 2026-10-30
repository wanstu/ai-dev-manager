//go:build windows

package desktop

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"ai-dev-manager-v2/internal/sandbox"
)

func TestSandboxACLTraversalPaths(t *testing.T) {
	got := sandboxACLTraversalPaths(`D:\projects`, `D:\projects\worktrees\ws\wt`)
	want := []string{`D:\projects`, `D:\projects\worktrees`, `D:\projects\worktrees\ws`}
	if len(got) != len(want) {
		t.Fatalf("traversal paths=%v, want %v", got, want)
	}
	for i := range want {
		if !strings.EqualFold(got[i], want[i]) {
			t.Fatalf("traversal paths[%d]=%q, want %q", i, got[i], want[i])
		}
	}
}

func TestSandboxACLBrokerE2E(t *testing.T) {
	if os.Getenv("ADM_SANDBOX_ACL_BROKER_E2E") != "1" {
		t.Skip("set ADM_SANDBOX_ACL_BROKER_E2E=1 for protected SYSTEM broker E2E")
	}
	environmentID := strings.TrimSpace(os.Getenv("ADM_SANDBOX_ACL_BROKER_ENVIRONMENT_ID"))
	if environmentID == "" {
		t.Fatal("ADM_SANDBOX_ACL_BROKER_ENVIRONMENT_ID is required")
	}
	id, workspaceRoot, root, err := localSandboxEnvironment(environmentID)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	profileName, err := sandbox.StableAppContainerProfileName(id)
	if err != nil {
		t.Fatal(err)
	}
	profile, _, err := sandbox.EnsureWindowsAppContainerProfile(profileName)
	if err != nil {
		t.Fatalf("create user AppContainer profile: %v", err)
	}
	traversalRoot, err := sandbox.ResolveEnvironmentTraversalRoot(workspaceRoot, root)
	if err != nil {
		t.Fatal(err)
	}
	ancestorBaseline := map[string]bool{}
	for _, path := range sandboxACLTraversalPaths(traversalRoot, root) {
		granted, grantErr := sandbox.WindowsAppContainerPathAccessGranted(path, profile.SID, sandbox.PathAccessTraverse)
		if grantErr != nil {
			t.Fatal(grantErr)
		}
		ancestorBaseline[path] = granted
	}

	t.Log("prepare-before")
	if _, err := requestSandboxACLBroker(ctx, sandboxACLBrokerRequest{
		Operation:       sandboxACLOpPrepare,
		EnvironmentID:   id,
		ProfileSID:      profile.SID,
		WorkspaceRoot:   workspaceRoot,
		EnvironmentRoot: root,
	}); err != nil {
		t.Fatalf("broker prepare failed: %v", err)
	}
	prepared := sandbox.InspectEnvironmentWorkspaceAccess(id, workspaceRoot, root)
	if !prepared.Prepared {
		t.Fatalf("broker prepare status=%+v", prepared)
	}
	for path, before := range ancestorBaseline {
		granted, grantErr := sandbox.WindowsAppContainerPathAccessGranted(path, profile.SID, sandbox.PathAccessTraverse)
		if grantErr != nil {
			t.Fatal(grantErr)
		}
		if granted != before {
			t.Fatalf("broker changed ancestor traversal ACL on %s: before=%v after=%v", path, before, granted)
		}
	}
	t.Log("prepare-after")

	executable, err := filepath.Abs(os.Args[0])
	if err != nil {
		t.Fatal(err)
	}
	execution, err := sandbox.Execute(ctx, sandbox.ExecutionRequest{
		Scope:           id,
		RuntimeRoot:     filepath.Join(t.TempDir(), "runtime"),
		WorktreeRoot:    root,
		Executable:      executable,
		Args:            []string{"-test.run=^TestSandboxACLBrokerExecutionChildHelper$", "-test.v"},
		Cwd:             root,
		BaseEnvironment: os.Environ(),
		ExtraEnvironment: map[string]string{
			"ADM_SANDBOX_ACTIVE": "1",
		},
		MaxOutputBytes: 16 * 1024,
	})
	if err != nil {
		t.Fatalf("broker-prepared sandbox execution failed: %v; result=%+v", err, execution)
	}
	if !execution.Started || execution.ExitCode != 0 || !strings.Contains(execution.Stdout, "sandbox-broker-execution-active") {
		t.Fatalf("broker-prepared sandbox execution result=%+v", execution)
	}
	t.Log("execution-after")
	t.Log("cleanup-before")

	if _, err := requestSandboxACLBroker(ctx, sandboxACLBrokerRequest{
		Operation:       sandboxACLOpCleanup,
		EnvironmentID:   id,
		ProfileSID:      profile.SID,
		WorkspaceRoot:   workspaceRoot,
		EnvironmentRoot: root,
	}); err != nil {
		t.Fatalf("broker cleanup failed: %v", err)
	}
	cleaned, err := sandbox.CleanupEnvironmentWorkspaceAccess(id, workspaceRoot, root)
	if err != nil {
		t.Fatalf("finalize user AppContainer profile cleanup: %v", err)
	}
	if cleaned.ProfilePresent || cleaned.HasAnyGrant {
		t.Fatalf("broker cleanup status=%+v", cleaned)
	}
	for path, before := range ancestorBaseline {
		granted, grantErr := sandbox.WindowsAppContainerPathAccessGranted(path, profile.SID, sandbox.PathAccessTraverse)
		if grantErr != nil {
			t.Fatal(grantErr)
		}
		if granted != before {
			t.Fatalf("broker cleanup changed ancestor traversal ACL on %s: before=%v after=%v", path, before, granted)
		}
	}
	t.Log("cleanup-after")
}

func TestSandboxACLBrokerExecutionChildHelper(t *testing.T) {
	if os.Getenv("ADM_SANDBOX_ACTIVE") != "1" {
		t.Skip("sandbox broker execution child helper")
	}
	t.Log("sandbox-broker-execution-active")
}

func TestSandboxACLBrokerRejectsMismatchedEnvironmentSIDE2E(t *testing.T) {
	if os.Getenv("ADM_SANDBOX_ACL_BROKER_E2E") != "1" {
		t.Skip("set ADM_SANDBOX_ACL_BROKER_E2E=1 for protected SYSTEM broker E2E")
	}
	environmentID := strings.TrimSpace(os.Getenv("ADM_SANDBOX_ACL_BROKER_ENVIRONMENT_ID"))
	if environmentID == "" {
		t.Fatal("ADM_SANDBOX_ACL_BROKER_ENVIRONMENT_ID is required")
	}
	id, workspaceRoot, root, err := localSandboxEnvironment(environmentID)
	if err != nil {
		t.Fatal(err)
	}
	otherName, err := sandbox.StableAppContainerProfileName(id + "-mismatch")
	if err != nil {
		t.Fatal(err)
	}
	otherProfile, created, err := sandbox.EnsureWindowsAppContainerProfile(otherName)
	if err != nil {
		t.Fatal(err)
	}
	if created {
		defer sandbox.DeleteWindowsAppContainerProfile(otherProfile.Name)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if _, err := requestSandboxACLBroker(ctx, sandboxACLBrokerRequest{
		Operation:       sandboxACLOpPrepare,
		EnvironmentID:   id,
		ProfileSID:      otherProfile.SID,
		WorkspaceRoot:   workspaceRoot,
		EnvironmentRoot: root,
	}); err == nil || !strings.Contains(err.Error(), "does not match Environment") {
		t.Fatalf("mismatched AppContainer SID was not rejected: %v", err)
	}
}

func TestSandboxEnvironmentPreparationUsesBrokerE2E(t *testing.T) {
	if os.Getenv("ADM_SANDBOX_ACL_BROKER_E2E") != "1" {
		t.Skip("set ADM_SANDBOX_ACL_BROKER_E2E=1 for protected SYSTEM broker integration E2E")
	}
	environmentID := strings.TrimSpace(os.Getenv("ADM_SANDBOX_ACL_BROKER_ENVIRONMENT_ID"))
	if environmentID == "" {
		t.Fatal("ADM_SANDBOX_ACL_BROKER_ENVIRONMENT_ID is required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	// Start from a clean profile/grant state so this exercises the complete
	// user-profile -> SYSTEM root ACL broker -> user cleanup lifecycle.
	if current, err := cleanupSandboxEnvironmentPreparationContext(ctx, environmentID); err != nil {
		t.Fatalf("initial cleanup failed: %v; status=%+v", err, current)
	}

	t.Log("integrated-prepare-before")
	prepared, err := enableSandboxEnvironmentPreparationContext(ctx, environmentID)
	if err != nil {
		t.Fatalf("integrated broker preparation failed: %v; status=%+v", err, prepared)
	}
	if !prepared.Prepared || prepared.ProfileSID == "" {
		t.Fatalf("integrated broker preparation status=%+v", prepared)
	}
	t.Log("integrated-prepare-after")

	t.Log("integrated-cleanup-before")
	cleaned, err := cleanupSandboxEnvironmentPreparationContext(ctx, environmentID)
	if err != nil {
		t.Fatalf("integrated broker cleanup failed: %v; status=%+v", err, cleaned)
	}
	if cleaned.ProfilePresent || cleaned.HasAnyGrant {
		t.Fatalf("integrated broker cleanup left persistent state: %+v", cleaned)
	}
	t.Log("integrated-cleanup-after")
}
