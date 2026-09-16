package app

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"ai-dev-manager-v2/internal/catalog"
	"ai-dev-manager-v2/internal/model"
	productversion "ai-dev-manager-v2/internal/version"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const mcpHealthProbeTimeout = 10 * time.Second

type MCPHealthState string

const (
	MCPHealthConfigured MCPHealthState = "configured"
	MCPHealthDisabled   MCPHealthState = "disabled"
	MCPHealthHealthy    MCPHealthState = "healthy"
	MCPHealthError      MCPHealthState = "error"
)

type MCPHealthStatus struct {
	MCPID     string         `json:"mcp_id"`
	State     MCPHealthState `json:"state"`
	ErrorKind string         `json:"error_kind,omitempty"`
	Message   string         `json:"message,omitempty"`
}

type MCPFailureStage string

const (
	MCPFailureActivation MCPFailureStage = "activation"
	MCPFailureConnect    MCPFailureStage = "connect"
	MCPFailurePing       MCPFailureStage = "ping"
	MCPFailureDiscover   MCPFailureStage = "discover"
	MCPFailureListTools  MCPFailureStage = "list_tools"
	MCPFailureCall       MCPFailureStage = "call"
	MCPFailureReconnect  MCPFailureStage = "reconnect"
)

// MCPToolInventoryItem is the bounded, public subset of a discovered tool.
// Input schemas remain available from environment_mcp_tools on demand.
type MCPToolInventoryItem struct {
	Name        string `json:"name"`
	Title       string `json:"title,omitempty"`
	Description string `json:"description,omitempty"`
}

// MCPRuntimeObservation is ephemeral evidence owned by one Gateway process.
// It is deliberately absent from persisted model.State.
type MCPRuntimeObservation struct {
	EnvironmentID       string                 `json:"environment_id"`
	MCPID               string                 `json:"mcp_id"`
	DesiredEnabled      bool                   `json:"desired_enabled"`
	Transport           string                 `json:"transport,omitempty"`
	State               MCPHealthState         `json:"state"`
	FailureStage        MCPFailureStage        `json:"failure_stage,omitempty"`
	ErrorKind           string                 `json:"error_kind,omitempty"`
	Message             string                 `json:"message,omitempty"`
	LastCheckAt         *time.Time             `json:"last_check_at,omitempty"`
	LastHealthyAt       *time.Time             `json:"last_healthy_at,omitempty"`
	LastSuccessAt       *time.Time             `json:"last_success_at,omitempty"`
	ConsecutiveFailures int                    `json:"consecutive_failures"`
	NextReconnectAt     *time.Time             `json:"next_reconnect_at,omitempty"`
	InFlight            bool                   `json:"in_flight"`
	ToolInventory       []MCPToolInventoryItem `json:"tool_inventory,omitempty"`
	InventoryFetchedAt  *time.Time             `json:"inventory_fetched_at,omitempty"`
}

type MCPRuntimeDesiredConfig struct {
	ID                  string                `json:"id"`
	Name                string                `json:"name"`
	Transport           string                `json:"transport"`
	AuthMode            string                `json:"auth_mode"`
	EndpointConfigured  bool                  `json:"endpoint_configured"`
	HeaderReferenceKeys []string              `json:"header_reference_keys,omitempty"`
	Executable          string                `json:"executable,omitempty"`
	Args                []string              `json:"args,omitempty"`
	EnvReferenceKeys    []string              `json:"env_reference_keys,omitempty"`
	HealthPolicy        model.MCPHealthPolicy `json:"health_policy"`
}

type MCPRuntimeInspection struct {
	Definition  *MCPRuntimeDesiredConfig `json:"definition,omitempty"`
	Observation MCPRuntimeObservation    `json:"observation"`
}

type MCPError struct {
	MCPID     string `json:"mcp_id"`
	ErrorKind string `json:"error_kind"`
	Message   string `json:"message"`
}

func (e MCPError) Error() string {
	return fmt.Sprintf("mcp_id=%s error_kind=%s message=%s", e.MCPID, e.ErrorKind, e.Message)
}

// MCPActivation is resolved only at a runtime activation boundary. Endpoint
// and header values may contain secrets and must never be persisted or exposed
// through ordinary inspection/status output.
type MCPActivation struct {
	MCPID      string
	Transport  string
	Endpoint   string
	Headers    map[string]string
	Executable string
	Args       []string
	Env        map[string]string
}

// ResolveMCPActivation separates persisted desired configuration from observed
// runtime state. A nil activation means the MCP is disabled, incomplete, or
// otherwise not activatable; status explains that desired/configuration state.
func (s *Service) ResolveMCPActivation(environmentID, mcpID string) (*MCPActivation, MCPHealthStatus, error) {
	env, err := s.Environments.Get(environmentID)
	if err != nil {
		return nil, MCPHealthStatus{}, err
	}
	enabled := false
	for _, id := range env.EnabledMCPIDs {
		if id == mcpID {
			enabled = true
			break
		}
	}
	if !enabled {
		return nil, MCPHealthStatus{MCPID: mcpID, State: MCPHealthDisabled}, nil
	}

	activation, status, err := s.resolveCatalogMCPActivation(mcpID)
	if err != nil || activation == nil {
		return activation, status, err
	}

	if activation.Transport == catalog.MCPTransportStdio {
		rt, _, err := s.Runtime(environmentID)
		if err != nil {
			return nil, MCPHealthStatus{}, err
		}
		_, err = rt.Command(context.Background(), activation.Executable, activation.Args, activation.Env)
		if err != nil {
			return nil, MCPHealthStatus{MCPID: mcpID, State: MCPHealthError, ErrorKind: "executable_not_allowed", Message: "stdio MCP executable is unavailable under Environment authority"}, nil
		}
	}
	return activation, MCPHealthStatus{MCPID: mcpID, State: MCPHealthConfigured}, nil
}

// resolveCatalogMCPActivation resolves configuration only; Agent callers must check selection first.
func (s *Service) resolveCatalogMCPActivation(mcpID string) (*MCPActivation, MCPHealthStatus, error) {
	entry, err := s.MCPs.Get(mcpID)
	if err != nil {
		return nil, MCPHealthStatus{
			MCPID:   mcpID,
			State:   MCPHealthConfigured,
			Message: "mcp catalog entry is unresolved",
		}, nil
	}
	if s.hasUnresolvedEnvRef(entry.Endpoint) || s.hasUnresolvedMapRef(entry.HeaderRefs) || s.hasUnresolvedMapRef(entry.EnvRefs) {
		return nil, MCPHealthStatus{
			MCPID:     mcpID,
			State:     MCPHealthConfigured,
			ErrorKind: "unresolved_secret_reference",
			Message:   "MCP 配置引用的环境变量尚未解析。请更新系统/用户环境变量，在 ADM 内刷新环境变量后重新探测。",
		}, nil
	}
	endpoint, _ := s.expandEnvironmentTemplate(entry.Endpoint)
	activation := &MCPActivation{
		MCPID:      mcpID,
		Transport:  entry.Transport,
		Endpoint:   endpoint,
		Headers:    s.resolveMap(entry.HeaderRefs),
		Executable: entry.Executable,
		Args:       append([]string(nil), entry.Args...),
		Env:        s.resolveMap(entry.EnvRefs),
	}
	return activation, MCPHealthStatus{MCPID: mcpID, State: MCPHealthConfigured}, nil
}

// ProbeMCPHealth performs one bounded on-demand probe for management callers.
// The long-lived Gateway uses the same activation resolution but owns sessions
// in its runtime owner instead of persisting or reusing this transient probe.
func (s *Service) ProbeMCPHealth(ctx context.Context, environmentID, mcpID string) (MCPHealthStatus, error) {
	activation, status, err := s.ResolveMCPActivation(environmentID, mcpID)
	if err != nil {
		return MCPHealthStatus{}, err
	}
	if activation == nil {
		return status, nil
	}

	return s.probeMCPActivation(ctx, mcpID, func(ctx context.Context) (mcp.Transport, error) {
		return s.mcpTransport(ctx, environmentID, activation)
	})
}

func (s *Service) probeMCPActivation(ctx context.Context, mcpID string, transportFor func(context.Context) (mcp.Transport, error)) (MCPHealthStatus, error) {
	definition, _ := s.MCPs.Get(mcpID)
	timeout := mcpHealthProbeTimeout
	if definition.HealthPolicy.ProbeTimeoutSeconds > 0 {
		timeout = time.Duration(definition.HealthPolicy.ProbeTimeoutSeconds) * time.Second
	}
	probeCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	client := mcp.NewClient(&mcp.Implementation{Name: "adm-v2-health-probe", Version: productversion.Current()}, nil)
	transport, err := transportFor(probeCtx)
	if err != nil {
		return mcpHealthErrorStatus(mcpID, "activation_failed"), nil
	}
	session, err := client.Connect(probeCtx, transport, nil)
	if err != nil {
		return mcpHealthErrorStatus(mcpID, classifyMCPError(err)), nil
	}
	defer session.Close()

	if _, err := session.ListTools(probeCtx, nil); err != nil {
		kind := classifyMCPError(err)
		if kind == "connection_failed" {
			kind = "tool_list_failed"
		}
		return mcpHealthErrorStatus(mcpID, kind), nil
	}
	return MCPHealthStatus{MCPID: mcpID, State: MCPHealthHealthy}, nil
}

func resolveMap(refs map[string]string) map[string]string {
	if len(refs) == 0 {
		return nil
	}
	resolved := make(map[string]string, len(refs))
	for key, value := range refs {
		resolved[key], _ = expandEnvironmentTemplate(value)
	}
	return resolved
}

func hasUnresolvedEnvRef(value string) bool {
	_, unresolved := expandEnvironmentTemplate(value)
	return unresolved
}

func expandEnvironmentTemplate(value string) (string, bool) {
	unresolved := false
	expanded := os.Expand(value, func(key string) string {
		name := key
		fallback := ""
		hasFallback := false
		if index := strings.Index(key, ":-"); index >= 0 {
			name = key[:index]
			fallback = key[index+2:]
			hasFallback = true
		}
		if current, ok := os.LookupEnv(name); ok && (!hasFallback || current != "") {
			return current
		}
		if hasFallback {
			return fallback
		}
		unresolved = true
		return ""
	})
	return expanded, unresolved
}

func hasUnresolvedMapRef(refs map[string]string) bool {
	for _, value := range refs {
		if hasUnresolvedEnvRef(value) {
			return true
		}
	}
	return false
}

func (s *Service) environmentLookup(name string) (string, bool) {
	if s != nil && s.HostEnvironment != nil {
		return s.HostEnvironment.Lookup(name)
	}
	return os.LookupEnv(name)
}

func (s *Service) expandEnvironmentTemplate(value string) (string, bool) {
	unresolved := false
	expanded := os.Expand(value, func(key string) string {
		name := key
		fallback := ""
		hasFallback := false
		if index := strings.Index(key, ":-"); index >= 0 {
			name = key[:index]
			fallback = key[index+2:]
			hasFallback = true
		}
		if current, ok := s.environmentLookup(name); ok && (!hasFallback || current != "") {
			return current
		}
		if hasFallback {
			return fallback
		}
		unresolved = true
		return ""
	})
	return expanded, unresolved
}

func (s *Service) resolveMap(refs map[string]string) map[string]string {
	if len(refs) == 0 {
		return nil
	}
	resolved := make(map[string]string, len(refs))
	for key, value := range refs {
		resolved[key], _ = s.expandEnvironmentTemplate(value)
	}
	return resolved
}

func (s *Service) hasUnresolvedEnvRef(value string) bool {
	_, unresolved := s.expandEnvironmentTemplate(value)
	return unresolved
}

func (s *Service) hasUnresolvedMapRef(refs map[string]string) bool {
	for _, value := range refs {
		if s.hasUnresolvedEnvRef(value) {
			return true
		}
	}
	return false
}

func (s *Service) mcpTransport(ctx context.Context, environmentID string, activation *MCPActivation) (mcp.Transport, error) {
	switch activation.Transport {
	case catalog.MCPTransportStreamableHTTP:
		transport := &mcp.StreamableClientTransport{Endpoint: activation.Endpoint, MaxRetries: -1, DisableStandaloneSSE: true}
		if len(activation.Headers) != 0 {
			transport.HTTPClient = &http.Client{Transport: mcpHeaderRoundTripper{base: http.DefaultTransport, headers: activation.Headers}}
		}
		return transport, nil
	case catalog.MCPTransportStdio:
		rt, _, err := s.Runtime(environmentID)
		if err != nil {
			return nil, err
		}
		cmd, err := rt.Command(ctx, activation.Executable, activation.Args, activation.Env)
		if err != nil {
			if isExecutableNotAllowedError(err) {
				s.recordExecDenial(environmentID, activation.Executable, "mcp_probe", err.Error())
			}
			return nil, err
		}
		return &mcp.CommandTransport{Command: cmd}, nil
	default:
		return nil, fmt.Errorf("unsupported mcp transport %q", activation.Transport)
	}
}

// MCPCommand returns the allowlisted command used for one resolved stdio
// activation. It is intentionally runtime-only and contains resolved values.
func (s *Service) MCPCommand(ctx context.Context, environmentID string, activation *MCPActivation) (*exec.Cmd, error) {
	if activation == nil || activation.Transport != catalog.MCPTransportStdio {
		return nil, fmt.Errorf("stdio MCP activation is required")
	}
	rt, _, err := s.Runtime(environmentID)
	if err != nil {
		return nil, err
	}
	cmd, err := rt.Command(ctx, activation.Executable, activation.Args, activation.Env)
	if err != nil {
		if isExecutableNotAllowedError(err) {
			s.recordExecDenial(environmentID, activation.Executable, "mcp_proxy", err.Error())
		}
		return nil, err
	}
	return cmd, nil
}

func classifyMCPError(err error) string {
	if err == nil {
		return ""
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return "timeout"
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return "timeout"
	}
	text := strings.ToLower(err.Error())
	switch {
	case strings.Contains(text, "deadline exceeded"), strings.Contains(text, "i/o timeout"), strings.Contains(text, "timed out"):
		return "timeout"
	case strings.Contains(text, "connection refused"), strings.Contains(text, "actively refused"):
		return "connection_refused"
	case strings.Contains(text, "401"), strings.Contains(text, "403"), strings.Contains(text, "unauthorized"), strings.Contains(text, "forbidden"):
		return "auth_failure"
	default:
		return "connection_failed"
	}
}

// ClassifyMCPError exposes the app-owned classifier to thin Gateway handlers.
// The canonical implementation remains classifyMCPError in this package.
func ClassifyMCPError(err error) string {
	return classifyMCPError(err)
}

func mcpHealthErrorStatus(mcpID, kind string) MCPHealthStatus {
	message := "MCP 探测失败。请检查 Endpoint、Transport 和网络连通性后重试。"
	switch kind {
	case "timeout":
		message = "MCP 探测超时。请确认 Endpoint 可访问；服务响应较慢时可适当提高 Probe timeout。"
	case "connection_refused":
		message = "MCP Endpoint 拒绝连接。请确认 MCP 服务已启动、Endpoint/端口正确，并且本机可以访问。"
	case "auth_failure":
		message = "MCP 鉴权失败。请检查 HTTP Auth、Header reference mapping，以及对应环境变量是否存在；变更系统/用户环境变量后可在 ADM 内刷新并重新探测。"
	case "tool_list_failed":
		message = "MCP 已建立连接，但读取 tools/list 失败。请确认 Endpoint 指向 Streamable HTTP MCP 服务并支持工具发现。"
	case "activation_failed":
		message = "MCP 无法建立运行配置。HTTP 请检查 Endpoint/Auth/Header references；stdio 请检查 executable、env references 与执行许可。"
	case "connection_failed":
		message = "MCP 协议连接失败。请检查 Endpoint、Transport、TLS/代理；若配置了 Header reference，请使用 Secret-backed headers，并确认引用的环境变量已注入 ADM 服务进程。"
	}
	return MCPHealthStatus{
		MCPID:     mcpID,
		State:     MCPHealthError,
		ErrorKind: kind,
		Message:   message,
	}
}

type mcpHeaderRoundTripper struct {
	base    http.RoundTripper
	headers map[string]string
}

func (r mcpHeaderRoundTripper) RoundTrip(request *http.Request) (*http.Response, error) {
	clone := request.Clone(request.Context())
	clone.Header = request.Header.Clone()
	for key, value := range r.headers {
		clone.Header.Set(key, value)
	}
	return r.base.RoundTrip(clone)
}
