//go:build windows

package sandbox

import (
	"errors"
	"fmt"
	"path/filepath"
)

func windowsAncestorPaths(path string) ([]string, error) {
	absolute, err := filepath.Abs(filepath.Clean(path))
	if err != nil {
		return nil, err
	}
	current := filepath.Dir(absolute)
	var reversed []string
	for {
		reversed = append(reversed, current)
		parent := filepath.Dir(current)
		if parent == current {
			break
		}
		current = parent
	}
	result := make([]string, 0, len(reversed))
	for i := len(reversed) - 1; i >= 0; i-- {
		result = append(result, reversed[i])
	}
	return result, nil
}

// EnsureWindowsAppContainerAncestorAccess gives the AppContainer only the
// ability needed to traverse and stat the directory chain leading to path.
// It intentionally does not grant FILE_LIST_DIRECTORY or file read access.
func EnsureWindowsAppContainerAncestorAccess(path, sidText string) ([]string, error) {
	ancestors, err := windowsAncestorPaths(path)
	if err != nil {
		return nil, err
	}
	changed := make([]string, 0, len(ancestors))
	for _, ancestor := range ancestors {
		didChange, grantErr := EnsureWindowsAppContainerPath(ancestor, sidText, PathAccessTraverse)
		if grantErr != nil {
			for i := len(changed) - 1; i >= 0; i-- {
				_ = RevokeWindowsAppContainerPath(changed[i], sidText)
			}
			return nil, fmt.Errorf("grant AppContainer ancestor traverse %s: %w", ancestor, grantErr)
		}
		if didChange {
			changed = append(changed, ancestor)
		}
	}
	return changed, nil
}

type WindowsAncestorAccessStatus struct {
	Prepared       bool
	HasAnyGrant    bool
	CanAutoPrepare bool
	RequiresAdmin  bool
}

func InspectWindowsAncestorDACLWritability(path string) (WindowsAncestorAccessStatus, error) {
	ancestors, err := windowsAncestorPaths(path)
	if err != nil {
		return WindowsAncestorAccessStatus{}, err
	}
	status := WindowsAncestorAccessStatus{CanAutoPrepare: true}
	for _, ancestor := range ancestors {
		writable, writeErr := WindowsPathDACLWritable(ancestor)
		if writeErr != nil {
			return WindowsAncestorAccessStatus{}, writeErr
		}
		if !writable {
			status.CanAutoPrepare = false
			status.RequiresAdmin = true
		}
	}
	return status, nil
}

func InspectWindowsAppContainerAncestorAccess(path, sidText string) (WindowsAncestorAccessStatus, error) {
	ancestors, err := windowsAncestorPaths(path)
	if err != nil {
		return WindowsAncestorAccessStatus{}, err
	}
	status := WindowsAncestorAccessStatus{Prepared: true, CanAutoPrepare: true}
	for _, ancestor := range ancestors {
		granted, grantErr := WindowsAppContainerPathAccessGranted(ancestor, sidText, PathAccessTraverse)
		if grantErr != nil {
			return WindowsAncestorAccessStatus{}, grantErr
		}
		if granted {
			status.HasAnyGrant = true
			continue
		}
		status.Prepared = false
		writable, writeErr := WindowsPathDACLWritable(ancestor)
		if writeErr != nil {
			return WindowsAncestorAccessStatus{}, writeErr
		}
		if !writable {
			status.CanAutoPrepare = false
			status.RequiresAdmin = true
		}
	}
	return status, nil
}

func cleanupChangedAncestorAccess(paths []string, sidText string) error {
	var result error
	for i := len(paths) - 1; i >= 0; i-- {
		if err := RevokeWindowsAppContainerPath(paths[i], sidText); err != nil {
			result = errors.Join(result, err)
		}
	}
	return result
}
