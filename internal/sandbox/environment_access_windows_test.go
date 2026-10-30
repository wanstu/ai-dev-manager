//go:build windows

package sandbox

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnvironmentWorkspaceAccessScopesPersistentACLToEnvironmentRoot(t *testing.T) {
	if os.Getenv("ADM_SANDBOX_PROFILE_E2E") != "1" {
		t.Skip("set ADM_SANDBOX_PROFILE_E2E=1 to exercise AppContainer ACL lifecycle")
	}

	workspace := t.TempDir()
	environment := filepath.Join(workspace, "worktrees", "ws_test", "wt_test")
	sibling := filepath.Join(workspace, "unrelated")
	if err := os.MkdirAll(environment, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(sibling, 0o755); err != nil {
		t.Fatal(err)
	}

	environmentID := t.Name()
	status, err := PrepareEnvironmentWorkspaceAccess(environmentID, workspace, environment)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		_, _ = CleanupEnvironmentWorkspaceAccess(environmentID, workspace, environment)
	}()
	if !status.Prepared {
		t.Fatalf("workspace-aware Environment access was not prepared: %+v", status)
	}

	paths, err := environmentWorkspaceTraversePaths(workspace, environment)
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range paths {
		granted, grantErr := WindowsAppContainerPathAccessGranted(path, status.ProfileSID, PathAccessTraverse)
		if grantErr != nil {
			t.Fatal(grantErr)
		}
		if granted {
			t.Fatalf("persistent Environment preparation unexpectedly mutated ancestor ACL: %s", path)
		}
	}

	siblingTraverse, err := WindowsAppContainerPathAccessGranted(sibling, status.ProfileSID, PathAccessTraverse)
	if err != nil {
		t.Fatal(err)
	}
	if siblingTraverse {
		t.Fatal("Environment preparation unexpectedly granted access to unrelated sibling directory")
	}

	rootGranted, err := WindowsAppContainerPathAccessGranted(environment, status.ProfileSID, PathAccessReadWrite)
	if err != nil {
		t.Fatal(err)
	}
	if !rootGranted {
		t.Fatal("Environment root missing read/write grant")
	}

	cleaned, err := CleanupEnvironmentWorkspaceAccess(environmentID, workspace, environment)
	if err != nil {
		t.Fatal(err)
	}
	if cleaned.ProfilePresent || cleaned.HasAnyGrant {
		t.Fatalf("workspace-aware cleanup left persistent state: %+v", cleaned)
	}
}

func TestEnvironmentRootOnlyPreparationSupportsSandboxExecution(t *testing.T) {
	if os.Getenv("ADM_SANDBOX_PROFILE_E2E") != "1" {
		t.Skip("set ADM_SANDBOX_PROFILE_E2E=1 to exercise AppContainer execution")
	}

	workspace := t.TempDir()
	environment := filepath.Join(workspace, "worktrees", "ws_test", "wt_exec")
	if err := os.MkdirAll(environment, 0o755); err != nil {
		t.Fatal(err)
	}
	environmentID := t.Name()
	status, err := PrepareEnvironmentWorkspaceAccess(environmentID, workspace, environment)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		_, _ = CleanupEnvironmentWorkspaceAccess(environmentID, workspace, environment)
	}()

	for _, path := range mustEnvironmentWorkspaceTraversePaths(t, workspace, environment) {
		granted, grantErr := WindowsAppContainerPathAccessGranted(path, status.ProfileSID, PathAccessTraverse)
		if grantErr != nil {
			t.Fatal(grantErr)
		}
		if granted {
			t.Fatalf("ancestor ACL must remain untouched before sandbox execution: %s", path)
		}
	}

	executable, err := filepath.Abs(os.Args[0])
	if err != nil {
		t.Fatal(err)
	}
	runtimeRoot := filepath.Join(t.TempDir(), "runtime")
	result, err := Execute(context.Background(), ExecutionRequest{
		Scope:           environmentID,
		RuntimeRoot:     runtimeRoot,
		WorktreeRoot:    environment,
		Executable:      executable,
		Args:            []string{"-test.run=^TestEnvironmentRootOnlyChildHelper$", "-test.v"},
		Cwd:             environment,
		BaseEnvironment: os.Environ(),
		MaxOutputBytes:  16 * 1024,
	})
	if err != nil {
		t.Fatalf("root-only prepared sandbox execution failed: %v; result=%+v", err, result)
	}
	if !result.Started || result.ExitCode != 0 {
		t.Fatalf("root-only sandbox execution result=%+v", result)
	}
	if !strings.Contains(result.Stdout, "environment-root-only-sandbox-active") {
		t.Fatalf("sandbox child marker missing: %s", result.Stdout)
	}

	for _, path := range mustEnvironmentWorkspaceTraversePaths(t, workspace, environment) {
		granted, grantErr := WindowsAppContainerPathAccessGranted(path, status.ProfileSID, PathAccessTraverse)
		if grantErr != nil {
			t.Fatal(grantErr)
		}
		if granted {
			t.Fatalf("sandbox execution unexpectedly mutated ancestor ACL: %s", path)
		}
	}
}

func TestEnvironmentRootOnlyChildHelper(t *testing.T) {
	fmt.Println("environment-root-only-sandbox-active")
}

func mustEnvironmentWorkspaceTraversePaths(t *testing.T, workspace, environment string) []string {
	t.Helper()
	paths, err := environmentWorkspaceTraversePaths(workspace, environment)
	if err != nil {
		t.Fatal(err)
	}
	return paths
}
