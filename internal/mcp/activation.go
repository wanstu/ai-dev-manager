package mcp

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"
	"sync"

	"ai-dev-manager/internal/model"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	HealthDisabled Health = "disabled"
	HealthStarting Health = "starting"
	HealthHealthy  Health = "healthy"
	HealthError    Health = "error"
	HealthStopped  Health = "stopped"
)

type ActivationErrorKind string

const (
	ActivationErrInvalidDefinition    ActivationErrorKind = "invalid_definition"
	ActivationErrUnsupportedTransport ActivationErrorKind = "unsupported_transport"
	ActivationErrMissingEnvRef        ActivationErrorKind = "missing_env_ref"
	ActivationErrConnect              ActivationErrorKind = "connect"
	ActivationErrProbe                ActivationErrorKind = "probe"
)

type ActivationError struct {
	Kind    ActivationErrorKind
	MCPID   string
	RefName string
	Err     error
}

func (e *ActivationError) Error() string {
	switch e.Kind {
	case ActivationErrInvalidDefinition:
		return fmt.Sprintf("invalid MCP definition %q", e.MCPID)
	case ActivationErrUnsupportedTransport:
		return fmt.Sprintf("unsupported MCP transport for %q", e.MCPID)
	case ActivationErrMissingEnvRef:
		return fmt.Sprintf("missing environment reference %q for MCP %q", e.RefName, e.MCPID)
	case ActivationErrConnect:
		return fmt.Sprintf("connect MCP %q failed", e.MCPID)
	case ActivationErrProbe:
		return fmt.Sprintf("probe MCP %q failed", e.MCPID)
	default:
		return fmt.Sprintf("MCP activation %q failed", e.MCPID)
	}
}

func (e *ActivationError) Unwrap() error { return e.Err }

type ActivationStatus struct {
	WorkspaceID   string              `json:"workspace_id"`
	ID            string              `json:"id"`
	Transport     string              `json:"transport,omitempty"`
	Health        Health              `json:"health"`
	Source        model.Scope         `json:"source,omitempty"`
	EnabledSource model.Scope         `json:"enabled_source,omitempty"`
	ToolNames     []string            `json:"tool_names,omitempty"`
	ErrorKind     ActivationErrorKind `json:"error_kind,omitempty"`
	Error         string              `json:"error,omitempty"`
}

type activeSession struct {
	status  ActivationStatus
	session *sdkmcp.ClientSession
}

type Activator struct {
	mu        sync.Mutex
	sessions  map[string]*activeSession
	lookupEnv func(string) (string, bool)
}

func NewActivator(lookupEnv func(string) (string, bool)) *Activator {
	if lookupEnv == nil {
		lookupEnv = os.LookupEnv
	}
	return &Activator{sessions: make(map[string]*activeSession), lookupEnv: lookupEnv}
}

func (a *Activator) Activate(ctx context.Context, workspaceID string, entry Entry) (ActivationStatus, error) {
	status := ActivationStatus{
		WorkspaceID:   workspaceID,
		ID:            entry.Definition.ID,
		Transport:     entry.Definition.Transport,
		Health:        HealthStarting,
		Source:        entry.Source,
		EnabledSource: entry.EnabledSource,
	}
	if entry.Definition.Enabled == nil || !*entry.Definition.Enabled {
		status.Health = HealthDisabled
		return status, nil
	}
	if strings.TrimSpace(workspaceID) == "" || strings.TrimSpace(entry.Definition.ID) == "" {
		return activationFailure(status, &ActivationError{Kind: ActivationErrInvalidDefinition, MCPID: entry.Definition.ID})
	}

	key := activationKey(workspaceID, entry.Definition.ID)
	a.mu.Lock()
	if current, ok := a.sessions[key]; ok && current.status.Health == HealthHealthy {
		result := cloneActivationStatus(current.status)
		a.mu.Unlock()
		return result, nil
	}
	a.mu.Unlock()

	client := sdkmcp.NewClient(&sdkmcp.Implementation{Name: "ai-dev-manager", Version: "v0.2.0"}, nil)
	session, err := a.connect(ctx, client, entry.Definition)
	if err != nil {
		failed, activationErr := activationFailure(status, err)
		a.record(key, failed, nil)
		return failed, activationErr
	}
	tools, err := session.ListTools(ctx, nil)
	if err != nil {
		_ = session.Close()
		failed, activationErr := activationFailure(status, &ActivationError{Kind: ActivationErrProbe, MCPID: entry.Definition.ID, Err: err})
		a.record(key, failed, nil)
		return failed, activationErr
	}
	status.Health = HealthHealthy
	status.ToolNames = make([]string, 0, len(tools.Tools))
	for _, tool := range tools.Tools {
		status.ToolNames = append(status.ToolNames, tool.Name)
	}
	sort.Strings(status.ToolNames)

	a.mu.Lock()
	if current, ok := a.sessions[key]; ok && current.status.Health == HealthHealthy {
		result := cloneActivationStatus(current.status)
		a.mu.Unlock()
		_ = session.Close()
		return result, nil
	}
	a.sessions[key] = &activeSession{status: cloneActivationStatus(status), session: session}
	a.mu.Unlock()
	return cloneActivationStatus(status), nil
}

func (a *Activator) connect(ctx context.Context, client *sdkmcp.Client, definition model.MCPDefinition) (*sdkmcp.ClientSession, error) {
	switch strings.ToLower(strings.TrimSpace(definition.Transport)) {
	case "stdio":
		if strings.TrimSpace(definition.Command) == "" {
			return nil, &ActivationError{Kind: ActivationErrInvalidDefinition, MCPID: definition.ID}
		}
		env, err := a.childEnvironment(definition)
		if err != nil {
			return nil, err
		}
		command := exec.Command(definition.Command, definition.Args...)
		command.Env = env
		session, err := client.Connect(ctx, &sdkmcp.CommandTransport{Command: command}, nil)
		if err != nil {
			return nil, &ActivationError{Kind: ActivationErrConnect, MCPID: definition.ID, Err: err}
		}
		return session, nil
	case "streamable-http":
		if strings.TrimSpace(definition.URL) == "" {
			return nil, &ActivationError{Kind: ActivationErrInvalidDefinition, MCPID: definition.ID}
		}
		session, err := client.Connect(ctx, &sdkmcp.StreamableClientTransport{Endpoint: definition.URL}, nil)
		if err != nil {
			return nil, &ActivationError{Kind: ActivationErrConnect, MCPID: definition.ID, Err: err}
		}
		return session, nil
	default:
		return nil, &ActivationError{Kind: ActivationErrUnsupportedTransport, MCPID: definition.ID}
	}
}

func (a *Activator) childEnvironment(definition model.MCPDefinition) ([]string, error) {
	values := make(map[string]string)
	for _, name := range []string{"PATH", "PATHEXT", "SystemRoot", "ComSpec", "TEMP", "TMP", "USERPROFILE", "HOME"} {
		if value, ok := os.LookupEnv(name); ok {
			values[name] = value
		}
	}
	for name, value := range definition.Env {
		values[name] = value
	}
	for childName, refName := range definition.EnvRefs {
		value, ok := a.lookupEnv(refName)
		if !ok {
			return nil, &ActivationError{Kind: ActivationErrMissingEnvRef, MCPID: definition.ID, RefName: refName}
		}
		values[childName] = value
	}
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	result := make([]string, 0, len(keys))
	for _, key := range keys {
		result = append(result, key+"="+values[key])
	}
	return result, nil
}

func (a *Activator) record(key string, status ActivationStatus, session *sdkmcp.ClientSession) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.sessions[key] = &activeSession{status: cloneActivationStatus(status), session: session}
}

func (a *Activator) Get(workspaceID, mcpID string) (ActivationStatus, bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	current, ok := a.sessions[activationKey(workspaceID, mcpID)]
	if !ok {
		return ActivationStatus{}, false
	}
	return cloneActivationStatus(current.status), true
}

func (a *Activator) Stop(workspaceID, mcpID string) error {
	key := activationKey(workspaceID, mcpID)
	a.mu.Lock()
	current, ok := a.sessions[key]
	if ok {
		delete(a.sessions, key)
	}
	a.mu.Unlock()
	if !ok || current.session == nil {
		return nil
	}
	return current.session.Close()
}

func (a *Activator) StopWorkspace(workspaceID string) error {
	a.mu.Lock()
	var sessions []*sdkmcp.ClientSession
	prefix := workspaceID + "\x00"
	for key, current := range a.sessions {
		if strings.HasPrefix(key, prefix) {
			sessions = append(sessions, current.session)
			delete(a.sessions, key)
		}
	}
	a.mu.Unlock()
	return closeSessions(sessions)
}

func (a *Activator) StopAll() error {
	a.mu.Lock()
	sessions := make([]*sdkmcp.ClientSession, 0, len(a.sessions))
	for key, current := range a.sessions {
		sessions = append(sessions, current.session)
		delete(a.sessions, key)
	}
	a.mu.Unlock()
	return closeSessions(sessions)
}

func closeSessions(sessions []*sdkmcp.ClientSession) error {
	var errs []error
	for _, session := range sessions {
		if session == nil {
			continue
		}
		if err := session.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func activationFailure(status ActivationStatus, err error) (ActivationStatus, error) {
	status.Health = HealthError
	var activationErr *ActivationError
	if ok := asActivationError(err, &activationErr); ok {
		status.ErrorKind = activationErr.Kind
		status.Error = activationErr.Error()
	}
	return status, err
}

func activationKey(workspaceID, mcpID string) string { return workspaceID + "\x00" + mcpID }

func cloneActivationStatus(source ActivationStatus) ActivationStatus {
	clone := source
	clone.ToolNames = append([]string(nil), source.ToolNames...)
	return clone
}

func asActivationError(err error, target **ActivationError) bool {
	for err != nil {
		if typed, ok := err.(*ActivationError); ok {
			*target = typed
			return true
		}
		type unwrapper interface{ Unwrap() error }
		value, ok := err.(unwrapper)
		if !ok {
			break
		}
		err = value.Unwrap()
	}
	return false
}
