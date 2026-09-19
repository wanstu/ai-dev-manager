package desktop

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDesktopThemePreferencesDefaultToSystemAurora(t *testing.T) {
	path := filepath.Join(t.TempDir(), "desktop-preferences.json")
	state, err := readPersistedDesktopPreferences(path)
	if err != nil {
		t.Fatal(err)
	}
	if state.ThemeMode != "system" || state.ThemePack != "aurora" {
		t.Fatalf("default theme = %q / %q; want system / aurora", state.ThemeMode, state.ThemePack)
	}
}

func TestSetDesktopThemePersistsPreferences(t *testing.T) {
	path := filepath.Join(t.TempDir(), "desktop-preferences.json")
	adapter := &Adapter{preferencesPath: path}

	preferences, err := adapter.SetDesktopTheme("dark", "forest")
	if err != nil {
		t.Fatal(err)
	}
	if preferences.ThemeMode != "dark" || preferences.ThemePack != "forest" {
		t.Fatalf("saved theme = %q / %q; want dark / forest", preferences.ThemeMode, preferences.ThemePack)
	}

	reloaded := &Adapter{preferencesPath: path}
	preferences, err = reloaded.GetDesktopPreferences()
	if err != nil {
		t.Fatal(err)
	}
	if preferences.ThemeMode != "dark" || preferences.ThemePack != "forest" {
		t.Fatalf("reloaded theme = %q / %q; want dark / forest", preferences.ThemeMode, preferences.ThemePack)
	}
}

func TestSetDesktopThemeAllowsKitDefaultAndFutureRuntimePack(t *testing.T) {
	path := filepath.Join(t.TempDir(), "desktop-preferences.json")
	adapter := &Adapter{preferencesPath: path}

	preferences, err := adapter.SetDesktopTheme("light", "")
	if err != nil {
		t.Fatal(err)
	}
	if preferences.ThemeMode != "light" || preferences.ThemePack != "" {
		t.Fatalf("saved theme = %q / %q; want light / empty", preferences.ThemeMode, preferences.ThemePack)
	}

	preferences, err = adapter.SetDesktopTheme("system", "future-pack")
	if err != nil {
		t.Fatal(err)
	}
	if preferences.ThemePack != "future-pack" {
		t.Fatalf("runtime theme pack = %q; want future-pack", preferences.ThemePack)
	}
}

func TestSetDesktopThemeRejectsInvalidValuesWithoutWriting(t *testing.T) {
	for _, tt := range []struct {
		name string
		mode string
		pack string
	}{
		{name: "mode", mode: "sepia", pack: "aurora"},
		{name: "pack spaces", mode: "system", pack: "not a theme"},
		{name: "pack path", mode: "system", pack: "../theme"},
		{name: "pack uppercase", mode: "system", pack: "Aurora!"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "desktop-preferences.json")
			adapter := &Adapter{preferencesPath: path}
			if _, err := adapter.SetDesktopTheme(tt.mode, tt.pack); err == nil {
				t.Fatal("expected validation error")
			}
			if _, err := os.Stat(path); !os.IsNotExist(err) {
				t.Fatalf("invalid theme must not write preferences, stat err=%v", err)
			}
		})
	}
}
