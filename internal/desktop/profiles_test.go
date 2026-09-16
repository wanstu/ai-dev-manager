package desktop

import (
	"os"
	"path/filepath"
	"testing"
)

func TestConnectionProfilesPersistEditSelectAndDelete(t *testing.T) {
	path := filepath.Join(t.TempDir(), "adm", "desktop-connections.json")
	a := &Adapter{profilesPath: path}
	initial, err := a.GetConnectionProfiles()
	if err != nil || initial.ActiveID != "local" {
		t.Fatalf("initial: %+v %v", initial, err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("read must not create preferences")
	}
	state, err := a.SaveConnectionProfile(ConnectionProfile{Name: "测试连接", BaseURL: "http://127.0.0.1:8001/", StartServiceOnDesktopLaunch: true})
	if err != nil {
		t.Fatal(err)
	}
	id := state.Profiles[1].ID
	if id == "" || state.Profiles[1].BaseURL != "http://127.0.0.1:8001" || !state.Profiles[1].StartServiceOnDesktopLaunch {
		t.Fatalf("saved: %+v", state)
	}
	if _, err := a.SelectConnectionProfile(id); err != nil {
		t.Fatal(err)
	}
	b := &Adapter{profilesPath: path}
	reopened, err := b.GetConnectionProfiles()
	if err != nil || reopened.ActiveID != id || len(reopened.Profiles) != 2 {
		t.Fatalf("reopen: %+v %v", reopened, err)
	}
	updated, err := b.SaveConnectionProfile(ConnectionProfile{ID: id, Name: "改名", BaseURL: "http://127.0.0.1:8002"})
	if err != nil || updated.Profiles[1].ID != id || updated.ActiveID != id || updated.Profiles[1].StartServiceOnDesktopLaunch {
		t.Fatalf("edit: %+v %v", updated, err)
	}
	deleted, err := b.DeleteConnectionProfile(id)
	if err != nil || deleted.ActiveID != "" || len(deleted.Profiles) != 1 {
		t.Fatalf("delete active: %+v %v", deleted, err)
	}
	if _, err := b.DeleteConnectionProfile("local"); err != nil {
		t.Fatal(err)
	}
	empty, err := a.GetConnectionProfiles()
	if err != nil || len(empty.Profiles) != 0 || empty.ActiveID != "" {
		t.Fatalf("empty must stay empty: %+v %v", empty, err)
	}
}

func TestConnectionProfileInvalidWritesPreserveFile(t *testing.T) {
	a := &Adapter{profilesPath: filepath.Join(t.TempDir(), "connections.json")}
	if _, err := a.SaveConnectionProfile(ConnectionProfile{Name: "good", BaseURL: "http://localhost:8001"}); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(a.profilesPath)
	for _, p := range []ConnectionProfile{
		{Name: "", BaseURL: "http://localhost:8001"},
		{Name: "bad", BaseURL: "file:///tmp"},
		{Name: "bad", BaseURL: "http://user:password@localhost:8001"},
		{Name: "bad", BaseURL: "http://localhost:8001?token=private"},
		{Name: "bad", BaseURL: "http://localhost:8001#private"},
		{ID: "missing", Name: "bad", BaseURL: "http://localhost:8001"},
		{Name: "remote-autostart", BaseURL: "https://adm.example.test", StartServiceOnDesktopLaunch: true},
	} {
		if _, err := a.SaveConnectionProfile(p); err == nil {
			t.Fatalf("accepted invalid profile %+v", p)
		}
	}
	if _, err := a.SelectConnectionProfile("missing"); err == nil {
		t.Fatal("selected missing profile")
	}
	if _, err := a.DeleteConnectionProfile("missing"); err == nil {
		t.Fatal("deleted missing profile")
	}
	after, _ := os.ReadFile(a.profilesPath)
	if string(before) != string(after) {
		t.Fatal("invalid mutation changed saved connections")
	}
	if err := os.WriteFile(a.profilesPath, []byte("{broken"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := a.SaveConnectionProfile(ConnectionProfile{Name: "new", BaseURL: "http://localhost:8002"}); err == nil {
		t.Fatal("silently overwrote corrupt preferences")
	}
	after, _ = os.ReadFile(a.profilesPath)
	if string(after) != "{broken" {
		t.Fatal("corrupt file not preserved")
	}
}

func TestConnectionProfilesAreIndependentOfGateway(t *testing.T) {
	a := &Adapter{profilesPath: filepath.Join(t.TempDir(), "connections.json")}
	if _, err := a.SaveConnectionProfile(ConnectionProfile{Name: "offline", BaseURL: "http://127.0.0.1:8001"}); err != nil {
		t.Fatal(err)
	}
	if _, err := a.GetSnapshot(); err == nil {
		t.Fatal("saving profiles must not connect or create a writable backend")
	}
	a.DisconnectADM()
	if _, err := a.GetConnectionProfiles(); err != nil {
		t.Fatal(err)
	}
}
