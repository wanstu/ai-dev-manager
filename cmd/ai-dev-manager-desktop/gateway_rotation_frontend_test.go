package main

import (
	"io/fs"
	"strings"
	"testing"
)

func TestGatewayRotationFrontendBindings(t *testing.T) {
	assets, err := frontendAssets()
	if err != nil {
		t.Fatal(err)
	}
	index, err := fs.ReadFile(assets, "index.html")
	if err != nil {
		t.Fatal(err)
	}
	for _, required := range []string{
		"gatewayGenerateAdminKey",
		"gatewayCopyAdminKey",
		"gatewayGenerateAgentKey",
		"gatewayCopyAgentKey",
		"轮换 Key",
		"复制新 Key",
		`rel="icon"`,
		"ai-dev-manager-window.png",
	} {
		if !strings.Contains(string(index), required) {
			t.Fatalf("desktop index missing gateway rotation marker %q", required)
		}
	}
	javascript, err := fs.ReadFile(assets, "app.js")
	if err != nil {
		t.Fatal(err)
	}
	for _, required := range []string{
		"RotateGatewayAdminAPIKey",
		"RotateGatewayAgentAPIKey",
		"rotateGatewayAPIKey",
		"Admin API Key 已复制",
		"Agent API Key 已复制",
	} {
		if !strings.Contains(string(javascript), required) {
			t.Fatalf("desktop app.js missing gateway rotation marker %q", required)
		}
	}

	webAdapter, err := fs.ReadFile(assets, "web-adapter.js")
	if err != nil {
		t.Fatal(err)
	}
	for _, required := range []string{
		"result?.admin_api_key",
		"result?.agent_api_key",
		"ConfigureGatewayAdminAPIKey",
	} {
		if !strings.Contains(string(webAdapter), required) {
			t.Fatalf("web adapter missing gateway key compatibility marker %q", required)
		}
	}

	authHTML, err := fs.ReadFile(assets, "web-auth.html")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(authHTML), `rel="icon"`) || !strings.Contains(string(authHTML), "ai-dev-manager-window.png") {
		t.Fatal("web auth page must use the ADM favicon")
	}
}
