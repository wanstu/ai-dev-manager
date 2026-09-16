package gateway

import (
	"context"
	"encoding/json"
	"sort"
	"time"

	"ai-dev-manager-v2/internal/app"
	"ai-dev-manager-v2/internal/model"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func (o *runtimeOwner) Inspect(environmentID, mcpID string) (app.MCPRuntimeInspection, error) {
	key := runtimeOwnerKey{environmentID: environmentID, mcpID: mcpID}
	environment, err := o.service.Environments.Get(environmentID)
	if err != nil {
		return app.MCPRuntimeInspection{}, err
	}
	desired := containsValue(environment.EnabledMCPIDs, mcpID)
	observation, ok := o.observation(key)
	if !ok {
		state := app.MCPHealthDisabled
		if desired {
			state = app.MCPHealthConfigured
		}
		observation = app.MCPRuntimeObservation{EnvironmentID: environmentID, MCPID: mcpID, DesiredEnabled: desired, State: state}
	}
	definition, err := o.service.MCPs.Get(mcpID)
	if err != nil {
		if desired {
			observation.State = app.MCPHealthConfigured
			observation.FailureStage = app.MCPFailureActivation
			observation.ErrorKind = "unresolved_definition"
			observation.Message = "mcp catalog entry is unresolved"
		}
		return app.MCPRuntimeInspection{Observation: observation}, nil
	}
	desiredConfig := sanitizedDesiredConfig(definition)
	return app.MCPRuntimeInspection{Definition: &desiredConfig, Observation: observation}, nil
}

func (o *runtimeOwner) Refresh(ctx context.Context, environmentID, mcpID string) (app.MCPRuntimeObservation, error) {
	key := runtimeOwnerKey{environmentID: environmentID, mcpID: mcpID}
	o.drop(key)
	generation, ok := o.beginWork(key)
	if !ok {
		observation, _ := o.observation(key)
		return observation, nil
	}
	_, _, err := o.ensureHealthySession(ctx, environmentID, mcpID)
	o.endWork(key, generation)
	if err != nil {
		return app.MCPRuntimeObservation{}, err
	}
	observation, _ := o.observation(key)
	return observation, nil
}

func (o *runtimeOwner) monitor() {
	ticker := time.NewTicker(runtimeOwnerMonitorTick)
	defer ticker.Stop()
	for {
		select {
		case <-o.ctx.Done():
			return
		case now := <-ticker.C:
			o.monitorOnce(now.UTC())
		}
	}
}

func (o *runtimeOwner) monitorOnce(now time.Time) {
	environments, err := o.service.Environments.List()
	if err != nil {
		return
	}
	desired := make(map[runtimeOwnerKey]struct{})
	for _, environment := range environments {
		for _, mcpID := range environment.EnabledMCPIDs {
			key := runtimeOwnerKey{environmentID: environment.ID, mcpID: mcpID}
			desired[key] = struct{}{}
			definition, err := o.service.MCPs.Get(mcpID)
			if err != nil {
				o.drop(key)
				continue
			}
			o.invalidateChangedDesiredConfig(key, definition)
			observation, observed := o.observation(key)
			if o.session(key) != nil {
				if definition.HealthPolicy.HealthCheckEnabled && checkDue(observation.LastCheckAt, definition.HealthPolicy.CheckIntervalSeconds, now) {
					go o.backgroundPing(key, definition.HealthPolicy)
				}
				continue
			}
			if definition.HealthPolicy.AutoReconnect && (!observed || observation.NextReconnectAt == nil || !observation.NextReconnectAt.After(now)) {
				go o.backgroundReconnect(key, definition.HealthPolicy)
			}
		}
	}
	o.pruneToDesired(desired)
}

func (o *runtimeOwner) backgroundPing(key runtimeOwnerKey, policy model.MCPHealthPolicy) {
	generation, ok := o.beginWork(key)
	if !ok {
		return
	}
	defer o.endWork(key, generation)
	session := o.session(key)
	if session == nil {
		return
	}
	ctx, cancel := context.WithTimeout(o.ctx, policyProbeTimeout(policy))
	defer cancel()
	if err := probeOwnedMCPSession(ctx, session); err != nil {
		o.failSessionForGeneration(key, generation, app.MCPFailurePing, pingErrorKind(err), "external MCP ping failed")
		return
	}
	o.recordSuccessForGeneration(key, generation, nil, false)
}

func (o *runtimeOwner) backgroundReconnect(key runtimeOwnerKey, policy model.MCPHealthPolicy) {
	generation, ok := o.beginWork(key)
	if !ok {
		return
	}
	defer o.endWork(key, generation)
	ctx, cancel := context.WithTimeout(o.ctx, policyProbeTimeout(policy))
	defer cancel()
	_, _, _ = o.ensureHealthySessionForGeneration(ctx, key.environmentID, key.mcpID, generation)
}

func (o *runtimeOwner) beginWork(key runtimeOwnerKey) (uint64, bool) {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.closed || o.inFlight[key] {
		return 0, false
	}
	o.inFlight[key] = true
	observation := o.observations[key]
	observation.EnvironmentID = key.environmentID
	observation.MCPID = key.mcpID
	observation.DesiredEnabled = true
	if observation.State == "" {
		observation.State = app.MCPHealthConfigured
	}
	observation.InFlight = true
	o.observations[key] = observation
	return o.generations[key], true
}

func (o *runtimeOwner) endWork(key runtimeOwnerKey, generation uint64) {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.generations[key] != generation {
		return
	}
	delete(o.inFlight, key)
	observation := o.observations[key]
	observation.InFlight = false
	o.observations[key] = observation
}

func (o *runtimeOwner) generation(key runtimeOwnerKey) uint64 {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.generations[key]
}

func (o *runtimeOwner) registerKey(key runtimeOwnerKey) uint64 {
	o.mu.Lock()
	defer o.mu.Unlock()
	if !o.closed {
		observation := o.observations[key]
		observation.EnvironmentID = key.environmentID
		observation.MCPID = key.mcpID
		observation.DesiredEnabled = true
		if observation.State == "" {
			observation.State = app.MCPHealthConfigured
		}
		o.observations[key] = observation
	}
	return o.generations[key]
}

func (o *runtimeOwner) observation(key runtimeOwnerKey) (app.MCPRuntimeObservation, bool) {
	o.mu.Lock()
	defer o.mu.Unlock()
	observation, ok := o.observations[key]
	observation.ToolInventory = append([]app.MCPToolInventoryItem(nil), observation.ToolInventory...)
	return observation, ok
}

func (o *runtimeOwner) invalidateChangedDesiredConfig(key runtimeOwnerKey, definition model.MCPDefinition) bool {
	fingerprintBytes, err := json.Marshal(struct {
		Transport    string                `json:"transport"`
		AuthMode     string                `json:"auth_mode"`
		Endpoint     string                `json:"endpoint"`
		HeaderRefs   map[string]string     `json:"header_refs"`
		Executable   string                `json:"executable"`
		Args         []string              `json:"args"`
		EnvRefs      map[string]string     `json:"env_refs"`
		HealthPolicy model.MCPHealthPolicy `json:"health_policy"`
	}{
		Transport: definition.Transport, AuthMode: definition.AuthMode, Endpoint: definition.Endpoint,
		HeaderRefs: definition.HeaderRefs, Executable: definition.Executable, Args: definition.Args,
		EnvRefs: definition.EnvRefs, HealthPolicy: definition.HealthPolicy,
	})
	if err != nil {
		return false
	}
	fingerprint := string(fingerprintBytes)

	var session ownedMCPSession
	o.mu.Lock()
	if o.closed {
		o.mu.Unlock()
		return false
	}
	previous, exists := o.desiredFingerprints[key]
	if !exists {
		o.desiredFingerprints[key] = fingerprint
		o.mu.Unlock()
		return false
	}
	if previous == fingerprint {
		o.mu.Unlock()
		return false
	}
	session = o.sessions[key]
	delete(o.sessions, key)
	delete(o.observations, key)
	delete(o.inFlight, key)
	o.generations[key]++
	o.desiredFingerprints[key] = fingerprint
	o.mu.Unlock()
	if session != nil {
		_ = session.Close()
	}
	return true
}

func (o *runtimeOwner) markDesired(key runtimeOwnerKey, generation uint64, transport string) {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.closed || o.generations[key] != generation {
		return
	}
	observation := o.observations[key]
	observation.EnvironmentID = key.environmentID
	observation.MCPID = key.mcpID
	observation.DesiredEnabled = true
	observation.Transport = transport
	if observation.State == "" {
		observation.State = app.MCPHealthConfigured
	}
	o.observations[key] = observation
}

func (o *runtimeOwner) recordSuccess(key runtimeOwnerKey, tools []*mcp.Tool, inventoryFetched bool) {
	o.recordSuccessForGeneration(key, o.generation(key), tools, inventoryFetched)
}

func (o *runtimeOwner) recordSuccessForGeneration(key runtimeOwnerKey, generation uint64, tools []*mcp.Tool, inventoryFetched bool) {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.closed || o.generations[key] != generation {
		return
	}
	now := time.Now().UTC()
	observation := o.observations[key]
	observation.EnvironmentID = key.environmentID
	observation.MCPID = key.mcpID
	observation.DesiredEnabled = true
	observation.State = app.MCPHealthHealthy
	observation.FailureStage = ""
	observation.ErrorKind = ""
	observation.Message = ""
	observation.LastCheckAt = timeReference(now)
	observation.LastHealthyAt = timeReference(now)
	observation.LastSuccessAt = timeReference(now)
	observation.ConsecutiveFailures = 0
	observation.NextReconnectAt = nil
	observation.InFlight = o.inFlight[key]
	if definition, err := o.service.MCPs.Get(key.mcpID); err == nil {
		observation.Transport = definition.Transport
	}
	if inventoryFetched {
		observation.ToolInventory = publicInventory(tools)
		observation.InventoryFetchedAt = timeReference(now)
	}
	o.observations[key] = observation
}

func (o *runtimeOwner) recordFailure(key runtimeOwnerKey, stage app.MCPFailureStage, kind, message string) {
	o.recordFailureForGeneration(key, o.generation(key), stage, kind, message)
}

func (o *runtimeOwner) recordFailureForGeneration(key runtimeOwnerKey, generation uint64, stage app.MCPFailureStage, kind, message string) {
	definition, _ := o.service.MCPs.Get(key.mcpID)
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.closed || o.generations[key] != generation {
		return
	}
	now := time.Now().UTC()
	observation := o.observations[key]
	observation.EnvironmentID = key.environmentID
	observation.MCPID = key.mcpID
	observation.DesiredEnabled = true
	observation.Transport = definition.Transport
	observation.State = app.MCPHealthError
	observation.FailureStage = stage
	observation.ErrorKind = kind
	observation.Message = message
	observation.LastCheckAt = timeReference(now)
	observation.ConsecutiveFailures++
	observation.InFlight = o.inFlight[key]
	if definition.HealthPolicy.AutoReconnect && definition.HealthPolicy.ReconnectIntervalSeconds > 0 {
		next := now.Add(time.Duration(definition.HealthPolicy.ReconnectIntervalSeconds) * time.Second)
		observation.NextReconnectAt = &next
	} else {
		observation.NextReconnectAt = nil
	}
	o.observations[key] = observation
}

func (o *runtimeOwner) failSession(key runtimeOwnerKey, stage app.MCPFailureStage, kind, message string) {
	generation := o.generation(key)
	o.failSessionForGeneration(key, generation, stage, kind, message)
}

func (o *runtimeOwner) failSessionForGeneration(key runtimeOwnerKey, generation uint64, stage app.MCPFailureStage, kind, message string) {
	o.mu.Lock()
	if o.closed || o.generations[key] != generation {
		o.mu.Unlock()
		return
	}
	session := o.sessions[key]
	delete(o.sessions, key)
	o.mu.Unlock()
	if session != nil {
		_ = session.Close()
	}
	o.recordFailureForGeneration(key, generation, stage, kind, message)
}

func listOwnedMCPTools(ctx context.Context, session ownedMCPSession) ([]*mcp.Tool, error) {
	params := &mcp.ListToolsParams{}
	tools := make([]*mcp.Tool, 0)
	for len(tools) < runtimeOwnerInventoryLimit {
		page, err := session.ListTools(ctx, params)
		if err != nil {
			return nil, err
		}
		remaining := runtimeOwnerInventoryLimit - len(tools)
		if len(page.Tools) > remaining {
			page.Tools = page.Tools[:remaining]
		}
		tools = append(tools, page.Tools...)
		if page.NextCursor == "" || len(tools) == runtimeOwnerInventoryLimit {
			return tools, nil
		}
		params.Cursor = page.NextCursor
	}
	return tools, nil
}

// Ping was removed by MCP 2026-07-28. The SDK exposes the negotiated version,
// so sessionless HTTP uses a safe discovery surface while legacy/stdio
// sessions use the protocol Ping required by their negotiated protocol.
func probeOwnedMCPSession(ctx context.Context, session ownedMCPSession) error {
	type initializedSession interface {
		InitializeResult() *mcp.InitializeResult
	}
	if initialized, ok := session.(initializedSession); ok {
		result := initialized.InitializeResult()
		if result != nil && result.ProtocolVersion >= "2026-07-28" {
			_, err := session.ListTools(ctx, &mcp.ListToolsParams{})
			return err
		}
	}
	return session.Ping(ctx, nil)
}

func publicInventory(tools []*mcp.Tool) []app.MCPToolInventoryItem {
	items := make([]app.MCPToolInventoryItem, 0, len(tools))
	for _, tool := range tools {
		if tool != nil {
			items = append(items, app.MCPToolInventoryItem{Name: tool.Name, Title: tool.Title, Description: tool.Description})
		}
	}
	return items
}

func sanitizedDesiredConfig(definition model.MCPDefinition) app.MCPRuntimeDesiredConfig {
	return app.MCPRuntimeDesiredConfig{
		ID:                  definition.ID,
		Name:                definition.Name,
		Transport:           definition.Transport,
		AuthMode:            definition.AuthMode,
		EndpointConfigured:  definition.Endpoint != "",
		HeaderReferenceKeys: sortedMapKeys(definition.HeaderRefs),
		Executable:          definition.Executable,
		Args:                append([]string(nil), definition.Args...),
		EnvReferenceKeys:    sortedMapKeys(definition.EnvRefs),
		HealthPolicy:        definition.HealthPolicy,
	}
}

func sortedMapKeys(values map[string]string) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func observationStatus(observation app.MCPRuntimeObservation) app.MCPHealthStatus {
	return app.MCPHealthStatus{MCPID: observation.MCPID, State: observation.State, ErrorKind: observation.ErrorKind, Message: observation.Message}
}

func pingErrorKind(err error) string {
	kind := runtimeMCPErrorKind(err)
	if kind == "connection_failed" {
		return "ping_protocol_failure"
	}
	return kind
}

func toolOperationErrorKind(err error, fallback string) string {
	kind := runtimeMCPErrorKind(err)
	if kind == "connection_failed" {
		return fallback
	}
	return kind
}

func containsValue(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func checkDue(last *time.Time, seconds int64, now time.Time) bool {
	return last == nil || !last.Add(time.Duration(seconds)*time.Second).After(now)
}

func policyProbeTimeout(policy model.MCPHealthPolicy) time.Duration {
	if policy.ProbeTimeoutSeconds > 0 {
		return time.Duration(policy.ProbeTimeoutSeconds) * time.Second
	}
	return runtimeOwnerReconcileTimeout
}

func timeReference(value time.Time) *time.Time { return &value }
