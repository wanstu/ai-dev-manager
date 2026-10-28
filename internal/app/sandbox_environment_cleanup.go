package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"ai-dev-manager-v2/internal/model"
	"ai-dev-manager-v2/internal/sandbox"
)

func (s *Service) EnsureEnvironmentSandboxAccess(environment model.Environment) error {
	if strings.TrimSpace(s.SandboxRoot) == "" {
		return fmt.Errorf("sandbox runtime root is not configured")
	}
	status, err := sandbox.PrepareEnvironmentAccess(environment.ID, environment.Root)
	if err != nil {
		return err
	}
	if !status.Prepared || strings.TrimSpace(status.ProfileSID) == "" {
		return fmt.Errorf("Environment sandbox root ACL preparation did not produce a verified stable SID")
	}

	sandboxRoot, err := filepath.Abs(filepath.Clean(s.SandboxRoot))
	if err != nil {
		return fmt.Errorf("resolve sandbox root: %w", err)
	}
	environmentRuntimeRoot := filepath.Join(sandboxRoot, environment.ID)
	if filepath.Dir(environmentRuntimeRoot) != sandboxRoot {
		return fmt.Errorf("refuse Environment sandbox root outside configured sandbox root")
	}
	if err := os.MkdirAll(environmentRuntimeRoot, 0o755); err != nil {
		return fmt.Errorf("create Environment sandbox root: %w", err)
	}
	if _, err := sandbox.EnsureWindowsAppContainerPath(environmentRuntimeRoot, status.ProfileSID, sandbox.PathAccessReadWrite); err != nil {
		return fmt.Errorf("prepare Environment sandbox runtime/cache ACL: %w", err)
	}
	return nil
}

func (s *Service) CleanupEnvironmentSandboxAccess(environment model.Environment) error {
	sandboxRoot := strings.TrimSpace(s.SandboxRoot)
	if sandboxRoot != "" {
		root, err := filepath.Abs(filepath.Clean(sandboxRoot))
		if err != nil {
			return fmt.Errorf("resolve sandbox root for Environment cleanup: %w", err)
		}
		environmentRuntimeRoot := filepath.Join(root, environment.ID)
		if filepath.Dir(environmentRuntimeRoot) != root {
			return fmt.Errorf("refuse sandbox cache cleanup outside configured sandbox root")
		}
		// Remove the ADM-owned inherited ACL subtree before deleting the stable
		// AppContainer profile. If this fails, keep Environment metadata so the
		// cleanup can be retried with the SID still resolvable.
		if err := os.RemoveAll(environmentRuntimeRoot); err != nil {
			return fmt.Errorf("remove Environment sandbox runtime/cache root: %w", err)
		}
	}

	status := sandbox.InspectEnvironmentAccess(environment.ID, environment.Root)
	if status.ProfilePresent || status.HasAnyGrant {
		cleaned, err := sandbox.CleanupEnvironmentAccess(environment.ID, environment.Root)
		if err != nil {
			return err
		}
		if cleaned.ProfilePresent || cleaned.HasAnyGrant {
			return fmt.Errorf("Environment sandbox cleanup did not remove all persistent AppContainer state")
		}
	}
	return nil
}
