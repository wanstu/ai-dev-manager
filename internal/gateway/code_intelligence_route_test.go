package gateway

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"ai-dev-manager-v2/internal/app"
	"ai-dev-manager-v2/internal/projectanalysis"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type codeIntelligenceRouteSession struct {
	mu        sync.Mutex
	callErr   error
	calls     []string
	arguments map[string]map[string]any
}

func (s *codeIntelligenceRouteSession) Ping(context.Context, *mcp.PingParams) error { return nil }

func (s *codeIntelligenceRouteSession) ListTools(context.Context, *mcp.ListToolsParams) (*mcp.ListToolsResult, error) {
	return &mcp.ListToolsResult{Tools: []*mcp.Tool{
		{Name: "code_intelligence_info"},
		{Name: "code_intelligence_query"},
		{Name: "code_intelligence_status"},
		{Name: "code_intelligence_references"},
		{Name: "code_intelligence_hierarchy"},
		{Name: "mutating_refactor"},
	}}, nil
}

func (s *codeIntelligenceRouteSession) CallTool(_ context.Context, params *mcp.CallToolParams) (*mcp.CallToolResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls = append(s.calls, params.Name)
	if s.arguments == nil {
		s.arguments = map[string]map[string]any{}
	}
	arguments, _ := params.Arguments.(map[string]any)
	s.arguments[params.Name] = arguments
	if s.callErr != nil {
		return nil, s.callErr
	}
	switch params.Name {
	case "code_intelligence_query":
		return &mcp.CallToolResult{StructuredContent: projectanalysis.IndexQueryResult{
			IndexPath: "provider://phpstorm",
			Matches: []projectanalysis.IndexQueryMatch{{
				Path: "src/Foo.php", Language: "PHP", Namespace: "App", Kind: "class",
				Name: "Foo", QualifiedName: "App\\Foo", Line: 17, Match: "provider",
			}},
			Returned: 1,
		}}, nil
	case "code_intelligence_status":
		return &mcp.CallToolResult{StructuredContent: projectanalysis.IndexStatusResult{
			State: "fresh", Fresh: true, Complete: true,
		}}, nil
	default:
		return &mcp.CallToolResult{IsError: true}, nil
	}
}

func (s *codeIntelligenceRouteSession) Close() error { return nil }

func TestCodeIntelligenceGenericToolsRouteToHealthyPhpStormProvider(t *testing.T) {
	ctx := context.Background()
	service := app.New(filepath.Join(t.TempDir(), "state.json"))
	root := t.TempDir()
	workspace, err := service.Workspaces.Add(root, "phpstorm-route")
	if err != nil {
		t.Fatal(err)
	}
	environment, err := service.Environments.Create(workspace.ID, "phpstorm-route", "")
	if err != nil {
		t.Fatal(err)
	}
	phpStorm, err := service.MCPs.AddMCP("PhpStorm Code Intelligence", "http://127.0.0.1:65532/mcp", false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.SetEnvironmentMCP(environment.ID, phpStorm.ID, true); err != nil {
		t.Fatal(err)
	}

	fake := &codeIntelligenceRouteSession{}
	owner := newRuntimeOwner(service)
	defer owner.Close()
	owner.connect = func(context.Context, string, string, map[string]string) (ownedMCPSession, error) {
		return fake, nil
	}
	status, err := owner.Status(ctx, environment.ID, phpStorm.ID)
	if err != nil || status.State != app.MCPHealthHealthy {
		t.Fatalf("prime PhpStorm provider: status=%+v err=%v", status, err)
	}

	session := connectInMemory(t, ctx, newServer(service, owner))
	defer session.Close()

	info := callGatewayTool(t, ctx, session, "code_intelligence_info", map[string]any{"environment_id": environment.ID})
	if info.IsError {
		t.Fatalf("code_intelligence_info failed: %s", toolText(t, info))
	}
	infoText := toolText(t, info)
	for _, want := range []string{"\"provider_id\":\"phpstorm\"", "\"definitions\":true", "\"references\":true", "\"hierarchy\":true", "\"requires_generated_index\":false"} {
		if !strings.Contains(infoText, want) {
			t.Fatalf("code_intelligence_info missing %q: %s", want, infoText)
		}
	}

	query := callGatewayTool(t, ctx, session, "code_intelligence_query", map[string]any{
		"environment_id": environment.ID,
		"query":          "Foo",
		"language":       "PHP",
		"max_results":    10,
	})
	if query.IsError {
		t.Fatalf("code_intelligence_query failed: %s", toolText(t, query))
	}
	queryText := toolText(t, query)
	for _, want := range []string{"\"provider_id\":\"phpstorm\"", "\"index_path\":\"provider://phpstorm\"", "\"qualified_name\":\"App\\\\Foo\"", "\"line\":17"} {
		if !strings.Contains(queryText, want) {
			t.Fatalf("code_intelligence_query missing %q: %s", want, queryText)
		}
	}

	providerStatus := callGatewayTool(t, ctx, session, "code_intelligence_status", map[string]any{
		"environment_id": environment.ID,
		"max_changes":    20,
	})
	if providerStatus.IsError {
		t.Fatalf("code_intelligence_status failed: %s", toolText(t, providerStatus))
	}
	statusText := toolText(t, providerStatus)
	for _, want := range []string{"\"provider_id\":\"phpstorm\"", "\"state\":\"fresh\"", "\"fresh\":true"} {
		if !strings.Contains(statusText, want) {
			t.Fatalf("code_intelligence_status missing %q: %s", want, statusText)
		}
	}

	fake.mu.Lock()
	queryArgs := fake.arguments["code_intelligence_query"]
	statusArgs := fake.arguments["code_intelligence_status"]
	fake.mu.Unlock()
	if queryArgs["project_root"] != root || statusArgs["project_root"] != root {
		t.Fatalf("provider project roots query=%v status=%v want=%q", queryArgs["project_root"], statusArgs["project_root"], root)
	}
	if queryArgs["environment_id"] != environment.ID {
		t.Fatalf("provider environment_id=%v want=%s", queryArgs["environment_id"], environment.ID)
	}
}

func TestCodeIntelligenceGenericQueryFallsBackToStaticIndex(t *testing.T) {
	ctx := context.Background()
	service := app.New(filepath.Join(t.TempDir(), "state.json"))
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/fallback\n\ngo 1.26\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "service.go"), []byte("package fallback\n\ntype Service struct{}\nfunc (s *Service) Run() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	workspace, err := service.Workspaces.Add(root, "fallback")
	if err != nil {
		t.Fatal(err)
	}
	environment, err := service.Environments.Create(workspace.ID, "fallback", "")
	if err != nil {
		t.Fatal(err)
	}
	const writer = "fallback-index"
	if _, err := service.Environments.AcquireWriter(environment.ID, writer); err != nil {
		t.Fatal(err)
	}
	if _, err := service.AnalyzeProject(environment.ID, writer, 100, 100); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Environments.ReleaseWriter(environment.ID, writer, false); err != nil {
		t.Fatal(err)
	}

	phpStorm, err := service.MCPs.AddMCP("PhpStorm Code Intelligence", "http://127.0.0.1:65531/mcp", false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.SetEnvironmentMCP(environment.ID, phpStorm.ID, true); err != nil {
		t.Fatal(err)
	}

	fake := &codeIntelligenceRouteSession{callErr: errors.New("provider temporarily unavailable")}
	owner := newRuntimeOwner(service)
	defer owner.Close()
	owner.connect = func(context.Context, string, string, map[string]string) (ownedMCPSession, error) {
		return fake, nil
	}
	status, err := owner.Status(ctx, environment.ID, phpStorm.ID)
	if err != nil || status.State != app.MCPHealthHealthy {
		t.Fatalf("prime PhpStorm provider: status=%+v err=%v", status, err)
	}

	session := connectInMemory(t, ctx, newServer(service, owner))
	defer session.Close()
	query := callGatewayTool(t, ctx, session, "code_intelligence_query", map[string]any{
		"environment_id": environment.ID,
		"query":          "Service.Run",
		"max_results":    10,
	})
	if query.IsError {
		t.Fatalf("fallback query failed: %s", toolText(t, query))
	}
	queryText := toolText(t, query)
	for _, want := range []string{
		"\"provider_id\":\"adm_static_index\"",
		"\"attempted_provider\":{\"provider_id\":\"phpstorm\"",
		"\"fallback_reason\":\"external_provider_call_failed\"",
		"\"qualified_name\":\"fallback.Service.Run\"",
	} {
		if !strings.Contains(queryText, want) {
			t.Fatalf("fallback query missing %q: %s", want, queryText)
		}
	}
}
