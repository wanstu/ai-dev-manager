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
