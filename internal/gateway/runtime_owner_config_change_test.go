package gateway

import (
	"context"
	"testing"
	"time"

	"ai-dev-manager-v2/internal/app"
	"ai-dev-manager-v2/internal/catalog"
)

func TestRuntimeOwnerMonitorInvalidatesOutOfBandMCPConfigChange(t *testing.T) {
	service, environmentID, mcpID := runtimeOwnerTestService(t)
	owner := newRuntimeOwner(service)
	defer owner.Close()

	fake := &fakeOwnedMCPSession{}
	owner.connect = func(context.Context, string, string, map[string]string) (ownedMCPSession, error) {
		return fake, nil
	}
	if status, err := owner.Status(context.Background(), environmentID, mcpID); err != nil || status.State != app.MCPHealthHealthy {
		t.Fatalf("initial status=%+v err=%v", status, err)
	}
	owner.monitorOnce(time.Now().UTC())
	if owner.Info().OwnedMCPSessions != 1 {
		t.Fatalf("initial owner info=%+v", owner.Info())
	}

	if _, err := service.MCPs.UpdateMCPConfig(mcpID, "owned", catalog.MCPConfig{
		Transport: catalog.MCPTransportStreamableHTTP,
		AuthMode:  catalog.MCPAuthNone,
		Endpoint:  "http://127.0.0.1:65533/mcp",
	}); err != nil {
		t.Fatal(err)
	}
	owner.monitorOnce(time.Now().UTC().Add(time.Second))

	_, _, closes := fake.counts()
	if closes != 1 || owner.Info().OwnedMCPSessions != 0 {
		t.Fatalf("changed desired config did not invalidate runtime: closes=%d info=%+v", closes, owner.Info())
	}
	inspection, err := owner.Inspect(environmentID, mcpID)
	if err != nil {
		t.Fatal(err)
	}
	if inspection.Observation.State != app.MCPHealthConfigured || len(inspection.Observation.ToolInventory) != 0 {
		t.Fatalf("stale observation survived config update: %+v", inspection.Observation)
	}
}

func TestRuntimeOwnerStaleReconnectGenerationDoesNotBorrowUpdatedDefinition(t *testing.T) {
	service, environmentID, mcpID := runtimeOwnerTestService(t)
	owner := newRuntimeOwner(service)
	defer owner.Close()
	key := runtimeOwnerKey{environmentID: environmentID, mcpID: mcpID}

	definition, err := service.MCPs.Get(mcpID)
	if err != nil {
		t.Fatal(err)
	}
	owner.invalidateChangedDesiredConfig(key, definition)
	generation, ok := owner.beginWork(key)
	if !ok {
		t.Fatal("failed to begin reconnect work")
	}
	connects := 0
	owner.connect = func(context.Context, string, string, map[string]string) (ownedMCPSession, error) {
		connects++
		return &fakeOwnedMCPSession{}, nil
	}

	if _, err := service.MCPs.UpdateMCPConfig(mcpID, "owned", catalog.MCPConfig{
		Transport: catalog.MCPTransportStreamableHTTP,
		AuthMode:  catalog.MCPAuthNone,
		Endpoint:  "http://127.0.0.1:65532/mcp",
	}); err != nil {
		t.Fatal(err)
	}
	session, status, err := owner.ensureHealthySessionForGeneration(context.Background(), environmentID, mcpID, generation)
	if err != nil {
		t.Fatal(err)
	}
	if session != nil || status.State != app.MCPHealthConfigured {
		t.Fatalf("stale reconnect result session=%v status=%+v", session, status)
	}
	if connects != 0 {
		t.Fatalf("stale reconnect borrowed updated definition and connected %d times", connects)
	}
	if owner.generation(key) == generation {
		t.Fatalf("definition update did not advance generation %d", generation)
	}
	owner.endWork(key, generation)
	inspection, err := owner.Inspect(environmentID, mcpID)
	if err != nil {
		t.Fatal(err)
	}
	if inspection.Observation.InFlight {
		t.Fatalf("stale generation remained in flight: %+v", inspection.Observation)
	}
}
