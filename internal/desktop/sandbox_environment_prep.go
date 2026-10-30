package desktop

import (
	"context"
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

// RequestSandboxEnvironmentPreparation is used by the local Desktop Gateway
// child when Sandbox execution needs a one-time ACL grant. Only the stable
// Environment ID crosses the elevation boundary; elevated code resolves paths
// from local ADM state and never accepts arbitrary host paths from the caller.
func RequestSandboxEnvironmentPreparation(ctx context.Context, environmentID string) error {
	status, err := enableSandboxEnvironmentPreparationContext(ctx, environmentID)
	if err != nil {
		return err
	}
	if !status.Prepared {
		return fmt.Errorf("Environment sandbox ACL preparation completed without a verified grant")
	}
	return nil
}

// RequestSandboxEnvironmentCleanup mirrors preparation for lifecycle teardown.
// Elevated code again receives only the stable Environment ID.
func RequestSandboxEnvironmentCleanup(ctx context.Context, environmentID string) error {
	status, err := cleanupSandboxEnvironmentPreparationContext(ctx, environmentID)
	if err != nil {
		return err
	}
	if status.ProfilePresent || status.HasAnyGrant {
		return fmt.Errorf("Environment sandbox ACL cleanup completed without removing all persistent grants")
	}
	return nil
}
