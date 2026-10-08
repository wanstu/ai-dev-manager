package gateway

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"

	"ai-dev-manager-v2/internal/app"
	"ai-dev-manager-v2/internal/projectanalysis"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type jetBrainsSchemaSession struct {
	mu          sync.Mutex
	tools       []*mcp.Tool
	callCount   int
	callResult  *mcp.CallToolResult
	callResults map[string]*mcp.CallToolResult
	callsByTool map[string][]*mcp.CallToolParams
	lastCall    *mcp.CallToolParams
}

func (s *jetBrainsSchemaSession) Ping(context.Context, *mcp.PingParams) error { return nil }

func (s *jetBrainsSchemaSession) ListTools(context.Context, *mcp.ListToolsParams) (*mcp.ListToolsResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return &mcp.ListToolsResult{Tools: s.tools}, nil
}

func (s *jetBrainsSchemaSession) CallTool(_ context.Context, params *mcp.CallToolParams) (*mcp.CallToolResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.callCount++
	s.lastCall = params
	if s.callsByTool == nil {
		s.callsByTool = map[string][]*mcp.CallToolParams{}
	}
	s.callsByTool[params.Name] = append(s.callsByTool[params.Name], params)
	if result := s.callResults[params.Name]; result != nil {
		return result, nil
	}
	if s.callResult != nil {
		return s.callResult, nil
	}
	return &mcp.CallToolResult{}, nil
}

func (s *jetBrainsSchemaSession) Close() error { return nil }

func (s *jetBrainsSchemaSession) calls() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.callCount
}

func (s *jetBrainsSchemaSession) toolCalls(name string) []*mcp.CallToolParams {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]*mcp.CallToolParams(nil), s.callsByTool[name]...)
}

func TestJetBrainsNativeSearchSymbolSchemaGate(t *testing.T) {
	tests := []struct {
		name                 string
		inputSchema          any
		outputSchema         any
		wantInputCompatible  bool
		wantOutputSchema     bool
		wantOutputCompatible bool
		wantAutoRoute        bool
		wantReason           string
		wantInputFields      []string
	}{
		{
			name: "official shape with structured output",
			inputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"q":                map[string]any{"type": "string"},
					"projectPath":      map[string]any{"type": "string"},
					"limit":            map[string]any{"type": "integer"},
					"paths":            map[string]any{"type": "array"},
					"include_external": map[string]any{"type": "boolean"},
				},
				"required": []any{"q"},
			},
			outputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"results": map[string]any{
						"type": "array",
						"items": map[string]any{
							"type": "object",
							"properties": map[string]any{
								"name":          map[string]any{"type": "string"},
								"filePath":      map[string]any{"type": "string"},
								"line":          map[string]any{"type": "integer"},
								"column":        map[string]any{"type": "integer"},
								"kind":          map[string]any{"type": "string"},
								"qualifiedName": map[string]any{"type": "string"},
							},
						},
					},
				},
			},
			wantInputCompatible:  true,
			wantOutputSchema:     true,
			wantOutputCompatible: true,
			wantAutoRoute:        true,
			wantReason:           "search_symbol_schema_ready",
			wantInputFields:      []string{"include_external", "limit", "paths", "projectPath", "q"},
		},
		{
			name: "jetbrains 2026.2 coordinate output",
			inputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"q":                map[string]any{"type": "string"},
					"projectPath":      map[string]any{"type": "string"},
					"limit":            map[string]any{"type": "integer"},
					"paths":            map[string]any{"type": "array"},
					"include_external": map[string]any{"type": "boolean"},
				},
				"required": []any{"q"},
			},
			outputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"items": map[string]any{
						"type": "array",
						"items": map[string]any{
							"type":     "object",
							"required": []any{"filePath"},
							"properties": map[string]any{
								"filePath":    map[string]any{"type": "string"},
								"startLine":   map[string]any{"type": []any{"integer", "null"}},
								"startColumn": map[string]any{"type": []any{"integer", "null"}},
								"endLine":     map[string]any{"type": []any{"integer", "null"}},
								"endColumn":   map[string]any{"type": []any{"integer", "null"}},
							},
						},
					},
					"more":                map[string]any{"type": "boolean"},
					"partialResultReason": map[string]any{"type": []any{"string", "null"}},
				},
			},
			wantInputCompatible:  true,
			wantOutputSchema:     true,
			wantOutputCompatible: true,
			wantAutoRoute:        true,
			wantReason:           "search_symbol_schema_ready",
			wantInputFields:      []string{"include_external", "limit", "paths", "projectPath", "q"},
		},
		{
			name: "q is not required",
			inputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"q":           map[string]any{"type": "string"},
					"projectPath": map[string]any{"type": "string"},
					"limit":       map[string]any{"type": "integer"},
				},
			},
			outputSchema: map[string]any{"type": "object"},
			wantReason:   "search_symbol_input_schema_incompatible",
		},
		{
			name: "output schema cannot map file path",
			inputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"q":           map[string]any{"type": "string"},
					"projectPath": map[string]any{"type": "string"},
					"limit":       map[string]any{"type": "integer"},
				},
				"required": []any{"q"},
			},
			outputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"results": map[string]any{
						"type": "array",
						"items": map[string]any{
							"type": "object",
							"properties": map[string]any{
								"name": map[string]any{"type": "string"},
								"line": map[string]any{"type": "integer"},
							},
						},
					},
				},
			},
			wantInputCompatible: true,
			wantOutputSchema:    true,
			wantReason:          "search_symbol_output_schema_incompatible",
			wantInputFields:     []string{"limit", "projectPath", "q"},
		},
		{
			name: "output schema missing",
			inputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"q":           map[string]any{"type": "string"},
					"projectPath": map[string]any{"type": "string"},
					"limit":       map[string]any{"type": "integer"},
				},
				"required": []any{"q"},
			},
			wantInputCompatible: true,
			wantReason:          "search_symbol_output_schema_missing",
			wantInputFields:     []string{"limit", "projectPath", "q"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := app.New(filepath.Join(t.TempDir(), "state.json"))
			workspace, err := service.Workspaces.Add(t.TempDir(), "jetbrains-schema")
			if err != nil {
				t.Fatal(err)
			}
			environment, err := service.Environments.Create(workspace.ID, "jetbrains-schema", "")
			if err != nil {
				t.Fatal(err)
			}
			entry, err := service.MCPs.AddMCP("PhpStorm", "http://127.0.0.1:65526/mcp", false)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := service.SetEnvironmentMCP(environment.ID, entry.ID, true); err != nil {
				t.Fatal(err)
			}

			fake := &jetBrainsSchemaSession{tools: []*mcp.Tool{
				{Name: "search_symbol", InputSchema: tt.inputSchema, OutputSchema: tt.outputSchema},
				{Name: "get_symbol_info"},
				{Name: "analyze_calls"},
			}}
			owner := newRuntimeOwner(service)
			defer owner.Close()
			owner.connect = func(context.Context, string, string, map[string]string) (ownedMCPSession, error) {
				return fake, nil
			}

			status, err := owner.Status(context.Background(), environment.ID, entry.ID)
			if err != nil || status.State != app.MCPHealthHealthy {
				t.Fatalf("prime JetBrains MCP: status=%+v err=%v", status, err)
			}
			got := owner.inspectJetBrainsNativeCompatibility(context.Background(), environment.ID)
			if got.ProviderID != app.InvestigationProviderJetBrainsNative || got.MCPID != entry.ID || got.Tool != "search_symbol" {
				t.Fatalf("identity=%+v", got)
			}
			if got.InputCompatible != tt.wantInputCompatible ||
				got.OutputSchemaAvailable != tt.wantOutputSchema ||
				got.OutputCompatible != tt.wantOutputCompatible ||
				got.AutoRouteEnabled != tt.wantAutoRoute ||
				got.Reason != tt.wantReason {
				t.Fatalf("compatibility=%+v", got)
			}
			if tt.wantInputFields != nil && !reflect.DeepEqual(got.InputFields, tt.wantInputFields) {
				t.Fatalf("input fields=%v want=%v", got.InputFields, tt.wantInputFields)
			}
			if fake.calls() != 0 {
				t.Fatalf("schema inspection called a business tool %d times", fake.calls())
			}
		})
	}
}

func TestJetBrainsNativeSearchProbeUsesBoundedEnvironmentScopedArguments(t *testing.T) {
	ctx := context.Background()
	service := app.New(filepath.Join(t.TempDir(), "state.json"))
	root := t.TempDir()
	workspace, err := service.Workspaces.Add(root, "jetbrains-probe")
	if err != nil {
		t.Fatal(err)
	}
	environment, err := service.Environments.Create(workspace.ID, "jetbrains-probe", "")
	if err != nil {
		t.Fatal(err)
	}
	entry, err := service.MCPs.AddMCP("PhpStorm", "http://127.0.0.1:65524/mcp", false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.SetEnvironmentMCP(environment.ID, entry.ID, true); err != nil {
		t.Fatal(err)
	}

	fake := &jetBrainsSchemaSession{
		tools: []*mcp.Tool{
			{
				Name: "search_symbol",
				InputSchema: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"q":                map[string]any{"type": "string"},
						"projectPath":      map[string]any{"type": "string"},
						"limit":            map[string]any{"type": "integer"},
						"include_external": map[string]any{"type": "boolean"},
					},
					"required": []any{"q"},
				},
			},
		},
		callResult: &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: "Foo src/Foo.php:17:1"}},
			StructuredContent: map[string]any{
				"results": []any{map[string]any{"name": "Foo", "filePath": "src/Foo.php", "line": 17}},
			},
		},
	}
	owner := newRuntimeOwner(service)
	defer owner.Close()
	owner.connect = func(context.Context, string, string, map[string]string) (ownedMCPSession, error) {
		return fake, nil
	}
	status, err := owner.Status(ctx, environment.ID, entry.ID)
	if err != nil || status.State != app.MCPHealthHealthy {
		t.Fatalf("prime JetBrains MCP: status=%+v err=%v", status, err)
	}

	result, err := owner.probeJetBrainsNativeSearch(ctx, environment.ID, " Foo ", 999)
	if err != nil {
		t.Fatal(err)
	}
	if result.ProviderID != app.InvestigationProviderJetBrainsNative || result.MCPID != entry.ID || result.Tool != "search_symbol" {
		t.Fatalf("probe identity=%+v", result)
	}
	if result.Query != "Foo" || !sameProjectRoot(result.ProjectPath, root) || result.Limit != 50 || result.IncludeExternal {
		t.Fatalf("probe scope=%+v", result)
	}
	if result.TextPreview != "Foo src/Foo.php:17:1" || result.StructuredJSONPreview == "" || result.Truncated {
		t.Fatalf("probe output=%+v", result)
	}

	fake.mu.Lock()
	call := fake.lastCall
	callCount := fake.callCount
	fake.mu.Unlock()
	if callCount != 1 || call == nil || call.Name != "search_symbol" {
		t.Fatalf("search_symbol calls=%d params=%+v", callCount, call)
	}
	args, ok := call.Arguments.(map[string]any)
	if !ok {
		t.Fatalf("arguments type=%T", call.Arguments)
	}
	if args["q"] != "Foo" || !sameProjectRoot(args["projectPath"], root) || args["limit"] != 50 || args["include_external"] != false {
		t.Fatalf("arguments=%+v", args)
	}
}

func compatibleJetBrainsSearchTool() *mcp.Tool {
	return &mcp.Tool{
		Name: "search_symbol",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"q":                map[string]any{"type": "string"},
				"paths":            map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
				"include_external": map[string]any{"type": "boolean"},
				"limit":            map[string]any{"type": "integer"},
				"projectPath":      map[string]any{"type": "string"},
			},
			"required": []any{"q"},
		},
		OutputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"items": map[string]any{
					"type": "array",
					"items": map[string]any{
						"type":     "object",
						"required": []any{"filePath"},
						"properties": map[string]any{
							"filePath":    map[string]any{"type": "string"},
							"startLine":   map[string]any{"type": []any{"integer", "null"}},
							"startColumn": map[string]any{"type": []any{"integer", "null"}},
							"endLine":     map[string]any{"type": []any{"integer", "null"}},
							"endColumn":   map[string]any{"type": []any{"integer", "null"}},
						},
					},
				},
				"more":                map[string]any{"type": "boolean"},
				"partialResultReason": map[string]any{"type": []any{"string", "null"}},
			},
		},
	}
}

func compatibleJetBrainsGetSymbolInfoTool() *mcp.Tool {
	return &mcp.Tool{
		Name: "get_symbol_info",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"filePath":    map[string]any{"type": "string"},
				"line":        map[string]any{"type": "integer"},
				"column":      map[string]any{"type": "integer"},
				"projectPath": map[string]any{"type": "string"},
			},
			"required": []any{"filePath", "line", "column"},
		},
		OutputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"symbolInfo": map[string]any{
					"type":     []any{"object", "null"},
					"required": []any{"declarationText"},
					"properties": map[string]any{
						"name":            map[string]any{"type": []any{"string", "null"}},
						"declarationText": map[string]any{"type": "string"},
						"declarationFile": map[string]any{"type": []any{"string", "null"}},
						"declarationLine": map[string]any{"type": []any{"integer", "null"}},
						"language":        map[string]any{"type": []any{"string", "null"}},
					},
				},
				"documentation":       map[string]any{"type": "string"},
				"partialResultReason": map[string]any{"type": []any{"string", "null"}},
			},
			"required": []any{"documentation"},
		},
	}
}

func TestJetBrainsGetSymbolInfoSchemaGate(t *testing.T) {
	if !jetBrainsGetSymbolInfoToolCompatible(compatibleJetBrainsGetSymbolInfoTool()) {
		t.Fatal("official get_symbol_info schema should be compatible")
	}
	incompatible := compatibleJetBrainsGetSymbolInfoTool()
	incompatible.OutputSchema = map[string]any{
		"type": "object",
		"properties": map[string]any{
			"documentation": map[string]any{"type": "string"},
		},
	}
	if jetBrainsGetSymbolInfoToolCompatible(incompatible) {
		t.Fatal("get_symbol_info without structured symbolInfo.name should be incompatible")
	}
}

func TestJetBrainsNativeExactQueryRejectsCoordinateOnlyNames(t *testing.T) {
	root := t.TempDir()
	_, err := decodeJetBrainsNativeSearchResult(root, projectanalysis.IndexQuery{
		Query:      "Foo",
		Exact:      true,
		MaxResults: 10,
	}, 10, &mcp.CallToolResult{
		StructuredContent: map[string]any{
			"items": []any{map[string]any{
				"filePath":  "src/Foo.php",
				"startLine": 17,
			}},
		},
	})
	if err == nil || !strings.Contains(err.Error(), "no mappable symbol rows") {
		t.Fatalf("exact coordinate-only result error = %v", err)
	}
}

func TestJetBrainsNativeSymbolInfoRequiresStructuredName(t *testing.T) {
	_, _, err := decodeJetBrainsNativeSymbolInfo(&mcp.CallToolResult{
		StructuredContent: map[string]any{
			"documentation": "function Foo(): void",
		},
	})
	if err == nil || !strings.Contains(err.Error(), "no symbolInfo") {
		t.Fatalf("documentation-only symbol info error = %v", err)
	}
}

func TestJetBrainsNativeExactQueryUsesStructuredSymbolInfo(t *testing.T) {
	ctx := context.Background()
	service := app.New(filepath.Join(t.TempDir(), "state.json"))
	root := t.TempDir()
	workspace, err := service.Workspaces.Add(root, "jetbrains-native-exact")
	if err != nil {
		t.Fatal(err)
	}
	environment, err := service.Environments.Create(workspace.ID, "jetbrains-native-exact", "")
	if err != nil {
		t.Fatal(err)
	}
	entry, err := service.MCPs.AddMCP("PhpStorm", "http://127.0.0.1:65521/mcp", false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.SetEnvironmentMCP(environment.ID, entry.ID, true); err != nil {
		t.Fatal(err)
	}

	fake := &jetBrainsSchemaSession{
		tools: []*mcp.Tool{compatibleJetBrainsSearchTool(), compatibleJetBrainsGetSymbolInfoTool()},
		callResults: map[string]*mcp.CallToolResult{
			"search_symbol": {
				StructuredContent: map[string]any{
					"items": []any{map[string]any{
						"filePath":    "src/Foo.js",
						"startLine":   17,
						"startColumn": 7,
						"endLine":     17,
						"endColumn":   10,
					}},
					"more": false,
				},
			},
			"get_symbol_info": {
				StructuredContent: map[string]any{
					"symbolInfo": map[string]any{
						"name":            "Foo",
						"declarationText": "function Foo() {}",
						"declarationFile": "src/Foo.js",
						"declarationLine": 17,
						"language":        "JavaScript",
					},
					"documentation": "function Foo(): void",
				},
			},
		},
	}
	owner := newRuntimeOwner(service)
	defer owner.Close()
	owner.connect = func(context.Context, string, string, map[string]string) (ownedMCPSession, error) {
		return fake, nil
	}
	status, err := owner.Status(ctx, environment.ID, entry.ID)
	if err != nil || status.State != app.MCPHealthHealthy {
		t.Fatalf("prime JetBrains MCP: status=%+v err=%v", status, err)
	}
	compatibility := owner.inspectJetBrainsNativeCompatibility(ctx, environment.ID)
	if !compatibility.AutoRouteEnabled || !compatibility.ExactSymbolInfoEnabled {
		t.Fatalf("compatibility=%+v", compatibility)
	}

	result, err := owner.queryCodeIntelligence(ctx, environment.ID, projectanalysis.IndexQuery{
		Query:      "Foo",
		Exact:      true,
		MaxResults: 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Provider.ID != app.InvestigationProviderJetBrainsNative || result.Result.Returned != 1 {
		t.Fatalf("result=%+v", result)
	}
	match := result.Result.Matches[0]
	if match.Path != "src/Foo.js" || match.Name != "Foo" || match.Language != "JavaScript" || match.Line != 17 {
		t.Fatalf("match=%+v", match)
	}

	searchCalls := fake.toolCalls("search_symbol")
	infoCalls := fake.toolCalls("get_symbol_info")
	if len(searchCalls) != 1 || len(infoCalls) != 1 {
		t.Fatalf("search calls=%d info calls=%d", len(searchCalls), len(infoCalls))
	}
	infoArgs, ok := infoCalls[0].Arguments.(map[string]any)
	if !ok {
		t.Fatalf("get_symbol_info arguments type=%T", infoCalls[0].Arguments)
	}
	if infoArgs["filePath"] != "src/Foo.js" || infoArgs["line"] != 17 || infoArgs["column"] != 7 || !sameProjectRoot(infoArgs["projectPath"], root) {
		t.Fatalf("get_symbol_info arguments=%+v", infoArgs)
	}
}

func TestJetBrainsNativeGenericQueryUsesCompatibleIDEIndex(t *testing.T) {
	ctx := context.Background()
	service := app.New(filepath.Join(t.TempDir(), "state.json"))
	root := t.TempDir()
	workspace, err := service.Workspaces.Add(root, "jetbrains-native-query")
	if err != nil {
		t.Fatal(err)
	}
	environment, err := service.Environments.Create(workspace.ID, "jetbrains-native-query", "")
	if err != nil {
		t.Fatal(err)
	}
	entry, err := service.MCPs.AddMCP("PhpStorm", "http://127.0.0.1:65523/mcp", false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.SetEnvironmentMCP(environment.ID, entry.ID, true); err != nil {
		t.Fatal(err)
	}

	outside := filepath.Join(t.TempDir(), "External.php")
	fake := &jetBrainsSchemaSession{
		tools: []*mcp.Tool{compatibleJetBrainsSearchTool()},
		callResult: &mcp.CallToolResult{
			StructuredContent: map[string]any{
				"items": []any{
					map[string]any{
						"filePath":    "src/Foo.php",
						"startLine":   17,
						"startColumn": 7,
						"endLine":     17,
						"endColumn":   10,
					},
					map[string]any{
						"filePath":  outside,
						"startLine": 4,
					},
				},
				"more": false,
			},
		},
	}
	owner := newRuntimeOwner(service)
	defer owner.Close()
	owner.connect = func(context.Context, string, string, map[string]string) (ownedMCPSession, error) {
		return fake, nil
	}
	status, err := owner.Status(ctx, environment.ID, entry.ID)
	if err != nil || status.State != app.MCPHealthHealthy {
		t.Fatalf("prime JetBrains MCP: status=%+v err=%v", status, err)
	}

	info, err := owner.preferredCodeIntelligenceInfo(ctx, environment.ID)
	if err != nil {
		t.Fatal(err)
	}
	if info.ID != app.InvestigationProviderJetBrainsNative || !info.Capabilities.Definitions || info.Capabilities.References || info.Capabilities.Hierarchy {
		t.Fatalf("preferred provider=%+v", info)
	}

	result, err := owner.queryCodeIntelligence(ctx, environment.ID, projectanalysis.IndexQuery{
		Query:      "Foo",
		MaxResults: 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Provider.ID != app.InvestigationProviderJetBrainsNative {
		t.Fatalf("provider=%+v", result.Provider)
	}
	if result.Result.IndexPath != "provider://jetbrains_native/search_symbol" || result.Result.Returned != 1 || result.Result.Truncated {
		t.Fatalf("query result=%+v", result.Result)
	}
	match := result.Result.Matches[0]
	if match.Path != "src/Foo.php" || match.Name != "Foo" || match.QualifiedName != "" || match.Line != 17 || match.Kind != "" || match.Language != "" {
		t.Fatalf("match=%+v", match)
	}

	fake.mu.Lock()
	call := fake.lastCall
	fake.mu.Unlock()
	if call == nil || call.Name != "search_symbol" {
		t.Fatalf("last call=%+v", call)
	}
	args, ok := call.Arguments.(map[string]any)
	if !ok {
		t.Fatalf("arguments type=%T", call.Arguments)
	}
	if args["q"] != "Foo" || !sameProjectRoot(args["projectPath"], root) || args["limit"] != 10 || args["include_external"] != false {
		t.Fatalf("arguments=%+v", args)
	}
}

func TestJetBrainsNativeInvalidResultFallsBackToStaticIndex(t *testing.T) {
	ctx := context.Background()
	service := app.New(filepath.Join(t.TempDir(), "state.json"))
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/nativefallback\n\ngo 1.26\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "service.go"), []byte("package nativefallback\n\ntype Service struct{}\nfunc (s *Service) Run() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	workspace, err := service.Workspaces.Add(root, "jetbrains-native-fallback")
	if err != nil {
		t.Fatal(err)
	}
	environment, err := service.Environments.Create(workspace.ID, "jetbrains-native-fallback", "")
	if err != nil {
		t.Fatal(err)
	}
	const writer = "native-fallback-index"
	if _, err := service.Environments.AcquireWriter(environment.ID, writer); err != nil {
		t.Fatal(err)
	}
	if _, err := service.AnalyzeProject(environment.ID, writer, 100, 100); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Environments.ReleaseWriter(environment.ID, writer, false); err != nil {
		t.Fatal(err)
	}

	entry, err := service.MCPs.AddMCP("PhpStorm", "http://127.0.0.1:65522/mcp", false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.SetEnvironmentMCP(environment.ID, entry.ID, true); err != nil {
		t.Fatal(err)
	}
	fake := &jetBrainsSchemaSession{
		tools: []*mcp.Tool{compatibleJetBrainsSearchTool()},
		callResult: &mcp.CallToolResult{StructuredContent: map[string]any{
			"results": []any{map[string]any{"name": "Service.Run"}},
		}},
	}
	owner := newRuntimeOwner(service)
	defer owner.Close()
	owner.connect = func(context.Context, string, string, map[string]string) (ownedMCPSession, error) {
		return fake, nil
	}
	status, err := owner.Status(ctx, environment.ID, entry.ID)
	if err != nil || status.State != app.MCPHealthHealthy {
		t.Fatalf("prime JetBrains MCP: status=%+v err=%v", status, err)
	}

	result, err := owner.queryCodeIntelligence(ctx, environment.ID, projectanalysis.IndexQuery{
		Query:      "Service.Run",
		MaxResults: 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Provider.ID != "adm_static_index" {
		t.Fatalf("provider=%+v", result.Provider)
	}
	if result.AttemptedProvider == nil || result.AttemptedProvider.ID != app.InvestigationProviderJetBrainsNative || result.FallbackReason != "jetbrains_native_invalid_result" {
		t.Fatalf("fallback=%+v reason=%q", result.AttemptedProvider, result.FallbackReason)
	}
	if result.Result.Returned != 1 || result.Result.Matches[0].QualifiedName != "nativefallback.Service.Run" {
		t.Fatalf("static fallback result=%+v", result.Result)
	}
}
