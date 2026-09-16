package catalog_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ai-dev-manager-v2/internal/catalog"
	"ai-dev-manager-v2/internal/model"
	"ai-dev-manager-v2/internal/pathutil"
	"ai-dev-manager-v2/internal/store"
)

func TestConfiguredCatalogEntriesRequireRuntimeSources(t *testing.T) {
	state := store.New(filepath.Join(t.TempDir(), "state.json"))
	mcps := catalog.NewMCP(state)
	skills := catalog.New(state, catalog.KindSkill)

	for _, endpoint := range []string{"", "filesystem", "ftp://example.test/mcp"} {
		if _, err := mcps.AddMCP("invalid-"+strings.ReplaceAll(endpoint, ":", "-"), endpoint, false); err == nil || !strings.Contains(err.Error(), "valid http(s) URL") {
			t.Fatalf("AddMCP(%q) error = %v", endpoint, err)
		}
	}
	mcpEntry, err := mcps.AddMCP("external", "http://127.0.0.1:9000/mcp", true)
	if err != nil {
		t.Fatal(err)
	}
	if mcpEntry.Endpoint != "http://127.0.0.1:9000/mcp" || mcpEntry.Transport != catalog.MCPTransportStreamableHTTP || mcpEntry.AuthMode != catalog.MCPAuthNone || !mcpEntry.DefaultIncludeInEnv {
		t.Fatalf("configured MCP = %+v", mcpEntry)
	}

	if entries, err := skills.List(); err != nil || len(entries) != 0 {
		t.Fatalf("Skill catalog must not scan host paths before explicit root configuration: entries=%+v err=%v", entries, err)
	}
	skillRoot := filepath.Join(t.TempDir(), "skills")
	supportRoot := filepath.Join(t.TempDir(), "gsd-core")
	if err := os.MkdirAll(filepath.Join(skillRoot, "gsd-next"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(supportRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	artifact := filepath.Join(skillRoot, "gsd-next", "SKILL.md")
	if err := os.WriteFile(artifact, []byte("# gsd-next\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	discovered, err := skills.AddSkillRoot(skillRoot, []string{supportRoot}, true)
	if err != nil || len(discovered) != 1 {
		t.Fatalf("AddSkillRoot entries=%+v err=%v", discovered, err)
	}
	entry := discovered[0]
	if entry.SourceID == "" || entry.RelativeArtifactPath != "gsd-next/skill.md" || entry.Name != "gsd-next" || !pathutil.Same(entry.ArtifactPath, artifact) || !pathutil.Same(entry.SourceRoot, skillRoot) || !entry.DefaultIncludeInEnv {
		t.Fatalf("configured Skill = %+v", entry)
	}
	if len(entry.SupportRoots) != 1 || !pathutil.Same(entry.SupportRoots[0], supportRoot) {
		t.Fatalf("configured Skill support roots = %+v", entry.SupportRoots)
	}
}

func TestMCPDefinitionsAreTypedAndCallerOwnedCollectionsAreCloned(t *testing.T) {
	state := store.New(filepath.Join(t.TempDir(), "state.json"))
	mcps := catalog.NewMCP(state)
	headerRefs := map[string]string{"X-Test": "${ADM_TEST_REF}"}

	httpEntry, err := mcps.AddMCPConfig("external", catalog.MCPConfig{
		Endpoint:       "http://${ADM_HOST_REF}/mcp",
		Transport:      catalog.MCPTransportStreamableHTTP,
		AuthMode:       catalog.MCPAuthHeaders,
		HeaderRefs:     headerRefs,
		DefaultInclude: true,
		HealthPolicy: model.MCPHealthPolicy{
			HealthCheckEnabled:   true,
			CheckIntervalSeconds: 15,
			ProbeTimeoutSeconds:  3,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if httpEntry.Transport != catalog.MCPTransportStreamableHTTP || httpEntry.AuthMode != catalog.MCPAuthHeaders || httpEntry.Endpoint != "http://${ADM_HOST_REF}/mcp" || !httpEntry.DefaultIncludeInEnv {
		t.Fatalf("configured HTTP MCP = %+v", httpEntry)
	}
	if httpEntry.HealthPolicy.AutoReconnect {
		t.Fatalf("auto_reconnect must default false: %+v", httpEntry.HealthPolicy)
	}

	headerRefs["X-Test"] = "changed-by-caller"
	persistedHTTP, err := mcps.Get(httpEntry.ID)
	if err != nil {
		t.Fatal(err)
	}
	if persistedHTTP.HeaderRefs["X-Test"] != "${ADM_TEST_REF}" {
		t.Fatalf("persisted header refs were mutated through caller map: %+v", persistedHTTP.HeaderRefs)
	}

	args := []string{"serve", "--stdio"}
	envRefs := map[string]string{"VALUE": "${ADM_STDIO_REF}"}
	stdioEntry, err := mcps.AddMCPConfig("local", catalog.MCPConfig{
		Transport:  catalog.MCPTransportStdio,
		AuthMode:   catalog.MCPAuthNone,
		Executable: "mcp-helper",
		Args:       args,
		EnvRefs:    envRefs,
		HealthPolicy: model.MCPHealthPolicy{
			AutoReconnect:            true,
			ReconnectIntervalSeconds: 7,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if stdioEntry.Transport != catalog.MCPTransportStdio || stdioEntry.Executable != "mcp-helper" || len(stdioEntry.Args) != 2 || stdioEntry.EnvRefs["VALUE"] != "${ADM_STDIO_REF}" {
		t.Fatalf("configured stdio MCP = %+v", stdioEntry)
	}
	args[0] = "changed-by-caller"
	envRefs["VALUE"] = "changed-by-caller"
	persistedStdio, err := mcps.Get(stdioEntry.ID)
	if err != nil {
		t.Fatal(err)
	}
	if persistedStdio.Args[0] != "serve" || persistedStdio.EnvRefs["VALUE"] != "${ADM_STDIO_REF}" {
		t.Fatalf("persisted stdio collections were mutated: %+v", persistedStdio)
	}
}

func TestMCPUpdatePreservesIdentityAndRevalidatesConfiguration(t *testing.T) {
	state := store.New(filepath.Join(t.TempDir(), "state.json"))
	mcps := catalog.NewMCP(state)
	entry, err := mcps.AddMCPConfig("external", catalog.MCPConfig{
		Endpoint:  "http://127.0.0.1:9001/mcp",
		Transport: catalog.MCPTransportStreamableHTTP,
		AuthMode:  catalog.MCPAuthNone,
	})
	if err != nil {
		t.Fatal(err)
	}
	updated, err := mcps.UpdateMCPConfig(entry.ID, "external-renamed", catalog.MCPConfig{
		Transport:  catalog.MCPTransportStdio,
		AuthMode:   catalog.MCPAuthNone,
		Executable: "mcp-helper",
		HealthPolicy: model.MCPHealthPolicy{
			HealthCheckEnabled:       true,
			CheckIntervalSeconds:     9,
			ProbeTimeoutSeconds:      2,
			AutoReconnect:            true,
			ReconnectIntervalSeconds: 4,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if updated.ID != entry.ID || updated.Name != "external-renamed" || updated.Transport != catalog.MCPTransportStdio || updated.Executable != "mcp-helper" || updated.HealthPolicy.CheckIntervalSeconds != 9 {
		t.Fatalf("updated MCP = %+v", updated)
	}
	if _, err := mcps.UpdateMCPConfig(entry.ID, "external-renamed", catalog.MCPConfig{Transport: catalog.MCPTransportStdio}); err == nil || !strings.Contains(err.Error(), "executable is required") {
		t.Fatalf("invalid update error = %v", err)
	}
	persisted, err := mcps.Get(entry.ID)
	if err != nil {
		t.Fatal(err)
	}
	if persisted.Executable != "mcp-helper" || persisted.HealthPolicy.CheckIntervalSeconds != 9 {
		t.Fatalf("invalid update mutated persisted definition: %+v", persisted)
	}
}

func TestMCPDefinitionValidationIsTransportLocal(t *testing.T) {
	state := store.New(filepath.Join(t.TempDir(), "state.json"))
	mcps := catalog.NewMCP(state)

	tests := []struct {
		name   string
		config catalog.MCPConfig
		want   string
	}{
		{name: "unsupported transport", config: catalog.MCPConfig{Transport: "sse", Endpoint: "http://127.0.0.1/mcp"}, want: "unsupported mcp transport"},
		{name: "unsupported auth", config: catalog.MCPConfig{Transport: catalog.MCPTransportStreamableHTTP, AuthMode: "oauth", Endpoint: "http://127.0.0.1/mcp"}, want: "unsupported mcp auth_mode"},
		{name: "headers require refs", config: catalog.MCPConfig{Transport: catalog.MCPTransportStreamableHTTP, AuthMode: catalog.MCPAuthHeaders, Endpoint: "http://127.0.0.1/mcp"}, want: "requires header_refs"},
		{name: "literal header rejected", config: catalog.MCPConfig{Transport: catalog.MCPTransportStreamableHTTP, AuthMode: catalog.MCPAuthHeaders, Endpoint: "http://127.0.0.1/mcp", HeaderRefs: map[string]string{"X-Test": "plain-placeholder"}}, want: "must use an environment reference"},
		{name: "none forbids headers", config: catalog.MCPConfig{Transport: catalog.MCPTransportStreamableHTTP, AuthMode: catalog.MCPAuthNone, Endpoint: "http://127.0.0.1/mcp", HeaderRefs: map[string]string{"X-Test": "${ADM_TEST_REF}"}}, want: "Secret-backed headers"},
		{name: "http forbids stdio", config: catalog.MCPConfig{Transport: catalog.MCPTransportStreamableHTTP, Endpoint: "http://127.0.0.1/mcp", Executable: "helper"}, want: "cannot configure stdio"},
		{name: "stdio requires executable", config: catalog.MCPConfig{Transport: catalog.MCPTransportStdio}, want: "executable is required"},
		{name: "stdio forbids endpoint", config: catalog.MCPConfig{Transport: catalog.MCPTransportStdio, Executable: "helper", Endpoint: "http://127.0.0.1/mcp"}, want: "cannot configure endpoint"},
		{name: "stdio env must reference", config: catalog.MCPConfig{Transport: catalog.MCPTransportStdio, Executable: "helper", EnvRefs: map[string]string{"VALUE": "plain-placeholder"}}, want: "must use an environment reference"},
		{name: "health interval required", config: catalog.MCPConfig{Endpoint: "http://127.0.0.1/mcp", HealthPolicy: model.MCPHealthPolicy{HealthCheckEnabled: true, ProbeTimeoutSeconds: 2}}, want: "check_interval_seconds"},
		{name: "probe timeout required", config: catalog.MCPConfig{Endpoint: "http://127.0.0.1/mcp", HealthPolicy: model.MCPHealthPolicy{HealthCheckEnabled: true, CheckIntervalSeconds: 2}}, want: "probe_timeout_seconds"},
		{name: "reconnect interval required", config: catalog.MCPConfig{Endpoint: "http://127.0.0.1/mcp", HealthPolicy: model.MCPHealthPolicy{AutoReconnect: true}}, want: "reconnect_interval_seconds"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := mcps.AddMCPConfig(tt.name, tt.config); err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("AddMCPConfig error = %v; want containing %q", err, tt.want)
			}
		})
	}
}

func TestMCPConfigDefaultsToStreamableHTTPNoneAuth(t *testing.T) {
	state := store.New(filepath.Join(t.TempDir(), "state.json"))
	mcps := catalog.NewMCP(state)
	entry, err := mcps.AddMCPConfig("external", catalog.MCPConfig{Endpoint: "http://127.0.0.1:9001/mcp"})
	if err != nil {
		t.Fatal(err)
	}
	if entry.Transport != catalog.MCPTransportStreamableHTTP || entry.AuthMode != catalog.MCPAuthNone || entry.HealthPolicy.AutoReconnect {
		t.Fatalf("default MCP definition = %+v", entry)
	}
}
