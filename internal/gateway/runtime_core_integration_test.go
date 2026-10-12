package gateway

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"ai-dev-manager-v2/internal/app"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// An outer 2026-07-28 session must not force its protocol version onto
// ADM's separate client connection to an older downstream MCP.
func TestCoreOwnedMCPNestedProtocolIsolation(t *testing.T) {
	old := mcp.NewServer(&mcp.Implementation{Name: "legacy-db", Version: "1"}, nil)
	mcp.AddTool(old, &mcp.Tool{Name: "select_one"}, func(context.Context, *mcp.CallToolRequest, EmptyInput) (*mcp.CallToolResult, any, error) {
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: "one"}}}, nil, nil
	})
	handler := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return old }, &mcp.StreamableHTTPOptions{Stateless: false, JSONResponse: true})
	db := httptest.NewServer(handler)
	defer db.Close()
	svc := app.New(filepath.Join(t.TempDir(), "state.json"))
	ws, err := svc.Workspaces.Add(t.TempDir(), "test")
	if err != nil {
		t.Fatal(err)
	}
	env, err := svc.Environments.Create(ws.ID, "test", "")
	if err != nil {
		t.Fatal(err)
	}
	entry, err := svc.MCPs.AddMCP("legacy-db", db.URL, false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = svc.SetEnvironmentMCP(env.ID, entry.ID, true); err != nil {
		t.Fatal(err)
	}
	owner := newRuntimeOwner(svc)
	defer owner.Close()
	adm := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return newServer(svc, owner) }, &mcp.StreamableHTTPOptions{Stateless: true, JSONResponse: true})
	gateway := httptest.NewServer(adm)
	defer gateway.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	client := mcp.NewClient(&mcp.Implementation{Name: "modern-client", Version: "1"}, nil)
	session, err := client.Connect(ctx, &mcp.StreamableClientTransport{Endpoint: gateway.URL, DisableStandaloneSSE: true}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	if got := session.InitializeResult().ProtocolVersion; got != "2026-07-28" {
		t.Logf("outer negotiated %s", got)
	}
	result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "environment_mcp_status", Arguments: map[string]any{"environment_id": env.ID, "mcp_id": entry.ID}})
	if err != nil {
		t.Fatalf("status error %v", err)
	}
	var texts []string
	for _, part := range result.Content {
		if p, ok := part.(*mcp.TextContent); ok {
			texts = append(texts, p.Text)
		}
	}
	if result.IsError || !strings.Contains(strings.Join(texts, ""), `"state":"healthy"`) {
		t.Fatalf("health %v error=%v", texts, result.IsError)
	}
	tools, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "environment_mcp_tools", Arguments: map[string]any{"environment_id": env.ID, "mcp_id": entry.ID}})
	if err != nil || tools.IsError {
		t.Fatalf("tools err=%v response=%v", err, tools)
	}
	call, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "environment_mcp_call", Arguments: map[string]any{"environment_id": env.ID, "mcp_id": entry.ID, "tool": "select_one", "arguments": map[string]any{}}})
	if err != nil || call == nil || call.IsError {
		t.Fatalf("call error=%v response=%v", err, call)
	}
}
