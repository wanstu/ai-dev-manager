package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

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
