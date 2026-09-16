package gateway

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"ai-dev-manager-v2/internal/app"
	"ai-dev-manager-v2/internal/catalog"
	"ai-dev-manager-v2/internal/model"
	"ai-dev-manager-v2/internal/pathutil"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestGatewayStdioMCPRealTransportAuthorityAndCleanup(t *testing.T) {
	const (
		modeSource  = "ADM_TEST_STDIO_MODE_SOURCE"
		valueSource = "ADM_TEST_STDIO_VALUE_SOURCE"
		exitFile    = "stdio-helper-exited.txt"
	)
	t.Setenv(modeSource, "1")
	t.Setenv(valueSource, "from-reference")

	root := t.TempDir()
	service := app.New(filepath.Join(t.TempDir(), "state.json"))
	workspace, err := service.Workspaces.Add(root, "stdio-mcp")
	if err != nil {
		t.Fatal(err)
	}
	environment, err := service.Environments.Create(workspace.ID, "stdio-mcp", "")
	if err != nil {
		t.Fatal(err)
	}
	entry, err := service.MCPs.AddMCPConfig("stdio-helper", catalog.MCPConfig{
		Transport:  catalog.MCPTransportStdio,
		Executable: os.Args[0],
		Args:       []string{"-test.run=^TestStdioMCPHelper$"},
		EnvRefs: map[string]string{
			"ADM_TEST_STDIO_MCP_HELPER": "${" + modeSource + "}",
			"ADM_TEST_STDIO_VALUE":      "${" + valueSource + "}",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.SetEnvironmentMCP(environment.ID, entry.ID, true); err != nil {
		t.Fatal(err)
	}

	owner := newRuntimeOwner(service)
	defer owner.Close()
	ctx := context.Background()
	status, err := owner.Status(ctx, environment.ID, entry.ID)
	if err != nil {
		t.Fatal(err)
	}
	if status.State != app.MCPHealthError || status.ErrorKind != "executable_not_allowed" {
		t.Fatalf("stdio MCP without allowlist status = %+v", status)
	}
	if err := service.AllowExecutable(os.Args[0]); err != nil {
		t.Fatal(err)
	}

	session := connectInMemory(t, ctx, newServerForSurface(service, owner, serverSurfaceAdmin))
	defer session.Close()
	tools := callGatewayTool(t, ctx, session, "environment_mcp_tools", map[string]any{
		"environment_id": environment.ID,
		"mcp_id":         entry.ID,
	})
	if tools.IsError || !strings.Contains(toolText(t, tools), "stdio_inspect") {
		t.Fatalf("stdio MCP tools result = %+v text=%s", tools, toolText(t, tools))
	}
	called := callGatewayTool(t, ctx, session, "environment_mcp_call", map[string]any{
		"environment_id": environment.ID,
		"mcp_id":         entry.ID,
		"tool":           "stdio_inspect",
	})
	if called.IsError {
		t.Fatalf("stdio MCP call failed: %s", toolText(t, called))
	}
	callText := toolText(t, called)
	escapedRoot := strings.ReplaceAll(pathutil.ForCompare(root), `\`, `\\`)
	if !strings.Contains(callText, escapedRoot) || !strings.Contains(callText, "from-reference") {
		t.Fatalf("stdio MCP did not inherit Environment root/ref values: %s", callText)
	}

	disabled := callGatewayTool(t, ctx, session, "environment_mcp_set", map[string]any{
		"environment_id": environment.ID,
		"id":             entry.ID,
		"enabled":        false,
	})
	if disabled.IsError {
		t.Fatalf("disable stdio MCP failed: %s", toolText(t, disabled))
	}
	waitForTestFile(t, filepath.Join(root, exitFile))

	if err := os.Remove(filepath.Join(root, exitFile)); err != nil {
		t.Fatal(err)
	}
	if _, err := service.SetEnvironmentMCP(environment.ID, entry.ID, true); err != nil {
		t.Fatal(err)
	}
	if _, err := owner.ListTools(ctx, environment.ID, entry.ID); err != nil {
		t.Fatal(err)
	}
	if err := owner.Close(); err != nil {
		t.Fatal(err)
	}
	waitForTestFile(t, filepath.Join(root, exitFile))
}

func TestStdioMCPHelper(t *testing.T) {
	if os.Getenv("ADM_TEST_STDIO_MCP_HELPER") != "1" {
		return
	}
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return
	}
	defer func() {
		_ = os.WriteFile(filepath.Join(cwd, "stdio-helper-exited.txt"), []byte("exited\n"), 0o644)
	}()
	server := mcp.NewServer(&mcp.Implementation{Name: "stdio-helper", Version: "dev"}, nil)
	mcp.AddTool(server, &mcp.Tool{Name: "stdio_inspect", Description: "Return helper process context."},
		func(context.Context, *mcp.CallToolRequest, EmptyInput) (*mcp.CallToolResult, any, error) {
			return nil, map[string]any{"cwd": cwd, "value": os.Getenv("ADM_TEST_STDIO_VALUE")}, nil
		})
	_ = server.Run(context.Background(), &mcp.StdioTransport{})
}

func waitForTestFile(t *testing.T, path string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(path); err == nil {
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", path)
}

func TestMCPHealthLifecycleEndToEnd(t *testing.T) {
	t.Setenv("ADM_MCP_ACCEPTANCE_TOKEN", "acceptance-secret")

	external := mcp.NewServer(&mcp.Implementation{Name: "external-acceptance", Version: "dev"}, nil)
	mcp.AddTool(external, &mcp.Tool{Name: "external_first", Description: "First external tool."},
		func(context.Context, *mcp.CallToolRequest, EmptyInput) (*mcp.CallToolResult, any, error) {
			return nil, map[string]any{"value": "first"}, nil
		})
	mcp.AddTool(external, &mcp.Tool{Name: "external_ping", Description: "Return an acceptance pong."},
		func(context.Context, *mcp.CallToolRequest, EmptyInput) (*mcp.CallToolResult, any, error) {
			return nil, map[string]any{"pong": "external-pong"}, nil
		})

	base := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return external }, &mcp.StreamableHTTPOptions{
		Stateless:                  true,
		DisableLocalhostProtection: true,
	})
	externalHTTP := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer acceptance-secret" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		base.ServeHTTP(w, r)
	}))
	closed := false
	t.Cleanup(func() {
		if !closed {
			externalHTTP.Close()
		}
	})

	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "marker.txt"), []byte("gateway-survives-broken-mcp\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	service := app.New(filepath.Join(t.TempDir(), "state.json"))
	ws, err := service.Workspaces.Add(root, "mcp-acceptance")
	if err != nil {
		t.Fatal(err)
	}
	entry, err := service.MCPs.AddMCPConfig("external", catalog.MCPConfig{
		Endpoint:   externalHTTP.URL,
		Transport:  catalog.MCPTransportStreamableHTTP,
		AuthMode:   catalog.MCPAuthHeaders,
		HeaderRefs: map[string]string{"Authorization": "Bearer ${ADM_MCP_ACCEPTANCE_TOKEN}"},
	})
	if err != nil {
		t.Fatal(err)
	}
	env, err := service.Environments.Create(ws.ID, "mcp-runtime", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.SetEnvironmentMCP(env.ID, entry.ID, true); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	session := connectInMemory(t, ctx, New(service))
	defer session.Close()

	status := callGatewayTool(t, ctx, session, "environment_mcp_status", map[string]any{
		"environment_id": env.ID,
		"mcp_id":         entry.ID,
	})
	if status.IsError || !strings.Contains(toolText(t, status), "healthy") {
		t.Fatalf("healthy MCP status failed: %+v text=%s", status, toolText(t, status))
	}

	listed := callGatewayTool(t, ctx, session, "environment_mcp_tools", map[string]any{
		"environment_id": env.ID,
		"mcp_id":         entry.ID,
	})
	if listed.IsError {
		t.Fatalf("healthy MCP tool listing failed: %s", toolText(t, listed))
	}
	listedText := toolText(t, listed)
	if !strings.Contains(listedText, "external_first") || !strings.Contains(listedText, "external_ping") {
		t.Fatalf("external tool listing missing tools: %s", listedText)
	}

	called := callGatewayTool(t, ctx, session, "environment_mcp_call", map[string]any{
		"environment_id": env.ID,
		"mcp_id":         entry.ID,
		"tool":           "external_ping",
	})
	if called.IsError || !strings.Contains(toolText(t, called), "external-pong") {
		t.Fatalf("external MCP call failed: %s", toolText(t, called))
	}

	missingTool := callGatewayTool(t, ctx, session, "environment_mcp_call", map[string]any{
		"environment_id": env.ID,
		"mcp_id":         entry.ID,
		"tool":           "   ",
	})
	if !missingTool.IsError || !strings.Contains(toolText(t, missingTool), "error_kind=missing_tool_name") {
		t.Fatalf("empty MCP tool name must be structured error: %+v text=%s", missingTool, toolText(t, missingTool))
	}

	if _, err := service.SetEnvironmentMCP(env.ID, entry.ID, false); err != nil {
		t.Fatal(err)
	}
	disabled := callGatewayTool(t, ctx, session, "environment_mcp_status", map[string]any{
		"environment_id": env.ID,
		"mcp_id":         entry.ID,
	})
	if disabled.IsError || !strings.Contains(toolText(t, disabled), "disabled") {
		t.Fatalf("disabled MCP status failed: %+v text=%s", disabled, toolText(t, disabled))
	}
	for _, name := range []string{"environment_mcp_tools", "environment_mcp_call"} {
		arguments := map[string]any{"environment_id": env.ID, "mcp_id": entry.ID}
		if name == "environment_mcp_call" {
			arguments["tool"] = "external_ping"
		}
		result := callGatewayTool(t, ctx, session, name, arguments)
		if !result.IsError || !strings.Contains(toolText(t, result), "error_kind=not_enabled") {
			t.Fatalf("%s must reject disabled MCP with not_enabled: %+v text=%s", name, result, toolText(t, result))
		}
	}

	if _, err := service.SetEnvironmentMCP(env.ID, entry.ID, true); err != nil {
		t.Fatal(err)
	}
	externalHTTP.Close()
	closed = true

	brokenStatus := callGatewayTool(t, ctx, session, "environment_mcp_status", map[string]any{
		"environment_id": env.ID,
		"mcp_id":         entry.ID,
	})
	if brokenStatus.IsError {
		t.Fatalf("health status tool itself must return structured status, not a tool error: %s", toolText(t, brokenStatus))
	}
	brokenStatusText := toolText(t, brokenStatus)
	if !strings.Contains(brokenStatusText, "error") || !strings.Contains(brokenStatusText, "connection_refused") {
		t.Fatalf("broken MCP status must report structured error: %s", brokenStatusText)
	}

	for _, name := range []string{"environment_mcp_tools", "environment_mcp_call"} {
		arguments := map[string]any{"environment_id": env.ID, "mcp_id": entry.ID}
		if name == "environment_mcp_call" {
			arguments["tool"] = "external_ping"
		}
		result := callGatewayTool(t, ctx, session, name, arguments)
		text := toolText(t, result)
		if !result.IsError || !strings.Contains(text, "error_kind=connection_refused") {
			t.Fatalf("%s must return structured connection error: %+v text=%s", name, result, text)
		}
		if strings.Contains(text, externalHTTP.URL) || strings.Contains(text, "acceptance-secret") {
			t.Fatalf("%s leaked endpoint or secret: %s", name, text)
		}
	}

	gatewayInfo := callGatewayTool(t, ctx, session, "gateway_info", map[string]any{})
	if gatewayInfo.IsError {
		t.Fatalf("broken external MCP must not break gateway_info: %s", toolText(t, gatewayInfo))
	}
	read := callGatewayTool(t, ctx, session, "read", map[string]any{
		"environment_id": env.ID,
		"path":           "marker.txt",
	})
	if read.IsError || !strings.Contains(toolText(t, read), "gateway-survives-broken-mcp") {
		t.Fatalf("broken external MCP must not break read: %+v text=%s", read, toolText(t, read))
	}
}

func TestGatewayGenericMCPServersAutoPreview(t *testing.T) {
	service := app.New(filepath.Join(t.TempDir(), "state.json"))
	owner := newRuntimeOwner(service)
	defer owner.Close()
	ctx := context.Background()
	session := connectInMemory(t, ctx, newServerForSurface(service, owner, serverSurfaceAdmin))
	defer session.Close()

	content := `{"mcpServers":{"db":{"command":"node","args":["server.js"],"env":{"DB_HOST":"db.example.test","DB_PASSWORD":"example-secret"},"type":"stdio","visibility":"public","owner":"example","options":{"progress":true}}}}`
	preview := callGatewayTool(t, ctx, session, "mcp_import_preview", map[string]any{
		"format":        app.MCPImportAuto,
		"json_or_jsonc": content,
	})
	text := toolText(t, preview)
	if preview.IsError || !strings.Contains(text, app.MCPImportGeneric) || !strings.Contains(text, "ADM_MCP_IMPORT_DB_ENV_DB_PASSWORD") {
		t.Fatalf("generic mcpServers Admin MCP preview failed: %s", text)
	}
	for _, literal := range []string{"db.example.test", "example-secret"} {
		if strings.Contains(text, literal) {
			t.Fatalf("generic Admin MCP preview leaked literal env value %q: %s", literal, text)
		}
	}
}

func TestGatewayImportedHTTPMCPActivatesThroughRealRuntime(t *testing.T) {
	const valueEnv = "ADM_MCP_IMPORT_ACCEPTANCE_VALUE"
	t.Setenv(valueEnv, "fixture-value")

	external := mcp.NewServer(&mcp.Implementation{Name: "imported-acceptance", Version: "dev"}, nil)
	mcp.AddTool(external, &mcp.Tool{Name: "imported_ping", Description: "Return imported pong."},
		func(context.Context, *mcp.CallToolRequest, EmptyInput) (*mcp.CallToolResult, any, error) {
			return nil, map[string]any{"pong": "imported-pong"}, nil
		})
	base := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return external }, &mcp.StreamableHTTPOptions{
		Stateless:                  true,
		DisableLocalhostProtection: true,
	})
	externalHTTP := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Import-Value") != "fixture-value" {
			http.Error(w, "missing import header", http.StatusUnauthorized)
			return
		}
		base.ServeHTTP(w, r)
	}))
	defer externalHTTP.Close()

	service := app.New(filepath.Join(t.TempDir(), "state.json"))
	workspace, err := service.Workspaces.Add(t.TempDir(), "import-acceptance")
	if err != nil {
		t.Fatal(err)
	}
	environment, err := service.Environments.Create(workspace.ID, "import-acceptance", "")
	if err != nil {
		t.Fatal(err)
	}
	owner := newRuntimeOwner(service)
	defer owner.Close()
	ctx := context.Background()
	session := connectInMemory(t, ctx, newServerForSurface(service, owner, serverSurfaceAdmin))
	defer session.Close()

	content := fmt.Sprintf(`{"imported":{"type":"http","url":%q,"headers":{"X-Import-Value":"${%s}"}}}`, externalHTTP.URL, valueEnv)
	preview := callGatewayTool(t, ctx, session, "mcp_import_preview", map[string]any{
		"format":        app.MCPImportCodexPlugin,
		"json_or_jsonc": content,
	})
	previewText := toolText(t, preview)
	if preview.IsError || !strings.Contains(previewText, "imported") || strings.Contains(previewText, "fixture-value") {
		t.Fatalf("import preview failed or leaked resolved value: %s", previewText)
	}
	applied := callGatewayTool(t, ctx, session, "mcp_import_apply", map[string]any{
		"format":         app.MCPImportCodexPlugin,
		"json_or_jsonc":  content,
		"selected_names": []string{"imported"},
	})
	if applied.IsError || strings.Contains(toolText(t, applied), "fixture-value") {
		t.Fatalf("import apply failed or leaked resolved value: %s", toolText(t, applied))
	}

	definitions, err := service.MCPs.List()
	if err != nil {
		t.Fatal(err)
	}
	var importedID string
	for _, definition := range definitions {
		if definition.Name == "imported" {
			importedID = definition.ID
			break
		}
	}
	if importedID == "" {
		t.Fatalf("imported MCP missing from catalog: %+v", definitions)
	}
	persistedEnvironment, err := service.Environments.Get(environment.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(persistedEnvironment.EnabledMCPIDs) != 0 {
		t.Fatalf("import silently changed Environment selection: %+v", persistedEnvironment.EnabledMCPIDs)
	}

	enabled := callGatewayTool(t, ctx, session, "environment_mcp_set", map[string]any{
		"environment_id": environment.ID,
		"id":             importedID,
		"enabled":        true,
	})
	if enabled.IsError {
		t.Fatalf("enable imported MCP failed: %s", toolText(t, enabled))
	}
	listed := callGatewayTool(t, ctx, session, "environment_mcp_tools", map[string]any{
		"environment_id": environment.ID,
		"mcp_id":         importedID,
	})
	if listed.IsError || !strings.Contains(toolText(t, listed), "imported_ping") {
		t.Fatalf("imported MCP tools failed: %s", toolText(t, listed))
	}
	called := callGatewayTool(t, ctx, session, "environment_mcp_call", map[string]any{
		"environment_id": environment.ID,
		"mcp_id":         importedID,
		"tool":           "imported_ping",
	})
	if called.IsError || !strings.Contains(toolText(t, called), "imported-pong") {
		t.Fatalf("imported MCP call failed: %s", toolText(t, called))
	}
}

func TestGatewayImportedStdioMCPActivatesThroughRealRuntime(t *testing.T) {
	const (
		modeSource  = "ADM_TEST_IMPORTED_STDIO_MODE_SOURCE"
		valueSource = "ADM_TEST_IMPORTED_STDIO_VALUE_SOURCE"
	)
	t.Setenv(modeSource, "1")
	t.Setenv(valueSource, "imported-reference")

	root := t.TempDir()
	service := app.New(filepath.Join(t.TempDir(), "state.json"))
	workspace, err := service.Workspaces.Add(root, "import-stdio")
	if err != nil {
		t.Fatal(err)
	}
	environment, err := service.Environments.Create(workspace.ID, "import-stdio", "")
	if err != nil {
		t.Fatal(err)
	}
	if err := service.AllowExecutable(os.Args[0]); err != nil {
		t.Fatal(err)
	}
	owner := newRuntimeOwner(service)
	defer owner.Close()
	ctx := context.Background()
	session := connectInMemory(t, ctx, newServerForSurface(service, owner, serverSurfaceAdmin))
	defer session.Close()

	content := fmt.Sprintf(`{"imported-stdio":{"command":%q,"args":["-test.run=^TestStdioMCPHelper$"],"env":{"ADM_TEST_STDIO_MCP_HELPER":"${%s}","ADM_TEST_STDIO_VALUE":"${%s}"}}}`, os.Args[0], modeSource, valueSource)
	applied := callGatewayTool(t, ctx, session, "mcp_import_apply", map[string]any{
		"format":         app.MCPImportCodexPlugin,
		"json_or_jsonc":  content,
		"selected_names": []string{"imported-stdio"},
	})
	if applied.IsError {
		t.Fatalf("stdio import apply failed: %s", toolText(t, applied))
	}
	definitions, err := service.MCPs.List()
	if err != nil {
		t.Fatal(err)
	}
	var importedID string
	for _, definition := range definitions {
		if definition.Name == "imported-stdio" {
			importedID = definition.ID
			if definition.Transport != catalog.MCPTransportStdio {
				t.Fatalf("imported stdio transport=%q", definition.Transport)
			}
			break
		}
	}
	if importedID == "" {
		t.Fatalf("imported stdio MCP missing: %+v", definitions)
	}
	persistedEnvironment, err := service.Environments.Get(environment.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(persistedEnvironment.EnabledMCPIDs) != 0 {
		t.Fatalf("stdio import silently changed Environment selection: %+v", persistedEnvironment.EnabledMCPIDs)
	}

	enabled := callGatewayTool(t, ctx, session, "environment_mcp_set", map[string]any{
		"environment_id": environment.ID,
		"id":             importedID,
		"enabled":        true,
	})
	if enabled.IsError {
		t.Fatalf("enable imported stdio MCP failed: %s", toolText(t, enabled))
	}
	listed := callGatewayTool(t, ctx, session, "environment_mcp_tools", map[string]any{
		"environment_id": environment.ID,
		"mcp_id":         importedID,
	})
	if listed.IsError || !strings.Contains(toolText(t, listed), "stdio_inspect") {
		t.Fatalf("imported stdio MCP tools failed: %s", toolText(t, listed))
	}
	called := callGatewayTool(t, ctx, session, "environment_mcp_call", map[string]any{
		"environment_id": environment.ID,
		"mcp_id":         importedID,
		"tool":           "stdio_inspect",
	})
	if called.IsError {
		t.Fatalf("imported stdio MCP call failed: %s", toolText(t, called))
	}
	callText := toolText(t, called)
	escapedRoot := strings.ReplaceAll(pathutil.ForCompare(root), `\`, `\\`)
	if !strings.Contains(callText, escapedRoot) || !strings.Contains(callText, "imported-reference") {
		t.Fatalf("imported stdio MCP did not use Environment root/ref values: %s", callText)
	}
}

func TestRuntimeOwnerRealHTTPRestartReconciliation(t *testing.T) {
	external := mcp.NewServer(&mcp.Implementation{Name: "phase5-owned-upstream", Version: "dev"}, nil)
	mcp.AddTool(external, &mcp.Tool{Name: "owner_ping", Description: "Return owner pong."},
		func(context.Context, *mcp.CallToolRequest, EmptyInput) (*mcp.CallToolResult, any, error) {
			return nil, map[string]any{"pong": "owner-pong"}, nil
		})
	base := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return external }, &mcp.StreamableHTTPOptions{
		Stateless:                  true,
		DisableLocalhostProtection: true,
	})
	var requests atomic.Int64
	externalHTTP := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		base.ServeHTTP(w, r)
	}))
	externalClosed := false
	t.Cleanup(func() {
		if !externalClosed {
			externalHTTP.Close()
		}
	})

	statePath := filepath.Join(t.TempDir(), "state.json")
	service := app.New(statePath)
	workspace, err := service.Workspaces.Add(t.TempDir(), "phase5-owner")
	if err != nil {
		t.Fatal(err)
	}
	entry, err := service.MCPs.AddMCPConfig("phase5-owned-upstream", catalog.MCPConfig{
		Endpoint:  externalHTTP.URL,
		Transport: catalog.MCPTransportStreamableHTTP,
		HealthPolicy: model.MCPHealthPolicy{
			HealthCheckEnabled:   true,
			CheckIntervalSeconds: 60,
			ProbeTimeoutSeconds:  2,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	environment, err := service.Environments.Create(workspace.ID, "phase5-owner", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.SetEnvironmentMCP(environment.ID, entry.ID, true); err != nil {
		t.Fatal(err)
	}
	freshService := app.New(statePath)
	freshEnvironment, err := freshService.Environments.Get(environment.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(freshEnvironment.EnabledMCPIDs) != 1 || freshEnvironment.EnabledMCPIDs[0] != entry.ID {
		t.Fatalf("fresh service did not load desired MCP selection: %+v", freshEnvironment.EnabledMCPIDs)
	}
	activation, desiredStatus, err := freshService.ResolveMCPActivation(environment.ID, entry.ID)
	if err != nil || activation == nil || desiredStatus.State != app.MCPHealthConfigured {
		t.Fatalf("fresh service activation=%+v status=%+v err=%v", activation, desiredStatus, err)
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	listen := listener.Addr().String()
	_ = listener.Close()
	endpoint := "http://" + listen + "/mcp"

	var running *exec.Cmd
	t.Cleanup(func() {
		if running != nil && running.Process != nil {
			_ = running.Process.Kill()
			_ = running.Wait()
		}
	})
	startGateway := func() (*exec.Cmd, HTTPStatus) {
		cmd := exec.Command(os.Args[0], "-test.run=^TestHTTPGatewayRestartHelperProcess$")
		cmd.Env = append(os.Environ(),
			"ADM_TEST_GATEWAY_RESTART_HELPER=1",
			"ADM_TEST_GATEWAY_RESTART_STATE="+statePath,
			"ADM_TEST_GATEWAY_RESTART_LISTEN="+listen,
		)
		if err := cmd.Start(); err != nil {
			t.Fatal(err)
		}
		running = cmd
		status, err := WaitHTTPReady(listen, 5*time.Second)
		if err != nil {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
			running = nil
			t.Fatal(err)
		}
		if status.OwnerID == "" {
			t.Fatalf("started Gateway has no runtime owner: %+v", status)
		}
		return cmd, status
	}
	stopGateway := func(cmd *exec.Cmd) {
		stopped, err := StopHTTP(listen)
		if err != nil {
			t.Fatal(err)
		}
		if stopped.State != HTTPStateStopped {
			t.Fatalf("Gateway did not stop cleanly: %+v", stopped)
		}
		if err := cmd.Wait(); err != nil {
			t.Fatalf("graceful Gateway process exit: %v", err)
		}
		running = nil
	}
	waitUpstreamRequests := func(minimum int64) {
		deadline := time.Now().Add(5 * time.Second)
		for time.Now().Before(deadline) && requests.Load() < minimum {
			time.Sleep(20 * time.Millisecond)
		}
		if got := requests.Load(); got < minimum {
			t.Fatalf("upstream requests=%d want at least %d", got, minimum)
		}
	}

	firstProcess, firstStatus := startGateway()
	waitUpstreamRequests(1)
	ctx := context.Background()
	var firstInventoryAt, firstLastCheckAt string
	for i := 0; i < 2; i++ {
		session := connectHTTPWithRetry(t, ctx, endpoint)
		var info *mcp.CallToolResult
		var infoText string
		deadline := time.Now().Add(5 * time.Second)
		for time.Now().Before(deadline) {
			info = callGatewayTool(t, ctx, session, "gateway_info", map[string]any{})
			infoText = toolText(t, info)
			if !info.IsError && strings.Contains(infoText, `"owned_mcp_sessions":1`) {
				break
			}
			time.Sleep(20 * time.Millisecond)
		}
		if info.IsError || !strings.Contains(infoText, firstStatus.OwnerID) || !strings.Contains(infoText, `"owned_mcp_sessions":1`) {
			inspection := callGatewayTool(t, ctx, session, "environment_mcp_inspect", map[string]any{"environment_id": environment.ID, "mcp_id": entry.ID})
			_ = session.Close()
			t.Fatalf("client %d did not observe owner %q: %s inspection=%s upstream_requests=%d", i+1, firstStatus.OwnerID, infoText, toolText(t, inspection), requests.Load())
		}
		status := callGatewayTool(t, ctx, session, "environment_mcp_status", map[string]any{
			"environment_id": environment.ID,
			"mcp_id":         entry.ID,
		})
		if status.IsError || !strings.Contains(toolText(t, status), "healthy") {
			_ = session.Close()
			t.Fatalf("client %d owner status failed: %s", i+1, toolText(t, status))
		}
		if i == 0 {
			inspection := callGatewayTool(t, ctx, session, "environment_mcp_inspect", map[string]any{"environment_id": environment.ID, "mcp_id": entry.ID})
			inspectionText := toolText(t, inspection)
			if inspection.IsError || !strings.Contains(inspectionText, "owner_ping") || !strings.Contains(inspectionText, `"check_interval_seconds":60`) {
				_ = session.Close()
				t.Fatalf("first owner inspection missing runtime evidence/policy: %s", inspectionText)
			}
			firstInventoryAt = jsonStringField(inspectionText, "inventory_fetched_at")
			firstLastCheckAt = jsonStringField(inspectionText, "last_check_at")
			if firstInventoryAt == "" || firstLastCheckAt == "" {
				_ = session.Close()
				t.Fatalf("first owner inspection missing timestamps: %s", inspectionText)
			}
		}
		if i == 1 {
			called := callGatewayTool(t, ctx, session, "environment_mcp_call", map[string]any{
				"environment_id": environment.ID,
				"mcp_id":         entry.ID,
				"tool":           "owner_ping",
			})
			if called.IsError || !strings.Contains(toolText(t, called), "owner-pong") {
				_ = session.Close()
				t.Fatalf("owned upstream call failed: %s", toolText(t, called))
			}
		}
		_ = session.Close()
	}
	firstRunRequests := requests.Load()
	stopGateway(firstProcess)

	secondProcess, secondStatus := startGateway()
	if secondStatus.OwnerID == firstStatus.OwnerID {
		t.Fatalf("Gateway restart reused runtime owner %q", firstStatus.OwnerID)
	}
	waitUpstreamRequests(firstRunRequests + 1)
	secondSession := connectHTTPWithRetry(t, ctx, endpoint)
	secondStatusResult := callGatewayTool(t, ctx, secondSession, "environment_mcp_status", map[string]any{
		"environment_id": environment.ID,
		"mcp_id":         entry.ID,
	})
	if secondStatusResult.IsError || !strings.Contains(toolText(t, secondStatusResult), "healthy") {
		_ = secondSession.Close()
		t.Fatalf("reconciled MCP after restart is not healthy: %s", toolText(t, secondStatusResult))
	}
	secondInspection := callGatewayTool(t, ctx, secondSession, "environment_mcp_inspect", map[string]any{"environment_id": environment.ID, "mcp_id": entry.ID})
	secondInspectionText := toolText(t, secondInspection)
	secondInventoryAt := jsonStringField(secondInspectionText, "inventory_fetched_at")
	secondLastCheckAt := jsonStringField(secondInspectionText, "last_check_at")
	if secondInspection.IsError || !strings.Contains(secondInspectionText, "owner_ping") || !strings.Contains(secondInspectionText, `"check_interval_seconds":60`) || secondInventoryAt == "" || secondLastCheckAt == "" {
		_ = secondSession.Close()
		t.Fatalf("second owner inspection missing fresh runtime evidence/policy: %s", secondInspectionText)
	}
	if secondInventoryAt == firstInventoryAt || secondLastCheckAt == firstLastCheckAt {
		_ = secondSession.Close()
		t.Fatalf("restart reused owner-local timestamps: first inventory=%q check=%q second inventory=%q check=%q", firstInventoryAt, firstLastCheckAt, secondInventoryAt, secondLastCheckAt)
	}
	inspected := callGatewayTool(t, ctx, secondSession, "environment_inspect", map[string]any{"environment_id": environment.ID})
	if inspected.IsError || !strings.Contains(toolText(t, inspected), entry.ID) {
		_ = secondSession.Close()
		t.Fatalf("persisted desired MCP selection missing after restart: %s", toolText(t, inspected))
	}

	externalHTTP.Close()
	externalClosed = true
	broken := callGatewayTool(t, ctx, secondSession, "environment_mcp_status", map[string]any{
		"environment_id": environment.ID,
		"mcp_id":         entry.ID,
	})
	brokenText := toolText(t, broken)
	if broken.IsError || !strings.Contains(brokenText, "error") || strings.Contains(brokenText, "healthy") {
		_ = secondSession.Close()
		t.Fatalf("dead upstream retained healthy observation: %s", brokenText)
	}
	ownerAfterFailure := callGatewayTool(t, ctx, secondSession, "gateway_info", map[string]any{})
	if ownerAfterFailure.IsError || !strings.Contains(toolText(t, ownerAfterFailure), `"owned_mcp_sessions":0`) {
		_ = secondSession.Close()
		t.Fatalf("dead upstream session was not evicted: %s", toolText(t, ownerAfterFailure))
	}
	_ = secondSession.Close()
	stopGateway(secondProcess)

	thirdProcess, thirdStatus := startGateway()
	if thirdStatus.OwnerID == secondStatus.OwnerID || thirdStatus.OwnerID == firstStatus.OwnerID {
		t.Fatalf("second restart reused runtime owner: first=%q second=%q third=%q", firstStatus.OwnerID, secondStatus.OwnerID, thirdStatus.OwnerID)
	}
	thirdSession := connectHTTPWithRetry(t, ctx, endpoint)
	thirdResult := callGatewayTool(t, ctx, thirdSession, "environment_mcp_status", map[string]any{
		"environment_id": environment.ID,
		"mcp_id":         entry.ID,
	})
	thirdText := toolText(t, thirdResult)
	if thirdResult.IsError || !strings.Contains(thirdText, "error") || strings.Contains(thirdText, "healthy") {
		_ = thirdSession.Close()
		t.Fatalf("restart resurrected stale healthy state: %s", thirdText)
	}
	thirdInspection := callGatewayTool(t, ctx, thirdSession, "environment_mcp_inspect", map[string]any{"environment_id": environment.ID, "mcp_id": entry.ID})
	thirdInspectionText := toolText(t, thirdInspection)
	if thirdInspection.IsError || !strings.Contains(thirdInspectionText, `"check_interval_seconds":60`) || strings.Contains(thirdInspectionText, "owner_ping") || strings.Contains(thirdInspectionText, firstInventoryAt) || strings.Contains(thirdInspectionText, secondInventoryAt) {
		_ = thirdSession.Close()
		t.Fatalf("failed restart reused stale owner-local inventory/observation: %s", thirdInspectionText)
	}
	persistedEnvironment, err := service.Environments.Get(environment.ID)
	if err != nil {
		_ = thirdSession.Close()
		t.Fatal(err)
	}
	if len(persistedEnvironment.EnabledMCPIDs) != 1 || persistedEnvironment.EnabledMCPIDs[0] != entry.ID {
		_ = thirdSession.Close()
		t.Fatalf("observed failure rewrote desired selection: %+v", persistedEnvironment.EnabledMCPIDs)
	}
	_ = thirdSession.Close()
	stopGateway(thirdProcess)
}

func jsonStringField(text, field string) string {
	marker := `"` + field + `":`
	index := strings.Index(text, marker)
	if index < 0 {
		return ""
	}
	rest := strings.TrimSpace(text[index+len(marker):])
	if !strings.HasPrefix(rest, `"`) {
		return ""
	}
	rest = rest[1:]
	end := strings.Index(rest, `"`)
	if end < 0 {
		return ""
	}
	return rest[:end]
}

func callGatewayTool(t *testing.T, ctx context.Context, session *mcp.ClientSession, name string, arguments map[string]any) *mcp.CallToolResult {
	t.Helper()
	result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: name, Arguments: arguments})
	if err != nil {
		t.Fatalf("%s transport error: %v", name, err)
	}
	return result
}
