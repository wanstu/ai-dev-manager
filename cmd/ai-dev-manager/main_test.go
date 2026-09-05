package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	goruntime "runtime"
	"strings"
	"testing"
	"time"

	"ai-dev-manager/internal/agent"
	"ai-dev-manager/internal/controlplane"
	"ai-dev-manager/internal/daemon"
	"ai-dev-manager/internal/environment"
	"ai-dev-manager/internal/model"

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

func TestCLIAgentRunLifecycleAcrossInvocationsAndRestart(t *testing.T) {
	configRoot := t.TempDir()
	workspaceA := t.TempDir()
	workspaceB := t.TempDir()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_, _ = daemon.Stop(ctx, configRoot)
	})

	var runAOut bytes.Buffer
	if err := run(context.Background(), []string{"--config-root", configRoot, "--json", "agent", "run", workspaceA}, &runAOut, io.Discard); err != nil {
		t.Fatalf("agent run A error = %v", err)
	}
	var runA agent.RunStatus
	if err := json.Unmarshal(runAOut.Bytes(), &runA); err != nil {
		t.Fatalf("decode agent run A: %v; output=%s", err, runAOut.String())
	}
	if runA.State != agent.StateRunning || runA.RunID == "" || runA.WorkspaceID == "" || runA.Executor != "lifecycle" {
		t.Fatalf("agent run A = %+v", runA)
	}

	var runBOut bytes.Buffer
	if err := run(context.Background(), []string{"--config-root", configRoot, "--json", "agent", "run", workspaceB}, &runBOut, io.Discard); err != nil {
		t.Fatalf("agent run B error = %v", err)
	}
	var runB agent.RunStatus
	if err := json.Unmarshal(runBOut.Bytes(), &runB); err != nil {
		t.Fatalf("decode agent run B: %v", err)
	}
	if runB.State != agent.StateRunning || runB.RunID == runA.RunID || runB.WorkspaceID == runA.WorkspaceID {
		t.Fatalf("agent run B = %+v; A=%+v", runB, runA)
	}

	var listOut bytes.Buffer
	if err := run(context.Background(), []string{"--config-root", configRoot, "--json", "agent", "list"}, &listOut, io.Discard); err != nil {
		t.Fatalf("agent list error = %v", err)
	}
	var listed []agent.RunStatus
	if err := json.Unmarshal(listOut.Bytes(), &listed); err != nil {
		t.Fatalf("decode agent list: %v; output=%s", err, listOut.String())
	}
	if len(listed) != 2 {
		t.Fatalf("agent list len = %d, want 2: %+v", len(listed), listed)
	}

	var statusOut bytes.Buffer
	if err := run(context.Background(), []string{"--config-root", configRoot, "--json", "agent", "status", runA.RunID}, &statusOut, io.Discard); err != nil {
		t.Fatalf("agent status A error = %v", err)
	}
	var observedA agent.RunStatus
	if err := json.Unmarshal(statusOut.Bytes(), &observedA); err != nil {
		t.Fatalf("decode agent status A: %v", err)
	}
	if observedA.State != agent.StateRunning || observedA.RunID != runA.RunID || observedA.WorkspaceID != runA.WorkspaceID {
		t.Fatalf("agent status A = %+v", observedA)
	}

	var cancelOut bytes.Buffer
	if err := run(context.Background(), []string{"--config-root", configRoot, "--json", "agent", "cancel", runA.RunID}, &cancelOut, io.Discard); err != nil {
		t.Fatalf("agent cancel A error = %v", err)
	}
	var cancelledA agent.RunStatus
	if err := json.Unmarshal(cancelOut.Bytes(), &cancelledA); err != nil {
		t.Fatalf("decode agent cancel A: %v", err)
	}
	if cancelledA.State != agent.StateCancelled || cancelledA.FinishedAt == nil {
		t.Fatalf("agent cancel A = %+v", cancelledA)
	}

	var repeatCancelOut bytes.Buffer
	if err := run(context.Background(), []string{"--config-root", configRoot, "--json", "agent", "cancel", runA.RunID}, &repeatCancelOut, io.Discard); err != nil {
		t.Fatalf("repeat agent cancel A error = %v", err)
	}
	var repeated agent.RunStatus
	if err := json.Unmarshal(repeatCancelOut.Bytes(), &repeated); err != nil {
		t.Fatalf("decode repeat cancel A: %v", err)
	}
	if repeated.State != agent.StateCancelled || repeated.RunID != runA.RunID {
		t.Fatalf("repeat agent cancel A = %+v", repeated)
	}

	var statusBOut bytes.Buffer
	if err := run(context.Background(), []string{"--config-root", configRoot, "--json", "agent", "status", runB.RunID}, &statusBOut, io.Discard); err != nil {
		t.Fatalf("agent status B error = %v", err)
	}
	var observedB agent.RunStatus
	if err := json.Unmarshal(statusBOut.Bytes(), &observedB); err != nil {
		t.Fatalf("decode agent status B: %v", err)
	}
	if observedB.State != agent.StateRunning {
		t.Fatalf("agent B changed when A cancelled: %+v", observedB)
	}

	if err := run(context.Background(), []string{"--config-root", configRoot, "ctl", "stop"}, io.Discard, io.Discard); err != nil {
		t.Fatalf("ctl stop with active agent error = %v", err)
	}
	if err := run(context.Background(), []string{"--config-root", configRoot, "ctl", "start"}, io.Discard, io.Discard); err != nil {
		t.Fatalf("ctl start after agent stop error = %v", err)
	}

	var afterRestartOut bytes.Buffer
	if err := run(context.Background(), []string{"--config-root", configRoot, "--json", "agent", "list"}, &afterRestartOut, io.Discard); err != nil {
		t.Fatalf("agent list after restart error = %v", err)
	}
	listed = nil
	if err := json.Unmarshal(afterRestartOut.Bytes(), &listed); err != nil {
		t.Fatalf("decode agent list after restart: %v; output=%s", err, afterRestartOut.String())
	}
	if len(listed) != 0 {
		t.Fatalf("agent runs resurrected after daemon restart: %+v", listed)
	}
}

func TestCLIAgentVerifyWorkflowAcrossInvocations(t *testing.T) {
	configRoot := t.TempDir()
	workspaceRoot := t.TempDir()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_, _ = daemon.Stop(ctx, configRoot)
	})

	if err := os.WriteFile(filepath.Join(workspaceRoot, "go.mod"), []byte("module example.com/phase16\n\ngo 1.25\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(workspaceRoot, "sample.go"), []byte("package sample\n\nfunc Add(a, b int) int { return a + b }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitPath, err := exec.LookPath("git")
	if err != nil {
		t.Fatalf("git required for verify workflow acceptance: %v", err)
	}
	if output, err := exec.Command(gitPath, "init", workspaceRoot).CombinedOutput(); err != nil {
		t.Fatalf("git init error = %v output=%s", err, output)
	}
	goPath := filepath.Join(goruntime.GOROOT(), "bin", "go")
	if goruntime.GOOS == "windows" {
		goPath += ".exe"
	}
	if _, err := os.Stat(goPath); err != nil {
		t.Fatalf("go tool missing at %s: %v", goPath, err)
	}

	service, err := controlplane.New(configRoot)
	if err != nil {
		t.Fatal(err)
	}
	enabled := true
	if err := service.Store().SaveProject(workspaceRoot, model.ConfigLayer{
		Scope: model.ScopeProject,
		Policy: &model.Policy{
			Mode: "full",
			ToolPaths: map[string]string{
				"git": gitPath,
				"go":  goPath,
			},
		},
		Verifiers: map[string]model.VerifierDefinition{
			"test": {ID: "test", Kind: "test", Enabled: &enabled, Executable: "go", Args: []string{"test", "./..."}},
		},
	}); err != nil {
		t.Fatalf("SaveProject() error = %v", err)
	}

	var runOut bytes.Buffer
	if err := run(context.Background(), []string{"--config-root", configRoot, "--json", "agent", "run", "--workflow", "verify", workspaceRoot}, &runOut, io.Discard); err != nil {
		t.Fatalf("agent run --workflow verify error = %v", err)
	}
	var started agent.RunStatus
	if err := json.Unmarshal(runOut.Bytes(), &started); err != nil {
		t.Fatalf("decode verify run: %v output=%s", err, runOut.String())
	}
	if started.RunID == "" || started.Executor != "verify" || started.State != agent.StateRunning {
		t.Fatalf("verify run start = %+v", started)
	}

	deadline := time.Now().Add(10 * time.Second)
	var observed agent.RunStatus
	for time.Now().Before(deadline) {
		var statusOut bytes.Buffer
		if err := run(context.Background(), []string{"--config-root", configRoot, "--json", "agent", "status", started.RunID}, &statusOut, io.Discard); err != nil {
			t.Fatalf("agent status verify error = %v", err)
		}
		if err := json.Unmarshal(statusOut.Bytes(), &observed); err != nil {
			t.Fatalf("decode verify status: %v output=%s", err, statusOut.String())
		}
		if observed.State != agent.StateRunning {
			break
		}
		time.Sleep(25 * time.Millisecond)
	}
	if observed.State != agent.StateCompleted {
		t.Fatalf("verify workflow state = %+v", observed)
	}
	if observed.Workflow != "verify" || observed.Stage != agent.StageCompleted || observed.Plan == nil || len(observed.Plan.Steps) != 3 || len(observed.Steps) != 3 {
		t.Fatalf("verify workflow audit trail = %+v", observed)
	}
	if observed.Review == nil || observed.Review.Decision != agent.ReviewPass {
		t.Fatalf("verify workflow review = %+v", observed.Review)
	}
}

func TestCLIAgentGSDWorkflowAcrossInvocations(t *testing.T) {
	configRoot := t.TempDir()
	workspaceRoot := t.TempDir()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_, _ = daemon.Stop(ctx, configRoot)
	})

	writeFixture := func(path, content string) {
		t.Helper()
		full := filepath.Join(workspaceRoot, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	writeFixture("sample.txt", "before\n")
	writeFixture("go.mod", "module example.com/gsdfixture\n\ngo 1.25\n")
	writeFixture("sample.go", "package sample\n\nfunc Add(a, b int) int { return a + b }\n")
	writeFixture(".planning/PROJECT.md", "# Project\n")
	writeFixture(".planning/STATE.md", "## Current Position\n\nPhase: 1 — Fixture Phase\nPlan: 01-01 — Fixture Plan\nStatus: In Progress\n")
	writeFixture(".planning/phases/01-fixture/01-CONTEXT.md", "# Context\n")
	writeFixture(".planning/phases/01-fixture/01-01-PLAN.md", `# Plan

## Execution Spec

`+"```json"+`
{
  "steps": [
    {
      "id": "edit-sample",
      "operation": "files.edit",
      "purpose": "apply fixture edit",
      "input": {
        "path": "sample.txt",
        "old_text": "before",
        "new_text": "after",
        "expected_replacements": 1
      }
    }
  ],
  "run_verifiers": true
}
`+"```"+`
`)
	writeFixture(".planning/phases/01-fixture/01-02-PLAN.md", "# Phase 1 Plan 01-02 — Next Fixture Plan\n")

	goPath := filepath.Join(goruntime.GOROOT(), "bin", "go")
	if goruntime.GOOS == "windows" {
		goPath += ".exe"
	}
	service, err := controlplane.New(configRoot)
	if err != nil {
		t.Fatal(err)
	}
	enabled := true
	if err := service.Store().SaveProject(workspaceRoot, model.ConfigLayer{
		Scope: model.ScopeProject,
		Policy: &model.Policy{
			Mode:               "standard",
			AllowedExecutables: []string{"go"},
			ToolPaths:          map[string]string{"go": goPath},
		},
		Verifiers: map[string]model.VerifierDefinition{
			"test": {ID: "test", Kind: "test", Enabled: &enabled, Executable: "go", Args: []string{"test", "./..."}},
		},
	}); err != nil {
		t.Fatalf("SaveProject() error = %v", err)
	}

	var runOut bytes.Buffer
	if err := run(context.Background(), []string{"--config-root", configRoot, "--json", "agent", "run", "--workflow", "gsd", workspaceRoot}, &runOut, io.Discard); err != nil {
		t.Fatalf("agent run --workflow gsd error = %v", err)
	}
	var started agent.RunStatus
	if err := json.Unmarshal(runOut.Bytes(), &started); err != nil {
		t.Fatalf("decode gsd run: %v output=%s", err, runOut.String())
	}
	if started.Executor != "gsd" || started.State != agent.StateRunning {
		t.Fatalf("gsd run start = %+v", started)
	}

	deadline := time.Now().Add(10 * time.Second)
	var observed agent.RunStatus
	for time.Now().Before(deadline) {
		var statusOut bytes.Buffer
		if err := run(context.Background(), []string{"--config-root", configRoot, "--json", "agent", "status", started.RunID}, &statusOut, io.Discard); err != nil {
			t.Fatalf("agent status gsd error = %v", err)
		}
		if err := json.Unmarshal(statusOut.Bytes(), &observed); err != nil {
			t.Fatalf("decode gsd status: %v output=%s", err, statusOut.String())
		}
		if observed.State != agent.StateRunning {
			break
		}
		time.Sleep(25 * time.Millisecond)
	}
	if observed.State != agent.StateCompleted || observed.Review == nil || observed.Review.Decision != agent.ReviewPass {
		t.Fatalf("gsd workflow result = %+v", observed)
	}
	if observed.Plan == nil || observed.Plan.Planning == nil || observed.Plan.Planning.Phase != 1 || observed.Plan.Planning.PlanID != "01-01" {
		t.Fatalf("gsd planning provenance = %+v", observed.Plan)
	}
	if observed.Advance == nil || observed.Advance.Status != agent.AdvanceAdvanced || observed.Advance.ToPhase != 1 || observed.Advance.ToPlan != "01-02" {
		t.Fatalf("gsd advance result = %+v", observed.Advance)
	}
	edited, err := os.ReadFile(filepath.Join(workspaceRoot, "sample.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(edited) != "after\n" {
		t.Fatalf("GSD edit result = %q, want after", edited)
	}
	stateData, err := os.ReadFile(filepath.Join(workspaceRoot, ".planning", "STATE.md"))
	if err != nil {
		t.Fatal(err)
	}
	stateText := string(stateData)
	if !strings.Contains(stateText, "Plan: 01-02 — Next Fixture Plan") || !strings.Contains(stateText, "Status: In Progress") {
		t.Fatalf("GSD STATE was not advanced to pre-existing next plan: %s", stateText)
	}
}

func TestCLIAgentParallelVerifyWorktreesAcrossInvocations(t *testing.T) {
	configRoot := t.TempDir()
	workspaceRoot := t.TempDir()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_, _ = daemon.Stop(ctx, configRoot)
	})

	gitPath, err := exec.LookPath("git")
	if err != nil {
		t.Fatalf("git required for parallel worktree acceptance: %v", err)
	}
	gitRun := func(args ...string) string {
		t.Helper()
		cmd := exec.Command(gitPath, args...)
		cmd.Dir = workspaceRoot
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %s error = %v output=%s", strings.Join(args, " "), err, output)
		}
		return strings.TrimSpace(string(output))
	}
	gitRun("init")
	gitRun("config", "user.email", "phase18@example.invalid")
	gitRun("config", "user.name", "Phase 18")
	if err := os.WriteFile(filepath.Join(workspaceRoot, "go.mod"), []byte("module example.com/phase18\n\ngo 1.25\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(workspaceRoot, "sample.go"), []byte("package sample\n\nfunc Add(a, b int) int { return a + b }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun("add", "go.mod", "sample.go")
	gitRun("commit", "-m", "baseline")
	gitRun("branch", "-M", "main")
	beforeHead := gitRun("rev-parse", "HEAD")
	beforeBranch := gitRun("branch", "--show-current")

	goPath := filepath.Join(goruntime.GOROOT(), "bin", "go")
	if goruntime.GOOS == "windows" {
		goPath += ".exe"
	}
	service, err := controlplane.New(configRoot)
	if err != nil {
		t.Fatal(err)
	}
	enabled := true
	if err := service.Store().SaveProject(workspaceRoot, model.ConfigLayer{
		Scope: model.ScopeProject,
		Policy: &model.Policy{
			Mode: "full",
			ToolPaths: map[string]string{
				"git": gitPath,
				"go":  goPath,
			},
		},
		Verifiers: map[string]model.VerifierDefinition{
			"test": {ID: "test", Kind: "test", Enabled: &enabled, Executable: "go", Args: []string{"test", "./..."}},
		},
	}); err != nil {
		t.Fatalf("SaveProject() error = %v", err)
	}

	var runOut bytes.Buffer
	args := []string{
		"--config-root", configRoot, "--json", "agent", "run",
		"--workflow", "parallel-verify",
		"--lane", "p18-a:p18-branch-a",
		"--lane", "p18-b:p18-branch-b",
		workspaceRoot,
	}
	if err := run(context.Background(), args, &runOut, io.Discard); err != nil {
		t.Fatalf("parallel agent run error = %v", err)
	}
	var started agent.RunStatus
	if err := json.Unmarshal(runOut.Bytes(), &started); err != nil {
		t.Fatalf("decode parallel run: %v output=%s", err, runOut.String())
	}
	if started.Executor != "parallel-verify" || started.State != agent.StateRunning {
		t.Fatalf("parallel start = %+v", started)
	}

	deadline := time.Now().Add(15 * time.Second)
	var observed agent.RunStatus
	for time.Now().Before(deadline) {
		var statusOut bytes.Buffer
		if err := run(context.Background(), []string{"--config-root", configRoot, "--json", "agent", "status", started.RunID}, &statusOut, io.Discard); err != nil {
			t.Fatalf("parallel status error = %v", err)
		}
		if err := json.Unmarshal(statusOut.Bytes(), &observed); err != nil {
			t.Fatalf("decode parallel status: %v output=%s", err, statusOut.String())
		}
		if observed.State != agent.StateRunning {
			break
		}
		time.Sleep(25 * time.Millisecond)
	}
	if observed.State != agent.StateCompleted || observed.Review == nil || observed.Review.Decision != agent.ReviewPass || observed.Parallel == nil {
		t.Fatalf("parallel result = %+v", observed)
	}
	if len(observed.Parallel.Lanes) != 2 {
		t.Fatalf("parallel lane count = %+v", observed.Parallel)
	}
	paths := map[string]bool{}
	for _, lane := range observed.Parallel.Lanes {
		if lane.State != agent.ParallelLaneCompleted || lane.Review == nil || lane.Review.Decision != agent.ReviewPass || lane.Cleanup != "removed" {
			t.Fatalf("parallel lane audit = %+v", lane)
		}
		if lane.WorktreePath == "" {
			t.Fatalf("lane missing worktree path: %+v", lane)
		}
		paths[strings.ToLower(filepath.Clean(lane.WorktreePath))] = true
		if _, err := os.Stat(lane.WorktreePath); !os.IsNotExist(err) {
			t.Fatalf("lane worktree still exists after cleanup: path=%s err=%v", lane.WorktreePath, err)
		}
	}
	if len(paths) != 2 {
		t.Fatalf("parallel lanes shared a worktree root: %+v", observed.Parallel.Lanes)
	}
	if afterHead := gitRun("rev-parse", "HEAD"); afterHead != beforeHead {
		t.Fatalf("main HEAD changed: %s -> %s", beforeHead, afterHead)
	}
	if afterBranch := gitRun("branch", "--show-current"); afterBranch != beforeBranch {
		t.Fatalf("main branch changed: %s -> %s", beforeBranch, afterBranch)
	}
	worktreeList := gitRun("worktree", "list", "--porcelain")
	if strings.Count(worktreeList, "worktree ") != 1 {
		t.Fatalf("managed worktrees not cleaned up:\n%s", worktreeList)
	}
}

func TestCLIEnvironmentLifecycleAcrossDaemonRestart(t *testing.T) {
	configRoot := t.TempDir()
	workspaceRoot := t.TempDir()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_, _ = daemon.Stop(ctx, configRoot)
	})

	gitPath, err := exec.LookPath("git")
	if err != nil {
		t.Fatalf("git required for environment acceptance: %v", err)
	}
	gitRun := func(args ...string) string {
		t.Helper()
		cmd := exec.Command(gitPath, args...)
		cmd.Dir = workspaceRoot
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %s error = %v output=%s", strings.Join(args, " "), err, output)
		}
		return strings.TrimSpace(string(output))
	}
	gitRun("init")
	gitRun("config", "user.email", "phase19@example.invalid")
	gitRun("config", "user.name", "Phase 19")
	if err := os.WriteFile(filepath.Join(workspaceRoot, "source.txt"), []byte("one\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun("add", "source.txt")
	gitRun("commit", "-m", "one")
	gitRun("branch", "-M", "dev")
	firstHead := gitRun("rev-parse", "HEAD")
	if err := os.WriteFile(filepath.Join(workspaceRoot, "source.txt"), []byte("two\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(workspaceRoot, "second.txt"), []byte("second\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun("add", "source.txt", "second.txt")
	gitRun("commit", "-m", "two")
	secondHead := gitRun("rev-parse", "HEAD")
	if err := os.WriteFile(filepath.Join(workspaceRoot, "source.txt"), []byte("dirty-main\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	service, err := controlplane.New(configRoot)
	if err != nil {
		t.Fatal(err)
	}
	if err := service.Store().SaveProject(workspaceRoot, model.ConfigLayer{
		Scope: model.ScopeProject,
		Policy: &model.Policy{
			Mode:               "standard",
			AllowedExecutables: []string{"git"},
			ToolPaths:          map[string]string{"git": gitPath},
		},
	}); err != nil {
		t.Fatal(err)
	}

	create := func(name string, extra ...string) environment.InspectResult {
		t.Helper()
		args := []string{"--config-root", configRoot, "--json", "env", "create", "--name", name}
		args = append(args, extra...)
		args = append(args, workspaceRoot)
		var out bytes.Buffer
		if err := run(context.Background(), args, &out, io.Discard); err != nil {
			t.Fatalf("env create %s error = %v", name, err)
		}
		var result environment.InspectResult
		if err := json.Unmarshal(out.Bytes(), &result); err != nil {
			t.Fatalf("decode env create %s: %v output=%s", name, err, out.String())
		}
		return result
	}

	a := create("phase19-a")
	b := create("phase19-b", "--base", firstHead)
	if a.Environment.ID == "" || b.Environment.ID == "" || a.Environment.ID == b.Environment.ID {
		t.Fatalf("invalid environment identities: A=%+v B=%+v", a.Environment, b.Environment)
	}
	if a.Environment.BaseRef != "dev" || a.Environment.BaseCommit != secondHead {
		t.Fatalf("default environment base = %+v", a.Environment)
	}
	if b.Environment.BaseCommit != firstHead {
		t.Fatalf("explicit base = %q, want %q", b.Environment.BaseCommit, firstHead)
	}
	if len(a.Warnings) == 0 || a.Warnings[0].Code != "changes_not_included" {
		t.Fatalf("dirty main warning missing: %+v", a.Warnings)
	}
	if samePathForTest(a.Environment.WorktreePath, b.Environment.WorktreePath) {
		t.Fatalf("environments share worktree path: %q", a.Environment.WorktreePath)
	}
	if got := strings.TrimSpace(string(mustReadFile(t, filepath.Join(a.Environment.WorktreePath, "source.txt")))); got != "two" {
		t.Fatalf("dirty main leaked into A: %q", got)
	}
	if _, err := os.Stat(filepath.Join(b.Environment.WorktreePath, "second.txt")); !os.IsNotExist(err) {
		t.Fatalf("explicit old base contains second.txt: %v", err)
	}

	var listOut bytes.Buffer
	if err := run(context.Background(), []string{"--config-root", configRoot, "--json", "env", "list"}, &listOut, io.Discard); err != nil {
		t.Fatalf("env list error = %v", err)
	}
	var listed []environment.Environment
	if err := json.Unmarshal(listOut.Bytes(), &listed); err != nil || len(listed) != 2 {
		t.Fatalf("env list = %+v, decode err=%v output=%s", listed, err, listOut.String())
	}

	if err := run(context.Background(), []string{"--config-root", configRoot, "ctl", "stop"}, io.Discard, io.Discard); err != nil {
		t.Fatalf("ctl stop error = %v", err)
	}
	if err := run(context.Background(), []string{"--config-root", configRoot, "ctl", "start"}, io.Discard, io.Discard); err != nil {
		t.Fatalf("ctl start error = %v", err)
	}
	var inspectOut bytes.Buffer
	if err := run(context.Background(), []string{"--config-root", configRoot, "--json", "env", "inspect", a.Environment.ID}, &inspectOut, io.Discard); err != nil {
		t.Fatalf("env inspect after restart error = %v", err)
	}
	var inspected environment.InspectResult
	if err := json.Unmarshal(inspectOut.Bytes(), &inspected); err != nil {
		t.Fatalf("decode env inspect: %v output=%s", err, inspectOut.String())
	}
	if inspected.Environment.ID != a.Environment.ID || inspected.Environment.State != environment.StateReady {
		t.Fatalf("inspect after restart = %+v", inspected.Environment)
	}

	if err := os.WriteFile(filepath.Join(a.Environment.WorktreePath, "source.txt"), []byte("dirty-a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := run(context.Background(), []string{"--config-root", configRoot, "env", "destroy", a.Environment.ID}, io.Discard, io.Discard); err == nil {
		t.Fatal("dirty environment destroy unexpectedly succeeded")
	}
	if err := run(context.Background(), []string{"--config-root", configRoot, "env", "destroy", b.Environment.ID}, io.Discard, io.Discard); err != nil {
		t.Fatalf("clean environment destroy error = %v", err)
	}
	if _, err := os.Stat(b.Environment.WorktreePath); !os.IsNotExist(err) {
		t.Fatalf("destroyed worktree still exists: %v", err)
	}
	if got := gitRun("branch", "--list", "adm/phase19-b"); !strings.Contains(got, "adm/phase19-b") {
		t.Fatalf("destroy deleted branch: %q", got)
	}
}

func mustReadFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func samePathForTest(left, right string) bool {
	return strings.EqualFold(filepath.Clean(left), filepath.Clean(right))
}

func TestCLIEnvironmentIncludeChangesAndForceAcrossRestart(t *testing.T) {
	configRoot := t.TempDir()
	workspaceRoot := t.TempDir()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_, _ = daemon.Stop(ctx, configRoot)
	})

	gitPath, err := exec.LookPath("git")
	if err != nil {
		t.Fatalf("git required for Phase 20 acceptance: %v", err)
	}
	gitRun := func(cwd string, args ...string) string {
		t.Helper()
		cmd := exec.Command(gitPath, args...)
		cmd.Dir = cwd
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %s error = %v output=%s", strings.Join(args, " "), err, output)
		}
		return strings.TrimSpace(string(output))
	}
	gitRun(workspaceRoot, "init")
	gitRun(workspaceRoot, "config", "user.email", "phase20@example.invalid")
	gitRun(workspaceRoot, "config", "user.name", "Phase 20")
	gitRun(workspaceRoot, "config", "core.autocrlf", "false")
	if err := os.WriteFile(filepath.Join(workspaceRoot, ".gitignore"), []byte("*.tmp\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(workspaceRoot, "source.txt"), []byte("base\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(workspaceRoot, "add", ".gitignore", "source.txt")
	gitRun(workspaceRoot, "commit", "-m", "baseline")
	gitRun(workspaceRoot, "branch", "-M", "dev")

	service, err := controlplane.New(configRoot)
	if err != nil {
		t.Fatal(err)
	}
	if err := service.Store().SaveProject(workspaceRoot, model.ConfigLayer{
		Scope: model.ScopeProject,
		Policy: &model.Policy{
			Mode:               "standard",
			AllowedExecutables: []string{"git"},
			ToolPaths:          map[string]string{"git": gitPath},
		},
	}); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(workspaceRoot, "source.txt"), []byte("staged\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(workspaceRoot, "add", "source.txt")
	if err := os.WriteFile(filepath.Join(workspaceRoot, "source.txt"), []byte("staged\nunstaged\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(workspaceRoot, "note.txt"), []byte("note\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(workspaceRoot, "secret.tmp"), []byte("ignored\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var createOut bytes.Buffer
	if err := run(context.Background(), []string{
		"--config-root", configRoot, "--json", "env", "create",
		"--name", "phase20-transfer", "--include-changes", workspaceRoot,
	}, &createOut, io.Discard); err != nil {
		t.Fatalf("env create --include-changes error = %v", err)
	}
	var created environment.InspectResult
	if err := json.Unmarshal(createOut.Bytes(), &created); err != nil {
		t.Fatalf("decode include-changes create: %v output=%s", err, createOut.String())
	}
	if created.Environment.State != environment.StateReady || created.Environment.Metadata["changes_included"] != "true" {
		t.Fatalf("created environment = %+v", created.Environment)
	}
	if staged := gitRun(created.Environment.WorktreePath, "diff", "--cached", "--name-only"); !strings.Contains(staged, "source.txt") {
		t.Fatalf("staged state not transferred: %q", staged)
	}
	if unstaged := gitRun(created.Environment.WorktreePath, "diff", "--name-only"); !strings.Contains(unstaged, "source.txt") {
		t.Fatalf("unstaged state not transferred: %q", unstaged)
	}
	if got := strings.TrimSpace(string(mustReadFile(t, filepath.Join(created.Environment.WorktreePath, "note.txt")))); got != "note" {
		t.Fatalf("untracked file not transferred: %q", got)
	}
	if _, err := os.Stat(filepath.Join(created.Environment.WorktreePath, "secret.tmp")); !os.IsNotExist(err) {
		t.Fatalf("ignored file transferred: %v", err)
	}

	if err := run(context.Background(), []string{"--config-root", configRoot, "ctl", "stop"}, io.Discard, io.Discard); err != nil {
		t.Fatalf("ctl stop error = %v", err)
	}
	if err := run(context.Background(), []string{"--config-root", configRoot, "ctl", "start"}, io.Discard, io.Discard); err != nil {
		t.Fatalf("ctl start error = %v", err)
	}
	var inspectOut bytes.Buffer
	if err := run(context.Background(), []string{"--config-root", configRoot, "--json", "env", "inspect", created.Environment.ID}, &inspectOut, io.Discard); err != nil {
		t.Fatalf("env inspect after restart error = %v", err)
	}
	var inspected environment.InspectResult
	if err := json.Unmarshal(inspectOut.Bytes(), &inspected); err != nil || inspected.Environment.State != environment.StateReady {
		t.Fatalf("inspect after restart = %+v decode=%v", inspected.Environment, err)
	}

	if err := run(context.Background(), []string{"--config-root", configRoot, "env", "destroy", created.Environment.ID}, io.Discard, io.Discard); err == nil {
		t.Fatal("normal destroy unexpectedly accepted transferred dirty environment")
	}
	if err := run(context.Background(), []string{"--config-root", configRoot, "env", "destroy", "--force", created.Environment.ID}, io.Discard, io.Discard); err != nil {
		t.Fatalf("env destroy --force error = %v", err)
	}
	if _, err := os.Stat(created.Environment.WorktreePath); !os.IsNotExist(err) {
		t.Fatalf("force destroy left worktree: %v", err)
	}
	if branch := gitRun(workspaceRoot, "branch", "--list", created.Environment.Branch); !strings.Contains(branch, created.Environment.Branch) {
		t.Fatalf("force destroy deleted branch: %q", branch)
	}
}

func TestCLIEnvironmentWriterAndBaseFactsAcrossRestart(t *testing.T) {
	configRoot := t.TempDir()
	workspaceRoot := t.TempDir()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_, _ = daemon.Stop(ctx, configRoot)
	})

	gitPath, err := exec.LookPath("git")
	if err != nil {
		t.Fatalf("git required for Phase 21 acceptance: %v", err)
	}
	gitRun := func(cwd string, args ...string) string {
		t.Helper()
		cmd := exec.Command(gitPath, args...)
		cmd.Dir = cwd
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %s error = %v output=%s", strings.Join(args, " "), err, output)
		}
		return strings.TrimSpace(string(output))
	}
	gitRun(workspaceRoot, "init")
	gitRun(workspaceRoot, "config", "user.email", "phase21@example.invalid")
	gitRun(workspaceRoot, "config", "user.name", "Phase 21")
	if err := os.WriteFile(filepath.Join(workspaceRoot, "source.txt"), []byte("base\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(workspaceRoot, "add", "source.txt")
	gitRun(workspaceRoot, "commit", "-m", "baseline")
	gitRun(workspaceRoot, "branch", "-M", "dev")

	service, err := controlplane.New(configRoot)
	if err != nil {
		t.Fatal(err)
	}
	if err := service.Store().SaveProject(workspaceRoot, model.ConfigLayer{
		Scope: model.ScopeProject,
		Policy: &model.Policy{
			Mode:               "standard",
			AllowedExecutables: []string{"git"},
			ToolPaths:          map[string]string{"git": gitPath},
		},
	}); err != nil {
		t.Fatal(err)
	}

	var createOut bytes.Buffer
	if err := run(context.Background(), []string{"--config-root", configRoot, "--json", "env", "create", "--name", "phase21-writer", workspaceRoot}, &createOut, io.Discard); err != nil {
		t.Fatalf("env create error = %v", err)
	}
	var created environment.InspectResult
	if err := json.Unmarshal(createOut.Bytes(), &created); err != nil {
		t.Fatalf("decode create: %v output=%s", err, createOut.String())
	}
	if created.Environment.ID == "" || created.Environment.WorktreePath == "" {
		t.Fatalf("created environment = %+v", created.Environment)
	}
	envHead := gitRun(created.Environment.WorktreePath, "rev-parse", "HEAD")

	var acquireOut bytes.Buffer
	if err := run(context.Background(), []string{"--config-root", configRoot, "--json", "env", "writer", "acquire", "--owner", "owner-a", created.Environment.ID}, &acquireOut, io.Discard); err != nil {
		t.Fatalf("writer acquire error = %v", err)
	}
	var acquired environment.Environment
	if err := json.Unmarshal(acquireOut.Bytes(), &acquired); err != nil || acquired.Writer == nil || acquired.Writer.Owner != "owner-a" {
		t.Fatalf("writer acquire = %+v decode=%v output=%s", acquired.Writer, err, acquireOut.String())
	}

	if err := run(context.Background(), []string{"--config-root", configRoot, "ctl", "stop"}, io.Discard, io.Discard); err != nil {
		t.Fatalf("ctl stop error = %v", err)
	}
	if err := run(context.Background(), []string{"--config-root", configRoot, "ctl", "start"}, io.Discard, io.Discard); err != nil {
		t.Fatalf("ctl start error = %v", err)
	}

	var inspectOut bytes.Buffer
	if err := run(context.Background(), []string{"--config-root", configRoot, "--json", "env", "inspect", created.Environment.ID}, &inspectOut, io.Discard); err != nil {
		t.Fatalf("inspect after restart error = %v", err)
	}
	var inspected environment.InspectResult
	if err := json.Unmarshal(inspectOut.Bytes(), &inspected); err != nil {
		t.Fatalf("decode inspect after restart: %v output=%s", err, inspectOut.String())
	}
	if inspected.Environment.Writer == nil || inspected.Environment.Writer.Owner != "owner-a" {
		t.Fatalf("writer did not survive restart: %+v", inspected.Environment.Writer)
	}
	if strings.Contains(inspectOut.String(), "required_action") || strings.Contains(inspectOut.String(), "next_step") {
		t.Fatalf("inspect exposed prescriptive action fields: %s", inspectOut.String())
	}

	if err := run(context.Background(), []string{"--config-root", configRoot, "env", "writer", "acquire", "--owner", "owner-b", created.Environment.ID}, io.Discard, io.Discard); err == nil {
		t.Fatal("second writer unexpectedly acquired Environment")
	}
	if err := run(context.Background(), []string{"--config-root", configRoot, "env", "writer", "acquire", "--owner", "owner-a", created.Environment.ID}, io.Discard, io.Discard); err != nil {
		t.Fatalf("same writer renew error = %v", err)
	}

	if err := os.WriteFile(filepath.Join(workspaceRoot, "base-new.txt"), []byte("new base\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(workspaceRoot, "add", "base-new.txt")
	gitRun(workspaceRoot, "commit", "-m", "base advances")

	inspectOut.Reset()
	if err := run(context.Background(), []string{"--config-root", configRoot, "--json", "env", "inspect", created.Environment.ID}, &inspectOut, io.Discard); err != nil {
		t.Fatalf("inspect after base advance error = %v", err)
	}
	if err := json.Unmarshal(inspectOut.Bytes(), &inspected); err != nil {
		t.Fatalf("decode inspect after base advance: %v", err)
	}
	behind := -1
	for _, fact := range inspected.Facts {
		if fact.Code == "behind" {
			if value, ok := fact.Value.(float64); ok {
				behind = int(value)
			}
		}
	}
	if behind != 1 {
		t.Fatalf("behind = %d, want 1; facts=%+v", behind, inspected.Facts)
	}
	if got := gitRun(created.Environment.WorktreePath, "rev-parse", "HEAD"); got != envHead {
		t.Fatalf("inspect auto-synchronized Environment HEAD: %s -> %s", envHead, got)
	}

	var releaseOut bytes.Buffer
	if err := run(context.Background(), []string{"--config-root", configRoot, "--json", "env", "writer", "release", "--force", created.Environment.ID}, &releaseOut, io.Discard); err != nil {
		t.Fatalf("force writer release error = %v", err)
	}
	var released environment.Environment
	if err := json.Unmarshal(releaseOut.Bytes(), &released); err != nil || released.Writer != nil {
		t.Fatalf("force release result writer=%+v decode=%v output=%s", released.Writer, err, releaseOut.String())
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
