package gateway

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"ai-dev-manager-v2/internal/app"
	"ai-dev-manager-v2/internal/model"
)

func TestGatewayInvestigationProviderUsesExistingObservationWithoutConnecting(t *testing.T) {
	service := app.New(filepath.Join(t.TempDir(), "state.json"))
	workspace, err := service.Workspaces.Add(t.TempDir(), "gateway-provider")
	if err != nil {
		t.Fatal(err)
	}
	gitNexus, err := service.MCPs.AddMCP("GitNexus", "http://127.0.0.1:65534/mcp", false)
	if err != nil {
		t.Fatal(err)
	}
	environment, err := service.Environments.Create(workspace.ID, "gateway-provider", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.SetEnvironmentMCP(environment.ID, gitNexus.ID, true); err != nil {
		t.Fatal(err)
	}

	owner := newRuntimeOwner(service)
	defer owner.Close()
	connects := 0
	owner.connect = func(context.Context, string, string, map[string]string) (ownedMCPSession, error) {
		connects++
		return nil, errors.New("passive provider inspection must not connect")
	}
	observedAt := time.Now().UTC().Add(-time.Minute)
	owner.mu.Lock()
	owner.observations[runtimeOwnerKey{environmentID: environment.ID, mcpID: gitNexus.ID}] = app.MCPRuntimeObservation{
		EnvironmentID:      environment.ID,
		MCPID:              gitNexus.ID,
		DesiredEnabled:     true,
		State:              app.MCPHealthHealthy,
		LastCheckAt:        &observedAt,
		LastHealthyAt:      &observedAt,
		LastSuccessAt:      &observedAt,
		ToolInventory:      []app.MCPToolInventoryItem{{Name: "query"}, {Name: "impact"}, {Name: "mutating_tool"}},
		InventoryFetchedAt: &observedAt,
	}
	owner.mu.Unlock()

	report, err := owner.InvestigationProviderReport(context.Background(), environment.ID)
	if err != nil {
		t.Fatal(err)
	}
	provider := requireGatewayInvestigationProviderFact(t, report, app.InvestigationProviderGitNexusKey)
	if provider.State != model.CapabilityStateAvailable || provider.Source != capabilitySourceGatewayOwner || provider.ObservedAt == nil {
		t.Fatalf("provider fact = %+v", provider)
	}
	if provider.Confidence != "medium" || provider.Freshness != "inventory_observed" {
		t.Fatalf("provider confidence/freshness = %q/%q; fact=%+v", provider.Confidence, provider.Freshness, provider)
	}
	availableTools := capabilityEvidenceDetail(provider, "code_intelligence_provider_inventory", "available_read_tools")
	if availableTools != "impact,query" {
		t.Fatalf("available read tools = %q, want impact,query; fact=%+v", availableTools, provider)
	}
	if strings.Contains(availableTools, "mutating_tool") {
		t.Fatalf("provider inventory exposed unrecognized tool: %+v", provider)
	}
	if connects != 0 {
		t.Fatalf("provider report made %d MCP connections", connects)
	}

	session := connectInMemory(t, context.Background(), newServer(service, owner))
	defer session.Close()
	result := callGatewayTool(t, context.Background(), session, "investigation_provider_inspect", map[string]any{"environment_id": environment.ID})
	if result.IsError {
		t.Fatalf("investigation_provider_inspect failed: %s", toolText(t, result))
	}
	text := toolText(t, result)
	for _, required := range []string{app.InvestigationProviderGitNexusKey, "inventory_observed", "impact,query", capabilitySourceGatewayOwner} {
		if !strings.Contains(text, required) {
			t.Fatalf("provider inspection output missing %q:\n%s", required, text)
		}
	}
	if connects != 0 {
		t.Fatalf("gateway provider inspection made %d MCP connections", connects)
	}
}

func requireGatewayInvestigationProviderFact(t *testing.T, report model.InvestigationProviderReport, key string) model.CapabilityFact {
	t.Helper()
	for _, provider := range report.Providers {
		if provider.Key == key {
			return provider
		}
	}
	t.Fatalf("missing provider fact %q in %+v", key, report.Providers)
	return model.CapabilityFact{}
}
