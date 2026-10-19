//go:build !windows

package sandbox

import "fmt"

func WindowsAppContainerAvailable() bool { return false }

func EnsureWindowsAppContainerProfile(name string) (AppContainerProfile, bool, error) {
	return AppContainerProfile{}, false, fmt.Errorf("Windows AppContainer is unavailable on this platform")
}

func DeleteWindowsAppContainerProfile(name string) error {
	return fmt.Errorf("Windows AppContainer is unavailable on this platform")
}
