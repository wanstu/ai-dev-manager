package config

import (
	"fmt"
	"sort"

	"ai-dev-manager/internal/model"
)

// ErrorKind classifies resolver validation failures without forcing callers to
// parse human-readable error strings.
type ErrorKind string

const (
	ErrInvalidScope ErrorKind = "invalid_scope"
	ErrEmptyID      ErrorKind = "empty_id"
	ErrIDMismatch   ErrorKind = "id_mismatch"
)

// ResolveError is a structured configuration-resolution error.
type ResolveError struct {
	Kind     ErrorKind
	Expected model.Scope
	Actual   model.Scope
	Entity   string
	Key      string
	ID       string
}

func (e *ResolveError) Error() string {
	switch e.Kind {
	case ErrInvalidScope:
		return fmt.Sprintf("invalid config scope: expected %q, got %q", e.Expected, e.Actual)
	case ErrEmptyID:
		return fmt.Sprintf("%s definition has an empty id (key %q)", e.Entity, e.Key)
	case ErrIDMismatch:
		return fmt.Sprintf("%s definition id %q does not match map key %q", e.Entity, e.ID, e.Key)
	default:
		return "configuration resolution failed"
	}
}

// Resolve applies the fixed configuration precedence from low to high:
// Global -> Profile -> Project -> Runtime Override.
//
// Optional layers may be nil. Scope placement is validated so callers cannot
// accidentally change precedence by reordering layers.
func Resolve(global model.ConfigLayer, profile, project, runtime *model.ConfigLayer) (model.EffectiveConfig, error) {
	layers := []struct {
		layer    *model.ConfigLayer
		expected model.Scope
	}{
		{layer: &global, expected: model.ScopeGlobal},
		{layer: profile, expected: model.ScopeProfile},
		{layer: project, expected: model.ScopeProject},
		{layer: runtime, expected: model.ScopeRuntime},
	}

	result := model.EffectiveConfig{
		MCPs:      make(map[string]model.ResolvedMCP),
		Skills:    make(map[string]model.ResolvedSkill),
		Verifiers: make(map[string]model.ResolvedVerifier),
	}

	for _, item := range layers {
		if item.layer == nil {
			continue
		}
		if err := validateLayerScope(item.layer.Scope, item.expected); err != nil {
			return model.EffectiveConfig{}, err
		}
		if err := validateDefinitions(*item.layer); err != nil {
			return model.EffectiveConfig{}, err
		}
		applyLayer(&result, *item.layer)
	}

	return result, nil
}

func validateLayerScope(actual, expected model.Scope) error {
	if actual != expected {
		return &ResolveError{Kind: ErrInvalidScope, Expected: expected, Actual: actual}
	}
	return nil
}

func validateDefinitions(layer model.ConfigLayer) error {
	for _, key := range sortedMCPKeys(layer.MCPs) {
		definition := layer.MCPs[key]
		if key == "" || definition.ID == "" {
			return &ResolveError{Kind: ErrEmptyID, Entity: "mcp", Key: key, ID: definition.ID}
		}
		if key != definition.ID {
			return &ResolveError{Kind: ErrIDMismatch, Entity: "mcp", Key: key, ID: definition.ID}
		}
	}

	for _, key := range sortedSkillKeys(layer.Skills) {
		definition := layer.Skills[key]
		if key == "" || definition.ID == "" {
			return &ResolveError{Kind: ErrEmptyID, Entity: "skill", Key: key, ID: definition.ID}
		}
		if key != definition.ID {
			return &ResolveError{Kind: ErrIDMismatch, Entity: "skill", Key: key, ID: definition.ID}
		}
	}

	for _, key := range sortedVerifierKeys(layer.Verifiers) {
		definition := layer.Verifiers[key]
		if key == "" || definition.ID == "" {
			return &ResolveError{Kind: ErrEmptyID, Entity: "verifier", Key: key, ID: definition.ID}
		}
		if key != definition.ID {
			return &ResolveError{Kind: ErrIDMismatch, Entity: "verifier", Key: key, ID: definition.ID}
		}
	}

	return nil
}

func applyLayer(result *model.EffectiveConfig, layer model.ConfigLayer) {
	for _, key := range sortedMCPKeys(layer.MCPs) {
		incoming := layer.MCPs[key]
		current, exists := result.MCPs[key]
		if !exists {
			result.MCPs[key] = newResolvedMCP(incoming, layer.Scope)
			continue
		}
		result.MCPs[key] = mergeMCP(current, incoming, layer.Scope)
	}

	for _, key := range sortedSkillKeys(layer.Skills) {
		incoming := layer.Skills[key]
		current, exists := result.Skills[key]
		if !exists {
			result.Skills[key] = newResolvedSkill(incoming, layer.Scope)
			continue
		}
		result.Skills[key] = mergeSkill(current, incoming, layer.Scope)
	}

	for _, key := range sortedVerifierKeys(layer.Verifiers) {
		incoming := layer.Verifiers[key]
		current, exists := result.Verifiers[key]
		if !exists {
			result.Verifiers[key] = newResolvedVerifier(incoming, layer.Scope)
			continue
		}
		result.Verifiers[key] = mergeVerifier(current, incoming, layer.Scope)
	}

	if layer.Policy != nil {
		result.Policy = &model.ResolvedPolicy{
			Policy: clonePolicy(*layer.Policy),
			Source: layer.Scope,
		}
	}
}

func newResolvedMCP(definition model.MCPDefinition, scope model.Scope) model.ResolvedMCP {
	resolved := model.ResolvedMCP{
		MCPDefinition: cloneMCPDefinition(definition),
		Source:        scope,
	}
	if definition.Enabled != nil {
		resolved.EnabledSource = scope
	}
	return resolved
}

func mergeMCP(current model.ResolvedMCP, incoming model.MCPDefinition, scope model.Scope) model.ResolvedMCP {
	merged := cloneResolvedMCP(current)
	bodyChanged := false

	if incoming.Transport != "" {
		merged.Transport = incoming.Transport
		bodyChanged = true
	}
	if incoming.Command != "" {
		merged.Command = incoming.Command
		bodyChanged = true
	}
	if incoming.URL != "" {
		merged.URL = incoming.URL
		bodyChanged = true
	}
	if incoming.Env != nil {
		if merged.Env == nil {
			merged.Env = make(map[string]string, len(incoming.Env))
		}
		for key, value := range incoming.Env {
			merged.Env[key] = value
		}
		bodyChanged = true
	}
	if incoming.EnvRefs != nil {
		if merged.EnvRefs == nil {
			merged.EnvRefs = make(map[string]string, len(incoming.EnvRefs))
		}
		for key, value := range incoming.EnvRefs {
			merged.EnvRefs[key] = value
		}
		bodyChanged = true
	}
	if incoming.Enabled != nil {
		merged.Enabled = boolPtr(*incoming.Enabled)
		merged.EnabledSource = scope
	}
	if bodyChanged {
		merged.Source = scope
	}

	return merged
}

func newResolvedSkill(definition model.SkillDefinition, scope model.Scope) model.ResolvedSkill {
	resolved := model.ResolvedSkill{
		SkillDefinition: cloneSkillDefinition(definition),
		Source:          scope,
	}
	if definition.Enabled != nil {
		resolved.EnabledSource = scope
	}
	return resolved
}

func mergeSkill(current model.ResolvedSkill, incoming model.SkillDefinition, scope model.Scope) model.ResolvedSkill {
	merged := cloneResolvedSkill(current)
	if incoming.Path != "" {
		merged.Path = incoming.Path
		merged.Source = scope
	}
	if incoming.Enabled != nil {
		merged.Enabled = boolPtr(*incoming.Enabled)
		merged.EnabledSource = scope
	}
	return merged
}

func newResolvedVerifier(definition model.VerifierDefinition, scope model.Scope) model.ResolvedVerifier {
	resolved := model.ResolvedVerifier{
		VerifierDefinition: cloneVerifierDefinition(definition),
		Source:             scope,
	}
	if definition.Enabled != nil {
		resolved.EnabledSource = scope
	}
	return resolved
}

func mergeVerifier(current model.ResolvedVerifier, incoming model.VerifierDefinition, scope model.Scope) model.ResolvedVerifier {
	merged := cloneResolvedVerifier(current)
	bodyChanged := false
	if incoming.Kind != "" {
		merged.Kind = incoming.Kind
		bodyChanged = true
	}
	if incoming.Executable != "" {
		merged.Executable = incoming.Executable
		bodyChanged = true
	}
	if incoming.Args != nil {
		merged.Args = append([]string(nil), incoming.Args...)
		bodyChanged = true
	}
	if incoming.Cwd != "" {
		merged.Cwd = incoming.Cwd
		bodyChanged = true
	}
	if incoming.TimeoutSeconds != 0 {
		merged.TimeoutSeconds = incoming.TimeoutSeconds
		bodyChanged = true
	}
	if incoming.Enabled != nil {
		merged.Enabled = boolPtr(*incoming.Enabled)
		merged.EnabledSource = scope
	}
	if bodyChanged {
		merged.Source = scope
	}
	return merged
}

func cloneVerifierDefinition(source model.VerifierDefinition) model.VerifierDefinition {
	clone := source
	clone.Enabled = cloneBoolPtr(source.Enabled)
	clone.Args = append([]string(nil), source.Args...)
	return clone
}

func cloneResolvedVerifier(source model.ResolvedVerifier) model.ResolvedVerifier {
	clone := source
	clone.VerifierDefinition = cloneVerifierDefinition(source.VerifierDefinition)
	return clone
}

func cloneMCPDefinition(source model.MCPDefinition) model.MCPDefinition {
	clone := source
	clone.Enabled = cloneBoolPtr(source.Enabled)
	clone.Env = cloneStringMap(source.Env)
	clone.EnvRefs = cloneStringMap(source.EnvRefs)
	return clone
}

func cloneResolvedMCP(source model.ResolvedMCP) model.ResolvedMCP {
	clone := source
	clone.MCPDefinition = cloneMCPDefinition(source.MCPDefinition)
	return clone
}

func cloneSkillDefinition(source model.SkillDefinition) model.SkillDefinition {
	clone := source
	clone.Enabled = cloneBoolPtr(source.Enabled)
	return clone
}

func cloneResolvedSkill(source model.ResolvedSkill) model.ResolvedSkill {
	clone := source
	clone.SkillDefinition = cloneSkillDefinition(source.SkillDefinition)
	return clone
}

func clonePolicy(source model.Policy) model.Policy {
	clone := source
	clone.AllowedExecutables = append([]string(nil), source.AllowedExecutables...)
	clone.ToolPaths = cloneStringMap(source.ToolPaths)
	return clone
}

func cloneBoolPtr(source *bool) *bool {
	if source == nil {
		return nil
	}
	return boolPtr(*source)
}

func boolPtr(value bool) *bool {
	copy := value
	return &copy
}

func cloneStringMap(source map[string]string) map[string]string {
	if source == nil {
		return nil
	}
	clone := make(map[string]string, len(source))
	for key, value := range source {
		clone[key] = value
	}
	return clone
}

func sortedMCPKeys(values map[string]model.MCPDefinition) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func sortedSkillKeys(values map[string]model.SkillDefinition) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func sortedVerifierKeys(values map[string]model.VerifierDefinition) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
