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
	if !strings.Contains(text, `"execution_mode":"sandbox"`) || !strings.Contains(text, "sandbox-exec-active") {
		t.Fatalf("strong Agent exec did not report/use sandbox mode: %s", text)
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

func TestAgentExplicitSandboxExecDoesNotRequireStrongPermissionControl(t *testing.T) {
	service, environmentID := agentSandboxExecTestService(t)
	if _, err := service.SetStrongPermissionControl(false); err != nil {
		t.Fatal(err)
	}
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
		"execution_mode":   "sandbox",
	})
	text := toolText(t, result)
	if !service.SandboxExecutionAvailable() {
		if !result.IsError || !strings.Contains(text, "strong sandbox execution is unavailable") {
			t.Fatalf("explicit sandbox exec must fail closed when sandbox is unavailable: %s", text)
		}
		return
	}
	if result.IsError {
		t.Fatalf("explicit sandbox exec failed while Strong is disabled: %s", text)
	}
	if !strings.Contains(text, `"execution_mode":"sandbox"`) || !strings.Contains(text, "sandbox-exec-active") {
		t.Fatalf("explicit sandbox exec did not report/use sandbox mode: %s", text)
	}
}

func TestAgentExplicitSandboxRunDoesNotRequireStrongPermissionControl(t *testing.T) {
	service, environmentID := agentSandboxExecTestService(t)
	if _, err := service.SetStrongPermissionControl(false); err != nil {
		t.Fatal(err)
	}
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
		"execution_mode":   "sandbox",
	})
	text := toolText(t, started)
	if !service.SandboxExecutionAvailable() {
		if !started.IsError || !strings.Contains(text, "strong sandbox execution is unavailable") {
			t.Fatalf("explicit sandbox run must fail closed when sandbox is unavailable: %s", text)
		}
		return
	}
	if started.IsError {
		t.Fatalf("explicit sandbox run failed while Strong is disabled: %s", text)
	}
	runID := runIDFromToolText(t, started)
	status := waitAgentRunTerminal(t, owner, environmentID, runID)
	if status.State != agentRunSucceeded || status.ExecutionMode != "sandbox" {
		t.Fatalf("explicit sandbox run terminal status=%+v", status)
	}
	if !strings.Contains(status.Stdout, "sandbox-exec-active") {
		t.Fatalf("explicit sandbox run did not execute inside sandbox: %+v", status)
	}
}

func TestAgentStrongExplicitHostIsRejected(t *testing.T) {
	service, environmentID := agentSandboxExecTestService(t)
	if _, err := service.SetStrongPermissionControl(true); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	agent := connectInMemory(t, ctx, New(service))
	defer agent.Close()

	result := callGatewayTool(t, ctx, agent, "exec", map[string]any{
		"environment_id": environmentID,
		"writer_owner":   "sandbox-exec-writer",
		"executable":     os.Args[0],
		"execution_mode": "host",
	})
	if !result.IsError || !strings.Contains(toolText(t, result), "execution_mode=host is not allowed") {
		t.Fatalf("Strong Agent explicit host exec was not rejected: %s", toolText(t, result))
	}

	owner := newRuntimeOwner(service)
	defer owner.Close()
	runAgent := connectInMemory(t, ctx, newServerForSurface(service, owner, serverSurfaceAgent))
	defer runAgent.Close()
	started := callGatewayTool(t, ctx, runAgent, "run_start", map[string]any{
		"environment_id": environmentID,
		"writer_owner":   "sandbox-exec-writer",
		"executable":     os.Args[0],
		"execution_mode": "host",
	})
	if !started.IsError || !strings.Contains(toolText(t, started), "execution_mode=host is not allowed") {
		t.Fatalf("Strong Agent explicit host run was not rejected: %s", toolText(t, started))
	}
}

func TestAgentRelaxedAutoExecUsesHost(t *testing.T) {
	service, environmentID := agentSandboxExecTestService(t)
	if _, err := service.SetStrongPermissionControl(false); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	agent := connectInMemory(t, ctx, New(service))
	defer agent.Close()

	result := callGatewayTool(t, ctx, agent, "exec", map[string]any{
		"environment_id":   environmentID,
		"writer_owner":     "sandbox-exec-writer",
		"executable":       os.Args[0],
		"args":             []string{"-test.run=^TestAgentHostExecChildHelper$", "-test.v"},
		"timeout_ms":       15000,
		"max_output_bytes": 8192,
		"execution_mode":   "auto",
	})
	text := toolText(t, result)
	if result.IsError {
		t.Fatalf("relaxed auto exec failed: %s", text)
	}
	if !strings.Contains(text, `"execution_mode":"host"`) || !strings.Contains(text, "host-exec-active") {
		t.Fatalf("relaxed auto exec did not report/use host mode: %s", text)
	}
}

func TestResolveCommandExecutionModeRejectsInvalidMode(t *testing.T) {
	service := app.New(filepath.Join(t.TempDir(), "state.json"))
	if _, err := resolveCommandExecutionMode(service, serverSurfaceAgent, "container"); err == nil || !strings.Contains(err.Error(), "auto, host, or sandbox") {
		t.Fatalf("invalid execution mode was not rejected: %v", err)
	}
}

func TestAgentSandboxExecChildHelper(t *testing.T) {
	if os.Getenv("ADM_SANDBOX_ACTIVE") != "1" {
		t.Skip("sandbox child helper")
	}
	fmt.Println("sandbox-exec-active")
}

func TestAgentHostExecChildHelper(t *testing.T) {
	if os.Getenv("ADM_SANDBOX_ACTIVE") == "1" {
		t.Fatal("host child unexpectedly ran inside sandbox")
	}
	fmt.Println("host-exec-active")
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
