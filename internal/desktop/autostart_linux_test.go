//go:build linux

package desktop

import "testing"

func TestLinuxAutostartLifecycle(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if enabled, err := launchAtLoginEnabled(); err != nil || enabled {
		t.Fatalf("initial autostart state = %v, %v; want disabled", enabled, err)
	}
	if err := setLaunchAtLogin(true); err != nil {
		t.Fatal(err)
	}
	if enabled, err := launchAtLoginEnabled(); err != nil || !enabled {
		t.Fatalf("enabled autostart state = %v, %v; want enabled", enabled, err)
	}
	if err := setLaunchAtLogin(false); err != nil {
		t.Fatal(err)
	}
	if enabled, err := launchAtLoginEnabled(); err != nil || enabled {
		t.Fatalf("disabled autostart state = %v, %v; want disabled", enabled, err)
	}
}
