package desktop

import (
	"fmt"
	"strings"

	"ai-dev-manager-v2/internal/gateway"
	"ai-dev-manager-v2/internal/sandbox"
)

func (a *Adapter) requireLocalSandboxEnvironmentTarget() error {
	if err := a.ready(); err != nil {
		return err
	}
	baseURL := strings.TrimSpace(a.connectedBaseURL)
	if baseURL == "" || !gateway.LocalHTTPLifecycleEligible(baseURL) {
		return fmt.Errorf("Environment strong-sandbox ACL preparation must run from a Desktop connected to the target host's local ADM")
	}
	return nil
}

func (a *Adapter) GetSandboxEnvironmentPreparationStatus(environmentID string) (sandbox.EnvironmentAccessStatus, error) {
	if err := a.requireLocalSandboxEnvironmentTarget(); err != nil {
		return sandbox.EnvironmentAccessStatus{EnvironmentID: strings.TrimSpace(environmentID)}, err
	}
	return sandboxEnvironmentPreparationStatus(environmentID)
}

func (a *Adapter) EnableSandboxEnvironmentPreparation(environmentID string) (sandbox.EnvironmentAccessStatus, error) {
	if err := a.requireLocalSandboxEnvironmentTarget(); err != nil {
		return sandbox.EnvironmentAccessStatus{EnvironmentID: strings.TrimSpace(environmentID)}, err
	}
	return enableSandboxEnvironmentPreparation(environmentID)
}

func (a *Adapter) CleanupSandboxEnvironmentPreparation(environmentID string) (sandbox.EnvironmentAccessStatus, error) {
	if err := a.requireLocalSandboxEnvironmentTarget(); err != nil {
		return sandbox.EnvironmentAccessStatus{EnvironmentID: strings.TrimSpace(environmentID)}, err
	}
	return cleanupSandboxEnvironmentPreparation(environmentID)
}
