package agent

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"ai-dev-manager/internal/model"
)

func TestManagerLifecycleCancelAndIsolation(t *testing.T) {
	manager := newTestManager(t, LifecycleExecutor{})

	a, err := manager.Start("ws_a")
	if err != nil {
		t.Fatalf("Start(ws_a) error = %v", err)
	}
	b, err := manager.Start("ws_b")
	if err != nil {
		t.Fatalf("Start(ws_b) error = %v", err)
	}
	if !strings.HasPrefix(a.RunID, "run_") || !strings.HasPrefix(b.RunID, "run_") || a.RunID == b.RunID {
		t.Fatalf("unexpected run ids: %q %q", a.RunID, b.RunID)
	}
	if a.State != StateRunning || b.State != StateRunning || a.Executor != "lifecycle" {
		t.Fatalf("unexpected initial states: a=%+v b=%+v", a, b)
	}

	list := manager.List()
	if len(list) != 2 {
		t.Fatalf("List() len = %d, want 2", len(list))
	}

	cancelled, err := manager.Cancel(a.RunID)
	if err != nil {
		t.Fatalf("Cancel() error = %v", err)
	}
	if cancelled.State != StateCancelled || cancelled.FinishedAt == nil {
		t.Fatalf("cancelled status = %+v", cancelled)
	}

	again, err := manager.Cancel(a.RunID)
	if err != nil {
		t.Fatalf("repeat Cancel() error = %v", err)
	}
	if again.State != StateCancelled || again.RunID != a.RunID {
		t.Fatalf("repeat Cancel() = %+v", again)
	}

	stillRunning, err := manager.Get(b.RunID)
	if err != nil {
		t.Fatalf("Get(ws_b run) error = %v", err)
	}
	if stillRunning.State != StateRunning || stillRunning.WorkspaceID != "ws_b" {
		t.Fatalf("ws_b run changed unexpectedly: %+v", stillRunning)
	}

	manager.StopAll()
	stopped, err := manager.Get(b.RunID)
	if err != nil {
		t.Fatalf("Get(after StopAll) error = %v", err)
	}
	if stopped.State != StateCancelled {
		t.Fatalf("StopAll state = %s, want cancelled", stopped.State)
	}
}

func TestManagerExecutorCompletionAndError(t *testing.T) {
	executor := executorFunc{
		name: "test",
		run: func(_ context.Context, spec RunSpec) error {
			if spec.Workspace.ID == "ws_error" {
				return errors.New("boom")
			}
			return nil
		},
	}
	manager := newTestManager(t, executor)

	completed, err := manager.Start("ws_ok")
	if err != nil {
		t.Fatalf("Start(ws_ok) error = %v", err)
	}
	errored, err := manager.Start("ws_error")
	if err != nil {
		t.Fatalf("Start(ws_error) error = %v", err)
	}

	completed = waitForState(t, manager, completed.RunID, StateCompleted)
	if completed.FinishedAt == nil || completed.Error != "" {
		t.Fatalf("completed status = %+v", completed)
	}
	errored = waitForState(t, manager, errored.RunID, StateError)
	if errored.FinishedAt == nil || errored.Error != "boom" {
		t.Fatalf("error status = %+v", errored)
	}
}

func TestManagerRejectsUnknownWorkspaceAndRun(t *testing.T) {
	manager := newTestManager(t, LifecycleExecutor{})
	if _, err := manager.Start("ws_missing"); err == nil {
		t.Fatal("Start(ws_missing) error = nil, want error")
	}
	if _, err := manager.Get("run_missing"); !errors.Is(err, ErrRunNotFound) {
		t.Fatalf("Get missing error = %v, want ErrRunNotFound", err)
	}
	if _, err := manager.Cancel("run_missing"); !errors.Is(err, ErrRunNotFound) {
		t.Fatalf("Cancel missing error = %v, want ErrRunNotFound", err)
	}
}

type executorFunc struct {
	name string
	run  func(context.Context, RunSpec) error
}

func (e executorFunc) Name() string { return e.name }
func (e executorFunc) Run(ctx context.Context, spec RunSpec) error {
	return e.run(ctx, spec)
}

func newTestManager(t *testing.T, executor Executor) *Manager {
	t.Helper()
	manager, err := NewManager(func(id string) (model.Workspace, error) {
		switch id {
		case "ws_a", "ws_b", "ws_ok", "ws_error":
			return model.Workspace{ID: id, Path: t.TempDir()}, nil
		default:
			return model.Workspace{}, errors.New("workspace not found")
		}
	}, executor)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}
	t.Cleanup(manager.StopAll)
	return manager
}

func waitForState(t *testing.T, manager *Manager, runID string, want State) RunStatus {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		status, err := manager.Get(runID)
		if err != nil {
			t.Fatalf("Get(%s) error = %v", runID, err)
		}
		if status.State == want {
			return status
		}
		time.Sleep(5 * time.Millisecond)
	}
	status, _ := manager.Get(runID)
	t.Fatalf("run %s state = %s, want %s", runID, status.State, want)
	return RunStatus{}
}
