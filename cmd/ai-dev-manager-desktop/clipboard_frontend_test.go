package main

import (
	"io/fs"
	"strings"
	"testing"
)

func TestClipboardFrontendUsesNativeRuntimeAndSingleHelper(t *testing.T) {
	assets, err := frontendAssets()
	if err != nil {
		t.Fatal(err)
	}
	javascript, err := fs.ReadFile(assets, "app.js")
	if err != nil {
		t.Fatal(err)
	}
	source := string(javascript)

	for _, required := range []string{
		"async function writeClipboardText(text)",
		"window.runtime?.ClipboardSetText",
		"await writeClipboardText(gatewayDiagnosticsReport())",
		"await writeClipboardText(elements.gatewayAdminAPIKey.value)",
		"await writeClipboardText(elements.gatewayAgentAPIKey.value)",
		"await writeClipboardText(text)",
	} {
		if !strings.Contains(source, required) {
			t.Fatalf("clipboard flow missing marker %q", required)
		}
	}
	if strings.Contains(source, "copyText(") {
		t.Fatal("undefined copyText helper must not be referenced")
	}
}
