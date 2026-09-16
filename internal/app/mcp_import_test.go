package app

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ai-dev-manager-v2/internal/catalog"
)

func TestMCPImportFixtureDetectionAndSanitizedPreview(t *testing.T) {
	tests := []struct {
		name       string
		fixture    string
		format     string
		scope      string
		wantFormat string
		wantCount  int
	}{
		{name: "opencode", fixture: "opencode.jsonc", format: MCPImportAuto, wantFormat: MCPImportOpenCode, wantCount: 2},
		{name: "workbuddy", fixture: "workbuddy.json", format: MCPImportAuto, wantFormat: MCPImportWorkBuddy, wantCount: 2},
		{name: "codex direct", fixture: "codex-plugin.json", format: MCPImportAuto, wantFormat: MCPImportCodexPlugin, wantCount: 2},
		{name: "claude scoped", fixture: "claude-code.json", format: MCPImportAuto, scope: "D:/projects/alpha", wantFormat: MCPImportClaudeCode, wantCount: 1},
		{name: "mcphub", fixture: "mcphub.json", format: MCPImportAuto, wantFormat: MCPImportMCPHub, wantCount: 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := New(filepath.Join(t.TempDir(), "state.json"))
			content := importFixture(t, tt.fixture)
			preview, err := service.PreviewMCPImport(MCPImportInput{Format: tt.format, Content: content, SourceScope: tt.scope})
			if err != nil {
				t.Fatal(err)
			}
			if preview.Format != tt.wantFormat || len(preview.Candidates) != tt.wantCount {
				t.Fatalf("preview format=%q candidates=%d: %+v", preview.Format, len(preview.Candidates), preview)
			}
			for _, candidate := range preview.Candidates {
				if len(candidate.Errors) != 0 {
					t.Fatalf("candidate %q errors=%+v", candidate.Name, candidate.Errors)
				}
			}
			encoded, err := json.Marshal(preview)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(string(encoded), "[REDACTED_SECRET]") {
				t.Fatalf("preview leaked literal credential: %s", encoded)
			}
		})
	}
}

func TestMCPImportPreviewSurfacesUnsupportedSourceExtensions(t *testing.T) {
	service := New(filepath.Join(t.TempDir(), "state.json"))
	preview, err := service.PreviewMCPImport(MCPImportInput{Format: MCPImportWorkBuddy, Content: importFixture(t, "workbuddy.json")})
	if err != nil {
		t.Fatal(err)
	}
	var warnings []MCPImportIssue
	for _, candidate := range preview.Candidates {
		if candidate.Name == "wb-local" {
			warnings = candidate.Warnings
			break
		}
	}
	paths := map[string]bool{}
	for _, warning := range warnings {
		if warning.Kind == "unsupported_source_field" {
			paths[warning.FieldPath] = true
		}
	}
	if !paths["runtime"] || !paths["x-workbuddy"] {
		t.Fatalf("unsupported WorkBuddy extensions were not surfaced: %+v", warnings)
	}
}

func TestMCPImportAutoDetectionIsDeterministic(t *testing.T) {
	service := New(filepath.Join(t.TempDir(), "state.json"))
	preview, err := service.PreviewMCPImport(MCPImportInput{
		Format:  MCPImportAuto,
		Content: `{"mcpServers":{"shared":{"command":"node","args":["server.js"]}}}`,
	})
	if err != nil {
		t.Fatal(err)
	}
	if preview.Format != MCPImportGeneric || len(preview.Candidates) != 1 || preview.Candidates[0].Transport != catalog.MCPTransportStdio {
		t.Fatalf("generic mcpServers auto preview=%+v", preview)
	}

	_, err = service.PreviewMCPImport(MCPImportInput{Format: MCPImportAuto, Content: importFixture(t, "claude-code.json")})
	var importErr *MCPImportError
	if !errors.As(err, &importErr) || importErr.ErrorKind != "source_scope_required" {
		t.Fatalf("Claude multi-project preview error=%v structured=%+v", err, importErr)
	}
}

func TestMCPImportGenericWrapperConvertsLiteralEnvValuesToReferences(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "state.json")
	service := New(statePath)
	content := `{"mcpServers":{"db":{"command":"node","args":["server.js"],"env":{"DB_HOST":"db.example.test","DB_PASSWORD":"example-secret","MAX_ROWS":"1000"},"type":"stdio","owner":"example","options":{"progress":true},"description":"example database"}}}`

	preview, err := service.PreviewMCPImport(MCPImportInput{Format: MCPImportAuto, Content: content})
	if err != nil {
		t.Fatal(err)
	}
	if preview.Format != MCPImportGeneric || len(preview.Candidates) != 1 {
		t.Fatalf("generic preview=%+v", preview)
	}
	candidate := preview.Candidates[0]
	if candidate.Transport != catalog.MCPTransportStdio || len(candidate.ReferenceRequirements) != 3 {
		t.Fatalf("generic candidate=%+v", candidate)
	}
	encoded, err := json.Marshal(preview)
	if err != nil {
		t.Fatal(err)
	}
	for _, literal := range []string{"db.example.test", "example-secret", "1000"} {
		if strings.Contains(string(encoded), literal) {
			t.Fatalf("preview leaked literal env value %q: %s", literal, encoded)
		}
	}

	result, err := service.ApplyMCPImport(MCPImportInput{Format: MCPImportAuto, Content: content})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Result.Mutations) != 1 {
		t.Fatalf("generic apply=%+v", result)
	}
	definition := result.Result.Mutations[0].Definition
	if definition.EnvRefs["DB_PASSWORD"] == "example-secret" || !strings.HasPrefix(definition.EnvRefs["DB_PASSWORD"], "${ADM_MCP_IMPORT_") {
		t.Fatalf("credential was not converted to a reference: %+v", definition.EnvRefs)
	}
	stateBytes, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(stateBytes), "example-secret") || strings.Contains(string(stateBytes), "db.example.test") {
		t.Fatalf("persisted state leaked literal imported env values: %s", stateBytes)
	}
}

func TestMCPImportCredentialConversionAndAtomicConflictPolicies(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "state.json")
	service := New(statePath)
	workspace, err := service.Workspaces.Add(t.TempDir(), "import-test")
	if err != nil {
		t.Fatal(err)
	}
	existing, err := service.MCPs.AddMCP("remote-tools", "https://old.example/mcp", false)
	if err != nil {
		t.Fatal(err)
	}
	environment, err := service.Environments.Create(workspace.ID, "import-test", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.SetEnvironmentMCP(environment.ID, existing.ID, true); err != nil {
		t.Fatal(err)
	}

	content := importFixture(t, "opencode.jsonc")
	_, err = service.ApplyMCPImport(MCPImportInput{
		Format:         MCPImportOpenCode,
		Content:        content,
		SelectedNames:  []string{"local-tools", "remote-tools"},
		ConflictPolicy: catalog.MCPConflictError,
	})
	if err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("conflict error=%v", err)
	}
	definitions, err := service.MCPs.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(definitions) != 1 || definitions[0].ID != existing.ID || definitions[0].Endpoint != "https://old.example/mcp" {
		t.Fatalf("failed atomic import mutated catalog: %+v", definitions)
	}

	result, err := service.ApplyMCPImport(MCPImportInput{
		Format:         MCPImportOpenCode,
		Content:        content,
		SelectedNames:  []string{"remote-tools"},
		ConflictPolicy: catalog.MCPConflictUpdateByName,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Result.Mutations) != 1 || result.Result.Mutations[0].Action != catalog.MCPConflictUpdateByName || result.Result.Mutations[0].Definition.ID != existing.ID {
		t.Fatalf("update result=%+v", result)
	}
	updated, err := service.MCPs.Get(existing.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Endpoint != "https://example.test/mcp" || updated.AuthMode != catalog.MCPAuthHeaders {
		t.Fatalf("updated definition=%+v", updated)
	}
	if got := updated.HeaderRefs["Authorization"]; !strings.HasPrefix(got, "Bearer ${ADM_MCP_IMPORT_REMOTE_TOOLS_HEADERS_AUTHORIZATION}") {
		t.Fatalf("generated authorization reference=%q", got)
	}
	persistedEnvironment, err := service.Environments.Get(environment.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(persistedEnvironment.EnabledMCPIDs) != 1 || persistedEnvironment.EnabledMCPIDs[0] != existing.ID {
		t.Fatalf("update_by_name changed Environment selection: %+v", persistedEnvironment.EnabledMCPIDs)
	}
	stateBytes, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(stateBytes), "[REDACTED_SECRET]") {
		t.Fatalf("persisted state leaked literal credential: %s", stateBytes)
	}
}

func TestMCPImportSelectedBatchRejectsInvalidCandidateWithoutPartialWrite(t *testing.T) {
	service := New(filepath.Join(t.TempDir(), "state.json"))
	content := `{
		"mcpServers": {
			"valid": {"command":"node","args":["server.js"]},
			"invalid": {"type":"sse","url":"http://127.0.0.1/sse"}
		}
	}`
	_, err := service.ApplyMCPImport(MCPImportInput{
		Format:        MCPImportCodexPlugin,
		Content:       content,
		SelectedNames: []string{"valid", "invalid"},
	})
	var importErr *MCPImportError
	if !errors.As(err, &importErr) || importErr.ErrorKind != "invalid_candidate" {
		t.Fatalf("invalid batch error=%v structured=%+v", err, importErr)
	}
	definitions, err := service.MCPs.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(definitions) != 0 {
		t.Fatalf("invalid selected batch partially persisted: %+v", definitions)
	}
}

func TestMCPImportRejectsDuplicateSourceKeys(t *testing.T) {
	service := New(filepath.Join(t.TempDir(), "state.json"))
	_, err := service.PreviewMCPImport(MCPImportInput{
		Format:  MCPImportCodexPlugin,
		Content: `{"mcpServers":{"dup":{"command":"node"},"dup":{"command":"python"}}}`,
	})
	var importErr *MCPImportError
	if !errors.As(err, &importErr) || importErr.ErrorKind != "duplicate_source_key" {
		t.Fatalf("duplicate key error=%v structured=%+v", err, importErr)
	}
}

func TestMCPImportPreviewBlocksCaseInsensitiveDuplicateNames(t *testing.T) {
	service := New(filepath.Join(t.TempDir(), "state.json"))
	content := `{"mcpServers":{"Shared":{"command":"node"},"shared":{"command":"python"}}}`
	preview, err := service.PreviewMCPImport(MCPImportInput{Format: MCPImportCodexPlugin, Content: content})
	if err != nil {
		t.Fatal(err)
	}
	if len(preview.Candidates) != 2 {
		t.Fatalf("preview candidates=%+v", preview.Candidates)
	}
	for _, candidate := range preview.Candidates {
		found := false
		for _, issue := range candidate.Errors {
			if issue.Kind == "duplicate_source_name" {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("candidate %q missing duplicate_source_name error: %+v", candidate.Name, candidate.Errors)
		}
	}
	if _, err := service.ApplyMCPImport(MCPImportInput{Format: MCPImportCodexPlugin, Content: content}); err == nil {
		t.Fatal("duplicate source names unexpectedly applied")
	}
	definitions, err := service.MCPs.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(definitions) != 0 {
		t.Fatalf("duplicate source names partially persisted: %+v", definitions)
	}
}

func TestMCPImportDefaultIncludeIsExplicit(t *testing.T) {
	service := New(filepath.Join(t.TempDir(), "state.json"))
	content := `{"only":{"command":"node"}}`
	result, err := service.ApplyMCPImport(MCPImportInput{Format: MCPImportCodexPlugin, Content: content})
	if err != nil {
		t.Fatal(err)
	}
	if result.Result.Mutations[0].Definition.DefaultIncludeInEnv {
		t.Fatalf("import default_include unexpectedly true: %+v", result)
	}
	service = New(filepath.Join(t.TempDir(), "state.json"))
	result, err = service.ApplyMCPImport(MCPImportInput{Format: MCPImportCodexPlugin, Content: content, DefaultInclude: true})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Result.Mutations[0].Definition.DefaultIncludeInEnv {
		t.Fatalf("explicit default_include was not persisted: %+v", result)
	}
}

func TestExpandEnvironmentTemplateSupportsClaudeStyleFallback(t *testing.T) {
	t.Setenv("ADM_IMPORT_EMPTY", "")
	got, unresolved := expandEnvironmentTemplate("https://${ADM_IMPORT_MISSING:-fallback.example}/mcp/${ADM_IMPORT_EMPTY:-default}")
	if unresolved || got != "https://fallback.example/mcp/default" {
		t.Fatalf("fallback expansion = %q unresolved=%v", got, unresolved)
	}
	if got, unresolved := expandEnvironmentTemplate("${ADM_IMPORT_REALLY_MISSING}"); !unresolved || got != "" {
		t.Fatalf("missing expansion = %q unresolved=%v", got, unresolved)
	}
}

func importFixture(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "mcp_import", name))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
