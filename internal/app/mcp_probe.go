package app

import (
	"ai-dev-manager-v2/internal/catalog"
	"ai-dev-manager-v2/internal/runtime"
	"context"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"os"
)

// MCPProbe is an explicit administrative catalog probe, never an Agent access gate.
// It creates no Environment and never invokes a remote business tool.
func (s *Service) MCPProbe(ctx context.Context, mcpID string) (MCPHealthStatus, error) {
	if _, err := s.MCPs.Get(mcpID); err != nil {
		return MCPHealthStatus{}, err
	}
	activation, status, err := s.resolveCatalogMCPActivation(mcpID)
	if err != nil || activation == nil {
		return status, err
	}
	if activation.Transport != catalog.MCPTransportStdio {
		return s.probeMCPActivation(ctx, mcpID, func(ctx context.Context) (mcp.Transport, error) { return s.mcpTransport(ctx, "", activation) })
	}
	// A global definition cannot silently borrow an Environment's working directory.
	root, err := os.MkdirTemp("", "adm-mcp-probe-")
	if err != nil {
		return mcpHealthErrorStatus(mcpID, "probe_directory_unavailable"), nil
	}
	defer os.RemoveAll(root)
	state, err := s.Store.Load()
	if err != nil {
		return MCPHealthStatus{}, err
	}
	allowed := append([]string(nil), state.AllowedExecutables...)
	if err != nil {
		return MCPHealthStatus{}, err
	}
	var rt *runtime.Runtime
	if s.HostEnvironment != nil {
		rt, err = runtime.NewWithExecutionPolicy(root, allowed, state.BlockedExecutables, s.HostEnvironment.Environ(), state.ExecFullAuthorization)
	} else {
		rt, err = runtime.NewWithExecutionPolicy(root, allowed, state.BlockedExecutables, nil, state.ExecFullAuthorization)
	}
	if err != nil {
		return mcpHealthErrorStatus(mcpID, "probe_directory_unavailable"), nil
	}
	// Validate separately to keep executable denials actionable and sanitized.
	if _, err := rt.Command(ctx, activation.Executable, activation.Args, activation.Env); err != nil {
		if isExecutableNotAllowedError(err) {
			s.recordExecDenial("", activation.Executable, "mcp_global_probe", err.Error())
			return mcpHealthErrorStatus(mcpID, "executable_not_allowed"), nil
		}
		return mcpHealthErrorStatus(mcpID, "executable_unavailable"), nil
	}
	return s.probeMCPActivation(ctx, mcpID, func(ctx context.Context) (mcp.Transport, error) {
		s.recordFullAuthorizationBypass(rt, "", activation.Executable, "mcp_global_probe")
		cmd, err := rt.Command(ctx, activation.Executable, activation.Args, activation.Env)
		if err != nil {
			return nil, err
		}
		return &mcp.CommandTransport{Command: cmd}, nil
	})
}
