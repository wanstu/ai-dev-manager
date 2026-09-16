//go:build windows

package runtime

import (
	"context"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"golang.org/x/sys/windows"
)

const createNoWindow = 0x08000000

func configureCommand(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: createNoWindow}
	// CommandContext's default cancellation kills only the direct process.
	// A verifier such as go test owns children that can otherwise retain cwd
	// and output handles after the parent is cancelled.
	if cmd.Cancel == nil {
		return
	}
	killParent := cmd.Cancel
	cmd.Cancel = func() error {
		systemDir, err := windows.GetSystemDirectory()
		if err == nil && cmd.Process != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			// Resolve the OS utility from the system directory, never caller PATH.
			cleanup := exec.CommandContext(ctx, filepath.Join(systemDir, "taskkill.exe"),
				"/PID", strconv.Itoa(cmd.Process.Pid), "/T", "/F")
			cleanup.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: createNoWindow}
			if cleanup.Run() == nil {
				return nil
			}
		}
		// Keep the original direct-process cancellation if tree cleanup is
		// unavailable or races with normal process exit.
		return killParent()
	}
}
