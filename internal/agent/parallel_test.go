package agent

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"ai-dev-manager/internal/adapter/runtimeadapter"
	"ai-dev-manager/internal/model"
)

func TestParallelVerifyExecutorRunsLanesConcurrentlyAndCleansUp(t *testing.T) {
	base := newFakeParallelBase()
	entered := make(chan string, 2)
	release := make(chan struct{})
	derived := map[string]*fakeParallelLaneRuntime{}
	builder := func(_ string, derivedID, path string) (runtimeadapter.Runtime, error) {
		lane := &fakeParallelLaneRuntime{id: derivedID, path: path, entered: entered, release: release, verifierStatus: "passed"}
		derived[path] = lane
		return lane, nil
	}
	executor, err := NewParallelVerifyExecutor(func(string) (runtimeadapter.Runtime, error) { return base, nil }, builder)
	if err != nil {
		t.Fatal(err)
	}

	var mu sync.Mutex
	var latest RunUpdate
	done := make(chan error, 1)
	go func() {
		done <- executor.Run(context.Background(), RunSpec{
			RunID:     "run_parallel",
			Workspace: model.Workspace{ID: "ws_a", Path: t.TempDir()},
			Input: parallelInput(false,
				ParallelLaneSpec{Name: "lane-a", Branch: "branch-a"},
				ParallelLaneSpec{Name: "lane-b", Branch: "branch-b"},
			),
			Update: func(update RunUpdate) {
				mu.Lock()
				latest = update
				mu.Unlock()
			},
		})
	}()

	seen := map[string]bool{}
	for len(seen) < 2 {
		select {
		case id := <-entered:
			seen[id] = true
		case <-time.After(2 * time.Second):
			t.Fatalf("parallel lanes did not overlap; entered=%v", seen)
		}
	}
	close(release)
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("parallel executor error = %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("parallel executor did not finish")
	}

	if len(base.created) != 2 || base.created[0] != "lane-a" || base.created[1] != "lane-b" {
		t.Fatalf("worktree create order = %v", base.created)
	}
	if len(base.removed) != 2 {
		t.Fatalf("worktree cleanup = %v", base.removed)
	}
	if len(derived) != 2 {
		t.Fatalf("derived runtimes = %d", len(derived))
	}
	mu.Lock()
	status := latest
	mu.Unlock()
	if status.Stage != StageCompleted || status.Review == nil || status.Review.Decision != ReviewPass || status.Parallel == nil {
		t.Fatalf("parent result = %+v", status)
	}
	if len(status.Parallel.Lanes) != 2 {
		t.Fatalf("parallel lanes = %+v", status.Parallel)
	}
	for _, lane := range status.Parallel.Lanes {
		if lane.State != ParallelLaneCompleted || lane.Review == nil || lane.Review.Decision != ReviewPass || lane.Cleanup != "removed" {
			t.Fatalf("lane audit = %+v", lane)
		}
	}
}

func TestParallelVerifyExecutorAggregatesReviewFailWithoutInfrastructureError(t *testing.T) {
	base := newFakeParallelBase()
	release := make(chan struct{})
	close(release)
	executor, err := NewParallelVerifyExecutor(
		func(string) (runtimeadapter.Runtime, error) { return base, nil },
		func(_ string, derivedID, path string) (runtimeadapter.Runtime, error) {
			status := "passed"
			if path == "worktree/lane-b" {
				status = "failed"
			}
			return &fakeParallelLaneRuntime{id: derivedID, path: path, release: release, verifierStatus: status}, nil
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	var latest RunUpdate
	err = executor.Run(context.Background(), RunSpec{
		RunID:     "run_parallel_fail",
		Workspace: model.Workspace{ID: "ws_a", Path: t.TempDir()},
		Input: parallelInput(false,
			ParallelLaneSpec{Name: "lane-a", Branch: "branch-a"},
			ParallelLaneSpec{Name: "lane-b", Branch: "branch-b"},
		),
		Update: func(update RunUpdate) { latest = update },
	})
	if err != nil {
		t.Fatalf("review fail became infrastructure error: %v", err)
	}
	if latest.Review == nil || latest.Review.Decision != ReviewFail || latest.Parallel == nil {
		t.Fatalf("parent review = %+v", latest)
	}
}

func TestParallelVerifyInputValidationAndKeepWorktrees(t *testing.T) {
	if _, err := decodeParallelVerifyInput(parallelInput(false, ParallelLaneSpec{Name: "one", Branch: "one"})); err == nil {
		t.Fatal("single lane unexpectedly accepted")
	}
	if _, err := decodeParallelVerifyInput(parallelInput(false,
		ParallelLaneSpec{Name: "same", Branch: "one"},
		ParallelLaneSpec{Name: "same", Branch: "two"},
	)); err == nil {
		t.Fatal("duplicate lane name unexpectedly accepted")
	}

	base := newFakeParallelBase()
	release := make(chan struct{})
	close(release)
	executor, _ := NewParallelVerifyExecutor(
		func(string) (runtimeadapter.Runtime, error) { return base, nil },
		func(_ string, derivedID, path string) (runtimeadapter.Runtime, error) {
			return &fakeParallelLaneRuntime{id: derivedID, path: path, release: release, verifierStatus: "passed"}, nil
		},
	)
	var latest RunUpdate
	if err := executor.Run(context.Background(), RunSpec{
		RunID: "run_keep", Workspace: model.Workspace{ID: "ws_a", Path: t.TempDir()},
		Input: parallelInput(true,
			ParallelLaneSpec{Name: "lane-a", Branch: "branch-a"},
			ParallelLaneSpec{Name: "lane-b", Branch: "branch-b"},
		),
		Update: func(update RunUpdate) { latest = update },
	}); err != nil {
		t.Fatal(err)
	}
	if len(base.removed) != 0 || latest.Parallel == nil {
		t.Fatalf("keep-worktrees cleanup = removed:%v result:%+v", base.removed, latest.Parallel)
	}
	for _, lane := range latest.Parallel.Lanes {
		if lane.Cleanup != "preserved" {
			t.Fatalf("lane not preserved: %+v", lane)
		}
	}
}

type fakeParallelBase struct {
	mu      sync.Mutex
	created []string
	removed []string
}

func newFakeParallelBase() *fakeParallelBase       { return &fakeParallelBase{} }
func (r *fakeParallelBase) ID() string             { return "base" }
func (r *fakeParallelBase) WorkspaceID() string    { return "ws_a" }
func (r *fakeParallelBase) Capabilities() []string { return []string{"git.worktree"} }
func (r *fakeParallelBase) Status(context.Context) runtimeadapter.Status {
	return runtimeadapter.Status{ID: r.ID(), WorkspaceID: r.WorkspaceID(), State: runtimeadapter.StateReady, Capabilities: r.Capabilities()}
}
func (r *fakeParallelBase) Invoke(_ context.Context, operation string, input map[string]any) (any, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	switch operation {
	case runtimeadapter.OpGitWorktreeCreate:
		name, _ := input["name"].(string)
		r.created = append(r.created, name)
		return map[string]any{"Path": "worktree/" + name}, nil
	case runtimeadapter.OpGitWorktreeRemove:
		name, _ := input["name"].(string)
		r.removed = append(r.removed, name)
		return map[string]any{"removed": true}, nil
	default:
		return nil, fmt.Errorf("unexpected base operation %s", operation)
	}
}

type fakeParallelLaneRuntime struct {
	id             string
	path           string
	entered        chan<- string
	release        <-chan struct{}
	verifierStatus string
}

func (r *fakeParallelLaneRuntime) ID() string          { return r.id }
func (r *fakeParallelLaneRuntime) WorkspaceID() string { return r.id }
func (r *fakeParallelLaneRuntime) Capabilities() []string {
	return []string{runtimeadapter.OpGitStatus, runtimeadapter.OpGitDiff, "verify.run"}
}
func (r *fakeParallelLaneRuntime) Status(context.Context) runtimeadapter.Status {
	return runtimeadapter.Status{ID: r.ID(), WorkspaceID: r.WorkspaceID(), State: runtimeadapter.StateReady, Capabilities: r.Capabilities()}
}
func (r *fakeParallelLaneRuntime) Invoke(ctx context.Context, operation string, _ map[string]any) (any, error) {
	switch operation {
	case runtimeadapter.OpGitStatus:
		return []map[string]any{}, nil
	case runtimeadapter.OpGitDiff:
		return map[string]any{"path": r.path}, nil
	case runtimeadapter.OpVerifierRunMany:
		if r.entered != nil {
			r.entered <- r.id
		}
		if r.release != nil {
			select {
			case <-r.release:
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
		return []map[string]any{{"id": "test", "status": r.verifierStatus}}, nil
	default:
		return nil, errors.New("unexpected lane operation")
	}
}

func parallelInput(keep bool, lanes ...ParallelLaneSpec) map[string]any {
	return map[string]any{"lanes": lanes, "keep_worktrees": keep}
}
