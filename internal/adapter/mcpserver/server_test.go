package mcpserver

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"ai-dev-manager/internal/adapter/runtimeadapter"
	"ai-dev-manager/internal/model"
	admruntime "ai-dev-manager/internal/runtime"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestInMemoryMCPReadOnlyToolSurfaceAndRead(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("workspace-a"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	native, err := admruntime.NewNative(model.Workspace{ID: "ws_a", Path: root}, model.EffectiveConfig{})
	if err != nil {
		t.Fatalf("NewNative() error = %v", err)
	}
	adapter, err := runtimeadapter.NewNative("native-a", native)
	if err != nil {
		t.Fatalf("NewNativeAdapter() error = %v", err)
	}

	ctx := context.Background()
	session := connectInMemory(t, ctx, New(adapter))
	defer session.Close()

	tools, err := session.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("ListTools() error = %v", err)
	}
	gotNames := toolNames(tools.Tools)
	wantNames := []string{"read", "runtime_info", "search", "tree"}
	if strings.Join(gotNames, ",") != strings.Join(wantNames, ",") {
		t.Fatalf("read-only tools = %+v, want %+v", gotNames, wantNames)
	}

	result, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "read",
		Arguments: map[string]any{"path": "a.txt"},
	})
	if err != nil {
		t.Fatalf("CallTool(read) error = %v", err)
	}
	if result.IsError {
		t.Fatalf("CallTool(read) returned tool error: %+v", result.Content)
	}
	text := toolText(t, result)
	if !strings.Contains(text, "workspace-a") || !strings.Contains(text, `"Path":"a.txt"`) {
		t.Fatalf("unexpected read tool result: %s", text)
	}

	if structured, ok := result.StructuredContent.(map[string]any); !ok || structured["result"] == nil {
		t.Fatalf("read structuredContent = %#v, want object with result", result.StructuredContent)
	}

	treeResult, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "tree", Arguments: map[string]any{"max_depth": 1, "max_entries": 10}})
	if err != nil || treeResult.IsError {
		t.Fatalf("tree failed: err=%v result=%+v", err, treeResult)
	}
	treeStructured, ok := treeResult.StructuredContent.(map[string]any)
	if !ok {
		t.Fatalf("tree structuredContent type = %T, want map[string]any", treeResult.StructuredContent)
	}
	if _, ok := treeStructured["result"].([]any); !ok {
		t.Fatalf("tree structuredContent = %#v, want result array inside object", treeStructured)
	}

	info, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "runtime_info", Arguments: map[string]any{}})
	if err != nil || info.IsError {
		t.Fatalf("runtime_info failed: err=%v result=%+v", err, info)
	}
	infoText := toolText(t, info)
	if !strings.Contains(infoText, `"id":"native-a"`) || !strings.Contains(infoText, `"workspace_id":"ws_a"`) {
		t.Fatalf("unexpected runtime_info: %s", infoText)
	}
}

func TestExposedHTTPHandlerAllowsDockerHostAndRejectsUnexpectedHostOrOrigin(t *testing.T) {
	root := t.TempDir()
	native, err := admruntime.NewNative(model.Workspace{ID: "ws_docker", Path: root}, model.EffectiveConfig{})
	if err != nil {
		t.Fatalf("NewNative() error = %v", err)
	}
	adapter, err := runtimeadapter.NewNative("native-docker", native)
	if err != nil {
		t.Fatalf("NewNativeAdapter() error = %v", err)
	}
	handler := NewExposedHTTPHandler(adapter, ExposedHTTPOptions{AllowedHosts: []string{"host.docker.internal"}, AllowIPLiteral: true})

	allowed := httptest.NewRequest(http.MethodGet, "http://host.docker.internal:31857/mcp", nil)
	allowed.Host = "host.docker.internal:31857"
	allowedRecorder := httptest.NewRecorder()
	handler.ServeHTTP(allowedRecorder, allowed)
	if allowedRecorder.Code == http.StatusForbidden {
		t.Fatalf("Docker Host was rejected: status=%d body=%s", allowedRecorder.Code, allowedRecorder.Body.String())
	}

	badHost := httptest.NewRequest(http.MethodGet, "http://evil.example:31857/mcp", nil)
	badHost.Host = "evil.example:31857"
	badHostRecorder := httptest.NewRecorder()
	handler.ServeHTTP(badHostRecorder, badHost)
	if badHostRecorder.Code != http.StatusForbidden {
		t.Fatalf("unexpected Host status=%d, want 403", badHostRecorder.Code)
	}

	badOrigin := httptest.NewRequest(http.MethodGet, "http://host.docker.internal:31857/mcp", nil)
	badOrigin.Host = "host.docker.internal:31857"
	badOrigin.Header.Set("Origin", "https://evil.example")
	badOriginRecorder := httptest.NewRecorder()
	handler.ServeHTTP(badOriginRecorder, badOrigin)
	if badOriginRecorder.Code != http.StatusForbidden {
		t.Fatalf("unexpected Origin status=%d, want 403", badOriginRecorder.Code)
	}
}

func TestInMemoryMCPWorkspaceWriteCanEditButDoesNotExposeExec(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("before"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	native, err := admruntime.NewNative(model.Workspace{ID: "ws_write", Path: root}, model.EffectiveConfig{
		Policy: &model.ResolvedPolicy{Policy: model.Policy{Mode: string(admruntime.ModeWorkspaceWrite)}, Source: model.ScopeProject},
	})
	if err != nil {
		t.Fatalf("NewNative() error = %v", err)
	}
	adapter, err := runtimeadapter.NewNative("native-write", native)
	if err != nil {
		t.Fatalf("NewNativeAdapter() error = %v", err)
	}

	ctx := context.Background()
	session := connectInMemory(t, ctx, New(adapter))
	defer session.Close()

	tools, err := session.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("ListTools() error = %v", err)
	}
	names := toolNames(tools.Tools)
	if !contains(names, "edit") || !contains(names, "write") || contains(names, "exec") || contains(names, "git_status") {
		t.Fatalf("unexpected workspace-write tool surface: %+v", names)
	}

	result, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name: "edit",
		Arguments: map[string]any{
			"path":                  "a.txt",
			"old_text":              "before",
			"new_text":              "after",
			"expected_replacements": 1,
		},
	})
	if err != nil || result.IsError {
		t.Fatalf("CallTool(edit) failed: err=%v result=%+v", err, result)
	}
	data, err := os.ReadFile(filepath.Join(root, "a.txt"))
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if string(data) != "after" {
		t.Fatalf("edit tool wrote %q, want after", data)
	}
}

func connectInMemory(t *testing.T, ctx context.Context, server *mcp.Server) *mcp.ClientSession {
	t.Helper()
	clientTransport, serverTransport := mcp.NewInMemoryTransports()
	serverSession, err := server.Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatalf("server.Connect() error = %v", err)
	}
	t.Cleanup(func() { _ = serverSession.Close() })

	client := mcp.NewClient(&mcp.Implementation{Name: "adm-test-client", Version: "v0.1.0"}, nil)
	clientSession, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("client.Connect() error = %v", err)
	}
	return clientSession
}

func toolNames(tools []*mcp.Tool) []string {
	names := make([]string, 0, len(tools))
	for _, tool := range tools {
		names = append(names, tool.Name)
	}
	sort.Strings(names)
	return names
}

func toolText(t *testing.T, result *mcp.CallToolResult) string {
	t.Helper()
	if len(result.Content) == 0 {
		t.Fatal("tool result has no content")
	}
	text, ok := result.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("tool result content type = %T, want *mcp.TextContent", result.Content[0])
	}
	return text.Text
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
