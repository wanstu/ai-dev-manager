package main

import (
	"io/fs"
	"strings"
	"testing"
)

func TestClipboardFrontendUsesDesktopKitSingleHelper(t *testing.T) {
	assets, err := frontendAssets()
	if err != nil {
		t.Fatal(err)
	}
	index, err := fs.ReadFile(assets, "index.html")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(index), "/desktopkit/runtime.js") {
		t.Fatal("desktop index must load Desktop Kit runtime.js")
	}

	javascript, err := fs.ReadFile(assets, "app.js")
	if err != nil {
		t.Fatal(err)
	}
	source := string(javascript)

	for _, required := range []string{
		"async function writeClipboardText(text)",
		"window.DesktopKit?.clipboard",
		"await clipboard.writeText(String(text ?? ''))",
		"await writeClipboardText(gatewayDiagnosticsReport())",
		"await writeClipboardText(elements.gatewayAdminAPIKey.value)",
		"await writeClipboardText(elements.gatewayAgentAPIKey.value)",
		"await writeClipboardText(text)",
	} {
		if !strings.Contains(source, required) {
			t.Fatalf("clipboard flow missing marker %q", required)
		}
	}
	for _, forbidden := range []string{
		"window.runtime?.ClipboardSetText",
		"navigator.clipboard",
		"document.execCommand",
		"copyText(",
	} {
		if strings.Contains(source, forbidden) {
			t.Fatalf("ADM must delegate clipboard compatibility to Desktop Kit; found %q", forbidden)
		}
	}
}
