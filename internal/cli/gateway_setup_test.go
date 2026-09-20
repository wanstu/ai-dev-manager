package cli

import (
	"path/filepath"
	"strings"
	"testing"

	"ai-dev-manager-v2/internal/app"
)

func TestGatewaySetupRemoteGeneratesOnceAndKeepsExistingKeys(t *testing.T) {
	service := app.New(filepath.Join(t.TempDir(), "state.json"))

	first := captureStdout(t, func() {
		if err := runGatewaySetup(service, []string{"--remote", "--listen", ":8001"}); err != nil {
			t.Fatal(err)
		}
	})
	if !strings.Contains(first, "0.0.0.0:8001") ||
		!strings.Contains(first, "Admin Key：") ||
		!strings.Contains(first, "Agent Key：") {
		t.Fatalf("first setup output missing deployment details:\n%s", first)
	}

	second := captureStdout(t, func() {
		if err := runGatewaySetup(service, []string{"--remote", "--listen", ":8001"}); err != nil {
			t.Fatal(err)
		}
	})
	if strings.Contains(second, "Admin Key：") || strings.Contains(second, "Agent Key：") {
		t.Fatalf("idempotent setup leaked/rotated key unexpectedly:\n%s", second)
	}
	if !strings.Contains(second, "已有双 Key 已保留") {
		t.Fatalf("second setup did not explain preserved keys:\n%s", second)
	}
}

func TestGatewaySetupRemotePreservesHostsUnlessExplicit(t *testing.T) {
	service := app.New(filepath.Join(t.TempDir(), "state.json"))
	if _, err := service.SetupGatewayRemote([]string{"adm.example.com", "10.0.0.8"}, false); err != nil {
		t.Fatal(err)
	}
	before, err := service.GatewayAccessConfig()
	if err != nil {
		t.Fatal(err)
	}

	_ = captureStdout(t, func() {
		if err := runGatewaySetup(service, []string{"--remote", "--listen", ":8001"}); err != nil {
			t.Fatal(err)
		}
	})
	after, err := service.GatewayAccessConfig()
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(after.AllowedHosts, ",") != strings.Join(before.AllowedHosts, ",") {
		t.Fatalf("implicit setup changed hosts from %#v to %#v", before.AllowedHosts, after.AllowedHosts)
	}
	if after.AdminAPIKeyHash != before.AdminAPIKeyHash || after.AgentAPIKeyHash != before.AgentAPIKeyHash {
		t.Fatal("implicit setup unexpectedly rotated an existing API key")
	}

	_ = captureStdout(t, func() {
		if err := runGatewaySetup(service, []string{"--remote", "--listen", ":8001", "--hosts", "*"}); err != nil {
			t.Fatal(err)
		}
	})
	explicit, err := service.GatewayAccessConfig()
	if err != nil {
		t.Fatal(err)
	}
	if len(explicit.AllowedHosts) != 1 || explicit.AllowedHosts[0] != "*" {
		t.Fatalf("explicit --hosts did not replace host policy: %#v", explicit.AllowedHosts)
	}
}

func TestValidateGatewayStartReadinessExplainsMissingItems(t *testing.T) {
	service := app.New(filepath.Join(t.TempDir(), "state.json"))
	err := validateGatewayStartReadiness(service, "0.0.0.0:8001")
	if err == nil {
		t.Fatal("remote start without access configuration unexpectedly passed")
	}
	message := err.Error()
	for _, want := range []string{"✗ Host Policy", "✗ Admin API Key", "✗ Agent API Key", "adm gateway setup --remote"} {
		if !strings.Contains(message, want) {
			t.Fatalf("readiness error missing %q:\n%s", want, message)
		}
	}
	if err := validateGatewayStartReadiness(service, "127.0.0.1:8001"); err != nil {
		t.Fatalf("loopback start should not require remote access setup: %v", err)
	}
}

func TestClientAdminAPIKeyPrefersNewVariableAndKeepsLegacyFallback(t *testing.T) {
	t.Setenv(admAdminAPIKeyEnv, "new-client-key")
	t.Setenv(admLegacyAdminAPIKeyEnv, "legacy-client-key")
	if got := clientAdminAPIKey(); got != "new-client-key" {
		t.Fatalf("clientAdminAPIKey() = %q, want new-client-key", got)
	}

	t.Setenv(admAdminAPIKeyEnv, "")
	if got := clientAdminAPIKey(); got != "legacy-client-key" {
		t.Fatalf("legacy fallback = %q, want legacy-client-key", got)
	}
}
