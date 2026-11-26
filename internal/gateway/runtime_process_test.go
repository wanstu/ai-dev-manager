package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	goruntime "runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"ai-dev-manager-v2/internal/app"
	runtimepkg "ai-dev-manager-v2/internal/runtime"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const devProcessTestWriter = "phase6-process-writer"

func TestRuntimeOwnerDevProcessSurvivesAgentSessionWithoutHoldingWriter(t *testing.T) {
	service, environmentID, root := devProcessTestService(t)
	portFile := filepath.Join(root, "server.port")
	t.Setenv("ADM_TEST_DEV_PROCESS_HELPER", "1")
	t.Setenv("ADM_TEST_DEV_PROCESS_PORT_FILE", portFile)

	owner := newRuntimeOwner(service)
	defer owner.Close()
	server := newServerForSurface(service, owner, serverSurfaceAdmin)
	ctx := context.Background()
	first := connectInMemory(t, ctx, server)

	started := callGatewayTool(t, ctx, first, "process_start", map[string]any{
		"environment_id": environmentID,
		"writer_owner":   devProcessTestWriter,
		"executable":     os.Args[0],
		"args":           []string{"-test.run=^TestDevProcessHelper$"},
		"max_log_bytes":  4096,
	})
	if started.IsError {
		t.Fatalf("process_start failed: %s", toolText(t, started))
	}
	processID := processIDFromToolText(t, started)
	if !strings.HasPrefix(processID, "proc_") {
		t.Fatalf("process id=%q", processID)
	}
	_ = first.Close()

	port := waitDevProcessPortFile(t, portFile)
	if _, err := service.Environments.ReleaseWriter(environmentID, devProcessTestWriter, false); err != nil {
		t.Fatalf("release launch writer: %v", err)
	}
	if environment, err := service.Environments.Get(environmentID); err != nil {
		t.Fatal(err)
	} else if environment.Writer != nil {
		t.Fatalf("running process retained writer lease: %+v", environment.Writer)
	}

	const nextWriter = "phase6-process-writer-next"
	if _, err := service.Environments.AcquireWriter(environmentID, nextWriter); err != nil {
		t.Fatalf("acquire replacement writer: %v", err)
	}
	defer service.Environments.ReleaseWriter(environmentID, nextWriter, false)

	second := connectInMemory(t, ctx, server)
	defer second.Close()
	waitGatewayProcessPort(t, ctx, second, environmentID, processID, port)

	listed := callGatewayTool(t, ctx, second, "process_list", map[string]any{"environment_id": environmentID})
	if listed.IsError || !strings.Contains(toolText(t, listed), processID) {
		t.Fatalf("process_list missing %q: %s", processID, toolText(t, listed))
	}
	logs := callGatewayTool(t, ctx, second, "process_logs", map[string]any{"environment_id": environmentID, "process_id": processID})
	logsText := toolText(t, logs)
	if logs.IsError || !strings.Contains(logsText, "phase6-stdout-start") || !strings.Contains(logsText, "phase6-stderr-start") {
		t.Fatalf("process_logs missing output: %s", logsText)
	}
	if owner.Info().OwnedDevProcesses != 1 {
		t.Fatalf("owner info=%+v", owner.Info())
	}

	wrongWriter := callGatewayTool(t, ctx, second, "process_stop", map[string]any{
		"environment_id": environmentID,
		"writer_owner":   "wrong-writer",
		"process_id":     processID,
	})
	if !wrongWriter.IsError {
		t.Fatalf("non-current writer stopped process: %s", toolText(t, wrongWriter))
	}

	stopped := callGatewayTool(t, ctx, second, "process_stop", map[string]any{
		"environment_id": environmentID,
		"writer_owner":   nextWriter,
		"process_id":     processID,
	})
	if stopped.IsError || !strings.Contains(toolText(t, stopped), `"state":"exited"`) {
		t.Fatalf("process_stop failed: %s", toolText(t, stopped))
	}
	waitPortReleased(t, port)
	if owner.Info().OwnedDevProcesses != 0 {
		t.Fatalf("stopped process remained active: %+v", owner.Info())
	}
	logs = callGatewayTool(t, ctx, second, "process_logs", map[string]any{"environment_id": environmentID, "process_id": processID})
	if logs.IsError || !strings.Contains(toolText(t, logs), "phase6-stdout-start") {
		t.Fatalf("exited process logs were not retained: %s", toolText(t, logs))
	}
}

func TestRuntimeOwnerDropEnvironmentStopsAndForgetsDevProcess(t *testing.T) {
	service, environmentID, root := devProcessTestService(t)
	portFile := filepath.Join(root, "drop.port")
	t.Setenv("ADM_TEST_DEV_PROCESS_HELPER", "1")
	t.Setenv("ADM_TEST_DEV_PROCESS_PORT_FILE", portFile)
	owner := newRuntimeOwner(service)
	defer owner.Close()
	status, err := owner.StartDevProcess(environmentID, devProcessTestWriter, os.Args[0], []string{"-test.run=^TestDevProcessHelper$"}, "", 4096)
	if err != nil {
		t.Fatal(err)
	}
	port := waitDevProcessPortFile(t, portFile)
	owner.DropEnvironment(environmentID)
	waitPortReleased(t, port)
	if _, err := owner.DevProcessStatus(environmentID, status.ID); err == nil {
		t.Fatalf("dropped Environment retained process %q", status.ID)
	}
}

func TestProcessStartDoesNotRenewWriterLease(t *testing.T) {
	service, environmentID, root := devProcessTestService(t)
	portFile := filepath.Join(root, "lease.port")
	t.Setenv("ADM_TEST_DEV_PROCESS_HELPER", "1")
	t.Setenv("ADM_TEST_DEV_PROCESS_PORT_FILE", portFile)
	before, err := service.Environments.Get(environmentID)
	if err != nil {
		t.Fatal(err)
	}
	if before.Writer == nil {
		t.Fatal("writer lease missing before process start")
	}
	time.Sleep(20 * time.Millisecond)

	owner := newRuntimeOwner(service)
	defer owner.Close()
	status, err := owner.StartDevProcess(environmentID, devProcessTestWriter, os.Args[0], []string{"-test.run=^TestDevProcessHelper$"}, "", 4096)
	if err != nil {
		t.Fatal(err)
	}
	if status.ID == "" {
		t.Fatal("missing process identity")
	}
	after, err := service.Environments.Get(environmentID)
	if err != nil {
		t.Fatal(err)
	}
	if after.Writer == nil {
		t.Fatal("writer lease missing after process start")
	}
	if !after.Writer.LastSeenAt.Equal(before.Writer.LastSeenAt) {
		t.Fatalf("process_start changed writer lease: before=%s after=%s", before.Writer.LastSeenAt, after.Writer.LastSeenAt)
	}
	if !after.Writer.ExpiresAt.Equal(before.Writer.ExpiresAt) {
		t.Fatalf("process_start changed writer expiry: before=%s after=%s", before.Writer.ExpiresAt, after.Writer.ExpiresAt)
	}
	_ = waitDevProcessPortFile(t, portFile)
}

func TestRuntimeOwnerCloseStopsDevProcess(t *testing.T) {
	service, environmentID, root := devProcessTestService(t)
	portFile := filepath.Join(root, "close.port")
	t.Setenv("ADM_TEST_DEV_PROCESS_HELPER", "1")
	t.Setenv("ADM_TEST_DEV_PROCESS_PORT_FILE", portFile)
	owner := newRuntimeOwner(service)
	status, err := owner.StartDevProcess(environmentID, devProcessTestWriter, os.Args[0], []string{"-test.run=^TestDevProcessHelper$"}, "", 4096)
	if err != nil {
		t.Fatal(err)
	}
	port := waitDevProcessPortFile(t, portFile)
	if status.ID == "" {
		t.Fatal("missing process identity")
	}
	if err := owner.Close(); err != nil {
		t.Fatal(err)
	}
	waitPortReleased(t, port)
	if err := owner.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestProcessStartRejectsForbiddenExecutableAndEscapedCwd(t *testing.T) {
	service, environmentID, _ := devProcessTestService(t)
	owner := newRuntimeOwner(service)
	defer owner.Close()
	session := connectInMemory(t, context.Background(), newServerForSurface(service, owner, serverSurfaceAdmin))
	defer session.Close()

	for name, args := range map[string]map[string]any{
		"forbidden": {
			"environment_id": environmentID, "writer_owner": devProcessTestWriter,
			"executable": "definitely-not-allowed",
		},
		"escaped cwd": {
			"environment_id": environmentID, "writer_owner": devProcessTestWriter,
			"executable": os.Args[0], "args": []string{"-test.run=^$"}, "cwd": "../escape",
		},
	} {
		t.Run(name, func(t *testing.T) {
			result := callGatewayTool(t, context.Background(), session, "process_start", args)
			if !result.IsError {
				t.Fatalf("unsafe start unexpectedly passed: %s", toolText(t, result))
			}
		})
	}
}

func TestTailLogBufferKeepsBoundedTail(t *testing.T) {
	for _, test := range []struct {
		name      string
		writes    []string
		want      string
		truncated bool
	}{
		{name: "below limit", writes: []string{"12345"}, want: "12345"},
		{name: "exact limit", writes: []string{"12345678"}, want: "12345678"},
		{name: "exact limit split", writes: []string{"12345", "678"}, want: "12345678"},
		{name: "exact limit then empty", writes: []string{"12345678", ""}, want: "12345678"},
		{name: "overflow split", writes: []string{"12345", "67890"}, want: "34567890", truncated: true},
		{name: "replace existing tail", writes: []string{"1", "abcdefgh"}, want: "abcdefgh", truncated: true},
		{name: "oversized write", writes: []string{"abcdefghijk"}, want: "defghijk", truncated: true},
		{name: "truncation remains visible", writes: []string{"123456789", "0"}, want: "34567890", truncated: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			buffer := newTailLogBuffer(8)
			for _, value := range test.writes {
				if n, err := buffer.Write([]byte(value)); err != nil || n != len(value) {
					t.Fatalf("write consumed %d bytes, error %v", n, err)
				}
			}
			got, truncated := buffer.Snapshot()
			if got != test.want || truncated != test.truncated {
				t.Fatalf("tail=%q truncated=%v; want tail=%q truncated=%v", got, truncated, test.want, test.truncated)
			}
		})
	}
}

func devProcessTestService(t *testing.T) (*app.Service, string, string) {
	t.Helper()
	root := t.TempDir()
	service := app.New(filepath.Join(t.TempDir(), "state.json"))
	workspace, err := service.Workspaces.Add(root, "phase6-process")
	if err != nil {
		t.Fatal(err)
	}
	environment, err := service.Environments.Create(workspace.ID, "phase6-process", "")
	if err != nil {
		t.Fatal(err)
	}
	if err := service.AllowExecutable(os.Args[0]); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Environments.AcquireWriter(environment.ID, devProcessTestWriter); err != nil {
		t.Fatal(err)
	}
	return service, environment.ID, root
}

func waitDevProcessPortFile(t *testing.T, path string) int {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		data, err := os.ReadFile(path)
		if err == nil {
			port, _ := strconv.Atoi(strings.TrimSpace(string(data)))
			if port > 0 {
				return port
			}
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatalf("dev process did not report port in %s", path)
	return 0
}

func waitGatewayProcessPort(t *testing.T, ctx context.Context, session *mcp.ClientSession, environmentID, processID string, port int) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		status := callGatewayTool(t, ctx, session, "process_status", map[string]any{"environment_id": environmentID, "process_id": processID})
		if !status.IsError {
			var output struct {
				Result devProcessStatus `json:"result"`
			}
			if err := json.Unmarshal([]byte(toolText(t, status)), &output); err != nil {
				t.Fatalf("decode process status: %v", err)
			}
			if output.Result.ID == processID && output.Result.State == devProcessRunning {
				if goruntime.GOOS != "windows" && goruntime.GOOS != "linux" {
					// Keep lifecycle acceptance active on platforms without port observation.
					if len(output.Result.ListeningPorts) != 0 {
						t.Fatalf("unsupported platform unexpectedly reported ports: %+v", output.Result)
					}
					return
				}
				for _, observedPort := range output.Result.ListeningPorts {
					if observedPort == port {
						return
					}
				}
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("process %s did not report expected running state/listening port %d", processID, port)
}

func waitPortReleased(t *testing.T, port int) {
	t.Helper()
	address := fmt.Sprintf("127.0.0.1:%d", port)
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		connection, err := net.DialTimeout("tcp", address, 100*time.Millisecond)
		if err != nil {
			return
		}
		_ = connection.Close()
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("port %d remained reachable", port)
}

func processIDFromToolText(t *testing.T, result *mcp.CallToolResult) string {
	t.Helper()
	text := toolText(t, result)
	marker := `"id":"`
	start := strings.Index(text, marker)
	if start < 0 {
		t.Fatalf("process result missing id: %s", text)
	}
	start += len(marker)
	end := strings.Index(text[start:], `"`)
	if end < 0 {
		t.Fatalf("process result has malformed id: %s", text)
	}
	return text[start : start+end]
}

func TestDevProcessHelper(t *testing.T) {
	if os.Getenv("ADM_TEST_DEV_PROCESS_HELPER") != "1" {
		return
	}
	fmt.Fprintln(os.Stdout, "phase6-stdout-start")
	fmt.Fprintln(os.Stderr, "phase6-stderr-start")
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		panic(err)
	}
	defer listener.Close()
	port := listener.Addr().(*net.TCPAddr).Port
	if err := os.WriteFile(os.Getenv("ADM_TEST_DEV_PROCESS_PORT_FILE"), []byte(strconv.Itoa(port)), 0o644); err != nil {
		panic(err)
	}
	for {
		connection, err := listener.Accept()
		if err != nil {
			return
		}
		_, _ = connection.Write([]byte("phase6\n"))
		_ = connection.Close()
	}
}

func TestAdminManagesLongProcessWithoutWriterLease(t *testing.T) {
	service, environmentID, root := devProcessTestService(t)
	if _, err := service.Environments.ReleaseWriter(environmentID, devProcessTestWriter, false); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ADM_TEST_DEV_PROCESS_HELPER", "1")
	portFile := filepath.Join(root, "admin-process.port")
	t.Setenv("ADM_TEST_DEV_PROCESS_PORT_FILE", portFile)
	owner := newRuntimeOwner(service)
	defer owner.Close()
	ctx := context.Background()
	admin := connectInMemory(t, ctx, newServerForSurface(service, owner, serverSurfaceAdmin))
	defer admin.Close()
	started := callGatewayTool(t, ctx, admin, "process_start", map[string]any{
		"environment_id": environmentID, "executable": os.Args[0],
		"args": []string{"-test.run=^TestDevProcessHelper$"},
	})
	if started.IsError {
		t.Fatalf("admin start without Writer: %s", toolText(t, started))
	}
	id := processIDFromToolText(t, started)
	port := waitDevProcessPortFile(t, portFile)
	listed := callGatewayTool(t, ctx, admin, "process_list", map[string]any{"environment_id": ""})
	if listed.IsError || !strings.Contains(toolText(t, listed), id) || !strings.Contains(toolText(t, listed), environmentID) {
		t.Fatalf("all processes: %s", toolText(t, listed))
	}
	agent := connectInMemory(t, ctx, newServerForSurface(service, owner, serverSurfaceAgent))
	defer agent.Close()
	denied := callGatewayTool(t, ctx, agent, "process_stop", map[string]any{"environment_id": environmentID, "process_id": id})
	if !denied.IsError {
		t.Fatal("Agent bypassed stop authority")
	}
	stopped := callGatewayTool(t, ctx, admin, "process_stop", map[string]any{"environment_id": environmentID, "process_id": id})
	if stopped.IsError || !strings.Contains(toolText(t, stopped), "exited") {
		t.Fatalf("admin stop: %s", toolText(t, stopped))
	}
	waitPortReleased(t, port)
	env, err := service.Environments.Get(environmentID)
	if err != nil || env.Writer != nil {
		t.Fatalf("process operation changed Writer: %+v %v", env.Writer, err)
	}
}

func TestManagedProcessResourceSample(t *testing.T) {
	if goruntime.GOOS != "windows" && goruntime.GOOS != "linux" {
		t.Skip("resource sampling platform")
	}
	service, id, root := devProcessTestService(t)
	t.Setenv("ADM_TEST_DEV_PROCESS_HELPER", "1")
	file := filepath.Join(root, "metrics.port")
	t.Setenv("ADM_TEST_DEV_PROCESS_PORT_FILE", file)
	owner := newRuntimeOwner(service)
	defer owner.Close()
	started, err := owner.StartDevProcess(id, devProcessTestWriter, os.Args[0], []string{"-test.run=^TestDevProcessHelper$"}, "", 4096)
	if err != nil {
		t.Fatal(err)
	}
	port := waitDevProcessPortFile(t, file)
	time.Sleep(1100 * time.Millisecond)
	status, err := owner.DevProcessStatus(id, started.ID)
	if err != nil || status.Resources == nil || status.Resources.ProcessCount < 1 || status.Resources.MemoryBytes == 0 || status.Resources.CPUPercent == nil || status.UptimeSeconds < 1 {
		t.Fatalf("resource sample %+v / %+v / %v", status, status.Resources, err)
	}
	if _, err := owner.StopDevProcessAdmin(id, started.ID); err != nil {
		t.Fatal(err)
	}
	waitPortReleased(t, port)
	status, err = owner.DevProcessStatus(id, started.ID)
	if err != nil || status.Resources != nil || status.ExitReason != "stopped" {
		t.Fatalf("stopped %+v %v", status, err)
	}
}

func TestManagedProcessJobReclaimsChildOnParentExit(t *testing.T) {
	if goruntime.GOOS != "windows" {
		t.Skip("Windows job containment")
	}
	service, id, root := devProcessTestService(t)
	portFile := filepath.Join(root, "child.port")
	exitFile := filepath.Join(root, "parent-exit")
	t.Setenv("ADM_TEST_DEV_PROCESS_HELPER", "1")
	t.Setenv("ADM_TEST_DEV_PROCESS_PORT_FILE", portFile)
	t.Setenv("ADM_TEST_MANAGED_PARENT", "1")
	t.Setenv("ADM_TEST_MANAGED_PARENT_EXIT", exitFile)
	owner := newRuntimeOwner(service)
	defer owner.Close()
	started, err := owner.StartDevProcess(id, devProcessTestWriter, os.Args[0], []string{"-test.run=^TestManagedProcessParentHelper$"}, "", 4096)
	if err != nil {
		t.Fatal(err)
	}
	port := waitDevProcessPortFile(t, portFile)
	time.Sleep(1100 * time.Millisecond)
	status, err := owner.DevProcessStatus(id, started.ID)
	if err != nil || status.Resources == nil || status.Resources.ProcessCount < 2 {
		t.Fatalf("missing job child %+v %v", status.Resources, err)
	}
	if err := os.WriteFile(exitFile, []byte("exit"), 0600); err != nil {
		t.Fatal(err)
	}
	waitPortReleased(t, port)
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		status, err = owner.DevProcessStatus(id, started.ID)
		if err != nil {
			t.Fatal(err)
		}
		if status.State == devProcessExited {
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatal("parent exit did not complete Process")
}

func TestManagedProcessParentHelper(t *testing.T) {
	if os.Getenv("ADM_TEST_MANAGED_PARENT") != "1" {
		return
	}
	child := exec.Command(os.Args[0], "-test.run=^TestDevProcessHelper$")
	child.Stdout = os.Stdout
	child.Stderr = os.Stderr
	if err := child.Start(); err != nil {
		t.Fatal(err)
	}
	// The child belongs to the same Windows Job. Exit without waiting only in
	// this controlled test, to verify job close handles inherited pipes.
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(os.Getenv("ADM_TEST_MANAGED_PARENT_EXIT")); err == nil {
			os.Exit(0)
		}
		time.Sleep(25 * time.Millisecond)
	}
	_ = child.Process.Kill()
	_ = child.Wait()
	t.Fatal("parent exit not requested")
}

func TestManagedProcessJobReclaimsAfterOwnerCrash(t *testing.T) {
	if goruntime.GOOS != "windows" {
		t.Skip("Windows job containment")
	}
	portFile := filepath.Join(t.TempDir(), "crash.port")
	cmd := exec.Command(os.Args[0], "-test.run=^TestManagedProcessCrashOwnerHelper$")
	cmd.Env = append(os.Environ(), "ADM_TEST_CRASH_OWNER=1", "ADM_TEST_DEV_PROCESS_HELPER=1", "ADM_TEST_DEV_PROCESS_PORT_FILE="+portFile)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if cmd.ProcessState == nil {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
		}
	}()
	port := waitDevProcessPortFile(t, portFile)
	if err := cmd.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	_ = cmd.Wait()
	waitPortReleased(t, port)
}

func TestManagedProcessCrashOwnerHelper(t *testing.T) {
	if os.Getenv("ADM_TEST_CRASH_OWNER") != "1" {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	child := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestDevProcessHelper$")
	managed, err := runtimepkg.StartManagedProcess(child)
	if err != nil {
		t.Fatal(err)
	}
	defer managed.Close()
	// The outer test terminates this disposable process without normal cleanup.
	// A bounded timeout ensures even a failed test does not leave it running.
	time.Sleep(12 * time.Second)
	cancel()
	_ = child.Wait()
}
