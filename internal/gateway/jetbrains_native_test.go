package gateway

import (
	"context"
	"path/filepath"
	"reflect"
	"sync"
	"testing"

	"ai-dev-manager-v2/internal/app"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type jetBrainsSchemaSession struct {
	mu        sync.Mutex
	tools     []*mcp.Tool
	callCount int
}

func (s *jetBrainsSchemaSession) Ping(context.Context, *mcp.PingParams) error { return nil }

func (s *jetBrainsSchemaSession) ListTools(context.Context, *mcp.ListToolsParams) (*mcp.ListToolsResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return &mcp.ListToolsResult{Tools: s.tools}, nil
}

func (s *jetBrainsSchemaSession) CallTool(context.Context, *mcp.CallToolParams) (*mcp.CallToolResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.callCount++
	return &mcp.CallToolResult{}, nil
}

func (s *jetBrainsSchemaSession) Close() error { return nil }

func (s *jetBrainsSchemaSession) calls() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.callCount
}

func TestJetBrainsNativeSearchSymbolSchemaGate(t *testing.T) {
	tests := []struct {
		name                string
		inputSchema         any
		outputSchema        any
		wantInputCompatible bool
		wantOutputSchema    bool
		wantReason          string
		wantInputFields     []string
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
					"results": map[string]any{"type": "array"},
				},
			},
			wantInputCompatible: true,
			wantOutputSchema:    true,
			wantReason:          "search_symbol_schema_ready",
			wantInputFields:     []string{"include_external", "limit", "paths", "projectPath", "q"},
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
			if got.InputCompatible != tt.wantInputCompatible || got.OutputSchemaAvailable != tt.wantOutputSchema || got.Reason != tt.wantReason {
				t.Fatalf("compatibility=%+v", got)
			}
			if got.AutoRouteEnabled {
				t.Fatalf("schema gate must not enable auto route: %+v", got)
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
