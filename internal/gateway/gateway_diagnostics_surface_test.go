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

func TestGatewayDiagnosticsToolIsAdminOnly(t *testing.T) {
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

	agent := connect("/mcp", "diagnostics-agent-surface-test")
	defer agent.Close()
	agentTools, err := agent.ListTools(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if contains(toolNames(agentTools.Tools), "gateway_diagnostics") {
		t.Fatal("gateway_diagnostics leaked into Agent MCP")
	}

	admin := connect("/admin/mcp", "diagnostics-admin-surface-test")
	defer admin.Close()
	adminTools, err := admin.ListTools(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !contains(toolNames(adminTools.Tools), "gateway_diagnostics") {
		t.Fatal("Admin MCP missing gateway_diagnostics")
	}

	result, err := admin.CallTool(ctx, &mcp.CallToolParams{Name: "gateway_diagnostics", Arguments: map[string]any{}})
	if err != nil || result.IsError {
		t.Fatalf("gateway diagnostics failed: err=%v result=%+v", err, result)
	}
	text := toolText(t, result)
	for _, required := range []string{"process_pid", "state_path", "readiness", "service"} {
		if !strings.Contains(text, required) {
			t.Fatalf("gateway diagnostics missing %q: %s", required, text)
		}
	}
	if strings.Contains(text, "admin_api_key\":") || strings.Contains(text, "agent_api_key\":") {
		t.Fatalf("gateway diagnostics exposed a plaintext key field: %s", text)
	}
}
