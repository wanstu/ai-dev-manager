package cli

import (
	"path/filepath"
	"strings"
	"testing"

	"ai-dev-manager-v2/internal/app"
	"ai-dev-manager-v2/internal/catalog"
)

func TestMCPImportCLIUsesContentAndAtomicApply(t *testing.T) {
	service := app.New(filepath.Join(t.TempDir(), "state.json"))
	content := `{"cli-import":{"command":"node","args":["server.js"]}}`

	preview := captureStdout(t, func() {
		if err := runCatalog("mcp", service, service.MCPs, []string{"import-preview", "--format", app.MCPImportCodexPlugin, "--json-or-jsonc", content}); err != nil {
			t.Fatal(err)
		}
	})
	if !strings.Contains(preview, "cli-import") || !strings.Contains(preview, `"transport": "stdio"`) {
		t.Fatalf("import preview output = %s", preview)
	}

	applied := captureStdout(t, func() {
		if err := runCatalog("mcp", service, service.MCPs, []string{"import-apply", "--format", app.MCPImportCodexPlugin, "--json-or-jsonc", content, "--selected-names", "cli-import"}); err != nil {
			t.Fatal(err)
		}
	})
	if !strings.Contains(applied, "cli-import") || !strings.Contains(applied, `"action": "add"`) {
		t.Fatalf("import apply output = %s", applied)
	}

	definitions, err := service.MCPs.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(definitions) != 1 || definitions[0].Name != "cli-import" || definitions[0].Transport != catalog.MCPTransportStdio {
		t.Fatalf("imported definitions = %+v", definitions)
	}
}
