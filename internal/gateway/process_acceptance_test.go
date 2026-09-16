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

func TestDevProcessLifecycleAcrossRealGatewayRestart(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "state.json")
	projectRoot := t.TempDir()
	service := app.New(statePath)
	workspace, err := service.Workspaces.Add(projectRoot, "phase6-real")
	if err != nil {
		t.Fatal(err)
	}
	environment, err := service.Environments.Create(workspace.ID, "phase6-real", "")
	if err != nil {
		t.Fatal(err)
	}
	if err := service.AllowExecutable(os.Args[0]); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Environments.AcquireWriter(environment.ID, devProcessTestWriter); err != nil {
		t.Fatal(err)
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	listen := listener.Addr().String()
	_ = listener.Close()
	endpoint := "http://" + listen + "/mcp"

	portFile := filepath.Join(projectRoot, "phase6-real.port")
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

	firstGateway, firstGatewayStatus := startGateway()
	if firstGatewayStatus.OwnerID == "" {
		t.Fatal("first Gateway missing owner identity")
	}
	ctx := context.Background()
	firstClient := connectHTTPWithRetry(t, ctx, endpoint)
	started := callGatewayTool(t, ctx, firstClient, "process_start", map[string]any{
		"environment_id": environment.ID,
		"writer_owner":   devProcessTestWriter,
		"executable":     os.Args[0],
		"args":           []string{"-test.run=^TestDevProcessHelper$"},
		"max_log_bytes":  4096,
	})
	if started.IsError {
		t.Fatalf("real process_start failed: %s", toolText(t, started))
	}
	firstProcessID := processIDFromToolText(t, started)
	_ = firstClient.Close()

	firstPort := waitDevProcessPortFile(t, portFile)
	secondClient := connectHTTPWithRetry(t, ctx, endpoint)
	waitGatewayProcessPort(t, ctx, secondClient, environment.ID, firstProcessID, firstPort)
	logs := callGatewayTool(t, ctx, secondClient, "process_logs", map[string]any{"environment_id": environment.ID, "process_id": firstProcessID})
	if logs.IsError || !strings.Contains(toolText(t, logs), "phase6-stdout-start") {
		t.Fatalf("real process logs missing: %s", toolText(t, logs))
	}

	stateBytes, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatal(err)
	}
	stateText := string(stateBytes)
	for _, forbidden := range []string{firstProcessID, "owned_dev_processes", "listening_ports"} {
		if strings.Contains(stateText, forbidden) {
			t.Fatalf("observed process state leaked into persisted state via %q", forbidden)
		}
	}

	stopped := callGatewayTool(t, ctx, secondClient, "process_stop", map[string]any{
		"environment_id": environment.ID,
		"writer_owner":   devProcessTestWriter,
		"process_id":     firstProcessID,
	})
	if stopped.IsError || !strings.Contains(toolText(t, stopped), `"state":"exited"`) {
		t.Fatalf("real process_stop failed: %s", toolText(t, stopped))
	}
	waitPortReleased(t, firstPort)

	if err := os.Remove(portFile); err != nil && !errors.Is(err, os.ErrNotExist) {
		t.Fatal(err)
	}
	secondStarted := callGatewayTool(t, ctx, secondClient, "process_start", map[string]any{
		"environment_id": environment.ID,
		"writer_owner":   devProcessTestWriter,
		"executable":     os.Args[0],
		"args":           []string{"-test.run=^TestDevProcessHelper$"},
	})
	if secondStarted.IsError {
		t.Fatalf("second process_start failed: %s", toolText(t, secondStarted))
	}
	secondProcessID := processIDFromToolText(t, secondStarted)
	secondPort := waitDevProcessPortFile(t, portFile)
	waitGatewayProcessPort(t, ctx, secondClient, environment.ID, secondProcessID, secondPort)
	_ = secondClient.Close()

	stopGateway(firstGateway)
	waitPortReleased(t, secondPort)

	secondGateway, secondGatewayStatus := startGateway()
	if secondGatewayStatus.OwnerID == firstGatewayStatus.OwnerID {
		t.Fatalf("Gateway restart reused owner %q", firstGatewayStatus.OwnerID)
	}
	restartedClient := connectHTTPWithRetry(t, ctx, endpoint)
	defer restartedClient.Close()
	listed := callGatewayTool(t, ctx, restartedClient, "process_list", map[string]any{"environment_id": environment.ID})
	listedText := toolText(t, listed)
	if listed.IsError || strings.Contains(listedText, firstProcessID) || strings.Contains(listedText, secondProcessID) || strings.Contains(listedText, "proc_") {
		t.Fatalf("restart resurrected observed process state: %s", listedText)
	}
	oldStatus, err := restartedClient.CallTool(ctx, &mcp.CallToolParams{Name: "process_status", Arguments: map[string]any{
		"environment_id": environment.ID,
		"process_id":     secondProcessID,
	}})
	if err != nil {
		t.Fatalf("old process status transport error: %v", err)
	}
	if !oldStatus.IsError {
		t.Fatalf("old process identity unexpectedly survived restart: %s", toolText(t, oldStatus))
	}
	_ = restartedClient.Close()
	stopGateway(secondGateway)
}
