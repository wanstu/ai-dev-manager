package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"ai-dev-manager/internal/adapter/runtimeadapter"
)

type Stage string

const (
	StagePlanning  Stage = "planning"
	StageExecuting Stage = "executing"
	StageReviewing Stage = "reviewing"
	StageCompleted Stage = "completed"
)

type ReviewDecision string

const (
	ReviewPass ReviewDecision = "pass"
	ReviewFail ReviewDecision = "fail"
)

type Plan struct {
	Summary  string           `json:"summary"`
	Planning *PlanningSources `json:"planning,omitempty"`
	Steps    []PlanStep       `json:"steps"`
}

type PlanningSources struct {
	Phase       int    `json:"phase"`
	PlanID      string `json:"plan_id"`
	ProjectPath string `json:"project_path"`
	StatePath   string `json:"state_path"`
	ContextPath string `json:"context_path"`
	PlanPath    string `json:"plan_path"`
}

type PlanStep struct {
	ID        string         `json:"id"`
	Operation string         `json:"operation"`
	Purpose   string         `json:"purpose"`
	Input     map[string]any `json:"input,omitempty"`
}

type StepResult struct {
	StepID     string    `json:"step_id"`
	Operation  string    `json:"operation"`
	StartedAt  time.Time `json:"started_at"`
	FinishedAt time.Time `json:"finished_at"`
	Output     any       `json:"output,omitempty"`
	Error      string    `json:"error,omitempty"`
}

type ReviewResult struct {
	Decision ReviewDecision `json:"decision"`
	Summary  string         `json:"summary"`
}

type AdvanceStatus string

const (
	AdvanceAdvanced AdvanceStatus = "advanced"
	AdvanceBlocked  AdvanceStatus = "blocked"
	AdvanceSkipped  AdvanceStatus = "skipped"
)

type AdvanceResult struct {
	Status    AdvanceStatus `json:"status"`
	FromPhase int           `json:"from_phase,omitempty"`
	FromPlan  string        `json:"from_plan,omitempty"`
	ToPhase   int           `json:"to_phase,omitempty"`
	ToPlan    string        `json:"to_plan,omitempty"`
	Reason    string        `json:"reason,omitempty"`
}

type RunUpdate struct {
	Workflow string
	Stage    Stage
	Plan     *Plan
	Steps    []StepResult
	Review   *ReviewResult
	Advance  *AdvanceResult
	Parallel *ParallelResult
}

type Planner interface {
	Plan(context.Context, RunSpec, runtimeadapter.Runtime) (Plan, error)
}

type StepExecutor interface {
	Execute(context.Context, runtimeadapter.Runtime, Plan, func([]StepResult)) ([]StepResult, error)
}

type Reviewer interface {
	Review(context.Context, RunSpec, Plan, []StepResult) (ReviewResult, error)
}

type CompletionHandler interface {
	Complete(context.Context, RunSpec, runtimeadapter.Runtime, Plan, ReviewResult) (AdvanceResult, error)
}

type RuntimeBuilder func(string) (runtimeadapter.Runtime, error)

type WorkflowExecutor struct {
	name      string
	builder   RuntimeBuilder
	planner   Planner
	executor  StepExecutor
	reviewer  Reviewer
	completer CompletionHandler
}

func NewVerifyWorkflowExecutor(builder RuntimeBuilder) (*WorkflowExecutor, error) {
	if builder == nil {
		return nil, errors.New("verify workflow runtime builder is required")
	}
	return &WorkflowExecutor{
		name:     "verify",
		builder:  builder,
		planner:  VerifyPlanner{},
		executor: RuntimeStepExecutor{},
		reviewer: VerifyReviewer{},
	}, nil
}

func (e *WorkflowExecutor) Name() string { return e.name }

func (e *WorkflowExecutor) Run(ctx context.Context, spec RunSpec) error {
	if spec.Update == nil {
		return errors.New("workflow run update sink is required")
	}
	runtime, err := e.builder(spec.Workspace.ID)
	if err != nil {
		return fmt.Errorf("build workflow runtime: %w", err)
	}

	spec.Update(RunUpdate{Workflow: e.name, Stage: StagePlanning})
	plan, err := e.planner.Plan(ctx, spec, runtime)
	if err != nil {
		return fmt.Errorf("plan workflow: %w", err)
	}
	planCopy := clonePlan(plan)
	spec.Update(RunUpdate{Workflow: e.name, Stage: StageExecuting, Plan: &planCopy})

	results, err := e.executor.Execute(ctx, runtime, plan, func(current []StepResult) {
		spec.Update(RunUpdate{Workflow: e.name, Stage: StageExecuting, Steps: cloneStepResults(current)})
	})
	if err != nil {
		return fmt.Errorf("execute workflow: %w", err)
	}

	spec.Update(RunUpdate{Workflow: e.name, Stage: StageReviewing, Steps: cloneStepResults(results)})
	review, err := e.reviewer.Review(ctx, spec, plan, results)
	if err != nil {
		return fmt.Errorf("review workflow: %w", err)
	}
	reviewCopy := review
	update := RunUpdate{Workflow: e.name, Stage: StageCompleted, Review: &reviewCopy}
	if e.completer != nil {
		advance, err := e.completer.Complete(ctx, spec, runtime, plan, review)
		if err != nil {
			return fmt.Errorf("complete workflow: %w", err)
		}
		advanceCopy := advance
		update.Advance = &advanceCopy
	}
	spec.Update(update)
	return nil
}

type VerifyPlanner struct{}

func (VerifyPlanner) Plan(ctx context.Context, _ RunSpec, runtime runtimeadapter.Runtime) (Plan, error) {
	if err := ctx.Err(); err != nil {
		return Plan{}, err
	}
	capabilities := make(map[string]struct{})
	for _, capability := range runtime.Capabilities() {
		capabilities[capability] = struct{}{}
	}
	required := []string{runtimeadapter.OpGitStatus, runtimeadapter.OpGitDiff, "verify.run"}
	for _, capability := range required {
		if _, ok := capabilities[capability]; !ok {
			return Plan{}, fmt.Errorf("runtime capability %q is required", capability)
		}
	}
	return Plan{
		Summary: "Inspect Git changes and run all configured verifiers.",
		Steps: []PlanStep{
			{ID: "git-status", Operation: runtimeadapter.OpGitStatus, Purpose: "Capture structured Git status before review.", Input: map[string]any{}},
			{ID: "git-diff", Operation: runtimeadapter.OpGitDiff, Purpose: "Capture the current structured Git diff.", Input: map[string]any{}},
			{ID: "verifiers", Operation: runtimeadapter.OpVerifierRunMany, Purpose: "Run all configured project verifiers.", Input: map[string]any{"ids": []string{}}},
		},
	}, nil
}

type RuntimeStepExecutor struct{}

func (RuntimeStepExecutor) Execute(ctx context.Context, runtime runtimeadapter.Runtime, plan Plan, report func([]StepResult)) ([]StepResult, error) {
	results := make([]StepResult, 0, len(plan.Steps))
	for _, step := range plan.Steps {
		if err := ctx.Err(); err != nil {
			return results, err
		}
		started := time.Now().UTC()
		output, err := runtime.Invoke(ctx, step.Operation, cloneMap(step.Input))
		finished := time.Now().UTC()
		result := StepResult{StepID: step.ID, Operation: step.Operation, StartedAt: started, FinishedAt: finished}
		if err != nil {
			result.Error = err.Error()
			results = append(results, result)
			if report != nil {
				report(results)
			}
			return results, fmt.Errorf("step %s (%s): %w", step.ID, step.Operation, err)
		}
		result.Output = normalizeJSONValue(output)
		results = append(results, result)
		if report != nil {
			report(results)
		}
	}
	return results, nil
}

type VerifyReviewer struct{}

func (VerifyReviewer) Review(ctx context.Context, _ RunSpec, _ Plan, results []StepResult) (ReviewResult, error) {
	if err := ctx.Err(); err != nil {
		return ReviewResult{}, err
	}
	for _, result := range results {
		if result.Error != "" {
			return ReviewResult{}, fmt.Errorf("step %s failed before review: %s", result.StepID, result.Error)
		}
		if result.Operation != runtimeadapter.OpVerifierRunMany {
			continue
		}
		failed, count, err := verifierFailureSummary(result.Output)
		if err != nil {
			return ReviewResult{}, err
		}
		if failed > 0 {
			return ReviewResult{Decision: ReviewFail, Summary: fmt.Sprintf("%d of %d verifiers failed", failed, count)}, nil
		}
		return ReviewResult{Decision: ReviewPass, Summary: fmt.Sprintf("all %d configured verifiers passed or were skipped", count)}, nil
	}
	return ReviewResult{}, errors.New("verify workflow did not produce verifier results")
}

func verifierFailureSummary(value any) (failed, count int, err error) {
	data, err := json.Marshal(value)
	if err != nil {
		return 0, 0, fmt.Errorf("encode verifier results: %w", err)
	}
	var rows []map[string]any
	if err := json.Unmarshal(data, &rows); err != nil {
		return 0, 0, fmt.Errorf("decode verifier results: %w", err)
	}
	for _, row := range rows {
		count++
		status := ""
		for key, raw := range row {
			if strings.EqualFold(key, "status") {
				status, _ = raw.(string)
				break
			}
		}
		if strings.EqualFold(status, "failed") {
			failed++
		}
	}
	return failed, count, nil
}

func clonePlan(source Plan) Plan {
	clone := source
	if source.Planning != nil {
		planning := *source.Planning
		clone.Planning = &planning
	}
	clone.Steps = make([]PlanStep, len(source.Steps))
	for i, step := range source.Steps {
		clone.Steps[i] = step
		clone.Steps[i].Input = cloneMap(step.Input)
	}
	return clone
}

func cloneStepResults(source []StepResult) []StepResult {
	return append([]StepResult(nil), source...)
}

func cloneMap(source map[string]any) map[string]any {
	if source == nil {
		return nil
	}
	clone := make(map[string]any, len(source))
	for key, value := range source {
		clone[key] = value
	}
	return clone
}

func normalizeJSONValue(value any) any {
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Sprintf("%v", value)
	}
	var normalized any
	if err := json.Unmarshal(data, &normalized); err != nil {
		return fmt.Sprintf("%v", value)
	}
	return normalized
}
