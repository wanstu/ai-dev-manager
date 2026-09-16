//go:build !windows

package runtime

import (
	"bufio"
	"context"
	"errors"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestConfigureCommandCreatesAndCancelsUnixProcessGroup(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cmd := exec.CommandContext(ctx, "/bin/sh", "-c", "sleep 30 & child=$!; echo $child; wait")
	configureCommand(cmd)
	if cmd.SysProcAttr == nil || !cmd.SysProcAttr.Setpgid {
		t.Fatal("configureCommand must isolate Unix commands in their own process group")
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}

	line, err := bufio.NewReader(stdout).ReadString('\n')
	if err != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		t.Fatalf("read child pid: %v", err)
	}
	childPID, err := strconv.Atoi(strings.TrimSpace(line))
	if err != nil || childPID <= 0 {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		t.Fatalf("invalid child pid %q: %v", line, err)
	}

	cancel()
	if err := cmd.Wait(); err == nil {
		t.Fatal("cancelled process group unexpectedly exited without an error")
	}

	deadline := time.Now().Add(2 * time.Second)
	for {
		err := syscall.Kill(childPID, 0)
		if errors.Is(err, syscall.ESRCH) {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("child process %d survived parent cancellation: kill(0)=%v", childPID, err)
		}
		time.Sleep(20 * time.Millisecond)
	}
}
