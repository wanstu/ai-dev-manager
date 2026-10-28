//go:build windows

package sandbox

import (
	"os"
	"testing"
)

func TestEnsureWindowsAppContainerPathReusesExistingGrant(t *testing.T) {
	if os.Getenv("ADM_SANDBOX_PROFILE_E2E") != "1" {
		t.Skip("set ADM_SANDBOX_PROFILE_E2E=1 to exercise AppContainer ACL lifecycle")
	}
	name, err := StableAppContainerProfileName(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	profile, _, err := EnsureWindowsAppContainerProfile(name)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = DeleteWindowsAppContainerProfile(name)
	}()

	root := t.TempDir()
	changed, err := EnsureWindowsAppContainerPath(root, profile.SID, PathAccessReadWrite)
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Fatal("first grant unexpectedly reported no ACL change")
	}
	defer func() {
		if err := RevokeWindowsAppContainerPath(root, profile.SID); err != nil {
			t.Errorf("revoke AppContainer path: %v", err)
		}
	}()

	granted, err := WindowsAppContainerPathAccessGranted(root, profile.SID, PathAccessReadWrite)
	if err != nil {
		t.Fatal(err)
	}
	if !granted {
		t.Fatal("granted AppContainer access was not detected")
	}
	changed, err = EnsureWindowsAppContainerPath(root, profile.SID, PathAccessReadWrite)
	if err != nil {
		t.Fatal(err)
	}
	if changed {
		t.Fatal("existing sufficient grant should be reused without rewriting the DACL")
	}
}
