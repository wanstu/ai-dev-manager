package app

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"ai-dev-manager-v2/internal/catalog"
)

func TestTrackedMCPCommandTransportRecordsOnlyAfterSuccessfulStart(t *testing.T) {
	service := New(filepath.Join(t.TempDir(), "state.json"))
	cmd := exec.Command(os.Args[0], "-test.run=^TestExecUsageCommandTransportHelper$")
	cmd.Env = append(os.Environ(), "ADM_EXEC_USAGE_HELPER=1")

	transport := service.TrackMCPCommandTransport(cmd, "env_test", os.Args[0], "mcp_stdio")
	connection, err := transport.Connect(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	_ = connection
	_ = cmd.Wait()

	items, err := service.ExecUsages()
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].Count != 1 || items[0].LastSurface != "mcp_stdio" {
		t.Fatalf("tracked MCP usage=%+v", items)
	}
}

func TestTrackedMCPCommandTransportDoesNotRecordFailedStart(t *testing.T) {
	service := New(filepath.Join(t.TempDir(), "state.json"))
	missing := filepath.Join(t.TempDir(), "missing-executable")
	cmd := exec.Command(missing)

	transport := service.TrackMCPCommandTransport(cmd, "env_test", missing, "mcp_stdio")
	if _, err := transport.Connect(context.Background()); err == nil {
		t.Fatal("missing executable unexpectedly started")
	}
	items, err := service.ExecUsages()
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 0 {
		t.Fatalf("failed start was recorded: %+v", items)
	}
}

func TestGlobalMCPCommandPreparationDoesNotRecordUsage(t *testing.T) {
	service := New(filepath.Join(t.TempDir(), "state.json"))
	if err := service.AllowExecutable(os.Args[0]); err != nil {
		t.Fatal(err)
	}
	activation := &MCPActivation{
		MCPID:      "mcp_test",
		Transport:  catalog.MCPTransportStdio,
		Executable: os.Args[0],
	}
	if _, err := service.GlobalMCPCommand(context.Background(), activation); err != nil {
		t.Fatal(err)
	}
	items, err := service.ExecUsages()
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 0 {
		t.Fatalf("command preparation was recorded as execution: %+v", items)
	}
}

func TestExecUsageCommandTransportHelper(t *testing.T) {
	if os.Getenv("ADM_EXEC_USAGE_HELPER") != "1" {
		return
	}
	os.Exit(0)
}
