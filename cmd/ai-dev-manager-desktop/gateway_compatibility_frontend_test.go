package main

import (
	"io/fs"
	"strings"
	"testing"
)

func TestGatewayCompatibilityFrontendKeepsOnlyForceStopAvailable(t *testing.T) {
	assets, err := frontendAssets()
	if err != nil {
		t.Fatal(err)
	}
	index, err := fs.ReadFile(assets, "index.html")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(index), "强制停止 ADM") {
		t.Fatal("Desktop must expose force-stop action for incompatible ADM Gateways")
	}

	javascript, err := fs.ReadFile(assets, "app.js")
	if err != nil {
		t.Fatal(err)
	}
	for _, required := range []string{
		"recognized_adm_gateway",
		"elements.gatewayStopButton.disabled = !recognized || state === 'stopped' || state === 'unknown'",
		"ADM 版本/管理协议不兼容；已禁止管理操作，仅允许强制停止 Gateway。",
		"if (status?.state === 'running') await refreshSnapshot()",
		"runGatewayAction('强制停止 ADM'",
	} {
		if !strings.Contains(string(javascript), required) {
			t.Fatalf("Desktop compatibility flow missing marker %q", required)
		}
	}
}
