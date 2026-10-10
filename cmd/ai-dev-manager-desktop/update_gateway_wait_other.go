//go:build !windows

package main

func waitForGatewayProcessExit(int) error {
	return nil // The automatic installer path is Windows-only.
}
