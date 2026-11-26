//go:build linux

package runtime

import (
	"context"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func waitSupervisorFile(t *testing.T, path string) string {
	t.Helper()
	deadline := time.Now().Add(6 * time.Second)
	for time.Now().Before(deadline) {
		if b, err := os.ReadFile(path); err == nil && len(b) > 0 {
			return string(b)
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("missing helper readiness: ", path)
	return ""
}
func waitSupervisorPortReleased(t *testing.T, addr string) {
	t.Helper()
	deadline := time.Now().Add(6 * time.Second)
	for time.Now().Before(deadline) {
		l, err := net.Listen("tcp", addr)
		if err == nil {
			l.Close()
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatal("managed descendant port retained: ", addr)
}
func TestManagedLinuxSupervisorParentExit(t *testing.T) {
	dir := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestManagedLinuxTreeHelper$")
	cmd.Env = append(os.Environ(), "ADM_LINUX_TREE_DIR="+dir)
	configureCommand(cmd)
	m, err := StartManagedProcess(cmd)
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	waited := false
	defer func() {
		if !waited {
			cancel()
			_ = cmd.Wait()
		}
	}()
	addr := waitSupervisorFile(t, filepath.Join(dir, "port"))
	if err := os.WriteFile(filepath.Join(dir, "exit"), []byte("exit"), 0600); err != nil {
		t.Fatal(err)
	}
	err = cmd.Wait()
	waited = true
	if err != nil {
		t.Fatal("workload exit status not preserved:", err)
	}
	waitSupervisorPortReleased(t, addr)
}
func TestManagedLinuxSupervisorOwnerCrash(t *testing.T) {
	dir := t.TempDir()
	owner := exec.Command(os.Args[0], "-test.run=^TestManagedLinuxOwnerHelper$")
	owner.Env = append(os.Environ(), "ADM_LINUX_OWNER_DIR="+dir, "ADM_LINUX_TREE_DIR="+dir)
	if err := owner.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if owner.ProcessState == nil {
			_ = owner.Process.Kill()
			_ = owner.Wait()
		}
	}()
	addr := waitSupervisorFile(t, filepath.Join(dir, "port"))
	waitSupervisorFile(t, filepath.Join(dir, "owner-ready"))
	if err := owner.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	_ = owner.Wait()
	waitSupervisorPortReleased(t, addr)
}
func TestManagedLinuxOwnerHelper(t *testing.T) {
	dir := os.Getenv("ADM_LINUX_OWNER_DIR")
	if dir == "" {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestManagedLinuxTreeHelper$")
	configureCommand(cmd)
	m, err := StartManagedProcess(cmd)
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	if err := os.WriteFile(filepath.Join(dir, "owner-ready"), []byte("ready"), 0600); err != nil {
		t.Fatal(err)
	}
	_ = cmd.Wait()
}
func TestManagedLinuxTreeHelper(t *testing.T) {
	dir := os.Getenv("ADM_LINUX_TREE_DIR")
	if dir == "" {
		return
	}
	child := exec.Command(os.Args[0], "-test.run=^TestManagedLinuxLeafHelper$")
	child.Stdout = os.Stdout
	child.Stderr = os.Stderr
	if err := child.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = child.Process.Kill(); _ = child.Wait() }()
	deadline := time.Now().Add(12 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(filepath.Join(dir, "exit")); err == nil {
			os.Exit(0)
		}
		time.Sleep(20 * time.Millisecond)
	}
}
func TestManagedLinuxLeafHelper(t *testing.T) {
	dir := os.Getenv("ADM_LINUX_TREE_DIR")
	if dir == "" {
		return
	}
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	if err := os.WriteFile(filepath.Join(dir, "port"), []byte(l.Addr().String()), 0600); err != nil {
		t.Fatal(err)
	}
	time.Sleep(15 * time.Second)
}
func TestManagedLinuxExitCodeAndLaunchError(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestManagedLinuxExitHelper$")
	cmd.Env = append(os.Environ(), "ADM_LINUX_EXIT_CODE=1")
	configureCommand(cmd)
	m, err := StartManagedProcess(cmd)
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	if err := cmd.Wait(); err == nil || cmd.ProcessState.ExitCode() != 23 {
		t.Fatalf("exit code: %v", err)
	}
	bad := exec.CommandContext(ctx, filepath.Join(t.TempDir(), "missing"))
	configureCommand(bad)
	if m, err := StartManagedProcess(bad); err == nil {
		m.Close()
		_ = bad.Wait()
		t.Fatal("missing executable succeeded")
	} else if !strings.Contains(err.Error(), "managed process launch failed") {
		t.Fatal(err)
	}
}
func TestManagedLinuxExitHelper(t *testing.T) {
	if os.Getenv("ADM_LINUX_EXIT_CODE") == "1" {
		os.Exit(23)
	}
}
