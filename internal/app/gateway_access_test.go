package app

import (
	"encoding/hex"
	"path/filepath"
	"testing"
)

func TestGenerateGatewayAPIKeyUses256Bits(t *testing.T) {
	first, err := GenerateGatewayAPIKey()
	if err != nil {
		t.Fatal(err)
	}
	second, err := GenerateGatewayAPIKey()
	if err != nil {
		t.Fatal(err)
	}
	if len(first) != 64 || len(second) != 64 {
		t.Fatalf("generated key lengths = %d, %d; want 64", len(first), len(second))
	}
	if _, err := hex.DecodeString(first); err != nil {
		t.Fatalf("generated key is not hex: %v", err)
	}
	if first == second {
		t.Fatal("two independently generated gateway API keys unexpectedly matched")
	}
}

func TestSetupGatewayRemoteIsIdempotentWithoutRotate(t *testing.T) {
	service := New(filepath.Join(t.TempDir(), "state.json"))

	first, err := service.SetupGatewayRemote(nil, false)
	if err != nil {
		t.Fatal(err)
	}
	if !first.Readiness.Ready || !first.AdminKeyGenerated || !first.AgentKeyGenerated {
		t.Fatalf("first setup = %+v", first)
	}
	if len(first.Status.AllowedHosts) != 1 || first.Status.AllowedHosts[0] != "*" {
		t.Fatalf("first setup hosts = %#v", first.Status.AllowedHosts)
	}
	if first.AdminAPIKey == "" || first.AgentAPIKey == "" || first.AdminAPIKey == first.AgentAPIKey {
		t.Fatalf("invalid generated keys: admin=%q agent=%q", first.AdminAPIKey, first.AgentAPIKey)
	}

	second, err := service.SetupGatewayRemote(nil, false)
	if err != nil {
		t.Fatal(err)
	}
	if second.AdminKeyGenerated || second.AgentKeyGenerated {
		t.Fatalf("second setup unexpectedly rotated keys: %+v", second)
	}
	if second.AdminAPIKey != "" || second.AgentAPIKey != "" {
		t.Fatalf("second setup unexpectedly returned secrets: %+v", second)
	}
	if ok, _ := service.VerifyGatewayAdminAPIKey(first.AdminAPIKey); !ok {
		t.Fatal("first Admin key stopped working after idempotent setup")
	}
	if ok, _ := service.VerifyGatewayAgentAPIKey(first.AgentAPIKey); !ok {
		t.Fatal("first Agent key stopped working after idempotent setup")
	}
}

func TestSetupGatewayRemoteWithoutHostsPreservesExistingPolicy(t *testing.T) {
	service := New(filepath.Join(t.TempDir(), "state.json"))
	first, err := service.SetupGatewayRemote([]string{"adm.example.com", "10.0.0.8"}, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Status.AllowedHosts) != 2 {
		t.Fatalf("initial hosts = %#v", first.Status.AllowedHosts)
	}

	second, err := service.SetupGatewayRemote(nil, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(second.Status.AllowedHosts) != 2 ||
		second.Status.AllowedHosts[0] != "10.0.0.8" ||
		second.Status.AllowedHosts[1] != "adm.example.com" {
		t.Fatalf("implicit setup widened host policy: %#v", second.Status.AllowedHosts)
	}
	if second.AdminKeyGenerated || second.AgentKeyGenerated {
		t.Fatalf("implicit setup unexpectedly rotated keys: %+v", second)
	}
}

func TestSetupGatewayRemoteRotateKeysInvalidatesOldKeys(t *testing.T) {
	service := New(filepath.Join(t.TempDir(), "state.json"))
	first, err := service.SetupGatewayRemote([]string{"example.test"}, false)
	if err != nil {
		t.Fatal(err)
	}
	second, err := service.SetupGatewayRemote([]string{"example.test"}, true)
	if err != nil {
		t.Fatal(err)
	}
	if !second.AdminKeyGenerated || !second.AgentKeyGenerated {
		t.Fatalf("rotate setup did not generate both keys: %+v", second)
	}
	if first.AdminAPIKey == second.AdminAPIKey || first.AgentAPIKey == second.AgentAPIKey {
		t.Fatal("rotate setup reused an old key")
	}
	if ok, _ := service.VerifyGatewayAdminAPIKey(first.AdminAPIKey); ok {
		t.Fatal("old Admin key still works after rotation")
	}
	if ok, _ := service.VerifyGatewayAgentAPIKey(first.AgentAPIKey); ok {
		t.Fatal("old Agent key still works after rotation")
	}
	if ok, _ := service.VerifyGatewayAdminAPIKey(second.AdminAPIKey); !ok {
		t.Fatal("new Admin key does not work")
	}
	if ok, _ := service.VerifyGatewayAgentAPIKey(second.AgentAPIKey); !ok {
		t.Fatal("new Agent key does not work")
	}
}

func TestGatewayRemoteReadinessReportsMissingPrerequisites(t *testing.T) {
	service := New(filepath.Join(t.TempDir(), "state.json"))
	readiness, err := service.GatewayRemoteReadiness()
	if err != nil {
		t.Fatal(err)
	}
	if readiness.Ready || len(readiness.Missing) != 3 {
		t.Fatalf("initial readiness = %+v", readiness)
	}
	if _, err := service.SetGatewayAllowedHosts([]string{"*"}); err != nil {
		t.Fatal(err)
	}
	if _, err := service.SetGatewayAdminAPIKey("admin-key-0123456789abcdef"); err != nil {
		t.Fatal(err)
	}
	readiness, err = service.GatewayRemoteReadiness()
	if err != nil {
		t.Fatal(err)
	}
	if readiness.Ready || len(readiness.Missing) != 1 || readiness.Missing[0] != "Agent API Key" {
		t.Fatalf("partial readiness = %+v", readiness)
	}
}
