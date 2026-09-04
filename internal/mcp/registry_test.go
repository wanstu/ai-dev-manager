package mcp

import (
	"reflect"
	"testing"

	"ai-dev-manager/internal/model"
)

func TestRegistryPreservesDisabledEntriesAndSourceTrace(t *testing.T) {
	cfg := model.EffectiveConfig{
		MCPs: map[string]model.ResolvedMCP{
			"github": {
				MCPDefinition: model.MCPDefinition{ID: "github", Enabled: mcpBool(false), Command: "github-mcp"},
				Source:        model.ScopeGlobal,
				EnabledSource: model.ScopeProject,
			},
		},
	}
	registry := FromEffective(cfg)
	entry, ok := registry.Get("github")
	if !ok {
		t.Fatal("github entry missing")
	}
	if entry.Health != HealthUnprobed {
		t.Fatalf("health = %q, want %q", entry.Health, HealthUnprobed)
	}
	if entry.Source != model.ScopeGlobal || entry.EnabledSource != model.ScopeProject {
		t.Fatalf("trace lost: %+v", entry)
	}
	if entry.Definition.Enabled == nil || *entry.Definition.Enabled {
		t.Fatalf("disabled state lost: %+v", entry.Definition)
	}
	if len(registry.Enabled()) != 0 {
		t.Fatalf("disabled MCP should not appear in Enabled(): %+v", registry.Enabled())
	}
}

func TestRegistryListAndEnabledAreDeterministic(t *testing.T) {
	cfg := model.EffectiveConfig{MCPs: map[string]model.ResolvedMCP{
		"zeta":   {MCPDefinition: model.MCPDefinition{ID: "zeta", Enabled: mcpBool(true)}},
		"alpha":  {MCPDefinition: model.MCPDefinition{ID: "alpha", Enabled: mcpBool(true)}},
		"middle": {MCPDefinition: model.MCPDefinition{ID: "middle"}},
	}}
	registry := FromEffective(cfg)

	wantList := []string{"alpha", "middle", "zeta"}
	wantEnabled := []string{"alpha", "zeta"}
	for i := 0; i < 20; i++ {
		if got := entryIDs(registry.List()); !reflect.DeepEqual(got, wantList) {
			t.Fatalf("List() = %+v, want %+v", got, wantList)
		}
		if got := entryIDs(registry.Enabled()); !reflect.DeepEqual(got, wantEnabled) {
			t.Fatalf("Enabled() = %+v, want %+v", got, wantEnabled)
		}
	}
}

func TestRegistryDoesNotAliasEffectiveConfigOrReturnedEntries(t *testing.T) {
	cfg := model.EffectiveConfig{MCPs: map[string]model.ResolvedMCP{
		"github": {
			MCPDefinition: model.MCPDefinition{
				ID:      "github",
				Enabled: mcpBool(true),
				Env:     map[string]string{"MODE": "shared"},
				EnvRefs: map[string]string{"SERVICE_CREDENTIAL": "REFERENCE_NAME"},
			},
		},
	}}
	registry := FromEffective(cfg)

	entry, ok := registry.Get("github")
	if !ok {
		t.Fatal("github entry missing")
	}
	entry.Definition.Env["MODE"] = "changed"
	entry.Definition.EnvRefs["SERVICE_CREDENTIAL"] = "CHANGED_REFERENCE"
	*entry.Definition.Enabled = false

	original := cfg.MCPs["github"]
	if original.Env["MODE"] != "shared" || original.EnvRefs["SERVICE_CREDENTIAL"] != "REFERENCE_NAME" || original.Enabled == nil || !*original.Enabled {
		t.Fatalf("registry aliases EffectiveConfig: %+v", original)
	}

	again, _ := registry.Get("github")
	if again.Definition.Env["MODE"] != "shared" || again.Definition.EnvRefs["SERVICE_CREDENTIAL"] != "REFERENCE_NAME" || again.Definition.Enabled == nil || !*again.Definition.Enabled {
		t.Fatalf("Get() result aliases registry state: %+v", again)
	}
}

func entryIDs(entries []Entry) []string {
	ids := make([]string, 0, len(entries))
	for _, entry := range entries {
		ids = append(ids, entry.Definition.ID)
	}
	return ids
}

func mcpBool(value bool) *bool {
	copy := value
	return &copy
}
