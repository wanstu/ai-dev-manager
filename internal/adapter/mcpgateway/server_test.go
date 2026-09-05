package mcpgateway

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"
	"time"

	"ai-dev-manager/internal/environment"
	"ai-dev-manager/internal/gateway"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type fakeDiscovery struct {
	inspectErr error
	createErr  error
	destroyErr error
	acquireErr error
	releaseErr error
}

func (fakeDiscovery) Info() gateway.Info {
	return gateway.Info{Name: gateway.ProductName, Role: gateway.ProductRole, APIVersion: gateway.APIVersion}
}

func (fakeDiscovery) Workspaces() ([]gateway.WorkspaceSummary, error) {
	return []gateway.WorkspaceSummary{{WorkspaceID: "ws_a", Path: `D:\repo`}}, nil
}

func (fakeDiscovery) Environments() ([]environment.Environment, error) {
	return []environment.Environment{{ID: "env_a", WorkspaceID: "ws_a", Name: "task", State: environment.StateReady}}, nil
}

func (f fakeDiscovery) InspectEnvironment(_ context.Context, id string) (environment.InspectResult, error) {
	if f.inspectErr != nil {
		return environment.InspectResult{}, f.inspectErr
	}
	return environment.InspectResult{
		Environment: environment.Environment{ID: id, WorkspaceID: "ws_a", State: environment.StateReady, LastActivityAt: time.Date(2026, 9, 5, 1, 2, 3, 0, time.UTC)},
		Facts:       []environment.Fact{{Code: "dirty", Message: "dirty fact", Value: false}},
		Hints:       []environment.Hint{{Code: "soft_hint", Message: "consider checking"}},
	}, nil
}

func (f fakeDiscovery) CreateEnvironment(_ context.Context, request environment.CreateRequest) (environment.InspectResult, error) {
	result := environment.InspectResult{Environment: environment.Environment{ID: "env_created", WorkspaceID: request.WorkspaceID, Name: request.Name, State: environment.StateReady}}
	return result, f.createErr
}

func (f fakeDiscovery) DestroyEnvironment(_ context.Context, id string, _ bool) (environment.Environment, error) {
	return environment.Environment{ID: id, WorkspaceID: "ws_a", State: environment.StateReady}, f.destroyErr
}

func (f fakeDiscovery) AcquireEnvironmentWriter(_ context.Context, id, owner string) (environment.Environment, error) {
	return environment.Environment{ID: id, WorkspaceID: "ws_a", State: environment.StateReady, Writer: &environment.WriterLease{Owner: owner}}, f.acquireErr
}

func (f fakeDiscovery) ReleaseEnvironmentWriter(id, _ string, _ bool) (environment.Environment, error) {
	return environment.Environment{ID: id, WorkspaceID: "ws_a", State: environment.StateReady}, f.releaseErr
}

func (f fakeDiscovery) InvokeEnvironmentRead(_ context.Context, id, operation string, input map[string]any) (any, error) {
	return map[string]any{"environment_id": id, "operation": operation, "input": input}, nil
}

func (f fakeDiscovery) InvokeEnvironmentMutation(_ context.Context, id, owner, operation string, input map[string]any) (any, error) {
	return map[string]any{"environment_id": id, "writer_owner": owner, "operation": operation, "input": input}, nil
}

func (f fakeDiscovery) DomainError(_ context.Context, err error, env *environment.Environment, inspection *environment.InspectResult) gateway.DomainError {
	result := gateway.DomainError{Code: "gateway_operation_failed", Message: "Gateway operation failed."}
	if envErr, ok := err.(*environment.Error); ok {
		result.Code = string(envErr.Code)
		result.Message = envErr.Message
		result.EnvironmentID = envErr.EnvironmentID
	}
	if env != nil && result.EnvironmentID == "" {
		result.EnvironmentID = env.ID
	}
	if inspection != nil {
		result.Facts = append([]environment.Fact(nil), inspection.Facts...)
		result.Warnings = append([]environment.Warning(nil), inspection.Warnings...)
		result.Hints = append([]environment.Hint(nil), inspection.Hints...)
	}
	return result
}

func TestGatewayToolSurfaceAndDiscoveryCalls(t *testing.T) {
	ctx := context.Background()
	session := connectGatewayInMemory(t, ctx, New(fakeDiscovery{}))
	defer session.Close()

	tools, err := session.ListTools(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	names := make([]string, 0, len(tools.Tools))
	for _, tool := range tools.Tools {
		names = append(names, tool.Name)
	}
	sort.Strings(names)
	want := []string{"delete", "edit", "environment_create", "environment_destroy", "environment_inspect", "environment_list", "environment_writer_acquire", "environment_writer_release", "exec", "gateway_info", "git_branch", "git_diff", "git_status", "read", "run_verifier", "run_verifiers", "search", "tree", "workspace_list", "write"}
	if strings.Join(names, ",") != strings.Join(want, ",") {
		t.Fatalf("gateway tools = %+v, want %+v", names, want)
	}
	encodedTools, err := json.Marshal(tools.Tools)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encodedTools), `"result":true`) {
		t.Fatalf("gateway tools contain MCPHub-incompatible boolean result schema: %s", encodedTools)
	}

	for _, call := range []mcp.CallToolParams{
		{Name: "gateway_info", Arguments: map[string]any{}},
		{Name: "workspace_list", Arguments: map[string]any{}},
		{Name: "environment_list", Arguments: map[string]any{}},
		{Name: "environment_inspect", Arguments: map[string]any{"environment_id": "env_a"}},
		{Name: "environment_create", Arguments: map[string]any{"workspace_id": "ws_a", "name": "created"}},
		{Name: "environment_writer_acquire", Arguments: map[string]any{"environment_id": "env_a", "owner": "owner-a"}},
		{Name: "environment_writer_release", Arguments: map[string]any{"environment_id": "env_a", "owner": "owner-a"}},
		{Name: "environment_destroy", Arguments: map[string]any{"environment_id": "env_a", "force": true}},
		{Name: "tree", Arguments: map[string]any{"environment_id": "env_a", "max_depth": 1}},
		{Name: "read", Arguments: map[string]any{"environment_id": "env_a", "path": "a.txt"}},
		{Name: "search", Arguments: map[string]any{"environment_id": "env_a", "query": "needle"}},
		{Name: "git_status", Arguments: map[string]any{"environment_id": "env_a"}},
		{Name: "git_diff", Arguments: map[string]any{"environment_id": "env_a"}},
		{Name: "git_branch", Arguments: map[string]any{"environment_id": "env_a"}},
		{Name: "write", Arguments: map[string]any{"environment_id": "env_a", "writer_owner": "owner-a", "path": "a.txt", "content": "x"}},
		{Name: "edit", Arguments: map[string]any{"environment_id": "env_a", "writer_owner": "owner-a", "path": "a.txt", "old_text": "x", "new_text": "y"}},
		{Name: "delete", Arguments: map[string]any{"environment_id": "env_a", "writer_owner": "owner-a", "path": "a.txt"}},
		{Name: "exec", Arguments: map[string]any{"environment_id": "env_a", "writer_owner": "owner-a", "executable": "git", "args": []any{"status"}}},
		{Name: "run_verifier", Arguments: map[string]any{"environment_id": "env_a", "writer_owner": "owner-a", "id": "test"}},
		{Name: "run_verifiers", Arguments: map[string]any{"environment_id": "env_a", "writer_owner": "owner-a"}},
	} {
		result, err := session.CallTool(ctx, &call)
		if err != nil || result.IsError {
			t.Fatalf("CallTool(%s) err=%v result=%+v", call.Name, err, result)
		}
		if result.StructuredContent == nil {
			t.Fatalf("CallTool(%s) missing structured content", call.Name)
		}
	}
}

func TestExposedGatewayHandlerRestrictsHostAndOrigin(t *testing.T) {
	handler := NewExposedHTTPHandler(fakeDiscovery{}, ExposedHTTPOptions{AllowedHosts: []string{"host.docker.internal"}})

	for _, tc := range []struct {
		name       string
		host       string
		origin     string
		wantStatus int
	}{
		{name: "bad host", host: "evil.example:41137", wantStatus: http.StatusForbidden},
		{name: "bad origin", host: "host.docker.internal:41137", origin: "https://evil.example", wantStatus: http.StatusForbidden},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "http://host.docker.internal:41137/mcp", strings.NewReader(`{}`))
			req.Host = tc.host
			if tc.origin != "" {
				req.Header.Set("Origin", tc.origin)
			}
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, req)
			if recorder.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d body=%s", recorder.Code, tc.wantStatus, recorder.Body.String())
			}
		})
	}

	req := httptest.NewRequest(http.MethodPost, "http://host.docker.internal:41137/mcp", strings.NewReader(`{}`))
	req.Host = "host.docker.internal:41137"
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)
	if recorder.Code == http.StatusForbidden {
		t.Fatalf("allowed Docker Host was rejected: %s", recorder.Body.String())
	}
}

func TestGatewayEnvironmentErrorKeepsStableCode(t *testing.T) {
	ctx := context.Background()
	session := connectGatewayInMemory(t, ctx, New(fakeDiscovery{inspectErr: &environment.Error{
		Code:          environment.ErrNotFound,
		EnvironmentID: "env_missing",
		Message:       "environment not found",
	}}))
	defer session.Close()

	result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "environment_inspect", Arguments: map[string]any{"environment_id": "env_missing"}})
	if err != nil {
		t.Fatalf("CallTool returned protocol error: %v", err)
	}
	if !result.IsError || len(result.Content) == 0 {
		t.Fatalf("expected tool error, got %+v", result)
	}
	structured, ok := result.StructuredContent.(map[string]any)
	if !ok {
		t.Fatalf("tool error structuredContent type = %T value=%#v", result.StructuredContent, result.StructuredContent)
	}
	errorValue, ok := structured["error"].(map[string]any)
	if !ok || errorValue["code"] != "environment_not_found" || structured["ok"] != false {
		t.Fatalf("unexpected structured domain error: %#v", structured)
	}
	text, ok := result.Content[0].(*mcp.TextContent)
	if !ok || !strings.Contains(text.Text, `"code":"environment_not_found"`) || strings.Contains(text.Text, "wrapped") {
		t.Fatalf("unexpected error content: %#v", result.Content)
	}
}

func connectGatewayInMemory(t *testing.T, ctx context.Context, server *mcp.Server) *mcp.ClientSession {
	t.Helper()
	clientTransport, serverTransport := mcp.NewInMemoryTransports()
	serverSession, err := server.Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatalf("server.Connect() error = %v", err)
	}
	t.Cleanup(func() { _ = serverSession.Close() })
	client := mcp.NewClient(&mcp.Implementation{Name: "adm-gateway-test", Version: "v0.6.0"}, nil)
	clientSession, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("client.Connect() error = %v", err)
	}
	return clientSession
}
