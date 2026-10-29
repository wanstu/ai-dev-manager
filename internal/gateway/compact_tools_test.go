package gateway

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ai-dev-manager-v2/internal/app"
)

func TestCompactAgentSurfaceStaysUnderToolLimitAndDispatchesLegacyActions(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("compact-surface\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	service := app.New(filepath.Join(t.TempDir(), "state.json"))
	workspace, err := service.Workspaces.Add(root, "compact")
	if err != nil {
		t.Fatal(err)
	}
	environment, err := service.Environments.Create(workspace.ID, "compact", "")
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	agent := connectInMemory(t, ctx, newCompactAgentServer(service, nil))
	defer agent.Close()

	listed, err := agent.ListTools(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(listed.Tools) > compactAgentToolLimit {
		t.Fatalf("compact Agent surface exposes %d tools; limit=%d", len(listed.Tools), compactAgentToolLimit)
	}
	available := map[string]bool{}
	for _, tool := range listed.Tools {
		available[tool.Name] = true
	}
	for _, name := range []string{"adm_help", "adm_file", "adm_git", "adm_environment", "adm_gateway"} {
		if !available[name] {
			t.Fatalf("compact Agent surface missing %q; tools=%v", name, toolNames(listed.Tools))
		}
	}
	for _, legacy := range []string{"git_status", "read", "environment_capability_report", "gateway_access_status"} {
		if available[legacy] {
			t.Fatalf("legacy tool %q leaked into compact Agent surface", legacy)
		}
	}

	help := callGatewayTool(t, ctx, agent, "adm_help", map[string]any{
		"group":  "git",
		"action": "status",
	})
	helpText := toolText(t, help)
	if help.IsError || !strings.Contains(helpText, `"legacy_name":"git_status"`) || !strings.Contains(helpText, "environment_id") {
		t.Fatalf("compact help did not preserve legacy schema metadata: %s", helpText)
	}

	execHelp := callGatewayTool(t, ctx, agent, "adm_help", map[string]any{
		"group":  "exec",
		"action": "run",
	})
	execHelpText := toolText(t, execHelp)
	if execHelp.IsError || !strings.Contains(execHelpText, "execution_mode") || !strings.Contains(execHelpText, "auto, host, or sandbox") {
		t.Fatalf("compact exec help lost explicit execution mode schema/description: %s", execHelpText)
	}
	runHelp := callGatewayTool(t, ctx, agent, "adm_help", map[string]any{
		"group":  "run",
		"action": "start",
	})
	runHelpText := toolText(t, runHelp)
	if runHelp.IsError || !strings.Contains(runHelpText, "execution_mode") || !strings.Contains(runHelpText, "auto, host, or sandbox") {
		t.Fatalf("compact run help lost explicit execution mode schema/description: %s", runHelpText)
	}

	environmentInspect := callGatewayTool(t, ctx, agent, "adm_environment", map[string]any{
		"action": "inspect",
		"arguments": map[string]any{
			"environment_id": environment.ID,
		},
	})
	if environmentInspect.IsError || !strings.Contains(toolText(t, environmentInspect), environment.ID) {
		t.Fatalf("adm_environment inspect failed: %s", toolText(t, environmentInspect))
	}

	read := callGatewayTool(t, ctx, agent, "adm_file", map[string]any{
		"action": "read",
		"arguments": map[string]any{
			"environment_id": environment.ID,
			"path":           "README.md",
		},
	})
	if read.IsError || !strings.Contains(toolText(t, read), "compact-surface") {
		t.Fatalf("adm_file read failed: %s", toolText(t, read))
	}

	adminOnly := callGatewayTool(t, ctx, agent, "adm_gateway", map[string]any{
		"action": "access_status",
	})
	if !adminOnly.IsError || !strings.Contains(toolText(t, adminOnly), "requires the Admin surface") {
		t.Fatalf("compact Admin-only permission boundary changed: %s", toolText(t, adminOnly))
	}

	strongOnly := callGatewayTool(t, ctx, agent, "adm_skill", map[string]any{
		"action": "global_list",
	})
	if !strongOnly.IsError || !strings.Contains(toolText(t, strongOnly), "Strong Permission Control is enabled") {
		t.Fatalf("compact Strong-only permission boundary changed: %s", toolText(t, strongOnly))
	}
}

func TestCompactAgentHelpListsGroupsWithoutLegacyTopLevelExplosion(t *testing.T) {
	service := app.New(filepath.Join(t.TempDir(), "state.json"))
	ctx := context.Background()
	agent := connectInMemory(t, ctx, newCompactAgentServer(service, nil))
	defer agent.Close()

	result := callGatewayTool(t, ctx, agent, "adm_help", map[string]any{})
	textValue := toolText(t, result)
	if result.IsError {
		t.Fatalf("adm_help failed: %s", textValue)
	}
	for _, expected := range []string{`"git"`, `"environment"`, `"file"`, `"gateway"`, `"tool_count"`} {
		if !strings.Contains(textValue, expected) {
			t.Fatalf("adm_help missing %s: %s", expected, textValue)
		}
	}
}
