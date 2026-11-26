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

func TestAgentMemoryHistoryRestoreAndDuplicateTools(t *testing.T) {
	service := app.New(filepath.Join(t.TempDir(), "state.json"))
	ws, err := service.Workspaces.Add(t.TempDir(), "history-test")
	if err != nil {
		t.Fatal(err)
	}
	env, err := service.Environments.Create(ws.ID, "history-test", "")
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	agent := connectInMemory(t, ctx, newCompactAgentServer(service, nil))
	defer agent.Close()
	call := func(action string, args map[string]any) string {
		t.Helper()
		r := callGatewayTool(t, ctx, agent, "adm_memory", map[string]any{"action": action, "arguments": args})
		if r.IsError {
			t.Fatal(toolText(t, r))
		}
		return toolText(t, r)
	}
	call("environment_write", map[string]any{"environment_id": env.ID, "key": "a", "value": "old"})
	call("environment_write", map[string]any{"environment_id": env.ID, "key": "a", "value": "new"})
	if h := call("environment_history", map[string]any{"environment_id": env.ID, "key": "a", "revision": 1}); !strings.Contains(h, "old") {
		t.Fatal(h)
	}
	call("environment_restore", map[string]any{"environment_id": env.ID, "key": "a", "revision": 1, "expected_revision": 2})
	call("environment_write", map[string]any{"environment_id": env.ID, "key": "b", "value": "old"})
	if d := call("environment_duplicates", map[string]any{"environment_id": env.ID}); !strings.Contains(d, "total_groups") {
		t.Fatal(d)
	}
	denied := callGatewayTool(t, ctx, agent, "adm_memory", map[string]any{"action": "global_restore", "arguments": map[string]any{"key": "a", "revision": 1, "expected_revision": 2}})
	if !denied.IsError || !strings.Contains(toolText(t, denied), "Admin surface") {
		t.Fatal(toolText(t, denied))
	}
}

func TestAgentMemoryTrashAndPurgePermissions(t *testing.T) {
	service := app.New(filepath.Join(t.TempDir(), "state.json"))
	ws, err := service.Workspaces.Add(t.TempDir(), "trash")
	if err != nil {
		t.Fatal(err)
	}
	env, err := service.Environments.Create(ws.ID, "trash", "")
	if err != nil {
		t.Fatal(err)
	}
	if err := service.Memory.EnvironmentWrite(env.ID, "note", "content"); err != nil {
		t.Fatal(err)
	}
	if err := service.Memory.EnvironmentDelete(env.ID, "note"); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	agent := connectInMemory(t, ctx, newCompactAgentServer(service, nil))
	defer agent.Close()
	args := map[string]any{"environment_id": env.ID, "key": "note", "expected_revision": 2}
	r := callGatewayTool(t, ctx, agent, "adm_memory", map[string]any{"action": "environment_trash", "arguments": map[string]any{"environment_id": env.ID}})
	if r.IsError || !strings.Contains(toolText(t, r), "note") {
		t.Fatal(toolText(t, r))
	}
	r = callGatewayTool(t, ctx, agent, "adm_memory", map[string]any{"action": "environment_purge", "arguments": args})
	if !r.IsError || !strings.Contains(toolText(t, r), "Admin surface") {
		t.Fatal(toolText(t, r))
	}
	admin := connectInMemory(t, ctx, NewAdmin(service))
	defer admin.Close()
	r = callGatewayTool(t, ctx, admin, "memory_environment_purge", args)
	if r.IsError {
		t.Fatal(toolText(t, r))
	}
	if _, err := service.Memory.History(env.ID, false, "note", nil); err == nil {
		t.Fatal("history retained after purge")
	}
}
