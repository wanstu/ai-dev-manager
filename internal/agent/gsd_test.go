package agent

import (
	"context"
	"errors"
	"strings"
	"testing"

	"ai-dev-manager/internal/adapter/runtimeadapter"
)

func TestGSDAdvancerSamePhaseAndNextPhase(t *testing.T) {
	t.Run("same phase next plan", func(t *testing.T) {
		runtime := newAdvancerRuntime()
		runtime.state = "## Current Position\n\nPhase: 17 — GSD Phase Executor\nPlan: 17-01 — Loader\nStatus: In Progress\n"
		runtime.trees[cleanPath(".planning/phases/17-gsd-phase-executor")] = []map[string]any{
			{"Path": cleanPath(".planning/phases/17-gsd-phase-executor/17-01-PLAN.md"), "IsDir": false},
			{"Path": cleanPath(".planning/phases/17-gsd-phase-executor/17-02-PLAN.md"), "IsDir": false},
		}
		runtime.files[cleanPath(".planning/phases/17-gsd-phase-executor/17-02-PLAN.md")] = "# Phase 17 Plan 17-02 — Verified State Transition\n"

		result, err := (GSDAdvancer{}).Complete(context.Background(), RunSpec{}, runtime, Plan{Planning: &PlanningSources{
			Phase: 17, PlanID: "17-01", StatePath: ".planning/STATE.md",
			ContextPath: cleanPath(".planning/phases/17-gsd-phase-executor/17-CONTEXT.md"),
		}}, ReviewResult{Decision: ReviewPass})
		if err != nil {
			t.Fatalf("Complete() error = %v", err)
		}
		if result.Status != AdvanceAdvanced || result.ToPhase != 17 || result.ToPlan != "17-02" {
			t.Fatalf("advance result = %+v", result)
		}
		if !strings.Contains(runtime.state, "Plan: 17-02 — Verified State Transition") {
			t.Fatalf("STATE not advanced: %s", runtime.state)
		}
	})

	t.Run("next phase first plan", func(t *testing.T) {
		runtime := newAdvancerRuntime()
		runtime.state = "## Current Position\n\nPhase: 17 — GSD Phase Executor\nPlan: 17-02 — Verified State Transition\nStatus: In Progress\n"
		runtime.trees[cleanPath(".planning/phases/17-gsd-phase-executor")] = []map[string]any{
			{"Path": cleanPath(".planning/phases/17-gsd-phase-executor/17-01-PLAN.md"), "IsDir": false},
			{"Path": cleanPath(".planning/phases/17-gsd-phase-executor/17-02-PLAN.md"), "IsDir": false},
		}
		runtime.trees[cleanPath(".planning/phases")] = []map[string]any{
			{"Path": cleanPath(".planning/phases/17-gsd-phase-executor"), "IsDir": true},
			{"Path": cleanPath(".planning/phases/18-parallel-agents"), "IsDir": true},
		}
		runtime.files[cleanPath(".planning/phases/18-parallel-agents/18-CONTEXT.md")] = "# Phase 18 Context — Parallel Agents / Worktrees\n"
		runtime.trees[cleanPath(".planning/phases/18-parallel-agents")] = []map[string]any{
			{"Path": cleanPath(".planning/phases/18-parallel-agents/18-01-PLAN.md"), "IsDir": false},
		}
		runtime.files[cleanPath(".planning/phases/18-parallel-agents/18-01-PLAN.md")] = "# Phase 18 Plan 18-01 — Managed Parallel Runs\n"

		result, err := (GSDAdvancer{}).Complete(context.Background(), RunSpec{}, runtime, Plan{Planning: &PlanningSources{
			Phase: 17, PlanID: "17-02", StatePath: ".planning/STATE.md",
			ContextPath: cleanPath(".planning/phases/17-gsd-phase-executor/17-CONTEXT.md"),
		}}, ReviewResult{Decision: ReviewPass})
		if err != nil {
			t.Fatalf("Complete() error = %v", err)
		}
		if result.Status != AdvanceAdvanced || result.ToPhase != 18 || result.ToPlan != "18-01" {
			t.Fatalf("advance result = %+v", result)
		}
		if !strings.Contains(runtime.state, "Phase: 18 — Parallel Agents / Worktrees") || !strings.Contains(runtime.state, "Plan: 18-01 — Managed Parallel Runs") {
			t.Fatalf("STATE not advanced to next phase: %s", runtime.state)
		}
	})
}

func TestGSDAdvancerBlockedSkippedAndCapabilityBoundary(t *testing.T) {
	t.Run("missing next phase blocks", func(t *testing.T) {
		runtime := newAdvancerRuntime()
		runtime.state = "Phase: 17 — GSD Phase Executor\nPlan: 17-02 — Verified State Transition\nStatus: In Progress\n"
		runtime.trees[cleanPath(".planning/phases/17-gsd-phase-executor")] = []map[string]any{
			{"Path": cleanPath(".planning/phases/17-gsd-phase-executor/17-02-PLAN.md"), "IsDir": false},
		}
		runtime.trees[cleanPath(".planning/phases")] = []map[string]any{
			{"Path": cleanPath(".planning/phases/17-gsd-phase-executor"), "IsDir": true},
		}
		before := runtime.state
		result, err := (GSDAdvancer{}).Complete(context.Background(), RunSpec{}, runtime, gsdAdvancePlan("17-02"), ReviewResult{Decision: ReviewPass})
		if err != nil {
			t.Fatalf("Complete() error = %v", err)
		}
		if result.Status != AdvanceBlocked || !strings.Contains(result.Reason, "next phase 18 planning directory does not exist") {
			t.Fatalf("blocked result = %+v", result)
		}
		if runtime.state != before {
			t.Fatalf("blocked advance changed STATE: %q", runtime.state)
		}
	})

	t.Run("ambiguous next phase blocks", func(t *testing.T) {
		runtime := newAdvancerRuntime()
		runtime.state = "Phase: 17 — GSD Phase Executor\nPlan: 17-02 — Verified State Transition\nStatus: In Progress\n"
		runtime.trees[cleanPath(".planning/phases/17-gsd-phase-executor")] = []map[string]any{
			{"Path": cleanPath(".planning/phases/17-gsd-phase-executor/17-02-PLAN.md"), "IsDir": false},
		}
		runtime.trees[cleanPath(".planning/phases")] = []map[string]any{
			{"Path": cleanPath(".planning/phases/18-one"), "IsDir": true},
			{"Path": cleanPath(".planning/phases/18-two"), "IsDir": true},
		}
		result, err := (GSDAdvancer{}).Complete(context.Background(), RunSpec{}, runtime, gsdAdvancePlan("17-02"), ReviewResult{Decision: ReviewPass})
		if err != nil {
			t.Fatalf("Complete() error = %v", err)
		}
		if result.Status != AdvanceBlocked || !strings.Contains(result.Reason, "ambiguous (2 matches)") {
			t.Fatalf("ambiguous result = %+v", result)
		}
	})

	t.Run("review fail skips without runtime access", func(t *testing.T) {
		runtime := newAdvancerRuntime()
		runtime.failAll = true
		result, err := (GSDAdvancer{}).Complete(context.Background(), RunSpec{}, runtime, gsdAdvancePlan("17-02"), ReviewResult{Decision: ReviewFail})
		if err != nil {
			t.Fatalf("Complete() error = %v", err)
		}
		if result.Status != AdvanceSkipped || result.Reason != "review did not pass" {
			t.Fatalf("skipped result = %+v", result)
		}
	})

	t.Run("missing edit capability blocks after target resolution", func(t *testing.T) {
		runtime := newAdvancerRuntime()
		runtime.caps = []string{runtimeadapter.OpRead, runtimeadapter.OpTree}
		runtime.state = "Phase: 17 — GSD Phase Executor\nPlan: 17-01 — Loader\nStatus: In Progress\n"
		runtime.trees[cleanPath(".planning/phases/17-gsd-phase-executor")] = []map[string]any{
			{"Path": cleanPath(".planning/phases/17-gsd-phase-executor/17-01-PLAN.md"), "IsDir": false},
			{"Path": cleanPath(".planning/phases/17-gsd-phase-executor/17-02-PLAN.md"), "IsDir": false},
		}
		runtime.files[cleanPath(".planning/phases/17-gsd-phase-executor/17-02-PLAN.md")] = "# Phase 17 Plan 17-02 — Verified State Transition\n"
		before := runtime.state
		result, err := (GSDAdvancer{}).Complete(context.Background(), RunSpec{}, runtime, gsdAdvancePlan("17-01"), ReviewResult{Decision: ReviewPass})
		if err != nil {
			t.Fatalf("Complete() error = %v", err)
		}
		if result.Status != AdvanceBlocked || result.ToPlan != "17-02" || !strings.Contains(result.Reason, `files.edit`) {
			t.Fatalf("capability result = %+v", result)
		}
		if runtime.state != before {
			t.Fatalf("capability block changed STATE: %q", runtime.state)
		}
	})
}

func gsdAdvancePlan(planID string) Plan {
	return Plan{Planning: &PlanningSources{
		Phase: 17, PlanID: planID, StatePath: ".planning/STATE.md",
		ContextPath: cleanPath(".planning/phases/17-gsd-phase-executor/17-CONTEXT.md"),
	}}
}

type advancerRuntime struct {
	caps    []string
	state   string
	files   map[string]string
	trees   map[string][]map[string]any
	failAll bool
}

func newAdvancerRuntime() *advancerRuntime {
	return &advancerRuntime{
		caps:  []string{runtimeadapter.OpRead, runtimeadapter.OpTree, runtimeadapter.OpEdit},
		files: map[string]string{},
		trees: map[string][]map[string]any{},
	}
}

func (r *advancerRuntime) ID() string          { return "advancer:ws" }
func (r *advancerRuntime) WorkspaceID() string { return "ws" }
func (r *advancerRuntime) Capabilities() []string {
	return append([]string(nil), r.caps...)
}
func (r *advancerRuntime) Status(context.Context) runtimeadapter.Status {
	return runtimeadapter.Status{ID: r.ID(), WorkspaceID: r.WorkspaceID(), State: runtimeadapter.StateReady, Capabilities: r.Capabilities()}
}
func (r *advancerRuntime) Invoke(_ context.Context, operation string, input map[string]any) (any, error) {
	if r.failAll {
		return nil, errors.New("runtime should not have been accessed")
	}
	switch operation {
	case runtimeadapter.OpRead:
		path, _ := input["path"].(string)
		path = cleanPath(path)
		if path == cleanPath(".planning/STATE.md") {
			return map[string]any{"content": r.state}, nil
		}
		content, ok := r.files[path]
		if !ok {
			return nil, errors.New("file not found: " + path)
		}
		return map[string]any{"content": content}, nil
	case runtimeadapter.OpTree:
		path, _ := input["path"].(string)
		return r.trees[cleanPath(path)], nil
	case runtimeadapter.OpEdit:
		path, _ := input["path"].(string)
		if cleanPath(path) != cleanPath(".planning/STATE.md") {
			return nil, errors.New("unexpected edit path")
		}
		oldText, _ := input["old_text"].(string)
		newText, _ := input["new_text"].(string)
		if !strings.Contains(r.state, oldText) {
			return nil, errors.New("old state block not found")
		}
		r.state = strings.Replace(r.state, oldText, newText, 1)
		return map[string]any{"replacements": 1}, nil
	default:
		return nil, errors.New("unexpected operation: " + operation)
	}
}

func cleanPath(path string) string {
	return strings.ReplaceAll(path, "\\", "/")
}
