//go:build windows

package desktop

import (
	"fmt"
	"os"
	"strings"
	"time"

	"ai-dev-manager-v2/internal/app"
	"ai-dev-manager-v2/internal/sandbox"
	"ai-dev-manager-v2/internal/store"

	"golang.org/x/sys/windows"
)

func localSandboxEnvironment(environmentID string) (string, string, error) {
	environmentID = strings.TrimSpace(environmentID)
	if environmentID == "" {
		return "", "", fmt.Errorf("environment id is required")
	}
	statePath, err := store.DefaultPath()
	if err != nil {
		return "", "", err
	}
	service := app.New(statePath)
	environment, err := service.Environments.Get(environmentID)
	if err != nil {
		return "", "", err
	}
	return environment.ID, environment.Root, nil
}

func sandboxEnvironmentPreparationStatus(environmentID string) (sandbox.EnvironmentAccessStatus, error) {
	id, root, err := localSandboxEnvironment(environmentID)
	if err != nil {
		return sandbox.EnvironmentAccessStatus{}, err
	}
	return sandbox.InspectEnvironmentAccess(id, root), nil
}

func enableSandboxEnvironmentPreparation(environmentID string) (sandbox.EnvironmentAccessStatus, error) {
	status, err := sandboxEnvironmentPreparationStatus(environmentID)
	if err != nil {
		return status, err
	}
	if status.Prepared {
		return status, nil
	}
	if status.CanAutoPrepare {
		id, root, resolveErr := localSandboxEnvironment(environmentID)
		if resolveErr != nil {
			return status, resolveErr
		}
		return sandbox.PrepareEnvironmentAccess(id, root)
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
	deadline := time.Now().Add(90 * time.Second)
	for time.Now().Before(deadline) {
		current, statusErr := sandboxEnvironmentPreparationStatus(environmentID)
		if statusErr == nil && current.Prepared {
			return current, nil
		}
		time.Sleep(300 * time.Millisecond)
	}
	current, _ := sandboxEnvironmentPreparationStatus(environmentID)
	return current, fmt.Errorf("administrator Environment sandbox preparation did not produce a verifiable ACL")
}

func cleanupSandboxEnvironmentPreparation(environmentID string) (sandbox.EnvironmentAccessStatus, error) {
	id, root, err := localSandboxEnvironment(environmentID)
	if err != nil {
		return sandbox.EnvironmentAccessStatus{}, err
	}
	status, cleanupErr := sandbox.CleanupEnvironmentAccess(id, root)
	if cleanupErr == nil {
		return status, nil
	}
	if !status.RequiresAdmin {
		return status, cleanupErr
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
	deadline := time.Now().Add(90 * time.Second)
	for time.Now().Before(deadline) {
		current, statusErr := sandboxEnvironmentPreparationStatus(environmentID)
		if statusErr == nil && !current.ProfilePresent && !current.HasAnyGrant {
			return current, nil
		}
		time.Sleep(300 * time.Millisecond)
	}
	current, _ := sandboxEnvironmentPreparationStatus(environmentID)
	return current, fmt.Errorf("administrator Environment sandbox cleanup did not produce a verifiable result")
}

// RunSandboxEnvironmentPreparation is the elevated hidden Desktop mode. It
// accepts only an Environment ID and resolves the root from local ADM state.
func RunSandboxEnvironmentPreparation(environmentID string) error {
	if !windows.GetCurrentProcessToken().IsElevated() {
		return fmt.Errorf("Environment sandbox preparation requires administrator elevation")
	}
	id, root, err := localSandboxEnvironment(environmentID)
	if err != nil {
		return err
	}
	status, err := sandbox.PrepareEnvironmentAccess(id, root)
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
		return fmt.Errorf("Environment sandbox cleanup requires administrator elevation")
	}
	id, root, err := localSandboxEnvironment(environmentID)
	if err != nil {
		return err
	}
	status, err := sandbox.CleanupEnvironmentAccess(id, root)
	if err != nil {
		return err
	}
	if status.ProfilePresent || status.HasAnyGrant {
		return fmt.Errorf("Environment sandbox cleanup completed without removing all persistent grants")
	}
	return nil
}
