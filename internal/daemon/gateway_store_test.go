package daemon

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestGatewayStoreRoundTripAndVersionErrors(t *testing.T) {
	root := t.TempDir()
	store := NewGatewayStore(root)

	initial, err := store.Load()
	if err != nil {
		t.Fatalf("Load(empty) error = %v", err)
	}
	if initial.DesiredRunning || initial.Listen != "" {
		t.Fatalf("empty gateway desired = %+v", initial)
	}

	want := GatewayDesired{DesiredRunning: true, Listen: "0.0.0.0:43127", Exposed: true}
	if err := store.Save(want); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	got, err := store.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got != want {
		t.Fatalf("gateway desired = %+v, want %+v", got, want)
	}

	if err := store.Save(GatewayDesired{DesiredRunning: false, Listen: want.Listen}); err != nil {
		t.Fatal(err)
	}
	stopped, err := store.Load()
	if err != nil || stopped.DesiredRunning || stopped.Listen != want.Listen {
		t.Fatalf("stopped desired = %+v err=%v", stopped, err)
	}

	path := gatewayStatePath(root)
	if err := os.WriteFile(path, []byte(`{"version":999,"desired_running":true}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Load(); !errors.Is(err, ErrUnsupportedGatewayState) {
		t.Fatalf("unsupported version error = %v", err)
	}
	if err := os.WriteFile(path, []byte(`{bad`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Load(); !errors.Is(err, ErrCorruptGatewayState) {
		t.Fatalf("corrupt state error = %v", err)
	}

	if _, err := os.Stat(filepath.Dir(path)); err != nil {
		t.Fatalf("gateway state directory missing: %v", err)
	}
}
