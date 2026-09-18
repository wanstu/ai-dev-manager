package gateway

import (
	"context"
	"fmt"

	"ai-dev-manager-v2/internal/app"
	"ai-dev-manager-v2/internal/catalog"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type GlobalExecInput struct {
	Executable     string   `json:"executable"`
	Args           []string `json:"args,omitempty"`
	TimeoutMS      int64    `json:"timeout_ms,omitempty"`
	MaxOutputBytes int      `json:"max_output_bytes,omitempty"`
}

type GlobalMCPInput struct {
	MCPID string `json:"mcp_id"`
}

type GlobalMCPCallInput struct {
	MCPID     string         `json:"mcp_id"`
	Tool      string         `json:"tool"`
	Arguments map[string]any `json:"arguments,omitempty"`
}

type GlobalSkillInput struct {
	SkillID  string `json:"skill_id"`
	Path     string `json:"path,omitempty"`
	MaxBytes int    `json:"max_bytes,omitempty"`
}

type GlobalSkillFilesInput struct {
	SkillID    string `json:"skill_id"`
	RootKind   string `json:"root_kind,omitempty"`
	MaxEntries int    `json:"max_entries,omitempty"`
}

func registerGlobalTools(server *mcp.Server, surface serverSurface, service *app.Service) {
	addScopedTool(server, surface, &mcp.Tool{
		Name:        "global_exec",
		Description: "Run one host-global executable without an Environment or writer. The command runs in ADM's isolated global-runtime directory and follows strict/full execution authorization.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in GlobalExecInput) (*mcp.CallToolResult, any, error) {
		result, err := service.GlobalExec(ctx, in.Executable, in.Args, in.TimeoutMS, in.MaxOutputBytes)
		return toolResult(result, err)
	})

	addScopedTool(server, surface, &mcp.Tool{
		Name:        "global_mcp_status",
		Description: "Probe one global MCP catalog entry without borrowing any Environment or writer.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in GlobalMCPInput) (*mcp.CallToolResult, any, error) {
		status, err := service.MCPProbe(ctx, in.MCPID)
		return toolResult(status, err)
	})

	addScopedTool(server, surface, &mcp.Tool{
		Name:        "global_mcp_tools",
		Description: "List tools from one global MCP catalog entry without an Environment or writer.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in GlobalMCPInput) (*mcp.CallToolResult, any, error) {
		session, err := connectGlobalMCP(ctx, service, in.MCPID)
		if err != nil {
			return toolResult(nil, err)
		}
		defer session.Close()
		result, err := session.ListTools(ctx, nil)
		return toolResult(result, err)
	})

	addScopedTool(server, surface, &mcp.Tool{
		Name:        "global_mcp_call",
		Description: "Call one tool on a global MCP catalog entry without an Environment or writer.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in GlobalMCPCallInput) (*mcp.CallToolResult, any, error) {
		session, err := connectGlobalMCP(ctx, service, in.MCPID)
		if err != nil {
			return toolResult(nil, err)
		}
		defer session.Close()
		result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: in.Tool, Arguments: in.Arguments})
		return toolResult(result, err)
	})

	addScopedTool(server, surface, &mcp.Tool{
		Name:        "global_skill_list",
		Description: "List catalog Skill availability without selecting an Environment.",
	}, func(context.Context, *mcp.CallToolRequest, EmptyInput) (*mcp.CallToolResult, any, error) {
		result, err := service.GlobalSkillAvailabilities()
		return toolResult(result, err)
	})

	addScopedTool(server, surface, &mcp.Tool{
		Name:        "global_skill_read",
		Description: "Read one global Skill artifact/support file without selecting an Environment.",
	}, func(_ context.Context, _ *mcp.CallToolRequest, in GlobalSkillInput) (*mcp.CallToolResult, any, error) {
		result, err := service.ReadGlobalSkill(in.SkillID, in.Path, in.MaxBytes)
		return toolResult(result, err)
	})

	addScopedTool(server, surface, &mcp.Tool{
		Name:        "global_skill_files",
		Description: "List bounded files for one global Skill artifact/support root without selecting an Environment.",
	}, func(_ context.Context, _ *mcp.CallToolRequest, in GlobalSkillFilesInput) (*mcp.CallToolResult, any, error) {
		result, err := service.GlobalSkillFiles(in.SkillID, in.RootKind, in.MaxEntries)
		return toolResult(result, err)
	})
}

func connectGlobalMCP(ctx context.Context, service *app.Service, mcpID string) (*mcp.ClientSession, error) {
	activation, status, err := service.ResolveGlobalMCPActivation(mcpID)
	if err != nil {
		return nil, err
	}
	if activation == nil {
		return nil, statusAsMCPError(status)
	}
	switch activation.Transport {
	case catalog.MCPTransportStreamableHTTP:
		return connectExternalMCP(ctx, mcpID, activation.Endpoint, activation.Headers)
	case catalog.MCPTransportStdio:
		cmd, err := service.GlobalMCPCommand(ctx, activation)
		if err != nil {
			return nil, err
		}
		client := mcp.NewClient(&mcp.Implementation{Name: serverName + "-global", Version: serverVersion}, nil)
		session, err := client.Connect(ctx, &mcp.CommandTransport{Command: cmd}, nil)
		if err != nil {
			return nil, &app.MCPError{MCPID: mcpID, ErrorKind: app.ClassifyMCPError(err), Message: "global MCP stdio connection failed"}
		}
		return session, nil
	default:
		return nil, fmt.Errorf("unsupported MCP transport %q", activation.Transport)
	}
}
