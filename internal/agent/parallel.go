package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"ai-dev-manager/internal/adapter/runtimeadapter"
)

type ParallelLaneSpec struct {
	Name   string `json:"name"`
	Branch string `json:"branch"`
}

type ParallelLaneState string

const (
	ParallelLanePending   ParallelLaneState = "pending"
	ParallelLanePreparing ParallelLaneState = "preparing"
	ParallelLaneRunning   ParallelLaneState = "running"
	ParallelLaneCompleted ParallelLaneState = "completed"
	ParallelLaneError     ParallelLaneState = "error"
)

type ParallelLaneStatus struct {
	Name         string            `json:"name"`
	Branch       string            `json:"branch"`
	WorktreePath string            `json:"worktree_path,omitempty"`
	State        ParallelLaneState `json:"state"`
	StartedAt    *time.Time        `json:"started_at,omitempty"`
	FinishedAt   *time.Time        `json:"finished_at,omitempty"`
	Plan         *Plan             `json:"plan,omitempty"`
	Steps        []StepResult      `json:"steps,omitempty"`
	Review       *ReviewResult     `json:"review,omitempty"`
	Error        string            `json:"error,omitempty"`
	Cleanup      string            `json:"cleanup,omitempty"`
}

type ParallelResult struct {
	KeepWorktrees bool                 `json:"keep_worktrees"`
	Lanes         []ParallelLaneStatus `json:"lanes"`
}

type DerivedRuntimeBuilder func(baseWorkspaceID, derivedID, path string) (runtimeadapter.Runtime, error)

type ParallelVerifyExecutor struct {
	baseBuilder    RuntimeBuilder
	derivedBuilder DerivedRuntimeBuilder
}

func NewParallelVerifyExecutor(baseBuilder RuntimeBuilder, derivedBuilder DerivedRuntimeBuilder) (*ParallelVerifyExecutor, error) {
	if baseBuilder == nil || derivedBuilder == nil {
		return nil, errors.New("parallel verify runtime builders are required")
	}
	return &ParallelVerifyExecutor{baseBuilder: baseBuilder, derivedBuilder: derivedBuilder}, nil
}

func (e *ParallelVerifyExecutor) Name() string { return "parallel-verify" }

func (e *ParallelVerifyExecutor) Run(ctx context.Context, spec RunSpec) error {
	input, err := decodeParallelVerifyInput(spec.Input)
	if err != nil {
		return err
	}
	base, err := e.baseBuilder(spec.Workspace.ID)
	if err != nil {
		return fmt.Errorf("build parallel base runtime: %w", err)
	}
	if !hasCapability(base, "git.worktree") {
		return errors.New(`runtime capability "git.worktree" is required for parallel verify`)
	}

	result := ParallelResult{KeepWorktrees: input.KeepWorktrees, Lanes: make([]ParallelLaneStatus, len(input.Lanes))}
	for i, lane := range input.Lanes {
		result.Lanes[i] = ParallelLaneStatus{Name: lane.Name, Branch: lane.Branch, State: ParallelLanePending}
	}
	publishParallel(spec, StagePlanning, nil, result)

	laneRuntimes := make([]runtimeadapter.Runtime, len(input.Lanes))
	created := make([]bool, len(input.Lanes))
	for i, lane := range input.Lanes {
		if err := ctx.Err(); err != nil {
			return err
		}
		result.Lanes[i].State = ParallelLanePreparing
		publishParallel(spec, StagePlanning, nil, result)
		output, err := base.Invoke(ctx, runtimeadapter.OpGitWorktreeCreate, map[string]any{"name": lane.Name, "branch": lane.Branch})
		if err != nil {
			result.Lanes[i].State = ParallelLaneError
			result.Lanes[i].Error = err.Error()
			publishParallel(spec, StagePlanning, nil, result)
			cleanupErr := cleanupParallelWorktrees(ctx, base, &result, created, input.KeepWorktrees)
			return errors.Join(fmt.Errorf("create lane %s worktree: %w", lane.Name, err), cleanupErr)
		}
		path, err := parallelWorktreePath(output)
		if err != nil {
			result.Lanes[i].State = ParallelLaneError
			result.Lanes[i].Error = err.Error()
			publishParallel(spec, StagePlanning, nil, result)
			cleanupErr := cleanupParallelWorktrees(ctx, base, &result, created, input.KeepWorktrees)
			return errors.Join(fmt.Errorf("decode lane %s worktree: %w", lane.Name, err), cleanupErr)
		}
		created[i] = true
		result.Lanes[i].WorktreePath = path
		derivedID := spec.RunID + ":lane:" + lane.Name
		derived, err := e.derivedBuilder(spec.Workspace.ID, derivedID, path)
		if err != nil {
			result.Lanes[i].State = ParallelLaneError
			result.Lanes[i].Error = err.Error()
			publishParallel(spec, StagePlanning, nil, result)
			cleanupErr := cleanupParallelWorktrees(ctx, base, &result, created, input.KeepWorktrees)
			return errors.Join(fmt.Errorf("build lane %s runtime: %w", lane.Name, err), cleanupErr)
		}
		laneRuntimes[i] = derived
		result.Lanes[i].State = ParallelLaneRunning
		now := time.Now().UTC()
		result.Lanes[i].StartedAt = &now
	}
	publishParallel(spec, StageExecuting, nil, result)

	var mu sync.Mutex
	var wg sync.WaitGroup
	laneErrors := make([]error, len(input.Lanes))
	for i := range input.Lanes {
		i := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			laneRuntime := laneRuntimes[i]
			plan, err := (VerifyPlanner{}).Plan(ctx, spec, laneRuntime)
			if err != nil {
				mu.Lock()
				laneErrors[i] = fmt.Errorf("lane %s plan: %w", input.Lanes[i].Name, err)
				result.Lanes[i].State = ParallelLaneError
				result.Lanes[i].Error = err.Error()
				finished := time.Now().UTC()
				result.Lanes[i].FinishedAt = &finished
				snapshot := cloneParallelResult(result)
				mu.Unlock()
				publishParallel(spec, StageExecuting, nil, snapshot)
				return
			}
			mu.Lock()
			planCopy := clonePlan(plan)
			result.Lanes[i].Plan = &planCopy
			snapshot := cloneParallelResult(result)
			mu.Unlock()
			publishParallel(spec, StageExecuting, nil, snapshot)

			steps, err := (RuntimeStepExecutor{}).Execute(ctx, laneRuntime, plan, func(current []StepResult) {
				mu.Lock()
				result.Lanes[i].Steps = cloneStepResults(current)
				snapshot := cloneParallelResult(result)
				mu.Unlock()
				publishParallel(spec, StageExecuting, nil, snapshot)
			})
			if err != nil {
				mu.Lock()
				laneErrors[i] = fmt.Errorf("lane %s execute: %w", input.Lanes[i].Name, err)
				result.Lanes[i].State = ParallelLaneError
				result.Lanes[i].Error = err.Error()
				result.Lanes[i].Steps = cloneStepResults(steps)
				finished := time.Now().UTC()
				result.Lanes[i].FinishedAt = &finished
				snapshot := cloneParallelResult(result)
				mu.Unlock()
				publishParallel(spec, StageExecuting, nil, snapshot)
				return
			}
			review, err := (VerifyReviewer{}).Review(ctx, spec, plan, steps)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				laneErrors[i] = fmt.Errorf("lane %s review: %w", input.Lanes[i].Name, err)
				result.Lanes[i].State = ParallelLaneError
				result.Lanes[i].Error = err.Error()
			} else {
				reviewCopy := review
				result.Lanes[i].Review = &reviewCopy
				result.Lanes[i].State = ParallelLaneCompleted
			}
			result.Lanes[i].Steps = cloneStepResults(steps)
			finished := time.Now().UTC()
			result.Lanes[i].FinishedAt = &finished
		}()
	}
	wg.Wait()

	mu.Lock()
	snapshot := cloneParallelResult(result)
	mu.Unlock()
	publishParallel(spec, StageReviewing, nil, snapshot)

	var infraErrors []error
	for _, laneErr := range laneErrors {
		if laneErr != nil {
			infraErrors = append(infraErrors, laneErr)
		}
	}
	if len(infraErrors) > 0 {
		cleanupErr := cleanupParallelWorktrees(ctx, base, &result, created, input.KeepWorktrees)
		publishParallel(spec, StageReviewing, nil, result)
		return errors.Join(append(infraErrors, cleanupErr)...)
	}

	parentReview := ReviewResult{Decision: ReviewPass, Summary: fmt.Sprintf("all %d parallel verify lanes passed", len(result.Lanes))}
	failed := 0
	for _, lane := range result.Lanes {
		if lane.Review != nil && lane.Review.Decision == ReviewFail {
			failed++
		}
	}
	if failed > 0 {
		parentReview.Decision = ReviewFail
		parentReview.Summary = fmt.Sprintf("%d of %d parallel verify lanes failed review", failed, len(result.Lanes))
	}
	publishParallel(spec, StageReviewing, &parentReview, result)

	if err := cleanupParallelWorktrees(ctx, base, &result, created, input.KeepWorktrees); err != nil {
		publishParallel(spec, StageReviewing, &parentReview, result)
		return err
	}
	publishParallel(spec, StageCompleted, &parentReview, result)
	return nil
}

type parallelVerifyInput struct {
	Lanes         []ParallelLaneSpec `json:"lanes"`
	KeepWorktrees bool               `json:"keep_worktrees"`
}

func decodeParallelVerifyInput(input map[string]any) (parallelVerifyInput, error) {
	data, err := json.Marshal(input)
	if err != nil {
		return parallelVerifyInput{}, fmt.Errorf("encode parallel verify input: %w", err)
	}
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	var decoded parallelVerifyInput
	if err := decoder.Decode(&decoded); err != nil {
		return parallelVerifyInput{}, fmt.Errorf("decode parallel verify input: %w", err)
	}
	if len(decoded.Lanes) < 2 || len(decoded.Lanes) > 8 {
		return parallelVerifyInput{}, errors.New("parallel verify requires between 2 and 8 lanes")
	}
	names := map[string]struct{}{}
	branches := map[string]struct{}{}
	for i := range decoded.Lanes {
		decoded.Lanes[i].Name = strings.TrimSpace(decoded.Lanes[i].Name)
		decoded.Lanes[i].Branch = strings.TrimSpace(decoded.Lanes[i].Branch)
		if decoded.Lanes[i].Name == "" || decoded.Lanes[i].Branch == "" {
			return parallelVerifyInput{}, errors.New("parallel verify lane name and branch are required")
		}
		if _, exists := names[decoded.Lanes[i].Name]; exists {
			return parallelVerifyInput{}, fmt.Errorf("duplicate parallel lane name %q", decoded.Lanes[i].Name)
		}
		if _, exists := branches[decoded.Lanes[i].Branch]; exists {
			return parallelVerifyInput{}, fmt.Errorf("duplicate parallel lane branch %q", decoded.Lanes[i].Branch)
		}
		names[decoded.Lanes[i].Name] = struct{}{}
		branches[decoded.Lanes[i].Branch] = struct{}{}
	}
	return decoded, nil
}

func parallelWorktreePath(value any) (string, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	var row map[string]any
	if err := json.Unmarshal(data, &row); err != nil {
		return "", err
	}
	path := strings.TrimSpace(jsonStringField(row, "path"))
	if path == "" {
		return "", errors.New("managed worktree result did not include path")
	}
	return path, nil
}

func publishParallel(spec RunSpec, stage Stage, review *ReviewResult, result ParallelResult) {
	if spec.Update == nil {
		return
	}
	snapshot := cloneParallelResult(result)
	update := RunUpdate{Workflow: "parallel-verify", Stage: stage, Parallel: &snapshot}
	if review != nil {
		reviewCopy := *review
		update.Review = &reviewCopy
	}
	spec.Update(update)
}

func cleanupParallelWorktrees(ctx context.Context, base runtimeadapter.Runtime, result *ParallelResult, created []bool, keep bool) error {
	if keep {
		for i := range result.Lanes {
			if created[i] {
				result.Lanes[i].Cleanup = "preserved"
			}
		}
		return nil
	}
	var errs []error
	for i := range result.Lanes {
		if !created[i] {
			continue
		}
		_, err := base.Invoke(ctx, runtimeadapter.OpGitWorktreeRemove, map[string]any{"name": result.Lanes[i].Name})
		if err != nil {
			result.Lanes[i].Cleanup = "error"
			errs = append(errs, fmt.Errorf("remove lane %s worktree: %w", result.Lanes[i].Name, err))
			continue
		}
		result.Lanes[i].Cleanup = "removed"
	}
	return errors.Join(errs...)
}

func cloneParallelResult(source ParallelResult) ParallelResult {
	clone := source
	clone.Lanes = make([]ParallelLaneStatus, len(source.Lanes))
	for i, lane := range source.Lanes {
		clone.Lanes[i] = lane
		if lane.StartedAt != nil {
			started := *lane.StartedAt
			clone.Lanes[i].StartedAt = &started
		}
		if lane.FinishedAt != nil {
			finished := *lane.FinishedAt
			clone.Lanes[i].FinishedAt = &finished
		}
		if lane.Plan != nil {
			plan := clonePlan(*lane.Plan)
			clone.Lanes[i].Plan = &plan
		}
		clone.Lanes[i].Steps = cloneStepResults(lane.Steps)
		if lane.Review != nil {
			review := *lane.Review
			clone.Lanes[i].Review = &review
		}
	}
	return clone
}
