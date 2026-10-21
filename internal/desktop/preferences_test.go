package desktop

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestDesktopThemePreferencesDefaultToSystemAuroraSolid(t *testing.T) {
	path := filepath.Join(t.TempDir(), "desktop-preferences.json")
	state, err := readPersistedDesktopPreferences(path)
	if err != nil {
		t.Fatal(err)
	}
	if state.ThemeMode != "system" || state.ThemePack != "aurora" || state.ThemeStyle != "solid" {
		t.Fatalf("default theme = %q / %q / %q; want system / aurora / solid", state.ThemeMode, state.ThemePack, state.ThemeStyle)
	}
	if !reflect.DeepEqual(state.ThemeGradientPacks, []string{"aurora", "ocean"}) {
		t.Fatalf("default gradient packs = %#v; want aurora/ocean", state.ThemeGradientPacks)
	}
}

func TestSetDesktopThemePersistsSolidPreferences(t *testing.T) {
	path := filepath.Join(t.TempDir(), "desktop-preferences.json")
	adapter := &Adapter{preferencesPath: path}

	preferences, err := adapter.SetDesktopTheme("dark", "forest", "solid", nil)
	if err != nil {
		t.Fatal(err)
	}
	if preferences.ThemeMode != "dark" || preferences.ThemePack != "forest" || preferences.ThemeStyle != "solid" {
		t.Fatalf("saved theme = %q / %q / %q; want dark / forest / solid", preferences.ThemeMode, preferences.ThemePack, preferences.ThemeStyle)
	}

	reloaded := &Adapter{preferencesPath: path}
	preferences, err = reloaded.GetDesktopPreferences()
	if err != nil {
		t.Fatal(err)
	}
	if preferences.ThemeMode != "dark" || preferences.ThemePack != "forest" || preferences.ThemeStyle != "solid" {
		t.Fatalf("reloaded theme = %q / %q / %q; want dark / forest / solid", preferences.ThemeMode, preferences.ThemePack, preferences.ThemeStyle)
	}
}

func TestSetDesktopThemePersistsGradientPreferences(t *testing.T) {
	path := filepath.Join(t.TempDir(), "desktop-preferences.json")
	adapter := &Adapter{preferencesPath: path}

	preferences, err := adapter.SetDesktopTheme("system", "aurora", "gradient", []string{"Aurora", "ocean", "forest"})
	if err != nil {
		t.Fatal(err)
	}
	if preferences.ThemeStyle != "gradient" {
		t.Fatalf("theme style=%q, want gradient", preferences.ThemeStyle)
	}
	if !reflect.DeepEqual(preferences.ThemeGradientPacks, []string{"aurora", "ocean", "forest"}) {
		t.Fatalf("gradient packs=%#v", preferences.ThemeGradientPacks)
	}

	reloaded := &Adapter{preferencesPath: path}
	preferences, err = reloaded.GetDesktopPreferences()
	if err != nil {
		t.Fatal(err)
	}
	if preferences.ThemeStyle != "gradient" || !reflect.DeepEqual(preferences.ThemeGradientPacks, []string{"aurora", "ocean", "forest"}) {
		t.Fatalf("reloaded gradient preferences=%+v", preferences)
	}
}

func TestCustomGradientPresetPersistsAndSurvivesThemeChanges(t *testing.T) {
	path := filepath.Join(t.TempDir(), "desktop-preferences.json")
	adapter := &Adapter{preferencesPath: path}

	preferences, err := adapter.SaveDesktopGradientPreset("工作", []string{"Aurora", "sakura", "ocean"})
	if err != nil {
		t.Fatal(err)
	}
	want := []DesktopGradientPreset{{Name: "工作", Packs: []string{"aurora", "sakura", "ocean"}}}
	if !reflect.DeepEqual(preferences.CustomGradientPresets, want) {
		t.Fatalf("custom gradient presets=%#v, want %#v", preferences.CustomGradientPresets, want)
	}

	if _, err := adapter.SetDesktopTheme("dark", "forest", "gradient", []string{"forest", "sunset"}); err != nil {
		t.Fatal(err)
	}
	reloaded := &Adapter{preferencesPath: path}
	preferences, err = reloaded.GetDesktopPreferences()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(preferences.CustomGradientPresets, want) {
		t.Fatalf("theme change lost custom gradient presets: %#v", preferences.CustomGradientPresets)
	}
}

func TestCustomGradientPresetSaveReplacesByNameAndDeletePersists(t *testing.T) {
	path := filepath.Join(t.TempDir(), "desktop-preferences.json")
	adapter := &Adapter{preferencesPath: path}

	if _, err := adapter.SaveDesktopGradientPreset("夜间", []string{"midnight", "neon"}); err != nil {
		t.Fatal(err)
	}
	preferences, err := adapter.SaveDesktopGradientPreset("夜间", []string{"obsidian", "cyber", "neon"})
	if err != nil {
		t.Fatal(err)
	}
	want := []DesktopGradientPreset{{Name: "夜间", Packs: []string{"obsidian", "cyber", "neon"}}}
	if !reflect.DeepEqual(preferences.CustomGradientPresets, want) {
		t.Fatalf("replaced presets=%#v, want %#v", preferences.CustomGradientPresets, want)
	}

	preferences, err = adapter.DeleteDesktopGradientPreset("夜间")
	if err != nil {
		t.Fatal(err)
	}
	if len(preferences.CustomGradientPresets) != 0 {
		t.Fatalf("deleted presets=%#v, want empty", preferences.CustomGradientPresets)
	}
	reloaded := &Adapter{preferencesPath: path}
	preferences, err = reloaded.GetDesktopPreferences()
	if err != nil {
		t.Fatal(err)
	}
	if len(preferences.CustomGradientPresets) != 0 {
		t.Fatalf("reloaded deleted presets=%#v, want empty", preferences.CustomGradientPresets)
	}
}

func TestCustomGradientPresetRejectsInvalidValues(t *testing.T) {
	path := filepath.Join(t.TempDir(), "desktop-preferences.json")
	adapter := &Adapter{preferencesPath: path}

	for _, tt := range []struct {
		name  string
		packs []string
	}{
		{name: "", packs: []string{"aurora", "ocean"}},
		{name: "one", packs: []string{"aurora"}},
		{name: "duplicate", packs: []string{"aurora", "aurora"}},
		{name: "bad", packs: []string{"aurora", "../theme"}},
	} {
		if _, err := adapter.SaveDesktopGradientPreset(tt.name, tt.packs); err == nil {
			t.Fatalf("SaveDesktopGradientPreset(%q, %#v) expected validation error", tt.name, tt.packs)
		}
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("invalid preset must not write preferences, stat err=%v", err)
	}
}

func TestDesktopThemePreferencesNormalizeOlderSolidFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "desktop-preferences.json")
	if err := os.WriteFile(path, []byte(`{"theme_mode":"dark","theme_pack":"forest"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	state, err := readPersistedDesktopPreferences(path)
	if err != nil {
		t.Fatal(err)
	}
	if state.ThemeMode != "dark" || state.ThemePack != "forest" || state.ThemeStyle != "solid" {
		t.Fatalf("old theme normalized to %+v", state)
	}
	if !reflect.DeepEqual(state.ThemeGradientPacks, []string{"aurora", "ocean"}) {
		t.Fatalf("old preference gradient defaults=%#v", state.ThemeGradientPacks)
	}
}

func TestSetDesktopThemeAllowsKitDefaultAndFutureRuntimePack(t *testing.T) {
	path := filepath.Join(t.TempDir(), "desktop-preferences.json")
	adapter := &Adapter{preferencesPath: path}

	preferences, err := adapter.SetDesktopTheme("light", "", "solid", nil)
	if err != nil {
		t.Fatal(err)
	}
	if preferences.ThemeMode != "light" || preferences.ThemePack != "" {
		t.Fatalf("saved theme = %q / %q; want light / empty", preferences.ThemeMode, preferences.ThemePack)
	}

	preferences, err = adapter.SetDesktopTheme("system", "future-pack", "solid", nil)
	if err != nil {
		t.Fatal(err)
	}
	if preferences.ThemePack != "future-pack" {
		t.Fatalf("runtime theme pack = %q; want future-pack", preferences.ThemePack)
	}
}

func TestSetDesktopThemeRejectsInvalidValuesWithoutWriting(t *testing.T) {
	for _, tt := range []struct {
		name     string
		mode     string
		pack     string
		style    string
		gradient []string
	}{
		{name: "mode", mode: "sepia", pack: "aurora", style: "solid"},
		{name: "pack spaces", mode: "system", pack: "not a theme", style: "solid"},
		{name: "pack path", mode: "system", pack: "../theme", style: "solid"},
		{name: "pack uppercase punctuation", mode: "system", pack: "Aurora!", style: "solid"},
		{name: "style", mode: "system", pack: "aurora", style: "animated"},
		{name: "gradient duplicate", mode: "system", pack: "aurora", style: "gradient", gradient: []string{"aurora", "aurora"}},
		{name: "gradient single", mode: "system", pack: "aurora", style: "gradient", gradient: []string{"aurora"}},
		{name: "gradient too many", mode: "system", pack: "aurora", style: "gradient", gradient: []string{"aurora", "ocean", "forest", "sunset", "future"}},
		{name: "gradient path", mode: "system", pack: "aurora", style: "gradient", gradient: []string{"aurora", "../theme"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "desktop-preferences.json")
			adapter := &Adapter{preferencesPath: path}
			if _, err := adapter.SetDesktopTheme(tt.mode, tt.pack, tt.style, tt.gradient); err == nil {
				t.Fatal("expected validation error")
			}
			if _, err := os.Stat(path); !os.IsNotExist(err) {
				t.Fatalf("invalid theme must not write preferences, stat err=%v", err)
			}
		})
	}
}
