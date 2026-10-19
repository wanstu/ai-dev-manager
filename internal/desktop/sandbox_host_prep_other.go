//go:build !windows

package desktop

import (
	"fmt"
	"runtime"
)

func sandboxHostPreparationStatus() SandboxHostPreparationStatus {
	return SandboxHostPreparationStatus{
		Platform:  runtime.GOOS,
		Supported: false,
		Message:   "Windows AppContainer host preparation is not applicable on this platform",
	}
}

func enableSandboxHostPreparation() (SandboxHostPreparationStatus, error) {
	status := sandboxHostPreparationStatus()
	return status, fmt.Errorf("strong Windows sandbox host preparation is unavailable on %s", runtime.GOOS)
}

func RunSandboxHostPreparationInstall() error {
	return fmt.Errorf("Windows sandbox host preparation is unavailable on %s", runtime.GOOS)
}

func RunSandboxHostPreparationBoot() error {
	return fmt.Errorf("Windows sandbox host preparation is unavailable on %s", runtime.GOOS)
}
