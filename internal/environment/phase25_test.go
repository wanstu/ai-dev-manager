package environment

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"ai-dev-manager/internal/adapter/runtimeadapter"
	"ai-dev-manager/internal/model"
)

func TestManagerInvokeMutationRequiresWriterAndRenewsOnlyOnSuccess(t *testing.T) {
	root := t.TempDir()
	store := NewStore(root)
	t0 := time.Date(2026, 9, 5, 1, 0, 0, 0, time.UTC)
	t1 := t0.Add(time.Minute)
	env := mutationFixtureEnvironment(root, "env_mut", "ws_mut", "owner-a", t0)
	if err := store.Put(env); err != nil {
		t.Fatal(err)
	}

	var invokes atomic.Int32
	failRuntime := atomic.Bool{}
	base := mutationBaseRuntime(env, nil)
	manager, err := NewManager(
		store,
		func(id string) (model.Workspace, error) { return model.Workspace{ID: id, Path: root}, nil },
		func(string) (runtimeadapter.Runtime, error) { return base, nil },
		func(workspaceID, runtimeID, path string) (runtimeadapter.Runtime, error) {
			return &phase24Runtime{
				id: runtimeID, workspaceID: workspaceID, capabilities: []string{"files.write", "files.edit", "files.delete", "shell.exec", "verify.run"},
				invoke: func(operation string, input map[string]any) (any, error) {
					invokes.Add(1)
					if failRuntime.Load() {
						return nil, errors.New("runtime failed")
					}
					return map[string]any{"operation": operation, "input": input}, nil
				},
			}, nil
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	manager.now = func() time.Time { return t1 }

	if _, err := manager.InvokeMutation(context.Background(), env.ID, "owner-b", runtimeadapter.OpDelete, map[string]any{"path": "x"}); !isEnvironmentCode(err, ErrWriterNotOwner) {
		t.Fatalf("wrong owner error = %v, want writer_not_owner", err)
	}
	if invokes.Load() != 0 {
		t.Fatalf("wrong writer reached Runtime: invokes=%d", invokes.Load())
	}

	if _, err := manager.InvokeMutation(context.Background(), env.ID, "owner-a", runtimeadapter.OpDelete, map[string]any{"path": "x"}); err != nil {
		t.Fatalf("correct owner delete mutation error = %v", err)
	}
	persisted, err := store.Get(env.ID)
	if err != nil {
		t.Fatal(err)
	}
	if persisted.Writer == nil || persisted.Writer.Owner != "owner-a" || !persisted.Writer.AcquiredAt.Equal(t0) || !persisted.Writer.LastSeenAt.Equal(t1) || !persisted.LastActivityAt.Equal(t1) {
		t.Fatalf("writer/activity renewal = %+v env=%+v", persisted.Writer, persisted)
	}

	failRuntime.Store(true)
	manager.now = func() time.Time { return t1.Add(time.Minute) }
	if _, err := manager.InvokeMutation(context.Background(), env.ID, "owner-a", runtimeadapter.OpWrite, map[string]any{"path": "y", "content": "y"}); !isEnvironmentCode(err, ErrRuntime) {
		t.Fatalf("runtime failure error = %v, want runtime_error", err)
	}
	afterFailure, err := store.Get(env.ID)
	if err != nil {
		t.Fatal(err)
	}
	if afterFailure.Writer == nil || afterFailure.Writer.Owner != "owner-a" || !afterFailure.Writer.LastSeenAt.Equal(t1) || !afterFailure.LastActivityAt.Equal(t1) {
		t.Fatalf("failed mutation changed/released writer activity: %+v", afterFailure)
	}
	if _, err := manager.InvokeMutation(context.Background(), env.ID, "owner-a", runtimeadapter.OpRead, map[string]any{"path": "x"}); !isEnvironmentCode(err, ErrInvalidInput) {
		t.Fatalf("read operation through mutation boundary error = %v, want invalid_input", err)
	}
}

func TestManagerMutationBlocksWriterReleaseUntilInvokeCompletes(t *testing.T) {
	root := t.TempDir()
	store := NewStore(root)
	t0 := time.Date(2026, 9, 5, 2, 0, 0, 0, time.UTC)
	env := mutationFixtureEnvironment(root, "env_release", "ws_release", "owner-a", t0)
	if err := store.Put(env); err != nil {
		t.Fatal(err)
	}
	entered := make(chan struct{})
	unblock := make(chan struct{})
	base := mutationBaseRuntime(env, nil)
	manager := newMutationTestManager(t, store, root, base, func(workspaceID, runtimeID, path string) (runtimeadapter.Runtime, error) {
		return &phase24Runtime{id: runtimeID, workspaceID: workspaceID, capabilities: []string{"files.write"}, invoke: func(string, map[string]any) (any, error) {
			close(entered)
			<-unblock
			return "ok", nil
		}}, nil
	})
	manager.now = func() time.Time { return t0.Add(time.Minute) }

	mutationDone := make(chan error, 1)
	go func() {
		_, err := manager.InvokeMutation(context.Background(), env.ID, "owner-a", runtimeadapter.OpWrite, map[string]any{"path": "x", "content": "x"})
		mutationDone <- err
	}()
	<-entered

	releaseDone := make(chan error, 1)
	go func() {
		_, err := manager.ReleaseWriter(env.ID, "owner-a", false)
		releaseDone <- err
	}()
	assertStillBlocked(t, releaseDone, "writer release")
	close(unblock)
	if err := <-mutationDone; err != nil {
		t.Fatalf("mutation error = %v", err)
	}
	if err := <-releaseDone; err != nil {
		t.Fatalf("release after mutation error = %v", err)
	}
	persisted, err := store.Get(env.ID)
	if err != nil {
		t.Fatal(err)
	}
	if persisted.Writer != nil {
		t.Fatalf("release did not clear writer: %+v", persisted.Writer)
	}
}

func TestManagerMutationBlocksForceDestroyAndAllowsParallelMutations(t *testing.T) {
	root := t.TempDir()
	store := NewStore(root)
	t0 := time.Date(2026, 9, 5, 3, 0, 0, 0, time.UTC)
	envA := mutationFixtureEnvironment(root, "env_a", "ws_shared", "owner-a", t0)
	envB := mutationFixtureEnvironment(root, "env_b", "ws_shared", "owner-b", t0)
	if err := store.Put(envA); err != nil {
		t.Fatal(err)
	}
	if err := store.Put(envB); err != nil {
		t.Fatal(err)
	}

	enteredA := make(chan struct{})
	enteredB := make(chan struct{})
	unblock := make(chan struct{})
	removed := make(chan string, 2)
	base := &phase24Runtime{
		id: "base", workspaceID: "ws_shared", capabilities: []string{"git.worktree"},
		invoke: func(operation string, input map[string]any) (any, error) {
			switch operation {
			case runtimeadapter.OpGitWorktreeGet:
				name, _ := input["name"].(string)
				if name == envA.WorktreeName {
					return worktreeInfo{Path: envA.WorktreePath, Branch: envA.Branch}, nil
				}
				if name == envB.WorktreeName {
					return worktreeInfo{Path: envB.WorktreePath, Branch: envB.Branch}, nil
				}
				return nil, errors.New("unknown worktree")
			case runtimeadapter.OpGitWorktreeRemove:
				name, _ := input["name"].(string)
				removed <- name
				return map[string]any{"removed": true}, nil
			default:
				return nil, fmt.Errorf("unexpected base operation %s", operation)
			}
		},
	}
	manager := newMutationTestManager(t, store, root, base, func(workspaceID, runtimeID, path string) (runtimeadapter.Runtime, error) {
		entered := enteredA
		if runtimeID == envB.ID {
			entered = enteredB
		}
		return &phase24Runtime{id: runtimeID, workspaceID: workspaceID, capabilities: []string{"files.write"}, invoke: func(string, map[string]any) (any, error) {
			close(entered)
			<-unblock
			return "ok", nil
		}}, nil
	})

	var wg sync.WaitGroup
	mutationErrs := make(chan error, 2)
	for _, tc := range []struct{ id, owner string }{{envA.ID, "owner-a"}, {envB.ID, "owner-b"}} {
		wg.Add(1)
		go func(id, owner string) {
			defer wg.Done()
			_, err := manager.InvokeMutation(context.Background(), id, owner, runtimeadapter.OpWrite, map[string]any{"path": "x", "content": owner})
			mutationErrs <- err
		}(tc.id, tc.owner)
	}
	select {
	case <-enteredA:
	case <-time.After(time.Second):
		t.Fatal("mutation A did not enter Runtime")
	}
	select {
	case <-enteredB:
	case <-time.After(time.Second):
		t.Fatal("mutation B did not enter Runtime concurrently")
	}

	destroyDone := make(chan error, 1)
	go func() {
		_, err := manager.Destroy(context.Background(), envA.ID, true)
		destroyDone <- err
	}()
	assertStillBlocked(t, destroyDone, "force destroy")
	select {
	case name := <-removed:
		t.Fatalf("worktree removed while mutation still running: %s", name)
	default:
	}

	close(unblock)
	wg.Wait()
	close(mutationErrs)
	for err := range mutationErrs {
		if err != nil {
			t.Fatalf("parallel mutation error = %v", err)
		}
	}
	if err := <-destroyDone; err != nil {
		t.Fatalf("force destroy after mutation error = %v", err)
	}
	select {
	case name := <-removed:
		if name != envA.WorktreeName {
			t.Fatalf("removed worktree = %s, want %s", name, envA.WorktreeName)
		}
	case <-time.After(time.Second):
		t.Fatal("force destroy never removed worktree after mutation")
	}
}

func mutationFixtureEnvironment(root, id, workspaceID, owner string, at time.Time) Environment {
	return Environment{
		ID:             id,
		WorkspaceID:    workspaceID,
		Name:           id,
		Branch:         "adm/" + id,
		BaseRef:        "dev",
		BaseCommit:     "abc",
		WorktreeName:   "wt-" + id,
		WorktreePath:   filepath.Join(root, "worktrees", id),
		State:          StateReady,
		CreatedAt:      at,
		UpdatedAt:      at,
		LastActivityAt: at,
		Writer:         &WriterLease{Owner: owner, AcquiredAt: at, LastSeenAt: at},
	}
}

func mutationBaseRuntime(env Environment, removed chan<- string) runtimeadapter.Runtime {
	return &phase24Runtime{
		id: "base", workspaceID: env.WorkspaceID, capabilities: []string{"git.worktree"},
		invoke: func(operation string, input map[string]any) (any, error) {
			switch operation {
			case runtimeadapter.OpGitWorktreeGet:
				return worktreeInfo{Path: env.WorktreePath, Branch: env.Branch}, nil
			case runtimeadapter.OpGitWorktreeRemove:
				if removed != nil {
					removed <- env.WorktreeName
				}
				return map[string]any{"removed": true}, nil
			default:
				return nil, fmt.Errorf("unexpected base operation %s", operation)
			}
		},
	}
}

func newMutationTestManager(t *testing.T, store *Store, root string, base runtimeadapter.Runtime, buildDerived DerivedRuntimeBuilder) *Manager {
	t.Helper()
	manager, err := NewManager(
		store,
		func(id string) (model.Workspace, error) { return model.Workspace{ID: id, Path: root}, nil },
		func(string) (runtimeadapter.Runtime, error) { return base, nil },
		buildDerived,
	)
	if err != nil {
		t.Fatal(err)
	}
	return manager
}

func assertStillBlocked(t *testing.T, done <-chan error, label string) {
	t.Helper()
	select {
	case err := <-done:
		t.Fatalf("%s completed while mutation was in flight: %v", label, err)
	case <-time.After(100 * time.Millisecond):
	}
}
