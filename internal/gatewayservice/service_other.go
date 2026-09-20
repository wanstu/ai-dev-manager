//go:build !linux

package gatewayservice

import "fmt"

func platformSupported() bool { return false }

func unsupported() error {
	return fmt.Errorf("gateway service management is currently implemented only on Linux/systemd")
}

func checkManagementPrivilegesPlatform() error { return unsupported() }

func resolveTargetUser(string) (TargetUser, error)   { return TargetUser{}, unsupported() }
func fixStateOwnership(TargetUser) error             { return unsupported() }
func installPlatform(InstallOptions) (Status, error) { return Status{Supported: false}, unsupported() }
func reconfigurePlatform(InstallOptions) (Status, error) {
	return Status{Supported: false}, unsupported()
}
func startPlatform() (Status, error)   { return Status{Supported: false}, unsupported() }
func restartPlatform() (Status, error) { return Status{Supported: false}, unsupported() }
func stopPlatform() (Status, error)    { return Status{Supported: false}, unsupported() }
func setEnabledPlatform(bool) (Status, error) {
	return Status{Supported: false}, unsupported()
}
func uninstallPlatform() error         { return unsupported() }
func inspectPlatform() (Status, error) { return Status{Supported: false}, unsupported() }
