package desktop

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"ai-dev-manager-v2/internal/gateway"
)

type ConnectionProfile struct {
	ID                          string `json:"id"`
	Name                        string `json:"name"`
	BaseURL                     string `json:"base_url"`
	StartServiceOnDesktopLaunch bool   `json:"start_service_on_desktop_launch,omitempty"`
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
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "adm", "desktop-connections.json"), nil
}

func validateConnectionProfile(p ConnectionProfile) (ConnectionProfile, error) {
	p.Name = strings.TrimSpace(p.Name)
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

func readConnectionProfiles(path string) (ConnectionProfiles, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return ConnectionProfiles{Profiles: []ConnectionProfile{{ID: "local", Name: "鏈湴 ADM", BaseURL: defaultADMBaseURL()}}, ActiveID: "local"}, nil
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

func (a *Adapter) GetConnectionProfiles() (ConnectionProfiles, error) {
	connectionProfilesMu.Lock()
	defer connectionProfilesMu.Unlock()
	path, err := a.connectionProfilesPath()
	if err != nil {
		return ConnectionProfiles{}, err
	}
	return readConnectionProfiles(path)
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
				state.Profiles[i] = profile
				found = true
				break
			}
		}
		if !found {
			return state, errors.New("connection does not exist")
		}
	}
	return state, writeConnectionProfiles(path, state)
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
			return state, writeConnectionProfiles(path, state)
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
	return state, writeConnectionProfiles(path, state)
}
func (a *Adapter) DisconnectADM() {
	if a != nil {
		a.management = nil
		a.runtime = nil
	}
}
