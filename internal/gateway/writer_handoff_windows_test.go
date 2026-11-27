//go:build windows

package gateway

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// Reproduce the Windows failure: a finite exec parent exits but a descendant
// keeps the inherited output pipes open. Recovery must not require Gateway exit.
func TestWindowsWriterHandoffRecoversOrphanedExecPipes(t *testing.T) {
	service, id, root := agentRunTestService(t)
	marker := filepath.Join(root, "exec-child.pid")
	owner := newRuntimeOwner(service)
	defer owner.Close()
	ctx := context.Background()
	admin := connectInMemory(t, ctx, newServerForSurface(service, owner, serverSurfaceAdmin))
	defer admin.Close()
	agent := connectInMemory(t, ctx, newServerForSurface(service, owner, serverSurfaceAgent))
	defer agent.Close()
	done := make(chan error, 1)
	go func() {
		_, err := service.Exec(ctx, id, agentRunTestWriter, os.Args[0], []string{"-test.run=^TestHandoffPipeHelper$", "--", "parent", marker}, "", 30000, 4096)
		done <- err
	}()
	var pid int
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if data, err := os.ReadFile(marker); err == nil {
			pid, _ = strconv.Atoi(string(data))
			if pid > 0 {
				break
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	if pid == 0 {
		t.Fatal("child did not start")
	}
	handle, err := syscall.OpenProcess(syscall.SYNCHRONIZE|syscall.PROCESS_TERMINATE, false, uint32(pid))
	if err != nil {
		t.Fatal(err)
	}
	defer syscall.CloseHandle(handle)
	defer syscall.TerminateProcess(handle, 1)
	release := callGatewayTool(t, ctx, admin, "environment_writer_release", map[string]any{"environment_id": id, "force": true})
	if release.IsError {
		t.Fatal(toolText(t, release))
	}
	const next = "recovery-session"
	acquired := callGatewayTool(t, ctx, agent, "environment_writer_acquire", map[string]any{"environment_id": id, "owner": next})
	if acquired.IsError {
		t.Fatal(toolText(t, acquired))
	}
	select {
	case <-done:
	case <-time.After(6 * time.Second):
		t.Fatal("old exec did not return; handoff remains blocked")
	}
	state, err := syscall.WaitForSingleObject(handle, 3000)
	if err != nil || state != syscall.WAIT_OBJECT_0 {
		t.Fatalf("old child still active: %v %v", state, err)
	}
	diagnostics := callGatewayTool(t, ctx, admin, "environment_writer_diagnostics", map[string]any{"environment_id": id})
	if diagnostics.IsError || !strings.Contains(toolText(t, diagnostics), `"handoff_pending":false`) {
		t.Fatal(toolText(t, diagnostics))
	}
	write := callGatewayTool(t, ctx, agent, "write", map[string]any{"environment_id": id, "writer_owner": next, "path": "recovered.txt", "content": "recovered"})
	if write.IsError {
		t.Fatal(toolText(t, write))
	}
	data, err := os.ReadFile(filepath.Join(root, "recovered.txt"))
	if err != nil || string(data) != "recovered" {
		t.Fatalf("write after recovery: %q %v", data, err)
	}
}

func TestHandoffPipeHelper(t *testing.T) {
	var args []string
	for i, a := range os.Args {
		if a == "--" {
			args = os.Args[i+1:]
			break
		}
	}
	if len(args) != 2 {
		return
	}
	if args[0] == "child" {
		if err := os.WriteFile(args[1], []byte(fmt.Sprint(os.Getpid())), 0600); err != nil {
			os.Exit(2)
		}
		time.Sleep(30 * time.Second)
		os.Exit(0)
	}
	if args[0] != "parent" {
		return
	}
	child := exec.Command(os.Args[0], "-test.run=^TestHandoffPipeHelper$", "--", "child", args[1])
	child.Stdout = os.Stdout
	child.Stderr = os.Stderr
	if err := child.Start(); err != nil {
		os.Exit(2)
	}
	os.Exit(0)
}
