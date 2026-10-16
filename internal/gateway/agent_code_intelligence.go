package gateway

import (
	"fmt"
	"strings"

	"ai-dev-manager-v2/internal/app"
	"ai-dev-manager-v2/internal/codeintel"
)

func agentStaticCodeIntelligenceReferences(owner *runtimeOwner, service *app.Service, environmentID string, symbol codeintel.SymbolLocator, maxResults int) (CodeIntelligenceReferencesResult, error) {
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
	staticInfo, err := service.CodeIntelligenceInfo(environmentID)
	if err != nil {
		return CodeIntelligenceReferencesResult{}, err
	}
	result := CodeIntelligenceReferencesResult{
		Provider:  staticInfo,
		Available: false,
		Reason:    "provider_capability_unavailable",
	}
	if owner == nil {
		return result, nil
	}
	if candidate, staticErr := owner.staticPHPCallReferences(environmentID, symbol, maxResults); staticErr == nil {
		result.Available = true
		result.Reason = ""
		result.Result = &candidate
	} else if strings.EqualFold(symbol.Language, "PHP") || strings.HasSuffix(strings.ToLower(symbol.Path), ".php") {
		result.Reason = "static_php_reference_index_unavailable"
	}
	return result, nil
}

func agentStaticCodeIntelligenceHierarchy(service *app.Service, environmentID string, symbol codeintel.SymbolLocator, direction string) (CodeIntelligenceHierarchyResult, error) {
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
	staticInfo, err := service.CodeIntelligenceInfo(environmentID)
	if err != nil {
		return CodeIntelligenceHierarchyResult{}, err
	}
	return CodeIntelligenceHierarchyResult{
		Provider:  staticInfo,
		Available: false,
		Reason:    "provider_capability_unavailable",
	}, nil
}
