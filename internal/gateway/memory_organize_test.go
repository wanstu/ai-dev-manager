package gateway

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"ai-dev-manager-v2/internal/app"
)

func TestAgentMemoryOrganizeSurface(t *testing.T) {
	service := app.New(filepath.Join(t.TempDir(), "state.json"))
	ws, err := service.Workspaces.Add(t.TempDir(), "memory-test")
	if err != nil {
		t.Fatal(err)
	}
	env, err := service.Environments.Create(ws.ID, "memory-test", "")
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	agent := connectInMemory(t, ctx, New(service))
	defer agent.Close()
	admin := connectInMemory(t, ctx, NewAdmin(service))
	defer admin.Close()
	changes := []map[string]any{{"key": "decision", "expected_value": nil, "value": "verified build"}}
	args := map[string]any{"environment_id": env.ID, "changes": changes}
	result := callGatewayTool(t, ctx, agent, "memory_environment_organize", args)
	if result.IsError {
		t.Fatal(toolText(t, result))
	}
	result = callGatewayTool(t, ctx, agent, "memory_environment_search", map[string]any{"environment_id": env.ID, "query": "build"})
	if result.IsError || !strings.Contains(toolText(t, result), "verified build") {
		t.Fatal(toolText(t, result))
	}
	for _, strong := range []bool{true, false} {
		if _, err := service.SetStrongPermissionControl(strong); err != nil {
			t.Fatal(err)
		}
		result = callGatewayTool(t, ctx, agent, "memory_global_organize", map[string]any{"changes": changes})
		if !result.IsError || !strings.Contains(toolText(t, result), "Admin surface") {
			t.Fatal(toolText(t, result))
		}
	}
	result = callGatewayTool(t, ctx, admin, "memory_global_organize", map[string]any{"changes": changes})
	if result.IsError {
		t.Fatal(toolText(t, result))
	}
	result = callGatewayTool(t, ctx, agent, "memory_global_search", map[string]any{"query": "build"})
	if result.IsError || !strings.Contains(toolText(t, result), "verified build") {
		t.Fatal(toolText(t, result))
	}
	compact := connectInMemory(t, ctx, newCompactAgentServer(service, nil))
	defer compact.Close()
	result = callGatewayTool(t, ctx, compact, "adm_memory", map[string]any{"action": "environment_search", "arguments": map[string]any{"environment_id": env.ID, "query": "build"}})
	if result.IsError || !strings.Contains(toolText(t, result), "verified build") {
		t.Fatal(toolText(t, result))
	}
}
