package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/wanstu/wails-desktop-kit/updater"
)

type testUpdateProvider struct{ release updater.Release }

func (p testUpdateProvider) Latest(context.Context) (updater.Release, error) {
	return p.release, nil
}

type blockingUpdateProvider struct{}

func (blockingUpdateProvider) Latest(ctx context.Context) (updater.Release, error) {
	<-ctx.Done()
	return updater.Release{}, ctx.Err()
}

func fakeUpdaterInstallation(mode updater.InstallationMode) func() (updater.Installation, error) {
	return func() (updater.Installation, error) {
		return updater.Installation{AppID: desktopUpdateAppID, Mode: mode}, nil
	}
}

func TestDesktopUpdateDownloadVerifiedAndPortableNotInstalled(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows setup update surface")
	}
	payload := []byte("setup fixture for ADM updater")
	digest := sha256.Sum256(payload)
	sha := hex.EncodeToString(digest[:])
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(payload)
	}))
	defer server.Close()

	m := newUpdateManager("v1.4.0-rc.3")
	m.installation = fakeUpdaterInstallation(updater.InstallationPortable)
	m.cacheDir = func() (string, error) { return t.TempDir(), nil }
	m.clientFactory = func(prerelease bool) *updater.Client {
		return &updater.Client{
			CurrentVersion: m.version,
			Provider: testUpdateProvider{release: updater.Release{
				Version: "v1.4.0-rc.4", Prerelease: true,
				PageURL: "https://github.com/wanstu/ai-dev-manager/releases/tag/v1.4.0-rc.4",
				Assets: []updater.Asset{{
					Name: "adm-desktop-v1.4.0-rc.4-windows-amd64-setup.exe",
					URL:  server.URL + "/setup.exe", Size: int64(len(payload)), SHA256: sha,
				}},
			}},
			SelectAsset: updater.AssetBySuffix("windows-amd64-setup.exe"),
			HTTPClient:  server.Client(),
		}
	}

	checked, err := m.CheckDesktopUpdate(true)
	if err != nil {
		t.Fatal(err)
	}
	if !checked.UpdateAvailable || checked.Phase != "checked" || checked.LatestVersion != "v1.4.0-rc.4" {
		t.Fatalf("invalid check response: %+v", checked)
	}
	result, err := m.DownloadDesktopUpdate()
	if err != nil {
		t.Fatal(err)
	}
	if result.Phase != "downloaded" || result.InstallReady || result.Downloaded != int64(len(payload)) {
		t.Fatalf("invalid verified download: %+v", result)
	}
	data, err := os.ReadFile(result.DownloadPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != string(payload) {
		t.Fatal("download payload mismatch")
	}
	if err := m.InstallDesktopUpdate(); err == nil || !strings.Contains(err.Error(), "Portable") {
		t.Fatalf("portable install must be denied: %v", err)
	}
	if _, err := m.DownloadDesktopUpdate(); err == nil {
		t.Fatal("duplicate download should be refused")
	}
	if _, err := os.Stat(filepath.Dir(result.DownloadPath)); err != nil {
		t.Fatal(err)
	}
}

func TestDesktopUpdateCannotCheckDevBuild(t *testing.T) {
	m := newUpdateManager("dev")
	m.installation = fakeUpdaterInstallation(updater.InstallationPortable)
	status, err := m.GetDesktopUpdateStatus()
	if err != nil {
		t.Fatal(err)
	}
	if status.Supported {
		t.Fatal("dev build must not appear updatable")
	}
	if _, err := m.CheckDesktopUpdate(true); err == nil {
		t.Fatal("dev check must fail")
	}
}

func TestDesktopUpdateCancelInFlightCheck(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows setup update surface")
	}
	m := newUpdateManager("v1.4.0-rc.3")
	m.installation = fakeUpdaterInstallation(updater.InstallationPortable)
	started := make(chan struct{})
	m.clientFactory = func(bool) *updater.Client {
		close(started)
		return &updater.Client{
			CurrentVersion: m.version,
			Provider:       blockingUpdateProvider{},
			SelectAsset:    updater.AssetBySuffix("windows-amd64-setup.exe"),
		}
	}
	done := make(chan error, 1)
	go func() { _, err := m.CheckDesktopUpdate(true); done <- err }()
	select {
	case <-started:
	case <-time.After(3 * time.Second):
		t.Fatal("check did not start")
	}
	if _, err := m.CancelDesktopUpdate(); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("cancel result: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("check did not respond to cancellation")
	}
}
