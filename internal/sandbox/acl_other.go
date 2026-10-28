//go:build !windows

package sandbox

import "fmt"

type PathAccess string

const (
	PathAccessTraverse  PathAccess = "traverse"
	PathAccessReadOnly  PathAccess = "read_only"
	PathAccessReadWrite PathAccess = "read_write"
)

func WindowsAppContainerPathAccessGranted(path, sidText string, access PathAccess) (bool, error) {
	return false, fmt.Errorf("Windows AppContainer ACLs are unavailable on this platform")
}

func EnsureWindowsAppContainerPath(path, sidText string, access PathAccess) (bool, error) {
	return false, fmt.Errorf("Windows AppContainer ACLs are unavailable on this platform")
}

func GrantWindowsAppContainerPath(path, sidText string, access PathAccess) error {
	return fmt.Errorf("Windows AppContainer ACLs are unavailable on this platform")
}

func WindowsPathDACLWritable(path string) (bool, error) {
	return false, fmt.Errorf("Windows AppContainer ACLs are unavailable on this platform")
}

func RevokeWindowsAppContainerPath(path, sidText string) error {
	return fmt.Errorf("Windows AppContainer ACLs are unavailable on this platform")
}
