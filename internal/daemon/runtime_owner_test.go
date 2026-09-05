package daemon

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"ai-dev-manager/internal/controlplane"
	"ai-dev-manager/internal/model"
	"ai-dev-manager/internal/testutil"
	"ai-dev-manager/internal/workspace"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestRuntimeOwnerRunsTwoWorkspacesAndStopsIndependently(t *testing.T) {
	testutil.RequireNetworkAcceptance(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	service, err := controlplane.New(t.TempDir())
	if err != nil {
		t.Fatalf("controlplane.New() error = %v", err)
	}
	defer service.StopAll(ctx)

	rootA := t.TempDir()
	rootB := t.TempDir()
	if err := os.WriteFile(filepath.Join(rootA, "value.txt"), []byte("workspace-a"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(rootB, "value.txt"), []byte("workspace-b"), 0o644); err != nil {
		t.Fatal(err)
	}
	wsA, err := service.Registry().Add(workspace.Input{Path: rootA})
	if err != nil {
		t.Fatalf("Registry.Add(A) error = %v", err)
	}
	wsB, err := service.Registry().Add(workspace.Input{Path: rootB})
	if err != nil {
		t.Fatalf("Registry.Add(B) error = %v", err)
	}

	owner := NewRuntimeOwner(service)
	statusA, err := owner.Start(ctx, wsA.ID)
	if err != nil {
		t.Fatalf("owner.Start(A) error = %v", err)
	}
	statusB, err := owner.Start(ctx, wsB.ID)
	if err != nil {
		t.Fatalf("owner.Start(B) error = %v", err)
	}
	if statusA.State != RuntimeRunning || statusB.State != RuntimeRunning || statusA.MCPHost == nil || statusB.MCPHost == nil {
		t.Fatalf("statuses A=%+v B=%+v", statusA, statusB)
	}
	if statusA.MCPHost.Endpoint == statusB.MCPHost.Endpoint {
		t.Fatalf("workspaces share MCP endpoint %q", statusA.MCPHost.Endpoint)
	}

	repeatedA, err := owner.Start(ctx, wsA.ID)
	if err != nil {
		t.Fatalf("owner.Start(A repeat) error = %v", err)
	}
	if repeatedA.MCPHost == nil || repeatedA.MCPHost.Endpoint != statusA.MCPHost.Endpoint || repeatedA.MCPHost.ID != statusA.MCPHost.ID {
		t.Fatalf("repeat A changed host: first=%+v repeat=%+v", statusA.MCPHost, repeatedA.MCPHost)
	}

	assertMCPRead(t, ctx, statusA.MCPHost.Endpoint, "workspace-a")
	assertMCPRead(t, ctx, statusB.MCPHost.Endpoint, "workspace-b")

	stoppedA, err := owner.Stop(ctx, wsA.ID)
	if err != nil {
		t.Fatalf("owner.Stop(A) error = %v", err)
	}
	if stoppedA.State != RuntimeStopped || stoppedA.DesiredRunning || stoppedA.MCPHost != nil {
		t.Fatalf("stopped A = %+v", stoppedA)
	}
	stillB, err := owner.Status(wsB.ID)
	if err != nil {
		t.Fatalf("owner.Status(B) error = %v", err)
	}
	if stillB.State != RuntimeRunning || stillB.MCPHost == nil || stillB.MCPHost.Endpoint != statusB.MCPHost.Endpoint {
		t.Fatalf("B changed after A stop: %+v", stillB)
	}
	assertMCPRead(t, ctx, stillB.MCPHost.Endpoint, "workspace-b")
}

func TestRuntimeOwnerRetainsConfiguredMCPSession(t *testing.T) {
	testutil.RequireNetworkAcceptance(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	service, err := controlplane.New(t.TempDir())
	if err != nil {
		t.Fatalf("controlplane.New() error = %v", err)
	}
	defer service.StopAll(ctx)

	fixtureWS, err := service.Registry().Add(workspace.Input{Path: t.TempDir()})
	if err != nil {
		t.Fatalf("Registry.Add(fixture) error = %v", err)
	}
	upstream, err := service.StartMCP("owner-configured-upstream", fixtureWS.ID, nil, "")
	if err != nil {
		t.Fatalf("StartMCP(upstream) error = %v", err)
	}
	enabled := true
	cfg, err := service.Store().LoadUserConfig()
	if err != nil {
		t.Fatalf("LoadUserConfig() error = %v", err)
	}
	cfg.Global.MCPs["shared-http"] = model.MCPDefinition{
		ID: "shared-http", Enabled: &enabled, Transport: "streamable-http", URL: upstream.Endpoint,
	}
	if err := service.Store().SaveUserConfig(cfg); err != nil {
		t.Fatalf("SaveUserConfig() error = %v", err)
	}
	target, err := service.Registry().Add(workspace.Input{Path: t.TempDir()})
	if err != nil {
		t.Fatalf("Registry.Add(target) error = %v", err)
	}

	owner := NewRuntimeOwner(service)
	started, err := owner.Start(ctx, target.ID)
	if err != nil {
		t.Fatalf("owner.Start(target) error = %v, status=%+v", err, started)
	}
	if len(started.ConfiguredMCPs) != 1 || started.ConfiguredMCPs[0].Health != "healthy" {
		t.Fatalf("configured MCP activation = %+v", started.ConfiguredMCPs)
	}
	observed, err := owner.Status(target.ID)
	if err != nil {
		t.Fatalf("owner.Status(target) error = %v", err)
	}
	if len(observed.ConfiguredMCPs) != 1 || observed.ConfiguredMCPs[0].Health != "healthy" {
		t.Fatalf("configured MCP was not retained: %+v", observed.ConfiguredMCPs)
	}
}

func TestRuntimeOwnerDesiredStatePersistsAcrossShutdownAndReconcile(t *testing.T) {
	testutil.RequireNetworkAcceptance(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	configRoot := t.TempDir()
	service, err := controlplane.New(configRoot)
	if err != nil {
		t.Fatalf("controlplane.New() error = %v", err)
	}

	wsA, err := service.Registry().Add(workspace.Input{Path: t.TempDir()})
	if err != nil {
		t.Fatalf("Registry.Add(A) error = %v", err)
	}
	wsB, err := service.Registry().Add(workspace.Input{Path: t.TempDir()})
	if err != nil {
		t.Fatalf("Registry.Add(B) error = %v", err)
	}

	owner := NewRuntimeOwner(service)
	if _, err := owner.Start(ctx, wsA.ID); err != nil {
		t.Fatalf("owner.Start(A) error = %v", err)
	}
	if _, err := owner.Start(ctx, wsB.ID); err != nil {
		t.Fatalf("owner.Start(B) error = %v", err)
	}
	ids, err := NewDesiredStore(configRoot).Load()
	if err != nil {
		t.Fatalf("desired.Load() error = %v", err)
	}
	if len(ids) != 2 || ids[0] != wsA.ID && ids[1] != wsA.ID {
		t.Fatalf("desired after start = %v", ids)
	}

	if err := owner.StopAll(ctx); err != nil {
		t.Fatalf("owner.StopAll() error = %v", err)
	}
	ids, err = NewDesiredStore(configRoot).Load()
	if err != nil {
		t.Fatalf("desired.Load(after shutdown) error = %v", err)
	}
	if len(ids) != 2 {
		t.Fatalf("daemon shutdown cleared desired state: %v", ids)
	}
	if err := service.StopAll(ctx); err != nil {
		t.Fatalf("service.StopAll() error = %v", err)
	}

	service2, err := controlplane.New(configRoot)
	if err != nil {
		t.Fatalf("controlplane.New(second) error = %v", err)
	}
	defer service2.StopAll(ctx)
	owner2 := NewRuntimeOwner(service2)
	statuses, err := owner2.Reconcile(ctx)
	if err != nil {
		t.Fatalf("owner2.Reconcile() error = %v", err)
	}
	if len(statuses) != 2 || statuses[0].State != RuntimeRunning || statuses[1].State != RuntimeRunning {
		t.Fatalf("reconciled statuses = %+v", statuses)
	}
	if _, err := owner2.Stop(ctx, wsA.ID); err != nil {
		t.Fatalf("owner2.Stop(A) error = %v", err)
	}
	ids, err = NewDesiredStore(configRoot).Load()
	if err != nil {
		t.Fatalf("desired.Load(after explicit stop) error = %v", err)
	}
	if len(ids) != 1 || ids[0] != wsB.ID {
		t.Fatalf("desired after explicit stop A = %v, want [%s]", ids, wsB.ID)
	}
}

func TestRuntimeOwnerRebindsForDockerAndReconcilesExposure(t *testing.T) {
	testutil.RequireNetworkAcceptance(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	configRoot := t.TempDir()
	service, err := controlplane.New(configRoot)
	if err != nil {
		t.Fatalf("controlplane.New() error = %v", err)
	}
	ws, err := service.Registry().Add(workspace.Input{Path: t.TempDir()})
	if err != nil {
		t.Fatalf("Registry.Add() error = %v", err)
	}
	owner := NewRuntimeOwner(service)
	loopback, err := owner.Start(ctx, ws.ID)
	if err != nil {
		t.Fatalf("owner.Start(loopback) error = %v", err)
	}
	if loopback.MCPHost == nil || loopback.Listen != DefaultRuntimeListen || loopback.Exposed || loopback.DockerEndpoint != "" {
		t.Fatalf("loopback status = %+v", loopback)
	}

	exposed, err := owner.StartWithOptions(ctx, ws.ID, RuntimeStartOptions{Listen: DockerRuntimeListen, Exposed: true})
	if err != nil {
		t.Fatalf("owner.StartWithOptions(docker) error = %v", err)
	}
	if exposed.MCPHost == nil || !strings.HasPrefix(exposed.MCPHost.Address, "0.0.0.0:") || exposed.DockerEndpoint == "" || exposed.LocalEndpoint == "" {
		t.Fatalf("docker status = %+v", exposed)
	}
	if loopback.MCPHost.Endpoint == exposed.MCPHost.Endpoint {
		t.Fatalf("docker rebind reused loopback endpoint %q", exposed.MCPHost.Endpoint)
	}
	if err := owner.StopAll(ctx); err != nil {
		t.Fatalf("owner.StopAll() error = %v", err)
	}
	if err := service.StopAll(ctx); err != nil {
		t.Fatalf("service.StopAll() error = %v", err)
	}

	service2, err := controlplane.New(configRoot)
	if err != nil {
		t.Fatalf("controlplane.New(second) error = %v", err)
	}
	defer service2.StopAll(ctx)
	owner2 := NewRuntimeOwner(service2)
	statuses, err := owner2.Reconcile(ctx)
	if err != nil {
		t.Fatalf("owner2.Reconcile() error = %v", err)
	}
	if len(statuses) != 1 || statuses[0].State != RuntimeRunning || !statuses[0].Exposed || statuses[0].Listen != DockerRuntimeListen || statuses[0].DockerEndpoint == "" {
		t.Fatalf("reconciled docker status = %+v", statuses)
	}
}

func TestRuntimeOwnerReconcileContinuesAfterBrokenDesiredWorkspace(t *testing.T) {
	testutil.RequireNetworkAcceptance(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	configRoot := t.TempDir()
	service, err := controlplane.New(configRoot)
	if err != nil {
		t.Fatalf("controlplane.New() error = %v", err)
	}
	defer service.StopAll(ctx)
	good, err := service.Registry().Add(workspace.Input{Path: t.TempDir()})
	if err != nil {
		t.Fatalf("Registry.Add(good) error = %v", err)
	}
	if err := NewDesiredStore(configRoot).Replace([]string{"ws_missing", good.ID}); err != nil {
		t.Fatalf("desired.Replace() error = %v", err)
	}

	owner := NewRuntimeOwner(service)
	statuses, err := owner.Reconcile(ctx)
	if err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}
	if len(statuses) != 2 {
		t.Fatalf("Reconcile() statuses = %+v", statuses)
	}
	goodStatus, err := owner.Status(good.ID)
	if err != nil || goodStatus.State != RuntimeRunning {
		t.Fatalf("good status = %+v, err=%v", goodStatus, err)
	}
	missingStatus, err := owner.Status("ws_missing")
	if err != nil {
		t.Fatalf("missing desired status error = %v", err)
	}
	if missingStatus.State != RuntimeError || !missingStatus.DesiredRunning || missingStatus.Error == "" {
		t.Fatalf("missing desired status = %+v", missingStatus)
	}
}

func TestRuntimeOwnerRollsBackHostWhenConfiguredMCPActivationFails(t *testing.T) {
	testutil.RequireNetworkAcceptance(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	service, err := controlplane.New(t.TempDir())
	if err != nil {
		t.Fatalf("controlplane.New() error = %v", err)
	}
	defer service.StopAll(ctx)

	enabled := true
	cfg, err := service.Store().LoadUserConfig()
	if err != nil {
		t.Fatalf("LoadUserConfig() error = %v", err)
	}
	cfg.Global.MCPs["broken"] = model.MCPDefinition{ID: "broken", Enabled: &enabled, Transport: "unsupported"}
	if err := service.Store().SaveUserConfig(cfg); err != nil {
		t.Fatalf("SaveUserConfig() error = %v", err)
	}
	ws, err := service.Registry().Add(workspace.Input{Path: t.TempDir()})
	if err != nil {
		t.Fatalf("Registry.Add() error = %v", err)
	}

	owner := NewRuntimeOwner(service)
	status, err := owner.Start(ctx, ws.ID)
	if err == nil {
		t.Fatalf("owner.Start() unexpectedly succeeded: %+v", status)
	}
	if status.State != RuntimeError || !status.DesiredRunning || status.MCPHost != nil || status.Error == "" {
		t.Fatalf("error status = %+v", status)
	}
	if _, ok := service.GetMCP(runtimeHostInstanceID(ws.ID)); ok {
		t.Fatal("runtime host remained after activation failure")
	}
	mcpStatuses, statusErr := service.MCPStatuses(ws.ID, nil)
	if statusErr != nil {
		t.Fatalf("MCPStatuses() error = %v", statusErr)
	}
	if len(mcpStatuses) != 1 || mcpStatuses[0].Health == "healthy" {
		t.Fatalf("broken MCP status = %+v", mcpStatuses)
	}
	ids, desiredErr := NewDesiredStore(service.Store().Root()).Load()
	if desiredErr != nil {
		t.Fatalf("desired.Load() error = %v", desiredErr)
	}
	if len(ids) != 1 || ids[0] != ws.ID {
		t.Fatalf("activation failure did not preserve desired state: %v", ids)
	}
}

func assertMCPRead(t *testing.T, ctx context.Context, endpoint, want string) {
	t.Helper()
	client := mcp.NewClient(&mcp.Implementation{Name: "daemon-owner-test", Version: "v0.3.0"}, nil)
	session, err := client.Connect(ctx, &mcp.StreamableClientTransport{Endpoint: endpoint}, nil)
	if err != nil {
		t.Fatalf("Connect(%s) error = %v", endpoint, err)
	}
	defer session.Close()
	result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "read", Arguments: map[string]any{"path": "value.txt"}})
	if err != nil {
		t.Fatalf("CallTool(read) error = %v", err)
	}
	if result.IsError || len(result.Content) == 0 {
		t.Fatalf("CallTool(read) result = %+v", result)
	}
	text, ok := result.Content[0].(*mcp.TextContent)
	if !ok || !containsText(text.Text, want) {
		t.Fatalf("read content = %+v, want %q", result.Content, want)
	}
}

func containsText(value, target string) bool {
	for i := 0; i+len(target) <= len(value); i++ {
		if value[i:i+len(target)] == target {
			return true
		}
	}
	return false
}
