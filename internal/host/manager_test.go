package host

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"ai-dev-manager/internal/adapter/runtimeadapter"
	"ai-dev-manager/internal/model"
	admruntime "ai-dev-manager/internal/runtime"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestManagerRunsTwoIsolatedWorkspaceMCPInstances(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	rootA := t.TempDir()
	rootB := t.TempDir()
	if err := os.WriteFile(filepath.Join(rootA, "value.txt"), []byte("workspace-A"), 0o644); err != nil {
		t.Fatalf("WriteFile A error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(rootB, "value.txt"), []byte("workspace-B"), 0o644); err != nil {
		t.Fatalf("WriteFile B error = %v", err)
	}

	adapterA := newReadOnlyAdapter(t, "native-a", "ws_a", rootA)
	adapterB := newReadOnlyAdapter(t, "native-b", "ws_b", rootB)
	manager := NewManager()
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cleanupCancel()
		_ = manager.StopAll(cleanupCtx)
	})

	instanceA, err := manager.StartHTTP("instance-a", adapterA, "")
	if err != nil {
		t.Fatalf("StartHTTP(A) error = %v", err)
	}
	instanceB, err := manager.StartHTTP("instance-b", adapterB, "")
	if err != nil {
		t.Fatalf("StartHTTP(B) error = %v", err)
	}
	if instanceA.Endpoint == instanceB.Endpoint || instanceA.Address == instanceB.Address {
		t.Fatalf("instances share endpoint: A=%+v B=%+v", instanceA, instanceB)
	}
	if instanceA.WorkspaceID != "ws_a" || instanceB.WorkspaceID != "ws_b" {
		t.Fatalf("workspace identity lost: A=%+v B=%+v", instanceA, instanceB)
	}

	sessionA := connectHTTP(t, ctx, instanceA.Endpoint)
	defer sessionA.Close()
	sessionB := connectHTTP(t, ctx, instanceB.Endpoint)
	defer sessionB.Close()

	textA := callReadText(t, ctx, sessionA, "value.txt")
	textB := callReadText(t, ctx, sessionB, "value.txt")
	if !strings.Contains(textA, "workspace-A") || strings.Contains(textA, "workspace-B") {
		t.Fatalf("A returned wrong workspace content: %s", textA)
	}
	if !strings.Contains(textB, "workspace-B") || strings.Contains(textB, "workspace-A") {
		t.Fatalf("B returned wrong workspace content: %s", textB)
	}

	outsideResult, err := sessionA.CallTool(ctx, &mcp.CallToolParams{
		Name:      "read",
		Arguments: map[string]any{"path": filepath.Join(rootB, "value.txt")},
	})
	if err != nil {
		t.Fatalf("A cross-workspace CallTool transport error = %v", err)
	}
	if !outsideResult.IsError {
		t.Fatalf("A cross-workspace read unexpectedly succeeded: %+v", outsideResult)
	}

	if err := manager.Stop(ctx, "instance-a"); err != nil {
		t.Fatalf("Stop(A) error = %v", err)
	}
	if err := manager.Stop(ctx, "instance-a"); err != nil {
		t.Fatalf("second Stop(A) should be idempotent: %v", err)
	}

	stillB := callReadText(t, ctx, sessionB, "value.txt")
	if !strings.Contains(stillB, "workspace-B") {
		t.Fatalf("B stopped working after A stopped: %s", stillB)
	}
	if _, exists := manager.Get("instance-a"); exists {
		t.Fatal("stopped instance A still present")
	}
	if _, exists := manager.Get("instance-b"); !exists {
		t.Fatal("instance B disappeared when A stopped")
	}
}

func TestManagerRejectsNonLoopbackAndDuplicateInstanceID(t *testing.T) {
	adapter := newReadOnlyAdapter(t, "native", "ws", t.TempDir())
	manager := NewManager()
	if _, err := manager.StartHTTP("public", adapter, "0.0.0.0:0"); err == nil {
		t.Fatal("non-loopback listen unexpectedly accepted")
	}
	exposed, err := manager.StartHTTPExposed("public-explicit", adapter, "0.0.0.0:0")
	if err != nil {
		t.Fatalf("StartHTTPExposed() error = %v", err)
	}
	if !strings.HasPrefix(exposed.Address, "0.0.0.0:") {
		t.Fatalf("exposed address = %q, want 0.0.0.0:*", exposed.Address)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = manager.Stop(ctx, exposed.ID)
	})
	instance, err := manager.StartHTTP("same", adapter, "")
	if err != nil {
		t.Fatalf("StartHTTP() error = %v", err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = manager.Stop(ctx, instance.ID)
	})
	if _, err := manager.StartHTTP("same", adapter, ""); err == nil {
		t.Fatal("duplicate instance id unexpectedly accepted")
	}
}

func TestManagerHostsExternalRuntimeContract(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	adapter := &fakeRuntime{
		id: "external-1", workspaceID: "ws_external",
		capabilities: []string{"external.echo"},
	}
	manager := NewManager()
	instance, err := manager.StartHTTP("external-instance", adapter, "")
	if err != nil {
		t.Fatalf("StartHTTP(external) error = %v", err)
	}
	defer manager.Stop(ctx, instance.ID)

	session := connectHTTP(t, ctx, instance.Endpoint)
	defer session.Close()
	info, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "runtime_info", Arguments: map[string]any{}})
	if err != nil || info.IsError {
		t.Fatalf("runtime_info external failed: err=%v result=%+v", err, info)
	}
	text := firstText(t, info)
	if !strings.Contains(text, `"id":"external-1"`) || !strings.Contains(text, `"external.echo"`) {
		t.Fatalf("external runtime info missing contract data: %s", text)
	}
	tools, err := session.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("ListTools(external) error = %v", err)
	}
	if len(tools.Tools) != 1 || tools.Tools[0].Name != "runtime_info" {
		t.Fatalf("unknown external capability should not auto-create MCP tool: %+v", tools.Tools)
	}
}

func newReadOnlyAdapter(t *testing.T, runtimeID, workspaceID, root string) runtimeadapter.Runtime {
	t.Helper()
	native, err := admruntime.NewNative(model.Workspace{ID: workspaceID, Path: root}, model.EffectiveConfig{})
	if err != nil {
		t.Fatalf("NewNative() error = %v", err)
	}
	adapter, err := runtimeadapter.NewNative(runtimeID, native)
	if err != nil {
		t.Fatalf("NewNativeAdapter() error = %v", err)
	}
	return adapter
}

func connectHTTP(t *testing.T, ctx context.Context, endpoint string) *mcp.ClientSession {
	t.Helper()
	client := mcp.NewClient(&mcp.Implementation{Name: "adm-http-test", Version: "v0.1.0"}, nil)
	session, err := client.Connect(ctx, &mcp.StreamableClientTransport{Endpoint: endpoint}, nil)
	if err != nil {
		t.Fatalf("HTTP client Connect(%s) error = %v", endpoint, err)
	}
	return session
}

func callReadText(t *testing.T, ctx context.Context, session *mcp.ClientSession, path string) string {
	t.Helper()
	result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "read", Arguments: map[string]any{"path": path}})
	if err != nil {
		t.Fatalf("CallTool(read %q) error = %v", path, err)
	}
	if result.IsError {
		t.Fatalf("CallTool(read %q) tool error: %+v", path, result.Content)
	}
	return firstText(t, result)
}

func firstText(t *testing.T, result *mcp.CallToolResult) string {
	t.Helper()
	if len(result.Content) == 0 {
		t.Fatal("tool result has no content")
	}
	text, ok := result.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("tool content type = %T, want *mcp.TextContent", result.Content[0])
	}
	return text.Text
}

type fakeRuntime struct {
	id           string
	workspaceID  string
	capabilities []string
}

func (f *fakeRuntime) ID() string             { return f.id }
func (f *fakeRuntime) WorkspaceID() string    { return f.workspaceID }
func (f *fakeRuntime) Capabilities() []string { return append([]string(nil), f.capabilities...) }
func (f *fakeRuntime) Status(context.Context) runtimeadapter.Status {
	return runtimeadapter.Status{
		ID: f.id, WorkspaceID: f.workspaceID, State: runtimeadapter.StateReady,
		Capabilities: f.Capabilities(),
	}
}
func (f *fakeRuntime) Invoke(_ context.Context, operation string, input map[string]any) (any, error) {
	if operation == "external.echo" {
		return input, nil
	}
	return nil, &runtimeadapter.AdapterError{Kind: runtimeadapter.ErrUnsupportedOperation, Operation: operation}
}
