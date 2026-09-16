package gateway

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"ai-dev-manager-v2/internal/app"
	"ai-dev-manager-v2/internal/model"
	"ai-dev-manager-v2/internal/verifier"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestAsyncVerifierToolsSharedAgentAdminSurface(t *testing.T) {
	service, environmentID, _ := asyncVerifierTestService(t)
	owner := newRuntimeOwner(service)
	defer owner.Close()
	definition := addAsyncVerifier(t, service, environmentID, "surface-long", true, os.Args[0], "", 30)
	t.Setenv("ADM_TEST_ASYNC_VERIFIER_HELPER", "1")
	t.Setenv("ADM_TEST_ASYNC_VERIFIER_MODE", "long")

	httpServer := httptest.NewServer(newHTTPHandler(service, owner))
	defer httpServer.Close()
	ctx := context.Background()
	agent := connectHTTPWithRetry(t, ctx, httpServer.URL+"/mcp")
	defer agent.Close()
	admin := connectHTTPWithRetry(t, ctx, httpServer.URL+"/admin/mcp")
	defer admin.Close()

	for surface, session := range map[string]*mcp.ClientSession{"agent": agent, "admin": admin} {
		tools, err := session.ListTools(ctx, nil)
		if err != nil {
			t.Fatalf("%s list tools: %v", surface, err)
		}
		descriptions := map[string]string{}
		for _, tool := range tools.Tools {
			descriptions[tool.Name] = tool.Description
		}
		for _, name := range []string{"environment_verifier_run_start", "environment_verifier_run_list", "environment_verifier_run_status", "environment_verifier_run_cancel"} {
			if _, ok := descriptions[name]; !ok {
				t.Fatalf("%s surface missing %s", surface, name)
			}
		}
		if !strings.Contains(strings.ToLower(descriptions["environment_verifier_run_start"]), "long") {
			t.Fatalf("%s async start description does not prefer long verification: %q", surface, descriptions["environment_verifier_run_start"])
		}
		if !strings.Contains(strings.ToLower(descriptions["environment_verifier_run"]), "blocks") || !strings.Contains(descriptions["environment_verifier_run"], "environment_verifier_run_start") {
			t.Fatalf("%s blocking verifier description is incomplete: %q", surface, descriptions["environment_verifier_run"])
		}
	}

	started := callGatewayTool(t, ctx, agent, "environment_verifier_run_start", map[string]any{
		"environment_id":   environmentID,
		"writer_owner":     asyncVerifierTestWriter,
		"verifier_id":      definition.ID,
		"max_output_bytes": 4096,
	})
	if started.IsError {
		t.Fatalf("agent async verifier start: %s", toolText(t, started))
	}
	runID := verifierRunIDFromToolText(t, started)
	waitVerifierHTTPOutput(t, ctx, admin, environmentID, runID, "phase21-long-ready")

	adminStatus := callGatewayTool(t, ctx, admin, "environment_verifier_run_status", map[string]any{"environment_id": environmentID, "verifier_run_id": runID})
	if adminStatus.IsError || !strings.Contains(toolText(t, adminStatus), `"state":"running"`) {
		t.Fatalf("admin could not inspect agent-started verifier: %s", toolText(t, adminStatus))
	}
	listed := callGatewayTool(t, ctx, admin, "environment_verifier_run_list", map[string]any{"environment_id": environmentID})
	if listed.IsError || !strings.Contains(toolText(t, listed), runID) {
		t.Fatalf("admin could not list agent-started verifier: %s", toolText(t, listed))
	}
	canceled := callGatewayTool(t, ctx, agent, "environment_verifier_run_cancel", map[string]any{
		"environment_id":  environmentID,
		"writer_owner":    asyncVerifierTestWriter,
		"verifier_run_id": runID,
	})
	if canceled.IsError || !strings.Contains(toolText(t, canceled), `"state":"canceled"`) {
		t.Fatalf("agent cancel failed: %s", toolText(t, canceled))
	}
	beforeReadOnly, err := service.Environments.Get(environmentID)
	if err != nil || beforeReadOnly.Writer == nil {
		t.Fatalf("writer before read-only verifier observation: env=%+v err=%v", beforeReadOnly, err)
	}
	_ = callGatewayTool(t, ctx, admin, "environment_verifier_run_list", map[string]any{"environment_id": environmentID})
	_ = callGatewayTool(t, ctx, admin, "environment_verifier_run_status", map[string]any{"environment_id": environmentID, "verifier_run_id": runID})
	afterReadOnly, err := service.Environments.Get(environmentID)
	if err != nil || afterReadOnly.Writer == nil {
		t.Fatalf("writer after read-only verifier observation: env=%+v err=%v", afterReadOnly, err)
	}
	if !afterReadOnly.Writer.LastSeenAt.Equal(beforeReadOnly.Writer.LastSeenAt) || !afterReadOnly.Writer.ExpiresAt.Equal(beforeReadOnly.Writer.ExpiresAt) {
		t.Fatalf("read-only verifier list/status renewed writer: before=%+v after=%+v", beforeReadOnly.Writer, afterReadOnly.Writer)
	}
}

func TestAsyncVerifierRealHTTPDisconnectBoundsCancelAndRestart(t *testing.T) {
	service, environmentID, statePath := asyncVerifierTestService(t)
	definition := addAsyncVerifier(t, service, environmentID, "http-stream", true, os.Args[0], "", 10)
	timeoutDefinition := addAsyncVerifier(t, service, environmentID, "http-timeout", true, os.Args[0], "", 1)
	t.Setenv("ADM_TEST_ASYNC_VERIFIER_HELPER", "1")
	t.Setenv("ADM_TEST_ASYNC_VERIFIER_MODE", "stream-large")

	owner := newRuntimeOwner(service)
	server := httptest.NewServer(newHTTPHandler(service, owner))
	ctx := context.Background()
	clientA := connectHTTPWithRetry(t, ctx, server.URL+"/mcp")
	started := callGatewayTool(t, ctx, clientA, "environment_verifier_run_start", map[string]any{
		"environment_id":   environmentID,
		"writer_owner":     asyncVerifierTestWriter,
		"verifier_id":      definition.ID,
		"max_output_bytes": 64,
	})
	if started.IsError {
		t.Fatalf("HTTP verifier start failed: %s", toolText(t, started))
	}
	runID := verifierRunIDFromToolText(t, started)
	if !strings.HasPrefix(runID, "vfrun_") {
		t.Fatalf("unexpected verifier run identity %q", runID)
	}
	_ = clientA.Close()

	clientB := connectHTTPWithRetry(t, ctx, server.URL+"/mcp")
	listed := callGatewayTool(t, ctx, clientB, "environment_verifier_run_list", map[string]any{"environment_id": environmentID})
	if listed.IsError || !strings.Contains(toolText(t, listed), runID) {
		t.Fatalf("later client cannot list disconnected verifier %q: %s", runID, toolText(t, listed))
	}
	waitVerifierHTTPTruncatedOutput(t, ctx, clientB, environmentID, runID)
	terminal := waitVerifierHTTPTerminal(t, ctx, clientB, environmentID, runID, 8*time.Second)
	if terminal.State != verifierRunSucceeded || terminal.Result == nil || terminal.Result.Status != verifier.StatusPassed || terminal.Result.ExitCode != 0 {
		t.Fatalf("HTTP verifier terminal result lost semantics: %+v", terminal)
	}
	if len(terminal.Stdout) != 64 || len(terminal.Stderr) != 64 || !terminal.StdoutTruncated || !terminal.StderrTruncated {
		t.Fatalf("HTTP verifier terminal output not bounded: %+v", terminal)
	}

	t.Setenv("ADM_TEST_ASYNC_VERIFIER_MODE", "fail")
	failedStart := callGatewayTool(t, ctx, clientB, "environment_verifier_run_start", map[string]any{
		"environment_id": environmentID,
		"writer_owner":   asyncVerifierTestWriter,
		"verifier_id":    definition.ID,
	})
	if failedStart.IsError {
		t.Fatalf("HTTP failing verifier start failed: %s", toolText(t, failedStart))
	}
	failedStatus := waitVerifierHTTPTerminal(t, ctx, clientB, environmentID, verifierRunIDFromToolText(t, failedStart), 5*time.Second)
	if failedStatus.State != verifierRunFailed || failedStatus.Result == nil || failedStatus.Result.Status != verifier.StatusFailed || failedStatus.Result.ExitCode != 9 || failedStatus.Result.TimedOut {
		t.Fatalf("HTTP non-zero verifier lost classified semantics: %+v", failedStatus)
	}

	t.Setenv("ADM_TEST_ASYNC_VERIFIER_MODE", "timeout")
	timeoutStart := callGatewayTool(t, ctx, clientB, "environment_verifier_run_start", map[string]any{
		"environment_id": environmentID,
		"writer_owner":   asyncVerifierTestWriter,
		"verifier_id":    timeoutDefinition.ID,
	})
	if timeoutStart.IsError {
		t.Fatalf("HTTP timeout verifier start failed: %s", toolText(t, timeoutStart))
	}
	timeoutStatus := waitVerifierHTTPTerminal(t, ctx, clientB, environmentID, verifierRunIDFromToolText(t, timeoutStart), 5*time.Second)
	if timeoutStatus.State != verifierRunFailed || timeoutStatus.Result == nil || timeoutStatus.Result.Status != verifier.StatusFailed || !timeoutStatus.Result.TimedOut || timeoutStatus.Result.Summary != "verifier timed out" {
		t.Fatalf("HTTP configured verifier timeout lost classified semantics: %+v", timeoutStatus)
	}

	t.Setenv("ADM_TEST_ASYNC_VERIFIER_MODE", "long")
	cancelStart := callGatewayTool(t, ctx, clientB, "environment_verifier_run_start", map[string]any{
		"environment_id": environmentID,
		"writer_owner":   asyncVerifierTestWriter,
		"verifier_id":    definition.ID,
	})
	if cancelStart.IsError {
		t.Fatalf("cancel verifier start failed: %s", toolText(t, cancelStart))
	}
	cancelID := verifierRunIDFromToolText(t, cancelStart)
	waitVerifierHTTPOutput(t, ctx, clientB, environmentID, cancelID, "phase21-long-ready")
	wrong := callGatewayTool(t, ctx, clientB, "environment_verifier_run_cancel", map[string]any{
		"environment_id":  environmentID,
		"writer_owner":    "wrong-writer",
		"verifier_run_id": cancelID,
	})
	if !wrong.IsError {
		t.Fatalf("wrong writer canceled verifier: %s", toolText(t, wrong))
	}
	canceled := callGatewayTool(t, ctx, clientB, "environment_verifier_run_cancel", map[string]any{
		"environment_id":  environmentID,
		"writer_owner":    asyncVerifierTestWriter,
		"verifier_run_id": cancelID,
	})
	if canceled.IsError || !strings.Contains(toolText(t, canceled), `"state":"canceled"`) {
		t.Fatalf("matching writer cancel failed: %s", toolText(t, canceled))
	}
	postCancel := callGatewayTool(t, ctx, clientB, "environment_verifier_run_status", map[string]any{"environment_id": environmentID, "verifier_run_id": cancelID})
	if postCancel.IsError || !strings.Contains(toolText(t, postCancel), `"state":"canceled"`) {
		t.Fatalf("canceled verifier no longer queryable: %s", toolText(t, postCancel))
	}

	noVerifierRoot := t.TempDir()
	if err := os.WriteFile(filepath.Join(noVerifierRoot, "marker.txt"), []byte("plain-file-ok"), 0o644); err != nil {
		t.Fatal(err)
	}
	ws, err := service.Workspaces.Add(noVerifierRoot, "no-verifier")
	if err != nil {
		t.Fatal(err)
	}
	noVerifierEnv, err := service.Environments.Create(ws.ID, "no-verifier", "")
	if err != nil {
		t.Fatal(err)
	}
	read := callGatewayTool(t, ctx, clientB, "read", map[string]any{"environment_id": noVerifierEnv.ID, "path": "marker.txt"})
	if read.IsError || !strings.Contains(toolText(t, read), "plain-file-ok") {
		t.Fatalf("no-verifier Environment lost ordinary files: %s", toolText(t, read))
	}
	missing := callGatewayTool(t, ctx, clientB, "environment_verifier_run_start", map[string]any{
		"environment_id": noVerifierEnv.ID,
		"writer_owner":   "unused",
		"verifier_id":    "vf_missing",
	})
	if !missing.IsError {
		t.Fatalf("missing verifier unexpectedly started: %s", toolText(t, missing))
	}

	persisted, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{runID, cancelID, "vfrun_", "verifier_runs"} {
		if strings.Contains(string(persisted), forbidden) {
			t.Fatalf("verifier observation leaked into persisted state via %q", forbidden)
		}
	}

	restartStart := callGatewayTool(t, ctx, clientB, "environment_verifier_run_start", map[string]any{
		"environment_id": environmentID,
		"writer_owner":   asyncVerifierTestWriter,
		"verifier_id":    definition.ID,
	})
	if restartStart.IsError {
		t.Fatalf("restart verifier start failed: %s", toolText(t, restartStart))
	}
	restartID := verifierRunIDFromToolText(t, restartStart)
	waitVerifierHTTPOutput(t, ctx, clientB, environmentID, restartID, "phase21-long-ready")
	_ = clientB.Close()
	server.Close()
	if err := owner.Close(); err != nil {
		t.Fatal(err)
	}

	freshOwner := newRuntimeOwner(service)
	defer freshOwner.Close()
	freshServer := httptest.NewServer(newHTTPHandler(service, freshOwner))
	defer freshServer.Close()
	clientC := connectHTTPWithRetry(t, ctx, freshServer.URL+"/mcp")
	defer clientC.Close()
	freshList := callGatewayTool(t, ctx, clientC, "environment_verifier_run_list", map[string]any{"environment_id": environmentID})
	if freshList.IsError || strings.Contains(toolText(t, freshList), "vfrun_") {
		t.Fatalf("Gateway restart resurrected verifier runs: %s", toolText(t, freshList))
	}
	oldStatus := callGatewayTool(t, ctx, clientC, "environment_verifier_run_status", map[string]any{"environment_id": environmentID, "verifier_run_id": restartID})
	if !oldStatus.IsError {
		t.Fatalf("old verifier identity survived restart: %s", toolText(t, oldStatus))
	}
}

func TestAsyncVerifierSurfaceValidationInstallsNoGhostRuns(t *testing.T) {
	service, environmentID, _ := asyncVerifierTestService(t)
	owner := newRuntimeOwner(service)
	defer owner.Close()
	disabled := addAsyncVerifier(t, service, environmentID, "surface-disabled", false, os.Args[0], "", 5)
	forbidden := addAsyncVerifier(t, service, environmentID, "surface-forbidden", true, "definitely-not-allowed", "", 5)
	escaped := addAsyncVerifier(t, service, environmentID, "surface-escaped", true, os.Args[0], "../escape", 5)
	t.Setenv("ADM_TEST_ASYNC_VERIFIER_HELPER", "1")
	t.Setenv("ADM_TEST_ASYNC_VERIFIER_MODE", "success")

	server := httptest.NewServer(newHTTPHandler(service, owner))
	defer server.Close()
	ctx := context.Background()
	client := connectHTTPWithRetry(t, ctx, server.URL+"/mcp")
	defer client.Close()
	before := callGatewayTool(t, ctx, client, "environment_verifier_run_list", map[string]any{"environment_id": environmentID})
	if before.IsError {
		t.Fatalf("initial verifier list failed: %s", toolText(t, before))
	}

	cases := []map[string]any{
		{"environment_id": environmentID, "writer_owner": asyncVerifierTestWriter, "verifier_id": "vf_missing"},
		{"environment_id": environmentID, "writer_owner": asyncVerifierTestWriter, "verifier_id": disabled.ID},
		{"environment_id": environmentID, "writer_owner": "wrong-writer", "verifier_id": escaped.ID},
		{"environment_id": environmentID, "writer_owner": asyncVerifierTestWriter, "verifier_id": forbidden.ID},
		{"environment_id": environmentID, "writer_owner": asyncVerifierTestWriter, "verifier_id": escaped.ID},
	}
	for _, args := range cases {
		result := callGatewayTool(t, ctx, client, "environment_verifier_run_start", args)
		if !result.IsError {
			t.Fatalf("invalid verifier start unexpectedly succeeded: args=%+v result=%s", args, toolText(t, result))
		}
	}
	after := callGatewayTool(t, ctx, client, "environment_verifier_run_list", map[string]any{"environment_id": environmentID})
	if after.IsError || toolText(t, after) != toolText(t, before) {
		t.Fatalf("invalid starts changed owner resources: before=%s after=%s", toolText(t, before), toolText(t, after))
	}
}

func waitVerifierHTTPOutput(t *testing.T, ctx context.Context, session *mcp.ClientSession, environmentID, runID, marker string) verifierRunStatus {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		status := verifierRunStatusFromTool(t, callGatewayTool(t, ctx, session, "environment_verifier_run_status", map[string]any{"environment_id": environmentID, "verifier_run_id": runID}))
		if strings.Contains(status.Stdout, marker) {
			return status
		}
		if status.State != verifierRunRunning {
			t.Fatalf("verifier %s ended before marker %q: %+v", runID, marker, status)
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("verifier %s did not emit %q", runID, marker)
	return verifierRunStatus{}
}

func waitVerifierHTTPTruncatedOutput(t *testing.T, ctx context.Context, session *mcp.ClientSession, environmentID, runID string) verifierRunStatus {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		status := verifierRunStatusFromTool(t, callGatewayTool(t, ctx, session, "environment_verifier_run_status", map[string]any{"environment_id": environmentID, "verifier_run_id": runID}))
		if status.State == verifierRunRunning && status.StdoutTruncated && status.StderrTruncated {
			if len(status.Stdout) != 64 || len(status.Stderr) != 64 {
				t.Fatalf("running output exceeded requested bounds: %+v", status)
			}
			return status
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("verifier %s never exposed bounded live output", runID)
	return verifierRunStatus{}
}

func waitVerifierHTTPTerminal(t *testing.T, ctx context.Context, session *mcp.ClientSession, environmentID, runID string, timeout time.Duration) verifierRunStatus {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		status := verifierRunStatusFromTool(t, callGatewayTool(t, ctx, session, "environment_verifier_run_status", map[string]any{"environment_id": environmentID, "verifier_run_id": runID}))
		if status.State != verifierRunRunning {
			return status
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("verifier %s did not reach terminal state", runID)
	return verifierRunStatus{}
}

func verifierRunStatusFromTool(t *testing.T, result *mcp.CallToolResult) verifierRunStatus {
	t.Helper()
	if result.IsError {
		t.Fatalf("verifier status tool error: %s", toolText(t, result))
	}
	var envelope struct {
		Result verifierRunStatus `json:"result"`
	}
	if err := json.Unmarshal([]byte(toolText(t, result)), &envelope); err != nil {
		t.Fatalf("decode verifier status: %v body=%s", err, toolText(t, result))
	}
	return envelope.Result
}

func verifierRunIDFromToolText(t *testing.T, result *mcp.CallToolResult) string {
	t.Helper()
	status := verifierRunStatusFromTool(t, result)
	if status.ID == "" {
		t.Fatalf("verifier start missing identity: %s", toolText(t, result))
	}
	return status.ID
}

func TestAsyncVerifierManagedRootValidationInstallsNoRun(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not available")
	}
	source := gatewayTestGitRepo(t)
	service := app.New(filepath.Join(t.TempDir(), "adm", "state.json"))
	ws, err := service.Workspaces.Add(source, "managed-verifier-source")
	if err != nil {
		t.Fatal(err)
	}
	created, err := service.CreateManagedWorktree(context.Background(), ws.ID, "managed-verifier", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	managed := created.ManagedWorktree
	defer func() {
		_, _ = gatewayGitCommand(source, "worktree", "remove", "--force", managed.Root)
		_, _ = gatewayGitCommand(source, "branch", "-D", managed.Branch)
	}()
	if err := service.AllowExecutable(os.Args[0]); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Environments.AcquireWriter(created.Environment.ID, asyncVerifierTestWriter); err != nil {
		t.Fatal(err)
	}
	definition, err := service.AddVerifier(created.Environment.ID, model.VerifierDefinition{
		Name: "managed-verifier", Kind: verifier.KindTest, Enabled: true, Executable: os.Args[0], Args: []string{"-test.run=^TestAsyncVerifierCommandHelper$"}, TimeoutSeconds: 5,
	})
	if err != nil {
		t.Fatal(err)
	}
	gatewayGitRun(t, managed.Root, "checkout", "-b", "tampered-verifier-branch")

	owner := newRuntimeOwner(service)
	defer owner.Close()
	if _, err := owner.StartVerifierRun(created.Environment.ID, asyncVerifierTestWriter, definition.ID, 4096); err == nil || !strings.Contains(err.Error(), "branch changed") {
		t.Fatalf("tampered managed root accepted: %v", err)
	}
	listed, err := owner.ListVerifierRuns(created.Environment.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(listed) != 0 {
		t.Fatalf("tampered managed root installed verifier run: %+v", listed)
	}
}
