package mcp

import (
	"sort"

	"ai-dev-manager/internal/model"
)

type Health string

const HealthUnprobed Health = "unprobed"

// Entry is a stable catalog view of one resolved MCP definition. Health and
// Capabilities are runtime-facing extension points only; Phase 3 never probes
// or starts MCP processes.
type Entry struct {
	Definition    model.MCPDefinition
	Source        model.Scope
	EnabledSource model.Scope
	Health        Health
	Capabilities  []string
}

// Registry is a read-only catalog built from EffectiveConfig.
type Registry struct {
	entries map[string]Entry
}

func FromEffective(cfg model.EffectiveConfig) *Registry {
	registry := &Registry{entries: make(map[string]Entry, len(cfg.MCPs))}
	for id, resolved := range cfg.MCPs {
		registry.entries[id] = Entry{
			Definition:    cloneDefinition(resolved.MCPDefinition),
			Source:        resolved.Source,
			EnabledSource: resolved.EnabledSource,
			Health:        HealthUnprobed,
			Capabilities:  []string{},
		}
	}
	return registry
}

func (r *Registry) Get(id string) (Entry, bool) {
	if r == nil {
		return Entry{}, false
	}
	entry, ok := r.entries[id]
	if !ok {
		return Entry{}, false
	}
	return cloneEntry(entry), true
}

func (r *Registry) List() []Entry {
	if r == nil {
		return []Entry{}
	}
	ids := make([]string, 0, len(r.entries))
	for id := range r.entries {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	entries := make([]Entry, 0, len(ids))
	for _, id := range ids {
		entries = append(entries, cloneEntry(r.entries[id]))
	}
	return entries
}

// Enabled returns only MCPs that are explicitly enabled. A nil Enabled value
// remains undecided at this layer and is intentionally not treated as an
// implicit runtime default in Phase 3.
func (r *Registry) Enabled() []Entry {
	all := r.List()
	enabled := make([]Entry, 0, len(all))
	for _, entry := range all {
		if entry.Definition.Enabled != nil && *entry.Definition.Enabled {
			enabled = append(enabled, entry)
		}
	}
	return enabled
}

func cloneEntry(source Entry) Entry {
	clone := source
	clone.Definition = cloneDefinition(source.Definition)
	clone.Capabilities = append([]string(nil), source.Capabilities...)
	return clone
}

func cloneDefinition(source model.MCPDefinition) model.MCPDefinition {
	clone := source
	clone.Enabled = cloneBool(source.Enabled)
	clone.Env = cloneMap(source.Env)
	clone.EnvRefs = cloneMap(source.EnvRefs)
	return clone
}

func cloneBool(source *bool) *bool {
	if source == nil {
		return nil
	}
	value := *source
	return &value
}

func cloneMap(source map[string]string) map[string]string {
	if source == nil {
		return nil
	}
	clone := make(map[string]string, len(source))
	for key, value := range source {
		clone[key] = value
	}
	return clone
}
