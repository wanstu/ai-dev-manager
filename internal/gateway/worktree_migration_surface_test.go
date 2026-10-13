package gateway

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"ai-dev-manager-v2/internal/app"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestManagedWorktreeMigrationIsAdminOnly(t *testing.T) {
	service := app.New(filepath.Join(t.TempDir(), "adm", "state.json"))
	ctx := context.Background()
	agent := connectInMemory(t, ctx, New(service))
	defer agent.Close()
	agentTools, err := agent.ListTools(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if contains(toolNames(agentTools.Tools), "environment_worktree_migrate_temporary") {
		t.Fatal("Agent must not be able to migrate managed worktree retention")
	}
	forbidden, forbiddenErr := agent.CallTool(ctx, &mcp.CallToolParams{Name: "environment_worktree_migrate_temporary", Arguments: map[string]any{"execute": true}})
	if forbiddenErr == nil && forbidden != nil && !forbidden.IsError {
		t.Fatalf("Agent migration unexpectedly succeeded: %+v", forbidden)
	}
	admin := connectInMemory(t, ctx, NewAdmin(service))
	defer admin.Close()
	adminTools, err := admin.ListTools(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !contains(toolNames(adminTools.Tools), "environment_worktree_migrate_temporary") {
		t.Fatal("Admin migration tool is missing")
	}
	result := callGatewayTool(t, ctx, admin, "environment_worktree_migrate_temporary", map[string]any{"execute": false})
	if result.IsError || !strings.Contains(toolText(t, result), "\"dry_run\":true") {
		t.Fatalf("Admin migration preview failed: %s", toolText(t, result))
	}
}
