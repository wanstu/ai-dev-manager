package mcp

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"ai-dev-manager/internal/adapter/runtimeadapter"
	"ai-dev-manager/internal/host"
	"ai-dev-manager/internal/model"
	admruntime "ai-dev-manager/internal/runtime"
	"ai-dev-manager/internal/testutil"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

const fixtureModeEnv = "ADM_MCP_FIXTURE_MODE"

func TestMain(m *testing.M) {
	if os.Getenv(fixtureModeEnv) == "stdio" {
		runStdioFixture()
		os.Exit(0)
	}
	os.Exit(m.Run())
}

type fixtureInput struct{}
type fixtureOutput struct {
	Present bool `json:"present"`
}

func runStdioFixture() {
	server := sdkmcp.NewServer(&sdkmcp.Implementation{Name: "adm-fixture", Version: "v0.2.0"}, nil)
	if os.Getenv("ADM_CHILD_MARKER") != "" {
		sdkmcp.AddTool(server, &sdkmcp.Tool{Name: "environment_present"}, func(context.Context, *sdkmcp.CallToolRequest, fixtureInput) (*sdkmcp.CallToolResult, fixtureOutput, error) {
			return nil, fixtureOutput{Present: true}, nil
		})
	}
	if err := server.Run(context.Background(), &sdkmcp.StdioTransport{}); err != nil {
		os.Exit(2)
	}
}

func TestActivatorStdioResolvesEnvRefsAndKeepsWorkspaceSessionsIsolated(t *testing.T) {
	executable, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable() error = %v", err)
	}
	enabled := true
	lookup := func(name string) (string, bool) {
		if name == "ADM_HOST_MARKER" {
			return "fixture-reference-value", true
		}
		return "", false
	}
	activator := NewActivator(lookup)
	entry := Entry{
		Definition: model.MCPDefinition{
			ID: "fixture", Enabled: &enabled, Transport: "stdio", Command: executable,
			Env:     map[string]string{fixtureModeEnv: "stdio"},
			EnvRefs: map[string]string{"ADM_CHILD_MARKER": "ADM_HOST_MARKER"},
		},
		Source: model.ScopeGlobal, EnabledSource: model.ScopeGlobal,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	statusA, err := activator.Activate(ctx, "ws_a", entry)
	if err != nil {
		t.Fatalf("Activate(A) error = %v, status=%+v", err, statusA)
	}
	if statusA.Health != HealthHealthy || !containsTool(statusA.ToolNames, "environment_present") {
		t.Fatalf("status A = %+v", statusA)
	}
	statusB, err := activator.Activate(ctx, "ws_b", entry)
	if err != nil {
		t.Fatalf("Activate(B) error = %v, status=%+v", err, statusB)
	}
	if statusB.Health != HealthHealthy || !containsTool(statusB.ToolNames, "environment_present") {
		t.Fatalf("status B = %+v", statusB)
	}
	encoded, _ := json.Marshal([]ActivationStatus{statusA, statusB})
	if strings.Contains(string(encoded), "fixture-reference-value") {
		t.Fatalf("status leaked resolved environment value: %s", encoded)
	}

	if err := activator.Stop("ws_a", "fixture"); err != nil {
		t.Fatalf("Stop(A) error = %v", err)
	}
	if _, ok := activator.Get("ws_a", "fixture"); ok {
		t.Fatal("A session still active")
	}
	if current, ok := activator.Get("ws_b", "fixture"); !ok || current.Health != HealthHealthy {
		t.Fatalf("B session lost after A stop: %+v %v", current, ok)
	}
	if err := activator.StopWorkspace("ws_b"); err != nil {
		t.Fatalf("StopWorkspace(B) error = %v", err)
	}
}

func TestActivatorMissingEnvRefReturnsSafeStructuredError(t *testing.T) {
	enabled := true
	activator := NewActivator(func(string) (string, bool) { return "", false })
	entry := Entry{Definition: model.MCPDefinition{
		ID: "fixture", Enabled: &enabled, Transport: "stdio", Command: filepath.Join(t.TempDir(), "not-used"),
		EnvRefs: map[string]string{"ADM_CHILD_MARKER": "ADM_MISSING_MARKER"},
	}}
	status, err := activator.Activate(context.Background(), "ws", entry)
	if err == nil {
		t.Fatal("Activate() error = nil, want missing env ref")
	}
	activationErr, ok := err.(*ActivationError)
	if !ok || activationErr.Kind != ActivationErrMissingEnvRef || activationErr.RefName != "ADM_MISSING_MARKER" {
		t.Fatalf("error = %#v", err)
	}
	if status.Health != HealthError || status.ErrorKind != ActivationErrMissingEnvRef || strings.Contains(status.Error, "not-used") {
		t.Fatalf("unsafe/error status = %+v", status)
	}
}

func TestActivatorProbesStreamableHTTP(t *testing.T) {
	testutil.RequireNetworkAcceptance(t)
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "value.txt"), []byte("http-fixture"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	native, err := admruntime.NewNative(model.Workspace{ID: "ws_http", Path: root}, model.EffectiveConfig{})
	if err != nil {
		t.Fatalf("NewNative() error = %v", err)
	}
	adapter, err := runtimeadapter.NewNative("native:http", native)
	if err != nil {
		t.Fatalf("NewNativeAdapter() error = %v", err)
	}
	manager := host.NewManager()
	instance, err := manager.StartHTTP("fixture-http", adapter, "")
	if err != nil {
		t.Fatalf("StartHTTP() error = %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	defer manager.Stop(ctx, instance.ID)

	enabled := true
	activator := NewActivator(nil)
	status, err := activator.Activate(ctx, "ws_http", Entry{Definition: model.MCPDefinition{
		ID: "http", Enabled: &enabled, Transport: "streamable-http", URL: instance.Endpoint,
	}})
	if err != nil {
		t.Fatalf("Activate(HTTP) error = %v, status=%+v", err, status)
	}
	if status.Health != HealthHealthy || !containsTool(status.ToolNames, "runtime_info") || !containsTool(status.ToolNames, "read") {
		t.Fatalf("HTTP status = %+v", status)
	}
	if err := activator.Stop("ws_http", "http"); err != nil {
		t.Fatalf("Stop(HTTP) error = %v", err)
	}
}

func containsTool(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
