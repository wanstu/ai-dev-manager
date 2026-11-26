package gateway

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"sort"
	"sync"
	"time"

	"ai-dev-manager-v2/internal/identity"
	runtimepkg "ai-dev-manager-v2/internal/runtime"
)

const (
	defaultDevProcessLogBytes = 64 * 1024
	maxDevProcessLogBytes     = 1024 * 1024
	devProcessStopTimeout     = 7 * time.Second
)

type devProcessState string

const (
	devProcessRunning devProcessState = "running"
	devProcessExited  devProcessState = "exited"
)

type devProcessStatus struct {
	Resources      *runtimepkg.ProcessResources `json:"resources,omitempty"`
	ResourceError  string                       `json:"resource_error,omitempty"`
	UptimeSeconds  int64                        `json:"uptime_seconds"`
	ExitReason     string                       `json:"exit_reason,omitempty"`
	Executable     string                       `json:"executable"`
	Cwd            string                       `json:"cwd,omitempty"`
	GatewayPID     int                          `json:"gateway_pid"`
	ID             string                       `json:"id"`
	EnvironmentID  string                       `json:"environment_id"`
	State          devProcessState              `json:"state"`
	PID            int                          `json:"pid,omitempty"`
	StartedAt      time.Time                    `json:"started_at"`
	ExitedAt       *time.Time                   `json:"exited_at,omitempty"`
	ExitCode       *int                         `json:"exit_code,omitempty"`
	ListeningPorts []int                        `json:"listening_ports,omitempty"`
	ErrorKind      string                       `json:"error_kind,omitempty"`
}

type devProcessLogs struct {
	ProcessID       string `json:"process_id"`
	Stdout          string `json:"stdout,omitempty"`
	Stderr          string `json:"stderr,omitempty"`
	StdoutTruncated bool   `json:"stdout_truncated,omitempty"`
	StderrTruncated bool   `json:"stderr_truncated,omitempty"`
}

type ownedDevProcess struct {
	managed           runtimepkg.ManagedProcess
	sampleMu          sync.Mutex
	sample            *runtimepkg.ProcessResources
	sampleError       string
	lastSampleAttempt time.Time
	stopRequested     bool
	exitReason        string
	id                string
	environmentID     string
	executable        string
	cwd               string
	cmd               *exec.Cmd
	cancel            context.CancelFunc
	done              chan struct{}
	stdout            *tailLogBuffer
	stderr            *tailLogBuffer

	mu        sync.Mutex
	state     devProcessState
	startedAt time.Time
	exitedAt  *time.Time
	exitCode  *int
	errorKind string
}

// StartDevProcess treats a development service as a Gateway-owned runtime resource, not as a writer lease.
// Agent requests must have launch/stop authority. Administrator requests use
// dedicated methods and never acquire or renew the project's writer lease.
func (o *runtimeOwner) StartDevProcess(environmentID, writerOwner, executable string, args []string, cwd string, maxLogBytes int) (devProcessStatus, error) {
	if o == nil || o.service == nil {
		return devProcessStatus{}, fmt.Errorf("runtime owner is not initialized")
	}
	if _, err := o.service.Environments.RequireWriter(environmentID, writerOwner); err != nil {
		return devProcessStatus{}, err
	}
	return o.StartDevProcessAdmin(environmentID, executable, args, cwd, maxLogBytes)
}

func (o *runtimeOwner) StartDevProcessAdmin(environmentID, executable string, args []string, cwd string, maxLogBytes int) (devProcessStatus, error) {
	if o == nil || o.service == nil {
		return devProcessStatus{}, fmt.Errorf("runtime owner is not initialized")
	}
	rt, _, err := o.service.Runtime(environmentID)
	if err != nil {
		return devProcessStatus{}, err
	}
	if maxLogBytes <= 0 {
		maxLogBytes = defaultDevProcessLogBytes
	}
	if maxLogBytes > maxDevProcessLogBytes {
		return devProcessStatus{}, fmt.Errorf("max_log_bytes exceeds %d", maxDevProcessLogBytes)
	}
	processID, err := identity.New("proc")
	if err != nil {
		return devProcessStatus{}, err
	}
	processCtx, cancel := context.WithCancel(o.processContext())
	o.service.RecordFullAuthorizationBypass(rt, environmentID, executable, "process_start")
	cmd, err := rt.PrepareCommand(processCtx, executable, args, cwd)
	if err != nil {
		if appIsExecutableNotAllowedError(o.service, err) {
			o.service.RecordExecDenial(environmentID, executable, "process_start", err.Error())
		}
		cancel()
		return devProcessStatus{}, err
	}
	stdout := newTailLogBuffer(maxLogBytes)
	stderr := newTailLogBuffer(maxLogBytes)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	o.service.Log("info", "process.start", map[string]string{"environment_id": environmentID, "executable": executable, "surface": "process"})
	managed, err := runtimepkg.StartManagedProcess(cmd)
	if err != nil {
		cancel()
		return devProcessStatus{}, err
	}
	_ = o.service.RecordExecUsage(environmentID, executable, "process_start")
	process := &ownedDevProcess{
		id:            processID,
		environmentID: environmentID,
		executable:    executable,
		cwd:           cmd.Dir,
		managed:       managed,
		cmd:           cmd,
		cancel:        cancel,
		done:          make(chan struct{}),
		stdout:        stdout,
		stderr:        stderr,
		state:         devProcessRunning,
		startedAt:     time.Now().UTC(),
	}
	if err := o.installDevProcess(process); err != nil {
		cancel()
		_ = cmd.Wait()
		managed.Close()
		return devProcessStatus{}, err
	}
	go o.waitDevProcess(process)
	return o.devProcessStatus(process), nil
}

func (o *runtimeOwner) ListDevProcesses(environmentID string) ([]devProcessStatus, error) {
	if _, err := o.service.Environments.Get(environmentID); err != nil {
		return nil, err
	}
	return o.listDevProcesses(environmentID), nil
}

func (o *runtimeOwner) listDevProcesses(environmentID string) []devProcessStatus {
	o.mu.Lock()
	items := make([]*ownedDevProcess, 0)
	for _, process := range o.processes {
		if environmentID == "" || process.environmentID == environmentID {
			items = append(items, process)
		}
	}
	o.mu.Unlock()
	statuses := make([]devProcessStatus, 0, len(items))
	for _, process := range items {
		statuses = append(statuses, o.devProcessStatus(process))
	}
	sort.Slice(statuses, func(i, j int) bool { return statuses[i].StartedAt.Before(statuses[j].StartedAt) })
	return statuses
}

func (o *runtimeOwner) DevProcessStatus(environmentID, processID string) (devProcessStatus, error) {
	process, err := o.devProcess(environmentID, processID)
	if err != nil {
		return devProcessStatus{}, err
	}
	return o.devProcessStatus(process), nil
}

func (o *runtimeOwner) DevProcessLogs(environmentID, processID string) (devProcessLogs, error) {
	process, err := o.devProcess(environmentID, processID)
	if err != nil {
		return devProcessLogs{}, err
	}
	stdout, stdoutTruncated := process.stdout.Snapshot()
	stderr, stderrTruncated := process.stderr.Snapshot()
	return devProcessLogs{
		ProcessID:       process.id,
		Stdout:          stdout,
		Stderr:          stderr,
		StdoutTruncated: stdoutTruncated,
		StderrTruncated: stderrTruncated,
	}, nil
}

func (o *runtimeOwner) StopDevProcess(environmentID, writerOwner, processID string) (devProcessStatus, error) {
	if _, err := o.service.Environments.RequireWriter(environmentID, writerOwner); err != nil {
		return devProcessStatus{}, err
	}
	return o.StopDevProcessAdmin(environmentID, processID)
}

func (o *runtimeOwner) StopDevProcessAdmin(environmentID, processID string) (devProcessStatus, error) {
	process, err := o.devProcess(environmentID, processID)
	if err != nil {
		return devProcessStatus{}, err
	}
	process.mu.Lock()
	if process.state == devProcessRunning {
		process.stopRequested = true
	}
	process.mu.Unlock()
	process.cancel()
	if err := waitOwnedProcess(process, devProcessStopTimeout); err != nil {
		return devProcessStatus{}, err
	}
	return o.devProcessStatus(process), nil
}

func (o *runtimeOwner) installDevProcess(process *ownedDevProcess) error {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.closed {
		return fmt.Errorf("runtime owner is closed")
	}
	if o.processes == nil {
		o.processes = map[string]*ownedDevProcess{}
	}
	o.processes[process.id] = process
	return nil
}

func (o *runtimeOwner) devProcess(environmentID, processID string) (*ownedDevProcess, error) {
	o.mu.Lock()
	process := o.processes[processID]
	o.mu.Unlock()
	if process == nil || process.environmentID != environmentID {
		return nil, fmt.Errorf("process %q not found for environment %q", processID, environmentID)
	}
	return process, nil
}

func (o *runtimeOwner) devProcessStatus(process *ownedDevProcess) devProcessStatus {
	process.mu.Lock()
	status := devProcessStatus{
		Executable:    process.executable,
		Cwd:           process.cwd,
		GatewayPID:    o.pid,
		ID:            process.id,
		EnvironmentID: process.environmentID,
		State:         process.state,
		StartedAt:     process.startedAt,
		ExitedAt:      process.exitedAt,
		ExitCode:      process.exitCode,
		ErrorKind:     process.errorKind,
		ExitReason:    process.exitReason,
	}
	if process.cmd != nil && process.cmd.Process != nil {
		status.PID = process.cmd.Process.Pid
	}
	end := time.Now()
	if status.ExitedAt != nil {
		end = *status.ExitedAt
	}
	status.UptimeSeconds = max(0, int64(end.Sub(status.StartedAt).Seconds()))
	process.mu.Unlock()
	if status.State == devProcessRunning && status.PID > 0 {
		status.Resources, status.ResourceError = process.sampleResources()
		ports, err := runtimepkg.ListeningTCPPorts(status.PID)
		if err == nil {
			status.ListeningPorts = ports
		}
	}
	return status
}

func (o *runtimeOwner) waitDevProcess(process *ownedDevProcess) {
	err := process.cmd.Wait()
	process.managed.Close()
	now := time.Now().UTC()
	exitCode := 0
	kind := ""
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = -1
			if process.cmd.ProcessState != nil {
				exitCode = process.cmd.ProcessState.ExitCode()
			}
			kind = "process_wait_failed"
			if errors.Is(err, exec.ErrWaitDelay) {
				kind = "process_output_wait_timeout"
			}
		}
	}
	process.mu.Lock()
	process.state = devProcessExited
	if process.stopRequested {
		process.exitReason = "stopped"
	} else if err != nil {
		process.exitReason = "failed"
	} else {
		process.exitReason = "completed"
	}
	process.exitedAt = &now
	process.exitCode = &exitCode
	if process.errorKind == "" {
		process.errorKind = kind
	}
	process.mu.Unlock()
	o.service.Log("info", "process.completed", map[string]string{"environment_id": process.environmentID, "executable": process.executable, "surface": "process", "exit_code": fmt.Sprint(exitCode), "error_kind": kind})
	process.cancel()
	close(process.done)
}

func (o *runtimeOwner) dropDevProcessesForEnvironment(environmentID string) {
	o.mu.Lock()
	var processes []*ownedDevProcess
	for processID, process := range o.processes {
		if process.environmentID == environmentID {
			processes = append(processes, process)
			delete(o.processes, processID)
		}
	}
	o.mu.Unlock()
	for _, process := range processes {
		process.cancel()
		_ = waitOwnedProcess(process, devProcessStopTimeout)
	}
}

func (o *runtimeOwner) closeDevProcesses(processes []*ownedDevProcess) error {
	var errs []error
	for _, process := range processes {
		process.cancel()
	}
	for _, process := range processes {
		if err := waitOwnedProcess(process, devProcessStopTimeout); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func waitOwnedProcess(process *ownedDevProcess, timeout time.Duration) error {
	select {
	case <-process.done:
		return nil
	case <-time.After(timeout):
		return fmt.Errorf("process %q did not stop within %s", process.id, timeout)
	}
}

func (o *runtimeOwner) runningDevProcessCountLocked() int {
	count := 0
	for _, process := range o.processes {
		process.mu.Lock()
		running := process.state == devProcessRunning
		process.mu.Unlock()
		if running {
			count++
		}
	}
	return count
}

func (o *runtimeOwner) processContext() context.Context {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.ctx == nil {
		return context.Background()
	}
	return o.ctx
}

type tailLogBuffer struct {
	mu        sync.Mutex
	limit     int
	data      []byte
	truncated bool
}

func newTailLogBuffer(limit int) *tailLogBuffer {
	return &tailLogBuffer{limit: limit}
}

func (b *tailLogBuffer) Write(p []byte) (int, error) {
	original := len(p)
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.limit <= 0 {
		b.truncated = b.truncated || len(p) > 0
		return original, nil
	}
	if len(p) >= b.limit {
		b.truncated = b.truncated || len(b.data) > 0 || len(p) > b.limit
		b.data = append(b.data[:0], p[len(p)-b.limit:]...)
		return original, nil
	}
	if overflow := len(b.data) + len(p) - b.limit; overflow > 0 {
		copy(b.data, b.data[overflow:])
		b.data = b.data[:len(b.data)-overflow]
		b.truncated = true
	}
	b.data = append(b.data, p...)
	return original, nil
}

func (b *tailLogBuffer) Snapshot() (string, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return string(append([]byte(nil), b.data...)), b.truncated
}

var _ io.Writer = (*tailLogBuffer)(nil)

func (p *ownedDevProcess) sampleResources() (*runtimepkg.ProcessResources, string) {
	p.sampleMu.Lock()
	defer p.sampleMu.Unlock()
	if time.Since(p.lastSampleAttempt) < time.Second {
		return p.sample, p.sampleError
	}
	p.lastSampleAttempt = time.Now()
	if p.managed == nil {
		return nil, "resource observation unavailable"
	}
	next, err := p.managed.Sample()
	if err != nil {
		p.sample = nil
		p.sampleError = err.Error()
		return nil, p.sampleError
	}
	// CPU is percentage of one logical core, not the entire host. Windows
	// job CPU is cumulative; Linux live-member totals can reset on child exit.
	if p.sample != nil && p.sample.Complete && next.Complete && next.CPUTimeMS >= p.sample.CPUTimeMS && p.sample.Scope == next.Scope && next.CPUBasis == p.sample.CPUBasis {
		elapsed := next.SampledAt.Sub(p.sample.SampledAt).Seconds()
		if elapsed > 0 {
			cpu := float64(next.CPUTimeMS-p.sample.CPUTimeMS) / 1000 / elapsed * 100
			next.CPUPercent = &cpu
		}
	}
	p.sample = &next
	p.sampleError = ""
	return p.sample, ""
}
