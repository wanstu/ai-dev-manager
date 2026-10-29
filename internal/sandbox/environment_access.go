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

func ResolveEnvironmentTraversalRoot(workspaceRoot, environmentRoot string) (string, error) {
	workspaceRoot = filepath.Clean(strings.TrimSpace(workspaceRoot))
	environmentRoot = filepath.Clean(strings.TrimSpace(environmentRoot))
	if workspaceRoot == "." || !filepath.IsAbs(workspaceRoot) {
		return "", fmt.Errorf("workspace root must be an absolute path")
	}
	if environmentRoot == "." || !filepath.IsAbs(environmentRoot) {
		return "", fmt.Errorf("environment root must be an absolute path")
	}
	if pathWithin(workspaceRoot, environmentRoot) {
		return workspaceRoot, nil
	}

	workspaceVolume := filepath.VolumeName(workspaceRoot)
	environmentVolume := filepath.VolumeName(environmentRoot)
	if !strings.EqualFold(workspaceVolume, environmentVolume) {
		return "", fmt.Errorf("workspace and Environment roots are on different volumes")
	}

	current := workspaceRoot
	for {
		if pathWithin(current, environmentRoot) {
			volumeRoot := filepath.Clean(workspaceVolume + string(filepath.Separator))
			if workspaceVolume == "" {
				volumeRoot = filepath.Clean(string(filepath.Separator))
			}
			if sameTraversalPath(current, volumeRoot) {
				return "", fmt.Errorf("workspace and Environment only share the volume root; refuse broad traversal ACL")
			}
			return current, nil
		}
		next := filepath.Dir(current)
		if next == current {
			break
		}
		current = next
	}
	return "", fmt.Errorf("workspace and Environment do not share a safe traversal root")
}

func sameTraversalPath(left, right string) bool {
	return strings.EqualFold(filepath.Clean(left), filepath.Clean(right))
}

func InspectEnvironmentWorkspaceAccess(environmentID, workspaceRoot, root string) EnvironmentAccessStatus {
	status := InspectEnvironmentAccess(environmentID, root)
	if status.EnvironmentID == "" || status.ProfileName == "" {
		return status
	}
	traversalRoot, err := ResolveEnvironmentTraversalRoot(workspaceRoot, status.Root)
	if err != nil {
		status.Prepared = false
		status.CanAutoPrepare = false
		status.Message = err.Error()
		return status
	}
	return inspectEnvironmentWorkspaceAccessPlatform(status, traversalRoot)
}

func PrepareEnvironmentWorkspaceAccess(environmentID, workspaceRoot, root string) (EnvironmentAccessStatus, error) {
	status := InspectEnvironmentWorkspaceAccess(environmentID, workspaceRoot, root)
	if status.EnvironmentID == "" || status.ProfileName == "" {
		return status, fmt.Errorf("invalid Environment sandbox preparation request: %s", status.Message)
	}
	traversalRoot, err := ResolveEnvironmentTraversalRoot(workspaceRoot, status.Root)
	if err != nil {
		return status, err
	}
	return prepareEnvironmentWorkspaceAccessPlatform(status, traversalRoot)
}

func CleanupEnvironmentWorkspaceAccess(environmentID, workspaceRoot, root string) (EnvironmentAccessStatus, error) {
	status := InspectEnvironmentWorkspaceAccess(environmentID, workspaceRoot, root)
	if status.EnvironmentID == "" || status.ProfileName == "" {
		return status, fmt.Errorf("invalid Environment sandbox cleanup request: %s", status.Message)
	}
	traversalRoot, err := ResolveEnvironmentTraversalRoot(workspaceRoot, status.Root)
	if err != nil {
		return status, err
	}
	return cleanupEnvironmentWorkspaceAccessPlatform(status, traversalRoot)
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
