package desktop

import (
	"fmt"
	"strings"

	"ai-dev-manager-v2/internal/adminmcp"
	"ai-dev-manager-v2/internal/app"
	"ai-dev-manager-v2/internal/gateway"
)

type gatewayAdminRotationContext struct {
	Path    string
	Profile ConnectionProfile
	OldKey  string
}

func (a *Adapter) gatewayAdminRotationContext() (gatewayAdminRotationContext, error) {
	connectionProfilesMu.Lock()
	defer connectionProfilesMu.Unlock()

	path, err := a.connectionProfilesPath()
	if err != nil {
		return gatewayAdminRotationContext{}, err
	}
	state, err := readConnectionProfiles(path)
	if err != nil {
		return gatewayAdminRotationContext{}, err
	}
	for i := range state.Profiles {
		if state.Profiles[i].ID == state.ActiveID {
			return gatewayAdminRotationContext{
				Path:    path,
				Profile: state.Profiles[i],
				OldKey:  state.Profiles[i].APIKey,
			}, nil
		}
	}
	return gatewayAdminRotationContext{}, fmt.Errorf("active Desktop connection profile is required")
}

func (a *Adapter) finishGatewayAdminRotation(ctx gatewayAdminRotationContext, apiKey string, status app.GatewayAccessStatus) (app.GatewayAccessStatus, error) {
	connectionProfilesMu.Lock()
	state, readErr := readConnectionProfiles(ctx.Path)
	profile := ctx.Profile
	if readErr == nil {
		index := -1
		for i := range state.Profiles {
			if state.Profiles[i].ID == ctx.Profile.ID && state.ActiveID == ctx.Profile.ID {
				index = i
				break
			}
		}
		if index < 0 {
			readErr = fmt.Errorf("active Desktop connection profile changed during API key rotation")
		} else {
			state.Profiles[index].APIKey = apiKey
			readErr = writeConnectionProfiles(ctx.Path, state)
			profile = state.Profiles[index]
		}
	}
	connectionProfilesMu.Unlock()

	if readErr != nil {
		// The server has already accepted the new key. Authenticate with the
		// one-time new key to restore the prior server-side state so Desktop is
		// not silently locked out when local profile persistence fails.
		target, targetErr := gateway.ResolveHTTPTarget(ctx.Profile.BaseURL)
		if targetErr == nil {
			rollback := adminmcp.NewWithAPIKey(target.AdminMCPURL, apiKey)
			if strings.TrimSpace(ctx.OldKey) == "" {
				_, _ = rollback.GatewayAdminAPIKeyClear()
			} else {
				_, _ = rollback.GatewayAdminAPIKeySet(ctx.OldKey)
			}
		}
		return app.GatewayAccessStatus{}, fmt.Errorf("persist Desktop Admin API key after server rotation: %w", readErr)
	}

	target, err := gateway.ResolveHTTPTarget(profile.BaseURL)
	if err != nil {
		return app.GatewayAccessStatus{}, err
	}
	client := adminmcp.NewWithAPIKey(target.AdminMCPURL, apiKey)
	a.management = client
	a.runtime = client
	return status, nil
}

// ConfigureGatewayAdminAPIKey manually sets the /admin/mcp key and updates the
// active Desktop profile in the same operation. Normal UI rotation uses the
// server-side RotateGatewayAdminAPIKey workflow instead.
func (a *Adapter) ConfigureGatewayAdminAPIKey(apiKey string) (app.GatewayAccessStatus, error) {
	if err := a.ready(); err != nil {
		return app.GatewayAccessStatus{}, err
	}
	apiKey = strings.TrimSpace(apiKey)
	if len(apiKey) < 16 {
		return app.GatewayAccessStatus{}, fmt.Errorf("ADM Admin API key must be at least 16 characters")
	}
	ctx, err := a.gatewayAdminRotationContext()
	if err != nil {
		return app.GatewayAccessStatus{}, err
	}
	status, err := a.management.GatewayAdminAPIKeySet(apiKey)
	if err != nil {
		return app.GatewayAccessStatus{}, err
	}
	return a.finishGatewayAdminRotation(ctx, apiKey, status)
}

func (a *Adapter) RotateGatewayAdminAPIKey() (string, error) {
	if err := a.ready(); err != nil {
		return "", err
	}
	ctx, err := a.gatewayAdminRotationContext()
	if err != nil {
		return "", err
	}
	result, err := a.management.GatewayAdminAPIKeyRotate()
	if err != nil {
		return "", err
	}
	apiKey := strings.TrimSpace(result.AdminAPIKey)
	if apiKey == "" {
		return "", fmt.Errorf("server returned an empty rotated Admin API key")
	}
	if _, err := a.finishGatewayAdminRotation(ctx, apiKey, result.Status); err != nil {
		return "", err
	}
	return apiKey, nil
}

func (a *Adapter) RotateGatewayAgentAPIKey() (string, error) {
	if err := a.ready(); err != nil {
		return "", err
	}
	result, err := a.management.GatewayAgentAPIKeyRotate()
	if err != nil {
		return "", err
	}
	apiKey := strings.TrimSpace(result.AgentAPIKey)
	if apiKey == "" {
		return "", fmt.Errorf("server returned an empty rotated Agent API key")
	}
	return apiKey, nil
}
