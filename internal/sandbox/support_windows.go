//go:build windows

package sandbox

import "golang.org/x/sys/windows"

func PlatformSupport() Support {
	major, _, build := windows.RtlGetNtVersionNumbers()
	supported := major >= 10
	notes := []string{
		"strong file read/write isolation requires a dedicated local sandbox identity",
		"process-tree cleanup uses a Windows Job Object",
		"restricted tokens and low integrity are defense-in-depth, not the confidentiality boundary",
	}
	if !supported {
		notes = append(notes, "Windows 10 or newer is required")
	}
	_ = build
	return Support{
		Platform:                    "windows",
		Supported:                   supported,
		EnvironmentSanitization:     true,
		ProcessTreeBoundary:         supported,
		DedicatedIdentitySupported:  supported,
		StrongFileBoundarySupported: supported,
		RequiresAdminSetup:          true,
		Notes:                       notes,
	}
}
