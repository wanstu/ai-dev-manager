package desktop

import (
	"fmt"
	"strings"

	"ai-dev-manager-v2/internal/adminmcp"
	"ai-dev-manager-v2/internal/app"
	"ai-dev-manager-v2/internal/gateway"
)

// ConfigureGatewayAdminAPIKey rotates the /admin/mcp key and updates the
// active Desktop profile in the same operation. The old connected client is
// used only for the server-side rotation request; subsequent calls use a fresh
// client carrying the new Admin key.
func (a *Adapter) ConfigureGatewayAdminAPIKey(apiKey string) (app.GatewayAccessStatus, error) {
	if err := a.ready(); err != nil {
		return app.GatewayAccessStatus{}, err
	}
	apiKey = strings.TrimSpace(apiKey)
	if len(apiKey) < 16 {
		return app.GatewayAccessStatus{}, fmt.Errorf("ADM Admin API key must be at least 16 characters")
	}

	connectionProfilesMu.Lock()
	path, err := a.connectionProfilesPath()
	if err != nil {
		connectionProfilesMu.Unlock()
		return app.GatewayAccessStatus{}, err
	}
	state, err := readConnectionProfiles(path)
	if err != nil {
		connectionProfilesMu.Unlock()
		return app.GatewayAccessStatus{}, err
	}
	index := -1
	for i := range state.Profiles {
		if state.Profiles[i].ID == state.ActiveID {
			index = i
			break
		}
	}
	if index < 0 {
		connectionProfilesMu.Unlock()
		return app.GatewayAccessStatus{}, fmt.Errorf("active Desktop connection profile is required")
	}
	profile := state.Profiles[index]
	oldKey := profile.APIKey
	connectionProfilesMu.Unlock()

	status, err := a.management.GatewayAdminAPIKeySet(apiKey)
	if err != nil {
		return app.GatewayAccessStatus{}, err
	}

	connectionProfilesMu.Lock()
	state, readErr := readConnectionProfiles(path)
	if readErr == nil {
		index = -1
		for i := range state.Profiles {
			if state.Profiles[i].ID == state.ActiveID {
				index = i
				break
			}
		}
		if index < 0 {
			readErr = fmt.Errorf("active Desktop connection profile disappeared during API key rotation")
		} else {
			state.Profiles[index].APIKey = apiKey
			readErr = writeConnectionProfiles(path, state)
			profile = state.Profiles[index]
		}
	}
	connectionProfilesMu.Unlock()
	if readErr != nil {
		// Server already accepted the new key. Use a temporary client carrying
		// that key to restore the previous server-side state when profile
		// persistence fails, avoiding a silent Desktop lockout.
		target, targetErr := gateway.ResolveHTTPTarget(profile.BaseURL)
		if targetErr == nil {
			rollback := adminmcp.NewWithAPIKey(target.AdminMCPURL, apiKey)
			if strings.TrimSpace(oldKey) == "" {
				_, _ = rollback.GatewayAdminAPIKeyClear()
			} else {
				_, _ = rollback.GatewayAdminAPIKeySet(oldKey)
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
