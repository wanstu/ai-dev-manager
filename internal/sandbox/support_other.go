//go:build !windows

package sandbox

import "runtime"

func PlatformSupport() Support {
	return Support{
		Platform:                runtime.GOOS,
		Supported:               false,
		EnvironmentSanitization: true,
		Notes: []string{
			"this milestone implements the Windows strong sandbox first",
		},
	}
}
