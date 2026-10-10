//go:build windows

package main

import (
	"errors"
	"fmt"

	"golang.org/x/sys/windows"
)

// waitForGatewayProcessExit waits only for the recognized Gateway PID reported
// before its controlled shutdown. It never terminates or opens unrelated PIDs.
func waitForGatewayProcessExit(pid int) error {
	if pid <= 0 {
		return fmt.Errorf("Gateway 未提供可验证的进程 PID")
	}
	handle, err := windows.OpenProcess(windows.SYNCHRONIZE, false, uint32(pid))
	if errors.Is(err, windows.ERROR_INVALID_PARAMETER) {
		return nil // Process already exited.
	}
	if err != nil {
		return err
	}
	defer windows.CloseHandle(handle)
	wait, err := windows.WaitForSingleObject(handle, 5000)
	if err != nil {
		return err
	}
	if wait != windows.WAIT_OBJECT_0 {
		return fmt.Errorf("本地 Gateway 进程 %d 在 5 秒内未退出", pid)
	}
	return nil
}
