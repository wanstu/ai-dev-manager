package workspace

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ai-dev-manager/internal/config"
)

func TestRegistryAddReloadUpdateAndRemovePreservesStableID(t *testing.T) {
	store := config.NewStore(t.TempDir())
	registry := NewRegistry(store)
	pathA := t.TempDir()
	pathB := t.TempDir()

	created, err := registry.Add(Input{Path: pathA, ProfileID: "work", RuntimeID: "native"})
	if err != nil {
		t.Fatalf("Add() error = %v", err)
	}
	if !strings.HasPrefix(created.ID, "ws_") || len(created.ID) != 35 {
		t.Fatalf("unexpected generated workspace id %q", created.ID)
	}
	if created.Path != filepath.Clean(pathA) {
		t.Fatalf("created path = %q, want %q", created.Path, filepath.Clean(pathA))
	}

	reloadedRegistry := NewRegistry(config.NewStore(store.Root()))
	reloaded, err := reloadedRegistry.Get(created.ID)
	if err != nil {
		t.Fatalf("Get() after reload error = %v", err)
	}
	if reloaded.ID != created.ID || reloaded.Path != created.Path || reloaded.ProfileID != "work" || reloaded.RuntimeID != "native" {
		t.Fatalf("workspace did not survive reload: %+v", reloaded)
	}

	updated, err := reloadedRegistry.Update(created.ID, Input{Path: pathB, ProfileID: "personal", RuntimeID: "devspace"})
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if updated.ID != created.ID {
		t.Fatalf("Update() changed stable ID: got %q want %q", updated.ID, created.ID)
	}
	if updated.Path != filepath.Clean(pathB) || updated.ProfileID != "personal" || updated.RuntimeID != "devspace" {
		t.Fatalf("unexpected updated workspace: %+v", updated)
	}

	if err := reloadedRegistry.Remove(created.ID); err != nil {
		t.Fatalf("Remove() error = %v", err)
	}
	_, err = reloadedRegistry.Get(created.ID)
	assertRegistryErrorKind(t, err, ErrNotFound)
}

func TestRegistryReloadsTwoWorkspacesWithStableIDs(t *testing.T) {
	store := config.NewStore(t.TempDir())
	registry := NewRegistry(store)
	workspaceA, err := registry.Add(Input{Path: t.TempDir(), ProfileID: "work", RuntimeID: "native"})
	if err != nil {
		t.Fatalf("Add A error = %v", err)
	}
	workspaceB, err := registry.Add(Input{Path: t.TempDir(), ProfileID: "personal", RuntimeID: "devspace"})
	if err != nil {
		t.Fatalf("Add B error = %v", err)
	}

	reloaded := NewRegistry(config.NewStore(store.Root()))
	listed, err := reloaded.List()
	if err != nil {
		t.Fatalf("List() after reload error = %v", err)
	}
	if len(listed) != 2 {
		t.Fatalf("reloaded workspace count = %d, want 2", len(listed))
	}
	byID := map[string]bool{listed[0].ID: true, listed[1].ID: true}
	if !byID[workspaceA.ID] || !byID[workspaceB.ID] {
		t.Fatalf("stable IDs not preserved across reload: got %+v, want %q and %q", listed, workspaceA.ID, workspaceB.ID)
	}
}

func TestRegistryRejectsDuplicateWindowsSemanticPath(t *testing.T) {
	store := config.NewStore(t.TempDir())
	registry := NewRegistry(store)
	workspacePath := t.TempDir()

	first, err := registry.Add(Input{Path: workspacePath})
	if err != nil {
		t.Fatalf("first Add() error = %v", err)
	}

	caseVariant := strings.ToUpper(workspacePath)
	_, err = registry.Add(Input{Path: caseVariant})
	assertRegistryErrorKind(t, err, ErrDuplicatePath)

	listed, err := registry.List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(listed) != 1 || listed[0].ID != first.ID {
		t.Fatalf("duplicate attempt changed registry: %+v", listed)
	}
}

func TestRegistryUpdateCannotTakeAnotherWorkspacePath(t *testing.T) {
	store := config.NewStore(t.TempDir())
	registry := NewRegistry(store)
	pathA := t.TempDir()
	pathB := t.TempDir()

	workspaceA, err := registry.Add(Input{Path: pathA})
	if err != nil {
		t.Fatalf("Add A error = %v", err)
	}
	workspaceB, err := registry.Add(Input{Path: pathB})
	if err != nil {
		t.Fatalf("Add B error = %v", err)
	}

	_, err = registry.Update(workspaceB.ID, Input{Path: pathA})
	assertRegistryErrorKind(t, err, ErrDuplicatePath)

	stillA, err := registry.Get(workspaceA.ID)
	if err != nil {
		t.Fatalf("Get A error = %v", err)
	}
	stillB, err := registry.Get(workspaceB.ID)
	if err != nil {
		t.Fatalf("Get B error = %v", err)
	}
	if stillA.Path != filepath.Clean(pathA) || stillB.Path != filepath.Clean(pathB) {
		t.Fatalf("failed update mutated workspaces: A=%+v B=%+v", stillA, stillB)
	}
}

func TestRegistryRejectsInvalidWorkspacePaths(t *testing.T) {
	store := config.NewStore(t.TempDir())
	registry := NewRegistry(store)

	filePath := filepath.Join(t.TempDir(), "file.txt")
	if err := os.WriteFile(filePath, []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	missingPath := filepath.Join(t.TempDir(), "missing")

	for _, path := range []string{"", "relative/path", filePath, missingPath} {
		t.Run(strings.ReplaceAll(path, `\\`, `_`), func(t *testing.T) {
			_, err := registry.Add(Input{Path: path})
			assertRegistryErrorKind(t, err, ErrInvalidPath)
		})
	}
}

func TestRegistryRemoveOnlyTargetWorkspace(t *testing.T) {
	store := config.NewStore(t.TempDir())
	registry := NewRegistry(store)
	workspaceA, err := registry.Add(Input{Path: t.TempDir()})
	if err != nil {
		t.Fatalf("Add A error = %v", err)
	}
	workspaceB, err := registry.Add(Input{Path: t.TempDir()})
	if err != nil {
		t.Fatalf("Add B error = %v", err)
	}

	if err := registry.Remove(workspaceA.ID); err != nil {
		t.Fatalf("Remove A error = %v", err)
	}
	listed, err := registry.List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(listed) != 1 || listed[0].ID != workspaceB.ID {
		t.Fatalf("Remove() affected wrong workspace: %+v", listed)
	}
}

func TestRegistryUnknownIDReturnsStructuredError(t *testing.T) {
	registry := NewRegistry(config.NewStore(t.TempDir()))
	_, err := registry.Get("ws_missing")
	assertRegistryErrorKind(t, err, ErrNotFound)
	_, err = registry.Update("ws_missing", Input{Path: t.TempDir()})
	assertRegistryErrorKind(t, err, ErrNotFound)
	err = registry.Remove("ws_missing")
	assertRegistryErrorKind(t, err, ErrNotFound)
}

func assertRegistryErrorKind(t *testing.T, err error, want ErrorKind) {
	t.Helper()
	if err == nil {
		t.Fatalf("error = nil, want RegistryError kind %q", want)
	}
	var registryErr *RegistryError
	if !errors.As(err, &registryErr) {
		t.Fatalf("error type = %T, want *RegistryError", err)
	}
	if registryErr.Kind != want {
		t.Fatalf("RegistryError.Kind = %q, want %q (error: %v)", registryErr.Kind, want, err)
	}
}
