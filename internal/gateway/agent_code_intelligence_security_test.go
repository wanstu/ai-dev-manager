package gateway

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"ai-dev-manager-v2/internal/app"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestAgentCodeIntelligenceUsesStaticIndexWithoutExternalMCP(t *testing.T) {
	service := app.New(filepath.Join(t.TempDir(), "state.json"))
	workspace, err := service.Workspaces.Add(t.TempDir(), "agent-codeintel-boundary")
	if err != nil {
		t.Fatal(err)
	}
	external, err := service.MCPs.AddMCP("PhpStorm Code Intelligence", "http://127.0.0.1:65533/mcp", false)
	if err != nil {
		t.Fatal(err)
	}
	environment, err := service.Environments.Create(workspace.ID, "agent-codeintel-boundary", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.SetEnvironmentMCP(environment.ID, external.ID, true); err != nil {
		t.Fatal(err)
	}

	owner := newRuntimeOwner(service)
	defer owner.Close()
	observedAt := time.Now().UTC()
	owner.mu.Lock()
	owner.observations[runtimeOwnerKey{environmentID: environment.ID, mcpID: external.ID}] = app.MCPRuntimeObservation{
		EnvironmentID:  environment.ID,
		MCPID:          external.ID,
		DesiredEnabled: true,
		State:          app.MCPHealthHealthy,
		LastCheckAt:    &observedAt,
		LastHealthyAt:  &observedAt,
		LastSuccessAt:  &observedAt,
		ToolInventory: []app.MCPToolInventoryItem{
			{Name: "code_intelligence_info"},
			{Name: "code_intelligence_query"},
			{Name: "code_intelligence_status"},
			{Name: "code_intelligence_references"},
			{Name: "code_intelligence_hierarchy"},
		},
		InventoryFetchedAt: &observedAt,
	}
	owner.mu.Unlock()

	connects := 0
	owner.connect = func(context.Context, string, string, map[string]string) (ownedMCPSession, error) {
		connects++
		return nil, fmt.Errorf("external MCP connection attempted")
	}

	ctx := context.Background()
	agent := connectInMemory(t, ctx, newServerForSurface(service, owner, serverSurfaceAgent))
	defer agent.Close()
	result, err := agent.CallTool(ctx, &mcp.CallToolParams{
		Name:      "code_intelligence_info",
		Arguments: map[string]any{"environment_id": environment.ID},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.IsError {
		t.Fatalf("Agent static code intelligence failed: %s", toolText(t, result))
	}
	if connects != 0 {
		t.Fatalf("Agent code intelligence attempted %d external MCP connection(s)", connects)
	}

	admin := connectInMemory(t, ctx, newServerForSurface(service, owner, serverSurfaceAdmin))
	defer admin.Close()
	if _, err := admin.CallTool(ctx, &mcp.CallToolParams{
		Name:      "code_intelligence_info",
		Arguments: map[string]any{"environment_id": environment.ID},
	}); err != nil {
		t.Fatal(err)
	}
	if connects == 0 {
		t.Fatal("test setup did not exercise Admin external-provider negotiation")
	}
}
