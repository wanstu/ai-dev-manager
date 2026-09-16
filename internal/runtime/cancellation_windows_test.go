//go:build windows

package runtime

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

func TestWindowsCommandCancellationStopsChild(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	ready := filepath.Join(t.TempDir(), "child-ready")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cmd := exec.CommandContext(ctx, exe, "-test.run=^TestWindowsCancellationHelper$")
	cmd.Env = append(os.Environ(), "ADM_CANCEL_HELPER=parent", "ADM_CANCEL_READY="+ready)
	configureCommand(cmd)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer cmd.Process.Kill()
	var pid int
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		data, readErr := os.ReadFile(ready)
		if readErr == nil {
			pid, err = strconv.Atoi(strings.TrimSpace(string(data)))
			if err == nil {
				break
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	if pid == 0 {
		t.Fatal("child did not become ready")
	}
	handle, err := syscall.OpenProcess(syscall.SYNCHRONIZE|syscall.PROCESS_TERMINATE, false, uint32(pid))
	if err != nil {
		t.Fatal(err)
	}
	defer syscall.CloseHandle(handle)
	defer syscall.TerminateProcess(handle, 1)
	cancel()
	if err := cmd.Wait(); err == nil {
		t.Fatal("cancelled parent unexpectedly passed")
	}
	state, err := syscall.WaitForSingleObject(handle, 3000)
	if err != nil || state != syscall.WAIT_OBJECT_0 {
		t.Fatalf("child survived parent cancellation: wait=%d err=%v", state, err)
	}
}

func TestWindowsCancellationHelper(t *testing.T) {
	role := os.Getenv("ADM_CANCEL_HELPER")
	if role == "" {
		return
	}
	if role == "child" {
		if err := os.WriteFile(os.Getenv("ADM_CANCEL_READY"), []byte(fmt.Sprint(os.Getpid())), 0600); err != nil {
			os.Exit(2)
		}
		time.Sleep(30 * time.Second)
		os.Exit(0)
	}
	exe, err := os.Executable()
	if err != nil {
		os.Exit(2)
	}
	cmd := exec.Command(exe, "-test.run=^TestWindowsCancellationHelper$")
	cmd.Env = append(os.Environ(), "ADM_CANCEL_HELPER=child")
	configureCommand(cmd)
	if err := cmd.Run(); err != nil {
		os.Exit(2)
	}
	os.Exit(0)
}
