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

func TestRemoteConnectionDoesNotBlockDesktopQuitDecision(t *testing.T) {
	profile := desktop.ConnectionProfile{ID: "remote", BaseURL: "http://112.124.57.12:8001"}
	status := desktop.ADMConnectionStatus{
		State:                  "running",
		BaseURL:                profile.BaseURL,
		LocalBootstrapEligible: false,
	}
	if shouldStopActiveLocalBackground(profile, status) {
		t.Fatal("remote connection must not be treated as a local background service that Desktop must stop before quitting")
	}

	local := desktop.ConnectionProfile{ID: "local", BaseURL: "http://127.0.0.1:43137"}
	status.BaseURL = local.BaseURL
	status.LocalBootstrapEligible = true
	if !shouldStopActiveLocalBackground(local, status) {
		t.Fatal("running Desktop-managed local ADM should still be stopped before quitting")
	}
}

func TestStopAndExitUsesOnlySafeLocalADMPath(t *testing.T) {
	source, err := os.ReadFile("desktop_runtime.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(source)
	for _, required := range []string{
		`shouldStopActiveLocalBackground(profile, status)`,
		`status.LocalBootstrapEligible`,
		`desktop.ADMConnectionInput{BaseURL: profile.BaseURL}`,
		`Desktop 保持运行，后台服务没有被强制终止`,
	} {
		if !strings.Contains(text, required) {
			t.Fatalf("safe stop-and-exit source missing %q", required)
		}
	}
}
