package environment

import (
	"os"
	"testing"
	"time"
)

func TestStoreRoundTripAndVersionErrors(t *testing.T) {
	root := t.TempDir()
	store := NewStore(root)
	now := time.Date(2026, 9, 5, 2, 0, 0, 0, time.UTC)
	envA := Environment{ID: "env_b", WorkspaceID: "ws_1", Name: "b", State: StateCreating, CreatedAt: now, UpdatedAt: now, LastActivityAt: now}
	envB := Environment{ID: "env_a", WorkspaceID: "ws_1", Name: "a", State: StateReady, CreatedAt: now, UpdatedAt: now, LastActivityAt: now}
	if err := store.Put(envA); err != nil {
		t.Fatalf("Put(envA) error = %v", err)
	}
	if err := store.Put(envB); err != nil {
		t.Fatalf("Put(envB) error = %v", err)
	}
	items, err := NewStore(root).List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(items) != 2 || items[0].ID != "env_a" || items[1].ID != "env_b" {
		t.Fatalf("unexpected items: %+v", items)
	}
	envA.State = StateError
	if err := store.Put(envA); err != nil {
		t.Fatalf("update Put(envA) error = %v", err)
	}
	got, err := store.Get(envA.ID)
	if err != nil || got.State != StateError {
		t.Fatalf("Get(updated) = %+v, %v", got, err)
	}
	if err := store.Remove(envB.ID); err != nil {
		t.Fatalf("Remove() error = %v", err)
	}
	if items, err := store.List(); err != nil || len(items) != 1 || items[0].ID != envA.ID {
		t.Fatalf("List(after remove) = %+v, %v", items, err)
	}

	if err := os.WriteFile(store.Path(), []byte("{not-json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := store.List(); err == nil {
		t.Fatal("corrupt store unexpectedly accepted")
	}
	if err := os.WriteFile(store.Path(), []byte("{\"version\":999,\"environments\":[]}"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := store.List(); err == nil {
		t.Fatal("unsupported store version unexpectedly accepted")
	}
}
