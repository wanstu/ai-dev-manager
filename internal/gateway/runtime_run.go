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

	"ai-dev-manager-v2/internal/identity"
	runtimepkg "ai-dev-manager-v2/internal/runtime"
)

const agentRunStopTimeout = 7 * time.Second

type agentRunState string

const (
	agentRunRunning   agentRunState = "running"
	agentRunSucceeded agentRunState = "succeeded"
	agentRunFailed    agentRunState = "failed"
	agentRunCanceled  agentRunState = "canceled"
)

type agentRunStatus struct {
	ID              string        `json:"id"`
	EnvironmentID   string        `json:"environment_id"`
	State           agentRunState `json:"state"`
	Executable      string        `json:"executable"`
	Args            []string      `json:"args,omitempty"`
	Cwd             string        `json:"cwd,omitempty"`
	StartedAt       time.Time     `json:"started_at"`
	CompletedAt     *time.Time    `json:"completed_at,omitempty"`
	ExitCode        *int          `json:"exit_code,omitempty"`
	Stdout          string        `json:"stdout,omitempty"`
	Stderr          string        `json:"stderr,omitempty"`
	StdoutTruncated bool          `json:"stdout_truncated,omitempty"`
	StderrTruncated bool          `json:"stderr_truncated,omitempty"`
	ErrorKind       string        `json:"error_kind,omitempty"`
	Message         string        `json:"message,omitempty"`
}

type ownedAgentRun struct {
	id            string
	environmentID string
	writerOwner   string
	executable    string
	args          []string
	cwd           string
	timeoutMS     int64
	maxOutput     int
	ctx           context.Context
	cancel        context.CancelFunc
	done          chan struct{}

	mu              sync.Mutex
	state           agentRunState
	startedAt       time.Time
	completedAt     *time.Time
	stdout          *ownerOutputBuffer
	stderr          *ownerOutputBuffer
	result          runtimepkg.CommandResult
	hasResult       bool
	hasExitCode     bool
	cancelRequested bool
	errorKind       string
	message         string
}

func (o *runtimeOwner) StartAgentRun(environmentID, writerOwner, executable string, args []string, cwd string, timeoutMS int64, maxOutputBytes int) (agentRunStatus, error) {
	if o == nil || o.service == nil {
		return agentRunStatus{}, fmt.Errorf("runtime owner is not initialized")
	}
	if _, err := o.service.Environments.RequireWriter(environmentID, writerOwner); err != nil {
		return agentRunStatus{}, err
	}
	rt, _, err := o.service.Runtime(environmentID)
	if err != nil {
		return agentRunStatus{}, err
	}
	normalizedTimeoutMS := timeoutMS
	if normalizedTimeoutMS <= 0 {
		normalizedTimeoutMS = 30000
	}
	runCtx, cancel := context.WithTimeout(o.processContext(), time.Duration(normalizedTimeoutMS)*time.Millisecond)
	o.service.RecordFullAuthorizationBypass(rt, environmentID, executable, "run_start")
	cmd, err := rt.PrepareCommand(runCtx, executable, args, cwd)
	if err != nil {
		if appIsExecutableNotAllowedError(o.service, err) {
			o.service.RecordExecDenial(environmentID, executable, "run_start", err.Error())
		}
		cancel()
		return agentRunStatus{}, err
	}
	o.service.Log("info", "run.start", map[string]string{"environment_id": environmentID, "executable": executable, "surface": "run"})
	runID, err := identity.New("run")
	if err != nil {
		cancel()
		return agentRunStatus{}, err
	}
	run := &ownedAgentRun{
		id:            runID,
		environmentID: environmentID,
		writerOwner:   writerOwner,
		executable:    executable,
		args:          append([]string(nil), args...),
		cwd:           cwd,
		timeoutMS:     normalizedTimeoutMS,
		maxOutput:     maxOutputBytes,
		ctx:           runCtx,
		cancel:        cancel,
		done:          make(chan struct{}),
		stdout:        newOwnerOutputBuffer(maxOutputBytes),
		stderr:        newOwnerOutputBuffer(maxOutputBytes),
		state:         agentRunRunning,
		startedAt:     time.Now().UTC(),
	}
	cmd.Stdout = run.stdout
	cmd.Stderr = run.stderr
	if err := o.installAgentRun(run); err != nil {
		cancel()
		return agentRunStatus{}, err
	}
	go o.executeAgentRun(run, cmd)
	go o.heartbeatAgentRun(run)
	return o.agentRunStatus(run), nil
}

func (o *runtimeOwner) ListAgentRuns(environmentID string) ([]agentRunStatus, error) {
	if _, err := o.service.Environments.Get(environmentID); err != nil {
		return nil, err
	}
	o.mu.Lock()
	items := make([]*ownedAgentRun, 0)
	for _, run := range o.runs {
		if run.environmentID == environmentID {
			items = append(items, run)
		}
	}
	o.mu.Unlock()
	statuses := make([]agentRunStatus, 0, len(items))
	for _, run := range items {
		statuses = append(statuses, o.agentRunStatus(run))
	}
	sort.Slice(statuses, func(i, j int) bool { return statuses[i].StartedAt.Before(statuses[j].StartedAt) })
	return statuses, nil
}

func (o *runtimeOwner) AgentRunStatus(environmentID, runID string) (agentRunStatus, error) {
	run, err := o.agentRun(environmentID, runID)
	if err != nil {
		return agentRunStatus{}, err
	}
	return o.agentRunStatus(run), nil
}

func (o *runtimeOwner) CancelAgentRun(environmentID, writerOwner, runID string) (agentRunStatus, error) {
	if _, err := o.service.Environments.RequireWriter(environmentID, writerOwner); err != nil {
		return agentRunStatus{}, err
	}
	run, err := o.agentRun(environmentID, runID)
	if err != nil {
		return agentRunStatus{}, err
	}
	if run.writerOwner != writerOwner {
		return agentRunStatus{}, fmt.Errorf("run %q belongs to another writer", runID)
	}
	o.requestAgentRunCancel(run, "", "")
	if err := waitOwnedAgentRun(run, agentRunStopTimeout); err != nil {
		return agentRunStatus{}, err
	}
	return o.agentRunStatus(run), nil
}

func (o *runtimeOwner) executeAgentRun(run *ownedAgentRun, cmd *exec.Cmd) {
	err := cmd.Start()
	if err == nil {
		err = cmd.Wait()
	}
	now := time.Now().UTC()
	stdout, stdoutTruncated := run.stdout.Snapshot()
	stderr, stderrTruncated := run.stderr.Snapshot()
	result := runtimepkg.CommandResult{ExitCode: 0, Stdout: stdout, Stderr: stderr}

	run.mu.Lock()
	run.result = result
	run.hasResult = true
	canceled := run.cancelRequested || run.ctx.Err() == context.Canceled
	timedOut := errors.Is(run.ctx.Err(), context.DeadlineExceeded)
	switch {
	case canceled:
		run.state = agentRunCanceled
	case timedOut:
		run.state = agentRunFailed
		if run.errorKind == "" {
			run.errorKind = "timeout"
		}
		if run.message == "" {
			run.message = fmt.Sprintf("command timed out after %dms", run.timeoutMS)
		}
	case err != nil:
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			result.ExitCode = exitErr.ExitCode()
			run.result = result
			run.hasExitCode = true
			run.state = agentRunFailed
			if run.errorKind == "" {
				run.errorKind = "command_failed"
			}
		} else {
			run.state = agentRunFailed
			if run.errorKind == "" {
				run.errorKind = "run_exec_failed"
			}
			if run.message == "" {
				run.message = err.Error()
			}
		}
	case result.ExitCode != 0:
		run.state = agentRunFailed
		run.hasExitCode = true
		if run.errorKind == "" {
			run.errorKind = "command_failed"
		}
	default:
		run.state = agentRunSucceeded
		run.hasExitCode = true
	}
	if stdoutTruncated && run.message == "" {
		run.message = "stdout was truncated to max_output_bytes"
	}
	if stderrTruncated && run.message == "" {
		run.message = "stderr was truncated to max_output_bytes"
	}
	run.completedAt = &now
	finalState := run.state
	finalErrorKind := run.errorKind
	finalExitCode := run.result.ExitCode
	run.mu.Unlock()
	o.service.Log("info", "run.completed", map[string]string{"environment_id": run.environmentID, "executable": run.executable, "surface": "run", "state": string(finalState), "exit_code": fmt.Sprint(finalExitCode), "error_kind": finalErrorKind})

	run.cancel()
	close(run.done)
}

func (o *runtimeOwner) heartbeatAgentRun(run *ownedAgentRun) {
	interval := o.service.Environments.WriterLeaseTTL() / 3
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
				o.requestAgentRunCancel(run, "writer_heartbeat_failed", err.Error())
				return
			}
		}
	}
}

func (o *runtimeOwner) installAgentRun(run *ownedAgentRun) error {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.closed {
		return fmt.Errorf("runtime owner is closed")
	}
	if o.runs == nil {
		o.runs = map[string]*ownedAgentRun{}
	}
	o.runs[run.id] = run
	return nil
}

func (o *runtimeOwner) agentRun(environmentID, runID string) (*ownedAgentRun, error) {
	o.mu.Lock()
	run := o.runs[runID]
	o.mu.Unlock()
	if run == nil || run.environmentID != environmentID {
		return nil, fmt.Errorf("run %q not found for environment %q", runID, environmentID)
	}
	return run, nil
}

func (o *runtimeOwner) agentRunStatus(run *ownedAgentRun) agentRunStatus {
	run.mu.Lock()
	defer run.mu.Unlock()
	status := agentRunStatus{
		ID:            run.id,
		EnvironmentID: run.environmentID,
		State:         run.state,
		Executable:    run.executable,
		Args:          append([]string(nil), run.args...),
		Cwd:           run.cwd,
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
	if run.hasResult {
		status.Stdout = run.result.Stdout
		status.Stderr = run.result.Stderr
	}
	if run.hasExitCode {
		exitCode := run.result.ExitCode
		status.ExitCode = &exitCode
	}
	return status
}

func (o *runtimeOwner) requestAgentRunCancel(run *ownedAgentRun, errorKind, message string) {
	run.mu.Lock()
	if run.state != agentRunRunning {
		run.mu.Unlock()
		return
	}
	run.cancelRequested = true
	if errorKind != "" && run.errorKind == "" {
		run.errorKind = errorKind
	}
	if message != "" && run.message == "" {
		run.message = message
	}
	cancel := run.cancel
	run.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func (o *runtimeOwner) dropAgentRunsForEnvironment(environmentID string) {
	o.mu.Lock()
	var runs []*ownedAgentRun
	for runID, run := range o.runs {
		if run.environmentID == environmentID {
			runs = append(runs, run)
			delete(o.runs, runID)
		}
	}
	o.mu.Unlock()
	for _, run := range runs {
		o.requestAgentRunCancel(run, "", "")
		_ = waitOwnedAgentRun(run, agentRunStopTimeout)
	}
}

func (o *runtimeOwner) closeAgentRuns(runs []*ownedAgentRun) error {
	var errs []string
	for _, run := range runs {
		o.requestAgentRunCancel(run, "", "")
	}
	for _, run := range runs {
		if err := waitOwnedAgentRun(run, agentRunStopTimeout); err != nil {
			errs = append(errs, err.Error())
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("agent run cleanup failed: %s", strings.Join(errs, "; "))
	}
	return nil
}

func waitOwnedAgentRun(run *ownedAgentRun, timeout time.Duration) error {
	select {
	case <-run.done:
		return nil
	case <-time.After(timeout):
		return fmt.Errorf("run %q did not stop within %s", run.id, timeout)
	}
}

func (o *runtimeOwner) runningAgentRunCountLocked() int {
	count := 0
	for _, run := range o.runs {
		run.mu.Lock()
		running := run.state == agentRunRunning
		run.mu.Unlock()
		if running {
			count++
		}
	}
	return count
}
