package secretvault

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEncryptedRoundTripAndMetadataOnly(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	s := New(path)
	if err := s.Set("mcp-key", "super-private-Token-XYZ"); err != nil {
		t.Fatal(err)
	}
	encoded, err := os.ReadFile(path + ".secrets.json")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "super-private-Token-XYZ") {
		t.Fatalf("vault stored secret plaintext")
	}
	v, err := New(path).Get("mcp-key")
	if err != nil || v != "super-private-Token-XYZ" {
		t.Fatalf("secret decryption failure: %v", err)
	}
	names, err := s.List()
	if err != nil || len(names) != 1 || names[0].Name != "mcp-key" {
		t.Fatalf("metadata listing: %+v %v", names, err)
	}
	if strings.Contains(strings.ToLower(strings.TrimSpace(string(encoded))), "super-private") {
		t.Fatal("unencrypted token detected")
	}
	if err := s.Set("mcp-key", "rotated-XYZ"); err != nil {
		t.Fatal(err)
	}
	current, _ := s.Get("mcp-key")
	if current != "rotated-XYZ" {
		t.Fatal("rotation not durable")
	}
	if err := s.Delete("mcp-key"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Get("mcp-key"); err == nil {
		t.Fatal("deleted token still present")
	}
}
func TestVaultRejectsInvalidNamesAndCorruption(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	s := New(path)
	for _, name := range []string{"", "../path", "x/y", "has spaces", strings.Repeat("x", 100)} {
		if err := s.Set(name, "key"); err == nil {
			t.Fatalf("accepted invalid name %q", name)
		}
	}
	if err := s.Set("good", "secret"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path+".secrets.json", []byte("corrupt"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Get("good"); err == nil {
		t.Fatal("corruption not detected")
	}
}
