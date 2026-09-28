package gateway

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

	"ai-dev-manager-v2/internal/app"
	"ai-dev-manager-v2/internal/codeintel"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestWebCodeIntelligenceOverviewUsesStaticProviderWithoutExternalMCP(t *testing.T) {
	service := app.New(filepath.Join(t.TempDir(), "state.json"))
	root := t.TempDir()
	workspace, err := service.Workspaces.Add(root, "web-codeintel")
	if err != nil {
		t.Fatal(err)
	}
	environment, err := service.Environments.Create(workspace.ID, "web-codeintel", "")
	if err != nil {
		t.Fatal(err)
	}
	handler, ok := newWebManagementHandler(service, nil).(*webManagementHandler)
	if !ok {
		t.Fatal("web management handler type assertion failed")
	}
	arg, err := json.Marshal(environment.ID)
	if err != nil {
		t.Fatal(err)
	}
	value, err := handler.dispatch(context.Background(), webCallRequest{
		Method: "CodeIntelligenceOverview",
		Args:   []json.RawMessage{arg},
	})
	if err != nil {
		t.Fatal(err)
	}
	overview, ok := value.(webCodeIntelligenceOverview)
	if !ok {
		t.Fatalf("overview type=%T value=%+v", value, value)
	}
	if overview.Provider.ID != "adm_static_index" {
		t.Fatalf("provider=%+v", overview.Provider)
	}
	if overview.Provider.Contract != codeintel.ContractName || overview.Provider.ProtocolVersion != codeintel.ContractProtocolVersion {
		t.Fatalf("provider contract=%+v", overview.Provider)
	}
	if !overview.Provider.Capabilities.Definitions || overview.Provider.Capabilities.References || overview.Provider.Capabilities.Hierarchy {
		t.Fatalf("provider capabilities=%+v", overview.Provider.Capabilities)
	}
	if overview.AttemptedProvider != nil || overview.FallbackReason != "" {
		t.Fatalf("unexpected fallback: attempted=%+v reason=%q", overview.AttemptedProvider, overview.FallbackReason)
	}
	if len(overview.Providers) < 2 {
		t.Fatalf("expected passive provider facts, got %+v", overview.Providers)
	}
}

func TestWebCodeIntelligenceOverviewShowsProtocolFallback(t *testing.T) {
	ctx := context.Background()
	service := app.New(filepath.Join(t.TempDir(), "state.json"))
	root := t.TempDir()
	workspace, err := service.Workspaces.Add(root, "web-protocol-fallback")
	if err != nil {
		t.Fatal(err)
	}
	environment, err := service.Environments.Create(workspace.ID, "web-protocol-fallback", "")
	if err != nil {
		t.Fatal(err)
	}
	phpStorm, err := service.MCPs.AddMCP("PhpStorm Code Intelligence", "http://127.0.0.1:65528/mcp", false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.SetEnvironmentMCP(environment.ID, phpStorm.ID, true); err != nil {
		t.Fatal(err)
	}

	fake := &codeIntelligenceRouteSession{protocolVersion: codeintel.ContractProtocolVersion + 1}
	runtimeOwner := newRuntimeOwner(service)
	defer runtimeOwner.Close()
	runtimeOwner.connect = func(context.Context, string, string, map[string]string) (ownedMCPSession, error) {
		return fake, nil
	}
	status, err := runtimeOwner.Status(ctx, environment.ID, phpStorm.ID)
	if err != nil || status.State != app.MCPHealthHealthy {
		t.Fatalf("prime PhpStorm provider: status=%+v err=%v", status, err)
	}

	handler, ok := newWebManagementHandler(service, runtimeOwner).(*webManagementHandler)
	if !ok {
		t.Fatal("web management handler type assertion failed")
	}
	arg, err := json.Marshal(environment.ID)
	if err != nil {
		t.Fatal(err)
	}
	value, err := handler.dispatch(ctx, webCallRequest{
		Method: "CodeIntelligenceOverview",
		Args:   []json.RawMessage{arg},
	})
	if err != nil {
		t.Fatal(err)
	}
	overview, ok := value.(webCodeIntelligenceOverview)
	if !ok {
		t.Fatalf("overview type=%T value=%+v", value, value)
	}
	if overview.Provider.ID != "adm_static_index" {
		t.Fatalf("provider=%+v", overview.Provider)
	}
	if overview.AttemptedProvider == nil || overview.AttemptedProvider.ID != "phpstorm" {
		t.Fatalf("attempted provider=%+v", overview.AttemptedProvider)
	}
	if overview.FallbackReason != "external_provider_protocol_mismatch" {
		t.Fatalf("fallback reason=%q", overview.FallbackReason)
	}
}

func TestWebCodeIntelligenceOverviewReportsJetBrainsNativeSchemaGate(t *testing.T) {
	ctx := context.Background()
	service := app.New(filepath.Join(t.TempDir(), "state.json"))
	workspace, err := service.Workspaces.Add(t.TempDir(), "web-jetbrains-native")
	if err != nil {
		t.Fatal(err)
	}
	environment, err := service.Environments.Create(workspace.ID, "web-jetbrains-native", "")
	if err != nil {
		t.Fatal(err)
	}
	jetBrains, err := service.MCPs.AddMCP("PhpStorm", "http://127.0.0.1:65525/mcp", false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.SetEnvironmentMCP(environment.ID, jetBrains.ID, true); err != nil {
		t.Fatal(err)
	}

	fake := &jetBrainsSchemaSession{tools: []*mcp.Tool{
		{
			Name: "search_symbol",
			InputSchema: map[string]any{
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
			OutputSchema: map[string]any{
				"type":       "object",
				"properties": map[string]any{"results": map[string]any{"type": "array"}},
			},
		},
		{Name: "get_symbol_info"},
		{Name: "analyze_calls"},
	}}
	runtimeOwner := newRuntimeOwner(service)
	defer runtimeOwner.Close()
	runtimeOwner.connect = func(context.Context, string, string, map[string]string) (ownedMCPSession, error) {
		return fake, nil
	}
	status, err := runtimeOwner.Status(ctx, environment.ID, jetBrains.ID)
	if err != nil || status.State != app.MCPHealthHealthy {
		t.Fatalf("prime JetBrains provider: status=%+v err=%v", status, err)
	}

	handler, ok := newWebManagementHandler(service, runtimeOwner).(*webManagementHandler)
	if !ok {
		t.Fatal("web management handler type assertion failed")
	}
	arg, err := json.Marshal(environment.ID)
	if err != nil {
		t.Fatal(err)
	}
	value, err := handler.dispatch(ctx, webCallRequest{
		Method: "CodeIntelligenceOverview",
		Args:   []json.RawMessage{arg},
	})
	if err != nil {
		t.Fatal(err)
	}
	overview, ok := value.(webCodeIntelligenceOverview)
	if !ok {
		t.Fatalf("overview type=%T value=%+v", value, value)
	}
	if overview.Provider.ID != "adm_static_index" {
		t.Fatalf("native candidate must not replace active provider yet: %+v", overview.Provider)
	}
	compatibility := overview.JetBrainsNativeCompatibility
	if compatibility == nil || !compatibility.InputCompatible || !compatibility.OutputSchemaAvailable {
		t.Fatalf("native compatibility=%+v", compatibility)
	}
	if compatibility.AutoRouteEnabled || compatibility.Reason != "search_symbol_schema_ready" {
		t.Fatalf("native route state=%+v", compatibility)
	}
	if fake.calls() != 0 {
		t.Fatalf("Web schema inspection called JetBrains business tools %d times", fake.calls())
	}
}

func TestWebCodeIntelligenceOverviewUsesNegotiatedPhpStormProvider(t *testing.T) {
	ctx := context.Background()
	service := app.New(filepath.Join(t.TempDir(), "state.json"))
	root := t.TempDir()
	workspace, err := service.Workspaces.Add(root, "web-phpstorm")
	if err != nil {
		t.Fatal(err)
	}
	environment, err := service.Environments.Create(workspace.ID, "web-phpstorm", "")
	if err != nil {
		t.Fatal(err)
	}
	phpStorm, err := service.MCPs.AddMCP("PhpStorm Code Intelligence", "http://127.0.0.1:65529/mcp", false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.SetEnvironmentMCP(environment.ID, phpStorm.ID, true); err != nil {
		t.Fatal(err)
	}

	fake := &codeIntelligenceRouteSession{}
	runtimeOwner := newRuntimeOwner(service)
	defer runtimeOwner.Close()
	runtimeOwner.connect = func(context.Context, string, string, map[string]string) (ownedMCPSession, error) {
		return fake, nil
	}
	status, err := runtimeOwner.Status(ctx, environment.ID, phpStorm.ID)
	if err != nil || status.State != app.MCPHealthHealthy {
		t.Fatalf("prime PhpStorm provider: status=%+v err=%v", status, err)
	}

	handler, ok := newWebManagementHandler(service, runtimeOwner).(*webManagementHandler)
	if !ok {
		t.Fatal("web management handler type assertion failed")
	}
	arg, err := json.Marshal(environment.ID)
	if err != nil {
		t.Fatal(err)
	}
	value, err := handler.dispatch(ctx, webCallRequest{
		Method: "CodeIntelligenceOverview",
		Args:   []json.RawMessage{arg},
	})
	if err != nil {
		t.Fatal(err)
	}
	overview, ok := value.(webCodeIntelligenceOverview)
	if !ok {
		t.Fatalf("overview type=%T value=%+v", value, value)
	}
	if overview.Provider.ID != "phpstorm" {
		t.Fatalf("provider=%+v", overview.Provider)
	}
	if overview.Provider.Contract != codeintel.ContractName || overview.Provider.ProtocolVersion != codeintel.ContractProtocolVersion {
		t.Fatalf("provider contract=%+v", overview.Provider)
	}
	if !overview.Provider.Capabilities.Definitions || !overview.Provider.Capabilities.References || !overview.Provider.Capabilities.Hierarchy {
		t.Fatalf("provider capabilities=%+v", overview.Provider.Capabilities)
	}
	if overview.AttemptedProvider != nil || overview.FallbackReason != "" {
		t.Fatalf("unexpected fallback: attempted=%+v reason=%q", overview.AttemptedProvider, overview.FallbackReason)
	}
}
