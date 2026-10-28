//go:build !windows

package sandbox

import "fmt"

func inspectEnvironmentAccessPlatform(status EnvironmentAccessStatus) EnvironmentAccessStatus {
	status.Message = "Environment AppContainer ACL preparation is unavailable on this platform"
	return status
}

func prepareEnvironmentAccessPlatform(status EnvironmentAccessStatus) (EnvironmentAccessStatus, error) {
	return status, fmt.Errorf("Environment AppContainer ACL preparation is unavailable on this platform")
}

func cleanupEnvironmentAccessPlatform(status EnvironmentAccessStatus) (EnvironmentAccessStatus, error) {
	return status, nil
}
