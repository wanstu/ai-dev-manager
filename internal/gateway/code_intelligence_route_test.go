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
	"ai-dev-manager-v2/internal/codeintel"
	"ai-dev-manager-v2/internal/projectanalysis"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type codeIntelligenceRouteSession struct {
	mu              sync.Mutex
	callErr         error
	callErrByTool   map[string]error
	protocolVersion int
	calls           []string
	arguments       map[string]map[string]any
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
	if s.callErrByTool != nil {
		if err := s.callErrByTool[params.Name]; err != nil {
			return nil, err
		}
	}
	if s.callErr != nil {
		return nil, s.callErr
	}
	switch params.Name {
	case "code_intelligence_info":
		protocolVersion := s.protocolVersion
		if protocolVersion == 0 {
			protocolVersion = codeintel.ContractProtocolVersion
		}
		return &mcp.CallToolResult{StructuredContent: codeintel.ProviderInfo{
			Contract:               codeintel.ContractName,
			ProtocolVersion:        protocolVersion,
			ID:                     "phpstorm",
			Name:                   "PhpStorm Test Provider",
			ProviderVersion:        "0.1.0-test",
			Source:                 "phpstorm.psi",
			RequiresGeneratedIndex: false,
			Capabilities: codeintel.Capabilities{
				Definitions: true,
				References:  true,
				Hierarchy:   true,
			},
		}}, nil
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
	case "code_intelligence_references":
		return &mcp.CallToolResult{StructuredContent: codeintel.ReferencesResult{
			References: []codeintel.Reference{{
				Path: "src/Bar.php", Line: 31, Column: 9, Language: "PHP",
				Kind: "call", Name: "Foo", QualifiedName: "App\\Foo", Context: "$foo = new Foo();",
			}},
			Returned: 1,
		}}, nil
	case "code_intelligence_hierarchy":
		return &mcp.CallToolResult{StructuredContent: codeintel.HierarchyResult{
			Direction: "both",
			Nodes: []codeintel.HierarchyNode{
				{ID: "foo", Path: "src/Foo.php", Line: 17, Language: "PHP", Kind: "class", Name: "Foo", QualifiedName: "App\\Foo"},
				{ID: "base", Path: "src/Base.php", Line: 8, Language: "PHP", Kind: "class", Name: "Base", QualifiedName: "App\\Base"},
			},
			Edges:    []codeintel.HierarchyEdge{{From: "base", To: "foo", Kind: "extends"}},
			Returned: 2,
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
	for _, want := range []string{"\"contract\":\"adm.code_intelligence\"", "\"protocol_version\":1", "\"provider_id\":\"phpstorm\"", "\"provider_version\":\"0.1.0-test\"", "\"definitions\":true", "\"references\":true", "\"hierarchy\":true", "\"requires_generated_index\":false"} {
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

	references := callGatewayTool(t, ctx, session, "code_intelligence_references", map[string]any{
		"environment_id": environment.ID,
		"symbol": map[string]any{
			"path":           "src/Foo.php",
			"line":           17,
			"name":           "Foo",
			"qualified_name": "App\\Foo",
			"kind":           "class",
			"language":       "PHP",
		},
		"max_results": 25,
	})
	if references.IsError {
		t.Fatalf("code_intelligence_references failed: %s", toolText(t, references))
	}
	referencesText := toolText(t, references)
	for _, want := range []string{"\"provider_id\":\"phpstorm\"", "\"available\":true", "\"path\":\"src/Bar.php\"", "\"line\":31", "\"context\":\"$foo = new Foo();\""} {
		if !strings.Contains(referencesText, want) {
			t.Fatalf("code_intelligence_references missing %q: %s", want, referencesText)
		}
	}

	hierarchy := callGatewayTool(t, ctx, session, "code_intelligence_hierarchy", map[string]any{
		"environment_id": environment.ID,
		"symbol": map[string]any{
			"path":           "src/Foo.php",
			"line":           17,
			"qualified_name": "App\\Foo",
		},
		"direction":   "both",
		"max_depth":   3,
		"max_results": 30,
	})
	if hierarchy.IsError {
		t.Fatalf("code_intelligence_hierarchy failed: %s", toolText(t, hierarchy))
	}
	hierarchyText := toolText(t, hierarchy)
	for _, want := range []string{"\"provider_id\":\"phpstorm\"", "\"available\":true", "\"qualified_name\":\"App\\\\Base\"", "\"kind\":\"extends\"", "\"returned\":2"} {
		if !strings.Contains(hierarchyText, want) {
			t.Fatalf("code_intelligence_hierarchy missing %q: %s", want, hierarchyText)
		}
	}

	fake.mu.Lock()
	infoArgs := fake.arguments["code_intelligence_info"]
	queryArgs := fake.arguments["code_intelligence_query"]
	statusArgs := fake.arguments["code_intelligence_status"]
	fake.mu.Unlock()
	if infoArgs["contract"] != codeintel.ContractName || infoArgs["protocol_version"] != codeintel.ContractProtocolVersion {
		t.Fatalf("provider negotiation args=%+v", infoArgs)
	}
	if queryArgs["contract"] != codeintel.ContractName || queryArgs["protocol_version"] != codeintel.ContractProtocolVersion {
		t.Fatalf("provider query contract args=%+v", queryArgs)
	}
	if !sameProjectRoot(queryArgs["project_root"], root) || !sameProjectRoot(statusArgs["project_root"], root) {
		t.Fatalf("provider project roots query=%v status=%v want=%q", queryArgs["project_root"], statusArgs["project_root"], root)
	}
	if queryArgs["environment_id"] != environment.ID {
		t.Fatalf("provider environment_id=%v want=%s", queryArgs["environment_id"], environment.ID)
	}
}

func TestCodeIntelligenceRelationsReportUnavailableWithoutCapableProvider(t *testing.T) {
	ctx := context.Background()
	service := app.New(filepath.Join(t.TempDir(), "state.json"))
	root := t.TempDir()
	workspace, err := service.Workspaces.Add(root, "static-relations")
	if err != nil {
		t.Fatal(err)
	}
	environment, err := service.Environments.Create(workspace.ID, "static-relations", "")
	if err != nil {
		t.Fatal(err)
	}
	owner := newRuntimeOwner(service)
	defer owner.Close()
	session := connectInMemory(t, ctx, newServer(service, owner))
	defer session.Close()

	for _, call := range []struct {
		name string
		args map[string]any
	}{
		{
			name: "code_intelligence_references",
			args: map[string]any{
				"environment_id": environment.ID,
				"symbol":         map[string]any{"path": "service.go", "name": "Run"},
			},
		},
		{
			name: "code_intelligence_hierarchy",
			args: map[string]any{
				"environment_id": environment.ID,
				"symbol":         map[string]any{"path": "service.go", "name": "Service"},
			},
		},
	} {
		result := callGatewayTool(t, ctx, session, call.name, call.args)
		if result.IsError {
			t.Fatalf("%s failed: %s", call.name, toolText(t, result))
		}
		got := toolText(t, result)
		for _, want := range []string{"\"provider_id\":\"adm_static_index\"", "\"available\":false", "\"reason\":\"provider_capability_unavailable\""} {
			if !strings.Contains(got, want) {
				t.Fatalf("%s missing %q: %s", call.name, want, got)
			}
		}
	}
}

func TestCodeIntelligenceProtocolMismatchFallsBackBeforeBusinessToolCall(t *testing.T) {
	ctx := context.Background()
	service := app.New(filepath.Join(t.TempDir(), "state.json"))
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/protocol\n\ngo 1.26\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "service.go"), []byte("package protocol\n\ntype Service struct{}\nfunc (s *Service) Run() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	workspace, err := service.Workspaces.Add(root, "protocol-mismatch")
	if err != nil {
		t.Fatal(err)
	}
	environment, err := service.Environments.Create(workspace.ID, "protocol-mismatch", "")
	if err != nil {
		t.Fatal(err)
	}
	const writer = "protocol-index"
	if _, err := service.Environments.AcquireWriter(environment.ID, writer); err != nil {
		t.Fatal(err)
	}
	if _, err := service.AnalyzeProject(environment.ID, writer, 100, 100); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Environments.ReleaseWriter(environment.ID, writer, false); err != nil {
		t.Fatal(err)
	}

	phpStorm, err := service.MCPs.AddMCP("PhpStorm Code Intelligence", "http://127.0.0.1:65530/mcp", false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.SetEnvironmentMCP(environment.ID, phpStorm.ID, true); err != nil {
		t.Fatal(err)
	}

	fake := &codeIntelligenceRouteSession{protocolVersion: codeintel.ContractProtocolVersion + 1}
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
	})
	if query.IsError {
		t.Fatalf("protocol mismatch fallback failed: %s", toolText(t, query))
	}
	queryText := toolText(t, query)
	for _, want := range []string{
		"\"provider_id\":\"adm_static_index\"",
		"\"attempted_provider\":{\"contract\":\"adm.code_intelligence\"",
		"\"fallback_reason\":\"external_provider_protocol_mismatch\"",
		"\"qualified_name\":\"protocol.Service.Run\"",
	} {
		if !strings.Contains(queryText, want) {
			t.Fatalf("protocol mismatch fallback missing %q: %s", want, queryText)
		}
	}

	fake.mu.Lock()
	calls := append([]string(nil), fake.calls...)
	fake.mu.Unlock()
	if !contains(calls, "code_intelligence_info") {
		t.Fatalf("provider negotiation was not attempted: %v", calls)
	}
	if contains(calls, "code_intelligence_query") {
		t.Fatalf("business tool called despite protocol mismatch: %v", calls)
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

	fake := &codeIntelligenceRouteSession{callErrByTool: map[string]error{"code_intelligence_query": errors.New("provider temporarily unavailable")}}
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
		"\"attempted_provider\":{\"contract\":\"adm.code_intelligence\",\"protocol_version\":1,\"provider_id\":\"phpstorm\"",
		"\"fallback_reason\":\"external_provider_call_failed\"",
		"\"qualified_name\":\"fallback.Service.Run\"",
	} {
		if !strings.Contains(queryText, want) {
			t.Fatalf("fallback query missing %q: %s", want, queryText)
		}
	}
}
