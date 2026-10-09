package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/wanstu/wails-desktop-kit/updater"
)

type lateSuccessUpdateProvider struct {
	started chan struct{}
	release updater.Release
}

func (p lateSuccessUpdateProvider) Latest(ctx context.Context) (updater.Release, error) {
	close(p.started)
	<-ctx.Done()
	// Simulate a slow provider that ignores cancellation and returns a stale success.
	return p.release, nil
}

func TestCanceledCheckDoesNotCommitLateProviderResult(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows updater integration")
	}
	started := make(chan struct{})
	manager := newUpdateManager("v1.4.0-rc.3")
	manager.installation = fakeUpdaterInstallation(updater.InstallationPortable)
	manager.clientFactory = func(bool) *updater.Client {
		return &updater.Client{
			CurrentVersion: manager.version,
			Provider: lateSuccessUpdateProvider{
				started: started,
				release: updater.Release{
					Version: "v1.4.0-rc.4",
					Assets: []updater.Asset{{
						Name: "adm-desktop-v1.4.0-rc.4-windows-amd64-setup.exe",
						URL:  "https://example.test/setup.exe", SHA256: strings.Repeat("a", 64),
					}},
				},
			},
			SelectAsset: updater.AssetBySuffix("windows-amd64-setup.exe"),
		}
	}
	done := make(chan error, 1)
	go func() { _, err := manager.CheckDesktopUpdate(true); done <- err }()
	select {
	case <-started:
	case <-time.After(3 * time.Second):
		t.Fatal("provider did not start")
	}
	if _, err := manager.CancelDesktopUpdate(); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("late success must be canceled: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("cancel did not complete")
	}
	state, err := manager.GetDesktopUpdateStatus()
	if err != nil {
		t.Fatal(err)
	}
	if state.UpdateAvailable || state.LatestVersion != "" || state.Phase != "error" {
		t.Fatalf("late success incorrectly committed: %+v", state)
	}
}

func TestDesktopUpdateChecksumMismatchRefusesUnverifiedSetup(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows updater integration")
	}
	payload := []byte("binary payload that differs from trusted checksum")
	digest := sha256.Sum256([]byte("expected setup bytes"))
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(payload)
	}))
	defer server.Close()

	manager := newUpdateManager("v1.4.0-rc.3")
	manager.installation = fakeUpdaterInstallation(updater.InstallationUser)
	manager.cacheDir = func() (string, error) { return t.TempDir(), nil }
	manager.clientFactory = func(bool) *updater.Client {
		return &updater.Client{
			CurrentVersion: manager.version,
			Provider: testUpdateProvider{release: updater.Release{
				Version: "v1.4.0-rc.4",
				Assets: []updater.Asset{{
					Name: "adm-desktop-v1.4.0-rc.4-windows-amd64-setup.exe",
					URL:  server.URL, SHA256: hex.EncodeToString(digest[:]),
					Size: int64(len(payload)),
				}},
			}},
			SelectAsset: updater.AssetBySuffix("windows-amd64-setup.exe"),
			HTTPClient:  server.Client(),
		}
	}
	if _, err := manager.CheckDesktopUpdate(true); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.DownloadDesktopUpdate(); err == nil {
		t.Fatal("unverified update setup must fail")
	}
	state, err := manager.GetDesktopUpdateStatus()
	if err != nil {
		t.Fatal(err)
	}
	if state.InstallReady || state.DownloadPath != "" || state.Phase != "error" {
		t.Fatalf("unverified setup must never be installable: %+v", state)
	}
}

func TestDesktopUpdateInstallingBlocksDuplicateActions(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows updater integration")
	}
	manager := newUpdateManager("v1.4.0-rc.3")
	manager.installation = fakeUpdaterInstallation(updater.InstallationUser)
	manager.mu.Lock()
	manager.phase = "installing"
	manager.installing = true
	manager.download = updater.DownloadResult{Path: "already-downloaded.exe"}
	manager.mu.Unlock()

	state, err := manager.GetDesktopUpdateStatus()
	if err != nil {
		t.Fatal(err)
	}
	if state.InstallReady {
		t.Fatal("installing must not expose another Install action")
	}
	if _, err := manager.CheckDesktopUpdate(true); err == nil {
		t.Fatal("check must refuse during install")
	}
	if _, err := manager.DownloadDesktopUpdate(); err == nil {
		t.Fatal("download must refuse during install")
	}
	if err := manager.InstallDesktopUpdate(); err == nil {
		t.Fatal("duplicate installer launch must be refused")
	}
}
