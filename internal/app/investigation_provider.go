package app

import (
	"context"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"ai-dev-manager-v2/internal/model"
)

const (
	CapabilityKindCodeIntelligenceProvider = "code_intelligence_provider"
	InvestigationProviderGitNexus          = "gitnexus"
	InvestigationProviderGitNexusKey       = "code_intelligence.gitnexus"
)

var gitNexusReadOnlyTools = []string{"context", "detect_changes", "impact", "list_repos", "query"}

// InvestigationProvider describes one optional external source of code
// intelligence. Providers are discovered from existing Environment-authorized
// capabilities; this abstraction does not create a second authorization model.
type InvestigationProvider interface {
	ID() string
	CapabilityKey() string
	MatchMCP(model.MCPDefinition) bool
	ReadOnlyTools() []string
}

type gitNexusInvestigationProvider struct{}

func (gitNexusInvestigationProvider) ID() string            { return InvestigationProviderGitNexus }
func (gitNexusInvestigationProvider) CapabilityKey() string { return InvestigationProviderGitNexusKey }
func (gitNexusInvestigationProvider) ReadOnlyTools() []string {
	return append([]string(nil), gitNexusReadOnlyTools...)
}

func (gitNexusInvestigationProvider) MatchMCP(entry model.MCPDefinition) bool {
	if compactProviderToken(entry.Name) == "gitnexus" {
		return true
	}
	parts := append([]string{filepath.Base(strings.TrimSpace(entry.Executable))}, entry.Args...)
	joined := strings.ToLower(strings.Join(parts, " "))
	return strings.Contains(joined, "gitnexus") && strings.Contains(joined, "mcp")
}

func investigationProviders() []InvestigationProvider {
	return []InvestigationProvider{gitNexusInvestigationProvider{}}
}

// InvestigationProviderReadOnlyTools returns the provider capabilities ADM may
// consume as investigation evidence. Mutating provider tools are intentionally
// excluded.
func InvestigationProviderReadOnlyTools(providerID string) []string {
	for _, provider := range investigationProviders() {
		if provider.ID() == providerID {
			return provider.ReadOnlyTools()
		}
	}
	return nil
}

// InvestigationProviderReport returns provider facts only. It is passive: it
// does not connect to MCPs, refresh inventories, invoke provider tools, index a
// repository, run verifiers, or mutate project state.
func (s *Service) InvestigationProviderReport(ctx context.Context, environmentID string) (model.InvestigationProviderReport, error) {
	report, err := s.EnvironmentCapabilityReport(ctx, environmentID)
	if err != nil {
		return model.InvestigationProviderReport{}, err
	}
	return InvestigationProviderReportFromCapabilityReport(report), nil
}

func InvestigationProviderReportFromCapabilityReport(report model.CapabilityReport) model.InvestigationProviderReport {
	providers := make([]model.CapabilityFact, 0)
	for _, fact := range report.Facts {
		if fact.Kind == CapabilityKindCodeIntelligenceProvider {
			providers = append(providers, fact)
		}
	}
	return model.InvestigationProviderReport{
		EnvironmentID: report.EnvironmentID,
		GeneratedAt:   report.GeneratedAt,
		Providers:     providers,
	}
}

func (s *Service) investigationProviderCapabilityFacts(env model.Environment, ws model.Workspace, mcpFacts []model.CapabilityFact) []model.CapabilityFact {
	entries, err := s.MCPs.List()
	if err != nil {
		return []model.CapabilityFact{investigationProviderFact(
			InvestigationProviderGitNexusKey,
			model.CapabilityStateUnavailable,
			"provider_catalog_unavailable",
			sanitizeCapabilityMessage(err.Error()),
			nil,
			"none",
			"unknown",
			[]string{"provider_catalog_unavailable", "static_fallback_available"},
		)}
	}

	mcpByKey := make(map[string]model.CapabilityFact, len(mcpFacts))
	for _, fact := range mcpFacts {
		mcpByKey[fact.Key] = fact
	}
	enabled := make(map[string]bool, len(env.EnabledMCPIDs))
	for _, id := range env.EnabledMCPIDs {
		enabled[id] = true
	}

	providers := investigationProviders()
	facts := make([]model.CapabilityFact, 0, len(providers))
	for _, provider := range providers {
		matches := make([]model.MCPDefinition, 0)
		for _, entry := range entries {
			if provider.MatchMCP(entry) {
				matches = append(matches, entry)
			}
		}
		if len(matches) == 0 {
			facts = append(facts, investigationProviderFact(
				provider.CapabilityKey(),
				model.CapabilityStateUnconfigured,
				"provider_not_configured",
				"Optional code intelligence provider is not configured for this Environment; built-in static investigation remains available.",
				[]model.CapabilityEvidence{providerEvidence(env, ws, provider, model.MCPDefinition{}, false, 0)},
				"none",
				"unknown",
				[]string{"provider_not_configured", "static_fallback_available"},
			))
			continue
		}

		sort.SliceStable(matches, func(i, j int) bool {
			if enabled[matches[i].ID] != enabled[matches[j].ID] {
				return enabled[matches[i].ID]
			}
			return strings.ToLower(matches[i].ID) < strings.ToLower(matches[j].ID)
		})
		selected := matches[0]
		evidence := []model.CapabilityEvidence{providerEvidence(env, ws, provider, selected, enabled[selected.ID], len(matches))}
		uncertainties := []string{"static_fallback_available"}
		if len(matches) > 1 {
			uncertainties = append(uncertainties, "multiple_provider_bindings")
		}

		if !enabled[selected.ID] {
			facts = append(facts, investigationProviderFact(
				provider.CapabilityKey(),
				model.CapabilityStateDisabled,
				"provider_mcp_disabled",
				"A matching provider MCP exists but is not enabled for this Environment.",
				evidence,
				"none",
				"unknown",
				append(uncertainties, "provider_not_authorized_for_environment"),
			))
			continue
		}

		mcpFact, ok := mcpByKey["mcp/"+selected.ID]
		if !ok {
			facts = append(facts, investigationProviderFact(
				provider.CapabilityKey(),
				model.CapabilityStateUnavailable,
				"provider_mcp_fact_missing",
				"Provider MCP authorization is selected, but its capability fact is unavailable.",
				evidence,
				"none",
				"unknown",
				append(uncertainties, "provider_mcp_fact_missing"),
			))
			continue
		}
		if mcpFact.State != model.CapabilityStateAvailable {
			facts = append(facts, investigationProviderFact(
				provider.CapabilityKey(),
				mcpFact.State,
				mcpFact.ReasonCode,
				mcpFact.Message,
				evidence,
				"none",
				"unknown",
				append(uncertainties, "provider_unavailable"),
			))
			continue
		}

		facts = append(facts, investigationProviderFact(
			provider.CapabilityKey(),
			model.CapabilityStateDegraded,
			"provider_runtime_not_observed",
			"Provider MCP is configured and Environment-authorized, but application-level passive inspection does not connect or inspect its runtime inventory.",
			evidence,
			"low",
			"unknown",
			append(uncertainties, "provider_health_not_observed", "provider_inventory_not_observed", "provider_index_freshness_not_observed"),
		))
	}
	return facts
}

func investigationProviderFact(key string, state model.CapabilityState, reason, message string, evidence []model.CapabilityEvidence, confidence, freshness string, uncertainties []string) model.CapabilityFact {
	return model.CapabilityFact{
		Key:           key,
		Kind:          CapabilityKindCodeIntelligenceProvider,
		State:         state,
		ReasonCode:    strings.TrimSpace(reason),
		Message:       sanitizeCapabilityMessage(message),
		Evidence:      evidence,
		Source:        capabilitySourceStatic,
		Confidence:    confidence,
		Freshness:     freshness,
		Uncertainties: uniqueStrings(uncertainties),
	}
}

func providerEvidence(env model.Environment, ws model.Workspace, provider InvestigationProvider, entry model.MCPDefinition, enabled bool, matchCount int) model.CapabilityEvidence {
	details := map[string]string{
		"provider":                 provider.ID(),
		"authorization":            "environment_mcp_selection",
		"environment_id":           env.ID,
		"environment_root":         env.Root,
		"workspace_id":             ws.ID,
		"enabled":                  strconv.FormatBool(enabled),
		"mcp_id":                   entry.ID,
		"mcp_name":                 entry.Name,
		"mcp_transport":            entry.Transport,
		"matching_binding_count":   strconv.Itoa(matchCount),
		"read_only_provider_tools": strings.Join(provider.ReadOnlyTools(), ","),
	}
	return model.CapabilityEvidence{
		Kind:    CapabilityKindCodeIntelligenceProvider,
		ID:      entry.ID,
		Name:    provider.ID(),
		Path:    env.Root,
		State:   map[bool]string{true: "selected", false: "not_selected"}[enabled],
		Details: compactDetails(details),
	}
}

func compactProviderToken(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var b strings.Builder
	for _, r := range value {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func uniqueStrings(values []string) []string {
	seen := map[string]struct{}{}
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}
