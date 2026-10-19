//go:build windows

package sandbox

import "golang.org/x/sys/windows"

func PlatformSupport() Support {
	major, _, _ := windows.RtlGetNtVersionNumbers()
	supported := major >= 10
	notes := []string{
		"Windows 10 backend uses classic AppContainer isolation with explicit filesystem ACL grants",
		"sanitized environment redirects profile, temp, and build caches away from the interactive user profile",
		"process trees are contained in a Windows Job Object with kill-on-close semantics",
		"Windows 11 experimental CreateProcessInSandbox may become an optional backend when available, but ADM does not depend on it",
	}
	if !supported {
		notes = append(notes, "Windows 10 or newer is required")
	}
	return Support{
		Platform:                    "windows",
		Supported:                   supported,
		EnvironmentSanitization:     true,
		ProcessTreeBoundary:         supported,
		DedicatedIdentitySupported:  false,
		StrongFileBoundarySupported: supported && WindowsAppContainerAvailable(),
		RequiresAdminSetup:          false,
		Notes:                       notes,
	}
}
