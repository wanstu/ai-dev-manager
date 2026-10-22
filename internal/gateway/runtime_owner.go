package gateway

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"ai-dev-manager-v2/internal/app"
	"ai-dev-manager-v2/internal/catalog"
	"ai-dev-manager-v2/internal/environment"
	"ai-dev-manager-v2/internal/model"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	mcpcenter "github.com/wanstu/mcp-center-core"
)

const (
	runtimeOwnerReconcileTimeout = 10 * time.Second
	runtimeOwnerMonitorTick      = 100 * time.Millisecond
	runtimeOwnerInventoryLimit   = 256
)

var (
	runtimeOwnerSequence             atomic.Uint64
	runtimeWriterObservationSequence atomic.Uint64
)

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

type writerHeartbeatSource struct {
	Kind           string    `json:"kind"`
	ID             string    `json:"id"`
	State          string    `json:"state"`
	PID            int       `json:"pid,omitempty"`
	Executable     string    `json:"executable,omitempty"`
	Cwd            string    `json:"cwd,omitempty"`
	ExecutionMode  string    `json:"execution_mode,omitempty"`
	VerifierID     string    `json:"verifier_id,omitempty"`
	VerifierName   string    `json:"verifier_name,omitempty"`
	GatewayOwnerID string    `json:"gateway_owner_id,omitempty"`
	GatewayPID     int       `json:"gateway_pid,omitempty"`
	StartedAt      time.Time `json:"started_at,omitempty"`
	ObservedAt     time.Time `json:"observed_at,omitempty"`
}

type writerDiagnostics struct {
	EnvironmentID            string                  `json:"environment_id"`
	Active                   bool                    `json:"active"`
	OwnerFingerprint         string                  `json:"owner_fingerprint,omitempty"`
	AcquiredAt               *time.Time              `json:"acquired_at,omitempty"`
	LastSeenAt               *time.Time              `json:"last_seen_at,omitempty"`
	ExpiresAt                *time.Time              `json:"expires_at,omitempty"`
	HeartbeatIntervalSeconds int64                   `json:"heartbeat_interval_seconds,omitempty"`
	Attribution              string                  `json:"attribution"`
	GatewayOwner             runtimeOwnerInfo        `json:"gateway_owner"`
	HeartbeatSources         []writerHeartbeatSource `json:"heartbeat_sources"`
	Note                     string                  `json:"note,omitempty"`
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
	activeMCPOperations       map[runtimeOwnerKey]int
	observations              map[runtimeOwnerKey]app.MCPRuntimeObservation
	inFlight                  map[runtimeOwnerKey]bool
	generations               map[runtimeOwnerKey]uint64
	desiredFingerprints       map[runtimeOwnerKey]string
	processes                 map[string]*ownedDevProcess
	runs                      map[string]*ownedAgentRun
	verifierRuns              map[string]*ownedVerifierRun
	indexWatchers             map[string]context.CancelFunc
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
		activeMCPOperations: map[runtimeOwnerKey]int{},
		observations:        map[runtimeOwnerKey]app.MCPRuntimeObservation{},
		inFlight:            map[runtimeOwnerKey]bool{},
		generations:         map[runtimeOwnerKey]uint64{},
		desiredFingerprints: map[runtimeOwnerKey]string{},
		processes:           map[string]*ownedDevProcess{},
		runs:                map[string]*ownedAgentRun{},
		verifierRuns:        map[string]*ownedVerifierRun{},
		indexWatchers:       map[string]context.CancelFunc{},
	}
	owner.connect = func(ctx context.Context, mcpID, endpoint string, headers map[string]string) (ownedMCPSession, error) {
		session, err := mcpcenter.OpenSession(ctx, mcpcenter.Activation{MCPID: mcpID, Transport: mcpcenter.TransportStreamableHTTP, Endpoint: endpoint, Headers: headers})
		if err != nil {
			return nil, &app.MCPError{MCPID: mcpID, ErrorKind: app.ClassifyMCPError(err), Message: "external MCP connection failed"}
		}
		return session, nil
	}
	go owner.monitor()
	go owner.restoreAutoIndexWatches()
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

func (o *runtimeOwner) writerObservation(sourceKind, sourceID string, sourcePID int, executable, cwd, executionMode, verifierID, verifierName string) environment.WriterObservation {
	if o == nil {
		return environment.WriterObservation{}
	}
	return environment.WriterObservation{
		GatewayOwnerID: o.id,
		GatewayPID:     o.pid,
		SourceKind:     sourceKind,
		SourceID:       sourceID,
		SourcePID:      sourcePID,
		Executable:     executable,
		Cwd:            cwd,
		ExecutionMode:  executionMode,
		VerifierID:     verifierID,
		VerifierName:   verifierName,
	}
}

func (o *runtimeOwner) observeWriter(environmentID, writerOwner string, observation environment.WriterObservation) error {
	if o == nil || o.service == nil {
		return fmt.Errorf("runtime owner is not initialized")
	}
	_, err := o.service.Environments.ObserveWriter(environmentID, writerOwner, observation)
	return err
}

func (o *runtimeOwner) clearWriterObservation(environmentID string, observation environment.WriterObservation) {
	if o == nil || o.service == nil {
		return
	}
	_ = o.service.Environments.ClearWriterObservation(environmentID, observation)
}

func (o *runtimeOwner) nextWriterObservationID(prefix string) string {
	return fmt.Sprintf("%s_%x", prefix, runtimeWriterObservationSequence.Add(1))
}

func (o *runtimeOwner) beginWriterObservation(environmentID, writerOwner string, observation environment.WriterObservation) (func(), error) {
	if observation.SourceID == "" {
		observation.SourceID = o.nextWriterObservationID(observation.SourceKind)
	}
	if err := o.observeWriter(environmentID, writerOwner, observation); err != nil {
		return nil, err
	}
	stop := make(chan struct{})
	var once sync.Once
	interval := o.service.Environments.WriterLeaseTTL() / 3
	if interval <= 0 {
		interval = time.Second
	}
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				if err := o.observeWriter(environmentID, writerOwner, observation); err != nil {
					o.clearWriterObservation(environmentID, observation)
					return
				}
			}
		}
	}()
	return func() {
		once.Do(func() {
			close(stop)
			o.clearWriterObservation(environmentID, observation)
		})
	}, nil
}

func (o *runtimeOwner) WriterDiagnostics(environmentID string) (writerDiagnostics, error) {
	if o == nil || o.service == nil {
		return writerDiagnostics{}, fmt.Errorf("runtime owner is not initialized")
	}
	environment, err := o.service.Environments.Get(environmentID)
	if err != nil {
		return writerDiagnostics{}, err
	}
	diagnostics := writerDiagnostics{
		EnvironmentID:    environmentID,
		Active:           environment.Writer != nil,
		Attribution:      "none",
		GatewayOwner:     o.Info(),
		HeartbeatSources: []writerHeartbeatSource{},
	}
	if environment.Writer == nil {
		diagnostics.Note = "No active Writer lease."
		return diagnostics, nil
	}

	lease := *environment.Writer
	acquiredAt, lastSeenAt, expiresAt := lease.AcquiredAt, lease.LastSeenAt, lease.ExpiresAt
	diagnostics.AcquiredAt = &acquiredAt
	diagnostics.LastSeenAt = &lastSeenAt
	diagnostics.ExpiresAt = &expiresAt
	sum := sha256.Sum256([]byte(lease.Owner))
	diagnostics.OwnerFingerprint = fmt.Sprintf("%x", sum[:6])
	interval := o.service.Environments.WriterLeaseTTL() / 3
	if interval <= 0 {
		interval = time.Second
	}
	diagnostics.HeartbeatIntervalSeconds = int64(interval / time.Second)

	o.mu.Lock()
	processes := make([]*ownedDevProcess, 0, len(o.processes))
	for _, process := range o.processes {
		if process.environmentID == environmentID {
			processes = append(processes, process)
		}
	}
	runs := make([]*ownedAgentRun, 0, len(o.runs))
	for _, run := range o.runs {
		if run.environmentID == environmentID {
			runs = append(runs, run)
		}
	}
	verifierRuns := make([]*ownedVerifierRun, 0, len(o.verifierRuns))
	for _, run := range o.verifierRuns {
		if run.environmentID == environmentID {
			verifierRuns = append(verifierRuns, run)
		}
	}
	o.mu.Unlock()

	for _, process := range processes {
		process.mu.Lock()
		if process.writerOwner == lease.Owner && process.state == devProcessRunning {
			source := writerHeartbeatSource{
				Kind:           "process",
				ID:             process.id,
				State:          string(process.state),
				Executable:     process.executable,
				Cwd:            process.cwd,
				GatewayOwnerID: o.id,
				GatewayPID:     o.pid,
				StartedAt:      process.startedAt,
				ObservedAt:     lease.LastSeenAt,
			}
			if process.cmd != nil && process.cmd.Process != nil {
				source.PID = process.cmd.Process.Pid
			}
			diagnostics.HeartbeatSources = append(diagnostics.HeartbeatSources, source)
		}
		process.mu.Unlock()
	}
	for _, run := range runs {
		run.mu.Lock()
		if run.writerOwner == lease.Owner && run.state == agentRunRunning {
			diagnostics.HeartbeatSources = append(diagnostics.HeartbeatSources, writerHeartbeatSource{
				Kind:           "run",
				ID:             run.id,
				State:          string(run.state),
				Executable:     run.executable,
				Cwd:            run.cwd,
				PID:            run.pid,
				ExecutionMode:  run.executionMode,
				GatewayOwnerID: o.id,
				GatewayPID:     o.pid,
				StartedAt:      run.startedAt,
				ObservedAt:     lease.LastSeenAt,
			})
		}
		run.mu.Unlock()
	}
	for _, run := range verifierRuns {
		run.mu.Lock()
		if run.writerOwner == lease.Owner && run.state == verifierRunRunning {
			diagnostics.HeartbeatSources = append(diagnostics.HeartbeatSources, writerHeartbeatSource{
				Kind:           "verifier",
				ID:             run.id,
				State:          string(run.state),
				Executable:     run.prepared.Definition.Executable,
				Cwd:            run.prepared.Definition.Cwd,
				ExecutionMode:  run.executionMode,
				VerifierID:     run.prepared.Definition.ID,
				VerifierName:   run.prepared.Definition.Name,
				GatewayOwnerID: o.id,
				GatewayPID:     o.pid,
				StartedAt:      run.startedAt,
				ObservedAt:     lease.LastSeenAt,
			})
		}
		run.mu.Unlock()
	}
	sort.Slice(diagnostics.HeartbeatSources, func(i, j int) bool {
		return diagnostics.HeartbeatSources[i].StartedAt.Before(diagnostics.HeartbeatSources[j].StartedAt)
	})
	if len(diagnostics.HeartbeatSources) > 0 {
		diagnostics.Attribution = "current_gateway_runtime"
		diagnostics.Note = "Active Writer is being renewed by one or more running resources owned by this Gateway."
		return diagnostics, nil
	}
	observations, observationErr := o.service.Environments.WriterObservations(environmentID, &lease)
	if observationErr == nil && len(observations) > 0 {
		observation := observations[0]
		sourceState := "observed"
		diagnostics.Attribution = "lease_observation"
		diagnostics.Note = "The latest Writer renewal was attributed by the holding Gateway to this ephemeral safe source observation. The Writer owner credential is not included."
		if !observation.ObservedAt.Equal(lease.LastSeenAt) {
			sourceState = "last_observed"
			diagnostics.Attribution = "last_known_source"
			diagnostics.Note = "This is the last identified Writer source, but a later generic renewal occurred without source metadata. Treat it as historical context, not proof that the source is still running."
		}
		diagnostics.HeartbeatSources = append(diagnostics.HeartbeatSources, writerHeartbeatSource{
			Kind:           observation.SourceKind,
			ID:             observation.SourceID,
			State:          sourceState,
			PID:            observation.SourcePID,
			Executable:     observation.Executable,
			Cwd:            observation.Cwd,
			ExecutionMode:  observation.ExecutionMode,
			VerifierID:     observation.VerifierID,
			VerifierName:   observation.VerifierName,
			GatewayOwnerID: observation.GatewayOwnerID,
			GatewayPID:     observation.GatewayPID,
			ObservedAt:     observation.ObservedAt,
		})
		return diagnostics, nil
	}
	diagnostics.Attribution = "unattributed"
	diagnostics.Note = "Active Writer has no safe source observation. It may come from a legacy Gateway, a direct non-Gateway caller, or an observation registry that is temporarily unavailable."
	return diagnostics, nil
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
		// Expired temporary Environments must not be reactivated at startup.
		if expiredTemporaryEnvironment(env, time.Now().UTC()) {
			continue
		}
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
	defer o.trackMCPOperation(runtimeOwnerKey{environmentID: environmentID, mcpID: mcpID})()
	_, status, err := o.ensureHealthySession(ctx, environmentID, mcpID)
	return status, err
}

func (o *runtimeOwner) ListTools(ctx context.Context, environmentID, mcpID string) ([]*mcp.Tool, error) {
	defer o.trackMCPOperation(runtimeOwnerKey{environmentID: environmentID, mcpID: mcpID})()
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
	defer o.trackMCPOperation(runtimeOwnerKey{environmentID: environmentID, mcpID: mcpID})()
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
	if o.service != nil {
		if _, err := o.service.Environments.Get(environmentID); err != nil {
			o.stopAutoIndexWatch(environmentID)
			_ = o.service.CleanupProjectIndex(environmentID)
		}
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
	o.activeMCPOperations = map[runtimeOwnerKey]int{}
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
	o.dropMatchingInternal(match, false)
}

func (o *runtimeOwner) dropMatchingInternal(match func(runtimeOwnerKey) bool, protectActive bool) {
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
			if protectActive && (o.inFlight[key] || o.activeMCPOperations[key] > 0) {
				continue
			}
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
	o.dropMatchingInternal(func(key runtimeOwnerKey) bool {
		_, ok := desired[key]
		return !ok
	}, true)
}

func expiredTemporaryEnvironment(environment model.Environment, now time.Time) bool {
	return environment.Retention.Persistence == model.PersistenceTemporary &&
		environment.Retention.ExpiresAt != nil &&
		!now.Before(environment.Retention.ExpiresAt.UTC())
}

// Cached upstream sessions are not user activity. Track actual use separately
// so expired worktrees can be cleaned without interrupting MCP calls.
func (o *runtimeOwner) trackMCPOperation(key runtimeOwnerKey) func() {
	o.mu.Lock()
	o.activeMCPOperations[key]++
	o.mu.Unlock()
	return func() {
		o.mu.Lock()
		if o.activeMCPOperations[key] <= 1 {
			delete(o.activeMCPOperations, key)
		} else {
			o.activeMCPOperations[key]--
		}
		o.mu.Unlock()
	}
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
