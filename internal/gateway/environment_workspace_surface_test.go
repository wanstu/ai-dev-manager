package gateway

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ai-dev-manager-v2/internal/app"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestEnvironmentWorkspaceRebindIsAdminOnlyAndUsesMostSpecificContainingWorkspace(t *testing.T) {
	root := t.TempDir()
	broadRoot := filepath.Join(root, "work")
	specificRoot := filepath.Join(broadRoot, "wm_group")
	environmentRoot := filepath.Join(specificRoot, "wm_main")
	outsideRoot := filepath.Join(root, "outside")
	for _, dir := range []string{environmentRoot, outsideRoot} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}

	service := app.New(filepath.Join(t.TempDir(), "state.json"))
	broad, err := service.Workspaces.Add(broadRoot, "work")
	if err != nil {
		t.Fatal(err)
	}
	specific, err := service.Workspaces.Add(specificRoot, "wm_group")
	if err != nil {
		t.Fatal(err)
	}
	outside, err := service.Workspaces.Add(outsideRoot, "outside")
	if err != nil {
		t.Fatal(err)
	}
	env, err := service.Environments.Create(broad.ID, "wm_main", environmentRoot)
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	agent := connectInMemory(t, ctx, New(service))
	defer agent.Close()
	agentTools, err := agent.ListTools(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	agentNames := toolNames(agentTools.Tools)
	for _, adminOnly := range []string{"environment_workspace_options", "environment_workspace_recommendations", "environment_workspace_set"} {
		if contains(agentNames, adminOnly) {
			t.Fatalf("Admin-only Environment Workspace tool %q leaked into Agent surface: %v", adminOnly, agentNames)
		}
	}

	admin := connectInMemory(t, ctx, NewAdmin(service))
	defer admin.Close()
	adminTools, err := admin.ListTools(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	adminNames := toolNames(adminTools.Tools)
	for _, required := range []string{"environment_workspace_options", "environment_workspace_recommendations", "environment_workspace_set"} {
		if !contains(adminNames, required) {
			t.Fatalf("Admin surface missing Environment Workspace tool %q: %v", required, adminNames)
		}
	}

	recommendations, err := admin.CallTool(ctx, &mcp.CallToolParams{Name: "environment_workspace_recommendations", Arguments: map[string]any{}})
	if err != nil || recommendations.IsError {
		t.Fatalf("environment_workspace_recommendations failed: err=%v result=%+v", err, recommendations)
	}
	recommendationsText := toolText(t, recommendations)
	if !strings.Contains(recommendationsText, env.ID) || !strings.Contains(recommendationsText, specific.ID) || !strings.Contains(recommendationsText, `"root"`) {
		t.Fatalf("workspace recommendations do not expose the actionable drift: %s", recommendationsText)
	}

	options, err := admin.CallTool(ctx, &mcp.CallToolParams{
		Name:      "environment_workspace_options",
		Arguments: map[string]any{"environment_id": env.ID},
	})
	if err != nil || options.IsError {
		t.Fatalf("environment_workspace_options failed: err=%v result=%+v", err, options)
	}
	optionsText := toolText(t, options)
	if !strings.Contains(optionsText, specific.ID) || !strings.Contains(optionsText, `"recommended_workspace_id"`) {
		t.Fatalf("workspace options do not expose the specific recommendation: %s", optionsText)
	}

	rebound, err := admin.CallTool(ctx, &mcp.CallToolParams{
		Name: "environment_workspace_set",
		Arguments: map[string]any{
			"environment_id": env.ID,
			"workspace_id":   specific.ID,
		},
	})
	if err != nil || rebound.IsError {
		t.Fatalf("environment_workspace_set failed: err=%v result=%+v", err, rebound)
	}
	visible, err := service.Environments.Get(env.ID)
	if err != nil {
		t.Fatal(err)
	}
	if visible.WorkspaceID != specific.ID || visible.Root != env.Root {
		t.Fatalf("rebind changed unexpected Environment identity: %+v", visible)
	}
	recommendations, err = admin.CallTool(ctx, &mcp.CallToolParams{Name: "environment_workspace_recommendations", Arguments: map[string]any{}})
	if err != nil || recommendations.IsError {
		t.Fatalf("environment_workspace_recommendations after rebind failed: err=%v result=%+v", err, recommendations)
	}
	if text := toolText(t, recommendations); strings.Contains(text, env.ID) {
		t.Fatalf("rebound Environment must disappear from recommendations: %s", text)
	}

	invalid, err := admin.CallTool(ctx, &mcp.CallToolParams{
		Name: "environment_workspace_set",
		Arguments: map[string]any{
			"environment_id": env.ID,
			"workspace_id":   outside.ID,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if invalid == nil || !invalid.IsError {
		t.Fatalf("outside Workspace must be rejected, result=%+v", invalid)
	}
}
