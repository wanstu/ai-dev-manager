package gateway

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"ai-dev-manager-v2/internal/app"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const agentRunTestWriter = "phase8-run-writer"

func TestAgentRunLifecycleAcrossAgentSessions(t *testing.T) {
	service, environmentID, root := agentRunTestService(t)
	portFile := filepath.Join(root, "run.port")
	t.Setenv("ADM_TEST_DEV_PROCESS_HELPER", "1")
	t.Setenv("ADM_TEST_DEV_PROCESS_PORT_FILE", portFile)

	owner := newRuntimeOwner(service)
	defer owner.Close()
	server := newServer(service, owner)
	ctx := context.Background()
	first := connectInMemory(t, ctx, server)

	started := callGatewayTool(t, ctx, first, "run_start", map[string]any{
		"environment_id":   environmentID,
		"writer_owner":     agentRunTestWriter,
		"executable":       os.Args[0],
		"args":             []string{"-test.run=^TestDevProcessHelper$"},
		"timeout_ms":       30000,
		"max_output_bytes": 4096,
	})
	if started.IsError {
		t.Fatalf("run_start failed: %s", toolText(t, started))
	}
	runID := runIDFromToolText(t, started)
	if !strings.HasPrefix(runID, "run_") {
		t.Fatalf("run id=%q", runID)
	}
	_ = first.Close()

	port := waitDevProcessPortFile(t, portFile)
	second := connectInMemory(t, ctx, server)
	defer second.Close()
	listed := callGatewayTool(t, ctx, second, "run_list", map[string]any{"environment_id": environmentID})
	if listed.IsError || !strings.Contains(toolText(t, listed), runID) {
		t.Fatalf("later client cannot see run %q: %s", runID, toolText(t, listed))
	}
	status := callGatewayTool(t, ctx, second, "run_status", map[string]any{"environment_id": environmentID, "run_id": runID})
	if status.IsError || !strings.Contains(toolText(t, status), `"state":"running"`) {
		t.Fatalf("later client run status failed: %s", toolText(t, status))
	}
	if owner.Info().OwnedAgentRuns != 1 {
		t.Fatalf("owner info=%+v", owner.Info())
	}

	wrongWriter := callGatewayTool(t, ctx, second, "run_cancel", map[string]any{
		"environment_id": environmentID,
		"writer_owner":   "wrong-writer",
		"run_id":         runID,
	})
	if !wrongWriter.IsError {
		t.Fatalf("wrong writer canceled run: %s", toolText(t, wrongWriter))
	}

	canceled := callGatewayTool(t, ctx, second, "run_cancel", map[string]any{
		"environment_id": environmentID,
		"writer_owner":   agentRunTestWriter,
		"run_id":         runID,
	})
	if canceled.IsError || !strings.Contains(toolText(t, canceled), `"state":"canceled"`) {
		t.Fatalf("run_cancel failed: %s", toolText(t, canceled))
	}
	waitPortReleased(t, port)
	if owner.Info().OwnedAgentRuns != 0 {
		t.Fatalf("canceled run remained active: %+v", owner.Info())
	}
}

func TestAgentRunTerminalStatesAndAuthority(t *testing.T) {
	service, environmentID, _ := agentRunTestService(t)
	owner := newRuntimeOwner(service)
	defer owner.Close()

	t.Setenv("ADM_TEST_AGENT_RUN_HELPER", "1")
	t.Setenv("ADM_TEST_AGENT_RUN_MODE", "success")
	succeeded, err := owner.StartAgentRun(environmentID, agentRunTestWriter, os.Args[0], []string{"-test.run=^TestAgentRunCommandHelper$"}, "", 5000, 4096)
	if err != nil {
		t.Fatal(err)
	}
	succeeded = waitAgentRunTerminal(t, owner, environmentID, succeeded.ID)
	if succeeded.State != agentRunSucceeded || succeeded.ExitCode == nil || *succeeded.ExitCode != 0 || !strings.Contains(succeeded.Stdout, "phase8-success") {
		t.Fatalf("unexpected success status: %+v", succeeded)
	}

	t.Setenv("ADM_TEST_AGENT_RUN_MODE", "fail")
	failed, err := owner.StartAgentRun(environmentID, agentRunTestWriter, os.Args[0], []string{"-test.run=^TestAgentRunCommandHelper$"}, "", 5000, 4096)
	if err != nil {
		t.Fatal(err)
	}
	failed = waitAgentRunTerminal(t, owner, environmentID, failed.ID)
	if failed.State != agentRunFailed || failed.ExitCode == nil || *failed.ExitCode != 7 || failed.ErrorKind != "command_failed" || !strings.Contains(failed.Stderr, "phase8-fail") {
		t.Fatalf("unexpected failure status: %+v", failed)
	}

	t.Setenv("ADM_TEST_AGENT_RUN_MODE", "timeout")
	timedOut, err := owner.StartAgentRun(environmentID, agentRunTestWriter, os.Args[0], []string{"-test.run=^TestAgentRunCommandHelper$"}, "", 50, 4096)
	if err != nil {
		t.Fatal(err)
	}
	timedOut = waitAgentRunTerminal(t, owner, environmentID, timedOut.ID)
	if timedOut.State != agentRunFailed || timedOut.ErrorKind != "timeout" {
		t.Fatalf("unexpected timeout status: %+v", timedOut)
	}

	t.Setenv("ADM_TEST_AGENT_RUN_MODE", "large")
	bounded, err := owner.StartAgentRun(environmentID, agentRunTestWriter, os.Args[0], []string{"-test.run=^TestAgentRunCommandHelper$"}, "", 5000, 64)
	if err != nil {
		t.Fatal(err)
	}
	bounded = waitAgentRunTerminal(t, owner, environmentID, bounded.ID)
	if bounded.State != agentRunSucceeded || len(bounded.Stdout) != 64 {
		t.Fatalf("unexpected bounded output status: state=%s stdout_len=%d", bounded.State, len(bounded.Stdout))
	}

	before, err := owner.ListAgentRuns(environmentID)
	if err != nil {
		t.Fatal(err)
	}
	for name, call := range map[string]func() error{
		"forbidden executable": func() error {
			_, err := owner.StartAgentRun(environmentID, agentRunTestWriter, "definitely-not-allowed", nil, "", 1000, 1024)
			return err
		},
		"escaped cwd": func() error {
			_, err := owner.StartAgentRun(environmentID, agentRunTestWriter, os.Args[0], []string{"-test.run=^$"}, "../escape", 1000, 1024)
			return err
		},
	} {
		t.Run(name, func(t *testing.T) {
			if err := call(); err == nil {
				t.Fatal("unsafe run start unexpectedly succeeded")
			}
		})
	}
	after, err := owner.ListAgentRuns(environmentID)
	if err != nil {
		t.Fatal(err)
	}
	if len(after) != len(before) {
		t.Fatalf("failed authority check installed run: before=%d after=%d", len(before), len(after))
	}
}

func TestAgentRunStatusIncludesRunningOutput(t *testing.T) {
	service, environmentID, _ := agentRunTestService(t)
	owner := newRuntimeOwner(service)
	defer owner.Close()

	t.Setenv("ADM_TEST_AGENT_RUN_HELPER", "1")
	t.Setenv("ADM_TEST_AGENT_RUN_MODE", "stream_large")
	started, err := owner.StartAgentRun(environmentID, agentRunTestWriter, os.Args[0], []string{"-test.run=^TestAgentRunCommandHelper$"}, "", 30000, 64)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _, _ = owner.CancelAgentRun(environmentID, agentRunTestWriter, started.ID) }()

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		status, err := owner.AgentRunStatus(environmentID, started.ID)
		if err != nil {
			t.Fatal(err)
		}
		if status.State == agentRunRunning && strings.Contains(status.Stdout, "phase8-stream") {
			if len(status.Stdout) != 64 || !status.StdoutTruncated {
				t.Fatalf("running output was not bounded: stdout_len=%d truncated=%v status=%+v", len(status.Stdout), status.StdoutTruncated, status)
			}
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("running status never exposed bounded live stdout for run %s", started.ID)
}

func TestAgentRunDropEnvironmentCancelsAndForgets(t *testing.T) {
	service, environmentID, root := agentRunTestService(t)
	portFile := filepath.Join(root, "drop-run.port")
	t.Setenv("ADM_TEST_DEV_PROCESS_HELPER", "1")
	t.Setenv("ADM_TEST_DEV_PROCESS_PORT_FILE", portFile)
	owner := newRuntimeOwner(service)
	defer owner.Close()

	started, err := owner.StartAgentRun(environmentID, agentRunTestWriter, os.Args[0], []string{"-test.run=^TestDevProcessHelper$"}, "", 30000, 4096)
	if err != nil {
		t.Fatal(err)
	}
	port := waitDevProcessPortFile(t, portFile)
	owner.DropEnvironment(environmentID)
	waitPortReleased(t, port)
	if _, err := owner.AgentRunStatus(environmentID, started.ID); err == nil {
		t.Fatalf("dropped Environment retained run %q", started.ID)
	}
}

func TestAgentRunOwnerCloseCancelsActiveRun(t *testing.T) {
	service, environmentID, root := agentRunTestService(t)
	portFile := filepath.Join(root, "close-run.port")
	t.Setenv("ADM_TEST_DEV_PROCESS_HELPER", "1")
	t.Setenv("ADM_TEST_DEV_PROCESS_PORT_FILE", portFile)
	owner := newRuntimeOwner(service)
	started, err := owner.StartAgentRun(environmentID, agentRunTestWriter, os.Args[0], []string{"-test.run=^TestDevProcessHelper$"}, "", 30000, 4096)
	if err != nil {
		t.Fatal(err)
	}
	if started.ID == "" {
		t.Fatal("missing run identity")
	}
	port := waitDevProcessPortFile(t, portFile)
	if err := owner.Close(); err != nil {
		t.Fatal(err)
	}
	waitPortReleased(t, port)
	if err := owner.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestAgentRunStartRenewsWriterLease(t *testing.T) {
	service, environmentID, _ := agentRunTestService(t)
	before, err := service.Environments.Get(environmentID)
	if err != nil || before.Writer == nil {
		t.Fatalf("writer before run: env=%+v err=%v", before, err)
	}
	time.Sleep(20 * time.Millisecond)
	owner := newRuntimeOwner(service)
	defer owner.Close()
	t.Setenv("ADM_TEST_AGENT_RUN_HELPER", "1")
	t.Setenv("ADM_TEST_AGENT_RUN_MODE", "success")
	started, err := owner.StartAgentRun(environmentID, agentRunTestWriter, os.Args[0], []string{"-test.run=^TestAgentRunCommandHelper$"}, "", 5000, 1024)
	if err != nil {
		t.Fatal(err)
	}
	_ = waitAgentRunTerminal(t, owner, environmentID, started.ID)
	after, err := service.Environments.Get(environmentID)
	if err != nil || after.Writer == nil {
		t.Fatalf("writer after run: env=%+v err=%v", after, err)
	}
	if !after.Writer.LastSeenAt.After(before.Writer.LastSeenAt) || !after.Writer.ExpiresAt.After(before.Writer.ExpiresAt) {
		t.Fatalf("run_start did not renew writer lease: before=%+v after=%+v", before.Writer, after.Writer)
	}
}

func agentRunTestService(t *testing.T) (*app.Service, string, string) {
	t.Helper()
	root := t.TempDir()
	service := app.New(filepath.Join(t.TempDir(), "state.json"))
	workspace, err := service.Workspaces.Add(root, "phase8-run")
	if err != nil {
		t.Fatal(err)
	}
	environment, err := service.Environments.Create(workspace.ID, "phase8-run", "")
	if err != nil {
		t.Fatal(err)
	}
	if err := service.AllowExecutable(os.Args[0]); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Environments.AcquireWriter(environment.ID, agentRunTestWriter); err != nil {
		t.Fatal(err)
	}
	return service, environment.ID, root
}

func waitAgentRunTerminal(t *testing.T, owner *runtimeOwner, environmentID, runID string) agentRunStatus {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		status, err := owner.AgentRunStatus(environmentID, runID)
		if err != nil {
			t.Fatal(err)
		}
		if status.State != agentRunRunning {
			return status
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("run %s did not reach terminal state", runID)
	return agentRunStatus{}
}

func runIDFromToolText(t *testing.T, result *mcp.CallToolResult) string {
	t.Helper()
	text := toolText(t, result)
	marker := `"id":"`
	start := strings.Index(text, marker)
	if start < 0 {
		t.Fatalf("run result missing id: %s", text)
	}
	start += len(marker)
	end := strings.Index(text[start:], `"`)
	if end < 0 {
		t.Fatalf("run result has malformed id: %s", text)
	}
	return text[start : start+end]
}

func TestAgentRunCommandHelper(t *testing.T) {
	if os.Getenv("ADM_TEST_AGENT_RUN_HELPER") != "1" {
		return
	}
	switch os.Getenv("ADM_TEST_AGENT_RUN_MODE") {
	case "fail":
		fmt.Fprintln(os.Stderr, "phase8-fail")
		os.Exit(7)
	case "timeout":
		time.Sleep(2 * time.Second)
	case "large":
		fmt.Fprint(os.Stdout, strings.Repeat("x", 4096))
	case "stream_large":
		fmt.Fprint(os.Stdout, "phase8-stream "+strings.Repeat("x", 4096))
		time.Sleep(2 * time.Second)
	default:
		fmt.Fprintln(os.Stdout, "phase8-success")
	}
}
