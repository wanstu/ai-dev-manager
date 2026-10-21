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
	defaultDesktopThemeMode  = "system"
	defaultDesktopThemePack  = "aurora"
	defaultDesktopThemeStyle = "solid"
)

var defaultDesktopGradientPacks = []string{"aurora", "ocean"}

type persistedDesktopPreferences struct {
	ThemeMode          string   `json:"theme_mode"`
	ThemePack          string   `json:"theme_pack"`
	ThemeStyle         string   `json:"theme_style,omitempty"`
	ThemeGradientPacks []string `json:"theme_gradient_packs,omitempty"`
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
		ThemeMode:          string(preference.Mode),
		ThemePack:          preference.Pack,
		ThemeStyle:         defaultDesktopThemeStyle,
		ThemeGradientPacks: append([]string(nil), defaultDesktopGradientPacks...),
	}
}

func validateDesktopTheme(mode, pack, style string, gradientPacks []string) (string, string, string, []string, error) {
	mode = strings.TrimSpace(strings.ToLower(mode))
	if err := kittheme.ValidateMode(kittheme.Mode(mode)); err != nil {
		return "", "", "", nil, fmt.Errorf("invalid Desktop theme mode %q: %w", mode, err)
	}

	pack = strings.TrimSpace(strings.ToLower(pack))
	if pack != "" {
		if err := kittheme.ValidatePackName(pack); err != nil {
			return "", "", "", nil, fmt.Errorf("invalid Desktop theme pack %q: %w", pack, err)
		}
	}

	style = strings.TrimSpace(strings.ToLower(style))
	if style == "" {
		style = defaultDesktopThemeStyle
	}
	if style != "solid" && style != "gradient" {
		return "", "", "", nil, fmt.Errorf("invalid Desktop theme style %q", style)
	}

	normalizedGradient := make([]string, 0, len(gradientPacks))
	seen := map[string]struct{}{}
	for _, item := range gradientPacks {
		item = strings.TrimSpace(strings.ToLower(item))
		if item == "" {
			continue
		}
		if err := kittheme.ValidatePackName(item); err != nil {
			return "", "", "", nil, fmt.Errorf("invalid Desktop gradient theme pack %q: %w", item, err)
		}
		if _, exists := seen[item]; exists {
			return "", "", "", nil, fmt.Errorf("Desktop gradient theme packs must be distinct")
		}
		seen[item] = struct{}{}
		normalizedGradient = append(normalizedGradient, item)
	}
	if len(normalizedGradient) == 0 {
		normalizedGradient = append([]string(nil), defaultDesktopGradientPacks...)
	}
	if len(normalizedGradient) > 4 {
		return "", "", "", nil, fmt.Errorf("Desktop gradient supports at most 4 theme packs")
	}
	if style == "gradient" && len(normalizedGradient) < 2 {
		return "", "", "", nil, fmt.Errorf("Desktop gradient requires at least 2 distinct theme packs")
	}
	return mode, pack, style, normalizedGradient, nil
}

func desktopPreferenceStore(path string) *jsonstore.Store[persistedDesktopPreferences] {
	return jsonstore.New(path, jsonstore.Options[persistedDesktopPreferences]{
		Default: defaultPersistedDesktopPreferences,
		Validate: func(state persistedDesktopPreferences) error {
			_, _, _, _, err := validateDesktopTheme(state.ThemeMode, state.ThemePack, state.ThemeStyle, state.ThemeGradientPacks)
			return err
		},
	})
}

func readPersistedDesktopPreferences(path string) (persistedDesktopPreferences, error) {
	state, err := desktopPreferenceStore(path).Load()
	if err != nil {
		return defaultPersistedDesktopPreferences(), fmt.Errorf("read Desktop preferences: %w", err)
	}
	state.ThemeMode, state.ThemePack, state.ThemeStyle, state.ThemeGradientPacks, err = validateDesktopTheme(
		state.ThemeMode,
		state.ThemePack,
		state.ThemeStyle,
		state.ThemeGradientPacks,
	)
	if err != nil {
		return defaultPersistedDesktopPreferences(), err
	}
	return state, nil
}

func writePersistedDesktopPreferences(path string, state persistedDesktopPreferences) error {
	mode, pack, style, gradientPacks, err := validateDesktopTheme(
		state.ThemeMode,
		state.ThemePack,
		state.ThemeStyle,
		state.ThemeGradientPacks,
	)
	if err != nil {
		return err
	}
	state.ThemeMode = mode
	state.ThemePack = pack
	state.ThemeStyle = style
	state.ThemeGradientPacks = gradientPacks
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

func (a *Adapter) SetDesktopTheme(mode, pack, style string, gradientPacks []string) (DesktopPreferences, error) {
	mode, pack, style, gradientPacks, err := validateDesktopTheme(mode, pack, style, gradientPacks)
	if err != nil {
		return DesktopPreferences{}, err
	}

	desktopPreferencesMu.Lock()
	path, err := a.desktopPreferencesPath()
	if err == nil {
		err = writePersistedDesktopPreferences(path, persistedDesktopPreferences{
			ThemeMode:          mode,
			ThemePack:          pack,
			ThemeStyle:         style,
			ThemeGradientPacks: gradientPacks,
		})
	}
	desktopPreferencesMu.Unlock()
	if err != nil {
		return DesktopPreferences{}, err
	}
	return a.GetDesktopPreferences()
}
