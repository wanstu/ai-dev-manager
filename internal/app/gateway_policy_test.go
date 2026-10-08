package app

import (
	"path/filepath"
	"testing"

	"ai-dev-manager-v2/internal/model"
)

func TestGatewayLegacyHostPolicyRemainsActiveAfterUpgrade(t *testing.T) {
	svc := New(filepath.Join(t.TempDir(), "state.json"))
	if err := svc.Store.Update(func(state *model.State) error {
		state.GatewayAccess.AllowedHosts = []string{"adm.example.test"}
		// Legacy saved data does not have WhitelistEnabled.
		state.GatewayAccess.WhitelistEnabled = nil
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	status, err := svc.GatewayAccessStatus()
	if err != nil {
		t.Fatal(err)
	}
	if !status.WhitelistEnabled || len(status.AllowedHosts) != 1 {
		t.Fatalf("legacy whitelist unexpectedly disabled: %+v", status)
	}
	if _, err := svc.SetupGatewayRemote(nil, false); err != nil {
		t.Fatal(err)
	}
	status, err = svc.GatewayAccessStatus()
	if err != nil || !status.WhitelistEnabled {
		t.Fatalf("setup disabled old restrictions: %+v, %v", status, err)
	}
	if _, err := svc.SetGatewayAccessPolicy(false, []string{"adm.example.test"}, nil); err != nil {
		t.Fatal(err)
	}
	status, _ = svc.GatewayAccessStatus()
	if status.WhitelistEnabled {
		t.Fatal("explicitly disabling policy should work")
	}
}

func TestGatewayWhitelistSourceOnlyAndIPv6(t *testing.T) {
	svc := New(filepath.Join(t.TempDir(), "state.json"))
	status, err := svc.SetGatewayAccessPolicy(true, nil, []string{"2001:db8::1", "2001:db8:1::/64", "192.168.5.0/24"})
	if err != nil {
		t.Fatal(err)
	}
	if !status.WhitelistEnabled || len(status.AllowedClientIPs) != 3 {
		t.Fatalf("source-only settings = %+v", status)
	}
	if _, err = svc.SetGatewayAccessPolicy(true, []string{"*"}, nil); err == nil {
		t.Fatal("wildcard without a source restriction should not enable an ineffective allowlist")
	}
}
