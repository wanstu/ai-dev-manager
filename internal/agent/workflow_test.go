package agent

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"ai-dev-manager/internal/adapter/runtimeadapter"
	"ai-dev-manager/internal/model"
)

func TestVerifyWorkflowPassAndFailReview(t *testing.T) {
	tests := []struct {
		name     string
		statuses []string
		want     ReviewDecision
	}{
		{name: "pass", statuses: []string{"passed", "skipped"}, want: ReviewPass},
		{name: "fail", statuses: []string{"passed", "failed"}, want: ReviewFail},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runtime := newFakeWorkflowRuntime(tt.statuses)
			executor, err := NewVerifyWorkflowExecutor(func(string) (runtimeadapter.Runtime, error) { return runtime, nil })
			if err != nil {
				t.Fatalf("NewVerifyWorkflowExecutor() error = %v", err)
			}
			manager := newWorkflowManager(t, executor)
			started, err := manager.StartWithExecutor("ws_a", "verify")
			if err != nil {
				t.Fatalf("StartWithExecutor() error = %v", err)
			}
			status := waitForState(t, manager, started.RunID, StateCompleted)
			if status.Workflow != "verify" || status.Stage != StageCompleted {
				t.Fatalf("workflow status = %+v", status)
			}
			if status.Plan == nil || len(status.Plan.Steps) != 3 || len(status.Steps) != 3 {
				t.Fatalf("workflow audit trail incomplete: %+v", status)
			}
			if status.Review == nil || status.Review.Decision != tt.want {
				t.Fatalf("review = %+v, want %s", status.Review, tt.want)
			}
			if tt.want == ReviewFail && status.State != StateCompleted {
				t.Fatalf("review fail changed orchestration state: %+v", status)
			}
		})
	}
}

func TestVerifyWorkflowMissingCapabilityBecomesRunError(t *testing.T) {
	runtime := newFakeWorkflowRuntime([]string{"passed"})
	runtime.caps = []string{runtimeadapter.OpGitStatus, runtimeadapter.OpGitDiff}
	executor, err := NewVerifyWorkflowExecutor(func(string) (runtimeadapter.Runtime, error) { return runtime, nil })
	if err != nil {
		t.Fatal(err)
	}
	manager := newWorkflowManager(t, executor)
	started, err := manager.StartWithExecutor("ws_a", "verify")
	if err != nil {
		t.Fatalf("StartWithExecutor() error = %v", err)
	}
	status := waitForState(t, manager, started.RunID, StateError)
	if !strings.Contains(status.Error, `runtime capability "verify.run" is required`) {
		t.Fatalf("run error = %q", status.Error)
	}
	if status.Review != nil {
		t.Fatalf("unexpected review on planning error: %+v", status.Review)
	}
}

func TestVerifyWorkflowCancelWinsOverLateExecutorError(t *testing.T) {
	entered := make(chan struct{}, 1)
	runtime := newFakeWorkflowRuntime([]string{"passed"})
	runtime.invoke = func(ctx context.Context, operation string, _ map[string]any) (any, error) {
		if operation == runtimeadapter.OpGitStatus {
			select {
			case entered <- struct{}{}:
			default:
			}
			<-ctx.Done()
			return nil, ctx.Err()
		}
		return map[string]any{}, nil
	}
	executor, err := NewVerifyWorkflowExecutor(func(string) (runtimeadapter.Runtime, error) { return runtime, nil })
	if err != nil {
		t.Fatal(err)
	}
	manager := newWorkflowManager(t, executor)
	started, err := manager.StartWithExecutor("ws_a", "verify")
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-entered:
	case <-time.After(2 * time.Second):
		t.Fatal("workflow did not enter executor")
	}
	cancelled, err := manager.Cancel(started.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if cancelled.State != StateCancelled {
		t.Fatalf("Cancel() state = %s", cancelled.State)
	}
	time.Sleep(20 * time.Millisecond)
	status, err := manager.Get(started.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if status.State != StateCancelled || status.Error != "" {
		t.Fatalf("late executor return overwrote cancellation: %+v", status)
	}
}

func TestGSDWorkflowExecutesAuditedPlanAndVerifier(t *testing.T) {
	invoked := make([]string, 0)
	runtime := newFakeGSDRuntime(gsdPlanText(`{
  "steps": [{
    "id": "edit-sample",
    "operation": "files.edit",
    "purpose": "apply planned edit",
    "input": {"path":"sample.txt","old_text":"before","new_text":"after","expected_replacements":1}
  }],
  "run_verifiers": true
}`))
	runtime.invokeStep = func(_ context.Context, operation string, _ map[string]any) (any, error) {
		invoked = append(invoked, operation)
		switch operation {
		case runtimeadapter.OpEdit:
			return map[string]any{"path": "sample.txt", "replacements": 1}, nil
		case runtimeadapter.OpVerifierRunMany:
			return []map[string]any{{"id": "test", "status": "passed"}}, nil
		default:
			return nil, errors.New("unexpected execution operation")
		}
	}
	executor, err := NewGSDWorkflowExecutor(func(string) (runtimeadapter.Runtime, error) { return runtime, nil })
	if err != nil {
		t.Fatal(err)
	}
	manager := newWorkflowManager(t, executor)
	started, err := manager.StartWithExecutor("ws_a", "gsd")
	if err != nil {
		t.Fatal(err)
	}
	status := waitForState(t, manager, started.RunID, StateCompleted)
	if status.Workflow != "gsd" || status.Plan == nil || status.Plan.Planning == nil {
		t.Fatalf("missing GSD planning trace: %+v", status)
	}
	if status.Plan.Planning.Phase != 17 || status.Plan.Planning.PlanID != "17-01" || !strings.Contains(status.Plan.Planning.ContextPath, "17-CONTEXT.md") {
		t.Fatalf("unexpected GSD planning sources: %+v", status.Plan.Planning)
	}
	if len(status.Steps) != 2 || status.Steps[0].Operation != runtimeadapter.OpEdit || status.Steps[1].Operation != runtimeadapter.OpVerifierRunMany {
		t.Fatalf("unexpected GSD steps: %+v", status.Steps)
	}
	if status.Review == nil || status.Review.Decision != ReviewPass {
		t.Fatalf("unexpected GSD review: %+v", status.Review)
	}
	if len(invoked) != 2 || invoked[0] != runtimeadapter.OpEdit || invoked[1] != runtimeadapter.OpVerifierRunMany {
		t.Fatalf("execution order = %+v", invoked)
	}
}

func TestGSDWorkflowRejectsForbiddenOperationBeforeExecution(t *testing.T) {
	runtime := newFakeGSDRuntime(gsdPlanText(`{
  "steps": [{"id":"bad","operation":"shell.exec","purpose":"must not run","input":{"executable":"cmd"}}],
  "run_verifiers": false
}`))
	called := false
	runtime.invokeStep = func(_ context.Context, _ string, _ map[string]any) (any, error) {
		called = true
		return nil, nil
	}
	executor, err := NewGSDWorkflowExecutor(func(string) (runtimeadapter.Runtime, error) { return runtime, nil })
	if err != nil {
		t.Fatal(err)
	}
	manager := newWorkflowManager(t, executor)
	started, err := manager.StartWithExecutor("ws_a", "gsd")
	if err != nil {
		t.Fatal(err)
	}
	status := waitForState(t, manager, started.RunID, StateError)
	if !strings.Contains(status.Error, `GSD operation "shell.exec" is not allowed`) {
		t.Fatalf("unexpected error: %s", status.Error)
	}
	if called {
		t.Fatal("forbidden GSD operation reached execution")
	}
}

func TestGSDWorkflowRejectsAmbiguousPhaseAndMissingCapability(t *testing.T) {
	t.Run("ambiguous phase", func(t *testing.T) {
		runtime := newFakeGSDRuntime(gsdPlanText(`{"steps":[{"id":"read","operation":"files.read","input":{"path":"sample.txt"}}],"run_verifiers":false}`))
		runtime.phaseDirs = []string{".planning/phases/17-one", ".planning/phases/17-two"}
		executor, _ := NewGSDWorkflowExecutor(func(string) (runtimeadapter.Runtime, error) { return runtime, nil })
		manager := newWorkflowManager(t, executor)
		started, _ := manager.StartWithExecutor("ws_a", "gsd")
		status := waitForState(t, manager, started.RunID, StateError)
		if !strings.Contains(status.Error, "found 2 matches") {
			t.Fatalf("unexpected ambiguous phase error: %s", status.Error)
		}
	})

	t.Run("missing edit capability", func(t *testing.T) {
		runtime := newFakeGSDRuntime(gsdPlanText(`{"steps":[{"id":"edit","operation":"files.edit","input":{"path":"sample.txt","old_text":"a","new_text":"b","expected_replacements":1}}],"run_verifiers":false}`))
		runtime.caps = []string{runtimeadapter.OpRead, runtimeadapter.OpTree}
		executor, _ := NewGSDWorkflowExecutor(func(string) (runtimeadapter.Runtime, error) { return runtime, nil })
		manager := newWorkflowManager(t, executor)
		started, _ := manager.StartWithExecutor("ws_a", "gsd")
		status := waitForState(t, manager, started.RunID, StateError)
		if !strings.Contains(status.Error, `runtime capability "files.edit" is required`) {
			t.Fatalf("unexpected capability error: %s", status.Error)
		}
	})
}

func TestManagerRejectsUnknownExecutorAndKeepsLifecycleDefault(t *testing.T) {
	manager := newTestManager(t, LifecycleExecutor{})
	if _, err := manager.StartWithExecutor("ws_a", "missing"); err == nil {
		t.Fatal("unknown executor unexpectedly succeeded")
	}
	status, err := manager.Start("ws_a")
	if err != nil {
		t.Fatal(err)
	}
	if status.Executor != "lifecycle" || status.State != StateRunning {
		t.Fatalf("default lifecycle changed: %+v", status)
	}
}

type fakeWorkflowRuntime struct {
	caps   []string
	invoke func(context.Context, string, map[string]any) (any, error)
}

func newFakeWorkflowRuntime(statuses []string) *fakeWorkflowRuntime {
	runtime := &fakeWorkflowRuntime{
		caps: []string{runtimeadapter.OpGitStatus, runtimeadapter.OpGitDiff, "verify.run"},
	}
	runtime.invoke = func(_ context.Context, operation string, _ map[string]any) (any, error) {
		switch operation {
		case runtimeadapter.OpGitStatus:
			return map[string]any{"clean": false}, nil
		case runtimeadapter.OpGitDiff:
			return map[string]any{"files": 1}, nil
		case runtimeadapter.OpVerifierRunMany:
			rows := make([]map[string]any, 0, len(statuses))
			for i, status := range statuses {
				rows = append(rows, map[string]any{"id": string(rune('a' + i)), "status": status})
			}
			return rows, nil
		default:
			return nil, errors.New("unexpected operation")
		}
	}
	return runtime
}

func (r *fakeWorkflowRuntime) ID() string          { return "fake:ws_a" }
func (r *fakeWorkflowRuntime) WorkspaceID() string { return "ws_a" }
func (r *fakeWorkflowRuntime) Capabilities() []string {
	return append([]string(nil), r.caps...)
}
func (r *fakeWorkflowRuntime) Status(context.Context) runtimeadapter.Status {
	return runtimeadapter.Status{ID: r.ID(), WorkspaceID: r.WorkspaceID(), State: runtimeadapter.StateReady, Capabilities: r.Capabilities()}
}
func (r *fakeWorkflowRuntime) Invoke(ctx context.Context, operation string, input map[string]any) (any, error) {
	return r.invoke(ctx, operation, input)
}

type fakeGSDRuntime struct {
	caps       []string
	planText   string
	phaseDirs  []string
	invokeStep func(context.Context, string, map[string]any) (any, error)
}

func newFakeGSDRuntime(planText string) *fakeGSDRuntime {
	return &fakeGSDRuntime{
		caps: []string{
			runtimeadapter.OpRead,
			runtimeadapter.OpTree,
			runtimeadapter.OpEdit,
			"verify.run",
		},
		planText:  planText,
		phaseDirs: []string{".planning/phases/17-gsd-phase-executor"},
	}
}

func (r *fakeGSDRuntime) ID() string          { return "fake-gsd:ws_a" }
func (r *fakeGSDRuntime) WorkspaceID() string { return "ws_a" }
func (r *fakeGSDRuntime) Capabilities() []string {
	return append([]string(nil), r.caps...)
}
func (r *fakeGSDRuntime) Status(context.Context) runtimeadapter.Status {
	return runtimeadapter.Status{ID: r.ID(), WorkspaceID: r.WorkspaceID(), State: runtimeadapter.StateReady, Capabilities: r.Capabilities()}
}
func (r *fakeGSDRuntime) Invoke(ctx context.Context, operation string, input map[string]any) (any, error) {
	switch operation {
	case runtimeadapter.OpRead:
		path, _ := input["path"].(string)
		switch filepath.Clean(path) {
		case filepath.Clean(".planning/STATE.md"):
			return map[string]any{"content": "## Current Position\n\nPhase: 17 — GSD Phase Executor\nPlan: 17-01 — Loader\nStatus: In Progress\n"}, nil
		case filepath.Clean(".planning/PROJECT.md"):
			return map[string]any{"content": "# Project\n"}, nil
		case filepath.Clean(".planning/phases/17-gsd-phase-executor/17-CONTEXT.md"):
			return map[string]any{"content": "# Context\n"}, nil
		case filepath.Clean(".planning/phases/17-gsd-phase-executor/17-01-PLAN.md"):
			return map[string]any{"content": r.planText}, nil
		default:
			if r.invokeStep != nil {
				return r.invokeStep(ctx, operation, input)
			}
			return nil, errors.New("unexpected read path")
		}
	case runtimeadapter.OpTree:
		rows := make([]map[string]any, 0, len(r.phaseDirs))
		for _, path := range r.phaseDirs {
			rows = append(rows, map[string]any{"Path": path, "IsDir": true, "Size": 0})
		}
		return rows, nil
	default:
		if r.invokeStep != nil {
			return r.invokeStep(ctx, operation, input)
		}
		return nil, errors.New("unexpected GSD execution operation")
	}
}

func gsdPlanText(spec string) string {
	return "# Plan\n\n## Execution Spec\n\n```json\n" + spec + "\n```\n"
}

func newWorkflowManager(t *testing.T, verify Executor) *Manager {
	t.Helper()
	manager, err := NewManager(func(id string) (model.Workspace, error) {
		if id != "ws_a" {
			return model.Workspace{}, errors.New("workspace not found")
		}
		return model.Workspace{ID: id, Path: t.TempDir()}, nil
	}, LifecycleExecutor{})
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.RegisterExecutor(verify); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(manager.StopAll)
	return manager
}
