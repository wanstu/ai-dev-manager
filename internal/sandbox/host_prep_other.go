//go:build !windows

package sandbox

import (
	"fmt"
	"runtime"
)

func InspectHostPreparation() HostPreparationReport {
	return HostPreparationReport{
		Platform:           runtime.GOOS,
		NullDevicePrepared: false,
		RequiresAdminSetup: false,
		Message:            "Windows AppContainer host preparation is not applicable",
	}
}

func PrepareNullDeviceHost() error {
	return fmt.Errorf("Windows null device host preparation is unavailable on %s", runtime.GOOS)
}
