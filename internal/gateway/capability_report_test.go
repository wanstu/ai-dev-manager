package gateway

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"ai-dev-manager-v2/internal/app"
	"ai-dev-manager-v2/internal/catalog"
	"ai-dev-manager-v2/internal/model"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestGatewayCapabilityReportEnrichesOwnerObservationWithoutSideEffects(t *testing.T) {
	service, environmentID, healthyID, brokenID := gatewayCapabilityReportService(t)
	owner := newRuntimeOwner(service)
	defer owner.Close()
	connects := 0
	owner.connect = func(context.Context, string, string, map[string]string) (ownedMCPSession, error) {
		connects++
		return nil, errors.New("capability report must not connect")
	}
	observedAt := time.Now().UTC().Add(-time.Minute)
	owner.mu.Lock()
	owner.observations[runtimeOwnerKey{environmentID: environmentID, mcpID: healthyID}] = app.MCPRuntimeObservation{
		EnvironmentID:      environmentID,
		MCPID:              healthyID,
		DesiredEnabled:     true,
		Transport:          catalog.MCPTransportStreamableHTTP,
		State:              app.MCPHealthHealthy,
		LastCheckAt:        &observedAt,
		LastHealthyAt:      &observedAt,
		LastSuccessAt:      &observedAt,
		ToolInventory:      []app.MCPToolInventoryItem{{Name: "owned_fake"}},
		InventoryFetchedAt: &observedAt,
	}
	owner.observations[runtimeOwnerKey{environmentID: environmentID, mcpID: brokenID}] = app.MCPRuntimeObservation{
		EnvironmentID:       environmentID,
		MCPID:               brokenID,
		DesiredEnabled:      true,
		Transport:           catalog.MCPTransportStreamableHTTP,
		State:               app.MCPHealthError,
		FailureStage:        app.MCPFailurePing,
		ErrorKind:           "connection_refused",
		Message:             "external MCP ping failed",
		LastCheckAt:         &observedAt,
		ConsecutiveFailures: 2,
	}
	owner.mu.Unlock()

	report, err := owner.CapabilityReport(context.Background(), environmentID)
	if err != nil {
		t.Fatal(err)
	}
	if connects != 0 {
		t.Fatalf("capability report made %d MCP connections", connects)
	}
	healthy := requireCapabilityFact(t, report, "mcp/"+healthyID)
	if healthy.State != model.CapabilityStateAvailable || healthy.Source != capabilitySourceGatewayOwner || healthy.ObservedAt == nil {
		t.Fatalf("healthy MCP fact = %+v", healthy)
	}
	if got := capabilityEvidenceDetail(healthy, "mcp_runtime_observation", "tool_inventory_count"); got != "1" {
		t.Fatalf("healthy MCP tool inventory count = %q, want 1; fact=%+v", got, healthy)
	}
	broken := requireCapabilityFact(t, report, "mcp/"+brokenID)
	if broken.State != model.CapabilityStateUnavailable || broken.ReasonCode != "connection_refused" {
		t.Fatalf("broken MCP fact = %+v", broken)
	}
	if got := capabilityEvidenceDetail(broken, "mcp_runtime_observation", "failure_stage"); got != string(app.MCPFailurePing) {
		t.Fatalf("broken MCP failure stage = %q, want ping; fact=%+v", got, broken)
	}
	processFact := requireCapabilityFact(t, report, "process.lifecycle")
	if processFact.State != model.CapabilityStateAvailable || processFact.Source != capabilitySourceGatewayOwner || !processFact.RequiresWriter {
		t.Fatalf("process lifecycle fact = %+v", processFact)
	}
	runFact := requireCapabilityFact(t, report, "run.lifecycle")
	if runFact.State != model.CapabilityStateAvailable || runFact.Source != capabilitySourceGatewayOwner || !runFact.RequiresWriter {
		t.Fatalf("run lifecycle fact = %+v", runFact)
	}

	session := connectInMemory(t, context.Background(), newServer(service, owner))
	defer session.Close()
	result := callGatewayTool(t, context.Background(), session, "environment_capability_report", map[string]any{"environment_id": environmentID})
	if result.IsError {
		t.Fatalf("environment_capability_report failed: %s", toolText(t, result))
	}
	text := toolText(t, result)
	for _, required := range []string{healthyID, brokenID, capabilitySourceGatewayOwner, "mcp_runtime_observation", "tool_inventory_count", "connection_refused"} {
		if !strings.Contains(text, required) {
			t.Fatalf("capability report output missing %q:\n%s", required, text)
		}
	}
	if connects != 0 {
		t.Fatalf("gateway capability report made %d MCP connections", connects)
	}
}

func TestGatewayCapabilityReportAfterOwnerRestartDoesNotReuseStaleObservation(t *testing.T) {
	service, environmentID, healthyID, _ := gatewayCapabilityReportService(t)
	first := newRuntimeOwner(service)
	observedAt := time.Now().UTC()
	first.mu.Lock()
	first.observations[runtimeOwnerKey{environmentID: environmentID, mcpID: healthyID}] = app.MCPRuntimeObservation{
		EnvironmentID:  environmentID,
		MCPID:          healthyID,
		DesiredEnabled: true,
		Transport:      catalog.MCPTransportStreamableHTTP,
		State:          app.MCPHealthHealthy,
		LastCheckAt:    &observedAt,
	}
	first.mu.Unlock()
	firstReport, err := first.CapabilityReport(context.Background(), environmentID)
	if err != nil {
		t.Fatal(err)
	}
	if fact := requireCapabilityFact(t, firstReport, "mcp/"+healthyID); fact.State != model.CapabilityStateAvailable {
		t.Fatalf("first owner fact = %+v", fact)
	}
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}

	second := newRuntimeOwner(service)
	defer second.Close()
	connects := 0
	second.connect = func(context.Context, string, string, map[string]string) (ownedMCPSession, error) {
		connects++
		return nil, errors.New("capability report must not reconnect after owner restart")
	}
	secondReport, err := second.CapabilityReport(context.Background(), environmentID)
	if err != nil {
		t.Fatal(err)
	}
	fact := requireCapabilityFact(t, secondReport, "mcp/"+healthyID)
	if fact.State != model.CapabilityStateDegraded || fact.ReasonCode != "not_observed" {
		t.Fatalf("second owner should report absence of observation, got %+v", fact)
	}
	if got := capabilityEvidenceDetail(fact, "mcp_runtime_observation", "observed"); got != "false" {
		t.Fatalf("second owner observation detail = %q, want false; fact=%+v", got, fact)
	}
	if connects != 0 {
		t.Fatalf("second owner capability report made %d MCP connections", connects)
	}
}

func gatewayCapabilityReportService(t *testing.T) (*app.Service, string, string, string) {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "marker.txt"), []byte("capability\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	service := app.New(filepath.Join(t.TempDir(), "state.json"))
	workspace, err := service.Workspaces.Add(root, "gateway-capability")
	if err != nil {
		t.Fatal(err)
	}
	healthy, err := service.MCPs.AddMCP("healthy-owner", "http://127.0.0.1:65534/mcp", false)
	if err != nil {
		t.Fatal(err)
	}
	broken, err := service.MCPs.AddMCP("broken-owner", "http://127.0.0.1:65533/mcp", false)
	if err != nil {
		t.Fatal(err)
	}
	environment, err := service.Environments.Create(workspace.ID, "gateway-capability", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.SetEnvironmentMCP(environment.ID, healthy.ID, true); err != nil {
		t.Fatal(err)
	}
	if _, err := service.SetEnvironmentMCP(environment.ID, broken.ID, true); err != nil {
		t.Fatal(err)
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	if err := service.AllowExecutable(executable); err != nil {
		t.Fatal(err)
	}
	return service, environment.ID, healthy.ID, broken.ID
}

func requireCapabilityFact(t *testing.T, report model.CapabilityReport, key string) model.CapabilityFact {
	t.Helper()
	for _, fact := range report.Facts {
		if fact.Key == key {
			return fact
		}
	}
	t.Fatalf("missing capability fact %q in %+v", key, report.Facts)
	return model.CapabilityFact{}
}

func capabilityEvidenceDetail(fact model.CapabilityFact, evidenceKind, key string) string {
	for _, evidence := range fact.Evidence {
		if evidence.Kind == evidenceKind && evidence.Details != nil {
			return evidence.Details[key]
		}
	}
	return ""
}

var _ ownedMCPSession = (*fakeOwnedMCPSession)(nil)
var _ = mcp.Tool{}
