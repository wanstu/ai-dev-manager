package gateway

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ai-dev-manager-v2/internal/app"
)

func TestAgentStrongExecRoutesToSandboxWithoutHostFallback(t *testing.T) {
	service, environmentID := agentSandboxExecTestService(t)
	ctx := context.Background()
	agent := connectInMemory(t, ctx, New(service))
	defer agent.Close()

	result := callGatewayTool(t, ctx, agent, "exec", map[string]any{
		"environment_id":   environmentID,
		"writer_owner":     "sandbox-exec-writer",
		"executable":       os.Args[0],
		"args":             []string{"-test.run=^TestAgentSandboxExecChildHelper$", "-test.v"},
		"timeout_ms":       15000,
		"max_output_bytes": 8192,
	})
	text := toolText(t, result)
	if !service.SandboxExecutionAvailable() {
		if !result.IsError || !strings.Contains(text, "strong sandbox execution is unavailable") {
			t.Fatalf("strong Agent exec must fail closed when sandbox is unavailable: %s", text)
		}
		return
	}
	if result.IsError {
		t.Fatalf("strong Agent exec failed: %s", text)
	}
	if !strings.Contains(text, "sandbox-exec-active") {
		t.Fatalf("strong Agent exec did not execute inside sandbox: %s", text)
	}
}

func TestAgentStrongRunRoutesToSandboxWithoutHostFallback(t *testing.T) {
	service, environmentID := agentSandboxExecTestService(t)
	owner := newRuntimeOwner(service)
	defer owner.Close()
	ctx := context.Background()
	agent := connectInMemory(t, ctx, newServerForSurface(service, owner, serverSurfaceAgent))
	defer agent.Close()

	started := callGatewayTool(t, ctx, agent, "run_start", map[string]any{
		"environment_id":   environmentID,
		"writer_owner":     "sandbox-exec-writer",
		"executable":       os.Args[0],
		"args":             []string{"-test.run=^TestAgentSandboxExecChildHelper$", "-test.v"},
		"timeout_ms":       15000,
		"max_output_bytes": 8192,
	})
	text := toolText(t, started)
	if !service.SandboxExecutionAvailable() {
		if !started.IsError || !strings.Contains(text, "strong sandbox execution is unavailable") {
			t.Fatalf("strong Agent run must fail closed when sandbox is unavailable: %s", text)
		}
		return
	}
	if started.IsError {
		t.Fatalf("strong Agent run_start failed: %s", text)
	}
	runID := runIDFromToolText(t, started)
	status := waitAgentRunTerminal(t, owner, environmentID, runID)
	if status.State != agentRunSucceeded {
		t.Fatalf("sandbox run terminal state=%s message=%q stderr=%q", status.State, status.Message, status.Stderr)
	}
	if status.ExecutionMode != "sandbox" {
		t.Fatalf("sandbox run execution mode=%q", status.ExecutionMode)
	}
	if !strings.Contains(status.Stdout, "sandbox-exec-active") {
		t.Fatalf("sandbox run did not expose sandbox child output: %+v", status)
	}
}

func TestAgentSandboxExecChildHelper(t *testing.T) {
	if os.Getenv("ADM_SANDBOX_ACTIVE") != "1" {
		t.Skip("sandbox child helper")
	}
	fmt.Println("sandbox-exec-active")
}

func agentSandboxExecTestService(t *testing.T) (*app.Service, string) {
	t.Helper()
	root := t.TempDir()
	service := app.New(filepath.Join(t.TempDir(), "state.json"))
	workspace, err := service.Workspaces.Add(root, "sandbox-exec")
	if err != nil {
		t.Fatal(err)
	}
	environment, err := service.Environments.Create(workspace.ID, "sandbox-exec", "")
	if err != nil {
		t.Fatal(err)
	}
	if err := service.AllowExecutable(os.Args[0]); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Environments.AcquireWriter(environment.ID, "sandbox-exec-writer"); err != nil {
		t.Fatal(err)
	}
	return service, environment.ID
}
