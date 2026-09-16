package gateway

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"ai-dev-manager-v2/internal/app"
	"ai-dev-manager-v2/internal/catalog"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	runtimeOwnerReconcileTimeout = 10 * time.Second
	runtimeOwnerMonitorTick      = 100 * time.Millisecond
	runtimeOwnerInventoryLimit   = 256
)

var runtimeOwnerSequence atomic.Uint64

type ownedMCPSession interface {
	Ping(context.Context, *mcp.PingParams) error
	ListTools(context.Context, *mcp.ListToolsParams) (*mcp.ListToolsResult, error)
	CallTool(context.Context, *mcp.CallToolParams) (*mcp.CallToolResult, error)
	Close() error
}

type ownedMCPConnectFunc func(context.Context, string, string, map[string]string) (ownedMCPSession, error)

type runtimeOwnerKey struct {
	environmentID string
	mcpID         string
}

type runtimeOwnerInfo struct {
	ID                string    `json:"id"`
	PID               int       `json:"pid"`
	StartedAt         time.Time `json:"started_at"`
	OwnedMCPSessions  int       `json:"owned_mcp_sessions"`
	OwnedDevProcesses int       `json:"owned_dev_processes"`
	OwnedAgentRuns    int       `json:"owned_agent_runs"`
}

type runtimeOwner struct {
	service   *app.Service
	id        string
	pid       int
	startedAt time.Time
	connect   ownedMCPConnectFunc
	ctx       context.Context
	cancel    context.CancelFunc

	mu                        sync.Mutex
	closed                    bool
	sessions                  map[runtimeOwnerKey]ownedMCPSession
	observations              map[runtimeOwnerKey]app.MCPRuntimeObservation
	inFlight                  map[runtimeOwnerKey]bool
	generations               map[runtimeOwnerKey]uint64
	desiredFingerprints       map[runtimeOwnerKey]string
	processes                 map[string]*ownedDevProcess
	runs                      map[string]*ownedAgentRun
	verifierRuns              map[string]*ownedVerifierRun
	verifierHeartbeatInterval func(time.Duration) time.Duration
}

func newRuntimeOwner(service *app.Service) *runtimeOwner {
	startedAt := time.Now().UTC()
	ownerCtx, cancel := context.WithCancel(context.Background())
	owner := &runtimeOwner{
		service:             service,
		id:                  fmt.Sprintf("owner_%d_%x_%x", os.Getpid(), startedAt.UnixNano(), runtimeOwnerSequence.Add(1)),
		pid:                 os.Getpid(),
		startedAt:           startedAt,
		ctx:                 ownerCtx,
		cancel:              cancel,
		sessions:            map[runtimeOwnerKey]ownedMCPSession{},
		observations:        map[runtimeOwnerKey]app.MCPRuntimeObservation{},
		inFlight:            map[runtimeOwnerKey]bool{},
		generations:         map[runtimeOwnerKey]uint64{},
		desiredFingerprints: map[runtimeOwnerKey]string{},
		processes:           map[string]*ownedDevProcess{},
		runs:                map[string]*ownedAgentRun{},
		verifierRuns:        map[string]*ownedVerifierRun{},
	}
	owner.connect = func(ctx context.Context, mcpID, endpoint string, headers map[string]string) (ownedMCPSession, error) {
		return connectExternalMCP(ctx, mcpID, endpoint, headers)
	}
	go owner.monitor()
	return owner
}

func (o *runtimeOwner) Info() runtimeOwnerInfo {
	o.mu.Lock()
	defer o.mu.Unlock()
	return runtimeOwnerInfo{
		ID:                o.id,
		PID:               o.pid,
		StartedAt:         o.startedAt,
		OwnedMCPSessions:  len(o.sessions),
		OwnedDevProcesses: o.runningDevProcessCountLocked(),
		OwnedAgentRuns:    o.runningAgentRunCountLocked(),
	}
}

func (o *runtimeOwner) Reconcile(ctx context.Context) {
	if o == nil || o.service == nil {
		return
	}
	environments, err := o.service.Environments.List()
	if err != nil {
		return
	}
	desired := make(map[runtimeOwnerKey]struct{})
	for _, env := range environments {
		for _, mcpID := range env.EnabledMCPIDs {
			key := runtimeOwnerKey{environmentID: env.ID, mcpID: mcpID}
			desired[key] = struct{}{}
			probeCtx, cancel := context.WithTimeout(ctx, runtimeOwnerReconcileTimeout)
			_, _ = o.Status(probeCtx, env.ID, mcpID)
			cancel()
		}
	}
	o.pruneToDesired(desired)
}

func (o *runtimeOwner) Status(ctx context.Context, environmentID, mcpID string) (app.MCPHealthStatus, error) {
	_, status, err := o.ensureHealthySession(ctx, environmentID, mcpID)
	return status, err
}

func (o *runtimeOwner) ListTools(ctx context.Context, environmentID, mcpID string) ([]*mcp.Tool, error) {
	session, status, err := o.ensureHealthySession(ctx, environmentID, mcpID)
	if err != nil {
		return nil, err
	}
	if status.State != app.MCPHealthHealthy || session == nil {
		return nil, statusAsMCPError(status)
	}
	key := runtimeOwnerKey{environmentID: environmentID, mcpID: mcpID}
	tools, err := listOwnedMCPTools(ctx, session)
	if err != nil {
		kind := toolOperationErrorKind(err, "tool_list_failed")
		o.failSession(key, app.MCPFailureListTools, kind, "external MCP tool listing failed")
		return nil, &app.MCPError{MCPID: mcpID, ErrorKind: kind, Message: "external MCP tool listing failed"}
	}
	o.recordSuccess(key, tools, true)
	return tools, nil
}

func (o *runtimeOwner) CallTool(ctx context.Context, environmentID, mcpID, tool string, arguments map[string]any) (*mcp.CallToolResult, error) {
	session, status, err := o.ensureHealthySession(ctx, environmentID, mcpID)
	if err != nil {
		return nil, err
	}
	if status.State != app.MCPHealthHealthy || session == nil {
		return nil, statusAsMCPError(status)
	}
	result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: tool, Arguments: arguments})
	if err != nil {
		kind := toolOperationErrorKind(err, "tool_call_failed")
		o.failSession(runtimeOwnerKey{environmentID: environmentID, mcpID: mcpID}, app.MCPFailureCall, kind, "external MCP tool call failed")
		return nil, &app.MCPError{MCPID: mcpID, ErrorKind: kind, Message: "external MCP tool call failed"}
	}
	o.recordSuccess(runtimeOwnerKey{environmentID: environmentID, mcpID: mcpID}, nil, false)
	return result, nil
}

func (o *runtimeOwner) DropEnvironment(environmentID string) {
	if o == nil {
		return
	}
	o.dropDevProcessesForEnvironment(environmentID)
	o.dropAgentRunsForEnvironment(environmentID)
	o.dropVerifierRunsForEnvironment(environmentID)
	o.dropMatching(func(key runtimeOwnerKey) bool { return key.environmentID == environmentID })
}

func (o *runtimeOwner) DropMCP(mcpID string) {
	if o == nil {
		return
	}
	o.dropMatching(func(key runtimeOwnerKey) bool { return key.mcpID == mcpID })
}

func (o *runtimeOwner) DropAllMCP() {
	if o == nil {
		return
	}
	o.dropMatching(func(runtimeOwnerKey) bool { return true })
}

func (o *runtimeOwner) Drop(environmentID, mcpID string) {
	if o == nil {
		return
	}
	o.drop(runtimeOwnerKey{environmentID: environmentID, mcpID: mcpID})
}

func (o *runtimeOwner) Close() error {
	if o == nil {
		return nil
	}
	o.mu.Lock()
	if o.closed {
		o.mu.Unlock()
		return nil
	}
	o.closed = true
	ownerCancel := o.cancel
	o.cancel = nil
	sessions := make([]ownedMCPSession, 0, len(o.sessions))
	for _, session := range o.sessions {
		sessions = append(sessions, session)
	}
	processes := make([]*ownedDevProcess, 0, len(o.processes))
	for _, process := range o.processes {
		processes = append(processes, process)
	}
	runs := make([]*ownedAgentRun, 0, len(o.runs))
	for _, run := range o.runs {
		runs = append(runs, run)
	}
	verifierRuns := make([]*ownedVerifierRun, 0, len(o.verifierRuns))
	for _, run := range o.verifierRuns {
		verifierRuns = append(verifierRuns, run)
	}
	o.sessions = map[runtimeOwnerKey]ownedMCPSession{}
	o.observations = map[runtimeOwnerKey]app.MCPRuntimeObservation{}
	o.inFlight = map[runtimeOwnerKey]bool{}
	o.desiredFingerprints = map[runtimeOwnerKey]string{}
	o.processes = map[string]*ownedDevProcess{}
	o.runs = map[string]*ownedAgentRun{}
	o.verifierRuns = map[string]*ownedVerifierRun{}
	o.mu.Unlock()

	var errs []error
	for _, session := range sessions {
		if err := session.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	if ownerCancel != nil {
		ownerCancel()
	}
	if err := o.closeDevProcesses(processes); err != nil {
		errs = append(errs, err)
	}
	if err := o.closeAgentRuns(runs); err != nil {
		errs = append(errs, err)
	}
	if err := o.closeVerifierRuns(verifierRuns); err != nil {
		errs = append(errs, err)
	}
	return errors.Join(errs...)
}

func (o *runtimeOwner) ensureHealthySession(ctx context.Context, environmentID, mcpID string) (ownedMCPSession, app.MCPHealthStatus, error) {
	return o.ensureHealthySessionInternal(ctx, environmentID, mcpID, nil)
}

func (o *runtimeOwner) ensureHealthySessionForGeneration(ctx context.Context, environmentID, mcpID string, expectedGeneration uint64) (ownedMCPSession, app.MCPHealthStatus, error) {
	return o.ensureHealthySessionInternal(ctx, environmentID, mcpID, &expectedGeneration)
}

func (o *runtimeOwner) ensureHealthySessionInternal(ctx context.Context, environmentID, mcpID string, expectedGeneration *uint64) (ownedMCPSession, app.MCPHealthStatus, error) {
	if o == nil || o.service == nil {
		return nil, app.MCPHealthStatus{}, fmt.Errorf("runtime owner is not initialized")
	}
	key := runtimeOwnerKey{environmentID: environmentID, mcpID: mcpID}
	if definition, definitionErr := o.service.MCPs.Get(mcpID); definitionErr == nil {
		o.invalidateChangedDesiredConfig(key, definition)
	}
	if expectedGeneration != nil && o.generation(key) != *expectedGeneration {
		return nil, app.MCPHealthStatus{MCPID: mcpID, State: app.MCPHealthConfigured}, nil
	}
	generation := o.registerKey(key)
	if expectedGeneration != nil && generation != *expectedGeneration {
		return nil, app.MCPHealthStatus{MCPID: mcpID, State: app.MCPHealthConfigured}, nil
	}
	activation, status, err := o.service.ResolveMCPActivation(environmentID, mcpID)
	if err != nil {
		return nil, app.MCPHealthStatus{}, err
	}
	if expectedGeneration != nil && o.generation(key) != generation {
		return nil, app.MCPHealthStatus{MCPID: mcpID, State: app.MCPHealthConfigured}, nil
	}
	if activation == nil {
		o.drop(key)
		o.setObserved(key, status)
		return nil, status, nil
	}
	o.markDesired(key, generation, activation.Transport)

	if session := o.session(key); session != nil {
		if pingErr := probeOwnedMCPSession(ctx, session); pingErr == nil {
			status = app.MCPHealthStatus{MCPID: mcpID, State: app.MCPHealthHealthy}
			o.recordSuccess(key, nil, false)
			return session, status, nil
		} else {
			o.failSession(key, app.MCPFailurePing, pingErrorKind(pingErr), "external MCP ping failed")
			observation, _ := o.observation(key)
			return nil, observationStatus(observation), nil
		}
	}

	if expectedGeneration != nil && o.generation(key) != generation {
		return nil, app.MCPHealthStatus{MCPID: mcpID, State: app.MCPHealthConfigured}, nil
	}

	var session ownedMCPSession
	if activation.Transport == catalog.MCPTransportStdio {
		session, err = connectStdioMCP(ctx, o.ctx, o.service, environmentID, activation)
	} else {
		session, err = o.connect(ctx, mcpID, activation.Endpoint, activation.Headers)
	}
	if err != nil {
		status = ownerMCPErrorStatus(mcpID, runtimeMCPErrorKind(err))
		stage := app.MCPFailureConnect
		if observation, ok := o.observation(key); ok && observation.ConsecutiveFailures > 0 {
			stage = app.MCPFailureReconnect
		}
		o.recordFailureForGeneration(key, generation, stage, status.ErrorKind, "external MCP connection failed")
		return nil, status, nil
	}
	if expectedGeneration != nil && o.generation(key) != generation {
		_ = session.Close()
		return nil, app.MCPHealthStatus{MCPID: mcpID, State: app.MCPHealthConfigured}, nil
	}
	if err := probeOwnedMCPSession(ctx, session); err != nil {
		_ = session.Close()
		kind := pingErrorKind(err)
		status = ownerMCPErrorStatus(mcpID, kind)
		o.recordFailureForGeneration(key, generation, app.MCPFailurePing, kind, "external MCP ping failed")
		return nil, status, nil
	}
	tools, err := listOwnedMCPTools(ctx, session)
	if err != nil {
		_ = session.Close()
		kind := toolOperationErrorKind(err, "tool_list_failed")
		status = ownerMCPErrorStatus(mcpID, kind)
		o.recordFailureForGeneration(key, generation, app.MCPFailureDiscover, kind, "external MCP tool discovery failed")
		return nil, status, nil
	}
	current := o.installSession(key, session, generation)
	if current == nil {
		_ = session.Close()
		status = ownerMCPErrorStatus(mcpID, "owner_closed")
		return nil, status, nil
	}
	if current != session {
		_ = session.Close()
		session = current
	}
	status = app.MCPHealthStatus{MCPID: mcpID, State: app.MCPHealthHealthy}
	o.recordSuccess(key, tools, true)
	return session, status, nil
}

func (o *runtimeOwner) session(key runtimeOwnerKey) ownedMCPSession {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.closed {
		return nil
	}
	return o.sessions[key]
}

func (o *runtimeOwner) installSession(key runtimeOwnerKey, session ownedMCPSession, generation uint64) ownedMCPSession {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.closed || o.generations[key] != generation {
		return nil
	}
	if current := o.sessions[key]; current != nil {
		return current
	}
	o.sessions[key] = session
	return session
}

func (o *runtimeOwner) setObserved(key runtimeOwnerKey, status app.MCPHealthStatus) {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.closed {
		return
	}
	o.observations[key] = app.MCPRuntimeObservation{
		EnvironmentID:  key.environmentID,
		MCPID:          key.mcpID,
		DesiredEnabled: status.State != app.MCPHealthDisabled,
		State:          status.State,
		ErrorKind:      status.ErrorKind,
		Message:        status.Message,
	}
	if status.ErrorKind != "" || status.State == app.MCPHealthConfigured {
		observation := o.observations[key]
		observation.FailureStage = app.MCPFailureActivation
		o.observations[key] = observation
	}
}

func (o *runtimeOwner) drop(key runtimeOwnerKey) {
	var session ownedMCPSession
	o.mu.Lock()
	if !o.closed {
		session = o.sessions[key]
		delete(o.sessions, key)
		delete(o.observations, key)
		delete(o.inFlight, key)
		delete(o.desiredFingerprints, key)
		o.generations[key]++
	}
	o.mu.Unlock()
	if session != nil {
		_ = session.Close()
	}
}

func (o *runtimeOwner) dropMatching(match func(runtimeOwnerKey) bool) {
	var sessions []ownedMCPSession
	o.mu.Lock()
	if !o.closed {
		keys := make(map[runtimeOwnerKey]struct{})
		for key := range o.sessions {
			if match(key) {
				keys[key] = struct{}{}
			}
		}
		for key := range o.observations {
			if match(key) {
				keys[key] = struct{}{}
			}
		}
		for key := range o.desiredFingerprints {
			if match(key) {
				keys[key] = struct{}{}
			}
		}
		for key := range keys {
			session := o.sessions[key]
			if !match(key) {
				continue
			}
			if session != nil {
				sessions = append(sessions, session)
			}
			delete(o.sessions, key)
			delete(o.observations, key)
			delete(o.inFlight, key)
			delete(o.desiredFingerprints, key)
			o.generations[key]++
		}
	}
	o.mu.Unlock()
	for _, session := range sessions {
		_ = session.Close()
	}
}

func (o *runtimeOwner) pruneToDesired(desired map[runtimeOwnerKey]struct{}) {
	o.dropMatching(func(key runtimeOwnerKey) bool {
		_, ok := desired[key]
		return !ok
	})
}

func (o *runtimeOwner) observedStatuses() []app.MCPHealthStatus {
	o.mu.Lock()
	defer o.mu.Unlock()
	statuses := make([]app.MCPHealthStatus, 0, len(o.observations))
	for _, observation := range o.observations {
		statuses = append(statuses, observationStatus(observation))
	}
	sort.Slice(statuses, func(i, j int) bool { return statuses[i].MCPID < statuses[j].MCPID })
	return statuses
}

func ownerMCPErrorStatus(mcpID, kind string) app.MCPHealthStatus {
	if kind == "" {
		kind = "connection_failed"
	}
	return app.MCPHealthStatus{
		MCPID:     mcpID,
		State:     app.MCPHealthError,
		ErrorKind: kind,
		Message:   fmt.Sprintf("mcp health probe failed: %s", kind),
	}
}

func runtimeMCPErrorKind(err error) string {
	if err == nil {
		return ""
	}
	var mcpErr *app.MCPError
	if errors.As(err, &mcpErr) && mcpErr.ErrorKind != "" {
		return mcpErr.ErrorKind
	}
	return app.ClassifyMCPError(err)
}

func statusAsMCPError(status app.MCPHealthStatus) error {
	kind := status.ErrorKind
	if kind == "" {
		switch status.State {
		case app.MCPHealthDisabled:
			kind = "not_enabled"
		case app.MCPHealthConfigured:
			kind = "configured"
		default:
			kind = string(status.State)
		}
	}
	message := status.Message
	if message == "" {
		message = fmt.Sprintf("external MCP is %s", status.State)
	}
	return &app.MCPError{MCPID: status.MCPID, ErrorKind: kind, Message: message}
}
