package main

import (
	"os"
	"strings"
	"testing"

	"ai-dev-manager-v2/internal/desktop"
)

func TestActiveConnectionProfile(t *testing.T) {
	profiles := desktop.ConnectionProfiles{
		Profiles: []desktop.ConnectionProfile{
			{ID: "a", Name: "A", BaseURL: "http://127.0.0.1:43137"},
			{ID: "b", Name: "B", BaseURL: "http://127.0.0.1:43138"},
		},
		ActiveID: "b",
	}
	profile, ok := activeConnectionProfile(profiles)
	if !ok || profile.ID != "b" {
		t.Fatalf("active profile = %#v, %v; want b", profile, ok)
	}
	profiles.ActiveID = "missing"
	if _, ok := activeConnectionProfile(profiles); ok {
		t.Fatal("missing active profile should not resolve")
	}
}

func TestDesktopKitTrayKeepsTwoExplicitLifecycleChoices(t *testing.T) {
	source, err := os.ReadFile("desktop_runtime.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(source)
	for _, required := range []string{
		`trayQuitKeepBackgroundLabel = "退出（保留后台）"`,
		`trayQuitStopBackgroundLabel = "退出（不保留后台）"`,
		`desktopkit.Action(trayQuitKeepBackgroundLabel`,
		`desktopkit.Action(trayQuitStopBackgroundLabel`,
		`DisableQuit: true`,
		`StopLocalADM`,
	} {
		if !strings.Contains(text, required) {
			t.Fatalf("desktop runtime source missing %q", required)
		}
	}
}

func TestStopAndExitUsesOnlySafeLocalADMPath(t *testing.T) {
	source, err := os.ReadFile("desktop_runtime.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(source)
	for _, required := range []string{
		`status.State != "running"`,
		`!status.LocalBootstrapEligible`,
		`desktop.ADMConnectionInput{BaseURL: profile.BaseURL}`,
		`Desktop 保持运行，后台服务没有被强制终止`,
	} {
		if !strings.Contains(text, required) {
			t.Fatalf("safe stop-and-exit source missing %q", required)
		}
	}
}
