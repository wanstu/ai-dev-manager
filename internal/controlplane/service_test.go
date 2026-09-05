package controlplane

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"ai-dev-manager/internal/adapter/runtimeadapter"
	"ai-dev-manager/internal/model"
	"ai-dev-manager/internal/workspace"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestServiceInspectBuildsNativeFromPersistedWorkspaceAndKeepsSafeSources(t *testing.T) {
	service, err := New(t.TempDir())
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	enabled := true
	disabled := false
	userCfg, err := service.Store().LoadUserConfig()
	if err != nil {
		t.Fatalf("LoadUserConfig() error = %v", err)
	}
	userCfg.Global = model.ConfigLayer{
		Scope: model.ScopeGlobal,
		MCPs: map[string]model.MCPDefinition{
			"shared": {
				ID: "shared", Enabled: &enabled, Transport: "stdio", Command: "mcp-server",
				Env: map[string]string{"PUBLIC_CONFIG": "fixture-env-value"}, EnvRefs: map[string]string{"CONFIG_REF": "ADM_TEST_ENV"},
			},
		},
		Skills: map[string]model.SkillDefinition{
			"global-skill": {ID: "global-skill", Enabled: &enabled, Path: "global/skill"},
		},
	}
	if err := service.Store().SaveUserConfig(userCfg); err != nil {
		t.Fatalf("SaveUserConfig() error = %v", err)
	}
	if err := service.Store().SaveProfile("dev", model.ConfigLayer{
		Scope:  model.ScopeProfile,
		MCPs:   map[string]model.MCPDefinition{},
		Skills: map[string]model.SkillDefinition{},
		Policy: &model.Policy{Mode: "workspace-write"},
	}); err != nil {
		t.Fatalf("SaveProfile() error = %v", err)
	}

	workspaceRoot := t.TempDir()
	ws, err := service.Registry().Add(workspace.Input{Path: workspaceRoot, ProfileID: "dev"})
	if err != nil {
		t.Fatalf("Registry.Add() error = %v", err)
	}
	if err := service.Store().SaveProject(workspaceRoot, model.ConfigLayer{
		Scope: model.ScopeProject,
		MCPs: map[string]model.MCPDefinition{
			"shared": {ID: "shared", Enabled: &disabled},
		},
		Skills: map[string]model.SkillDefinition{},
		Verifiers: map[string]model.VerifierDefinition{
			"test": {ID: "test", Kind: "test", Enabled: &enabled, Executable: "go", Args: []string{"test", "./..."}},
		},
	}); err != nil {
		t.Fatalf("SaveProject() error = %v", err)
	}

	snapshot, err := service.Inspect(ws.ID, nil)
	if err != nil {
		t.Fatalf("Inspect() error = %v", err)
	}
	if snapshot.Workspace.RuntimeID != NativeRuntimeID || snapshot.Runtime.RuntimeID != NativeRuntimeID {
		t.Fatalf("runtime selection = workspace:%q runtime:%q, want native", snapshot.Workspace.RuntimeID, snapshot.Runtime.RuntimeID)
	}
	if snapshot.Runtime.PolicyMode != "workspace-write" || snapshot.Runtime.PolicySource != model.ScopeProfile {
		t.Fatalf("policy snapshot = %+v, want profile workspace-write", snapshot.Runtime)
	}
	if !contains(snapshot.Runtime.Capabilities, "files.write") || contains(snapshot.Runtime.Capabilities, "shell.exec") {
		t.Fatalf("capabilities = %v, want write without exec", snapshot.Runtime.Capabilities)
	}
	if len(snapshot.MCPs) != 1 {
		t.Fatalf("MCP snapshots = %+v", snapshot.MCPs)
	}
	mcpSnapshot := snapshot.MCPs[0]
	if mcpSnapshot.ID != "shared" || mcpSnapshot.Enabled == nil || *mcpSnapshot.Enabled || mcpSnapshot.Source != model.ScopeGlobal || mcpSnapshot.EnabledSource != model.ScopeProject {
		t.Fatalf("MCP snapshot = %+v, want global body disabled by project", mcpSnapshot)
	}
	if len(mcpSnapshot.EnvRefNames) != 1 || mcpSnapshot.EnvRefNames[0] != "CONFIG_REF" {
		t.Fatalf("EnvRefNames = %v", mcpSnapshot.EnvRefNames)
	}
	if len(snapshot.Skills) != 1 || snapshot.Skills[0].Source != model.ScopeGlobal {
		t.Fatalf("Skill snapshots = %+v", snapshot.Skills)
	}
	if len(snapshot.Verifiers) != 1 || snapshot.Verifiers[0].Source != model.ScopeProject {
		t.Fatalf("Verifier snapshots = %+v", snapshot.Verifiers)
	}

	encoded, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatalf("Marshal(snapshot) error = %v", err)
	}
	text := string(encoded)
	if strings.Contains(text, "fixture-env-value") || strings.Contains(text, "ADM_TEST_ENV") {
		t.Fatalf("snapshot leaked configured env values/references: %s", text)
	}

	adapter, err := service.BuildRuntime(ws.ID, nil)
	if err != nil {
		t.Fatalf("BuildRuntime() error = %v", err)
	}
	if adapter.ID() != NativeRuntimeID+":"+ws.ID || adapter.WorkspaceID() != ws.ID {
		t.Fatalf("adapter identity = id:%q workspace:%q", adapter.ID(), adapter.WorkspaceID())
	}
}

func TestServiceRejectsUnsupportedRuntimeSelection(t *testing.T) {
	service, err := New(t.TempDir())
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	ws, err := service.Registry().Add(workspace.Input{Path: t.TempDir(), RuntimeID: "devspace"})
	if err != nil {
		t.Fatalf("Registry.Add() error = %v", err)
	}
	_, err = service.BuildRuntime(ws.ID, nil)
	var controlErr *Error
	if !errors.As(err, &controlErr) || controlErr.Kind != ErrUnsupportedRuntime || controlErr.RuntimeID != "devspace" {
		t.Fatalf("BuildRuntime() error = %#v, want unsupported devspace", err)
	}
}

func TestServiceRunsTwoPersistedWorkspaceMCPInstances(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	service, err := New(t.TempDir())
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	rootA := t.TempDir()
	rootB := t.TempDir()
	if err := os.WriteFile(filepath.Join(rootA, "value.txt"), []byte("workspace-A"), 0o644); err != nil {
		t.Fatalf("WriteFile(A) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(rootB, "value.txt"), []byte("workspace-B"), 0o644); err != nil {
		t.Fatalf("WriteFile(B) error = %v", err)
	}
	wsA, err := service.Registry().Add(workspace.Input{Path: rootA})
	if err != nil {
		t.Fatalf("Registry.Add(A) error = %v", err)
	}
	wsB, err := service.Registry().Add(workspace.Input{Path: rootB, RuntimeID: NativeRuntimeID})
	if err != nil {
		t.Fatalf("Registry.Add(B) error = %v", err)
	}
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cleanupCancel()
		_ = service.StopAll(cleanupCtx)
	})

	instanceA, err := service.StartMCP("instance-a", wsA.ID, nil, "")
	if err != nil {
		t.Fatalf("StartMCP(A) error = %v", err)
	}
	instanceB, err := service.StartMCP("instance-b", wsB.ID, nil, "")
	if err != nil {
		t.Fatalf("StartMCP(B) error = %v", err)
	}
	if len(service.ListMCP()) != 2 {
		t.Fatalf("ListMCP() = %+v", service.ListMCP())
	}

	sessionA := connectHTTP(t, ctx, instanceA.Endpoint)
	defer sessionA.Close()
	sessionB := connectHTTP(t, ctx, instanceB.Endpoint)
	defer sessionB.Close()
	if got := callRead(t, ctx, sessionA, "value.txt"); !strings.Contains(got, "workspace-A") {
		t.Fatalf("A read = %s", got)
	}
	if got := callRead(t, ctx, sessionB, "value.txt"); !strings.Contains(got, "workspace-B") {
		t.Fatalf("B read = %s", got)
	}

	outside, err := sessionA.CallTool(ctx, &mcp.CallToolParams{Name: "read", Arguments: map[string]any{"path": filepath.Join(rootB, "value.txt")}})
	if err != nil {
		t.Fatalf("cross-workspace transport error = %v", err)
	}
	if !outside.IsError {
		t.Fatalf("cross-workspace read unexpectedly succeeded: %+v", outside)
	}

	if err := service.StopMCP(ctx, "instance-a"); err != nil {
		t.Fatalf("StopMCP(A) error = %v", err)
	}
	if _, exists := service.GetMCP("instance-a"); exists {
		t.Fatal("instance A still registered after stop")
	}
	if got := callRead(t, ctx, sessionB, "value.txt"); !strings.Contains(got, "workspace-B") {
		t.Fatalf("B stopped working after A stop: %s", got)
	}
}

func TestServiceConfiguredMCPActivationHonorsWorkspaceDisable(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	service, err := New(t.TempDir())
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	fixtureWS, err := service.Registry().Add(workspace.Input{Path: t.TempDir()})
	if err != nil {
		t.Fatalf("Registry.Add(fixture) error = %v", err)
	}
	upstream, err := service.StartMCP("configured-upstream", fixtureWS.ID, nil, "")
	if err != nil {
		t.Fatalf("StartMCP(upstream) error = %v", err)
	}
	defer service.StopAll(ctx)

	enabled := true
	disabled := false
	userCfg, err := service.Store().LoadUserConfig()
	if err != nil {
		t.Fatalf("LoadUserConfig() error = %v", err)
	}
	userCfg.Global.MCPs["shared-http"] = model.MCPDefinition{
		ID: "shared-http", Enabled: &enabled, Transport: "streamable-http", URL: upstream.Endpoint,
	}
	if err := service.Store().SaveUserConfig(userCfg); err != nil {
		t.Fatalf("SaveUserConfig() error = %v", err)
	}
	wsA, err := service.Registry().Add(workspace.Input{Path: t.TempDir()})
	if err != nil {
		t.Fatalf("Registry.Add(A) error = %v", err)
	}
	wsB, err := service.Registry().Add(workspace.Input{Path: t.TempDir()})
	if err != nil {
		t.Fatalf("Registry.Add(B) error = %v", err)
	}
	if err := service.Store().SaveProject(wsA.Path, model.ConfigLayer{
		Scope:  model.ScopeProject,
		MCPs:   map[string]model.MCPDefinition{"shared-http": {ID: "shared-http", Enabled: &disabled}},
		Skills: map[string]model.SkillDefinition{},
	}); err != nil {
		t.Fatalf("SaveProject(A) error = %v", err)
	}

	statusA, err := service.MCPStatuses(wsA.ID, nil)
	if err != nil {
		t.Fatalf("MCPStatuses(A) error = %v", err)
	}
	if len(statusA) != 1 || statusA[0].Health != "disabled" || statusA[0].EnabledSource != model.ScopeProject {
		t.Fatalf("A status = %+v", statusA)
	}
	activatedA, err := service.ActivateConfiguredMCPs(ctx, wsA.ID, nil)
	if err != nil || len(activatedA) != 0 {
		t.Fatalf("ActivateConfiguredMCPs(A) = %+v, err=%v; disabled entry should not start", activatedA, err)
	}

	activatedB, err := service.ActivateConfiguredMCPs(ctx, wsB.ID, nil)
	if err != nil {
		t.Fatalf("ActivateConfiguredMCPs(B) error = %v, status=%+v", err, activatedB)
	}
	if len(activatedB) != 1 || activatedB[0].Health != "healthy" || !contains(activatedB[0].ToolNames, "runtime_info") {
		t.Fatalf("B activation = %+v", activatedB)
	}
	statusB, err := service.MCPStatuses(wsB.ID, nil)
	if err != nil || len(statusB) != 1 || statusB[0].Health != "healthy" {
		t.Fatalf("B status = %+v, err=%v", statusB, err)
	}
	if err := service.StopConfiguredMCPs(wsB.ID); err != nil {
		t.Fatalf("StopConfiguredMCPs(B) error = %v", err)
	}
}

func TestBuildDerivedRuntimeReusesConfigWithoutPersistingWorkspace(t *testing.T) {
	service, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	baseRoot := t.TempDir()
	derivedRoot := t.TempDir()
	if err := os.WriteFile(filepath.Join(baseRoot, "root.txt"), []byte("base"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(derivedRoot, "root.txt"), []byte("derived"), 0o644); err != nil {
		t.Fatal(err)
	}
	ws, err := service.Registry().Add(workspace.Input{Path: baseRoot})
	if err != nil {
		t.Fatal(err)
	}
	if err := service.Store().SaveProject(baseRoot, model.ConfigLayer{
		Scope:  model.ScopeProject,
		Policy: &model.Policy{Mode: "read-only"},
	}); err != nil {
		t.Fatal(err)
	}

	before, err := service.Registry().List()
	if err != nil {
		t.Fatal(err)
	}
	derived, err := service.BuildDerivedRuntime(ws.ID, "lane:one", derivedRoot)
	if err != nil {
		t.Fatalf("BuildDerivedRuntime() error = %v", err)
	}
	if derived.WorkspaceID() != "lane:one" || !contains(derived.Capabilities(), runtimeadapter.OpRead) {
		t.Fatalf("derived runtime identity/capabilities = id:%q caps:%v", derived.WorkspaceID(), derived.Capabilities())
	}
	output, err := derived.Invoke(context.Background(), runtimeadapter.OpRead, map[string]any{"path": "root.txt"})
	if err != nil {
		t.Fatalf("derived read error = %v", err)
	}
	data, err := json.Marshal(output)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "derived") || strings.Contains(string(data), "base") {
		t.Fatalf("derived runtime read wrong root: %s", data)
	}
	after, err := service.Registry().List()
	if err != nil {
		t.Fatal(err)
	}
	if len(after) != len(before) {
		t.Fatalf("derived runtime persisted registry entry: before=%d after=%d", len(before), len(after))
	}
}

func connectHTTP(t *testing.T, ctx context.Context, endpoint string) *mcp.ClientSession {
	t.Helper()
	client := mcp.NewClient(&mcp.Implementation{Name: "adm-controlplane-test", Version: "v0.2.0"}, nil)
	session, err := client.Connect(ctx, &mcp.StreamableClientTransport{Endpoint: endpoint}, nil)
	if err != nil {
		t.Fatalf("Connect(%s) error = %v", endpoint, err)
	}
	return session
}

func callRead(t *testing.T, ctx context.Context, session *mcp.ClientSession, path string) string {
	t.Helper()
	result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "read", Arguments: map[string]any{"path": path}})
	if err != nil {
		t.Fatalf("CallTool(read %q) error = %v", path, err)
	}
	if result.IsError || len(result.Content) == 0 {
		t.Fatalf("CallTool(read %q) result = %+v", path, result)
	}
	text, ok := result.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("read content type = %T", result.Content[0])
	}
	return text.Text
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
