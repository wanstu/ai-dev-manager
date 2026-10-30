//go:build windows

package desktop

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"ai-dev-manager-v2/internal/app"
	"ai-dev-manager-v2/internal/sandbox"
	"ai-dev-manager-v2/internal/store"

	"golang.org/x/sys/windows"
)

func localSandboxEnvironment(environmentID string) (string, string, string, error) {
	environmentID = strings.TrimSpace(environmentID)
	if environmentID == "" {
		return "", "", "", fmt.Errorf("environment id is required")
	}
	statePath, err := store.DefaultPath()
	if err != nil {
		return "", "", "", err
	}
	service := app.New(statePath)
	environment, err := service.Environments.Get(environmentID)
	if err != nil {
		return "", "", "", err
	}
	workspace, err := service.Workspaces.Get(environment.WorkspaceID)
	if err != nil {
		return "", "", "", err
	}
	return environment.ID, workspace.Path, environment.Root, nil
}

func sandboxEnvironmentPreparationStatus(environmentID string) (sandbox.EnvironmentAccessStatus, error) {
	id, workspaceRoot, root, err := localSandboxEnvironment(environmentID)
	if err != nil {
		return sandbox.EnvironmentAccessStatus{}, err
	}
	return sandbox.InspectEnvironmentWorkspaceAccess(id, workspaceRoot, root), nil
}

func enableSandboxEnvironmentPreparation(environmentID string) (sandbox.EnvironmentAccessStatus, error) {
	return enableSandboxEnvironmentPreparationContext(context.Background(), environmentID)
}

func enableSandboxEnvironmentPreparationContext(ctx context.Context, environmentID string) (sandbox.EnvironmentAccessStatus, error) {
	status, err := sandboxEnvironmentPreparationStatus(environmentID)
	if err != nil {
		return status, err
	}
	if status.Prepared {
		return status, nil
	}
	id, workspaceRoot, root, resolveErr := localSandboxEnvironment(environmentID)
	if resolveErr != nil {
		return status, resolveErr
	}
	if status.CanAutoPrepare {
		return sandbox.PrepareEnvironmentWorkspaceAccess(id, workspaceRoot, root)
	}
	profileName, profileErr := sandbox.StableAppContainerProfileName(id)
	if profileErr != nil {
		return status, profileErr
	}
	profile, _, profileErr := sandbox.EnsureWindowsAppContainerProfile(profileName)
	if profileErr != nil {
		return status, fmt.Errorf("prepare Environment AppContainer profile before ACL broker: %w", profileErr)
	}
	if _, brokerErr := requestSandboxACLBroker(ctx, sandboxACLBrokerRequest{
		Operation:       sandboxACLOpPrepare,
		EnvironmentID:   id,
		ProfileSID:      profile.SID,
		WorkspaceRoot:   workspaceRoot,
		EnvironmentRoot: root,
	}); brokerErr == nil {
		current := sandbox.InspectEnvironmentWorkspaceAccess(id, workspaceRoot, root)
		if !current.Prepared {
			return current, fmt.Errorf("Sandbox ACL broker completed without a verified Environment grant")
		}
		return current, nil
	}

	executable, err := os.Executable()
	if err != nil {
		return status, fmt.Errorf("resolve Desktop executable: %w", err)
	}
	verb, err := windows.UTF16PtrFromString("runas")
	if err != nil {
		return status, err
	}
	file, err := windows.UTF16PtrFromString(executable)
	if err != nil {
		return status, err
	}
	// Only the stable Environment ID crosses the elevation boundary. The
	// elevated child resolves the root from ADM state and never accepts an
	// arbitrary host path from the caller.
	args, err := windows.UTF16PtrFromString("--sandbox-environment-prep " + environmentID)
	if err != nil {
		return status, err
	}
	if err := windows.ShellExecute(0, verb, file, args, nil, 0); err != nil {
		return status, fmt.Errorf("request administrator approval for Environment sandbox ACL: %w", err)
	}

	// Security products can replace or intercept the visible UAC flow. Do not
	// trust ShellExecute success; only the resulting ACL is authoritative.
	deadline := time.NewTimer(90 * time.Second)
	defer deadline.Stop()
	ticker := time.NewTicker(300 * time.Millisecond)
	defer ticker.Stop()
	for {
		current, statusErr := sandboxEnvironmentPreparationStatus(environmentID)
		if statusErr == nil && current.Prepared {
			return current, nil
		}
		select {
		case <-ctx.Done():
			current, _ := sandboxEnvironmentPreparationStatus(environmentID)
			return current, ctx.Err()
		case <-deadline.C:
			current, _ := sandboxEnvironmentPreparationStatus(environmentID)
			return current, fmt.Errorf("administrator Environment sandbox preparation did not produce a verifiable ACL")
		case <-ticker.C:
		}
	}
}

func cleanupSandboxEnvironmentPreparation(environmentID string) (sandbox.EnvironmentAccessStatus, error) {
	return cleanupSandboxEnvironmentPreparationContext(context.Background(), environmentID)
}

func cleanupSandboxEnvironmentPreparationContext(ctx context.Context, environmentID string) (sandbox.EnvironmentAccessStatus, error) {
	id, workspaceRoot, root, err := localSandboxEnvironment(environmentID)
	if err != nil {
		return sandbox.EnvironmentAccessStatus{}, err
	}
	status, cleanupErr := sandbox.CleanupEnvironmentWorkspaceAccess(id, workspaceRoot, root)
	if cleanupErr == nil {
		return status, nil
	}
	if !status.RequiresAdmin {
		return status, cleanupErr
	}
	profileSID := strings.TrimSpace(status.ProfileSID)
	if profileSID == "" {
		profileName := strings.TrimSpace(status.ProfileName)
		if profileName == "" {
			var nameErr error
			profileName, nameErr = sandbox.StableAppContainerProfileName(id)
			if nameErr != nil {
				return status, nameErr
			}
		}
		profile, deriveErr := sandbox.DeriveWindowsAppContainerProfile(profileName)
		if deriveErr != nil {
			return status, fmt.Errorf("derive Environment AppContainer profile before ACL broker cleanup: %w", deriveErr)
		}
		profileSID = profile.SID
	}
	if _, brokerErr := requestSandboxACLBroker(ctx, sandboxACLBrokerRequest{
		Operation:       sandboxACLOpCleanup,
		EnvironmentID:   id,
		ProfileSID:      profileSID,
		WorkspaceRoot:   workspaceRoot,
		EnvironmentRoot: root,
	}); brokerErr == nil {
		current, localCleanupErr := sandbox.CleanupEnvironmentWorkspaceAccess(id, workspaceRoot, root)
		if localCleanupErr != nil {
			return current, fmt.Errorf("finalize Environment profile cleanup after ACL broker: %w", localCleanupErr)
		}
		if current.ProfilePresent || current.HasAnyGrant {
			return current, fmt.Errorf("Sandbox ACL broker cleanup completed without removing all persistent grants")
		}
		return current, nil
	}

	executable, err := os.Executable()
	if err != nil {
		return status, fmt.Errorf("resolve Desktop executable: %w", err)
	}
	verb, err := windows.UTF16PtrFromString("runas")
	if err != nil {
		return status, err
	}
	file, err := windows.UTF16PtrFromString(executable)
	if err != nil {
		return status, err
	}
	args, err := windows.UTF16PtrFromString("--sandbox-environment-cleanup " + environmentID)
	if err != nil {
		return status, err
	}
	if err := windows.ShellExecute(0, verb, file, args, nil, 0); err != nil {
		return status, fmt.Errorf("request administrator approval for Environment sandbox cleanup: %w", err)
	}

	// As with preparation, endpoint-security software may own the visible UAC
	// experience. Success is defined only by the profile/grants disappearing.
	deadline := time.NewTimer(90 * time.Second)
	defer deadline.Stop()
	ticker := time.NewTicker(300 * time.Millisecond)
	defer ticker.Stop()
	for {
		current, statusErr := sandboxEnvironmentPreparationStatus(environmentID)
		if statusErr == nil && !current.ProfilePresent && !current.HasAnyGrant {
			return current, nil
		}
		select {
		case <-ctx.Done():
			current, _ := sandboxEnvironmentPreparationStatus(environmentID)
			return current, ctx.Err()
		case <-deadline.C:
			current, _ := sandboxEnvironmentPreparationStatus(environmentID)
			return current, fmt.Errorf("administrator Environment sandbox cleanup did not produce a verifiable result")
		case <-ticker.C:
		}
	}
}

// RunSandboxEnvironmentPreparation is the hidden Desktop entry point used by
// both ordinary-user broker requests and the elevated UAC fallback. Ordinary
// callers first use the protected SYSTEM ACL broker; an elevated fallback
// performs the narrow local ACL operation directly.
func RunSandboxEnvironmentPreparation(environmentID string) error {
	if !windows.GetCurrentProcessToken().IsElevated() {
		return RequestSandboxEnvironmentPreparation(context.Background(), environmentID)
	}
	id, workspaceRoot, root, err := localSandboxEnvironment(environmentID)
	if err != nil {
		return err
	}
	status, err := sandbox.PrepareEnvironmentWorkspaceAccess(id, workspaceRoot, root)
	if err != nil {
		return err
	}
	if !status.Prepared {
		return fmt.Errorf("Environment sandbox ACL preparation completed without a verified grant")
	}
	return nil
}

func RunSandboxEnvironmentCleanup(environmentID string) error {
	if !windows.GetCurrentProcessToken().IsElevated() {
		return RequestSandboxEnvironmentCleanup(context.Background(), environmentID)
	}
	id, workspaceRoot, root, err := localSandboxEnvironment(environmentID)
	if err != nil {
		return err
	}
	status, err := sandbox.CleanupEnvironmentWorkspaceAccess(id, workspaceRoot, root)
	if err != nil {
		return err
	}
	if status.ProfilePresent || status.HasAnyGrant {
		return fmt.Errorf("Environment sandbox cleanup completed without removing all persistent grants")
	}
	return nil
}
