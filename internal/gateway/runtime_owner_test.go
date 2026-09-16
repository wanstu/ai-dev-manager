package gateway

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"ai-dev-manager-v2/internal/app"
	"ai-dev-manager-v2/internal/catalog"
	"ai-dev-manager-v2/internal/model"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type fakeOwnedMCPSession struct {
	mu        sync.Mutex
	pingCalls int
	listCalls int
	callCalls int
	closes    int
	failAfter int
	pingDelay time.Duration
	toolName  string
	listErr   error
	callErr   error
}

func (s *fakeOwnedMCPSession) Ping(ctx context.Context, _ *mcp.PingParams) error {
	s.mu.Lock()
	s.pingCalls++
	call := s.pingCalls
	delay := s.pingDelay
	failAfter := s.failAfter
	listErr := s.listErr
	s.mu.Unlock()
	if delay > 0 {
		select {
		case <-time.After(delay):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	if failAfter > 0 && call > failAfter {
		if listErr != nil {
			return listErr
		}
		return errors.New("connection refused")
	}
	return nil
}

func (s *fakeOwnedMCPSession) ListTools(context.Context, *mcp.ListToolsParams) (*mcp.ListToolsResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.listCalls++
	if s.failAfter > 0 && s.listCalls > s.failAfter {
		if s.listErr != nil {
			return nil, s.listErr
		}
		return nil, errors.New("connection refused")
	}
	toolName := s.toolName
	if toolName == "" {
		toolName = "owned_fake"
	}
	return &mcp.ListToolsResult{Tools: []*mcp.Tool{{Name: toolName}}}, nil
}

func (s *fakeOwnedMCPSession) CallTool(context.Context, *mcp.CallToolParams) (*mcp.CallToolResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.callCalls++
	if s.callErr != nil {
		return nil, s.callErr
	}
	return &mcp.CallToolResult{}, nil
}

func (s *fakeOwnedMCPSession) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closes++
	return nil
}

func (s *fakeOwnedMCPSession) counts() (list, call, closes int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.listCalls, s.callCalls, s.closes
}

func (s *fakeOwnedMCPSession) healthCounts() (ping, list int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.pingCalls, s.listCalls
}

func (s *fakeOwnedMCPSession) setPingDelay(delay time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pingDelay = delay
}

func TestRuntimeOwnerReusesSessionAndClosesIt(t *testing.T) {
	service, environmentID, mcpID := runtimeOwnerTestService(t)
	owner := newRuntimeOwner(service)
	fake := &fakeOwnedMCPSession{}
	connects := 0
	owner.connect = func(context.Context, string, string, map[string]string) (ownedMCPSession, error) {
		connects++
		return fake, nil
	}

	ctx := context.Background()
	for i := 0; i < 2; i++ {
		status, err := owner.Status(ctx, environmentID, mcpID)
		if err != nil {
			t.Fatal(err)
		}
		if status.State != app.MCPHealthHealthy {
			t.Fatalf("status = %+v", status)
		}
	}
	if _, err := owner.ListTools(ctx, environmentID, mcpID); err != nil {
		t.Fatal(err)
	}
	if _, err := owner.CallTool(ctx, environmentID, mcpID, "owned_fake", nil); err != nil {
		t.Fatal(err)
	}
	if connects != 1 {
		t.Fatalf("connects = %d, want 1", connects)
	}
	if info := owner.Info(); info.OwnedMCPSessions != 1 || info.ID == "" || info.PID <= 0 {
		t.Fatalf("owner info = %+v", info)
	}
	if err := owner.Close(); err != nil {
		t.Fatal(err)
	}
	_, _, closes := fake.counts()
	if closes != 1 {
		t.Fatalf("session closes = %d, want 1", closes)
	}
	if err := owner.Close(); err != nil {
		t.Fatal(err)
	}
	_, _, closes = fake.counts()
	if closes != 1 {
		t.Fatalf("idempotent close count = %d, want 1", closes)
	}
}

func TestRuntimeOwnerRestartRebuildsPersistedDesiredState(t *testing.T) {
	service, environmentID, mcpID := runtimeOwnerTestService(t)

	first := newRuntimeOwner(service)
	firstSession := &fakeOwnedMCPSession{}
	firstConnects := 0
	first.connect = func(context.Context, string, string, map[string]string) (ownedMCPSession, error) {
		firstConnects++
		return firstSession, nil
	}
	first.Reconcile(context.Background())
	if firstConnects != 1 || first.Info().OwnedMCPSessions != 1 {
		t.Fatalf("first owner connects=%d info=%+v", firstConnects, first.Info())
	}
	firstID := first.Info().ID
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}

	second := newRuntimeOwner(service)
	secondSession := &fakeOwnedMCPSession{}
	secondConnects := 0
	second.connect = func(context.Context, string, string, map[string]string) (ownedMCPSession, error) {
		secondConnects++
		return secondSession, nil
	}
	second.Reconcile(context.Background())
	defer second.Close()
	if secondConnects != 1 || second.Info().OwnedMCPSessions != 1 {
		t.Fatalf("second owner connects=%d info=%+v", secondConnects, second.Info())
	}
	if second.Info().ID == firstID {
		t.Fatalf("restart reused owner id %q", firstID)
	}
	status, err := second.Status(context.Background(), environmentID, mcpID)
	if err != nil || status.State != app.MCPHealthHealthy {
		t.Fatalf("rebuilt status=%+v err=%v", status, err)
	}
	if secondConnects != 1 {
		t.Fatalf("rebuilt owner reconnected unnecessarily: %d", secondConnects)
	}
}

func TestRuntimeOwnerDeadSessionCannotRemainHealthy(t *testing.T) {
	service, environmentID, mcpID := runtimeOwnerTestService(t)
	owner := newRuntimeOwner(service)
	defer owner.Close()
	first := &fakeOwnedMCPSession{failAfter: 1, listErr: errors.New("connection refused")}
	connects := 0
	owner.connect = func(context.Context, string, string, map[string]string) (ownedMCPSession, error) {
		connects++
		if connects == 1 {
			return first, nil
		}
		return nil, errors.New("connection refused")
	}

	status, err := owner.Status(context.Background(), environmentID, mcpID)
	if err != nil || status.State != app.MCPHealthHealthy {
		t.Fatalf("initial status=%+v err=%v", status, err)
	}
	status, err = owner.Status(context.Background(), environmentID, mcpID)
	if err != nil {
		t.Fatal(err)
	}
	if status.State != app.MCPHealthError || status.ErrorKind != "connection_refused" {
		t.Fatalf("dead session status=%+v", status)
	}
	if owner.Info().OwnedMCPSessions != 0 {
		t.Fatalf("dead session remained owned: %+v", owner.Info())
	}
	_, _, closes := first.counts()
	if closes != 1 {
		t.Fatalf("dead session closes=%d, want 1", closes)
	}
}

func TestRuntimeOwnerDisabledDesiredStateEvictsSession(t *testing.T) {
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
	if _, err := service.SetEnvironmentMCP(environmentID, mcpID, false); err != nil {
		t.Fatal(err)
	}
	status, err := owner.Status(context.Background(), environmentID, mcpID)
	if err != nil {
		t.Fatal(err)
	}
	if status.State != app.MCPHealthDisabled {
		t.Fatalf("disabled status=%+v", status)
	}
	if owner.Info().OwnedMCPSessions != 0 {
		t.Fatalf("disabled session remained owned: %+v", owner.Info())
	}
	_, _, closes := fake.counts()
	if closes != 1 {
		t.Fatalf("disabled session closes=%d, want 1", closes)
	}
}

func TestRuntimeOwnerSharedAcrossIndependentAgentSessions(t *testing.T) {
	service, environmentID, mcpID := runtimeOwnerTestService(t)
	owner := newRuntimeOwner(service)
	defer owner.Close()
	fake := &fakeOwnedMCPSession{}
	connects := 0
	owner.connect = func(context.Context, string, string, map[string]string) (ownedMCPSession, error) {
		connects++
		return fake, nil
	}

	server := newServer(service, owner)
	ctx := context.Background()
	first := connectInMemory(t, ctx, server)
	defer first.Close()
	second := connectInMemory(t, ctx, server)
	defer second.Close()
	for _, session := range []*mcp.ClientSession{first, second} {
		status := callGatewayTool(t, ctx, session, "environment_mcp_status", map[string]any{
			"environment_id": environmentID,
			"mcp_id":         mcpID,
		})
		if status.IsError || !strings.Contains(toolText(t, status), "healthy") {
			t.Fatalf("owner status failed: %s", toolText(t, status))
		}
		info := callGatewayTool(t, ctx, session, "gateway_info", map[string]any{})
		if info.IsError || !strings.Contains(toolText(t, info), owner.Info().ID) {
			t.Fatalf("gateway_info missing shared owner %q: %s", owner.Info().ID, toolText(t, info))
		}
	}
	if connects != 1 {
		t.Fatalf("independent clients created %d owned sessions, want 1", connects)
	}
}

func TestRuntimeOwnerUsesPingForLivenessAndKeepsInventory(t *testing.T) {
	service, environmentID, mcpID := runtimeOwnerTestService(t)
	owner := newRuntimeOwner(service)
	defer owner.Close()
	fake := &fakeOwnedMCPSession{}
	owner.connect = func(context.Context, string, string, map[string]string) (ownedMCPSession, error) { return fake, nil }

	for i := 0; i < 2; i++ {
		if status, err := owner.Status(context.Background(), environmentID, mcpID); err != nil || status.State != app.MCPHealthHealthy {
			t.Fatalf("status=%+v err=%v", status, err)
		}
	}
	pingCalls, listCalls := fake.healthCounts()
	if pingCalls != 2 || listCalls != 1 {
		t.Fatalf("ping calls=%d list calls=%d, want 2/1", pingCalls, listCalls)
	}
	inspection, err := owner.Inspect(environmentID, mcpID)
	if err != nil {
		t.Fatal(err)
	}
	if inspection.Observation.InventoryFetchedAt == nil || len(inspection.Observation.ToolInventory) != 1 || inspection.Observation.ToolInventory[0].Name != "owned_fake" {
		t.Fatalf("inspection=%+v", inspection)
	}
}

func TestRuntimeOwnerAutoReconnectAndDisableInvalidation(t *testing.T) {
	service, environmentID, mcpID := runtimeOwnerPolicyTestService(t)
	owner := newRuntimeOwner(service)
	defer owner.Close()
	fake := &fakeOwnedMCPSession{}
	var connectMu sync.Mutex
	connects := 0
	owner.connect = func(context.Context, string, string, map[string]string) (ownedMCPSession, error) {
		connectMu.Lock()
		connects++
		current := connects
		connectMu.Unlock()
		if current == 1 {
			return nil, errors.New("connection refused")
		}
		return fake, nil
	}
	connectCount := func() int { connectMu.Lock(); defer connectMu.Unlock(); return connects }
	status, err := owner.Status(context.Background(), environmentID, mcpID)
	if err != nil || status.State != app.MCPHealthError {
		t.Fatalf("initial status=%+v err=%v", status, err)
	}
	inspection, err := owner.Inspect(environmentID, mcpID)
	if err != nil || inspection.Observation.NextReconnectAt == nil || inspection.Observation.FailureStage != app.MCPFailureConnect {
		t.Fatalf("failed inspection=%+v err=%v", inspection, err)
	}
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) && owner.Info().OwnedMCPSessions == 0 {
		time.Sleep(20 * time.Millisecond)
	}
	if connectCount() != 2 || owner.Info().OwnedMCPSessions != 1 {
		t.Fatalf("connects=%d info=%+v", connectCount(), owner.Info())
	}
	inspection, _ = owner.Inspect(environmentID, mcpID)
	if inspection.Observation.State != app.MCPHealthHealthy || inspection.Observation.ConsecutiveFailures != 0 {
		t.Fatalf("recovered inspection=%+v", inspection)
	}
	if _, err := service.SetEnvironmentMCP(environmentID, mcpID, false); err != nil {
		t.Fatal(err)
	}
	owner.Drop(environmentID, mcpID)
	connectsAfterDrop := connectCount()
	time.Sleep(1200 * time.Millisecond)
	if connectCount() != connectsAfterDrop || owner.Info().OwnedMCPSessions != 0 {
		t.Fatalf("disabled capability was resurrected: connects=%d info=%+v", connectCount(), owner.Info())
	}
}

func TestRuntimeOwnerPeriodicPingDetectsFailureAndReconnects(t *testing.T) {
	service, environmentID, mcpID := runtimeOwnerPolicyTestService(t)
	owner := newRuntimeOwner(service)
	defer owner.Close()
	unhealthy := &fakeOwnedMCPSession{failAfter: 1, listErr: errors.New("connection refused")}
	recovered := &fakeOwnedMCPSession{}
	var connectMu sync.Mutex
	connects := 0
	owner.connect = func(context.Context, string, string, map[string]string) (ownedMCPSession, error) {
		connectMu.Lock()
		defer connectMu.Unlock()
		connects++
		if connects == 1 {
			return unhealthy, nil
		}
		return recovered, nil
	}
	connectCount := func() int { connectMu.Lock(); defer connectMu.Unlock(); return connects }
	if status, err := owner.Status(context.Background(), environmentID, mcpID); err != nil || status.State != app.MCPHealthHealthy {
		t.Fatalf("initial status=%+v err=%v", status, err)
	}

	deadline := time.Now().Add(3 * time.Second)
	detected := false
	for time.Now().Before(deadline) {
		inspection, _ := owner.Inspect(environmentID, mcpID)
		if inspection.Observation.FailureStage == app.MCPFailurePing && inspection.Observation.State == app.MCPHealthError {
			detected = true
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if !detected {
		inspection, _ := owner.Inspect(environmentID, mcpID)
		t.Fatalf("periodic ping failure was not observed: %+v", inspection.Observation)
	}
	deadline = time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) && connectCount() < 2 {
		time.Sleep(20 * time.Millisecond)
	}
	inspection, _ := owner.Inspect(environmentID, mcpID)
	if connectCount() != 2 || inspection.Observation.State != app.MCPHealthHealthy || owner.Info().OwnedMCPSessions != 1 {
		t.Fatalf("reconnects=%d inspection=%+v info=%+v", connectCount(), inspection.Observation, owner.Info())
	}
}

func TestRuntimeOwnerPolicyUpdateTakesEffectWithoutRestart(t *testing.T) {
	service, environmentID, mcpID := runtimeOwnerTestService(t)
	owner := newRuntimeOwner(service)
	defer owner.Close()
	fakes := []*fakeOwnedMCPSession{{}, {}, {}}
	var connectMu sync.Mutex
	connects := 0
	owner.connect = func(context.Context, string, string, map[string]string) (ownedMCPSession, error) {
		connectMu.Lock()
		defer connectMu.Unlock()
		if connects >= len(fakes) {
			return nil, errors.New("unexpected extra reconnect")
		}
		fake := fakes[connects]
		connects++
		return fake, nil
	}
	session := connectInMemory(t, context.Background(), newServerForSurface(service, owner, serverSurfaceAdmin))
	defer session.Close()

	updatePolicy := func(enabled bool, interval int64) {
		t.Helper()
		result := callGatewayTool(t, context.Background(), session, "mcp_update", map[string]any{
			"id":        mcpID,
			"name":      "owned",
			"transport": catalog.MCPTransportStreamableHTTP,
			"auth_mode": catalog.MCPAuthNone,
			"endpoint":  "http://127.0.0.1:65534/mcp",
			"health_policy": map[string]any{
				"health_check_enabled":   enabled,
				"check_interval_seconds": interval,
				"probe_timeout_seconds": func() int64 {
					if enabled {
						return 1
					}
					return 0
				}(),
				"auto_reconnect": false,
			},
		})
		if result.IsError {
			t.Fatalf("mcp_update failed: %s", toolText(t, result))
		}
	}
	status := func() {
		t.Helper()
		result := callGatewayTool(t, context.Background(), session, "environment_mcp_status", map[string]any{"environment_id": environmentID, "mcp_id": mcpID})
		if result.IsError || !strings.Contains(toolText(t, result), "healthy") {
			t.Fatalf("status failed: %s", toolText(t, result))
		}
	}

	updatePolicy(true, 3)
	status()
	time.Sleep(1200 * time.Millisecond)
	if pings, _ := fakes[0].healthCounts(); pings != 1 {
		t.Fatalf("3s health interval produced background ping early: %d", pings)
	}

	updatePolicy(true, 1)
	_, _, closes := fakes[0].counts()
	if closes != 1 {
		t.Fatalf("policy update did not invalidate old session: closes=%d", closes)
	}
	status()
	deadline := time.Now().Add(2500 * time.Millisecond)
	for time.Now().Before(deadline) {
		if pings, _ := fakes[1].healthCounts(); pings > 1 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if pings, _ := fakes[1].healthCounts(); pings <= 1 {
		t.Fatalf("1s updated health interval did not schedule a background ping: %d", pings)
	}

	updatePolicy(false, 0)
	status()
	time.Sleep(1200 * time.Millisecond)
	if pings, _ := fakes[2].healthCounts(); pings != 1 {
		t.Fatalf("disabled health check still produced background ping: %d", pings)
	}
}

func TestRuntimeOwnerAutoReconnectFalseRequiresExplicitRecovery(t *testing.T) {
	service, environmentID, mcpID := runtimeOwnerTestService(t)
	if _, err := service.MCPs.UpdateMCPConfig(mcpID, "owned", catalog.MCPConfig{
		Transport: catalog.MCPTransportStreamableHTTP,
		AuthMode:  catalog.MCPAuthNone,
		Endpoint:  "http://127.0.0.1:65534/mcp",
		HealthPolicy: model.MCPHealthPolicy{
			AutoReconnect: false,
		},
	}); err != nil {
		t.Fatal(err)
	}
	owner := newRuntimeOwner(service)
	defer owner.Close()
	fake := &fakeOwnedMCPSession{}
	var connectMu sync.Mutex
	connects := 0
	owner.connect = func(context.Context, string, string, map[string]string) (ownedMCPSession, error) {
		connectMu.Lock()
		defer connectMu.Unlock()
		connects++
		if connects == 1 {
			return nil, errors.New("connection refused")
		}
		return fake, nil
	}
	connectCount := func() int { connectMu.Lock(); defer connectMu.Unlock(); return connects }
	status, err := owner.Status(context.Background(), environmentID, mcpID)
	if err != nil || status.State != app.MCPHealthError {
		t.Fatalf("initial status=%+v err=%v", status, err)
	}
	time.Sleep(1200 * time.Millisecond)
	if connectCount() != 1 {
		t.Fatalf("auto_reconnect=false triggered background reconnects: %d", connectCount())
	}
	observation, err := owner.Refresh(context.Background(), environmentID, mcpID)
	if err != nil || observation.State != app.MCPHealthHealthy || connectCount() != 2 {
		t.Fatalf("explicit refresh observation=%+v connects=%d err=%v", observation, connectCount(), err)
	}
}

func TestRuntimeOwnerProbeTimeoutBoundsBackgroundPing(t *testing.T) {
	service, environmentID, mcpID := runtimeOwnerTestService(t)
	if _, err := service.MCPs.UpdateMCPConfig(mcpID, "owned", catalog.MCPConfig{
		Transport: catalog.MCPTransportStreamableHTTP,
		AuthMode:  catalog.MCPAuthNone,
		Endpoint:  "http://127.0.0.1:65534/mcp",
		HealthPolicy: model.MCPHealthPolicy{
			HealthCheckEnabled:   true,
			CheckIntervalSeconds: 1,
			ProbeTimeoutSeconds:  1,
		},
	}); err != nil {
		t.Fatal(err)
	}
	owner := newRuntimeOwner(service)
	defer owner.Close()
	fake := &fakeOwnedMCPSession{}
	owner.connect = func(context.Context, string, string, map[string]string) (ownedMCPSession, error) { return fake, nil }
	if status, err := owner.Status(context.Background(), environmentID, mcpID); err != nil || status.State != app.MCPHealthHealthy {
		t.Fatalf("initial status=%+v err=%v", status, err)
	}
	fake.setPingDelay(3 * time.Second)
	deadline := time.Now().Add(4 * time.Second)
	for time.Now().Before(deadline) {
		inspection, _ := owner.Inspect(environmentID, mcpID)
		if inspection.Observation.FailureStage == app.MCPFailurePing && inspection.Observation.ErrorKind == "timeout" {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	inspection, _ := owner.Inspect(environmentID, mcpID)
	t.Fatalf("probe timeout was not observed: %+v", inspection.Observation)
}

func TestRuntimeOwnerAllowsOnlyOneRecoveryInFlight(t *testing.T) {
	service, environmentID, mcpID := runtimeOwnerTestService(t)
	owner := newRuntimeOwner(service)
	defer owner.Close()
	fake := &fakeOwnedMCPSession{}
	release := make(chan struct{})
	var releaseOnce sync.Once
	releaseConnect := func() { releaseOnce.Do(func() { close(release) }) }
	defer releaseConnect()
	var connectMu sync.Mutex
	connects := 0
	owner.connect = func(ctx context.Context, _ string, _ string, _ map[string]string) (ownedMCPSession, error) {
		connectMu.Lock()
		connects++
		connectMu.Unlock()
		select {
		case <-release:
			return fake, nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	connectCount := func() int { connectMu.Lock(); defer connectMu.Unlock(); return connects }
	key := runtimeOwnerKey{environmentID: environmentID, mcpID: mcpID}
	policy := model.MCPHealthPolicy{ProbeTimeoutSeconds: 5}
	for i := 0; i < 8; i++ {
		go owner.backgroundReconnect(key, policy)
	}
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) && connectCount() == 0 {
		time.Sleep(10 * time.Millisecond)
	}
	if connectCount() != 1 {
		t.Fatalf("parallel recovery started %d connects, want 1", connectCount())
	}
	time.Sleep(150 * time.Millisecond)
	if connectCount() != 1 {
		t.Fatalf("recovery in flight allowed duplicate connects: %d", connectCount())
	}
	inspection, err := owner.Inspect(environmentID, mcpID)
	if err != nil || !inspection.Observation.InFlight {
		t.Fatalf("in-flight recovery observation=%+v err=%v", inspection.Observation, err)
	}
	releaseConnect()
	deadline = time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		inspection, _ = owner.Inspect(environmentID, mcpID)
		if !inspection.Observation.InFlight && owner.Info().OwnedMCPSessions == 1 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("recovery did not settle: inspection=%+v info=%+v", inspection.Observation, owner.Info())
}

func TestGatewayImportUpdateByNameInvalidatesOwnedSession(t *testing.T) {
	service, environmentID, mcpID := runtimeOwnerTestService(t)
	owner := newRuntimeOwner(service)
	defer owner.Close()
	fake := &fakeOwnedMCPSession{}
	owner.connect = func(context.Context, string, string, map[string]string) (ownedMCPSession, error) { return fake, nil }
	if status, err := owner.Status(context.Background(), environmentID, mcpID); err != nil || status.State != app.MCPHealthHealthy {
		t.Fatalf("initial status=%+v err=%v", status, err)
	}
	if owner.Info().OwnedMCPSessions != 1 {
		t.Fatalf("initial owner info=%+v", owner.Info())
	}

	session := connectInMemory(t, context.Background(), newServerForSurface(service, owner, serverSurfaceAdmin))
	defer session.Close()
	content := `{"owned":{"type":"http","url":"http://127.0.0.1:65533/mcp"}}`
	result := callGatewayTool(t, context.Background(), session, "mcp_import_apply", map[string]any{
		"format":          app.MCPImportCodexPlugin,
		"json_or_jsonc":   content,
		"selected_names":  []string{"owned"},
		"conflict_policy": catalog.MCPConflictUpdateByName,
	})
	if result.IsError {
		t.Fatalf("import update failed: %s", toolText(t, result))
	}
	_, _, closes := fake.counts()
	if closes != 1 || owner.Info().OwnedMCPSessions != 0 {
		t.Fatalf("import update did not invalidate owned session: closes=%d info=%+v", closes, owner.Info())
	}
	updated, err := service.MCPs.Get(mcpID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.ID != mcpID || updated.Endpoint != "http://127.0.0.1:65533/mcp" {
		t.Fatalf("updated definition=%+v", updated)
	}
	environment, err := service.Environments.Get(environmentID)
	if err != nil {
		t.Fatal(err)
	}
	if len(environment.EnabledMCPIDs) != 1 || environment.EnabledMCPIDs[0] != mcpID {
		t.Fatalf("import update changed Environment selection: %+v", environment.EnabledMCPIDs)
	}
}

func TestRuntimeOwnerReconcilesDefinitionUpdatesWithoutHandlerNotification(t *testing.T) {
	service, environmentID, mcpID := runtimeOwnerTestService(t)
	owner := newRuntimeOwner(service)
	defer owner.Close()
	first := &fakeOwnedMCPSession{}
	second := &fakeOwnedMCPSession{}
	connects := 0
	var endpoints []string
	owner.connect = func(_ context.Context, _ string, endpoint string, _ map[string]string) (ownedMCPSession, error) {
		endpoints = append(endpoints, endpoint)
		connects++
		if connects == 1 {
			return first, nil
		}
		return second, nil
	}
	if status, err := owner.Status(context.Background(), environmentID, mcpID); err != nil || status.State != app.MCPHealthHealthy {
		t.Fatalf("initial status=%+v err=%v", status, err)
	}

	if _, err := service.MCPs.UpdateMCPConfig(mcpID, "owned", catalog.MCPConfig{
		Transport: catalog.MCPTransportStreamableHTTP,
		AuthMode:  catalog.MCPAuthNone,
		Endpoint:  "http://127.0.0.1:65532/mcp",
	}); err != nil {
		t.Fatal(err)
	}
	if status, err := owner.Status(context.Background(), environmentID, mcpID); err != nil || status.State != app.MCPHealthHealthy {
		t.Fatalf("updated status=%+v err=%v", status, err)
	}
	_, _, firstCloses := first.counts()
	if firstCloses != 1 || len(endpoints) != 2 || endpoints[1] != "http://127.0.0.1:65532/mcp" {
		t.Fatalf("explicit reconciliation closes=%d endpoints=%v", firstCloses, endpoints)
	}

	if _, err := service.MCPs.UpdateMCPConfig(mcpID, "owned", catalog.MCPConfig{
		Transport: catalog.MCPTransportStreamableHTTP,
		AuthMode:  catalog.MCPAuthNone,
		Endpoint:  "http://127.0.0.1:65531/mcp",
	}); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		_, _, closes := second.counts()
		if closes == 1 && owner.Info().OwnedMCPSessions == 0 {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	_, _, closes := second.counts()
	t.Fatalf("monitor did not invalidate externally updated definition: closes=%d info=%+v", closes, owner.Info())
}

func TestRuntimeOwnerToolFailureIsNeverReplayed(t *testing.T) {
	service, environmentID, mcpID := runtimeOwnerTestService(t)
	owner := newRuntimeOwner(service)
	defer owner.Close()
	fake := &fakeOwnedMCPSession{callErr: errors.New("protocol failure")}
	owner.connect = func(context.Context, string, string, map[string]string) (ownedMCPSession, error) { return fake, nil }

	if _, err := owner.CallTool(context.Background(), environmentID, mcpID, "owned_fake", nil); err == nil {
		t.Fatal("expected tool failure")
	}
	_, calls, _ := fake.counts()
	if calls != 1 {
		t.Fatalf("tool calls=%d, want exactly one", calls)
	}
	inspection, _ := owner.Inspect(environmentID, mcpID)
	if inspection.Observation.FailureStage != app.MCPFailureCall || inspection.Observation.State != app.MCPHealthError {
		t.Fatalf("call failure observation=%+v", inspection.Observation)
	}
}

func TestRuntimeOwnerInspectAndRefreshTools(t *testing.T) {
	service, environmentID, mcpID := runtimeOwnerTestService(t)
	owner := newRuntimeOwner(service)
	defer owner.Close()
	first := &fakeOwnedMCPSession{toolName: "owned_first"}
	second := &fakeOwnedMCPSession{toolName: "owned_second"}
	connects := 0
	owner.connect = func(context.Context, string, string, map[string]string) (ownedMCPSession, error) {
		connects++
		if connects == 1 {
			return first, nil
		}
		return second, nil
	}
	session := connectInMemory(t, context.Background(), newServer(service, owner))
	defer session.Close()

	refreshed := callGatewayTool(t, context.Background(), session, "environment_mcp_refresh", map[string]any{"environment_id": environmentID, "mcp_id": mcpID})
	if refreshed.IsError || !strings.Contains(toolText(t, refreshed), "owned_first") {
		t.Fatalf("refresh=%s", toolText(t, refreshed))
	}
	inspected := callGatewayTool(t, context.Background(), session, "environment_mcp_inspect", map[string]any{"environment_id": environmentID, "mcp_id": mcpID})
	if inspected.IsError || !strings.Contains(toolText(t, inspected), `"state":"healthy"`) || !strings.Contains(toolText(t, inspected), `"health_policy"`) {
		t.Fatalf("inspect=%s", toolText(t, inspected))
	}
	refreshed = callGatewayTool(t, context.Background(), session, "environment_mcp_refresh", map[string]any{"environment_id": environmentID, "mcp_id": mcpID})
	secondText := toolText(t, refreshed)
	if refreshed.IsError || connects != 2 || !strings.Contains(secondText, "owned_second") || strings.Contains(secondText, "owned_first") {
		t.Fatalf("second refresh connects=%d result=%s", connects, secondText)
	}
	_, _, closes := first.counts()
	if closes != 1 {
		t.Fatalf("stale session closes=%d, want 1", closes)
	}
}

func runtimeOwnerTestService(t *testing.T) (*app.Service, string, string) {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "marker.txt"), []byte("owner\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	service := app.New(filepath.Join(t.TempDir(), "state.json"))
	workspace, err := service.Workspaces.Add(root, "owner-test")
	if err != nil {
		t.Fatal(err)
	}
	entry, err := service.MCPs.AddMCP("owned", "http://127.0.0.1:65534/mcp", false)
	if err != nil {
		t.Fatal(err)
	}
	environment, err := service.Environments.Create(workspace.ID, "owner-test", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.SetEnvironmentMCP(environment.ID, entry.ID, true); err != nil {
		t.Fatal(err)
	}
	return service, environment.ID, entry.ID
}

func runtimeOwnerPolicyTestService(t *testing.T) (*app.Service, string, string) {
	t.Helper()
	root := t.TempDir()
	service := app.New(filepath.Join(t.TempDir(), "state.json"))
	workspace, err := service.Workspaces.Add(root, "owner-policy-test")
	if err != nil {
		t.Fatal(err)
	}
	entry, err := service.MCPs.AddMCPConfig("owned-policy", catalog.MCPConfig{
		Transport: catalog.MCPTransportStreamableHTTP,
		AuthMode:  catalog.MCPAuthNone,
		Endpoint:  "http://127.0.0.1:65534/mcp",
		HealthPolicy: model.MCPHealthPolicy{
			HealthCheckEnabled:       true,
			CheckIntervalSeconds:     1,
			ProbeTimeoutSeconds:      1,
			AutoReconnect:            true,
			ReconnectIntervalSeconds: 1,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	environment, err := service.Environments.Create(workspace.ID, "owner-policy-test", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.SetEnvironmentMCP(environment.ID, entry.ID, true); err != nil {
		t.Fatal(err)
	}
	return service, environment.ID, entry.ID
}
