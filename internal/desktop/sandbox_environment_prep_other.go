//go:build !windows

package desktop

import (
	"fmt"

	"ai-dev-manager-v2/internal/sandbox"
)

func sandboxEnvironmentPreparationStatus(environmentID string) (sandbox.EnvironmentAccessStatus, error) {
	return sandbox.EnvironmentAccessStatus{EnvironmentID: environmentID}, nil
}

func enableSandboxEnvironmentPreparation(environmentID string) (sandbox.EnvironmentAccessStatus, error) {
	return sandbox.EnvironmentAccessStatus{EnvironmentID: environmentID}, fmt.Errorf("Environment sandbox preparation is only available on Windows")
}

func cleanupSandboxEnvironmentPreparation(environmentID string) (sandbox.EnvironmentAccessStatus, error) {
	return sandbox.EnvironmentAccessStatus{EnvironmentID: environmentID}, nil
}

func RunSandboxEnvironmentPreparation(environmentID string) error {
	return fmt.Errorf("Environment sandbox preparation is only available on Windows")
}

func RunSandboxEnvironmentCleanup(environmentID string) error {
	return fmt.Errorf("Environment sandbox cleanup is only available on Windows")
}
