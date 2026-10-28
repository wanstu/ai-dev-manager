package gateway

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"ai-dev-manager-v2/internal/app"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const compactAgentToolLimit = 50

type CompactActionInput struct {
	Action    string         `json:"action"`
	Arguments map[string]any `json:"arguments,omitempty"`
}

type CompactHelpInput struct {
	Group  string `json:"group,omitempty"`
	Action string `json:"action,omitempty"`
}

type compactAction struct {
	group      string
	action     string
	legacyName string
	tool       *mcp.Tool
	permission agentToolPermissionClass
}

func newCompactAgentServer(service *app.Service, owner *runtimeOwner) *mcp.Server {
	legacy := newServerForSurface(service, owner, serverSurfaceAgent)
	clientTransport, serverTransport := mcp.NewInMemoryTransports()
	ctx := context.Background()
	serverSession, err := legacy.Connect(ctx, serverTransport, nil)
	if err != nil {
		service.Log("error", "gateway.compact_agent_legacy_connect_failed", map[string]string{"error": err.Error()})
		return legacy
	}
	client := mcp.NewClient(&mcp.Implementation{Name: serverName + "-compact-router", Version: serverVersion}, nil)
	clientSession, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		_ = serverSession.Close()
		service.Log("error", "gateway.compact_agent_client_connect_failed", map[string]string{"error": err.Error()})
		return legacy
	}
	listed, err := clientSession.ListTools(ctx, nil)
	if err != nil {
		_ = clientSession.Close()
		_ = serverSession.Close()
		service.Log("error", "gateway.compact_agent_list_failed", map[string]string{"error": err.Error()})
		return legacy
	}

	compact := mcp.NewServer(
		&mcp.Implementation{Name: serverName, Version: serverVersion},
		&mcp.ServerOptions{Instructions: gatewayAgentInstructions},
	)

	groups := map[string]map[string]compactAction{}
	for _, tool := range listed.Tools {
		group, action := compactToolRoute(tool.Name)
		if group == "" || action == "" {
			group, action = "misc", tool.Name
		}
		if groups[group] == nil {
			groups[group] = map[string]compactAction{}
		}
		if _, exists := groups[group][action]; exists {
			action = tool.Name
		}
		groups[group][action] = compactAction{
			group:      group,
			action:     action,
			legacyName: tool.Name,
			tool:       tool,
			permission: agentToolPermission(tool.Name),
		}
	}

	callLegacy := func(ctx context.Context, legacyName string, arguments map[string]any) (*mcp.CallToolResult, error) {
		// Keep both in-memory sessions reachable for the full lifetime of the
		// compact server. The legacy server remains the single source of truth
		// for permission checks and handler semantics.
		_ = serverSession
		if arguments == nil {
			arguments = map[string]any{}
		}
		return clientSession.CallTool(ctx, &mcp.CallToolParams{Name: legacyName, Arguments: arguments})
	}

	registerCompactHelp(compact, groups)
	groupNames := make([]string, 0, len(groups))
	for group := range groups {
		groupNames = append(groupNames, group)
	}
	sort.Strings(groupNames)
	for _, group := range groupNames {
		registerCompactGroup(compact, group, groups[group], callLegacy)
	}

	return compact
}

func registerCompactGroup(
	server *mcp.Server,
	group string,
	actions map[string]compactAction,
	callLegacy func(context.Context, string, map[string]any) (*mcp.CallToolResult, error),
) {
	actionNames := make([]string, 0, len(actions))
	for action := range actions {
		actionNames = append(actionNames, action)
	}
	sort.Strings(actionNames)

	description := fmt.Sprintf(
		"ADM compact %s command. Select one action: %s. Put the legacy action input object in arguments. Use adm_help for the exact input schema and permission class.",
		group,
		strings.Join(actionNames, ", "),
	)
	mcp.AddTool(server, &mcp.Tool{
		Name:        "adm_" + group,
		Description: description,
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in CompactActionInput) (*mcp.CallToolResult, any, error) {
		action := strings.TrimSpace(in.Action)
		entry, ok := actions[action]
		if !ok {
			return nil, nil, fmt.Errorf(
				"unknown ADM %s action %q; available actions: %s",
				group,
				action,
				strings.Join(actionNames, ", "),
			)
		}
		result, err := callLegacy(ctx, entry.legacyName, in.Arguments)
		if err != nil {
			return nil, nil, err
		}
		return result, nil, nil
	})
}

func registerCompactHelp(server *mcp.Server, groups map[string]map[string]compactAction) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "adm_help",
		Description: "Discover compact ADM command groups/actions, original descriptions, permission classes, and exact legacy input schemas without exposing every action as a top-level MCP tool.",
	}, func(_ context.Context, _ *mcp.CallToolRequest, in CompactHelpInput) (*mcp.CallToolResult, any, error) {
		group := strings.TrimSpace(in.Group)
		action := strings.TrimSpace(in.Action)
		if group == "" {
			out := map[string]any{}
			groupNames := make([]string, 0, len(groups))
			for name := range groups {
				groupNames = append(groupNames, name)
			}
			sort.Strings(groupNames)
			for _, name := range groupNames {
				actions := make([]string, 0, len(groups[name]))
				for actionName := range groups[name] {
					actions = append(actions, actionName)
				}
				sort.Strings(actions)
				out[name] = actions
			}
			return toolResult(map[string]any{
				"tool_count": len(groupNames) + 1,
				"groups":     out,
				"usage":      "call adm_<group> with {action, arguments}; call adm_help with group/action for an exact input schema",
			}, nil)
		}

		actions, ok := groups[group]
		if !ok {
			return nil, nil, fmt.Errorf("unknown ADM compact group %q", group)
		}
		if action == "" {
			out := make([]map[string]any, 0, len(actions))
			names := make([]string, 0, len(actions))
			for name := range actions {
				names = append(names, name)
			}
			sort.Strings(names)
			for _, name := range names {
				entry := actions[name]
				out = append(out, compactActionHelp(entry))
			}
			return toolResult(out, nil)
		}
		entry, ok := actions[action]
		if !ok {
			return nil, nil, fmt.Errorf("unknown ADM %s action %q", group, action)
		}
		return toolResult(compactActionHelp(entry), nil)
	})
}

func compactActionHelp(entry compactAction) map[string]any {
	permission := "agent"
	switch entry.permission {
	case agentToolPermissionAdminOnly:
		permission = "admin_only"
	case agentToolPermissionStrongOnly:
		permission = "relaxed_agent_only"
	}
	return map[string]any{
		"group":        entry.group,
		"action":       entry.action,
		"legacy_name":  entry.legacyName,
		"description":  entry.tool.Description,
		"permission":   permission,
		"input_schema": entry.tool.InputSchema,
	}
}

func compactToolRoute(name string) (string, string) {
	switch name {
	case "read", "write", "edit", "delete", "tree", "search":
		return "file", name
	case "compare_files":
		return "file", "compare"
	case "exec":
		return "exec", "run"
	case "gateway_info":
		return "gateway", "info"
	case "logging_status":
		return "gateway", "logging_status"
	case "management_snapshot":
		return "management", "snapshot"
	case "investigate_endpoint":
		return "investigation", "endpoint"
	}

	switch {
	case strings.HasPrefix(name, "environment_mcp_"):
		return "mcp", "environment_" + strings.TrimPrefix(name, "environment_mcp_")
	case strings.HasPrefix(name, "global_mcp_"):
		return "mcp", "global_" + strings.TrimPrefix(name, "global_mcp_")
	case strings.HasPrefix(name, "mcp_"):
		return "mcp", strings.TrimPrefix(name, "mcp_")
	case strings.HasPrefix(name, "environment_skill_"):
		return "skill", "environment_" + strings.TrimPrefix(name, "environment_skill_")
	case strings.HasPrefix(name, "global_skill_"):
		return "skill", "global_" + strings.TrimPrefix(name, "global_skill_")
	case strings.HasPrefix(name, "skill_"):
		return "skill", strings.TrimPrefix(name, "skill_")
	case strings.HasPrefix(name, "environment_verifier_"):
		return "verifier", strings.TrimPrefix(name, "environment_verifier_")
	case strings.HasPrefix(name, "environment_writer_"):
		return "writer", strings.TrimPrefix(name, "environment_writer_")
	case strings.HasPrefix(name, "environment_"):
		return "environment", strings.TrimPrefix(name, "environment_")
	case strings.HasPrefix(name, "git_"):
		return "git", strings.TrimPrefix(name, "git_")
	case strings.HasPrefix(name, "exec_"):
		return "exec", strings.TrimPrefix(name, "exec_")
	case strings.HasPrefix(name, "workspace_"):
		return "workspace", strings.TrimPrefix(name, "workspace_")
	case strings.HasPrefix(name, "worktree_settings_"):
		return "workspace", "worktree_settings_" + strings.TrimPrefix(name, "worktree_settings_")
	case strings.HasPrefix(name, "code_intelligence_"):
		return "code", strings.TrimPrefix(name, "code_intelligence_")
	case strings.HasPrefix(name, "project_"):
		return "project", strings.TrimPrefix(name, "project_")
	case strings.HasPrefix(name, "memory_environment_"):
		return "memory", "environment_" + strings.TrimPrefix(name, "memory_environment_")
	case strings.HasPrefix(name, "memory_global_"):
		return "memory", "global_" + strings.TrimPrefix(name, "memory_global_")
	case strings.HasPrefix(name, "process_"):
		return "process", strings.TrimPrefix(name, "process_")
	case strings.HasPrefix(name, "run_"):
		return "run", strings.TrimPrefix(name, "run_")
	case strings.HasPrefix(name, "investigation_"):
		return "investigation", strings.TrimPrefix(name, "investigation_")
	case strings.HasPrefix(name, "resource_retention_"):
		return "retention", strings.TrimPrefix(name, "resource_retention_")
	case strings.HasPrefix(name, "strong_permission_control_"):
		return "security", "strong_" + strings.TrimPrefix(name, "strong_permission_control_")
	case strings.HasPrefix(name, "secret_"):
		return "secret", strings.TrimPrefix(name, "secret_")
	case strings.HasPrefix(name, "host_"):
		return "host", strings.TrimPrefix(name, "host_")
	case strings.HasPrefix(name, "gateway_"):
		return "gateway", strings.TrimPrefix(name, "gateway_")
	case strings.HasPrefix(name, "global_"):
		return "global", strings.TrimPrefix(name, "global_")
	default:
		return "misc", name
	}
}
