package desktop

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"ai-dev-manager-v2/internal/configpath"
	"strings"
	"sync"

	"ai-dev-manager-v2/internal/gateway"
)

type ConnectionProfile struct {
	ID                          string `json:"id"`
	Name                        string `json:"name"`
	BaseURL                     string `json:"base_url"`
	StartServiceOnDesktopLaunch bool   `json:"start_service_on_desktop_launch,omitempty"`
	APIKey                      string `json:"api_key,omitempty"`
	APIKeyConfigured            bool   `json:"api_key_configured,omitempty"`
}
type ConnectionProfiles struct {
	Profiles []ConnectionProfile `json:"profiles"`
	ActiveID string              `json:"active_id"`
}

var connectionProfilesMu sync.Mutex

func (a *Adapter) connectionProfilesPath() (string, error) {
	if a == nil {
		return "", errors.New("desktop adapter is not initialized")
	}
	if a.profilesPath != "" {
		return a.profilesPath, nil
	}
	return configpath.File("desktop-connections.json")
}

func validateConnectionProfile(p ConnectionProfile) (ConnectionProfile, error) {
	p.Name = strings.TrimSpace(p.Name)
	p.APIKey = strings.TrimSpace(p.APIKey)
	if p.Name == "" {
		return p, errors.New("connection name is required")
	}
	target, err := gateway.ResolveHTTPTarget(p.BaseURL)
	if err != nil {
		return p, fmt.Errorf("invalid ADM URL: %w", err)
	}
	p.BaseURL = target.BaseURL
	if p.StartServiceOnDesktopLaunch && !gateway.LocalHTTPLifecycleEligible(p.BaseURL) {
		return p, errors.New("start service on Desktop launch requires a local loopback ADM URL")
	}
	return p, nil
}

func connectionProfilesView(state ConnectionProfiles) ConnectionProfiles {
	view := state
	view.Profiles = append([]ConnectionProfile(nil), state.Profiles...)
	for i := range view.Profiles {
		view.Profiles[i].APIKeyConfigured = strings.TrimSpace(view.Profiles[i].APIKey) != ""
		view.Profiles[i].APIKey = ""
	}
	return view
}

func validateConnectionProfileAccess(profile ConnectionProfile) error {
	if gateway.LocalHTTPLifecycleEligible(profile.BaseURL) {
		return nil
	}
	if strings.TrimSpace(profile.APIKey) == "" {
		return errors.New("remote ADM management connection requires an Admin API key")
	}
	return nil
}

func readConnectionProfiles(path string) (ConnectionProfiles, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return ConnectionProfiles{Profiles: []ConnectionProfile{{ID: "local", Name: "本地 ADM", BaseURL: defaultADMBaseURL()}}, ActiveID: "local"}, nil
	}
	if err != nil {
		return ConnectionProfiles{}, err
	}
	var state ConnectionProfiles
	if err := json.Unmarshal(data, &state); err != nil {
		return state, fmt.Errorf("read connection profiles: %w", err)
	}
	seen := map[string]bool{}
	for i, p := range state.Profiles {
		if p.ID == "" || seen[p.ID] {
			return state, errors.New("invalid or duplicate connection ID")
		}
		seen[p.ID] = true
		normalized, err := validateConnectionProfile(p)
		if err != nil {
			return state, err
		}
		state.Profiles[i] = normalized
	}
	if state.ActiveID != "" && !seen[state.ActiveID] {
		return state, errors.New("active connection does not exist")
	}
	if state.Profiles == nil {
		state.Profiles = []ConnectionProfile{}
	}
	return state, nil
}

func writeConnectionProfiles(path string, state ConnectionProfiles) error {
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".connections-*")
	if err != nil {
		return err
	}
	name := file.Name()
	defer os.Remove(name)
	if _, err = file.Write(append(data, '\n')); err != nil {
		file.Close()
		return err
	}
	if err = file.Sync(); err != nil {
		file.Close()
		return err
	}
	if err = file.Close(); err != nil {
		return err
	}
	return os.Rename(name, path)
}

func (a *Adapter) connectionAPIKey(baseURL, explicit string) string {
	if value := strings.TrimSpace(explicit); value != "" {
		return value
	}
	connectionProfilesMu.Lock()
	defer connectionProfilesMu.Unlock()
	path, err := a.connectionProfilesPath()
	if err != nil {
		return ""
	}
	state, err := readConnectionProfiles(path)
	if err != nil {
		return ""
	}
	for _, profile := range state.Profiles {
		if profile.ID == state.ActiveID && strings.EqualFold(strings.TrimSpace(profile.BaseURL), strings.TrimSpace(baseURL)) {
			return strings.TrimSpace(profile.APIKey)
		}
	}
	return ""
}

func (a *Adapter) GetConnectionProfiles() (ConnectionProfiles, error) {
	connectionProfilesMu.Lock()
	defer connectionProfilesMu.Unlock()
	path, err := a.connectionProfilesPath()
	if err != nil {
		return ConnectionProfiles{}, err
	}
	state, err := readConnectionProfiles(path)
	return connectionProfilesView(state), err
}
func (a *Adapter) SaveConnectionProfile(profile ConnectionProfile) (ConnectionProfiles, error) {
	connectionProfilesMu.Lock()
	defer connectionProfilesMu.Unlock()
	path, err := a.connectionProfilesPath()
	if err != nil {
		return ConnectionProfiles{}, err
	}
	profile, err = validateConnectionProfile(profile)
	if err != nil {
		return ConnectionProfiles{}, err
	}
	state, err := readConnectionProfiles(path)
	if err != nil {
		return state, err
	}
	if profile.ID == "" {
		if err := validateConnectionProfileAccess(profile); err != nil {
			return connectionProfilesView(state), err
		}
		var id [16]byte
		if _, err := rand.Read(id[:]); err != nil {
			return state, err
		}
		profile.ID = hex.EncodeToString(id[:])
		state.Profiles = append(state.Profiles, profile)
		if state.ActiveID == "" {
			state.ActiveID = profile.ID
		}
	} else {
		found := false
		for i := range state.Profiles {
			if state.Profiles[i].ID == profile.ID {
				if strings.TrimSpace(profile.APIKey) == "" {
					profile.APIKey = state.Profiles[i].APIKey
				}
				if err := validateConnectionProfileAccess(profile); err != nil {
					return connectionProfilesView(state), err
				}
				state.Profiles[i] = profile
				found = true
				break
			}
		}
		if !found {
			return state, errors.New("connection does not exist")
		}
	}
	err = writeConnectionProfiles(path, state)
	return connectionProfilesView(state), err
}
func (a *Adapter) SelectConnectionProfile(id string) (ConnectionProfiles, error) {
	connectionProfilesMu.Lock()
	defer connectionProfilesMu.Unlock()
	path, err := a.connectionProfilesPath()
	if err != nil {
		return ConnectionProfiles{}, err
	}
	state, err := readConnectionProfiles(path)
	if err != nil {
		return state, err
	}
	for _, p := range state.Profiles {
		if p.ID == id {
			state.ActiveID = id
			err = writeConnectionProfiles(path, state)
			return connectionProfilesView(state), err
		}
	}
	return state, errors.New("connection does not exist")
}
func (a *Adapter) DeleteConnectionProfile(id string) (ConnectionProfiles, error) {
	connectionProfilesMu.Lock()
	defer connectionProfilesMu.Unlock()
	path, err := a.connectionProfilesPath()
	if err != nil {
		return ConnectionProfiles{}, err
	}
	state, err := readConnectionProfiles(path)
	if err != nil {
		return state, err
	}
	found := false
	for i, p := range state.Profiles {
		if p.ID == id {
			state.Profiles = append(state.Profiles[:i], state.Profiles[i+1:]...)
			found = true
			break
		}
	}
	if !found {
		return state, errors.New("connection does not exist")
	}
	if state.ActiveID == id {
		state.ActiveID = ""
	}
	err = writeConnectionProfiles(path, state)
	return connectionProfilesView(state), err
}
func (a *Adapter) DisconnectADM() {
	if a != nil {
		a.management = nil
		a.runtime = nil
	}
}
