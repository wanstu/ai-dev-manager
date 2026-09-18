//go:build windows || darwin || linux

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

func TestTrayExitMenuHasTwoExplicitLifecycleChoices(t *testing.T) {
	source, err := os.ReadFile("tray_manager_supported.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(source)
	for _, required := range []string{
		`trayQuitKeepBackgroundLabel = "退出（保留后台）"`,
		`trayQuitStopBackgroundLabel = "退出（不保留后台）"`,
		`menu.Add(trayQuitKeepBackgroundLabel, t.quitDesktop)`,
		`menu.Add(trayQuitStopBackgroundLabel, t.quitAndStopLocalBackground)`,
		`func (t *trayManager) quitDesktop()`,
		`func (t *trayManager) quitAndStopLocalBackground()`,
		`StopLocalADM`,
	} {
		if !strings.Contains(text, required) {
			t.Fatalf("tray source missing %q", required)
		}
	}
	for _, forbidden := range []string{
		`QuestionDialog`,
		`DefaultButton`,
		`CancelButton`,
		`退出 Desktop（保留后台服务）`,
		`停止本地后台服务并退出`,
		`仅退出 Desktop`,
	} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("tray source must not contain the old ambiguous exit UI %q", forbidden)
		}
	}
}

func TestStopAndExitUsesOnlySafeLocalADMPath(t *testing.T) {
	source, err := os.ReadFile("tray_manager_supported.go")
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
