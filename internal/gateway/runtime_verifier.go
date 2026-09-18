package gateway

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"sort"
	"strings"
	"sync"
	"time"

	"ai-dev-manager-v2/internal/app"
	"ai-dev-manager-v2/internal/identity"
	runtimepkg "ai-dev-manager-v2/internal/runtime"
	"ai-dev-manager-v2/internal/verifier"
)

const verifierRunStopTimeout = 7 * time.Second

type verifierRunState string

const (
	verifierRunRunning   verifierRunState = "running"
	verifierRunSucceeded verifierRunState = "succeeded"
	verifierRunFailed    verifierRunState = "failed"
	verifierRunCanceled  verifierRunState = "canceled"
)

type verifierRunStatus struct {
	ID              string           `json:"id"`
	EnvironmentID   string           `json:"environment_id"`
	VerifierID      string           `json:"verifier_id"`
	VerifierName    string           `json:"verifier_name,omitempty"`
	Kind            string           `json:"kind"`
	State           verifierRunState `json:"state"`
	StartedAt       time.Time        `json:"started_at"`
	CompletedAt     *time.Time       `json:"completed_at,omitempty"`
	Stdout          string           `json:"stdout,omitempty"`
	Stderr          string           `json:"stderr,omitempty"`
	StdoutTruncated bool             `json:"stdout_truncated,omitempty"`
	StderrTruncated bool             `json:"stderr_truncated,omitempty"`
	Result          *verifier.Result `json:"result,omitempty"`
	ErrorKind       string           `json:"error_kind,omitempty"`
	Message         string           `json:"message,omitempty"`
}

type ownedVerifierRun struct {
	id            string
	environmentID string
	writerOwner   string
	prepared      app.PreparedVerifierExecution
	ctx           context.Context
	cancel        context.CancelFunc
	done          chan struct{}

	mu              sync.Mutex
	state           verifierRunState
	startedAt       time.Time
	completedAt     *time.Time
	stdout          *ownerOutputBuffer
	stderr          *ownerOutputBuffer
	result          *verifier.Result
	cancelRequested bool
	errorKind       string
	message         string
}

func (o *runtimeOwner) StartVerifierRun(environmentID, writerOwner, verifierID string, maxOutputBytes int) (verifierRunStatus, error) {
	if o == nil || o.service == nil {
		return verifierRunStatus{}, fmt.Errorf("runtime owner is not initialized")
	}
	prepared, err := o.service.PrepareVerifierExecution(o.processContext(), environmentID, writerOwner, verifierID)
	if err != nil {
		return verifierRunStatus{}, err
	}
	runCtx, cancel := context.WithTimeout(o.processContext(), prepared.Timeout)
	cmd, err := prepared.Runtime.PrepareCommand(runCtx, prepared.Definition.Executable, prepared.Definition.Args, prepared.Definition.Cwd)
	if err != nil {
		if appIsExecutableNotAllowedError(o.service, err) {
			o.service.RecordExecDenial(environmentID, prepared.Definition.Executable, "verifier_async_start", err.Error())
		}
		cancel()
		return verifierRunStatus{}, err
	}
	id, err := identity.New("vfrun")
	if err != nil {
		cancel()
		return verifierRunStatus{}, err
	}
	run := &ownedVerifierRun{
		id:            id,
		environmentID: environmentID,
		writerOwner:   writerOwner,
		prepared:      prepared,
		ctx:           runCtx,
		cancel:        cancel,
		done:          make(chan struct{}),
		state:         verifierRunRunning,
		startedAt:     time.Now().UTC(),
		stdout:        newOwnerOutputBuffer(maxOutputBytes),
		stderr:        newOwnerOutputBuffer(maxOutputBytes),
	}
	cmd.Stdout = run.stdout
	cmd.Stderr = run.stderr
	if err := o.installVerifierRun(run); err != nil {
		cancel()
		return verifierRunStatus{}, err
	}
	go o.executeVerifierRun(run, cmd)
	go o.heartbeatVerifierRun(run)
	return o.verifierRunStatus(run), nil
}

func (o *runtimeOwner) ListVerifierRuns(environmentID string) ([]verifierRunStatus, error) {
	if _, err := o.service.Environments.Get(environmentID); err != nil {
		return nil, err
	}
	o.mu.Lock()
	items := make([]*ownedVerifierRun, 0)
	for _, run := range o.verifierRuns {
		if run.environmentID == environmentID {
			items = append(items, run)
		}
	}
	o.mu.Unlock()
	statuses := make([]verifierRunStatus, 0, len(items))
	for _, run := range items {
		statuses = append(statuses, o.verifierRunStatus(run))
	}
	sort.Slice(statuses, func(i, j int) bool { return statuses[i].StartedAt.Before(statuses[j].StartedAt) })
	return statuses, nil
}

func (o *runtimeOwner) VerifierRunStatus(environmentID, runID string) (verifierRunStatus, error) {
	if _, err := o.service.Environments.Get(environmentID); err != nil {
		return verifierRunStatus{}, err
	}
	run, err := o.verifierRun(environmentID, runID)
	if err != nil {
		return verifierRunStatus{}, err
	}
	return o.verifierRunStatus(run), nil
}

func (o *runtimeOwner) CancelVerifierRun(environmentID, writerOwner, runID string) (verifierRunStatus, error) {
	if _, err := o.service.Environments.RequireWriter(environmentID, writerOwner); err != nil {
		return verifierRunStatus{}, err
	}
	run, err := o.verifierRun(environmentID, runID)
	if err != nil {
		return verifierRunStatus{}, err
	}
	if run.writerOwner != writerOwner {
		return verifierRunStatus{}, fmt.Errorf("verifier run %q belongs to another writer", runID)
	}
	o.requestVerifierRunCancel(run, "", "")
	if err := waitOwnedVerifierRun(run, verifierRunStopTimeout); err != nil {
		return verifierRunStatus{}, err
	}
	return o.verifierRunStatus(run), nil
}

func (o *runtimeOwner) executeVerifierRun(run *ownedVerifierRun, cmd *exec.Cmd) {
	commandStarted := time.Now()
	err := cmd.Start()
	if err == nil {
		err = cmd.Wait()
	}
	duration := time.Since(commandStarted)
	now := time.Now().UTC()
	stdout, _ := run.stdout.Snapshot()
	stderr, _ := run.stderr.Snapshot()
	commandResult := runtimepkg.CommandResult{ExitCode: 0, Stdout: stdout, Stderr: stderr}
	classifyExecErr := err
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			commandResult.ExitCode = exitErr.ExitCode()
			classifyExecErr = nil
		}
	}

	run.mu.Lock()
	canceled := run.cancelRequested || errors.Is(run.ctx.Err(), context.Canceled)
	timedOut := errors.Is(run.ctx.Err(), context.DeadlineExceeded)
	run.mu.Unlock()

	var (
		result      verifier.Result
		classifyErr error
	)
	if !canceled {
		result, classifyErr = verifier.Classify(run.prepared.Definition, commandResult, duration, timedOut, classifyExecErr)
		if classifyErr == nil {
			if touchErr := o.service.Environments.Touch(run.environmentID, run.writerOwner); touchErr != nil {
				classifyErr = fmt.Errorf("writer touch failed: %w", touchErr)
			}
		}
	}

	run.mu.Lock()
	switch {
	case canceled:
		run.state = verifierRunCanceled
	case classifyErr != nil:
		run.state = verifierRunFailed
		if run.errorKind == "" {
			run.errorKind = "verifier_execution_failed"
		}
		if run.message == "" {
			run.message = classifyErr.Error()
		}
	default:
		run.result = &result
		if result.Status == verifier.StatusPassed {
			run.state = verifierRunSucceeded
		} else {
			run.state = verifierRunFailed
		}
		if result.TimedOut && run.errorKind == "" {
			run.errorKind = "timeout"
		}
	}
	run.completedAt = &now
	run.mu.Unlock()

	run.cancel()
	close(run.done)
}

func (o *runtimeOwner) heartbeatVerifierRun(run *ownedVerifierRun) {
	interval := o.service.Environments.WriterLeaseTTL() / 3
	if o.verifierHeartbeatInterval != nil {
		interval = o.verifierHeartbeatInterval(o.service.Environments.WriterLeaseTTL())
	}
	if interval <= 0 {
		interval = time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-run.done:
			return
		case <-ticker.C:
			if _, err := o.service.Environments.HeartbeatWriter(run.environmentID, run.writerOwner); err != nil {
				o.requestVerifierRunCancel(run, "writer_heartbeat_failed", err.Error())
				return
			}
		}
	}
}

func (o *runtimeOwner) installVerifierRun(run *ownedVerifierRun) error {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.closed {
		return fmt.Errorf("runtime owner is closed")
	}
	if o.verifierRuns == nil {
		o.verifierRuns = map[string]*ownedVerifierRun{}
	}
	o.verifierRuns[run.id] = run
	return nil
}

func (o *runtimeOwner) verifierRun(environmentID, runID string) (*ownedVerifierRun, error) {
	o.mu.Lock()
	run := o.verifierRuns[runID]
	o.mu.Unlock()
	if run == nil || run.environmentID != environmentID {
		return nil, fmt.Errorf("verifier run %q not found for environment %q", runID, environmentID)
	}
	return run, nil
}

func (o *runtimeOwner) verifierRunStatus(run *ownedVerifierRun) verifierRunStatus {
	run.mu.Lock()
	defer run.mu.Unlock()
	status := verifierRunStatus{
		ID:            run.id,
		EnvironmentID: run.environmentID,
		VerifierID:    run.prepared.Definition.ID,
		VerifierName:  run.prepared.Definition.Name,
		Kind:          run.prepared.Definition.Kind,
		State:         run.state,
		StartedAt:     run.startedAt,
		CompletedAt:   run.completedAt,
		ErrorKind:     run.errorKind,
		Message:       run.message,
	}
	if run.stdout != nil {
		status.Stdout, status.StdoutTruncated = run.stdout.Snapshot()
	}
	if run.stderr != nil {
		status.Stderr, status.StderrTruncated = run.stderr.Snapshot()
	}
	if run.result != nil {
		result := *run.result
		status.Result = &result
		status.Stdout = result.Stdout
		status.Stderr = result.Stderr
	}
	return status
}

func (o *runtimeOwner) requestVerifierRunCancel(run *ownedVerifierRun, errorKind, message string) {
	run.mu.Lock()
	if run.state != verifierRunRunning {
		run.mu.Unlock()
		return
	}
	run.cancelRequested = true
	if errorKind != "" && run.errorKind == "" {
		run.errorKind = errorKind
	}
	if message != "" && run.message == "" {
		run.message = strings.TrimSpace(message)
	}
	cancel := run.cancel
	run.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func (o *runtimeOwner) dropVerifierRunsForEnvironment(environmentID string) {
	o.mu.Lock()
	var runs []*ownedVerifierRun
	for runID, run := range o.verifierRuns {
		if run.environmentID == environmentID {
			runs = append(runs, run)
			delete(o.verifierRuns, runID)
		}
	}
	o.mu.Unlock()
	for _, run := range runs {
		o.requestVerifierRunCancel(run, "", "")
		_ = waitOwnedVerifierRun(run, verifierRunStopTimeout)
	}
}

func (o *runtimeOwner) closeVerifierRuns(runs []*ownedVerifierRun) error {
	var errs []string
	for _, run := range runs {
		o.requestVerifierRunCancel(run, "", "")
	}
	for _, run := range runs {
		if err := waitOwnedVerifierRun(run, verifierRunStopTimeout); err != nil {
			errs = append(errs, err.Error())
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("verifier run cleanup failed: %s", strings.Join(errs, "; "))
	}
	return nil
}

func waitOwnedVerifierRun(run *ownedVerifierRun, timeout time.Duration) error {
	select {
	case <-run.done:
		return nil
	case <-time.After(timeout):
		return fmt.Errorf("verifier run %q did not stop within %s", run.id, timeout)
	}
}
