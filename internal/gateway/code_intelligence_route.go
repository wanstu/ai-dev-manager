package gateway

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"ai-dev-manager-v2/internal/app"
	"ai-dev-manager-v2/internal/codeintel"
	"ai-dev-manager-v2/internal/model"
	"ai-dev-manager-v2/internal/projectanalysis"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type codeIntelligenceExternalRoute struct {
	Provider codeintel.ProviderInfo
	MCPID    string
	Tools    map[string]bool
}

func (o *runtimeOwner) preferredCodeIntelligenceRoute(ctx context.Context, environmentID, requiredTool string) (codeIntelligenceExternalRoute, bool) {
	if o == nil {
		return codeIntelligenceExternalRoute{}, false
	}
	report, err := o.InvestigationProviderReport(ctx, environmentID)
	if err != nil {
		return codeIntelligenceExternalRoute{}, false
	}
	for _, fact := range report.Providers {
		if fact.Key != app.InvestigationProviderPhpStormKey || fact.State != model.CapabilityStateAvailable {
			continue
		}
		mcpID := investigationProviderMCPID(fact)
		if mcpID == "" {
			continue
		}
		available := investigationProviderInventoryTools(fact)
		if requiredTool != "" && !available[requiredTool] {
			continue
		}
		info := codeintel.ProviderInfo{
			Contract:               codeintel.ContractName,
			ProtocolVersion:        codeintel.ContractProtocolVersion,
			ID:                     app.InvestigationProviderPhpStorm,
			Name:                   "PhpStorm MCP Code Intelligence",
			Source:                 "mcp:" + mcpID,
			RequiresGeneratedIndex: false,
			Capabilities: codeintel.Capabilities{
				Definitions: available["code_intelligence_query"],
				References:  available["code_intelligence_references"],
				Hierarchy:   available["code_intelligence_hierarchy"],
			},
		}
		return codeIntelligenceExternalRoute{Provider: info, MCPID: mcpID, Tools: available}, true
	}
	return codeIntelligenceExternalRoute{}, false
}

func (o *runtimeOwner) negotiateCodeIntelligenceRoute(ctx context.Context, environmentID, requiredTool string) (codeIntelligenceExternalRoute, string, bool) {
	route, ok := o.preferredCodeIntelligenceRoute(ctx, environmentID, requiredTool)
	if !ok {
		return codeIntelligenceExternalRoute{}, "provider_capability_unavailable", false
	}
	if !route.Tools["code_intelligence_info"] {
		return route, "external_provider_info_unavailable", false
	}
	rt, _, err := o.service.Runtime(environmentID)
	if err != nil {
		return route, "external_provider_environment_unavailable", false
	}
	result, err := o.CallTool(ctx, environmentID, route.MCPID, "code_intelligence_info", map[string]any{
		"environment_id":   environmentID,
		"project_root":     rt.Root(),
		"contract":         codeintel.ContractName,
		"protocol_version": codeintel.ContractProtocolVersion,
	})
	if err != nil {
		return route, "external_provider_negotiation_failed", false
	}
	var info codeintel.ProviderInfo
	if err := decodeProviderStructuredResult(result, &info); err != nil {
		return route, "external_provider_invalid_info", false
	}
	info.Contract = strings.TrimSpace(info.Contract)
	info.ID = strings.TrimSpace(info.ID)
	info.Name = strings.TrimSpace(info.Name)
	info.ProviderVersion = strings.TrimSpace(info.ProviderVersion)
	info.Source = strings.TrimSpace(info.Source)
	if info.Contract != codeintel.ContractName || info.ProtocolVersion != codeintel.ContractProtocolVersion {
		return route, "external_provider_protocol_mismatch", false
	}
	if info.ID == "" {
		return route, "external_provider_invalid_info", false
	}
	if !codeIntelligenceProviderSupportsTool(info, requiredTool) {
		return route, "external_provider_capability_mismatch", false
	}
	if info.Name == "" {
		info.Name = route.Provider.Name
	}
	if info.Source == "" {
		info.Source = "mcp:" + route.MCPID
	}
	route.Provider = info
	return route, "", true
}

func codeIntelligenceProviderSupportsTool(info codeintel.ProviderInfo, tool string) bool {
	switch tool {
	case "", "code_intelligence_info", "code_intelligence_status":
		return true
	case "code_intelligence_query":
		return info.Capabilities.Definitions
	case "code_intelligence_references":
		return info.Capabilities.References
	case "code_intelligence_hierarchy":
		return info.Capabilities.Hierarchy
	default:
		return false
	}
}

func investigationProviderInventoryTools(fact model.CapabilityFact) map[string]bool {
	tools := map[string]bool{}
	for _, evidence := range fact.Evidence {
		if evidence.Kind != "code_intelligence_provider_inventory" || evidence.Details == nil {
			continue
		}
		for _, name := range strings.Split(evidence.Details["available_read_tools"], ",") {
			name = strings.TrimSpace(name)
			if name != "" {
				tools[name] = true
			}
		}
	}
	return tools
}

func (o *runtimeOwner) preferredCodeIntelligenceInfo(ctx context.Context, environmentID string) (codeintel.ProviderInfo, error) {
	if route, _, ok := o.negotiateCodeIntelligenceRoute(ctx, environmentID, ""); ok {
		return route.Provider, nil
	}
	return o.service.CodeIntelligenceInfo(environmentID)
}

func (o *runtimeOwner) queryCodeIntelligence(ctx context.Context, environmentID string, query projectanalysis.IndexQuery) (CodeIntelligenceQueryResult, error) {
	query.Query = strings.TrimSpace(query.Query)
	query.Path = strings.TrimSpace(query.Path)
	query.Kind = strings.TrimSpace(query.Kind)
	query.Language = strings.TrimSpace(query.Language)
	if query.Query == "" && query.Path == "" && query.Kind == "" && query.Language == "" {
		return CodeIntelligenceQueryResult{}, fmt.Errorf("query, path, kind, or language is required")
	}
	if query.MaxResults <= 0 {
		query.MaxResults = 50
	}
	if query.MaxResults > 200 {
		query.MaxResults = 200
	}
	staticInfo, infoErr := o.service.CodeIntelligenceInfo(environmentID)
	if infoErr != nil {
		return CodeIntelligenceQueryResult{}, infoErr
	}
	route, routeReason, ok := o.negotiateCodeIntelligenceRoute(ctx, environmentID, "code_intelligence_query")
	if !ok {
		value, err := o.service.ProjectIndexQuery(environmentID, query)
		result := CodeIntelligenceQueryResult{Provider: staticInfo, Result: value}
		if route.Provider.ID != "" && routeReason != "provider_capability_unavailable" {
			result.AttemptedProvider = &route.Provider
			result.FallbackReason = routeReason
		}
		return result, err
	}

	rt, _, err := o.service.Runtime(environmentID)
	if err != nil {
		return CodeIntelligenceQueryResult{}, err
	}
	external, callErr := o.CallTool(ctx, environmentID, route.MCPID, "code_intelligence_query", map[string]any{
		"environment_id":   environmentID,
		"project_root":     rt.Root(),
		"contract":         codeintel.ContractName,
		"protocol_version": codeintel.ContractProtocolVersion,
		"query":            query.Query,
		"path":             query.Path,
		"kind":             query.Kind,
		"language":         query.Language,
		"exact":            query.Exact,
		"max_results":      query.MaxResults,
	})
	if callErr == nil {
		var value projectanalysis.IndexQueryResult
		if decodeErr := decodeProviderStructuredResult(external, &value); decodeErr == nil {
			if value.Returned == 0 && len(value.Matches) > 0 {
				value.Returned = len(value.Matches)
			}
			if value.IndexPath == "" {
				value.IndexPath = "provider://phpstorm"
			}
			return CodeIntelligenceQueryResult{Provider: route.Provider, Result: value}, nil
		}
	}

	value, staticErr := o.service.ProjectIndexQuery(environmentID, query)
	if staticErr != nil {
		if callErr != nil {
			return CodeIntelligenceQueryResult{}, fmt.Errorf("phpstorm provider failed and static fallback failed: %w", errors.Join(callErr, staticErr))
		}
		return CodeIntelligenceQueryResult{}, fmt.Errorf("phpstorm provider returned invalid structured result and static fallback failed: %w", staticErr)
	}
	reason := "external_provider_invalid_result"
	if callErr != nil {
		reason = "external_provider_call_failed"
	}
	return CodeIntelligenceQueryResult{
		Provider:          staticInfo,
		Result:            value,
		AttemptedProvider: &route.Provider,
		FallbackReason:    reason,
	}, nil
}

func (o *runtimeOwner) statusCodeIntelligence(ctx context.Context, environmentID string, maxChanges int) (CodeIntelligenceStatusResult, error) {
	if maxChanges <= 0 {
		maxChanges = 50
	}
	if maxChanges > 200 {
		maxChanges = 200
	}
	staticInfo, infoErr := o.service.CodeIntelligenceInfo(environmentID)
	if infoErr != nil {
		return CodeIntelligenceStatusResult{}, infoErr
	}
	route, routeReason, ok := o.negotiateCodeIntelligenceRoute(ctx, environmentID, "code_intelligence_status")
	if !ok {
		value, err := o.service.ProjectIndexStatus(environmentID, maxChanges)
		result := CodeIntelligenceStatusResult{Provider: staticInfo, Result: value}
		if route.Provider.ID != "" && routeReason != "provider_capability_unavailable" {
			result.AttemptedProvider = &route.Provider
			result.FallbackReason = routeReason
		}
		return result, err
	}

	rt, _, err := o.service.Runtime(environmentID)
	if err != nil {
		return CodeIntelligenceStatusResult{}, err
	}
	external, callErr := o.CallTool(ctx, environmentID, route.MCPID, "code_intelligence_status", map[string]any{
		"environment_id":   environmentID,
		"project_root":     rt.Root(),
		"contract":         codeintel.ContractName,
		"protocol_version": codeintel.ContractProtocolVersion,
		"max_changes":      maxChanges,
	})
	if callErr == nil {
		var value projectanalysis.IndexStatusResult
		if decodeErr := decodeProviderStructuredResult(external, &value); decodeErr == nil && strings.TrimSpace(value.State) != "" {
			return CodeIntelligenceStatusResult{Provider: route.Provider, Result: value}, nil
		}
	}

	value, staticErr := o.service.ProjectIndexStatus(environmentID, maxChanges)
	if staticErr != nil {
		if callErr != nil {
			return CodeIntelligenceStatusResult{}, fmt.Errorf("phpstorm provider failed and static fallback failed: %w", errors.Join(callErr, staticErr))
		}
		return CodeIntelligenceStatusResult{}, fmt.Errorf("phpstorm provider returned invalid structured result and static fallback failed: %w", staticErr)
	}
	reason := "external_provider_invalid_result"
	if callErr != nil {
		reason = "external_provider_call_failed"
	}
	return CodeIntelligenceStatusResult{
		Provider:          staticInfo,
		Result:            value,
		AttemptedProvider: &route.Provider,
		FallbackReason:    reason,
	}, nil
}

func (o *runtimeOwner) referencesCodeIntelligence(ctx context.Context, environmentID string, symbol codeintel.SymbolLocator, maxResults int) (CodeIntelligenceReferencesResult, error) {
	symbol = normalizeCodeIntelligenceSymbol(symbol)
	if !symbol.Valid() {
		return CodeIntelligenceReferencesResult{}, fmt.Errorf("symbol path, name, or qualified_name is required")
	}
	if maxResults <= 0 {
		maxResults = 100
	}
	if maxResults > 500 {
		maxResults = 500
	}
	staticInfo, err := o.service.CodeIntelligenceInfo(environmentID)
	if err != nil {
		return CodeIntelligenceReferencesResult{}, err
	}
	route, routeReason, ok := o.negotiateCodeIntelligenceRoute(ctx, environmentID, "code_intelligence_references")
	if !ok {
		result := CodeIntelligenceReferencesResult{
			Provider:  staticInfo,
			Available: false,
			Reason:    routeReason,
		}
		if route.Provider.ID != "" && routeReason != "provider_capability_unavailable" {
			result.AttemptedProvider = &route.Provider
		}
		return result, nil
	}
	rt, _, err := o.service.Runtime(environmentID)
	if err != nil {
		return CodeIntelligenceReferencesResult{}, err
	}
	external, callErr := o.CallTool(ctx, environmentID, route.MCPID, "code_intelligence_references", map[string]any{
		"environment_id":   environmentID,
		"project_root":     rt.Root(),
		"contract":         codeintel.ContractName,
		"protocol_version": codeintel.ContractProtocolVersion,
		"symbol":           symbol,
		"max_results":      maxResults,
	})
	if callErr != nil {
		return CodeIntelligenceReferencesResult{
			Provider:          staticInfo,
			Available:         false,
			AttemptedProvider: &route.Provider,
			Reason:            "external_provider_call_failed",
		}, nil
	}
	var value codeintel.ReferencesResult
	if decodeErr := decodeProviderStructuredResult(external, &value); decodeErr != nil {
		return CodeIntelligenceReferencesResult{
			Provider:          staticInfo,
			Available:         false,
			AttemptedProvider: &route.Provider,
			Reason:            "external_provider_invalid_result",
		}, nil
	}
	if value.Returned == 0 && len(value.References) > 0 {
		value.Returned = len(value.References)
	}
	if !value.Symbol.Valid() {
		value.Symbol = symbol
	}
	return CodeIntelligenceReferencesResult{Provider: route.Provider, Available: true, Result: &value}, nil
}

func (o *runtimeOwner) hierarchyCodeIntelligence(ctx context.Context, environmentID string, symbol codeintel.SymbolLocator, direction string, maxDepth, maxResults int) (CodeIntelligenceHierarchyResult, error) {
	symbol = normalizeCodeIntelligenceSymbol(symbol)
	if !symbol.Valid() {
		return CodeIntelligenceHierarchyResult{}, fmt.Errorf("symbol path, name, or qualified_name is required")
	}
	direction = strings.ToLower(strings.TrimSpace(direction))
	if direction == "" {
		direction = "both"
	}
	switch direction {
	case "parents", "children", "both":
	default:
		return CodeIntelligenceHierarchyResult{}, fmt.Errorf("direction must be parents, children, or both")
	}
	if maxDepth <= 0 {
		maxDepth = 2
	}
	if maxDepth > 8 {
		maxDepth = 8
	}
	if maxResults <= 0 {
		maxResults = 100
	}
	if maxResults > 500 {
		maxResults = 500
	}
	staticInfo, err := o.service.CodeIntelligenceInfo(environmentID)
	if err != nil {
		return CodeIntelligenceHierarchyResult{}, err
	}
	route, routeReason, ok := o.negotiateCodeIntelligenceRoute(ctx, environmentID, "code_intelligence_hierarchy")
	if !ok {
		result := CodeIntelligenceHierarchyResult{
			Provider:  staticInfo,
			Available: false,
			Reason:    routeReason,
		}
		if route.Provider.ID != "" && routeReason != "provider_capability_unavailable" {
			result.AttemptedProvider = &route.Provider
		}
		return result, nil
	}
	rt, _, err := o.service.Runtime(environmentID)
	if err != nil {
		return CodeIntelligenceHierarchyResult{}, err
	}
	external, callErr := o.CallTool(ctx, environmentID, route.MCPID, "code_intelligence_hierarchy", map[string]any{
		"environment_id":   environmentID,
		"project_root":     rt.Root(),
		"contract":         codeintel.ContractName,
		"protocol_version": codeintel.ContractProtocolVersion,
		"symbol":           symbol,
		"direction":        direction,
		"max_depth":        maxDepth,
		"max_results":      maxResults,
	})
	if callErr != nil {
		return CodeIntelligenceHierarchyResult{
			Provider:          staticInfo,
			Available:         false,
			AttemptedProvider: &route.Provider,
			Reason:            "external_provider_call_failed",
		}, nil
	}
	var value codeintel.HierarchyResult
	if decodeErr := decodeProviderStructuredResult(external, &value); decodeErr != nil {
		return CodeIntelligenceHierarchyResult{
			Provider:          staticInfo,
			Available:         false,
			AttemptedProvider: &route.Provider,
			Reason:            "external_provider_invalid_result",
		}, nil
	}
	if value.Returned == 0 && len(value.Nodes) > 0 {
		value.Returned = len(value.Nodes)
	}
	if !value.Symbol.Valid() {
		value.Symbol = symbol
	}
	if strings.TrimSpace(value.Direction) == "" {
		value.Direction = direction
	}
	return CodeIntelligenceHierarchyResult{Provider: route.Provider, Available: true, Result: &value}, nil
}

func normalizeCodeIntelligenceSymbol(symbol codeintel.SymbolLocator) codeintel.SymbolLocator {
	symbol.Path = strings.TrimSpace(symbol.Path)
	symbol.Name = strings.TrimSpace(symbol.Name)
	symbol.QualifiedName = strings.TrimSpace(symbol.QualifiedName)
	symbol.Kind = strings.TrimSpace(symbol.Kind)
	symbol.Language = strings.TrimSpace(symbol.Language)
	if symbol.Line < 0 {
		symbol.Line = 0
	}
	return symbol
}

func decodeProviderStructuredResult(result *mcp.CallToolResult, target any) error {
	if result == nil {
		return fmt.Errorf("provider returned no result")
	}
	if result.IsError {
		return fmt.Errorf("provider returned tool error")
	}
	if result.StructuredContent == nil {
		return fmt.Errorf("provider returned no structured content")
	}
	raw, err := json.Marshal(result.StructuredContent)
	if err != nil {
		return fmt.Errorf("encode provider structured content: %w", err)
	}
	if err := json.Unmarshal(raw, target); err != nil {
		return fmt.Errorf("decode provider structured content: %w", err)
	}
	return nil
}
