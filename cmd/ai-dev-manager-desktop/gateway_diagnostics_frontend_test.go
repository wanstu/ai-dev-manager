package main

import (
	"io/fs"
	"strings"
	"testing"
)

func TestGatewayDiagnosticsFrontendBindings(t *testing.T) {
	assets, err := frontendAssets()
	if err != nil {
		t.Fatal(err)
	}
	index, err := fs.ReadFile(assets, "index.html")
	if err != nil {
		t.Fatal(err)
	}
	for _, required := range []string{
		"gatewayDiagnosticsReadiness",
		"gatewayDiagnosticsCopyButton",
		"gatewayDiagnosticStatePath",
		"gatewayDiagnosticService",
		"gatewayDiagnosticSuggestion",
		"复制诊断报告",
	} {
		if !strings.Contains(string(index), required) {
			t.Fatalf("desktop index missing gateway diagnostics marker %q", required)
		}
	}

	javascript, err := fs.ReadFile(assets, "app.js")
	if err != nil {
		t.Fatal(err)
	}
	for _, required := range []string{
		"GetGatewayDiagnostics",
		"renderGatewayDiagnostics",
		"gatewayDiagnosticSuggestedActions",
		"gatewayDiagnosticsReport",
		"Gateway 诊断报告已复制",
	} {
		if !strings.Contains(string(javascript), required) {
			t.Fatalf("desktop app.js missing gateway diagnostics marker %q", required)
		}
	}
}
