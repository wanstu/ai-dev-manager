//go:build !windows

package main

func waitForGatewayProcessExit(int) error {
	return nil // The automatic installer path is Windows-only.
}

func ensureNoOtherDesktopProcesses() error {
	return nil // The automatic installer path is Windows-only.
}

func ensureNoOtherDesktopProcessesExcept(uint32) error {
	return nil // The automatic installer path is Windows-only.
}
