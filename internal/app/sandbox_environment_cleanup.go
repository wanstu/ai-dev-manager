package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"ai-dev-manager-v2/internal/model"
	"ai-dev-manager-v2/internal/sandbox"
)

func (s *Service) SetSandboxEnvironmentElevationRequester(requester func(context.Context, string) error) {
	s.sandboxEnvironmentPrepMu.Lock()
	defer s.sandboxEnvironmentPrepMu.Unlock()
	s.sandboxEnvironmentElevationRequest = requester
}

func (s *Service) SetSandboxEnvironmentCleanupRequester(requester func(context.Context, string) error) {
	s.sandboxEnvironmentPrepMu.Lock()
	defer s.sandboxEnvironmentPrepMu.Unlock()
	s.sandboxEnvironmentCleanupRequest = requester
}

func (s *Service) EnsureEnvironmentSandboxAccess(ctx context.Context, environment model.Environment) error {
	if strings.TrimSpace(s.SandboxRoot) == "" {
		return fmt.Errorf("sandbox runtime root is not configured")
	}

	// Serialize one-time profile/ACL preparation so concurrent Sandbox commands
	// for the same host cannot race into duplicate UAC prompts or DACL writes.
	s.sandboxEnvironmentPrepMu.Lock()
	defer s.sandboxEnvironmentPrepMu.Unlock()

	workspace, err := s.Workspaces.Get(environment.WorkspaceID)
	if err != nil {
		return err
	}
	status := sandbox.InspectEnvironmentWorkspaceAccess(environment.ID, workspace.Path, environment.Root)
	if !status.Prepared {
		if status.CanAutoPrepare {
			prepared, prepareErr := sandbox.PrepareEnvironmentWorkspaceAccess(environment.ID, workspace.Path, environment.Root)
			if prepareErr == nil {
				status = prepared
			} else {
				current := sandbox.InspectEnvironmentWorkspaceAccess(environment.ID, workspace.Path, environment.Root)
				if !current.RequiresAdmin || s.sandboxEnvironmentElevationRequest == nil {
					return prepareErr
				}
				status = current
			}
		}

		if !status.Prepared && status.RequiresAdmin {
			if s.sandboxEnvironmentElevationRequest == nil {
				return fmt.Errorf("Environment sandbox ACL requires one-time administrator preparation; use the local Windows Desktop to approve the exact Environment ACL")
			}
			if err := s.sandboxEnvironmentElevationRequest(ctx, environment.ID); err != nil {
				return fmt.Errorf("request one-time administrator Environment sandbox ACL preparation: %w", err)
			}
			status = sandbox.InspectEnvironmentWorkspaceAccess(environment.ID, workspace.Path, environment.Root)
		}
		if !status.Prepared {
			message := strings.TrimSpace(status.Message)
			if message == "" {
				message = "Environment sandbox ACL is not prepared"
			}
			return fmt.Errorf("%s", message)
		}
	}
	if strings.TrimSpace(status.ProfileSID) == "" {
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

func (s *Service) CleanupEnvironmentSandboxAccess(environment model.Environment, workspace model.Workspace) error {
	// Serialize cleanup against one-time preparation/execution setup so a
	// persistent SID grant cannot be recreated while the Environment is being
	// removed.
	s.sandboxEnvironmentPrepMu.Lock()
	defer s.sandboxEnvironmentPrepMu.Unlock()

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

	status := sandbox.InspectEnvironmentWorkspaceAccess(environment.ID, workspace.Path, environment.Root)
	if !status.ProfilePresent && !status.HasAnyGrant {
		return nil
	}

	cleaned, cleanupErr := sandbox.CleanupEnvironmentWorkspaceAccess(environment.ID, workspace.Path, environment.Root)
	if cleanupErr == nil && !cleaned.ProfilePresent && !cleaned.HasAnyGrant {
		return nil
	}

	current := sandbox.InspectEnvironmentWorkspaceAccess(environment.ID, workspace.Path, environment.Root)
	if current.RequiresAdmin && s.sandboxEnvironmentCleanupRequest != nil {
		if err := s.sandboxEnvironmentCleanupRequest(context.Background(), environment.ID); err != nil {
			return fmt.Errorf("request one-time administrator Environment sandbox ACL cleanup: %w", err)
		}
		current = sandbox.InspectEnvironmentWorkspaceAccess(environment.ID, workspace.Path, environment.Root)
		if !current.ProfilePresent && !current.HasAnyGrant {
			return nil
		}
		return fmt.Errorf("Environment sandbox cleanup completed without removing all persistent AppContainer state")
	}
	if cleanupErr != nil {
		return cleanupErr
	}
	return fmt.Errorf("Environment sandbox cleanup completed without removing all persistent AppContainer state")
}
