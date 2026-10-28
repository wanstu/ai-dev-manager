package sandbox

import (
	"fmt"
	"path/filepath"
	"strings"
)

type EnvironmentAccessStatus struct {
	Platform       string `json:"platform"`
	EnvironmentID  string `json:"environment_id"`
	Root           string `json:"root"`
	ProfileName    string `json:"profile_name,omitempty"`
	ProfileSID     string `json:"profile_sid,omitempty"`
	ProfilePresent bool   `json:"profile_present"`
	Prepared       bool   `json:"prepared"`
	HasAnyGrant    bool   `json:"has_any_grant"`
	CanAutoPrepare bool   `json:"can_auto_prepare"`
	RequiresAdmin  bool   `json:"requires_admin"`
	Message        string `json:"message,omitempty"`
}

func InspectEnvironmentAccess(environmentID, root string) EnvironmentAccessStatus {
	environmentID = strings.TrimSpace(environmentID)
	root = filepath.Clean(strings.TrimSpace(root))
	status := EnvironmentAccessStatus{
		Platform:      PlatformSupport().Platform,
		EnvironmentID: environmentID,
		Root:          root,
	}
	if environmentID == "" {
		status.Message = "environment id is required"
		return status
	}
	if root == "." || !filepath.IsAbs(root) {
		status.Message = "environment root must be an absolute path"
		return status
	}
	profileName, err := StableAppContainerProfileName(environmentID)
	if err != nil {
		status.Message = err.Error()
		return status
	}
	status.ProfileName = profileName
	return inspectEnvironmentAccessPlatform(status)
}

func PrepareEnvironmentAccess(environmentID, root string) (EnvironmentAccessStatus, error) {
	status := InspectEnvironmentAccess(environmentID, root)
	if status.EnvironmentID == "" || status.ProfileName == "" {
		return status, fmt.Errorf("invalid Environment sandbox preparation request: %s", status.Message)
	}
	return prepareEnvironmentAccessPlatform(status)
}

func CleanupEnvironmentAccess(environmentID, root string) (EnvironmentAccessStatus, error) {
	status := InspectEnvironmentAccess(environmentID, root)
	if status.EnvironmentID == "" || status.ProfileName == "" {
		return status, fmt.Errorf("invalid Environment sandbox cleanup request: %s", status.Message)
	}
	return cleanupEnvironmentAccessPlatform(status)
}

func InspectEnvironmentWorkspaceAccess(environmentID, workspaceRoot, root string) EnvironmentAccessStatus {
	status := InspectEnvironmentAccess(environmentID, root)
	if status.EnvironmentID == "" || status.ProfileName == "" {
		return status
	}
	workspaceRoot = filepath.Clean(strings.TrimSpace(workspaceRoot))
	if workspaceRoot == "." || !filepath.IsAbs(workspaceRoot) {
		status.Prepared = false
		status.CanAutoPrepare = false
		status.Message = "workspace root must be an absolute path"
		return status
	}
	if !pathWithin(workspaceRoot, status.Root) {
		status.Prepared = false
		status.CanAutoPrepare = false
		status.Message = "environment root must stay inside the workspace root"
		return status
	}
	return inspectEnvironmentWorkspaceAccessPlatform(status, workspaceRoot)
}

func PrepareEnvironmentWorkspaceAccess(environmentID, workspaceRoot, root string) (EnvironmentAccessStatus, error) {
	status := InspectEnvironmentWorkspaceAccess(environmentID, workspaceRoot, root)
	if status.EnvironmentID == "" || status.ProfileName == "" {
		return status, fmt.Errorf("invalid Environment sandbox preparation request: %s", status.Message)
	}
	return prepareEnvironmentWorkspaceAccessPlatform(status, filepath.Clean(workspaceRoot))
}

func CleanupEnvironmentWorkspaceAccess(environmentID, workspaceRoot, root string) (EnvironmentAccessStatus, error) {
	status := InspectEnvironmentWorkspaceAccess(environmentID, workspaceRoot, root)
	if status.EnvironmentID == "" || status.ProfileName == "" {
		return status, fmt.Errorf("invalid Environment sandbox cleanup request: %s", status.Message)
	}
	return cleanupEnvironmentWorkspaceAccessPlatform(status, filepath.Clean(workspaceRoot))
}

func environmentWorkspaceTraversePaths(workspaceRoot, environmentRoot string) ([]string, error) {
	workspaceRoot, err := filepath.Abs(filepath.Clean(workspaceRoot))
	if err != nil {
		return nil, err
	}
	environmentRoot, err = filepath.Abs(filepath.Clean(environmentRoot))
	if err != nil {
		return nil, err
	}
	if !pathWithin(workspaceRoot, environmentRoot) {
		return nil, fmt.Errorf("environment root %s is outside workspace root %s", environmentRoot, workspaceRoot)
	}
	if filepath.Clean(workspaceRoot) == filepath.Clean(environmentRoot) {
		return nil, nil
	}

	current := filepath.Dir(environmentRoot)
	var reversed []string
	for {
		if !pathWithin(workspaceRoot, current) {
			return nil, fmt.Errorf("environment ancestor %s escaped workspace root %s", current, workspaceRoot)
		}
		reversed = append(reversed, current)
		rel, relErr := filepath.Rel(workspaceRoot, current)
		if relErr != nil {
			return nil, relErr
		}
		if rel == "." {
			break
		}
		next := filepath.Dir(current)
		if next == current {
			return nil, fmt.Errorf("could not reach workspace root %s from environment root %s", workspaceRoot, environmentRoot)
		}
		current = next
	}
	for left, right := 0, len(reversed)-1; left < right; left, right = left+1, right-1 {
		reversed[left], reversed[right] = reversed[right], reversed[left]
	}
	return reversed, nil
}
