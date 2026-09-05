package agent

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"ai-dev-manager/internal/model"
)

type State string

const (
	StateRunning   State = "running"
	StateCompleted State = "completed"
	StateCancelled State = "cancelled"
	StateError     State = "error"
)

var ErrRunNotFound = errors.New("agent run not found")

type RunStatus struct {
	RunID       string          `json:"run_id"`
	WorkspaceID string          `json:"workspace_id"`
	Executor    string          `json:"executor"`
	State       State           `json:"state"`
	CreatedAt   time.Time       `json:"created_at"`
	StartedAt   time.Time       `json:"started_at"`
	FinishedAt  *time.Time      `json:"finished_at,omitempty"`
	Error       string          `json:"error,omitempty"`
	Workflow    string          `json:"workflow,omitempty"`
	Stage       Stage           `json:"stage,omitempty"`
	Plan        *Plan           `json:"plan,omitempty"`
	Steps       []StepResult    `json:"steps,omitempty"`
	Review      *ReviewResult   `json:"review,omitempty"`
	Advance     *AdvanceResult  `json:"advance,omitempty"`
	Parallel    *ParallelResult `json:"parallel,omitempty"`
}

type RunSpec struct {
	RunID     string
	Workspace model.Workspace
	Input     map[string]any
	Update    func(RunUpdate)
}

type StartRequest struct {
	WorkspaceID string
	Executor    string
	Input       map[string]any
}

type Executor interface {
	Name() string
	Run(context.Context, RunSpec) error
}

type WorkspaceResolver func(string) (model.Workspace, error)

type managedRun struct {
	status RunStatus
	cancel context.CancelFunc
}

type Manager struct {
	mu              sync.Mutex
	resolve         WorkspaceResolver
	defaultExecutor string
	executors       map[string]Executor
	runs            map[string]*managedRun
}

func NewManager(resolve WorkspaceResolver, executor Executor) (*Manager, error) {
	if resolve == nil {
		return nil, errors.New("agent workspace resolver is required")
	}
	if executor == nil || strings.TrimSpace(executor.Name()) == "" {
		return nil, errors.New("agent executor is required")
	}
	name := strings.TrimSpace(executor.Name())
	return &Manager{
		resolve:         resolve,
		defaultExecutor: name,
		executors:       map[string]Executor{name: executor},
		runs:            map[string]*managedRun{},
	}, nil
}

func (m *Manager) RegisterExecutor(executor Executor) error {
	if executor == nil || strings.TrimSpace(executor.Name()) == "" {
		return errors.New("agent executor is required")
	}
	name := strings.TrimSpace(executor.Name())
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.executors[name]; exists {
		return fmt.Errorf("agent executor %q already registered", name)
	}
	m.executors[name] = executor
	return nil
}

func (m *Manager) Start(workspaceID string) (RunStatus, error) {
	return m.StartRequest(StartRequest{WorkspaceID: workspaceID})
}

func (m *Manager) StartWithExecutor(workspaceID, executorName string) (RunStatus, error) {
	return m.StartRequest(StartRequest{WorkspaceID: workspaceID, Executor: executorName})
}

func (m *Manager) StartRequest(request StartRequest) (RunStatus, error) {
	workspaceID := strings.TrimSpace(request.WorkspaceID)
	if workspaceID == "" {
		return RunStatus{}, errors.New("workspace_id is required")
	}
	workspace, err := m.resolve(workspaceID)
	if err != nil {
		return RunStatus{}, err
	}
	executorName := strings.TrimSpace(request.Executor)
	m.mu.Lock()
	if executorName == "" {
		executorName = m.defaultExecutor
	}
	executor, ok := m.executors[executorName]
	m.mu.Unlock()
	if !ok {
		return RunStatus{}, fmt.Errorf("agent executor %q is not registered", executorName)
	}
	runID, err := newRunID()
	if err != nil {
		return RunStatus{}, err
	}
	now := time.Now().UTC()
	runCtx, cancel := context.WithCancel(context.Background())
	status := RunStatus{
		RunID:       runID,
		WorkspaceID: workspace.ID,
		Executor:    executorName,
		State:       StateRunning,
		CreatedAt:   now,
		StartedAt:   now,
	}
	run := &managedRun{status: status, cancel: cancel}

	m.mu.Lock()
	m.runs[runID] = run
	m.mu.Unlock()

	spec := RunSpec{
		RunID:     runID,
		Workspace: workspace,
		Input:     cloneMap(request.Input),
		Update: func(update RunUpdate) {
			m.applyUpdate(runID, update)
		},
	}
	go m.execute(runCtx, executor, spec)
	return status, nil
}

func (m *Manager) execute(ctx context.Context, executor Executor, spec RunSpec) {
	err := executor.Run(ctx, spec)
	finished := time.Now().UTC()

	m.mu.Lock()
	defer m.mu.Unlock()
	run, ok := m.runs[spec.RunID]
	if !ok || run.status.State == StateCancelled {
		return
	}
	run.status.FinishedAt = &finished
	if err == nil {
		run.status.State = StateCompleted
		run.status.Error = ""
		return
	}
	run.status.State = StateError
	run.status.Error = err.Error()
}

func (m *Manager) applyUpdate(runID string, update RunUpdate) {
	m.mu.Lock()
	defer m.mu.Unlock()
	run, ok := m.runs[runID]
	if !ok || run.status.State != StateRunning {
		return
	}
	if strings.TrimSpace(update.Workflow) != "" {
		run.status.Workflow = strings.TrimSpace(update.Workflow)
	}
	if update.Stage != "" {
		run.status.Stage = update.Stage
	}
	if update.Plan != nil {
		plan := clonePlan(*update.Plan)
		run.status.Plan = &plan
	}
	if update.Steps != nil {
		run.status.Steps = cloneStepResults(update.Steps)
	}
	if update.Review != nil {
		review := *update.Review
		run.status.Review = &review
	}
	if update.Advance != nil {
		advance := *update.Advance
		run.status.Advance = &advance
	}
	if update.Parallel != nil {
		parallel := cloneParallelResult(*update.Parallel)
		run.status.Parallel = &parallel
	}
}

func (m *Manager) Get(runID string) (RunStatus, error) {
	runID = strings.TrimSpace(runID)
	m.mu.Lock()
	defer m.mu.Unlock()
	run, ok := m.runs[runID]
	if !ok {
		return RunStatus{}, fmt.Errorf("%w: %s", ErrRunNotFound, runID)
	}
	return run.status, nil
}

func (m *Manager) List() []RunStatus {
	m.mu.Lock()
	defer m.mu.Unlock()
	statuses := make([]RunStatus, 0, len(m.runs))
	for _, run := range m.runs {
		statuses = append(statuses, run.status)
	}
	sort.Slice(statuses, func(i, j int) bool {
		if statuses[i].CreatedAt.Equal(statuses[j].CreatedAt) {
			return statuses[i].RunID < statuses[j].RunID
		}
		return statuses[i].CreatedAt.Before(statuses[j].CreatedAt)
	})
	return statuses
}

func (m *Manager) Cancel(runID string) (RunStatus, error) {
	runID = strings.TrimSpace(runID)
	m.mu.Lock()
	run, ok := m.runs[runID]
	if !ok {
		m.mu.Unlock()
		return RunStatus{}, fmt.Errorf("%w: %s", ErrRunNotFound, runID)
	}
	if run.status.State != StateRunning {
		status := run.status
		m.mu.Unlock()
		return status, nil
	}
	finished := time.Now().UTC()
	run.status.State = StateCancelled
	run.status.FinishedAt = &finished
	run.status.Error = ""
	cancel := run.cancel
	status := run.status
	m.mu.Unlock()

	cancel()
	return status, nil
}

func (m *Manager) StopAll() {
	m.mu.Lock()
	cancels := make([]context.CancelFunc, 0, len(m.runs))
	finished := time.Now().UTC()
	for _, run := range m.runs {
		if run.status.State != StateRunning {
			continue
		}
		run.status.State = StateCancelled
		run.status.FinishedAt = &finished
		run.status.Error = ""
		cancels = append(cancels, run.cancel)
	}
	m.mu.Unlock()
	for _, cancel := range cancels {
		cancel()
	}
}

func newRunID() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return "run_" + hex.EncodeToString(buf), nil
}

type LifecycleExecutor struct{}

func (LifecycleExecutor) Name() string { return "lifecycle" }

func (LifecycleExecutor) Run(ctx context.Context, _ RunSpec) error {
	<-ctx.Done()
	return ctx.Err()
}
