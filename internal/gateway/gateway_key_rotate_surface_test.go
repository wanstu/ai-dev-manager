package gateway

import (
	"context"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"ai-dev-manager-v2/internal/app"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestGatewayKeyRotateToolsAreAdminOnlyAndReturnOneTimeSecrets(t *testing.T) {
	service := app.New(filepath.Join(t.TempDir(), "state.json"))
	httpServer := httptest.NewServer(NewHTTPHandler(service))
	defer httpServer.Close()

	ctx := context.Background()
	connect := func(path, name string) *mcp.ClientSession {
		t.Helper()
		client := mcp.NewClient(&mcp.Implementation{Name: name, Version: "dev"}, nil)
		session, err := client.Connect(ctx, &mcp.StreamableClientTransport{Endpoint: httpServer.URL + path}, nil)
		if err != nil {
			t.Fatal(err)
		}
		return session
	}

	agent := connect("/mcp", "rotate-agent-surface-test")
	defer agent.Close()
	agentTools, err := agent.ListTools(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	agentNames := toolNames(agentTools.Tools)
	for _, name := range []string{"gateway_admin_api_key_rotate", "gateway_agent_api_key_rotate"} {
		if contains(agentNames, name) {
			t.Fatalf("admin-only rotate tool %q leaked into Agent MCP", name)
		}
	}

	admin := connect("/admin/mcp", "rotate-admin-surface-test")
	defer admin.Close()
	adminTools, err := admin.ListTools(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	adminNames := toolNames(adminTools.Tools)
	for _, name := range []string{"gateway_admin_api_key_rotate", "gateway_agent_api_key_rotate"} {
		if !contains(adminNames, name) {
			t.Fatalf("Admin MCP missing rotate tool %q", name)
		}
	}

	adminResult, err := admin.CallTool(ctx, &mcp.CallToolParams{Name: "gateway_admin_api_key_rotate", Arguments: map[string]any{}})
	if err != nil || adminResult.IsError {
		t.Fatalf("admin rotate failed: err=%v result=%+v", err, adminResult)
	}
	adminText := toolText(t, adminResult)
	if !strings.Contains(adminText, "admin_api_key") {
		t.Fatalf("admin rotate did not return one-time key: %s", adminText)
	}

	agentResult, err := admin.CallTool(ctx, &mcp.CallToolParams{Name: "gateway_agent_api_key_rotate", Arguments: map[string]any{}})
	if err != nil || agentResult.IsError {
		t.Fatalf("agent rotate failed: err=%v result=%+v", err, agentResult)
	}
	agentText := toolText(t, agentResult)
	if !strings.Contains(agentText, "agent_api_key") {
		t.Fatalf("agent rotate did not return one-time key: %s", agentText)
	}

	status, err := service.GatewayAccessStatus()
	if err != nil {
		t.Fatal(err)
	}
	if !status.AdminAPIKeyConfigured || !status.AgentAPIKeyConfigured {
		t.Fatalf("rotation did not configure both server-side hashes: %+v", status)
	}
}
