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

func TestAgentSurfaceHidesHostEscapeTools(t *testing.T) {
	service := app.New(filepath.Join(t.TempDir(), "state.json"))
	ctx := context.Background()

	forbidden := map[string]bool{
		"global_exec":                   true,
		"global_mcp_status":             true,
		"global_mcp_tools":              true,
		"global_mcp_call":               true,
		"global_skill_list":             true,
		"global_skill_files":            true,
		"global_skill_read":             true,
		"exec":                          true,
		"run_start":                     true,
		"process_start":                 true,
		"environment_worktree_create":   true,
		"environment_worktree_destroy":  true,
		"environment_temporary_promote": true,
		"environment_temporary_cleanup": true,
		"git_fetch":                     true,
		"git_push":                      true,
		"git_tag_create":                true,
		"git_tag_delete":                true,
	}

	agent := connectInMemory(t, ctx, New(service))
	defer agent.Close()
	agentTools, err := agent.ListTools(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	available := map[string]bool{}
	for _, tool := range agentTools.Tools {
		available[tool.Name] = true
		if forbidden[tool.Name] {
			t.Fatalf("Agent MCP must not expose host-escape tool %q", tool.Name)
		}
	}
	for _, name := range []string{
		"environment_verifier_run", "environment_verifier_run_start",
		"git_log", "git_show", "git_remote_list", "git_tag_list",
		"git_cherry_pick", "git_cherry_pick_continue", "git_cherry_pick_abort",
		"git_merge", "git_merge_abort",
	} {
		if !available[name] {
			t.Fatalf("Agent MCP must retain approved verifier automation tool %q", name)
		}
	}

	admin := connectInMemory(t, ctx, NewAdmin(service))
	defer admin.Close()
	adminTools, err := admin.ListTools(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	found := make(map[string]bool, len(forbidden))
	for _, tool := range adminTools.Tools {
		if forbidden[tool.Name] {
			found[tool.Name] = true
		}
	}
	for name := range forbidden {
		if !found[name] {
			t.Fatalf("Admin MCP must retain management tool %q", name)
		}
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
