package desktop

import (
	"fmt"
	"strings"

	"ai-dev-manager-v2/internal/adminmcp"
	"ai-dev-manager-v2/internal/model"
	"ai-dev-manager-v2/internal/verifier"
)

func (a *Adapter) AcquireRuntimeWriter(environmentID, owner string) (model.Environment, error) {
	if err := a.readyRuntime(); err != nil {
		return model.Environment{}, err
	}
	owner = strings.TrimSpace(owner)
	if owner == "" {
		return model.Environment{}, fmt.Errorf("writer owner is required")
	}
	return a.runtime.WriterAcquire(environmentID, owner)
}

func (a *Adapter) ReleaseRuntimeWriter(environmentID, owner string) (model.Environment, error) {
	if err := a.readyRuntime(); err != nil {
		return model.Environment{}, err
	}
	owner = strings.TrimSpace(owner)
	if owner == "" {
		return model.Environment{}, fmt.Errorf("writer owner is required")
	}
	return a.runtime.WriterRelease(environmentID, owner, false)
}

func (a *Adapter) ListVerifiers(environmentID string) ([]model.VerifierDefinition, error) {
	if err := a.readyRuntime(); err != nil {
		return nil, err
	}
	return a.runtime.VerifierList(environmentID)
}

func (a *Adapter) RunVerifier(environmentID, writerOwner, verifierID string) (verifier.Result, error) {
	if err := a.readyRuntime(); err != nil {
		return verifier.Result{}, err
	}
	return a.runtime.VerifierRun(environmentID, writerOwner, verifierID, 64*1024)
}

func (a *Adapter) ListProcesses(environmentID string) ([]adminmcp.ProcessStatus, error) {
	if err := a.readyRuntime(); err != nil {
		return nil, err
	}
	return a.runtime.ProcessList(environmentID)
}

func (a *Adapter) GetProcessLogs(environmentID, processID string) (adminmcp.ProcessLogs, error) {
	if err := a.readyRuntime(); err != nil {
		return adminmcp.ProcessLogs{}, err
	}
	return a.runtime.ProcessLogs(environmentID, processID)
}

func (a *Adapter) StopProcess(environmentID, writerOwner, processID string) (adminmcp.ProcessStatus, error) {
	if err := a.readyRuntime(); err != nil {
		return adminmcp.ProcessStatus{}, err
	}
	return a.runtime.ProcessStop(environmentID, writerOwner, processID)
}

func (a *Adapter) ListRuns(environmentID string) ([]adminmcp.RunStatus, error) {
	if err := a.readyRuntime(); err != nil {
		return nil, err
	}
	return a.runtime.RunList(environmentID)
}

func (a *Adapter) CancelRun(environmentID, writerOwner, runID string) (adminmcp.RunStatus, error) {
	if err := a.readyRuntime(); err != nil {
		return adminmcp.RunStatus{}, err
	}
	return a.runtime.RunCancel(environmentID, writerOwner, runID)
}
