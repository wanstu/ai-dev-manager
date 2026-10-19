//go:build !windows

package sandbox

import "fmt"

func CurrentWindowsUserSID() (string, error) {
	return "", fmt.Errorf("Windows sandbox identity is unavailable on this platform")
}

func WindowsSandboxAccountName() (string, error) {
	return "", fmt.Errorf("Windows sandbox identity is unavailable on this platform")
}

func WindowsProcessElevated() bool { return false }

func LookupWindowsSandboxAccount(username string) (LocalAccount, error) {
	return LocalAccount{}, fmt.Errorf("Windows sandbox identity is unavailable on this platform")
}

func ProvisionWindowsSandboxAccount(username, password string) (LocalAccount, error) {
	return LocalAccount{}, fmt.Errorf("Windows sandbox identity is unavailable on this platform")
}

func DeleteWindowsSandboxAccount(username string) error {
	return fmt.Errorf("Windows sandbox identity is unavailable on this platform")
}
