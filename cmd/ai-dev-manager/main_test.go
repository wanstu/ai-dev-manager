package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"ai-dev-manager/internal/daemon"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestMain(m *testing.M) {
	if os.Getenv(daemon.ChildEnvironmentKey) == "1" {
		if err := run(context.Background(), os.Args[1:], os.Stdout, os.Stderr); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func TestCLIWorkspaceAddListShowAndInspect(t *testing.T) {
	configRoot := t.TempDir()
	workspaceRoot := t.TempDir()

	var addOut bytes.Buffer
	if err := run(context.Background(), []string{"--config-root", configRoot, "--json", "workspace", "add", "--path", workspaceRoot}, &addOut, io.Discard); err != nil {
		t.Fatalf("workspace add error = %v", err)
	}
	var added workspaceOutput
	if err := json.Unmarshal(addOut.Bytes(), &added); err != nil {
		t.Fatalf("decode add output: %v; output=%s", err, addOut.String())
	}
	if added.ID == "" || added.Path == "" {
		t.Fatalf("add output = %+v", added)
	}

	var listOut bytes.Buffer
	if err := run(context.Background(), []string{"--config-root", configRoot, "--json", "workspace", "list"}, &listOut, io.Discard); err != nil {
		t.Fatalf("workspace list error = %v", err)
	}
	var listed []workspaceOutput
	if err := json.Unmarshal(listOut.Bytes(), &listed); err != nil {
		t.Fatalf("decode list output: %v; output=%s", err, listOut.String())
	}
	if len(listed) != 1 || listed[0].ID != added.ID {
		t.Fatalf("list output = %+v, want %s", listed, added.ID)
	}

	var showOut bytes.Buffer
	if err := run(context.Background(), []string{"--config-root", configRoot, "--json", "workspace", "show", added.ID}, &showOut, io.Discard); err != nil {
		t.Fatalf("workspace show error = %v", err)
	}
	var shown workspaceOutput
	if err := json.Unmarshal(showOut.Bytes(), &shown); err != nil {
		t.Fatalf("decode show output: %v; output=%s", err, showOut.String())
	}
	if shown.ID != added.ID || shown.Path != added.Path {
		t.Fatalf("show output = %+v, want %+v", shown, added)
	}

	var inspectOut bytes.Buffer
	if err := run(context.Background(), []string{"--config-root", configRoot, "--json", "inspect", "--workspace", added.ID}, &inspectOut, io.Discard); err != nil {
		t.Fatalf("inspect error = %v", err)
	}
	var snapshot struct {
		Workspace struct {
			ID        string `json:"id"`
			RuntimeID string `json:"runtime_id"`
		} `json:"workspace"`
		Runtime struct {
			RuntimeID    string   `json:"runtime_id"`
			Capabilities []string `json:"capabilities"`
		} `json:"runtime"`
	}
	if err := json.Unmarshal(inspectOut.Bytes(), &snapshot); err != nil {
		t.Fatalf("decode inspect output: %v; output=%s", err, inspectOut.String())
	}
	if snapshot.Workspace.ID != added.ID || snapshot.Runtime.RuntimeID != "native" || !containsString(snapshot.Runtime.Capabilities, "files.read") {
		t.Fatalf("inspect output = %+v", snapshot)
	}
}

func TestCLIForegroundServeExposesWorkspaceAndStopsOnCancel(t *testing.T) {
	configRoot := t.TempDir()
	workspaceRoot := t.TempDir()
	if err := os.WriteFile(filepath.Join(workspaceRoot, "value.txt"), []byte("cli-workspace"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	var addOut bytes.Buffer
	if err := run(context.Background(), []string{"--config-root", configRoot, "--json", "workspace", "add", "--path", workspaceRoot}, &addOut, io.Discard); err != nil {
		t.Fatalf("workspace add error = %v", err)
	}
	var added workspaceOutput
	if err := json.Unmarshal(addOut.Bytes(), &added); err != nil {
		t.Fatalf("decode add output: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	reader, writer := io.Pipe()
	errCh := make(chan error, 1)
	go func() {
		err := run(ctx, []string{"--config-root", configRoot, "--json", "serve", "--workspace", added.ID}, writer, io.Discard)
		_ = writer.Close()
		errCh <- err
	}()

	lineCh := make(chan string, 1)
	readErrCh := make(chan error, 1)
	go func() {
		scanner := bufio.NewScanner(reader)
		if scanner.Scan() {
			lineCh <- scanner.Text()
			return
		}
		readErrCh <- scanner.Err()
	}()

	var line string
	select {
	case line = <-lineCh:
	case err := <-readErrCh:
		t.Fatalf("read serve output error = %v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for serve endpoint")
	}
	var instance struct {
		Endpoint string `json:"endpoint"`
	}
	if err := json.Unmarshal([]byte(line), &instance); err != nil {
		t.Fatalf("decode serve output: %v; line=%s", err, line)
	}
	if instance.Endpoint == "" {
		t.Fatalf("serve output missing endpoint: %s", line)
	}

	clientCtx, clientCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer clientCancel()
	client := mcp.NewClient(&mcp.Implementation{Name: "adm-cli-test", Version: "v0.2.0"}, nil)
	session, err := client.Connect(clientCtx, &mcp.StreamableClientTransport{Endpoint: instance.Endpoint}, nil)
	if err != nil {
		t.Fatalf("Connect(%s) error = %v", instance.Endpoint, err)
	}
	result, err := session.CallTool(clientCtx, &mcp.CallToolParams{Name: "read", Arguments: map[string]any{"path": "value.txt"}})
	if err != nil {
		t.Fatalf("CallTool(read) error = %v", err)
	}
	if result.IsError || len(result.Content) == 0 {
		t.Fatalf("CallTool(read) result = %+v", result)
	}
	text, ok := result.Content[0].(*mcp.TextContent)
	if !ok || !strings.Contains(text.Text, "cli-workspace") {
		t.Fatalf("read content = %+v", result.Content)
	}
	_ = session.Close()

	cancel()
	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("serve returned error after cancel = %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("serve did not stop after cancellation")
	}
}

func TestCLIDaemonLifecycleAcrossProcesses(t *testing.T) {
	configRoot := t.TempDir()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_, _ = daemon.Stop(ctx, configRoot)
	})

	var startOut bytes.Buffer
	if err := run(context.Background(), []string{"--config-root", configRoot, "--json", "start"}, &startOut, io.Discard); err != nil {
		t.Fatalf("start error = %v", err)
	}
	var started daemon.Metadata
	if err := json.Unmarshal(startOut.Bytes(), &started); err != nil {
		t.Fatalf("decode start output: %v; output=%s", err, startOut.String())
	}
	if started.InstanceID == "" || started.PID <= 0 || started.PID == os.Getpid() || started.State != daemon.StateRunning {
		t.Fatalf("start output = %+v", started)
	}

	var repeatOut bytes.Buffer
	if err := run(context.Background(), []string{"--config-root", configRoot, "--json", "start"}, &repeatOut, io.Discard); err != nil {
		t.Fatalf("repeat start error = %v", err)
	}
	var repeated daemon.Metadata
	if err := json.Unmarshal(repeatOut.Bytes(), &repeated); err != nil {
		t.Fatalf("decode repeat start output: %v", err)
	}
	if repeated.InstanceID != started.InstanceID || repeated.PID != started.PID {
		t.Fatalf("repeat start created another owner: first=%+v second=%+v", started, repeated)
	}

	var statusOut bytes.Buffer
	if err := run(context.Background(), []string{"--config-root", configRoot, "--json", "status"}, &statusOut, io.Discard); err != nil {
		t.Fatalf("status error = %v", err)
	}
	var status daemon.Metadata
	if err := json.Unmarshal(statusOut.Bytes(), &status); err != nil {
		t.Fatalf("decode status output: %v", err)
	}
	if status.InstanceID != started.InstanceID || status.PID != started.PID || status.State != daemon.StateRunning {
		t.Fatalf("status = %+v, want same running daemon %+v", status, started)
	}

	var stopOut bytes.Buffer
	if err := run(context.Background(), []string{"--config-root", configRoot, "--json", "stop"}, &stopOut, io.Discard); err != nil {
		t.Fatalf("stop error = %v", err)
	}
	var stopped daemon.Metadata
	if err := json.Unmarshal(stopOut.Bytes(), &stopped); err != nil {
		t.Fatalf("decode stop output: %v", err)
	}
	if stopped.InstanceID != started.InstanceID || stopped.State != daemon.StateStopped {
		t.Fatalf("stop output = %+v", stopped)
	}

	var afterOut bytes.Buffer
	if err := run(context.Background(), []string{"--config-root", configRoot, "--json", "status"}, &afterOut, io.Discard); err != nil {
		t.Fatalf("status after stop error = %v", err)
	}
	var after daemon.Metadata
	if err := json.Unmarshal(afterOut.Bytes(), &after); err != nil {
		t.Fatalf("decode status after stop output: %v", err)
	}
	if after.State != daemon.StateStopped || after.InstanceID != "" || after.PID != 0 {
		t.Fatalf("status after stop = %+v", after)
	}
}

func TestCLIPersistentRuntimeLifecycleAcrossDaemon(t *testing.T) {
	configRoot := t.TempDir()
	rootA := t.TempDir()
	rootB := t.TempDir()
	if err := os.WriteFile(filepath.Join(rootA, "value.txt"), []byte("daemon-workspace-a"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(rootB, "value.txt"), []byte("daemon-workspace-b"), 0o644); err != nil {
		t.Fatal(err)
	}

	addWorkspace := func(root string) workspaceOutput {
		t.Helper()
		var out bytes.Buffer
		if err := run(context.Background(), []string{"--config-root", configRoot, "--json", "workspace", "add", "--path", root}, &out, io.Discard); err != nil {
			t.Fatalf("workspace add %s error = %v", root, err)
		}
		var ws workspaceOutput
		if err := json.Unmarshal(out.Bytes(), &ws); err != nil {
			t.Fatalf("decode workspace add: %v; output=%s", err, out.String())
		}
		return ws
	}
	wsA := addWorkspace(rootA)
	wsB := addWorkspace(rootB)

	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_, _ = daemon.Stop(ctx, configRoot)
	})
	if err := run(context.Background(), []string{"--config-root", configRoot, "start"}, io.Discard, io.Discard); err != nil {
		t.Fatalf("daemon start error = %v", err)
	}

	startRuntime := func(workspaceID string) daemon.RuntimeStatus {
		t.Helper()
		var out bytes.Buffer
		if err := run(context.Background(), []string{"--config-root", configRoot, "--json", "runtime", "start", "--workspace", workspaceID}, &out, io.Discard); err != nil {
			t.Fatalf("runtime start %s error = %v", workspaceID, err)
		}
		var status daemon.RuntimeStatus
		if err := json.Unmarshal(out.Bytes(), &status); err != nil {
			t.Fatalf("decode runtime start: %v; output=%s", err, out.String())
		}
		return status
	}
	startedA := startRuntime(wsA.ID)
	startedB := startRuntime(wsB.ID)
	if startedA.State != daemon.RuntimeRunning || startedB.State != daemon.RuntimeRunning || startedA.MCPHost == nil || startedB.MCPHost == nil {
		t.Fatalf("runtime starts A=%+v B=%+v", startedA, startedB)
	}
	if startedA.MCPHost.Endpoint == startedB.MCPHost.Endpoint {
		t.Fatalf("A/B share endpoint %q", startedA.MCPHost.Endpoint)
	}
	repeatedA := startRuntime(wsA.ID)
	if repeatedA.MCPHost == nil || repeatedA.MCPHost.ID != startedA.MCPHost.ID || repeatedA.MCPHost.Endpoint != startedA.MCPHost.Endpoint {
		t.Fatalf("repeat A changed host: first=%+v repeat=%+v", startedA.MCPHost, repeatedA.MCPHost)
	}

	statusRuntime := func(workspaceID string) daemon.RuntimeStatus {
		t.Helper()
		var out bytes.Buffer
		if err := run(context.Background(), []string{"--config-root", configRoot, "--json", "runtime", "status", "--workspace", workspaceID}, &out, io.Discard); err != nil {
			t.Fatalf("runtime status %s error = %v", workspaceID, err)
		}
		var status daemon.RuntimeStatus
		if err := json.Unmarshal(out.Bytes(), &status); err != nil {
			t.Fatalf("decode runtime status: %v; output=%s", err, out.String())
		}
		return status
	}
	observedA := statusRuntime(wsA.ID)
	observedB := statusRuntime(wsB.ID)
	if observedA.MCPHost == nil || observedA.MCPHost.Endpoint != startedA.MCPHost.Endpoint || observedB.MCPHost == nil || observedB.MCPHost.Endpoint != startedB.MCPHost.Endpoint {
		t.Fatalf("observed runtime hosts changed: A=%+v B=%+v", observedA, observedB)
	}
	assertRuntimeEndpointRead(t, observedA.MCPHost.Endpoint, "daemon-workspace-a")
	assertRuntimeEndpointRead(t, observedB.MCPHost.Endpoint, "daemon-workspace-b")

	var listOut bytes.Buffer
	if err := run(context.Background(), []string{"--config-root", configRoot, "--json", "runtime", "list"}, &listOut, io.Discard); err != nil {
		t.Fatalf("runtime list error = %v", err)
	}
	var listed []daemon.RuntimeStatus
	if err := json.Unmarshal(listOut.Bytes(), &listed); err != nil {
		t.Fatalf("decode runtime list: %v; output=%s", err, listOut.String())
	}
	if len(listed) != 2 {
		t.Fatalf("runtime list = %+v, want two entries", listed)
	}

	var stopAOut bytes.Buffer
	if err := run(context.Background(), []string{"--config-root", configRoot, "--json", "runtime", "stop", "--workspace", wsA.ID}, &stopAOut, io.Discard); err != nil {
		t.Fatalf("runtime stop A error = %v", err)
	}
	var stoppedA daemon.RuntimeStatus
	if err := json.Unmarshal(stopAOut.Bytes(), &stoppedA); err != nil {
		t.Fatalf("decode runtime stop A: %v", err)
	}
	if stoppedA.State != daemon.RuntimeStopped || stoppedA.DesiredRunning {
		t.Fatalf("stopped A = %+v", stoppedA)
	}
	stillB := statusRuntime(wsB.ID)
	if stillB.State != daemon.RuntimeRunning || stillB.MCPHost == nil {
		t.Fatalf("B not running after A stop: %+v", stillB)
	}
	assertRuntimeEndpointRead(t, stillB.MCPHost.Endpoint, "daemon-workspace-b")

	if err := run(context.Background(), []string{"--config-root", configRoot, "stop"}, io.Discard, io.Discard); err != nil {
		t.Fatalf("daemon stop error = %v", err)
	}
	connectCtx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	client := mcp.NewClient(&mcp.Implementation{Name: "adm-runtime-cleanup", Version: "v0.3.0"}, nil)
	if session, err := client.Connect(connectCtx, &mcp.StreamableClientTransport{Endpoint: stillB.MCPHost.Endpoint}, nil); err == nil {
		_ = session.Close()
		t.Fatal("workspace B MCP endpoint still reachable after daemon stop")
	}
}

func assertRuntimeEndpointRead(t *testing.T, endpoint, want string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	client := mcp.NewClient(&mcp.Implementation{Name: "adm-runtime-cli-test", Version: "v0.3.0"}, nil)
	session, err := client.Connect(ctx, &mcp.StreamableClientTransport{Endpoint: endpoint}, nil)
	if err != nil {
		t.Fatalf("Connect(%s) error = %v", endpoint, err)
	}
	defer session.Close()
	result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "read", Arguments: map[string]any{"path": "value.txt"}})
	if err != nil {
		t.Fatalf("CallTool(read) error = %v", err)
	}
	if result.IsError || len(result.Content) == 0 {
		t.Fatalf("CallTool(read) result = %+v", result)
	}
	text, ok := result.Content[0].(*mcp.TextContent)
	if !ok || !strings.Contains(text.Text, want) {
		t.Fatalf("read content = %+v, want %q", result.Content, want)
	}
}

func TestCLIRuntimeDockerExposurePersistsAcrossDaemonRestart(t *testing.T) {
	configRoot := t.TempDir()
	workspaceRoot := t.TempDir()
	if err := os.WriteFile(filepath.Join(workspaceRoot, "value.txt"), []byte("docker-access"), 0o644); err != nil {
		t.Fatal(err)
	}
	var addOut bytes.Buffer
	if err := run(context.Background(), []string{"--config-root", configRoot, "--json", "workspace", "add", "--path", workspaceRoot}, &addOut, io.Discard); err != nil {
		t.Fatalf("workspace add error = %v", err)
	}
	var ws workspaceOutput
	if err := json.Unmarshal(addOut.Bytes(), &ws); err != nil {
		t.Fatalf("decode workspace add: %v", err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_, _ = daemon.Stop(ctx, configRoot)
	})
	if err := run(context.Background(), []string{"--config-root", configRoot, "start"}, io.Discard, io.Discard); err != nil {
		t.Fatalf("daemon start error = %v", err)
	}

	var runtimeOut bytes.Buffer
	if err := run(context.Background(), []string{"--config-root", configRoot, "--json", "runtime", "start", "--workspace", ws.ID, "--docker"}, &runtimeOut, io.Discard); err != nil {
		t.Fatalf("runtime start --docker error = %v", err)
	}
	var started daemon.RuntimeStatus
	if err := json.Unmarshal(runtimeOut.Bytes(), &started); err != nil {
		t.Fatalf("decode runtime start --docker: %v; output=%s", err, runtimeOut.String())
	}
	if started.State != daemon.RuntimeRunning || !started.Exposed || started.Listen != daemon.DockerRuntimeListen || started.DockerEndpoint == "" || started.LocalEndpoint == "" || started.MCPHost == nil {
		t.Fatalf("docker runtime status = %+v", started)
	}
	if !strings.Contains(started.DockerEndpoint, "host.docker.internal:") {
		t.Fatalf("docker endpoint = %q", started.DockerEndpoint)
	}
	assertRuntimeEndpointRead(t, started.LocalEndpoint, "docker-access")

	if err := run(context.Background(), []string{"--config-root", configRoot, "stop"}, io.Discard, io.Discard); err != nil {
		t.Fatalf("daemon stop error = %v", err)
	}
	if err := run(context.Background(), []string{"--config-root", configRoot, "start"}, io.Discard, io.Discard); err != nil {
		t.Fatalf("daemon restart error = %v", err)
	}
	var statusOut bytes.Buffer
	if err := run(context.Background(), []string{"--config-root", configRoot, "--json", "runtime", "status", "--workspace", ws.ID}, &statusOut, io.Discard); err != nil {
		t.Fatalf("runtime status after restart error = %v", err)
	}
	var recovered daemon.RuntimeStatus
	if err := json.Unmarshal(statusOut.Bytes(), &recovered); err != nil {
		t.Fatalf("decode recovered runtime: %v", err)
	}
	if recovered.State != daemon.RuntimeRunning || !recovered.Exposed || recovered.Listen != daemon.DockerRuntimeListen || recovered.DockerEndpoint == "" {
		t.Fatalf("recovered docker runtime = %+v", recovered)
	}
	assertRuntimeEndpointRead(t, recovered.LocalEndpoint, "docker-access")
}

func TestCLIDaemonCleanRestartReconcilesDesiredRuntimes(t *testing.T) {
	configRoot := t.TempDir()
	rootA := t.TempDir()
	rootB := t.TempDir()
	if err := os.WriteFile(filepath.Join(rootA, "value.txt"), []byte("restart-a"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(rootB, "value.txt"), []byte("restart-b"), 0o644); err != nil {
		t.Fatal(err)
	}

	addWorkspace := func(root string) workspaceOutput {
		t.Helper()
		var out bytes.Buffer
		if err := run(context.Background(), []string{"--config-root", configRoot, "--json", "workspace", "add", "--path", root}, &out, io.Discard); err != nil {
			t.Fatalf("workspace add %s error = %v", root, err)
		}
		var ws workspaceOutput
		if err := json.Unmarshal(out.Bytes(), &ws); err != nil {
			t.Fatalf("decode workspace add: %v", err)
		}
		return ws
	}
	wsA := addWorkspace(rootA)
	wsB := addWorkspace(rootB)
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_, _ = daemon.Stop(ctx, configRoot)
	})

	startDaemon := func() daemon.Metadata {
		t.Helper()
		var out bytes.Buffer
		if err := run(context.Background(), []string{"--config-root", configRoot, "--json", "start"}, &out, io.Discard); err != nil {
			t.Fatalf("daemon start error = %v", err)
		}
		var meta daemon.Metadata
		if err := json.Unmarshal(out.Bytes(), &meta); err != nil {
			t.Fatalf("decode daemon start: %v; output=%s", err, out.String())
		}
		return meta
	}
	startRuntime := func(workspaceID string) daemon.RuntimeStatus {
		t.Helper()
		var out bytes.Buffer
		if err := run(context.Background(), []string{"--config-root", configRoot, "--json", "runtime", "start", "--workspace", workspaceID}, &out, io.Discard); err != nil {
			t.Fatalf("runtime start %s error = %v", workspaceID, err)
		}
		var status daemon.RuntimeStatus
		if err := json.Unmarshal(out.Bytes(), &status); err != nil {
			t.Fatalf("decode runtime start: %v", err)
		}
		return status
	}
	statusRuntime := func(workspaceID string) daemon.RuntimeStatus {
		t.Helper()
		var out bytes.Buffer
		if err := run(context.Background(), []string{"--config-root", configRoot, "--json", "runtime", "status", "--workspace", workspaceID}, &out, io.Discard); err != nil {
			t.Fatalf("runtime status %s error = %v", workspaceID, err)
		}
		var status daemon.RuntimeStatus
		if err := json.Unmarshal(out.Bytes(), &status); err != nil {
			t.Fatalf("decode runtime status: %v", err)
		}
		return status
	}

	firstDaemon := startDaemon()
	firstA := startRuntime(wsA.ID)
	firstB := startRuntime(wsB.ID)
	if firstA.MCPHost == nil || firstB.MCPHost == nil {
		t.Fatalf("initial runtimes missing hosts: A=%+v B=%+v", firstA, firstB)
	}
	assertRuntimeEndpointRead(t, firstA.MCPHost.Endpoint, "restart-a")
	assertRuntimeEndpointRead(t, firstB.MCPHost.Endpoint, "restart-b")

	if err := run(context.Background(), []string{"--config-root", configRoot, "stop"}, io.Discard, io.Discard); err != nil {
		t.Fatalf("daemon stop error = %v", err)
	}
	assertRuntimeEndpointUnavailable(t, firstA.MCPHost.Endpoint)
	assertRuntimeEndpointUnavailable(t, firstB.MCPHost.Endpoint)

	secondDaemon := startDaemon()
	if secondDaemon.InstanceID == firstDaemon.InstanceID || secondDaemon.PID == firstDaemon.PID {
		t.Fatalf("daemon restart reused observed identity: first=%+v second=%+v", firstDaemon, secondDaemon)
	}
	secondA := statusRuntime(wsA.ID)
	secondB := statusRuntime(wsB.ID)
	if secondA.State != daemon.RuntimeRunning || secondB.State != daemon.RuntimeRunning || secondA.MCPHost == nil || secondB.MCPHost == nil {
		t.Fatalf("reconciled runtimes A=%+v B=%+v", secondA, secondB)
	}
	assertRuntimeEndpointRead(t, secondA.MCPHost.Endpoint, "restart-a")
	assertRuntimeEndpointRead(t, secondB.MCPHost.Endpoint, "restart-b")

	if err := run(context.Background(), []string{"--config-root", configRoot, "runtime", "stop", "--workspace", wsA.ID}, io.Discard, io.Discard); err != nil {
		t.Fatalf("runtime stop A error = %v", err)
	}
	if err := run(context.Background(), []string{"--config-root", configRoot, "stop"}, io.Discard, io.Discard); err != nil {
		t.Fatalf("second daemon stop error = %v", err)
	}
	thirdDaemon := startDaemon()
	if thirdDaemon.InstanceID == secondDaemon.InstanceID {
		t.Fatalf("third daemon reused instance id %s", thirdDaemon.InstanceID)
	}
	thirdA := statusRuntime(wsA.ID)
	thirdB := statusRuntime(wsB.ID)
	if thirdA.State != daemon.RuntimeStopped || thirdA.DesiredRunning {
		t.Fatalf("explicitly stopped A resurrected after restart: %+v", thirdA)
	}
	if thirdB.State != daemon.RuntimeRunning || !thirdB.DesiredRunning || thirdB.MCPHost == nil {
		t.Fatalf("desired B did not reconcile after restart: %+v", thirdB)
	}
	assertRuntimeEndpointRead(t, thirdB.MCPHost.Endpoint, "restart-b")
}

func TestCLIDaemonCrashRestartReclaimsStaleLeaseAndReconciles(t *testing.T) {
	configRoot := t.TempDir()
	workspaceRoot := t.TempDir()
	if err := os.WriteFile(filepath.Join(workspaceRoot, "value.txt"), []byte("crash-restart"), 0o644); err != nil {
		t.Fatal(err)
	}
	var addOut bytes.Buffer
	if err := run(context.Background(), []string{"--config-root", configRoot, "--json", "workspace", "add", "--path", workspaceRoot}, &addOut, io.Discard); err != nil {
		t.Fatalf("workspace add error = %v", err)
	}
	var ws workspaceOutput
	if err := json.Unmarshal(addOut.Bytes(), &ws); err != nil {
		t.Fatalf("decode workspace add: %v", err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_, _ = daemon.Stop(ctx, configRoot)
	})

	var startOut bytes.Buffer
	if err := run(context.Background(), []string{"--config-root", configRoot, "--json", "start"}, &startOut, io.Discard); err != nil {
		t.Fatalf("daemon start error = %v", err)
	}
	var firstDaemon daemon.Metadata
	if err := json.Unmarshal(startOut.Bytes(), &firstDaemon); err != nil {
		t.Fatalf("decode daemon start: %v", err)
	}
	var runtimeOut bytes.Buffer
	if err := run(context.Background(), []string{"--config-root", configRoot, "--json", "runtime", "start", "--workspace", ws.ID}, &runtimeOut, io.Discard); err != nil {
		t.Fatalf("runtime start error = %v", err)
	}
	var firstRuntime daemon.RuntimeStatus
	if err := json.Unmarshal(runtimeOut.Bytes(), &firstRuntime); err != nil {
		t.Fatalf("decode runtime start: %v", err)
	}
	if firstRuntime.MCPHost == nil {
		t.Fatalf("runtime missing host: %+v", firstRuntime)
	}
	assertRuntimeEndpointRead(t, firstRuntime.MCPHost.Endpoint, "crash-restart")

	process, err := os.FindProcess(firstDaemon.PID)
	if err != nil {
		t.Fatalf("FindProcess(%d) error = %v", firstDaemon.PID, err)
	}
	if err := process.Kill(); err != nil {
		t.Fatalf("Kill(%d) error = %v", firstDaemon.PID, err)
	}
	_ = process.Release()
	assertRuntimeEndpointUnavailable(t, firstRuntime.MCPHost.Endpoint)

	restartStarted := time.Now()
	var restartOut bytes.Buffer
	if err := run(context.Background(), []string{"--config-root", configRoot, "--json", "start"}, &restartOut, io.Discard); err != nil {
		t.Fatalf("daemon crash restart error = %v", err)
	}
	if time.Since(restartStarted) > leaseStaleBudgetForTest() {
		t.Fatalf("crash recovery exceeded bounded stale lease budget: %v", time.Since(restartStarted))
	}
	var secondDaemon daemon.Metadata
	if err := json.Unmarshal(restartOut.Bytes(), &secondDaemon); err != nil {
		t.Fatalf("decode crash restart daemon: %v", err)
	}
	if secondDaemon.InstanceID == firstDaemon.InstanceID || secondDaemon.PID == firstDaemon.PID {
		t.Fatalf("crash restart reused old daemon identity: first=%+v second=%+v", firstDaemon, secondDaemon)
	}
	var statusOut bytes.Buffer
	if err := run(context.Background(), []string{"--config-root", configRoot, "--json", "runtime", "status", "--workspace", ws.ID}, &statusOut, io.Discard); err != nil {
		t.Fatalf("runtime status after crash restart error = %v", err)
	}
	var recovered daemon.RuntimeStatus
	if err := json.Unmarshal(statusOut.Bytes(), &recovered); err != nil {
		t.Fatalf("decode recovered runtime: %v", err)
	}
	if recovered.State != daemon.RuntimeRunning || !recovered.DesiredRunning || recovered.MCPHost == nil {
		t.Fatalf("recovered runtime = %+v", recovered)
	}
	assertRuntimeEndpointRead(t, recovered.MCPHost.Endpoint, "crash-restart")
}

func assertRuntimeEndpointUnavailable(t *testing.T, endpoint string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
		client := mcp.NewClient(&mcp.Implementation{Name: "adm-runtime-unavailable", Version: "v0.3.0"}, nil)
		session, err := client.Connect(ctx, &mcp.StreamableClientTransport{Endpoint: endpoint}, nil)
		cancel()
		if err != nil {
			return
		}
		_ = session.Close()
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatalf("runtime endpoint %s remained reachable", endpoint)
}

func leaseStaleBudgetForTest() time.Duration {
	// Production Start owns the exact stale/recovery constants. The test only
	// asserts crash recovery is bounded rather than exposing those internals to
	// the CLI package.
	return 12 * time.Second
}

func TestCLIUpDownPSAndCtlRestart(t *testing.T) {
	configRoot := t.TempDir()
	workspaceRoot := t.TempDir()
	if err := os.WriteFile(filepath.Join(workspaceRoot, "value.txt"), []byte("ux-flow"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_, _ = daemon.Stop(ctx, configRoot)
	})

	var upOut bytes.Buffer
	if err := run(context.Background(), []string{"--config-root", configRoot, "--json", "up", "--docker", workspaceRoot}, &upOut, io.Discard); err != nil {
		t.Fatalf("up --docker error = %v", err)
	}
	var first upOutput
	if err := json.Unmarshal(upOut.Bytes(), &first); err != nil {
		t.Fatalf("decode up output: %v; output=%s", err, upOut.String())
	}
	if first.Workspace.ID == "" || first.Runtime.State != daemon.RuntimeRunning || first.Runtime.DockerEndpoint == "" {
		t.Fatalf("up output = %+v", first)
	}
	assertRuntimeEndpointRead(t, first.Runtime.LocalEndpoint, "ux-flow")

	var repeatOut bytes.Buffer
	if err := run(context.Background(), []string{"--config-root", configRoot, "--json", "up", "--docker", workspaceRoot}, &repeatOut, io.Discard); err != nil {
		t.Fatalf("repeat up error = %v", err)
	}
	var repeated upOutput
	if err := json.Unmarshal(repeatOut.Bytes(), &repeated); err != nil {
		t.Fatalf("decode repeat up: %v", err)
	}
	if repeated.Workspace.ID != first.Workspace.ID {
		t.Fatalf("repeat up created new workspace: first=%s repeat=%s", first.Workspace.ID, repeated.Workspace.ID)
	}

	var psOut bytes.Buffer
	if err := run(context.Background(), []string{"--config-root", configRoot, "--json", "ps"}, &psOut, io.Discard); err != nil {
		t.Fatalf("ps error = %v", err)
	}
	var listed psOutput
	if err := json.Unmarshal(psOut.Bytes(), &listed); err != nil {
		t.Fatalf("decode ps: %v; output=%s", err, psOut.String())
	}
	if listed.DaemonState != daemon.StateRunning || len(listed.Items) != 1 || listed.Items[0].State != daemon.RuntimeRunning || listed.Items[0].WorkspaceID != first.Workspace.ID {
		t.Fatalf("ps output = %+v", listed)
	}

	var restartOut bytes.Buffer
	if err := run(context.Background(), []string{"--config-root", configRoot, "--json", "ctl", "restart"}, &restartOut, io.Discard); err != nil {
		t.Fatalf("ctl restart error = %v", err)
	}
	var restarted daemon.Metadata
	if err := json.Unmarshal(restartOut.Bytes(), &restarted); err != nil {
		t.Fatalf("decode ctl restart: %v", err)
	}
	if restarted.State != daemon.StateRunning || restarted.InstanceID == first.Daemon.InstanceID {
		t.Fatalf("ctl restart output = %+v; first=%+v", restarted, first.Daemon)
	}

	var afterRestartPS bytes.Buffer
	if err := run(context.Background(), []string{"--config-root", configRoot, "--json", "ps"}, &afterRestartPS, io.Discard); err != nil {
		t.Fatalf("ps after restart error = %v", err)
	}
	listed = psOutput{}
	if err := json.Unmarshal(afterRestartPS.Bytes(), &listed); err != nil {
		t.Fatalf("decode ps after restart: %v", err)
	}
	if len(listed.Items) != 1 || listed.Items[0].State != daemon.RuntimeRunning || !listed.Items[0].DesiredRunning || listed.Items[0].DockerEndpoint == "" {
		t.Fatalf("ps after restart = %+v", listed)
	}

	var downOut bytes.Buffer
	if err := run(context.Background(), []string{"--config-root", configRoot, "--json", "down", workspaceRoot}, &downOut, io.Discard); err != nil {
		t.Fatalf("down by path error = %v", err)
	}
	var stopped daemon.RuntimeStatus
	if err := json.Unmarshal(downOut.Bytes(), &stopped); err != nil {
		t.Fatalf("decode down: %v", err)
	}
	if stopped.State != daemon.RuntimeStopped || stopped.DesiredRunning {
		t.Fatalf("down output = %+v", stopped)
	}

	if err := run(context.Background(), []string{"--config-root", configRoot, "ctl", "stop"}, io.Discard, io.Discard); err != nil {
		t.Fatalf("ctl stop error = %v", err)
	}
	var stoppedPS bytes.Buffer
	if err := run(context.Background(), []string{"--config-root", configRoot, "--json", "ps"}, &stoppedPS, io.Discard); err != nil {
		t.Fatalf("ps with daemon stopped error = %v", err)
	}
	listed = psOutput{}
	if err := json.Unmarshal(stoppedPS.Bytes(), &listed); err != nil {
		t.Fatalf("decode stopped ps: %v", err)
	}
	if listed.DaemonState != daemon.StateStopped || len(listed.Items) != 1 || listed.Items[0].DesiredRunning || listed.Items[0].State != daemon.RuntimeStopped {
		t.Fatalf("stopped ps = %+v", listed)
	}
}

func TestCLICtlShutdownClearsDesiredRuntimes(t *testing.T) {
	configRoot := t.TempDir()
	workspaceRoot := t.TempDir()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_, _ = daemon.Stop(ctx, configRoot)
	})

	var upOut bytes.Buffer
	if err := run(context.Background(), []string{"--config-root", configRoot, "--json", "up", workspaceRoot}, &upOut, io.Discard); err != nil {
		t.Fatalf("up error = %v", err)
	}
	var started upOutput
	if err := json.Unmarshal(upOut.Bytes(), &started); err != nil {
		t.Fatalf("decode up: %v", err)
	}
	if started.Runtime.State != daemon.RuntimeRunning || !started.Runtime.DesiredRunning {
		t.Fatalf("started runtime = %+v", started.Runtime)
	}

	var shutdownOut bytes.Buffer
	if err := run(context.Background(), []string{"--config-root", configRoot, "--json", "ctl", "shutdown"}, &shutdownOut, io.Discard); err != nil {
		t.Fatalf("ctl shutdown error = %v", err)
	}
	var stoppedDaemon daemon.Metadata
	if err := json.Unmarshal(shutdownOut.Bytes(), &stoppedDaemon); err != nil {
		t.Fatalf("decode shutdown: %v", err)
	}
	if stoppedDaemon.State != daemon.StateStopped {
		t.Fatalf("shutdown daemon = %+v", stoppedDaemon)
	}
	desired, err := daemon.NewDesiredStore(configRoot).LoadRuntimes()
	if err != nil {
		t.Fatalf("LoadRuntimes after shutdown error = %v", err)
	}
	if len(desired) != 0 {
		t.Fatalf("shutdown left desired runtimes: %+v", desired)
	}

	if err := run(context.Background(), []string{"--config-root", configRoot, "ctl", "start"}, io.Discard, io.Discard); err != nil {
		t.Fatalf("ctl start after shutdown error = %v", err)
	}
	var psOut bytes.Buffer
	if err := run(context.Background(), []string{"--config-root", configRoot, "--json", "ps"}, &psOut, io.Discard); err != nil {
		t.Fatalf("ps after shutdown/start error = %v", err)
	}
	var listed psOutput
	if err := json.Unmarshal(psOut.Bytes(), &listed); err != nil {
		t.Fatalf("decode ps after shutdown/start: %v", err)
	}
	if len(listed.Items) != 1 || listed.Items[0].State != daemon.RuntimeStopped || listed.Items[0].DesiredRunning {
		t.Fatalf("runtime resurrected after shutdown: %+v", listed)
	}
}

func TestCLIRejectsUnknownCommandAndMissingWorkspace(t *testing.T) {
	if err := run(context.Background(), []string{"--config-root", t.TempDir(), "unknown"}, io.Discard, io.Discard); err == nil {
		t.Fatal("unknown command unexpectedly succeeded")
	}
	if err := run(context.Background(), []string{"--config-root", t.TempDir(), "inspect"}, io.Discard, io.Discard); err == nil {
		t.Fatal("inspect without workspace unexpectedly succeeded")
	}
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
