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
	if !contains(toolNames(agentTools.Tools), "environment_worktree_migrate_temporary") {
		t.Fatal("Agent must keep managed worktree migration tool discoverable")
	}
	forbidden := callGatewayTool(t, ctx, agent, "environment_worktree_migrate_temporary", map[string]any{"execute": true})
	if !forbidden.IsError || !strings.Contains(toolText(t, forbidden), "requires the Admin surface") {
		t.Fatalf("Agent migration denial must explain Admin requirement: %s", toolText(t, forbidden))
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
