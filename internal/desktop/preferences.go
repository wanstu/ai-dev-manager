package desktop

import (
	"errors"
	"fmt"
	"strings"
	"sync"

	"ai-dev-manager-v2/internal/configpath"

	"github.com/wanstu/wails-desktop-kit/jsonstore"
	kittheme "github.com/wanstu/wails-desktop-kit/theme"
)

const (
	defaultDesktopThemeMode = "system"
	defaultDesktopThemePack = "aurora"
)

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
	preference := kittheme.DefaultPreference()
	return persistedDesktopPreferences{
		ThemeMode: string(preference.Mode),
		ThemePack: preference.Pack,
	}
}

func validateDesktopTheme(mode, pack string) (string, string, error) {
	mode = strings.TrimSpace(strings.ToLower(mode))
	if err := kittheme.ValidateMode(kittheme.Mode(mode)); err != nil {
		return "", "", fmt.Errorf("invalid Desktop theme mode %q: %w", mode, err)
	}

	pack = strings.TrimSpace(strings.ToLower(pack))
	if pack != "" {
		if err := kittheme.ValidatePackName(pack); err != nil {
			return "", "", fmt.Errorf("invalid Desktop theme pack %q: %w", pack, err)
		}
	}
	return mode, pack, nil
}

func desktopPreferenceStore(path string) *jsonstore.Store[persistedDesktopPreferences] {
	return jsonstore.New(path, jsonstore.Options[persistedDesktopPreferences]{
		Default: defaultPersistedDesktopPreferences,
		Validate: func(state persistedDesktopPreferences) error {
			_, _, err := validateDesktopTheme(state.ThemeMode, state.ThemePack)
			return err
		},
	})
}

func readPersistedDesktopPreferences(path string) (persistedDesktopPreferences, error) {
	state, err := desktopPreferenceStore(path).Load()
	if err != nil {
		return defaultPersistedDesktopPreferences(), fmt.Errorf("read Desktop preferences: %w", err)
	}
	state.ThemeMode, state.ThemePack, err = validateDesktopTheme(state.ThemeMode, state.ThemePack)
	if err != nil {
		return defaultPersistedDesktopPreferences(), err
	}
	return state, nil
}

func writePersistedDesktopPreferences(path string, state persistedDesktopPreferences) error {
	mode, pack, err := validateDesktopTheme(state.ThemeMode, state.ThemePack)
	if err != nil {
		return err
	}
	state.ThemeMode = mode
	state.ThemePack = pack
	return desktopPreferenceStore(path).Save(state)
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
