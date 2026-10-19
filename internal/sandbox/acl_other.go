//go:build !windows

package sandbox

import "fmt"

type PathAccess string

const (
	PathAccessReadOnly  PathAccess = "read_only"
	PathAccessReadWrite PathAccess = "read_write"
)

func GrantWindowsAppContainerPath(path, sidText string, access PathAccess) error {
	return fmt.Errorf("Windows AppContainer ACLs are unavailable on this platform")
}

func RevokeWindowsAppContainerPath(path, sidText string) error {
	return fmt.Errorf("Windows AppContainer ACLs are unavailable on this platform")
}
