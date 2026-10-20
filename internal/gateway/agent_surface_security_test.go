package gateway

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ai-dev-manager-v2/internal/app"
	"ai-dev-manager-v2/internal/catalog"
)

func TestAgentSurfaceKeepsPermissionGatedToolsDiscoverable(t *testing.T) {
	service := app.New(filepath.Join(t.TempDir(), "state.json"))
	ctx := context.Background()

	agent := connectInMemory(t, ctx, New(service))
	defer agent.Close()
	agentTools, err := agent.ListTools(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	available := map[string]bool{}
	for _, tool := range agentTools.Tools {
		available[tool.Name] = true
	}
	for _, name := range []string{
		"global_exec", "global_mcp_status", "global_mcp_tools", "global_mcp_call",
		"global_skill_list", "global_skill_files", "global_skill_read",
		"exec", "run_start", "process_start",
		"environment_worktree_create", "environment_worktree_destroy",
		"environment_temporary_promote", "environment_temporary_cleanup",
		"environment_verifier_agent_host_execution_set",
		"git_tag_delete", "secret_list", "strong_permission_control_status", "strong_permission_control_set",
		"environment_verifier_run", "environment_verifier_run_start",
		"git_status", "git_diff", "git_branch", "git_commit",
		"git_log", "git_show", "git_remote_list", "git_tag_list", "git_tag_create",
		"git_fetch", "git_push", "git_cherry_pick", "git_merge",
	} {
		if !available[name] {
			t.Fatalf("Agent MCP must keep permission-gated tool %q discoverable", name)
		}
	}

	status := callGatewayTool(t, ctx, agent, "strong_permission_control_status", map[string]any{})
	if status.IsError || !strings.Contains(toolText(t, status), `"enabled":true`) || !strings.Contains(toolText(t, status), `"mode":"strong"`) {
		t.Fatalf("unexpected default strong permission status: %s", toolText(t, status))
	}

	strongBlocked := callGatewayTool(t, ctx, agent, "global_skill_list", map[string]any{})
	strongText := toolText(t, strongBlocked)
	if !strongBlocked.IsError || !strings.Contains(strongText, "Strong Permission Control is enabled") || !strings.Contains(strongText, "v1.4.4-compatible") {
		t.Fatalf("strong-only permission denial is not actionable: %s", strongText)
	}

	adminBlocked := callGatewayTool(t, ctx, agent, "secret_list", map[string]any{})
	adminText := toolText(t, adminBlocked)
	if !adminBlocked.IsError || !strings.Contains(adminText, "requires the Admin surface") {
		t.Fatalf("Admin-only permission denial is not actionable: %s", adminText)
	}

	if _, err := service.SetStrongPermissionControl(false); err != nil {
		t.Fatal(err)
	}
	relaxed := callGatewayTool(t, ctx, agent, "global_skill_list", map[string]any{})
	if relaxed.IsError {
		t.Fatalf("v1.4.4-compatible mode must restore relaxed Agent capability: %s", toolText(t, relaxed))
	}
	stillAdmin := callGatewayTool(t, ctx, agent, "secret_list", map[string]any{})
	if !stillAdmin.IsError || !strings.Contains(toolText(t, stillAdmin), "requires the Admin surface") {
		t.Fatalf("v1.4.4-compatible mode must retain historical Admin boundary: %s", toolText(t, stillAdmin))
	}
	cannotToggle := callGatewayTool(t, ctx, agent, "strong_permission_control_set", map[string]any{"enabled": true})
	if !cannotToggle.IsError || !strings.Contains(toolText(t, cannotToggle), "requires the Admin surface") {
		t.Fatalf("Agent must not toggle its own permission mode: %s", toolText(t, cannotToggle))
	}

	admin := connectInMemory(t, ctx, NewAdmin(service))
	defer admin.Close()
	updated := callGatewayTool(t, ctx, admin, "strong_permission_control_set", map[string]any{"enabled": true})
	if updated.IsError || !strings.Contains(toolText(t, updated), `"enabled":true`) {
		t.Fatalf("Admin could not restore strong permission control: %s", toolText(t, updated))
	}
}

func TestAgentSurfaceRejectsStdioMCPHostExecution(t *testing.T) {
	service := app.New(filepath.Join(t.TempDir(), "state.json"))
	workspace, err := service.Workspaces.Add(t.TempDir(), "stdio-boundary")
	if err != nil {
		t.Fatal(err)
	}
	environment, err := service.Environments.Create(workspace.ID, "stdio-boundary", "")
	if err != nil {
		t.Fatal(err)
	}
	entry, err := service.MCPs.AddMCPConfig("stdio-boundary", catalog.MCPConfig{
		Transport:  catalog.MCPTransportStdio,
		Executable: os.Args[0],
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.SetEnvironmentMCP(environment.ID, entry.ID, true); err != nil {
		t.Fatal(err)
	}
	if err := service.AllowExecutable(os.Args[0]); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	agent := connectInMemory(t, ctx, New(service))
	defer agent.Close()

	for _, name := range []string{"environment_mcp_status", "environment_mcp_refresh", "environment_mcp_tools", "environment_mcp_call"} {
		arguments := map[string]any{
			"environment_id": environment.ID,
			"mcp_id":         entry.ID,
		}
		if name == "environment_mcp_call" {
			arguments["tool"] = "anything"
		}
		result := callGatewayTool(t, ctx, agent, name, arguments)
		text := toolText(t, result)
		if !result.IsError || !strings.Contains(text, "agent_stdio_mcp_blocked") {
			t.Fatalf("%s must reject stdio MCP host execution on Agent surface: %+v text=%s", name, result, text)
		}
	}
}
