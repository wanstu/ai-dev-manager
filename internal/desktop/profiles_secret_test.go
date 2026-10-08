package desktop

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDesktopConnectionCredentialsAreEncryptedAtRest(t *testing.T) {
	path := filepath.Join(t.TempDir(), "desktop-connections.json")
	a := &Adapter{profilesPath: path}
	saved, err := a.SaveConnectionProfile(ConnectionProfile{Name: "secure", BaseURL: "https://adm.example.test", APIKey: "secret-remote-admin-key"})
	if err != nil {
		t.Fatal(err)
	}
	id := saved.Profiles[1].ID
	if !saved.Profiles[1].APIKeyConfigured || saved.Profiles[1].APIKey != "" {
		t.Fatalf("UI must not receive credentials: %+v", saved.Profiles[1])
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "secret-remote-admin-key") {
		t.Fatal("API key stored in plaintext profile")
	}
	if _, err := a.SelectConnectionProfile(id); err != nil {
		t.Fatal(err)
	}
	if value := a.connectionAPIKey("https://adm.example.test", ""); value != "secret-remote-admin-key" {
		t.Fatal("encrypted connection key failed to resolve")
	}
	other := &Adapter{profilesPath: path}
	loaded, err := other.GetConnectionProfiles()
	if err != nil {
		t.Fatal(err)
	}
	if !loaded.Profiles[1].APIKeyConfigured || loaded.Profiles[1].APIKey != "" {
		t.Fatalf("re-opened profile: %+v", loaded.Profiles[1])
	}
	if _, err := other.SaveConnectionProfile(ConnectionProfile{ID: id, Name: "renamed", BaseURL: "https://adm.example.test"}); err != nil {
		t.Fatal(err)
	}
	if value := other.connectionAPIKey("https://adm.example.test", ""); value != "secret-remote-admin-key" {
		t.Fatalf("edit without key lost encrypted credential")
	}
	if _, err := other.DeleteConnectionProfile(id); err != nil {
		t.Fatal(err)
	}
	if value := other.connectionAPIKey("https://adm.example.test", ""); value != "" {
		t.Fatal("deleted connection key still accessible")
	}
}
func TestLegacyDesktopConnectionKeyMigratesOutOfPlaintext(t *testing.T) {
	path := filepath.Join(t.TempDir(), "desktop-connections.json")
	legacy := ConnectionProfiles{
		Profiles: []ConnectionProfile{{ID: "oldremote", Name: "legacy", BaseURL: "https://adm.example.test", APIKey: "legacy-private-token"}},
		ActiveID: "oldremote",
	}
	raw, _ := json.Marshal(legacy)
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	profiles, err := readConnectionProfiles(path)
	if err != nil {
		t.Fatal(err)
	}
	if profiles.Profiles[0].APIKey != "" || !profiles.Profiles[0].APIKeyConfigured {
		t.Fatalf("legacy migration did not sanitize in-memory profile")
	}
	persisted, _ := os.ReadFile(path)
	if strings.Contains(string(persisted), "legacy-private-token") {
		t.Fatal("legacy plaintext retained on disk")
	}
	a := &Adapter{profilesPath: path}
	if value := a.connectionAPIKey("https://adm.example.test", ""); value != "legacy-private-token" {
		t.Fatal("legacy key migration changed credential")
	}
}
