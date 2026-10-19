//go:build windows

package sandbox

import "golang.org/x/sys/windows"

func PlatformSupport() Support {
	major, _, _ := windows.RtlGetNtVersionNumbers()
	supported := major >= 10
	notes := []string{
		"Windows 10 backend uses classic AppContainer isolation with explicit filesystem ACL grants",
		"sanitized environment redirects profile, temp, and build caches away from the interactive user profile",
		"process-tree cleanup will use a Windows Job Object before Verifier integration is enabled",
		"Windows 11 experimental CreateProcessInSandbox may become an optional backend when available, but ADM does not depend on it",
	}
	if !supported {
		notes = append(notes, "Windows 10 or newer is required")
	}
	return Support{
		Platform:                    "windows",
		Supported:                   supported,
		EnvironmentSanitization:     true,
		ProcessTreeBoundary:         false,
		DedicatedIdentitySupported:  false,
		StrongFileBoundarySupported: supported && WindowsAppContainerAvailable(),
		RequiresAdminSetup:          false,
		Notes:                       notes,
	}
}
