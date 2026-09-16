package gateway

import (
	"context"
	"sort"
	"strconv"
	"strings"
	"time"

	"ai-dev-manager-v2/internal/app"
	"ai-dev-manager-v2/internal/model"
)

const capabilitySourceGatewayOwner = "gateway.owner"

func (o *runtimeOwner) CapabilityReport(ctx context.Context, environmentID string) (model.CapabilityReport, error) {
	report, err := o.service.EnvironmentCapabilityReport(ctx, environmentID)
	if err != nil {
		return model.CapabilityReport{}, err
	}
	return o.enrichCapabilityReport(environmentID, report), nil
}

func (o *runtimeOwner) enrichCapabilityReport(environmentID string, report model.CapabilityReport) model.CapabilityReport {
	info := o.Info()
	now := time.Now().UTC()
	for i := range report.Facts {
		report.Facts[i] = o.enrichMCPFact(environmentID, report.Facts[i], info, now)
		report.Facts[i] = o.enrichInvestigationProviderFact(environmentID, report.Facts[i], info, now)
		report.Facts[i] = enrichOwnerLifecycleFact(report.Facts[i], info, now)
	}
	sort.SliceStable(report.Facts, func(i, j int) bool {
		left := report.Facts[i].Kind + "/" + report.Facts[i].Key
		right := report.Facts[j].Kind + "/" + report.Facts[j].Key
		return strings.ToLower(left) < strings.ToLower(right)
	})
	return report
}

func (o *runtimeOwner) enrichMCPFact(environmentID string, fact model.CapabilityFact, info runtimeOwnerInfo, now time.Time) model.CapabilityFact {
	if fact.Kind != "mcp" || !strings.HasPrefix(fact.Key, "mcp/") || !factDesiredEnabled(fact, "mcp") {
		return fact
	}
	mcpID := strings.TrimPrefix(fact.Key, "mcp/")
	observation, observed := o.observation(runtimeOwnerKey{environmentID: environmentID, mcpID: mcpID})
	fact.Evidence = append(fact.Evidence, gatewayOwnerEvidence(info), mcpObservationEvidence(observation, observed))
	fact.Source = capabilitySourceGatewayOwner
	if observedAt, ok := mcpObservedAt(observation, observed, now); ok {
		fact.ObservedAt = &observedAt
	}

	if fact.State != model.CapabilityStateAvailable {
		return fact
	}
	if !observed {
		fact.State = model.CapabilityStateDegraded
		fact.ReasonCode = "not_observed"
		fact.Message = "Gateway owner has no runtime observation for this enabled MCP; desired configuration is statically available, but current health and inventory are not observed."
		return fact
	}
	switch observation.State {
	case app.MCPHealthHealthy:
		fact.State = model.CapabilityStateAvailable
		fact.ReasonCode = ""
		fact.Message = "Gateway owner currently observes this MCP as healthy."
	case app.MCPHealthError:
		fact.State = model.CapabilityStateUnavailable
		fact.ReasonCode = observation.ErrorKind
		if fact.ReasonCode == "" {
			fact.ReasonCode = "mcp_runtime_error"
		}
		fact.Message = sanitizeGatewayCapabilityMessage(observation.Message)
		if fact.Message == "" {
			fact.Message = "Gateway owner observed this MCP as unhealthy."
		}
	case app.MCPHealthDisabled:
		fact.State = model.CapabilityStateDisabled
		fact.ReasonCode = "mcp_disabled"
		fact.Message = "Gateway owner observes this MCP as disabled for this Environment."
	default:
		fact.State = model.CapabilityStateDegraded
		fact.ReasonCode = "mcp_not_healthy"
		fact.Message = "Gateway owner has an observation for this MCP, but it is not currently healthy."
	}
	return fact
}

func (o *runtimeOwner) InvestigationProviderReport(ctx context.Context, environmentID string) (model.InvestigationProviderReport, error) {
	report, err := o.CapabilityReport(ctx, environmentID)
	if err != nil {
		return model.InvestigationProviderReport{}, err
	}
	return app.InvestigationProviderReportFromCapabilityReport(report), nil
}

func (o *runtimeOwner) enrichInvestigationProviderFact(environmentID string, fact model.CapabilityFact, info runtimeOwnerInfo, now time.Time) model.CapabilityFact {
	if fact.Kind != app.CapabilityKindCodeIntelligenceProvider || fact.Key != app.InvestigationProviderGitNexusKey {
		return fact
	}
	if fact.State == model.CapabilityStateUnconfigured || fact.State == model.CapabilityStateDisabled || fact.State == model.CapabilityStateUnavailable {
		return fact
	}
	mcpID := investigationProviderMCPID(fact)
	if mcpID == "" {
		fact.State = model.CapabilityStateUnavailable
		fact.ReasonCode = "provider_mcp_binding_missing"
		fact.Message = "Provider capability lacks an Environment-authorized MCP binding."
		fact.Confidence = "none"
		fact.Freshness = "unknown"
		fact.Uncertainties = uniqueGatewayStrings(append(fact.Uncertainties, "provider_mcp_binding_missing", "static_fallback_available"))
		return fact
	}

	observation, observed := o.observation(runtimeOwnerKey{environmentID: environmentID, mcpID: mcpID})
	fact.Evidence = append(fact.Evidence, gatewayOwnerEvidence(info), investigationProviderObservationEvidence(observation, observed))
	fact.Source = capabilitySourceGatewayOwner
	if observedAt, ok := mcpObservedAt(observation, observed, now); ok {
		fact.ObservedAt = &observedAt
	}
	if !observed {
		fact.State = model.CapabilityStateDegraded
		fact.ReasonCode = "provider_not_observed"
		fact.Message = "Gateway owner has no existing runtime observation for the provider MCP; passive inspection did not connect or refresh it."
		fact.Confidence = "low"
		fact.Freshness = "unknown"
		fact.Uncertainties = uniqueGatewayStrings(append(fact.Uncertainties, "provider_health_not_observed", "provider_inventory_not_observed", "provider_index_freshness_not_observed", "static_fallback_available"))
		return fact
	}

	switch observation.State {
	case app.MCPHealthHealthy:
		availableTools := investigationProviderAvailableReadTools(observation.ToolInventory, app.InvestigationProviderReadOnlyTools(app.InvestigationProviderGitNexus))
		fact.Evidence = append(fact.Evidence, model.CapabilityEvidence{
			Kind:  "code_intelligence_provider_inventory",
			ID:    mcpID,
			State: "observed",
			Details: compactGatewayCapabilityDetails(map[string]string{
				"provider":             app.InvestigationProviderGitNexus,
				"available_read_tools": strings.Join(availableTools, ","),
				"read_tool_count":      strconv.Itoa(len(availableTools)),
				"inventory_fetched_at": formatGatewayCapabilityTime(observation.InventoryFetchedAt),
			}),
		})
		fact.Freshness = "inventory_observed"
		fact.Uncertainties = uniqueGatewayStrings(append(fact.Uncertainties, "provider_index_freshness_not_observed", "static_fallback_available"))
		if len(availableTools) == 0 {
			fact.State = model.CapabilityStateDegraded
			fact.ReasonCode = "provider_read_capabilities_not_observed"
			fact.Message = "Gateway owner observes the provider MCP as healthy, but its current inventory exposes none of the recognized read-only code intelligence tools."
			fact.Confidence = "low"
			return fact
		}
		fact.State = model.CapabilityStateAvailable
		fact.ReasonCode = ""
		fact.Message = "Gateway owner observes the Environment-authorized GitNexus MCP as healthy with recognized read-only code intelligence tools."
		fact.Confidence = "medium"
	case app.MCPHealthError:
		fact.State = model.CapabilityStateUnavailable
		fact.ReasonCode = observation.ErrorKind
		if fact.ReasonCode == "" {
			fact.ReasonCode = "provider_runtime_error"
		}
		fact.Message = sanitizeGatewayCapabilityMessage(observation.Message)
		if fact.Message == "" {
			fact.Message = "Gateway owner observes the provider MCP as unhealthy."
		}
		fact.Confidence = "none"
		fact.Freshness = "unknown"
		fact.Uncertainties = uniqueGatewayStrings(append(fact.Uncertainties, "provider_unavailable", "provider_index_freshness_not_observed", "static_fallback_available"))
	case app.MCPHealthDisabled:
		fact.State = model.CapabilityStateDisabled
		fact.ReasonCode = "provider_mcp_disabled"
		fact.Message = "Gateway owner observes the provider MCP as disabled for this Environment."
		fact.Confidence = "none"
		fact.Freshness = "unknown"
		fact.Uncertainties = uniqueGatewayStrings(append(fact.Uncertainties, "provider_not_authorized_for_environment", "static_fallback_available"))
	default:
		fact.State = model.CapabilityStateDegraded
		fact.ReasonCode = "provider_not_healthy"
		fact.Message = "Gateway owner has an existing provider MCP observation, but it is not currently healthy."
		fact.Confidence = "low"
		fact.Freshness = "unknown"
		fact.Uncertainties = uniqueGatewayStrings(append(fact.Uncertainties, "provider_health_uncertain", "provider_index_freshness_not_observed", "static_fallback_available"))
	}
	return fact
}

func investigationProviderMCPID(fact model.CapabilityFact) string {
	for _, evidence := range fact.Evidence {
		if evidence.Kind != app.CapabilityKindCodeIntelligenceProvider {
			continue
		}
		if evidence.Details != nil && strings.TrimSpace(evidence.Details["mcp_id"]) != "" {
			return strings.TrimSpace(evidence.Details["mcp_id"])
		}
		if strings.TrimSpace(evidence.ID) != "" {
			return strings.TrimSpace(evidence.ID)
		}
	}
	return ""
}

func investigationProviderAvailableReadTools(inventory []app.MCPToolInventoryItem, expected []string) []string {
	available := make(map[string]struct{}, len(inventory))
	for _, item := range inventory {
		available[strings.ToLower(strings.TrimSpace(item.Name))] = struct{}{}
	}
	result := make([]string, 0, len(expected))
	for _, name := range expected {
		if _, ok := available[strings.ToLower(name)]; ok {
			result = append(result, name)
		}
	}
	sort.Strings(result)
	return result
}

func investigationProviderObservationEvidence(observation app.MCPRuntimeObservation, observed bool) model.CapabilityEvidence {
	if !observed {
		return model.CapabilityEvidence{Kind: "code_intelligence_provider_observation", State: "not_observed", Details: map[string]string{"observed": "false"}}
	}
	return model.CapabilityEvidence{
		Kind:  "code_intelligence_provider_observation",
		ID:    observation.MCPID,
		State: string(observation.State),
		Details: compactGatewayCapabilityDetails(map[string]string{
			"observed":             "true",
			"transport":            observation.Transport,
			"error_kind":           observation.ErrorKind,
			"failure_stage":        string(observation.FailureStage),
			"last_check_at":        formatGatewayCapabilityTime(observation.LastCheckAt),
			"last_healthy_at":      formatGatewayCapabilityTime(observation.LastHealthyAt),
			"last_success_at":      formatGatewayCapabilityTime(observation.LastSuccessAt),
			"inventory_fetched_at": formatGatewayCapabilityTime(observation.InventoryFetchedAt),
		}),
	}
}

func uniqueGatewayStrings(values []string) []string {
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

func enrichOwnerLifecycleFact(fact model.CapabilityFact, info runtimeOwnerInfo, now time.Time) model.CapabilityFact {
	if fact.Key != "process.lifecycle" && fact.Key != "run.lifecycle" {
		return fact
	}
	fact.Evidence = append(fact.Evidence, gatewayOwnerEvidence(info), ownerLifecycleEvidence(fact.Key, info))
	fact.Source = capabilitySourceGatewayOwner
	fact.ObservedAt = &now
	if fact.State == model.CapabilityStateDegraded && fact.ReasonCode == "gateway_owner_observation_not_included" {
		fact.State = model.CapabilityStateAvailable
		fact.ReasonCode = ""
		if fact.Key == "process.lifecycle" {
			fact.Message = "Gateway owner is available for dev-process lifecycle operations; starting/stopping still requires the writer lease and Runtime executable authority."
		} else {
			fact.Message = "Gateway owner is available for generic async run lifecycle operations; starting/canceling still requires the writer lease and Runtime executable authority."
		}
	}
	return fact
}

func factDesiredEnabled(fact model.CapabilityFact, kind string) bool {
	for _, evidence := range fact.Evidence {
		if evidence.Kind != kind {
			continue
		}
		if evidence.State == "selected" {
			return true
		}
		if evidence.Details != nil && evidence.Details["enabled"] == "true" {
			return true
		}
	}
	return false
}

func gatewayOwnerEvidence(info runtimeOwnerInfo) model.CapabilityEvidence {
	return model.CapabilityEvidence{
		Kind: "gateway_owner",
		ID:   info.ID,
		Details: compactGatewayCapabilityDetails(map[string]string{
			"pid":                 strconv.Itoa(info.PID),
			"started_at":          info.StartedAt.UTC().Format(time.RFC3339Nano),
			"owned_mcp_sessions":  strconv.Itoa(info.OwnedMCPSessions),
			"owned_dev_processes": strconv.Itoa(info.OwnedDevProcesses),
			"owned_agent_runs":    strconv.Itoa(info.OwnedAgentRuns),
		}),
	}
}

func mcpObservationEvidence(observation app.MCPRuntimeObservation, observed bool) model.CapabilityEvidence {
	if !observed {
		return model.CapabilityEvidence{Kind: "mcp_runtime_observation", State: "not_observed", Details: map[string]string{"observed": "false"}}
	}
	return model.CapabilityEvidence{
		Kind:  "mcp_runtime_observation",
		ID:    observation.MCPID,
		State: string(observation.State),
		Details: compactGatewayCapabilityDetails(map[string]string{
			"observed":             "true",
			"desired_enabled":      strconv.FormatBool(observation.DesiredEnabled),
			"transport":            observation.Transport,
			"failure_stage":        string(observation.FailureStage),
			"error_kind":           observation.ErrorKind,
			"message":              sanitizeGatewayCapabilityMessage(observation.Message),
			"last_check_at":        formatGatewayCapabilityTime(observation.LastCheckAt),
			"last_healthy_at":      formatGatewayCapabilityTime(observation.LastHealthyAt),
			"last_success_at":      formatGatewayCapabilityTime(observation.LastSuccessAt),
			"next_reconnect_at":    formatGatewayCapabilityTime(observation.NextReconnectAt),
			"in_flight":            strconv.FormatBool(observation.InFlight),
			"consecutive_failures": strconv.Itoa(observation.ConsecutiveFailures),
			"tool_inventory_count": strconv.Itoa(len(observation.ToolInventory)),
			"inventory_fetched_at": formatGatewayCapabilityTime(observation.InventoryFetchedAt),
		}),
	}
}

func ownerLifecycleEvidence(key string, info runtimeOwnerInfo) model.CapabilityEvidence {
	kind := "process_observation"
	details := map[string]string{"running_count": strconv.Itoa(info.OwnedDevProcesses)}
	if key == "run.lifecycle" {
		kind = "run_observation"
		details = map[string]string{"running_count": strconv.Itoa(info.OwnedAgentRuns)}
	}
	return model.CapabilityEvidence{Kind: kind, Details: details}
}

func mcpObservedAt(observation app.MCPRuntimeObservation, observed bool, now time.Time) (time.Time, bool) {
	if !observed {
		return now, true
	}
	for _, value := range []*time.Time{observation.LastCheckAt, observation.LastSuccessAt, observation.LastHealthyAt, observation.InventoryFetchedAt} {
		if value != nil {
			return value.UTC(), true
		}
	}
	return now, true
}

func formatGatewayCapabilityTime(value *time.Time) string {
	if value == nil {
		return ""
	}
	return value.UTC().Format(time.RFC3339Nano)
}

func sanitizeGatewayCapabilityMessage(message string) string {
	message = strings.TrimSpace(message)
	message = strings.ReplaceAll(message, "\r", " ")
	message = strings.ReplaceAll(message, "\n", " ")
	return strings.Join(strings.Fields(message), " ")
}

func compactGatewayCapabilityDetails(details map[string]string) map[string]string {
	for key, value := range details {
		if strings.TrimSpace(value) == "" {
			delete(details, key)
		}
	}
	if len(details) == 0 {
		return nil
	}
	return details
}
