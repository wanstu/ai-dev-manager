//go:build windows

package sandbox

import (
	"errors"
	"fmt"
	"os"

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
		status.HasAnyGrant = rootGranted
		if rootGranted {
			status.Prepared = true
			status.CanAutoPrepare = true
			status.Message = "Environment root already grants the stable AppContainer SID the required access"
			return status
		}

		rootWritable, err := WindowsPathDACLWritable(status.Root)
		if err != nil {
			status.Message = err.Error()
			return status
		}
		status.CanAutoPrepare = rootWritable
		status.RequiresAdmin = !rootWritable
		if status.RequiresAdmin {
			status.Message = "Environment root requires one-time administrator ACL preparation"
		} else {
			status.Message = "Gateway can prepare the Environment root without administrator elevation"
		}
		return status
	}

	rootWritable, rootErr := WindowsPathDACLWritable(status.Root)
	if rootErr != nil {
		status.Message = fmt.Sprintf("derive stable AppContainer SID: %v; inspect Environment root DACL: %v", deriveErr, rootErr)
		return status
	}
	status.CanAutoPrepare = rootWritable
	status.RequiresAdmin = !rootWritable
	if status.RequiresAdmin {
		status.Message = "Environment sandbox profile is not initialized and the root requires one-time administrator ACL preparation"
	} else {
		status.Message = "Environment sandbox profile is not initialized; Gateway can create it and prepare the root ACL automatically"
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

	if _, statErr := os.Stat(status.Root); statErr != nil {
		if !os.IsNotExist(statErr) {
			return status, fmt.Errorf("inspect Environment root before AppContainer cleanup: %w", statErr)
		}
		// Managed worktree cleanup may remove the directory before ADM metadata
		// cleanup runs. The directory ACL disappears with the directory, so only
		// the stable AppContainer profile remains to be removed.
		if err := DeleteWindowsAppContainerProfile(status.ProfileName); err != nil {
			return status, fmt.Errorf("delete Environment AppContainer profile after root removal: %w", err)
		}
		status.ProfilePresent = false
		status.ProfileSID = ""
		status.Prepared = false
		status.HasAnyGrant = false
		status.CanAutoPrepare = false
		status.RequiresAdmin = false
		status.Message = "Environment root was already removed; AppContainer profile was removed"
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

	rootGranted, err = WindowsAppContainerPathAccessGranted(status.Root, sid, PathAccessReadWrite)
	if err != nil {
		return status, err
	}
	status.HasAnyGrant = rootGranted
	if status.HasAnyGrant {
		return status, fmt.Errorf("Environment sandbox ACL cleanup completed but the root grant is still present")
	}
	if err := DeleteWindowsAppContainerProfile(status.ProfileName); err != nil {
		return status, fmt.Errorf("delete Environment AppContainer profile: %w", err)
	}

	status.ProfilePresent = false
	status.ProfileSID = ""
	status.Prepared = false
	status.CanAutoPrepare = false
	status.RequiresAdmin = false
	status.Message = "Environment AppContainer root ACL and profile were removed"
	return status, nil
}

func prepareEnvironmentAccessPlatform(status EnvironmentAccessStatus) (EnvironmentAccessStatus, error) {
	profile, _, err := EnsureWindowsAppContainerProfile(status.ProfileName)
	if err != nil {
		return status, err
	}
	status.ProfileSID = profile.SID

	if _, err := EnsureWindowsAppContainerPath(status.Root, profile.SID, PathAccessReadWrite); err != nil {
		if errors.Is(err, windows.ERROR_ACCESS_DENIED) {
			status.RequiresAdmin = true
		}
		return status, fmt.Errorf("prepare Environment AppContainer root ACL: %w", err)
	}

	rootGranted, err := WindowsAppContainerPathAccessGranted(status.Root, profile.SID, PathAccessReadWrite)
	if err != nil {
		return status, err
	}
	if !rootGranted {
		return status, fmt.Errorf("Environment ACL preparation completed but root verification failed")
	}
	status.ProfilePresent = true
	status.HasAnyGrant = true
	status.Prepared = true
	status.CanAutoPrepare = true
	status.RequiresAdmin = false
	status.Message = "Environment root is prepared for the stable AppContainer SID"
	return status, nil
}

func inspectEnvironmentWorkspaceAccessPlatform(status EnvironmentAccessStatus, workspaceRoot string) EnvironmentAccessStatus {
	// Workspace/root relationship is validated by the caller. AppContainer can
	// open an explicitly granted Environment root without mutating ancestor
	// directory ACLs, so persistent preparation is intentionally root-only.
	return status
}

func prepareEnvironmentWorkspaceAccessPlatform(status EnvironmentAccessStatus, workspaceRoot string) (EnvironmentAccessStatus, error) {
	// Keep persistent authority scoped to the Environment root. Never grant the
	// stable AppContainer SID access to Workspace/ancestor directories.
	return prepareEnvironmentAccessPlatform(status)
}

func cleanupEnvironmentWorkspaceAccessPlatform(status EnvironmentAccessStatus, workspaceRoot string) (EnvironmentAccessStatus, error) {
	return cleanupEnvironmentAccessPlatform(status)
}
