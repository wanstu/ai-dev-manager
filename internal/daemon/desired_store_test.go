package daemon

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestDesiredStoreRoundTripSortDedupeAndRemove(t *testing.T) {
	root := t.TempDir()
	store := NewDesiredStore(root)

	ids, err := store.Load()
	if err != nil {
		t.Fatalf("Load(missing) error = %v", err)
	}
	if len(ids) != 0 {
		t.Fatalf("Load(missing) = %v, want empty", ids)
	}

	if err := store.Replace([]string{"ws_b", "ws_a", "ws_b", " ", "ws_c"}); err != nil {
		t.Fatalf("Replace() error = %v", err)
	}
	ids, err = store.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if want := []string{"ws_a", "ws_b", "ws_c"}; !reflect.DeepEqual(ids, want) {
		t.Fatalf("Load() = %v, want %v", ids, want)
	}

	if err := store.Add("ws_a"); err != nil {
		t.Fatalf("Add(existing) error = %v", err)
	}
	if err := store.Add("ws_d"); err != nil {
		t.Fatalf("Add(new) error = %v", err)
	}
	if err := store.Remove("ws_b"); err != nil {
		t.Fatalf("Remove() error = %v", err)
	}
	ids, err = store.Load()
	if err != nil {
		t.Fatalf("Load(after remove) error = %v", err)
	}
	if want := []string{"ws_a", "ws_c", "ws_d"}; !reflect.DeepEqual(ids, want) {
		t.Fatalf("Load(after remove) = %v, want %v", ids, want)
	}

	data, err := os.ReadFile(desiredStatePath(root))
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	var persisted desiredStateFile
	if err := json.Unmarshal(data, &persisted); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	wantRuntimes := []DesiredRuntime{
		{WorkspaceID: "ws_a", Listen: DefaultRuntimeListen},
		{WorkspaceID: "ws_c", Listen: DefaultRuntimeListen},
		{WorkspaceID: "ws_d", Listen: DefaultRuntimeListen},
	}
	if persisted.Version != desiredStateVersion || !reflect.DeepEqual(persisted.Runtimes, wantRuntimes) || len(persisted.WorkspaceIDs) != 0 {
		t.Fatalf("persisted = %+v", persisted)
	}
}

func TestDesiredStoreReadsLegacyV1AsLoopbackAndPersistsExposure(t *testing.T) {
	root := t.TempDir()
	path := desiredStatePath(root)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	legacy, err := json.Marshal(desiredStateFile{Version: desiredStateLegacyVersion, WorkspaceIDs: []string{"ws_b", "ws_a"}})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, legacy, 0o600); err != nil {
		t.Fatal(err)
	}
	store := NewDesiredStore(root)
	runtimes, err := store.LoadRuntimes()
	if err != nil {
		t.Fatalf("LoadRuntimes(v1) error = %v", err)
	}
	want := []DesiredRuntime{{WorkspaceID: "ws_a", Listen: DefaultRuntimeListen}, {WorkspaceID: "ws_b", Listen: DefaultRuntimeListen}}
	if !reflect.DeepEqual(runtimes, want) {
		t.Fatalf("LoadRuntimes(v1) = %+v, want %+v", runtimes, want)
	}
	if err := store.Set(DesiredRuntime{WorkspaceID: "ws_a", Listen: DockerRuntimeListen, Exposed: true}); err != nil {
		t.Fatalf("Set(exposed) error = %v", err)
	}
	runtimes, err = store.LoadRuntimes()
	if err != nil {
		t.Fatalf("LoadRuntimes(v2) error = %v", err)
	}
	if len(runtimes) != 2 || runtimes[0].WorkspaceID != "ws_a" || runtimes[0].Listen != DockerRuntimeListen || !runtimes[0].Exposed {
		t.Fatalf("LoadRuntimes(v2) = %+v", runtimes)
	}
}

func TestDesiredStoreRejectsCorruptAndUnsupportedState(t *testing.T) {
	root := t.TempDir()
	path := desiredStatePath(root)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("{broken"), 0o600); err != nil {
		t.Fatal(err)
	}
	store := NewDesiredStore(root)
	if _, err := store.Load(); !errors.Is(err, ErrCorruptDesiredState) {
		t.Fatalf("Load(corrupt) error = %v, want ErrCorruptDesiredState", err)
	}

	data, err := json.Marshal(desiredStateFile{Version: desiredStateVersion + 1, WorkspaceIDs: []string{"ws_a"}})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Load(); !errors.Is(err, ErrUnsupportedDesiredState) {
		t.Fatalf("Load(unsupported) error = %v, want ErrUnsupportedDesiredState", err)
	}
}
