package app

import (
	"path/filepath"
	"testing"
)

func TestGlobalMemoryAgentWritePersistsIndependently(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "state.json")
	service := New(statePath)
	status, err := service.StrongPermissionControlStatus()
	if err != nil || status.GlobalMemoryAgentWriteEnabled {
		t.Fatalf("expected default denied, status=%+v err=%v", status, err)
	}
	status, err = service.SetGlobalMemoryAgentWrite(true)
	if err != nil || !status.GlobalMemoryAgentWriteEnabled || !status.Enabled {
		t.Fatalf("delegation changed strong mode: %+v err=%v", status, err)
	}
	reloaded := New(statePath)
	status, err = reloaded.SetStrongPermissionControl(false)
	if err != nil || !status.GlobalMemoryAgentWriteEnabled || status.Enabled {
		t.Fatalf("strong mode changed delegation: %+v err=%v", status, err)
	}
	status, err = reloaded.SetGlobalMemoryAgentWrite(false)
	if err != nil || status.GlobalMemoryAgentWriteEnabled || status.Enabled {
		t.Fatalf("revocation changed strong mode: %+v err=%v", status, err)
	}
	status, err = New(statePath).StrongPermissionControlStatus()
	if err != nil || status.GlobalMemoryAgentWriteEnabled || status.Enabled {
		t.Fatalf("revocation not persisted: %+v err=%v", status, err)
	}
}

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
