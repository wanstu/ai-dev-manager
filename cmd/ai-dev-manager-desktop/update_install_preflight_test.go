package main

import (
	"errors"
	"runtime"
	"testing"

	"github.com/wanstu/wails-desktop-kit/updater"
)

func TestDesktopUpdatePreflightFailurePreventsSetupAndQuit(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows installer only")
	}
	m := newUpdateManager("v1.4.0-rc.4")
	m.installation = fakeUpdaterInstallation(updater.InstallationUser)
	want := errors.New("local Gateway has active jobs")
	prepareCalls, quitCalls := 0, 0
	m.onReady(nil, func() { quitCalls++ }, func() error {
		prepareCalls++
		return want
	})
	m.mu.Lock()
	m.phase = "downloaded"
	m.download = updater.DownloadResult{Path: "fixture-not-a-real-installer.exe"}
	m.mu.Unlock()
	if err := m.InstallDesktopUpdate(); !errors.Is(err, want) {
		t.Fatalf("preflight error = %v; want %v", err, want)
	}
	if prepareCalls != 1 || quitCalls != 0 {
		t.Fatalf("prepare=%d quit=%d; no installer or quit permitted", prepareCalls, quitCalls)
	}
	state, err := m.GetDesktopUpdateStatus()
	if err != nil {
		t.Fatal(err)
	}
	if !state.InstallReady || state.Phase != "downloaded" {
		t.Fatalf("preflight failure must retain verified download and allow retry: %+v", state)
	}
}

func TestOnlyLoopbackGatewayIsEligibleForUpdateShutdown(t *testing.T) {
	valid := []string{"http://localhost:8001", "http://127.0.0.1:8001", "https://[::1]:8001"}
	for _, address := range valid {
		if !isLoopbackGatewayBaseURL(address) {
			t.Errorf("loopback %q rejected", address)
		}
	}
	invalid := []string{"http://112.124.57.12:8001", "https://example.com:8001", "file:///tmp/adm", "http://localhost.evil.test:8001", ""}
	for _, address := range invalid {
		if isLoopbackGatewayBaseURL(address) {
			t.Errorf("remote %q treated as local", address)
		}
	}
}
