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

// DesktopGradientPreset is a named user-defined gradient combination.
type DesktopGradientPreset struct {
	Name  string   `json:"name"`
	Packs []string `json:"packs"`
}

type persistedDesktopPreferences struct {
	ThemeMode             string                  `json:"theme_mode"`
	ThemePack             string                  `json:"theme_pack"`
	ThemeStyle            string                  `json:"theme_style,omitempty"`
	ThemeGradientPacks    []string                `json:"theme_gradient_packs,omitempty"`
	CustomGradientPresets []DesktopGradientPreset `json:"custom_gradient_presets,omitempty"`
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

func validateDesktopGradientPreset(name string, packs []string) (DesktopGradientPreset, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return DesktopGradientPreset{}, fmt.Errorf("Desktop gradient preset name is required")
	}
	if len([]rune(name)) > 40 {
		return DesktopGradientPreset{}, fmt.Errorf("Desktop gradient preset name must not exceed 40 characters")
	}
	if strings.ContainsAny(name, "\r\n\t") {
		return DesktopGradientPreset{}, fmt.Errorf("Desktop gradient preset name contains unsupported control characters")
	}
	if len(packs) < 2 {
		return DesktopGradientPreset{}, fmt.Errorf("Desktop gradient preset requires at least 2 theme packs")
	}
	_, _, _, normalizedPacks, err := validateDesktopTheme(defaultDesktopThemeMode, "", "gradient", packs)
	if err != nil {
		return DesktopGradientPreset{}, err
	}
	return DesktopGradientPreset{Name: name, Packs: normalizedPacks}, nil
}

func normalizeCustomGradientPresets(presets []DesktopGradientPreset) ([]DesktopGradientPreset, error) {
	normalized := make([]DesktopGradientPreset, 0, len(presets))
	seen := map[string]struct{}{}
	for _, item := range presets {
		preset, err := validateDesktopGradientPreset(item.Name, item.Packs)
		if err != nil {
			return nil, err
		}
		key := strings.ToLower(preset.Name)
		if _, exists := seen[key]; exists {
			return nil, fmt.Errorf("Desktop gradient preset names must be unique")
		}
		seen[key] = struct{}{}
		normalized = append(normalized, preset)
	}
	return normalized, nil
}

func desktopPreferenceStore(path string) *jsonstore.Store[persistedDesktopPreferences] {
	return jsonstore.New(path, jsonstore.Options[persistedDesktopPreferences]{
		Default: defaultPersistedDesktopPreferences,
		Validate: func(state persistedDesktopPreferences) error {
			if _, _, _, _, err := validateDesktopTheme(state.ThemeMode, state.ThemePack, state.ThemeStyle, state.ThemeGradientPacks); err != nil {
				return err
			}
			_, err := normalizeCustomGradientPresets(state.CustomGradientPresets)
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
	state.CustomGradientPresets, err = normalizeCustomGradientPresets(state.CustomGradientPresets)
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
	customPresets, err := normalizeCustomGradientPresets(state.CustomGradientPresets)
	if err != nil {
		return err
	}
	state.ThemeMode = mode
	state.ThemePack = pack
	state.ThemeStyle = style
	state.ThemeGradientPacks = gradientPacks
	state.CustomGradientPresets = customPresets
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
		var state persistedDesktopPreferences
		state, err = readPersistedDesktopPreferences(path)
		if err == nil {
			state.ThemeMode = mode
			state.ThemePack = pack
			state.ThemeStyle = style
			state.ThemeGradientPacks = gradientPacks
			err = writePersistedDesktopPreferences(path, state)
		}
	}
	desktopPreferencesMu.Unlock()
	if err != nil {
		return DesktopPreferences{}, err
	}
	return a.GetDesktopPreferences()
}

func (a *Adapter) SaveDesktopGradientPreset(name string, packs []string) (DesktopPreferences, error) {
	preset, err := validateDesktopGradientPreset(name, packs)
	if err != nil {
		return DesktopPreferences{}, err
	}

	desktopPreferencesMu.Lock()
	path, err := a.desktopPreferencesPath()
	if err == nil {
		var state persistedDesktopPreferences
		state, err = readPersistedDesktopPreferences(path)
		if err == nil {
			replaced := false
			for index := range state.CustomGradientPresets {
				if strings.EqualFold(state.CustomGradientPresets[index].Name, preset.Name) {
					state.CustomGradientPresets[index] = preset
					replaced = true
					break
				}
			}
			if !replaced {
				state.CustomGradientPresets = append(state.CustomGradientPresets, preset)
			}
			err = writePersistedDesktopPreferences(path, state)
		}
	}
	desktopPreferencesMu.Unlock()
	if err != nil {
		return DesktopPreferences{}, err
	}
	return a.GetDesktopPreferences()
}

func (a *Adapter) DeleteDesktopGradientPreset(name string) (DesktopPreferences, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return DesktopPreferences{}, fmt.Errorf("Desktop gradient preset name is required")
	}

	desktopPreferencesMu.Lock()
	path, err := a.desktopPreferencesPath()
	if err == nil {
		var state persistedDesktopPreferences
		state, err = readPersistedDesktopPreferences(path)
		if err == nil {
			filtered := state.CustomGradientPresets[:0]
			for _, preset := range state.CustomGradientPresets {
				if !strings.EqualFold(preset.Name, name) {
					filtered = append(filtered, preset)
				}
			}
			state.CustomGradientPresets = filtered
			err = writePersistedDesktopPreferences(path, state)
		}
	}
	desktopPreferencesMu.Unlock()
	if err != nil {
		return DesktopPreferences{}, err
	}
	return a.GetDesktopPreferences()
}
