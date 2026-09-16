//go:build !windows

package runtime

import (
	"errors"
	"os/exec"
	"syscall"
)

func configureCommand(cmd *exec.Cmd) {
	// Put each ADM-owned command in its own Unix process group so verifier,
	// Run, and Process cancellation can clean up descendants as one bounded
	// tree instead of killing only the direct child.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if cmd.Cancel == nil {
		return
	}
	killParent := cmd.Cancel
	cmd.Cancel = func() error {
		if cmd.Process != nil {
			// A negative PID targets the process group created by Setpgid above.
			// ESRCH means the group already exited, which is a successful cleanup
			// outcome rather than a reason to retry against unrelated processes.
			if err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL); err == nil || errors.Is(err, syscall.ESRCH) {
				return nil
			}
		}
		// Preserve CommandContext's direct-process cancellation as a safe
		// fallback if group cleanup is unavailable or races with process start.
		return killParent()
	}
}
