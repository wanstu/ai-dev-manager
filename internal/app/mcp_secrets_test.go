package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ai-dev-manager-v2/internal/catalog"
)

func TestMCPSecretVaultResolutionAndDeletionProtection(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	s := New(path)
	if _, err := s.SecretSet("git-token", "secret-shall-not-persist-in-cleartext"); err != nil {
		t.Fatal(err)
	}
	secretName := "${secret:git-token}"
	m, err := s.MCPs.AddMCPConfig("demo", catalog.MCPConfig{
		Transport: "streamable-http", AuthMode: "headers", Endpoint: "https://example.test/mcp",
		HeaderRefs: map[string]string{"Authorization": "Bearer " + secretName},
	})
	if err != nil {
		t.Fatal(err)
	}
	activation, status, err := s.ResolveGlobalMCPActivation(m.ID)
	if err != nil || activation == nil || status.ErrorKind != "" {
		t.Fatalf("activation: %+v %+v %v", activation, status, err)
	}
	if activation.Headers["Authorization"] != "Bearer secret-shall-not-persist-in-cleartext" {
		t.Fatalf("vault reference did not resolve")
	}
	persisted, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(persisted), "secret-shall-not-persist-in-cleartext") {
		t.Fatal("MCP catalog leaked secret into state JSON")
	}
	meta, err := s.SecretList()
	if err != nil || len(meta) != 1 || meta[0].Name != "git-token" {
		t.Fatalf("metadata listing wrong: %+v %v", meta, err)
	}
	if err := s.SecretDelete("git-token"); err == nil {
		t.Fatal("removed secret still referenced by MCP")
	}
	if err := s.MCPs.Remove(m.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.SecretDelete("git-token"); err != nil {
		t.Fatal(err)
	}
	activation, status, err = s.ResolveGlobalMCPActivation(m.ID)
	if err != nil {
		t.Fatal(err)
	}
	if activation != nil && status.State == "healthy" {
		t.Fatal("nonexistent MCP should not be active")
	}
}
func TestMCPMissingVaultReferenceIsUnavailableWithoutLeaking(t *testing.T) {
	s := New(filepath.Join(t.TempDir(), "state.json"))
	m, err := s.MCPs.AddMCPConfig("missing", catalog.MCPConfig{
		Transport: "streamable-http", AuthMode: "headers", Endpoint: "https://example.test/mcp",
		HeaderRefs: map[string]string{"Authorization": "Bearer ${secret:not-found}"},
	})
	if err != nil {
		t.Fatal(err)
	}
	a, status, err := s.ResolveGlobalMCPActivation(m.ID)
	if err != nil || a != nil || status.ErrorKind != "unresolved_secret_reference" {
		t.Fatalf("missing secret: %+v %+v %v", a, status, err)
	}
}
