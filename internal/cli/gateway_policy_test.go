package cli

import (
	"path/filepath"
	"testing"

	"ai-dev-manager-v2/internal/app"
)

func TestGatewayAccessPolicyCLIEnableDisableRetainsRules(t *testing.T) {
	service := app.New(filepath.Join(t.TempDir(), "state.json"))
	_ = captureStdout(t, func() {
		if err := runGatewayAccess(service, []string{"set-policy", "--enabled=true", "--hosts", "adm.example.com", "--ips", "192.168.2.0/24"}); err != nil {
			t.Fatal(err)
		}
	})
	enabled, err := service.GatewayAccessStatus()
	if err != nil || !enabled.WhitelistEnabled || len(enabled.AllowedClientIPs) != 1 {
		t.Fatalf("enable policy: %+v, %v", enabled, err)
	}
	_ = captureStdout(t, func() {
		if err := runGatewayAccess(service, []string{"set-policy", "--enabled=false"}); err != nil {
			t.Fatal(err)
		}
	})
	disabled, err := service.GatewayAccessStatus()
	if err != nil || disabled.WhitelistEnabled || len(disabled.AllowedHosts) != 1 || len(disabled.AllowedClientIPs) != 1 {
		t.Fatalf("disable must retain Host/IP entries: %+v, %v", disabled, err)
	}
}
