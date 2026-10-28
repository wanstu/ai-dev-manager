//go:build !windows

package sandbox

func EnsureWindowsAppContainerAncestorAccess(path, sidText string) ([]string, error) {
	return nil, nil
}

type WindowsAncestorAccessStatus struct {
	Prepared       bool
	HasAnyGrant    bool
	CanAutoPrepare bool
	RequiresAdmin  bool
}

func InspectWindowsAncestorDACLWritability(path string) (WindowsAncestorAccessStatus, error) {
	return WindowsAncestorAccessStatus{CanAutoPrepare: true}, nil
}

func InspectWindowsAppContainerAncestorAccess(path, sidText string) (WindowsAncestorAccessStatus, error) {
	return WindowsAncestorAccessStatus{Prepared: true, CanAutoPrepare: true}, nil
}

func cleanupChangedAncestorAccess(paths []string, sidText string) error {
	return nil
}
