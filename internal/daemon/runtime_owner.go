package daemon

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sort"
	"sync"

	"ai-dev-manager/internal/controlplane"
	"ai-dev-manager/internal/host"
	admmcp "ai-dev-manager/internal/mcp"
)

type RuntimeState string

const (
	RuntimeStarting RuntimeState = "starting"
	RuntimeRunning  RuntimeState = "running"
	RuntimeStopping RuntimeState = "stopping"
	RuntimeStopped  RuntimeState = "stopped"
	RuntimeError    RuntimeState = "error"
)

type RuntimeStatus struct {
	WorkspaceID    string                    `json:"workspace_id"`
	RuntimeID      string                    `json:"runtime_id,omitempty"`
	DesiredRunning bool                      `json:"desired_running"`
	Listen         string                    `json:"listen,omitempty"`
	Exposed        bool                      `json:"exposed,omitempty"`
	LocalEndpoint  string                    `json:"local_endpoint,omitempty"`
	DockerEndpoint string                    `json:"docker_endpoint,omitempty"`
	State          RuntimeState              `json:"state"`
	MCPHost        *host.Instance            `json:"mcp_host,omitempty"`
	ConfiguredMCPs []admmcp.ActivationStatus `json:"configured_mcps,omitempty"`
	Error          string                    `json:"error,omitempty"`
}

type RuntimeStartOptions struct {
	Listen  string
	Exposed bool
}

type RuntimeOwner struct {
	mu      sync.Mutex
	service *controlplane.Service
	desired *DesiredStore
	entries map[string]RuntimeStatus
}

func NewRuntimeOwner(service *controlplane.Service) *RuntimeOwner {
	return &RuntimeOwner{
		service: service,
		desired: NewDesiredStore(service.Store().Root()),
		entries: make(map[string]RuntimeStatus),
	}
}

func runtimeHostInstanceID(workspaceID string) string {
	return "runtime:" + workspaceID
}

// Start records the user's desired state before creating observed runtime
// resources. If observed startup fails, desired=true remains persisted so a
// future daemon reconciliation can retry it.
func (o *RuntimeOwner) Start(ctx context.Context, workspaceID string) (RuntimeStatus, error) {
	return o.StartWithOptions(ctx, workspaceID, RuntimeStartOptions{})
}

func (o *RuntimeOwner) StartWithOptions(ctx context.Context, workspaceID string, options RuntimeStartOptions) (RuntimeStatus, error) {
	o.mu.Lock()
	defer o.mu.Unlock()

	snapshot, err := o.service.Inspect(workspaceID, nil)
	if err != nil {
		return RuntimeStatus{}, err
	}
	desired := normalizeDesiredRuntime(DesiredRuntime{
		WorkspaceID: workspaceID,
		Listen:      options.Listen,
		Exposed:     options.Exposed,
	})
	if err := o.desired.Set(desired); err != nil {
		return RuntimeStatus{}, err
	}
	if current, ok := o.entries[workspaceID]; ok && current.State == RuntimeRunning {
		if current.Listen == desired.Listen && current.Exposed == desired.Exposed {
			current.DesiredRunning = true
			o.entries[workspaceID] = cloneRuntimeStatus(current)
			return o.refreshLocked(workspaceID, current)
		}
		if _, err := o.stopObservedLocked(ctx, workspaceID, true); err != nil {
			return RuntimeStatus{}, err
		}
	}
	return o.startObservedLocked(ctx, desired, snapshot)
}

// Reconcile rebuilds observed runtime resources from persisted desired state.
// A broken individual Workspace is recorded as RuntimeError and does not stop
// reconciliation of the remaining Workspaces. Desired-store read errors are
// returned because silently treating them as empty would lose user intent.
func (o *RuntimeOwner) Reconcile(ctx context.Context) ([]RuntimeStatus, error) {
	runtimes, err := o.desired.LoadRuntimes()
	if err != nil {
		return nil, err
	}
	statuses := make([]RuntimeStatus, 0, len(runtimes))
	for _, desired := range runtimes {
		o.mu.Lock()
		snapshot, inspectErr := o.service.Inspect(desired.WorkspaceID, nil)
		if inspectErr != nil {
			status := RuntimeStatus{
				WorkspaceID:    desired.WorkspaceID,
				DesiredRunning: true,
				Listen:         desired.Listen,
				Exposed:        desired.Exposed,
				State:          RuntimeError,
				Error:          safeRuntimeError(inspectErr),
			}
			o.entries[desired.WorkspaceID] = cloneRuntimeStatus(status)
			statuses = append(statuses, cloneRuntimeStatus(status))
			o.mu.Unlock()
			continue
		}
		status, _ := o.startObservedLocked(ctx, desired, snapshot)
		statuses = append(statuses, cloneRuntimeStatus(status))
		o.mu.Unlock()
	}
	return statuses, nil
}

func (o *RuntimeOwner) startObservedLocked(ctx context.Context, desired DesiredRuntime, snapshot controlplane.Snapshot) (RuntimeStatus, error) {
	workspaceID := desired.WorkspaceID
	status := RuntimeStatus{
		WorkspaceID:    workspaceID,
		RuntimeID:      snapshot.Runtime.RuntimeID,
		DesiredRunning: true,
		Listen:         desired.Listen,
		Exposed:        desired.Exposed,
		State:          RuntimeStarting,
	}
	o.entries[workspaceID] = cloneRuntimeStatus(status)

	var (
		instance host.Instance
		err      error
	)
	if desired.Exposed {
		instance, err = o.service.StartMCPExposed(runtimeHostInstanceID(workspaceID), workspaceID, nil, desired.Listen)
	} else {
		instance, err = o.service.StartMCP(runtimeHostInstanceID(workspaceID), workspaceID, nil, desired.Listen)
	}
	if err != nil {
		status.State = RuntimeError
		status.Error = safeRuntimeError(err)
		o.entries[workspaceID] = cloneRuntimeStatus(status)
		return cloneRuntimeStatus(status), err
	}
	status.MCPHost = &instance
	applyRuntimeEndpoints(&status, instance)
	o.entries[workspaceID] = cloneRuntimeStatus(status)

	activated, activateErr := o.service.ActivateConfiguredMCPs(ctx, workspaceID, nil)
	status.ConfiguredMCPs = append([]admmcp.ActivationStatus(nil), activated...)
	if activateErr != nil {
		stopMCPsErr := o.service.StopConfiguredMCPs(workspaceID)
		stopHostErr := o.service.StopMCP(ctx, instance.ID)
		status.State = RuntimeError
		status.MCPHost = nil
		status.LocalEndpoint = ""
		status.DockerEndpoint = ""
		status.Error = safeRuntimeError(activateErr)
		o.entries[workspaceID] = cloneRuntimeStatus(status)
		return cloneRuntimeStatus(status), errors.Join(activateErr, stopMCPsErr, stopHostErr)
	}

	status.State = RuntimeRunning
	status.Error = ""
	o.entries[workspaceID] = cloneRuntimeStatus(status)
	return cloneRuntimeStatus(status), nil
}

func (o *RuntimeOwner) Status(workspaceID string) (RuntimeStatus, error) {
	o.mu.Lock()
	defer o.mu.Unlock()

	current, ok := o.entries[workspaceID]
	if !ok {
		ws, err := o.service.Registry().Get(workspaceID)
		if err != nil {
			return RuntimeStatus{}, err
		}
		runtimeID := ws.RuntimeID
		if runtimeID == "" {
			runtimeID = controlplane.NativeRuntimeID
		}
		return RuntimeStatus{WorkspaceID: workspaceID, RuntimeID: runtimeID, State: RuntimeStopped}, nil
	}
	if current.State != RuntimeRunning {
		return cloneRuntimeStatus(current), nil
	}
	return o.refreshLocked(workspaceID, current)
}

func (o *RuntimeOwner) List() []RuntimeStatus {
	o.mu.Lock()
	defer o.mu.Unlock()
	ids := make([]string, 0, len(o.entries))
	for id := range o.entries {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	result := make([]RuntimeStatus, 0, len(ids))
	for _, id := range ids {
		current := o.entries[id]
		refreshed, err := o.refreshLocked(id, current)
		if err != nil {
			refreshed = cloneRuntimeStatus(current)
		}
		result = append(result, refreshed)
	}
	return result
}

// Stop changes persistent desired state first, then tears down observed state.
// This prevents a crash during stop from resurrecting the Workspace on restart.
func (o *RuntimeOwner) Stop(ctx context.Context, workspaceID string) (RuntimeStatus, error) {
	o.mu.Lock()
	defer o.mu.Unlock()

	if err := o.desired.Remove(workspaceID); err != nil {
		return RuntimeStatus{}, err
	}
	return o.stopObservedLocked(ctx, workspaceID, false)
}

func (o *RuntimeOwner) stopObservedLocked(ctx context.Context, workspaceID string, preserveDesired bool) (RuntimeStatus, error) {
	current, ok := o.entries[workspaceID]
	if !ok {
		ws, err := o.service.Registry().Get(workspaceID)
		if err != nil {
			return RuntimeStatus{}, err
		}
		runtimeID := ws.RuntimeID
		if runtimeID == "" {
			runtimeID = controlplane.NativeRuntimeID
		}
		return RuntimeStatus{
			WorkspaceID:    workspaceID,
			RuntimeID:      runtimeID,
			DesiredRunning: preserveDesired,
			State:          RuntimeStopped,
		}, nil
	}
	current.DesiredRunning = preserveDesired
	current.State = RuntimeStopping
	current.Error = ""
	o.entries[workspaceID] = cloneRuntimeStatus(current)

	stopMCPsErr := o.service.StopConfiguredMCPs(workspaceID)
	stopHostErr := o.service.StopMCP(ctx, runtimeHostInstanceID(workspaceID))
	stopErr := errors.Join(stopMCPsErr, stopHostErr)
	current.ConfiguredMCPs = nil
	current.MCPHost = nil
	current.LocalEndpoint = ""
	current.DockerEndpoint = ""
	if stopErr != nil {
		current.State = RuntimeError
		current.Error = safeRuntimeError(stopErr)
		o.entries[workspaceID] = cloneRuntimeStatus(current)
		return cloneRuntimeStatus(current), stopErr
	}
	current.State = RuntimeStopped
	o.entries[workspaceID] = cloneRuntimeStatus(current)
	return cloneRuntimeStatus(current), nil
}

// StopAll tears down observed resources for daemon shutdown while preserving
// persisted desired state. Only an explicit Stop changes desired state.
func (o *RuntimeOwner) StopAll(ctx context.Context) error {
	o.mu.Lock()
	ids := make([]string, 0, len(o.entries))
	for id, entry := range o.entries {
		if entry.State != RuntimeStopped {
			ids = append(ids, id)
		}
	}
	o.mu.Unlock()
	sort.Strings(ids)
	var errs []error
	for _, id := range ids {
		o.mu.Lock()
		_, err := o.stopObservedLocked(ctx, id, true)
		o.mu.Unlock()
		if err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func (o *RuntimeOwner) refreshLocked(workspaceID string, current RuntimeStatus) (RuntimeStatus, error) {
	if current.State == RuntimeRunning {
		instance, ok := o.service.GetMCP(runtimeHostInstanceID(workspaceID))
		if !ok {
			current.State = RuntimeError
			current.MCPHost = nil
			current.Error = "daemon-owned MCP host is missing"
			o.entries[workspaceID] = cloneRuntimeStatus(current)
			return cloneRuntimeStatus(current), fmt.Errorf("runtime %q host missing", workspaceID)
		}
		current.MCPHost = &instance
		applyRuntimeEndpoints(&current, instance)
	}
	statuses, err := o.service.MCPStatuses(workspaceID, nil)
	if err != nil {
		return cloneRuntimeStatus(current), err
	}
	current.ConfiguredMCPs = append([]admmcp.ActivationStatus(nil), statuses...)
	o.entries[workspaceID] = cloneRuntimeStatus(current)
	return cloneRuntimeStatus(current), nil
}

func applyRuntimeEndpoints(status *RuntimeStatus, instance host.Instance) {
	status.LocalEndpoint = ""
	status.DockerEndpoint = ""
	hostValue, port, err := net.SplitHostPort(instance.Address)
	if err != nil {
		return
	}
	if hostValue == "0.0.0.0" || hostValue == "::" || hostValue == "" {
		status.LocalEndpoint = "http://127.0.0.1:" + port + "/mcp"
		if status.Exposed {
			status.DockerEndpoint = "http://host.docker.internal:" + port + "/mcp"
		}
		return
	}
	status.LocalEndpoint = instance.Endpoint
}

func safeRuntimeError(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func cloneRuntimeStatus(source RuntimeStatus) RuntimeStatus {
	clone := source
	if source.MCPHost != nil {
		value := *source.MCPHost
		clone.MCPHost = &value
	}
	clone.ConfiguredMCPs = make([]admmcp.ActivationStatus, len(source.ConfiguredMCPs))
	for i, status := range source.ConfiguredMCPs {
		clone.ConfiguredMCPs[i] = status
		clone.ConfiguredMCPs[i].ToolNames = append([]string(nil), status.ToolNames...)
	}
	return clone
}
