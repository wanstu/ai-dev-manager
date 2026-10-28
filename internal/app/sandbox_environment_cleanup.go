package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"ai-dev-manager-v2/internal/model"
	"ai-dev-manager-v2/internal/sandbox"
)

func (s *Service) CleanupEnvironmentSandboxAccess(environment model.Environment) error {
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

	sandboxRoot := strings.TrimSpace(s.SandboxRoot)
	if sandboxRoot == "" {
		return nil
	}
	root, err := filepath.Abs(filepath.Clean(sandboxRoot))
	if err != nil {
		return fmt.Errorf("resolve sandbox root for Environment cleanup: %w", err)
	}
	environmentRuntimeRoot := filepath.Join(root, environment.ID)
	if filepath.Dir(environmentRuntimeRoot) != root {
		return fmt.Errorf("refuse sandbox cache cleanup outside configured sandbox root")
	}
	if err := os.RemoveAll(environmentRuntimeRoot); err != nil {
		return fmt.Errorf("remove Environment sandbox runtime/cache root: %w", err)
	}
	return nil
}
