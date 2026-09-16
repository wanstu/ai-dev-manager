package app

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"ai-dev-manager-v2/internal/catalog"
	"ai-dev-manager-v2/internal/hostenv"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestProbeMCPHealthConfigured(t *testing.T) {
	service := New(filepath.Join(t.TempDir(), "state.json"))
	ws, err := service.Workspaces.Add(t.TempDir(), "configured")
	if err != nil {
		t.Fatal(err)
	}
	entry, err := service.MCPs.AddMCPConfig("configured", catalog.MCPConfig{Endpoint: "", Transport: "streamable-http"})
	if err == nil {
		t.Fatal("expected empty endpoint to be rejected by catalog")
	}

	// Simulate a selected MCP whose global definition was later removed.
	entry, err = service.MCPs.AddMCP("configured", "http://example.test/mcp", false)
	if err != nil {
		t.Fatal(err)
	}
	env, err := service.Environments.Create(ws.ID, "configured", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.SetEnvironmentMCP(env.ID, entry.ID, true); err != nil {
		t.Fatal(err)
	}
	if err := service.MCPs.Remove(entry.ID); err != nil {
		t.Fatal(err)
	}
	// Explicit delete now clears normal Environment selections. Re-introduce a
	// stale legacy reference directly to keep unresolved-health behavior covered.
	if _, err := service.Environments.SetMCP(env.ID, entry.ID, true); err != nil {
		t.Fatal(err)
	}

	status, err := service.ProbeMCPHealth(context.Background(), env.ID, entry.ID)
	if err != nil {
		t.Fatal(err)
	}
	if status.State != MCPHealthConfigured || status.MCPID != entry.ID {
		t.Fatalf("status = %+v; want configured for %s", status, entry.ID)
	}
}

func TestProbeMCPHealthDisabled(t *testing.T) {
	service := New(filepath.Join(t.TempDir(), "state.json"))
	ws, err := service.Workspaces.Add(t.TempDir(), "disabled")
	if err != nil {
		t.Fatal(err)
	}
	entry, err := service.MCPs.AddMCP("disabled", "http://127.0.0.1:1/mcp", false)
	if err != nil {
		t.Fatal(err)
	}
	env, err := service.Environments.Create(ws.ID, "disabled", "")
	if err != nil {
		t.Fatal(err)
	}

	status, err := service.ProbeMCPHealth(context.Background(), env.ID, entry.ID)
	if err != nil {
		t.Fatal(err)
	}
	if status.State != MCPHealthDisabled || status.ErrorKind != "" {
		t.Fatalf("status = %+v; want disabled", status)
	}
}

func TestProbeMCPHealthHealthyAndHeaderExpansion(t *testing.T) {
	t.Setenv("ADM_MCP_TOKEN", "secret-token")

	external := mcp.NewServer(&mcp.Implementation{Name: "health-test", Version: "dev"}, nil)
	mcp.AddTool(external, &mcp.Tool{Name: "ping", Description: "pong"},
		func(context.Context, *mcp.CallToolRequest, struct{}) (*mcp.CallToolResult, any, error) {
			return nil, map[string]any{"pong": true}, nil
		})
	base := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return external }, &mcp.StreamableHTTPOptions{Stateless: true, DisableLocalhostProtection: true})
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer secret-token" {
			t.Fatalf("Authorization = %q; want expanded secret header", got)
		}
		base.ServeHTTP(w, r)
	})
	server := httptest.NewServer(handler)
	defer server.Close()

	service := New(filepath.Join(t.TempDir(), "state.json"))
	ws, err := service.Workspaces.Add(t.TempDir(), "healthy")
	if err != nil {
		t.Fatal(err)
	}
	entry, err := service.MCPs.AddMCPConfig("healthy", catalog.MCPConfig{
		Endpoint:   server.URL,
		Transport:  "streamable-http",
		AuthMode:   catalog.MCPAuthHeaders,
		HeaderRefs: map[string]string{"Authorization": "Bearer ${ADM_MCP_TOKEN}"},
	})
	if err != nil {
		t.Fatal(err)
	}
	env, err := service.Environments.Create(ws.ID, "healthy", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.SetEnvironmentMCP(env.ID, entry.ID, true); err != nil {
		t.Fatal(err)
	}

	status, err := service.ProbeMCPHealth(context.Background(), env.ID, entry.ID)
	if err != nil {
		t.Fatal(err)
	}
	if status.State != MCPHealthHealthy || status.ErrorKind != "" || strings.Contains(status.Message, "secret-token") {
		t.Fatalf("status = %+v; want healthy without secret leakage", status)
	}
}

func TestProbeMCPHealthConnectionRefused(t *testing.T) {
	service, envID, mcpID := configuredEnabledMCP(t, "http://127.0.0.1:1/mcp")
	status, err := service.ProbeMCPHealth(context.Background(), envID, mcpID)
	if err != nil {
		t.Fatal(err)
	}
	if status.State != MCPHealthError || status.ErrorKind != "connection_refused" || !strings.Contains(status.Message, "Endpoint") || !strings.Contains(status.Message, "服务已启动") {
		t.Fatalf("status = %+v; want actionable connection_refused", status)
	}
}

func TestProbeMCPHealthUnresolvedSecretRefStaysConfigured(t *testing.T) {
	const missingEnv = "ADM_MCP_TEST_MISSING_TOKEN_7F3A"
	_ = os.Unsetenv(missingEnv)

	service := New(filepath.Join(t.TempDir(), "state.json"))
	ws, err := service.Workspaces.Add(t.TempDir(), "unresolved")
	if err != nil {
		t.Fatal(err)
	}
	entry, err := service.MCPs.AddMCPConfig("unresolved", catalog.MCPConfig{
		Endpoint:   "http://127.0.0.1:1/mcp",
		Transport:  "streamable-http",
		AuthMode:   catalog.MCPAuthHeaders,
		HeaderRefs: map[string]string{"Authorization": "Bearer ${" + missingEnv + "}"},
	})
	if err != nil {
		t.Fatal(err)
	}
	env, err := service.Environments.Create(ws.ID, "unresolved", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.SetEnvironmentMCP(env.ID, entry.ID, true); err != nil {
		t.Fatal(err)
	}

	status, err := service.ProbeMCPHealth(context.Background(), env.ID, entry.ID)
	if err != nil {
		t.Fatal(err)
	}
	if status.State != MCPHealthConfigured || status.ErrorKind != "unresolved_secret_reference" {
		t.Fatalf("status = %+v; want configured unresolved_secret_reference", status)
	}
}

func TestProbeMCPHealthAuthFailureDoesNotExposeSecrets(t *testing.T) {
	const secret = "health-probe-secret-4f92"
	t.Setenv("ADM_MCP_TEST_SECRET", secret)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	service := New(filepath.Join(t.TempDir(), "state.json"))
	ws, err := service.Workspaces.Add(t.TempDir(), "auth")
	if err != nil {
		t.Fatal(err)
	}
	entry, err := service.MCPs.AddMCPConfig("auth", catalog.MCPConfig{
		Endpoint:   server.URL + "/mcp?token=${ADM_MCP_TEST_SECRET}",
		Transport:  "streamable-http",
		AuthMode:   catalog.MCPAuthHeaders,
		HeaderRefs: map[string]string{"Authorization": "Bearer ${ADM_MCP_TEST_SECRET}"},
	})
	if err != nil {
		t.Fatal(err)
	}
	env, err := service.Environments.Create(ws.ID, "auth", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.SetEnvironmentMCP(env.ID, entry.ID, true); err != nil {
		t.Fatal(err)
	}

	status, err := service.ProbeMCPHealth(context.Background(), env.ID, entry.ID)
	if err != nil {
		t.Fatal(err)
	}
	if status.State != MCPHealthError || status.ErrorKind != "auth_failure" || !strings.Contains(status.Message, "HTTP Auth") || !strings.Contains(status.Message, "刷新") {
		t.Fatalf("status = %+v; want actionable auth_failure", status)
	}
	serialized := status.Message + " " + (&MCPError{MCPID: status.MCPID, ErrorKind: status.ErrorKind, Message: status.Message}).Error()
	if strings.Contains(serialized, secret) {
		t.Fatalf("health output exposed secret: %q", serialized)
	}
}

func TestProbeMCPHealthTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(300 * time.Millisecond)
		w.WriteHeader(http.StatusGatewayTimeout)
	}))
	defer server.Close()

	service, envID, mcpID := configuredEnabledMCP(t, server.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	status, err := service.ProbeMCPHealth(ctx, envID, mcpID)
	if err != nil {
		t.Fatal(err)
	}
	if status.State != MCPHealthError || status.ErrorKind != "timeout" || !strings.Contains(status.Message, "Probe timeout") {
		t.Fatalf("status = %+v; want actionable timeout", status)
	}
}

func TestResolveMCPActivationSeparatesDesiredAndRuntimeSecretValues(t *testing.T) {
	const secret = "phase5-runtime-secret"
	t.Setenv("ADM_PHASE5_MCP_SECRET", secret)
	statePath := filepath.Join(t.TempDir(), "state.json")
	service := New(statePath)
	workspace, err := service.Workspaces.Add(t.TempDir(), "activation")
	if err != nil {
		t.Fatal(err)
	}
	entry, err := service.MCPs.AddMCPConfig("activation", catalog.MCPConfig{
		Endpoint:   "http://127.0.0.1:65534/mcp?token=${ADM_PHASE5_MCP_SECRET}",
		Transport:  catalog.MCPTransportStreamableHTTP,
		AuthMode:   catalog.MCPAuthHeaders,
		HeaderRefs: map[string]string{"Authorization": "Bearer ${ADM_PHASE5_MCP_SECRET}"},
	})
	if err != nil {
		t.Fatal(err)
	}
	environment, err := service.Environments.Create(workspace.ID, "activation", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.SetEnvironmentMCP(environment.ID, entry.ID, true); err != nil {
		t.Fatal(err)
	}

	activation, status, err := service.ResolveMCPActivation(environment.ID, entry.ID)
	if err != nil {
		t.Fatal(err)
	}
	if activation == nil || status.State != MCPHealthConfigured {
		t.Fatalf("activation=%+v status=%+v", activation, status)
	}
	if !strings.Contains(activation.Endpoint, secret) || activation.Headers["Authorization"] != "Bearer "+secret {
		t.Fatalf("runtime activation did not expand secret: %+v", activation)
	}
	persisted, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(persisted), secret) {
		t.Fatal("resolved runtime secret was persisted")
	}
	if !strings.Contains(string(persisted), "${ADM_PHASE5_MCP_SECRET}") {
		t.Fatal("persisted desired state lost the unresolved secret reference")
	}

	if _, err := service.SetEnvironmentMCP(environment.ID, entry.ID, false); err != nil {
		t.Fatal(err)
	}
	activation, status, err = service.ResolveMCPActivation(environment.ID, entry.ID)
	if err != nil {
		t.Fatal(err)
	}
	if activation != nil || status.State != MCPHealthDisabled {
		t.Fatalf("disabled activation=%+v status=%+v", activation, status)
	}
}

func TestRefreshHostEnvironmentUpdatesMCPActivationWithoutPersistingValues(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "state.json")
	service := New(statePath)
	service.HostEnvironment = hostenv.NewWithReader(map[string]string{
		"ADM_DYNAMIC_HOST":  "127.0.0.1:8101",
		"ADM_DYNAMIC_TOKEN": "before-secret",
	}, func() (map[string]string, string, error) {
		return map[string]string{
			"ADM_DYNAMIC_HOST":  "127.0.0.1:8102",
			"ADM_DYNAMIC_TOKEN": "after-secret",
		}, "test_host", nil
	})
	workspace, err := service.Workspaces.Add(t.TempDir(), "dynamic-env")
	if err != nil {
		t.Fatal(err)
	}
	entry, err := service.MCPs.AddMCPConfig("dynamic-env", catalog.MCPConfig{
		Endpoint:   "http://${ADM_DYNAMIC_HOST}/mcp",
		Transport:  catalog.MCPTransportStreamableHTTP,
		AuthMode:   catalog.MCPAuthHeaders,
		HeaderRefs: map[string]string{"Authorization": "Bearer ${ADM_DYNAMIC_TOKEN}"},
	})
	if err != nil {
		t.Fatal(err)
	}
	environment, err := service.Environments.Create(workspace.ID, "dynamic-env", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.SetEnvironmentMCP(environment.ID, entry.ID, true); err != nil {
		t.Fatal(err)
	}

	activation, _, err := service.ResolveMCPActivation(environment.ID, entry.ID)
	if err != nil {
		t.Fatal(err)
	}
	if activation == nil || activation.Endpoint != "http://127.0.0.1:8101/mcp" || activation.Headers["Authorization"] != "Bearer before-secret" {
		t.Fatalf("pre-refresh activation = %+v", activation)
	}

	status, err := service.RefreshHostEnvironment()
	if err != nil {
		t.Fatal(err)
	}
	if status.LastRefreshStatus != "success" || status.Source != "test_host" || status.LastRefreshAt == nil {
		t.Fatalf("refresh status = %+v", status)
	}
	activation, _, err = service.ResolveMCPActivation(environment.ID, entry.ID)
	if err != nil {
		t.Fatal(err)
	}
	if activation == nil || activation.Endpoint != "http://127.0.0.1:8102/mcp" || activation.Headers["Authorization"] != "Bearer after-secret" {
		t.Fatalf("post-refresh activation = %+v", activation)
	}
	persisted, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(persisted), "before-secret") || strings.Contains(string(persisted), "after-secret") {
		t.Fatal("resolved host environment values must not be persisted")
	}
}

func TestResolveEndpointAndHeadersExpandEnvVars(t *testing.T) {
	t.Setenv("ADM_MCP_HOST", "127.0.0.1:8123")
	t.Setenv("ADM_MCP_TOKEN", "top-secret")
	if got := os.ExpandEnv("http://${ADM_MCP_HOST}/mcp"); got != "http://127.0.0.1:8123/mcp" {
		t.Fatalf("resolveEndpoint = %q", got)
	}
	headers := resolveMap(map[string]string{"Authorization": "Bearer ${ADM_MCP_TOKEN}"})
	if got := headers["Authorization"]; got != "Bearer top-secret" {
		t.Fatalf("resolveHeaders = %q", got)
	}
}

func TestMCPErrorImplementsErrorWithoutSecretLeakage(t *testing.T) {
	var err error = &MCPError{MCPID: "mcp_1", ErrorKind: "auth_failure", Message: "authentication failed"}
	got := err.Error()
	if !strings.Contains(got, "mcp_id=mcp_1") || !strings.Contains(got, "error_kind=auth_failure") || strings.Contains(got, "secret") {
		t.Fatalf("MCPError.Error() = %q", got)
	}
}

func configuredEnabledMCP(t *testing.T, endpoint string) (*Service, string, string) {
	t.Helper()
	service := New(filepath.Join(t.TempDir(), "state.json"))
	ws, err := service.Workspaces.Add(t.TempDir(), "health")
	if err != nil {
		t.Fatal(err)
	}
	entry, err := service.MCPs.AddMCP("health", endpoint, false)
	if err != nil {
		t.Fatal(err)
	}
	env, err := service.Environments.Create(ws.ID, "health", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.SetEnvironmentMCP(env.ID, entry.ID, true); err != nil {
		t.Fatal(err)
	}
	return service, env.ID, entry.ID
}
