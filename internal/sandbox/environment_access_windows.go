//go:build windows

package sandbox

import (
	"errors"
	"fmt"

	"golang.org/x/sys/windows"
)

func inspectEnvironmentAccessPlatform(status EnvironmentAccessStatus) EnvironmentAccessStatus {
	profile, deriveErr := DeriveWindowsAppContainerProfile(status.ProfileName)
	if deriveErr == nil {
		status.ProfileSID = profile.SID
		status.ProfilePresent = true

		rootGranted, rootErr := WindowsAppContainerPathAccessGranted(status.Root, profile.SID, PathAccessReadWrite)
		if rootErr != nil {
			status.Message = rootErr.Error()
			return status
		}
		ancestorStatus, ancestorErr := InspectWindowsAppContainerAncestorAccess(status.Root, profile.SID)
		if ancestorErr != nil {
			status.Message = ancestorErr.Error()
			return status
		}
		status.HasAnyGrant = rootGranted || ancestorStatus.HasAnyGrant
		if rootGranted && ancestorStatus.Prepared {
			status.Prepared = true
			status.CanAutoPrepare = true
			status.Message = "Environment root and ancestor traverse ACLs already grant the stable AppContainer SID the required access"
			return status
		}

		rootWritable := true
		if !rootGranted {
			var err error
			rootWritable, err = WindowsPathDACLWritable(status.Root)
			if err != nil {
				status.Message = err.Error()
				return status
			}
		}
		status.CanAutoPrepare = rootWritable && ancestorStatus.CanAutoPrepare
		status.RequiresAdmin = !status.CanAutoPrepare
		if status.RequiresAdmin {
			status.Message = "Environment root or one of its parent directories requires one-time administrator ACL preparation"
		} else {
			status.Message = "Gateway can prepare the Environment root and parent traversal ACLs without administrator elevation"
		}
		return status
	}

	rootWritable, rootErr := WindowsPathDACLWritable(status.Root)
	if rootErr != nil {
		status.Message = fmt.Sprintf("derive stable AppContainer SID: %v; inspect Environment root DACL: %v", deriveErr, rootErr)
		return status
	}
	ancestorStatus, ancestorErr := InspectWindowsAncestorDACLWritability(status.Root)
	if ancestorErr != nil {
		status.Message = fmt.Sprintf("derive stable AppContainer SID: %v; inspect Environment ancestor DACLs: %v", deriveErr, ancestorErr)
		return status
	}
	status.CanAutoPrepare = rootWritable && ancestorStatus.CanAutoPrepare
	status.RequiresAdmin = !status.CanAutoPrepare
	if status.RequiresAdmin {
		status.Message = "Environment sandbox profile is not initialized and the root or one of its parent directories requires one-time administrator ACL preparation"
	} else {
		status.Message = "Environment sandbox profile is not initialized; Gateway can create it and prepare the required ACLs automatically"
	}
	return status
}

func cleanupEnvironmentAccessPlatform(status EnvironmentAccessStatus) (EnvironmentAccessStatus, error) {
	if !status.ProfilePresent {
		status.Prepared = false
		status.HasAnyGrant = false
		status.Message = "Environment has no persistent AppContainer profile or ACL grant"
		return status, nil
	}

	sid := status.ProfileSID
	rootGranted, err := WindowsAppContainerPathAccessGranted(status.Root, sid, PathAccessReadWrite)
	if err != nil {
		return status, err
	}
	if rootGranted {
		if err := RevokeWindowsAppContainerPath(status.Root, sid); err != nil {
			if errors.Is(err, windows.ERROR_ACCESS_DENIED) {
				status.RequiresAdmin = true
			}
			return status, fmt.Errorf("revoke Environment AppContainer root ACL: %w", err)
		}
	}

	ancestors, err := windowsAncestorPaths(status.Root)
	if err != nil {
		return status, err
	}
	for _, ancestor := range ancestors {
		granted, grantErr := WindowsAppContainerPathAccessGranted(ancestor, sid, PathAccessTraverse)
		if grantErr != nil {
			return status, grantErr
		}
		if !granted {
			continue
		}
		if revokeErr := RevokeWindowsAppContainerPath(ancestor, sid); revokeErr != nil {
			if errors.Is(revokeErr, windows.ERROR_ACCESS_DENIED) {
				status.RequiresAdmin = true
			}
			return status, fmt.Errorf("revoke Environment AppContainer ancestor ACL %s: %w", ancestor, revokeErr)
		}
	}

	rootGranted, err = WindowsAppContainerPathAccessGranted(status.Root, sid, PathAccessReadWrite)
	if err != nil {
		return status, err
	}
	ancestorStatus, err := InspectWindowsAppContainerAncestorAccess(status.Root, sid)
	if err != nil {
		return status, err
	}
	status.HasAnyGrant = rootGranted || ancestorStatus.HasAnyGrant
	if status.HasAnyGrant {
		return status, fmt.Errorf("Environment sandbox ACL cleanup completed but grants are still present")
	}
	if err := DeleteWindowsAppContainerProfile(status.ProfileName); err != nil {
		return status, fmt.Errorf("delete Environment AppContainer profile: %w", err)
	}

	status.ProfilePresent = false
	status.ProfileSID = ""
	status.Prepared = false
	status.CanAutoPrepare = false
	status.RequiresAdmin = false
	status.Message = "Environment AppContainer ACLs and profile were removed"
	return status, nil
}

func prepareEnvironmentAccessPlatform(status EnvironmentAccessStatus) (EnvironmentAccessStatus, error) {
	profile, _, err := EnsureWindowsAppContainerProfile(status.ProfileName)
	if err != nil {
		return status, err
	}
	status.ProfileSID = profile.SID

	ancestorChanges, err := EnsureWindowsAppContainerAncestorAccess(status.Root, profile.SID)
	if err != nil {
		if errors.Is(err, windows.ERROR_ACCESS_DENIED) {
			status.RequiresAdmin = true
		}
		return status, fmt.Errorf("prepare Environment ancestor traversal ACLs: %w", err)
	}
	rootChanged, err := EnsureWindowsAppContainerPath(status.Root, profile.SID, PathAccessReadWrite)
	if err != nil {
		_ = cleanupChangedAncestorAccess(ancestorChanges, profile.SID)
		if errors.Is(err, windows.ERROR_ACCESS_DENIED) {
			status.RequiresAdmin = true
		}
		return status, fmt.Errorf("prepare Environment AppContainer ACL: %w", err)
	}
	_ = rootChanged // Preparation intentionally leaves newly-created grants in place.

	rootGranted, err := WindowsAppContainerPathAccessGranted(status.Root, profile.SID, PathAccessReadWrite)
	if err != nil {
		return status, err
	}
	ancestorStatus, err := InspectWindowsAppContainerAncestorAccess(status.Root, profile.SID)
	if err != nil {
		return status, err
	}
	if !rootGranted || !ancestorStatus.Prepared {
		return status, fmt.Errorf("Environment ACL preparation completed but verification failed")
	}
	status.Prepared = true
	status.CanAutoPrepare = true
	status.RequiresAdmin = false
	status.Message = "Environment root and parent traversal ACLs are prepared for the stable AppContainer SID"
	return status, nil
}
