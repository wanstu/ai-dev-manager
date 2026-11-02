//go:build windows

package sandbox

import (
	"os"
	"path/filepath"
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

func TestEnsureWindowsAppContainerPathLocalDoesNotInherit(t *testing.T) {
	name, err := StableAppContainerProfileName(t.Name())
	if err != nil {
		t.Fatal(err)
	}
	profile, err := DeriveWindowsAppContainerProfile(name)
	if err != nil {
		t.Fatal(err)
	}

	root := t.TempDir()
	before := filepath.Join(root, "before")
	if err := os.Mkdir(before, 0o755); err != nil {
		t.Fatal(err)
	}

	changed, err := EnsureWindowsAppContainerPathLocal(root, profile.SID, PathAccessTraverse)
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Fatal("first local traverse grant unexpectedly reported no ACL change")
	}
	defer func() {
		if err := RevokeWindowsAppContainerPathLocal(root, profile.SID); err != nil {
			t.Errorf("revoke local AppContainer traverse grant: %v", err)
		}
	}()

	granted, err := WindowsAppContainerPathAccessGranted(root, profile.SID, PathAccessTraverse)
	if err != nil {
		t.Fatal(err)
	}
	if !granted {
		t.Fatal("local traverse grant was not detected on target directory")
	}

	after := filepath.Join(root, "after")
	if err := os.Mkdir(after, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, child := range []string{before, after} {
		childGranted, err := WindowsAppContainerPathAccessGranted(child, profile.SID, PathAccessTraverse)
		if err != nil {
			t.Fatal(err)
		}
		if childGranted {
			t.Fatalf("local traverse grant propagated to child directory: %s", child)
		}
	}

	if err := RevokeWindowsAppContainerPathLocal(root, profile.SID); err != nil {
		t.Fatal(err)
	}
	granted, err = WindowsAppContainerPathAccessGranted(root, profile.SID, PathAccessTraverse)
	if err != nil {
		t.Fatal(err)
	}
	if granted {
		t.Fatal("local traverse grant remained after revoke")
	}
}
