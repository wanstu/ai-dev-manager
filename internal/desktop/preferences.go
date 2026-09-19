package desktop

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"ai-dev-manager-v2/internal/configpath"
)

const (
	defaultDesktopThemeMode = "system"
	defaultDesktopThemePack = "aurora"
)

var desktopThemePackPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,63}$`)

type persistedDesktopPreferences struct {
	ThemeMode string `json:"theme_mode"`
	ThemePack string `json:"theme_pack"`
}

var desktopPreferencesMu sync.Mutex

func (a *Adapter) desktopPreferencesPath() (string, error) {
	if a == nil {
		return "", errors.New("desktop adapter is not initialized")
	}
	if strings.TrimSpace(a.preferencesPath) != "" {
		return a.preferencesPath, nil
	}
	return configpath.File("desktop-preferences.json")
}

func defaultPersistedDesktopPreferences() persistedDesktopPreferences {
	return persistedDesktopPreferences{
		ThemeMode: defaultDesktopThemeMode,
		ThemePack: defaultDesktopThemePack,
	}
}

func validateDesktopTheme(mode, pack string) (string, string, error) {
	mode = strings.TrimSpace(strings.ToLower(mode))
	switch mode {
	case "light", "dark", "system":
	default:
		return "", "", fmt.Errorf("invalid Desktop theme mode %q", mode)
	}

	pack = strings.TrimSpace(strings.ToLower(pack))
	if pack != "" && !desktopThemePackPattern.MatchString(pack) {
		return "", "", fmt.Errorf("invalid Desktop theme pack %q", pack)
	}
	return mode, pack, nil
}

func readPersistedDesktopPreferences(path string) (persistedDesktopPreferences, error) {
	state := defaultPersistedDesktopPreferences()
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return state, nil
	}
	if err != nil {
		return state, err
	}
	if err := json.Unmarshal(data, &state); err != nil {
		return state, fmt.Errorf("read Desktop preferences: %w", err)
	}
	mode, pack, err := validateDesktopTheme(state.ThemeMode, state.ThemePack)
	if err != nil {
		return defaultPersistedDesktopPreferences(), err
	}
	state.ThemeMode = mode
	state.ThemePack = pack
	return state, nil
}

func writePersistedDesktopPreferences(path string, state persistedDesktopPreferences) error {
	mode, pack, err := validateDesktopTheme(state.ThemeMode, state.ThemePack)
	if err != nil {
		return err
	}
	state.ThemeMode = mode
	state.ThemePack = pack

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o600)
}

func (a *Adapter) desktopThemePreferences() (persistedDesktopPreferences, error) {
	desktopPreferencesMu.Lock()
	defer desktopPreferencesMu.Unlock()

	path, err := a.desktopPreferencesPath()
	if err != nil {
		return persistedDesktopPreferences{}, err
	}
	return readPersistedDesktopPreferences(path)
}

func (a *Adapter) SetDesktopTheme(mode, pack string) (DesktopPreferences, error) {
	mode, pack, err := validateDesktopTheme(mode, pack)
	if err != nil {
		return DesktopPreferences{}, err
	}

	desktopPreferencesMu.Lock()
	path, err := a.desktopPreferencesPath()
	if err == nil {
		err = writePersistedDesktopPreferences(path, persistedDesktopPreferences{
			ThemeMode: mode,
			ThemePack: pack,
		})
	}
	desktopPreferencesMu.Unlock()
	if err != nil {
		return DesktopPreferences{}, err
	}
	return a.GetDesktopPreferences()
}
