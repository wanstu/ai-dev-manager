//go:build windows

package sandbox

import (
	"os"
	"testing"
)

func TestWindowsAppContainerAPIsAvailable(t *testing.T) {
	if !WindowsAppContainerAvailable() {
		t.Fatal("Windows AppContainer APIs are unavailable")
	}
}

func TestWindowsAppContainerProfileLifecycle(t *testing.T) {
	if os.Getenv("ADM_SANDBOX_PROFILE_E2E") != "1" {
		t.Skip("set ADM_SANDBOX_PROFILE_E2E=1 to exercise AppContainer profile lifecycle")
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
		if err := DeleteWindowsAppContainerProfile(name); err != nil {
			t.Errorf("delete AppContainer profile: %v", err)
		}
	}()
	if profile.Name != name || profile.SID == "" {
		t.Fatalf("invalid profile: %+v", profile)
	}
	reopened, created, err := EnsureWindowsAppContainerProfile(name)
	if err != nil {
		t.Fatal(err)
	}
	if created {
		t.Fatal("existing AppContainer profile was unexpectedly recreated")
	}
	if reopened.SID != profile.SID {
		t.Fatalf("AppContainer SID changed: %q != %q", reopened.SID, profile.SID)
	}
}
