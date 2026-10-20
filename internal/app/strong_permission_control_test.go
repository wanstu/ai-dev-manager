package app

import (
	"path/filepath"
	"testing"
)

func TestStrongPermissionControlDefaultsEnabledAndPersists(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "state.json")
	service := New(statePath)

	status, err := service.StrongPermissionControlStatus()
	if err != nil {
		t.Fatal(err)
	}
	if !status.Enabled || status.Mode != "strong" || !status.DefaultEnabled {
		t.Fatalf("default strong permission status = %+v", status)
	}

	status, err = service.SetStrongPermissionControl(false)
	if err != nil {
		t.Fatal(err)
	}
	if status.Enabled || status.Mode != "v1.4.4_compatible" || status.Compatibility != "v1.4.4" {
		t.Fatalf("relaxed strong permission status = %+v", status)
	}

	reloaded := New(statePath)
	status, err = reloaded.StrongPermissionControlStatus()
	if err != nil {
		t.Fatal(err)
	}
	if status.Enabled || status.Mode != "v1.4.4_compatible" {
		t.Fatalf("relaxed mode did not persist: %+v", status)
	}

	status, err = reloaded.SetStrongPermissionControl(true)
	if err != nil {
		t.Fatal(err)
	}
	if !status.Enabled || status.Mode != "strong" {
		t.Fatalf("strong mode did not restore: %+v", status)
	}
}
