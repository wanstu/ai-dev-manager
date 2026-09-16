package gateway

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"ai-dev-manager-v2/internal/app"
	"ai-dev-manager-v2/internal/catalog"
	"ai-dev-manager-v2/internal/model"
	"ai-dev-manager-v2/internal/verifier"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestGatewayEnvironmentContextAgentAdminParityInstructionsAndAuthority(t *testing.T) {
	service := app.New(filepath.Join(t.TempDir(), "state.json"))
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "src"), 0o755); err != nil {
		t.Fatal(err)
	}
	ws, err := service.Workspaces.Add(root, "context-workspace")
	if err != nil {
		t.Fatal(err)
	}
	env, err := service.Environments.Create(ws.ID, "context-environment", "")
	if err != nil {
		t.Fatal(err)
	}
	owner := newRuntimeOwner(service)
	defer owner.Close()
	ctx := context.Background()

	agent := connectInMemory(t, ctx, newServer(service, owner))
	defer agent.Close()
	admin := connectInMemory(t, ctx, newServerForSurface(service, owner, serverSurfaceAdmin))
	defer admin.Close()

	for name, session := range map[string]*mcp.ClientSession{"agent": agent, "admin": admin} {
		init := session.InitializeResult()
		if init == nil || !strings.Contains(init.Instructions, "environment_context_bundle") || !strings.Contains(init.Instructions, "stable Workspace and Environment IDs") || !strings.Contains(init.Instructions, "no implicit current project") {
			t.Fatalf("%s initialize instructions=%+v", name, init)
		}
		if strings.Contains(init.Instructions, env.ID) || strings.Contains(init.Instructions, ws.ID) || strings.Contains(init.Instructions, root) {
			t.Fatalf("%s initialize instructions leaked Environment data: %q", name, init.Instructions)
		}
		tools, err := session.ListTools(ctx, nil)
		if err != nil {
			t.Fatal(err)
		}
		if !contains(toolNames(tools.Tools), "environment_context_bundle") {
			t.Fatalf("%s surface missing environment_context_bundle", name)
		}
		result := callGatewayTool(t, ctx, session, "environment_context_bundle", map[string]any{"environment_id": env.ID})
		text := toolText(t, result)
		if result.IsError || !strings.Contains(text, env.ID) || !strings.Contains(text, ws.ID) || !strings.Contains(text, `"files.inspect"`) {
			t.Fatalf("%s context result=%s", name, text)
		}
		unknown := callGatewayTool(t, ctx, session, "environment_context_bundle", map[string]any{"environment_id": "env_missing"})
		if !unknown.IsError {
			t.Fatalf("%s accepted unknown Environment: %s", name, toolText(t, unknown))
		}
		escape := callGatewayTool(t, ctx, session, "environment_context_bundle", map[string]any{"environment_id": env.ID, "path": ".."})
		if !escape.IsError {
			t.Fatalf("%s accepted parent traversal: %s", name, toolText(t, escape))
		}
	}
}

func TestGatewayEnvironmentContextUsesOnlyExistingMCPObservationAndRecompacts(t *testing.T) {
	var requests atomic.Int64
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		http.Error(w, "context must not connect", http.StatusTeapot)
	}))
	defer upstream.Close()

	service := app.New(filepath.Join(t.TempDir(), "state.json"))
	ws, err := service.Workspaces.Add(t.TempDir(), "passive-mcp")
	if err != nil {
		t.Fatal(err)
	}
	env, err := service.Environments.Create(ws.ID, "passive-mcp", "")
	if err != nil {
		t.Fatal(err)
	}
	entry, err := service.MCPs.AddMCPConfig("passive-upstream", catalog.MCPConfig{Transport: catalog.MCPTransportStreamableHTTP, AuthMode: catalog.MCPAuthNone, Endpoint: upstream.URL})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.SetEnvironmentMCP(env.ID, entry.ID, true); err != nil {
		t.Fatal(err)
	}

	owner := newRuntimeOwner(service)
	defer owner.Close()
	ctx := context.Background()
	session := connectInMemory(t, ctx, newServer(service, owner))
	defer session.Close()

	unobserved := callGatewayTool(t, ctx, session, "environment_context_bundle", map[string]any{"environment_id": env.ID})
	unobservedText := toolText(t, unobserved)
	if unobserved.IsError || !strings.Contains(unobservedText, `"observation_state":"not_observed"`) || requests.Load() != 0 {
		t.Fatalf("unobserved context traffic=%d text=%s", requests.Load(), unobservedText)
	}

	tools := make([]*mcp.Tool, 0, 160)
	for i := 0; i < 160; i++ {
		tools = append(tools, &mcp.Tool{Name: fmt.Sprintf("observed_tool_%03d_%s", i, strings.Repeat("x", 24)), Description: "DESCRIPTION_SENTINEL_MUST_NOT_APPEAR"})
	}
	owner.recordSuccess(runtimeOwnerKey{environmentID: env.ID, mcpID: entry.ID}, tools, true)
	observed := callGatewayTool(t, ctx, session, "environment_context_bundle", map[string]any{"environment_id": env.ID, "max_output_bytes": 262144})
	observedText := toolText(t, observed)
	if observed.IsError || requests.Load() != 0 {
		t.Fatalf("observed context performed protocol traffic=%d text=%s", requests.Load(), observedText)
	}
	for _, required := range []string{`"observation_state":"healthy"`, `"tool_inventory_observed":true`, "observed_tool_000_", `"mcp_tool_names":32`, `"mcp_tool_name_limit"`} {
		if !strings.Contains(observedText, required) {
			t.Fatalf("observed context missing %q: %s", required, observedText)
		}
	}
	if strings.Contains(observedText, "DESCRIPTION_SENTINEL_MUST_NOT_APPEAR") {
		t.Fatalf("context leaked MCP tool descriptions: %s", observedText)
	}

	small := callGatewayTool(t, ctx, session, "environment_context_bundle", map[string]any{"environment_id": env.ID, "max_output_bytes": 4096})
	smallText := toolText(t, small)
	if small.IsError || requests.Load() != 0 || len(smallText) > 4600 || !strings.Contains(smallText, `"output_byte_limit"`) {
		t.Fatalf("small owner context traffic=%d len=%d text=%s", requests.Load(), len(smallText), smallText)
	}
}

func TestGatewayEnvironmentContextDoesNotLeakMemorySkillContentOrExecuteVerifier(t *testing.T) {
	service := app.New(filepath.Join(t.TempDir(), "state.json"))
	root := t.TempDir()
	ws, err := service.Workspaces.Add(root, "safe-context")
	if err != nil {
		t.Fatal(err)
	}
	env, err := service.Environments.Create(ws.ID, "safe-context", "")
	if err != nil {
		t.Fatal(err)
	}

	const skillSentinel = "SKILL_CONTENT_CONTEXT_SENTINEL"
	skillRoot := filepath.Join(t.TempDir(), "skills")
	skillDir := filepath.Join(skillRoot, "safe-skill")
	supportRoot := filepath.Join(t.TempDir(), "support")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(supportRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("# safe\n"+skillSentinel+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	skills, err := service.Skills.AddSkillRoot(skillRoot, []string{supportRoot}, false)
	if err != nil || len(skills) != 1 {
		t.Fatalf("skills=%+v err=%v", skills, err)
	}
	if _, err := service.SetEnvironmentSkill(env.ID, skills[0].ID, true); err != nil {
		t.Fatal(err)
	}

	const globalSecret = "GLOBAL_CONTEXT_MEMORY_SENTINEL"
	const privateSecret = "PRIVATE_CONTEXT_MEMORY_SENTINEL"
	if err := service.Memory.GlobalWrite("global-secret", globalSecret); err != nil {
		t.Fatal(err)
	}
	if err := service.Memory.EnvironmentWrite(env.ID, "private-secret", privateSecret); err != nil {
		t.Fatal(err)
	}

	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	if err := service.AllowExecutable(executable); err != nil {
		t.Fatal(err)
	}
	sideEffect := filepath.Join(t.TempDir(), "verifier-ran.txt")
	t.Setenv("ADM_CONTEXT_GATEWAY_VERIFIER_SIDE_EFFECT", sideEffect)
	definition, err := service.AddVerifier(env.ID, model.VerifierDefinition{Name: "must-not-run", Kind: verifier.KindCustom, Enabled: true, Executable: executable, Args: []string{"-test.run=^TestGatewayEnvironmentContextVerifierHelper$"}})
	if err != nil {
		t.Fatal(err)
	}
	writerBefore, err := service.Environments.AcquireWriter(env.ID, "existing-writer")
	if err != nil {
		t.Fatal(err)
	}

	owner := newRuntimeOwner(service)
	defer owner.Close()
	infoBefore := owner.Info()
	ctx := context.Background()
	session := connectInMemory(t, ctx, newServer(service, owner))
	defer session.Close()
	result := callGatewayTool(t, ctx, session, "environment_context_bundle", map[string]any{"environment_id": env.ID})
	text := toolText(t, result)
	if result.IsError {
		t.Fatalf("context failed: %s", text)
	}
	for _, required := range []string{skills[0].ID, skills[0].Name, definition.ID, "must-not-run", `"private_memory_count":1`, `"writer_present":true`, `"files.mutate"`, `"requires_writer":true`} {
		if !strings.Contains(text, required) {
			t.Fatalf("context missing %q: %s", required, text)
		}
	}
	for _, forbidden := range []string{skillSentinel, globalSecret, privateSecret, sideEffect, "-test.run=^TestGatewayEnvironmentContextVerifierHelper$", "existing-writer"} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("context leaked %q: %s", forbidden, text)
		}
	}
	if _, err := os.Stat(sideEffect); !os.IsNotExist(err) {
		t.Fatalf("context executed verifier: %v", err)
	}
	after, err := service.Environments.Get(env.ID)
	if err != nil {
		t.Fatal(err)
	}
	if after.Writer == nil || writerBefore.Writer == nil || !after.Writer.ExpiresAt.Equal(writerBefore.Writer.ExpiresAt) || after.Writer.LastSeenAt != writerBefore.Writer.LastSeenAt {
		t.Fatalf("context changed writer lease: before=%+v after=%+v", writerBefore.Writer, after.Writer)
	}
	infoAfter := owner.Info()
	if infoAfter.OwnedDevProcesses != infoBefore.OwnedDevProcesses || infoAfter.OwnedAgentRuns != infoBefore.OwnedAgentRuns || infoAfter.OwnedMCPSessions != infoBefore.OwnedMCPSessions {
		t.Fatalf("context changed owner resources: before=%+v after=%+v", infoBefore, infoAfter)
	}
}

func TestGatewayEnvironmentContextVerifierHelper(t *testing.T) {
	path := os.Getenv("ADM_CONTEXT_GATEWAY_VERIFIER_SIDE_EFFECT")
	if path == "" || !strings.Contains(strings.Join(os.Args, " "), "TestGatewayEnvironmentContextVerifierHelper") {
		return
	}
	if err := os.WriteFile(path, []byte("executed"), 0o644); err != nil {
		panic(err)
	}
}
