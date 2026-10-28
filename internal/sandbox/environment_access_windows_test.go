//go:build windows

package sandbox

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEnvironmentWorkspaceAccessUsesNonInheritedTraverseACLs(t *testing.T) {
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
	name, err := StableAppContainerProfileName(t.Name())
	if err != nil {
		t.Fatal(err)
	}
	// Use the Environment ID that maps to the same stable profile name.
	environmentID := t.Name()
	status, err := PrepareEnvironmentWorkspaceAccess(environmentID, workspace, environment)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		_, _ = CleanupEnvironmentWorkspaceAccess(environmentID, workspace, environment)
	}()
	if status.ProfileName != name {
		t.Fatalf("profile=%s want %s", status.ProfileName, name)
	}
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
		if !granted {
			t.Fatalf("missing traverse grant on %s", path)
		}
		readGranted, readErr := WindowsAppContainerPathAccessGranted(path, status.ProfileSID, PathAccessReadOnly)
		if readErr != nil {
			t.Fatal(readErr)
		}
		if readGranted {
			t.Fatalf("traverse-only ancestor unexpectedly grants read access on %s", path)
		}
	}

	siblingTraverse, err := WindowsAppContainerPathAccessGranted(sibling, status.ProfileSID, PathAccessTraverse)
	if err != nil {
		t.Fatal(err)
	}
	if siblingTraverse {
		t.Fatal("Workspace traverse ACL unexpectedly inherited into unrelated sibling directory")
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
