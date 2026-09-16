package gateway

import (
	"context"
	"errors"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"ai-dev-manager-v2/internal/app"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestAgentRunLifecycleAcrossRealGatewayRestart(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "state.json")
	projectRoot := t.TempDir()
	service := app.New(statePath)
	workspace, err := service.Workspaces.Add(projectRoot, "phase8-real")
	if err != nil {
		t.Fatal(err)
	}
	environment, err := service.Environments.Create(workspace.ID, "phase8-real", "")
	if err != nil {
		t.Fatal(err)
	}
	if err := service.AllowExecutable(os.Args[0]); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Environments.AcquireWriter(environment.ID, agentRunTestWriter); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(projectRoot, "marker.txt"), []byte("phase8-file-still-works\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	listen := listener.Addr().String()
	_ = listener.Close()
	endpoint := "http://" + listen + "/mcp"

	portFile := filepath.Join(projectRoot, "phase8-real.port")
	t.Setenv("ADM_TEST_DEV_PROCESS_HELPER", "1")
	t.Setenv("ADM_TEST_DEV_PROCESS_PORT_FILE", portFile)

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
			t.Fatalf("Gateway missing owner identity: %+v", status)
		}
		return cmd, status
	}
	stopGateway := func(cmd *exec.Cmd) {
		stopped, err := StopHTTP(listen)
		if err != nil {
			t.Fatal(err)
		}
		if stopped.State != HTTPStateStopped {
			t.Fatalf("Gateway did not stop: %+v", stopped)
		}
		if err := cmd.Wait(); err != nil {
			t.Fatalf("Gateway process did not exit cleanly: %v", err)
		}
		running = nil
	}

	firstGateway, firstOwner := startGateway()
	ctx := context.Background()
	firstClient := connectHTTPWithRetry(t, ctx, endpoint)
	started := callGatewayTool(t, ctx, firstClient, "run_start", map[string]any{
		"environment_id":   environment.ID,
		"writer_owner":     agentRunTestWriter,
		"executable":       os.Args[0],
		"args":             []string{"-test.run=^TestDevProcessHelper$"},
		"timeout_ms":       30000,
		"max_output_bytes": 4096,
	})
	if started.IsError {
		t.Fatalf("real run_start failed: %s", toolText(t, started))
	}
	firstRunID := runIDFromToolText(t, started)
	_ = firstClient.Close()

	firstPort := waitDevProcessPortFile(t, portFile)
	secondClient := connectHTTPWithRetry(t, ctx, endpoint)
	listed := callGatewayTool(t, ctx, secondClient, "run_list", map[string]any{"environment_id": environment.ID})
	if listed.IsError || !strings.Contains(toolText(t, listed), firstRunID) {
		t.Fatalf("later HTTP client cannot see %s: %s", firstRunID, toolText(t, listed))
	}
	status := callGatewayTool(t, ctx, secondClient, "run_status", map[string]any{"environment_id": environment.ID, "run_id": firstRunID})
	if status.IsError || !strings.Contains(toolText(t, status), `"state":"running"`) {
		t.Fatalf("later HTTP client cannot inspect running Run: %s", toolText(t, status))
	}

	stateBytes, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{firstRunID, "owned_agent_runs", "agent_runs"} {
		if strings.Contains(string(stateBytes), forbidden) {
			t.Fatalf("Run observation leaked into persisted state via %q", forbidden)
		}
	}

	canceled := callGatewayTool(t, ctx, secondClient, "run_cancel", map[string]any{
		"environment_id": environment.ID,
		"writer_owner":   agentRunTestWriter,
		"run_id":         firstRunID,
	})
	if canceled.IsError || !strings.Contains(toolText(t, canceled), `"state":"canceled"`) {
		t.Fatalf("real run_cancel failed: %s", toolText(t, canceled))
	}
	waitPortReleased(t, firstPort)

	if err := os.Remove(portFile); err != nil && !errors.Is(err, os.ErrNotExist) {
		t.Fatal(err)
	}
	secondStarted := callGatewayTool(t, ctx, secondClient, "run_start", map[string]any{
		"environment_id": environment.ID,
		"writer_owner":   agentRunTestWriter,
		"executable":     os.Args[0],
		"args":           []string{"-test.run=^TestDevProcessHelper$"},
		"timeout_ms":     30000,
	})
	if secondStarted.IsError {
		t.Fatalf("second run_start failed: %s", toolText(t, secondStarted))
	}
	secondRunID := runIDFromToolText(t, secondStarted)
	secondPort := waitDevProcessPortFile(t, portFile)
	_ = secondClient.Close()

	stopGateway(firstGateway)
	waitPortReleased(t, secondPort)

	secondGateway, secondOwner := startGateway()
	if secondOwner.OwnerID == firstOwner.OwnerID {
		t.Fatalf("Gateway restart reused owner %q", firstOwner.OwnerID)
	}
	restartedClient := connectHTTPWithRetry(t, ctx, endpoint)
	listed = callGatewayTool(t, ctx, restartedClient, "run_list", map[string]any{"environment_id": environment.ID})
	listedText := toolText(t, listed)
	if listed.IsError || strings.Contains(listedText, firstRunID) || strings.Contains(listedText, secondRunID) || strings.Contains(listedText, "run_") {
		t.Fatalf("restart resurrected owner-local Runs: %s", listedText)
	}
	oldStatus, err := restartedClient.CallTool(ctx, &mcp.CallToolParams{Name: "run_status", Arguments: map[string]any{
		"environment_id": environment.ID,
		"run_id":         secondRunID,
	}})
	if err != nil {
		t.Fatalf("old Run status transport error: %v", err)
	}
	if !oldStatus.IsError {
		t.Fatalf("old Run identity unexpectedly survived restart: %s", toolText(t, oldStatus))
	}
	read := callGatewayTool(t, ctx, restartedClient, "read", map[string]any{"environment_id": environment.ID, "path": "marker.txt"})
	if read.IsError || !strings.Contains(toolText(t, read), "phase8-file-still-works") {
		t.Fatalf("Run lifecycle/restart broke ordinary file capability: %s", toolText(t, read))
	}
	_ = restartedClient.Close()
	stopGateway(secondGateway)
}
