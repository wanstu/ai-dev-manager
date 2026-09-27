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
	if route, ok := o.preferredCodeIntelligenceRoute(ctx, environmentID, "code_intelligence_query"); ok {
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
	route, ok := o.preferredCodeIntelligenceRoute(ctx, environmentID, "code_intelligence_query")
	if !ok {
		value, err := o.service.ProjectIndexQuery(environmentID, query)
		return CodeIntelligenceQueryResult{Provider: staticInfo, Result: value}, err
	}

	rt, _, err := o.service.Runtime(environmentID)
	if err != nil {
		return CodeIntelligenceQueryResult{}, err
	}
	external, callErr := o.CallTool(ctx, environmentID, route.MCPID, "code_intelligence_query", map[string]any{
		"environment_id": environmentID,
		"project_root":   rt.Root(),
		"query":          query.Query,
		"path":           query.Path,
		"kind":           query.Kind,
		"language":       query.Language,
		"exact":          query.Exact,
		"max_results":    query.MaxResults,
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
	route, ok := o.preferredCodeIntelligenceRoute(ctx, environmentID, "code_intelligence_status")
	if !ok {
		value, err := o.service.ProjectIndexStatus(environmentID, maxChanges)
		return CodeIntelligenceStatusResult{Provider: staticInfo, Result: value}, err
	}

	rt, _, err := o.service.Runtime(environmentID)
	if err != nil {
		return CodeIntelligenceStatusResult{}, err
	}
	external, callErr := o.CallTool(ctx, environmentID, route.MCPID, "code_intelligence_status", map[string]any{
		"environment_id": environmentID,
		"project_root":   rt.Root(),
		"max_changes":    maxChanges,
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
