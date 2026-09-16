package cli

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"ai-dev-manager-v2/internal/app"
	"ai-dev-manager-v2/internal/catalog"
	"ai-dev-manager-v2/internal/gateway"
	"ai-dev-manager-v2/internal/pathutil"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type phase23EmptyInput struct{}

func startOwnedCLIAdminGateway(t *testing.T, service *app.Service) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	listen := listener.Addr().String()
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- gateway.RunHTTP(ctx, service, listen) }()
	status, err := gateway.WaitHTTPReady(listen, 5*time.Second)
	if err != nil {
		cancel()
		t.Fatal(err)
	}
	if status.OwnerID == "" {
		cancel()
		t.Fatalf("test Gateway has no persistent runtime owner: %+v", status)
	}
	t.Cleanup(func() {
		_, _ = gateway.StopHTTP(listen)
		cancel()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Errorf("test Gateway did not stop")
		}
	})
	return status.BaseURL
}

func TestResolveMCPImportContentRequiresExactlyOneExplicitSource(t *testing.T) {
	file := filepath.Join(t.TempDir(), "mcp.json")
	if err := os.WriteFile(file, []byte(`{"from":"file"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name        string
		inline      string
		inlineSet   bool
		file        string
		fileSet     bool
		stdin       bool
		stdinBody   string
		interactive bool
		want        string
		wantErr     string
	}{
		{name: "inline", inline: `{"from":"inline"}`, inlineSet: true, want: `{"from":"inline"}`},
		{name: "file", file: file, fileSet: true, want: `{"from":"file"}`},
		{name: "stdin", stdin: true, stdinBody: `{"from":"stdin"}`, want: `{"from":"stdin"}`},
		{name: "zero", wantErr: "exactly one"},
		{name: "multiple", inline: `{}`, inlineSet: true, file: file, fileSet: true, wantErr: "exactly one"},
		{name: "interactive stdin", stdin: true, stdinBody: `{}`, interactive: true, wantErr: "interactive stdin"},
		{name: "empty", inlineSet: true, wantErr: "content is empty"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolveMCPImportContent(tt.inline, tt.inlineSet, tt.file, tt.fileSet, tt.stdin, strings.NewReader(tt.stdinBody), tt.interactive)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("error=%v want substring %q", err, tt.wantErr)
				}
				return
			}
			if err != nil || got != tt.want {
				t.Fatalf("content=%q err=%v want=%q", got, err, tt.want)
			}
		})
	}
}

func TestPhase23CLIImportFileAndStdinUseAdminMCPAndPreserveCredentialSafety(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "state.json")
	service := app.New(statePath)
	baseURL := startOwnedCLIAdminGateway(t, service)
	content := `{"mcpServers":{"db":{"command":"node","args":["server.js"],"env":{"DB_HOST":"db.example.test","DB_PASSWORD":"phase23-secret","MAX_ROWS":"1000"}}}}`
	file := filepath.Join(t.TempDir(), "mcp.json")
	if err := os.WriteFile(file, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	filePreview := captureStdout(t, func() {
		if err := run([]string{"--adm-url", baseURL, "mcp", "import-preview", "--file", file}); err != nil {
			t.Fatal(err)
		}
	})
	inlinePreview := captureStdout(t, func() {
		if err := run([]string{"--adm-url", baseURL, "mcp", "import-preview", "--json-or-jsonc", content}); err != nil {
			t.Fatal(err)
		}
	})
	var fileResult, inlineResult app.MCPImportPreview
	if err := json.Unmarshal([]byte(filePreview), &fileResult); err != nil {
		t.Fatalf("file preview is not JSON: %v\n%s", err, filePreview)
	}
	if err := json.Unmarshal([]byte(inlinePreview), &inlineResult); err != nil {
		t.Fatalf("inline preview is not JSON: %v\n%s", err, inlinePreview)
	}
	if len(fileResult.Candidates) != 1 || len(inlineResult.Candidates) != 1 || fileResult.Candidates[0].Name != inlineResult.Candidates[0].Name {
		t.Fatalf("file/inline preview mismatch: file=%+v inline=%+v", fileResult, inlineResult)
	}
	for _, literal := range []string{"phase23-secret", "db.example.test", "1000"} {
		if strings.Contains(filePreview, literal) {
			t.Fatalf("preview leaked literal %q: %s", literal, filePreview)
		}
	}

	if err := run([]string{"--adm-url", baseURL, "mcp", "import-apply"}); err == nil || !strings.Contains(err.Error(), "exactly one") {
		t.Fatalf("missing import source error=%v", err)
	}
	if err := run([]string{"--adm-url", baseURL, "mcp", "import-apply", "--file", file, "--json-or-jsonc", content}); err == nil || !strings.Contains(err.Error(), "exactly one") {
		t.Fatalf("multiple import source error=%v", err)
	}
	if definitions, err := service.MCPs.List(); err != nil || len(definitions) != 0 {
		t.Fatalf("invalid source selection mutated catalog: defs=%+v err=%v", definitions, err)
	}

	readPipe, writePipe, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := writePipe.WriteString(content); err != nil {
		t.Fatal(err)
	}
	if err := writePipe.Close(); err != nil {
		t.Fatal(err)
	}
	oldStdin := os.Stdin
	os.Stdin = readPipe
	t.Cleanup(func() { os.Stdin = oldStdin; _ = readPipe.Close() })
	applied := captureStdout(t, func() {
		if err := run([]string{"--adm-url", baseURL, "mcp", "import-apply", "--stdin"}); err != nil {
			t.Fatal(err)
		}
	})
	os.Stdin = oldStdin
	_ = readPipe.Close()
	if !json.Valid([]byte(applied)) {
		t.Fatalf("stdin apply did not emit JSON: %s", applied)
	}
	definitions, err := service.MCPs.List()
	if err != nil || len(definitions) != 1 {
		t.Fatalf("stdin apply definitions=%+v err=%v", definitions, err)
	}
	for key, literal := range map[string]string{"DB_HOST": "db.example.test", "DB_PASSWORD": "phase23-secret", "MAX_ROWS": "1000"} {
		got := definitions[0].EnvRefs[key]
		if got == literal || !strings.HasPrefix(got, "${ADM_MCP_IMPORT_") {
			t.Fatalf("literal env value %s=%q was not converted to a reference: %+v", key, literal, definitions[0].EnvRefs)
		}
	}
	stateBytes, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatal(err)
	}
	// Use distinctive nonnumeric literals for the raw-state leak check. A bare
	// numeric substring such as "1000" can occur coincidentally in persisted
	// timestamps/IDs even when MAX_ROWS itself was correctly reference-normalized.
	for _, literal := range []string{"phase23-secret", "db.example.test"} {
		if strings.Contains(string(stateBytes), literal) {
			t.Fatalf("persisted state leaked literal %q", literal)
		}
	}
}

func TestPhase23CLIMCPInspectIsPassiveAndRefreshDoesNotCallBusinessTools(t *testing.T) {
	service := app.New(filepath.Join(t.TempDir(), "state.json"))
	workspace, err := service.Workspaces.Add(t.TempDir(), "phase23-mcp")
	if err != nil {
		t.Fatal(err)
	}
	environment, err := service.Environments.Create(workspace.ID, "phase23-mcp", "")
	if err != nil {
		t.Fatal(err)
	}
	baseURL := startOwnedCLIAdminGateway(t, service)

	var protocolRequests atomic.Int64
	var businessCalls atomic.Int64
	external := mcp.NewServer(&mcp.Implementation{Name: "phase23-external", Version: "dev"}, nil)
	mcp.AddTool(external, &mcp.Tool{Name: "business_tool", Description: "must not be called by refresh"},
		func(context.Context, *mcp.CallToolRequest, phase23EmptyInput) (*mcp.CallToolResult, any, error) {
			businessCalls.Add(1)
			return nil, map[string]any{"ok": true}, nil
		})
	base := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return external }, &mcp.StreamableHTTPOptions{Stateless: true, DisableLocalhostProtection: true})
	remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		protocolRequests.Add(1)
		base.ServeHTTP(w, r)
	}))
	defer remote.Close()

	entry, err := service.MCPs.AddMCPConfig("phase23-external", catalog.MCPConfig{Transport: catalog.MCPTransportStreamableHTTP, AuthMode: catalog.MCPAuthNone, Endpoint: remote.URL})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.SetEnvironmentMCP(environment.ID, entry.ID, true); err != nil {
		t.Fatal(err)
	}
	beforeDefinition, err := service.MCPs.Get(entry.ID)
	if err != nil {
		t.Fatal(err)
	}
	beforeEnvironment, err := service.Environments.Get(environment.ID)
	if err != nil {
		t.Fatal(err)
	}

	inspectOutput := captureStdout(t, func() {
		if err := run([]string{"--adm-url", baseURL, "mcp", "inspect", "--id", entry.ID, "--environment-id", environment.ID}); err != nil {
			t.Fatal(err)
		}
	})
	if protocolRequests.Load() != 0 {
		t.Fatalf("passive inspect made %d upstream request(s)", protocolRequests.Load())
	}
	if !json.Valid([]byte(inspectOutput)) || !strings.Contains(inspectOutput, entry.ID) {
		t.Fatalf("inspect output is not structured JSON: %s", inspectOutput)
	}

	refreshOutput := captureStdout(t, func() {
		if err := run([]string{"--adm-url", baseURL, "mcp", "refresh", "--id", entry.ID, "--environment-id", environment.ID}); err != nil {
			t.Fatal(err)
		}
	})
	if protocolRequests.Load() == 0 {
		t.Fatal("explicit refresh made no upstream protocol requests")
	}
	if businessCalls.Load() != 0 {
		t.Fatalf("refresh invoked business tools %d time(s)", businessCalls.Load())
	}
	if !json.Valid([]byte(refreshOutput)) || !strings.Contains(refreshOutput, `"state": "healthy"`) || !strings.Contains(refreshOutput, "business_tool") {
		t.Fatalf("refresh output=%s", refreshOutput)
	}
	afterDefinition, err := service.MCPs.Get(entry.ID)
	if err != nil {
		t.Fatal(err)
	}
	afterEnvironment, err := service.Environments.Get(environment.ID)
	if err != nil {
		t.Fatal(err)
	}
	if beforeDefinition.Endpoint != afterDefinition.Endpoint || beforeDefinition.Name != afterDefinition.Name || strings.Join(beforeEnvironment.EnabledMCPIDs, ",") != strings.Join(afterEnvironment.EnabledMCPIDs, ",") {
		t.Fatalf("refresh mutated desired state: beforeDef=%+v afterDef=%+v beforeEnv=%+v afterEnv=%+v", beforeDefinition, afterDefinition, beforeEnvironment.EnabledMCPIDs, afterEnvironment.EnabledMCPIDs)
	}
}

func TestPhase23CLISkillProvisioningAndAvailabilityUseAdminMCP(t *testing.T) {
	service := app.New(filepath.Join(t.TempDir(), "state.json"))
	workspace, err := service.Workspaces.Add(t.TempDir(), "phase23-skill")
	if err != nil {
		t.Fatal(err)
	}
	environment, err := service.Environments.Create(workspace.ID, "phase23-skill", "")
	if err != nil {
		t.Fatal(err)
	}
	baseURL := startOwnedCLIAdminGateway(t, service)

	rootA := filepath.Join(t.TempDir(), "skills-a")
	rootB := filepath.Join(t.TempDir(), "skills-b")
	supportA := filepath.Join(t.TempDir(), "support-a")
	supportB := filepath.Join(t.TempDir(), "support-b")
	if err := os.MkdirAll(rootA, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(supportA, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(supportB, 0o755); err != nil {
		t.Fatal(err)
	}
	writeCLISkill(t, rootB, "phase23-skill", "# phase23 skill\n")

	addedOutput := captureStdout(t, func() {
		if err := run([]string{"--adm-url", baseURL, "skill", "source-add", "--root", rootA, "--support-root", supportA, "--support-root", supportB, "--default"}); err != nil {
			t.Fatal(err)
		}
	})
	var added struct {
		ID string `json:"skill_source_id"`
	}
	if err := json.Unmarshal([]byte(addedOutput), &added); err != nil || added.ID == "" {
		t.Fatalf("source-add output=%s err=%v", addedOutput, err)
	}
	updatedOutput := captureStdout(t, func() {
		if err := run([]string{"--adm-url", baseURL, "skill", "source-update", "--id", added.ID, "--root", rootB, "--support-root", supportA, "--support-root", supportB}); err != nil {
			t.Fatal(err)
		}
	})
	if !json.Valid([]byte(updatedOutput)) {
		t.Fatalf("source-update output is not JSON: %s", updatedOutput)
	}
	sources, err := service.Skills.ListSkillSources()
	if err != nil || len(sources) != 1 || len(sources[0].SupportRoots) != 2 || !sources[0].DefaultIncludeInEnv || !pathutil.Same(sources[0].Root, rootB) {
		t.Fatalf("updated source=%+v err=%v", sources, err)
	}
	if entries, err := service.Skills.List(); err != nil || len(entries) != 0 {
		t.Fatalf("source update implicitly refreshed catalog: entries=%+v err=%v", entries, err)
	}
	captureStdout(t, func() {
		if err := run([]string{"--adm-url", baseURL, "skill", "source-refresh", "--id", added.ID}); err != nil {
			t.Fatal(err)
		}
	})
	entries, err := service.Skills.List()
	if err != nil || len(entries) != 1 {
		t.Fatalf("explicit refresh entries=%+v err=%v", entries, err)
	}
	skillID := entries[0].ID

	globalOutput := captureStdout(t, func() {
		if err := run([]string{"--adm-url", baseURL, "skill", "availability"}); err != nil {
			t.Fatal(err)
		}
	})
	environmentOutput := captureStdout(t, func() {
		if err := run([]string{"--adm-url", baseURL, "environment", "skill", "list", "--environment-id", environment.ID}); err != nil {
			t.Fatal(err)
		}
	})
	inspectOutput := captureStdout(t, func() {
		if err := run([]string{"--adm-url", baseURL, "environment", "skill", "inspect", "--environment-id", environment.ID, "--skill-id", skillID}); err != nil {
			t.Fatal(err)
		}
	})
	for label, output := range map[string]string{"global": globalOutput, "environment": environmentOutput, "inspect": inspectOutput} {
		if !json.Valid([]byte(output)) {
			t.Fatalf("%s availability output is not JSON: %s", label, output)
		}
	}
	if !strings.Contains(globalOutput, `"state": "available"`) || !strings.Contains(environmentOutput, `"state": "disabled"`) || !strings.Contains(inspectOutput, `"state": "disabled"`) {
		t.Fatalf("availability scopes disagree: global=%s env=%s inspect=%s", globalOutput, environmentOutput, inspectOutput)
	}

	if _, err := service.SetEnvironmentSkill(environment.ID, skillID, true); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(entries[0].ArtifactPath); err != nil {
		t.Fatal(err)
	}
	brokenOutput := captureStdout(t, func() {
		if err := run([]string{"--adm-url", baseURL, "environment", "skill", "inspect", "--environment-id", environment.ID, "--skill-id", skillID}); err != nil {
			t.Fatal(err)
		}
	})
	if !strings.Contains(brokenOutput, `"state": "artifact_missing"`) {
		t.Fatalf("broken Skill did not remain operation-local/diagnosable: %s", brokenOutput)
	}
	if _, err := service.Workspaces.Get(workspace.ID); err != nil {
		t.Fatalf("broken Skill affected plain Workspace management: %v", err)
	}
	if _, err := service.Environments.Get(environment.ID); err != nil {
		t.Fatalf("broken Skill affected plain Environment management: %v", err)
	}
}

func TestPhase23NewCLICommandsUseNormalNoFallbackAdminPath(t *testing.T) {
	localHome := t.TempDir()
	t.Setenv("ADM_V2_HOME", localHome)
	local := app.New(filepath.Join(localHome, "state.json"))
	if _, err := local.Workspaces.Add(t.TempDir(), "local-only"); err != nil {
		t.Fatal(err)
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	baseURL := "http://" + listener.Addr().String()
	_ = listener.Close()
	t.Setenv(admBaseURLEnv, baseURL)
	for _, args := range [][]string{
		{"mcp", "inspect", "--id", "mcp_missing", "--environment-id", "env_missing"},
		{"skill", "availability"},
		{"environment", "skill", "list", "--environment-id", "env_missing"},
	} {
		err := run(args)
		if err == nil || !strings.Contains(err.Error(), "Admin MCP") {
			t.Fatalf("%v should fail through Admin MCP without fallback, got %v", args, err)
		}
	}
	items, err := local.Workspaces.List()
	if err != nil || len(items) != 1 || items[0].Name != "local-only" {
		t.Fatalf("failed remote commands touched local management state: items=%+v err=%v", items, err)
	}
}
