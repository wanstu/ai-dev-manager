package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"

	"ai-dev-manager-v2/internal/app"
	"ai-dev-manager-v2/internal/model"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type jetBrainsNativeCompatibility struct {
	ProviderID            string   `json:"provider_id"`
	MCPID                 string   `json:"mcp_id,omitempty"`
	Tool                  string   `json:"tool,omitempty"`
	InputCompatible       bool     `json:"input_compatible"`
	OutputSchemaAvailable bool     `json:"output_schema_available"`
	AutoRouteEnabled      bool     `json:"auto_route_enabled"`
	InputFields           []string `json:"input_fields,omitempty"`
	OutputFields          []string `json:"output_fields,omitempty"`
	Reason                string   `json:"reason,omitempty"`
}

func (o *runtimeOwner) inspectJetBrainsNativeCompatibility(ctx context.Context, environmentID string) jetBrainsNativeCompatibility {
	result := jetBrainsNativeCompatibility{
		ProviderID:       app.InvestigationProviderJetBrainsNative,
		Tool:             "search_symbol",
		AutoRouteEnabled: false,
	}
	if o == nil {
		result.Reason = "gateway_owner_unavailable"
		return result
	}

	report, err := o.InvestigationProviderReport(ctx, environmentID)
	if err != nil {
		result.Reason = "provider_report_unavailable"
		return result
	}

	var providerFact *model.CapabilityFact
	for i := range report.Providers {
		if report.Providers[i].Key == app.InvestigationProviderJetBrainsNativeKey {
			providerFact = &report.Providers[i]
			break
		}
	}
	if providerFact == nil || providerFact.State == model.CapabilityStateUnconfigured {
		result.Reason = "provider_not_configured"
		return result
	}
	result.MCPID = investigationProviderMCPID(*providerFact)
	if providerFact.State != model.CapabilityStateAvailable {
		result.Reason = "provider_not_available"
		return result
	}
	if result.MCPID == "" {
		result.Reason = "provider_mcp_binding_missing"
		return result
	}

	tools, err := o.ListTools(ctx, environmentID, result.MCPID)
	if err != nil {
		result.Reason = "tool_schema_unavailable"
		return result
	}
	var searchSymbol *mcp.Tool
	for _, tool := range tools {
		if tool != nil && strings.EqualFold(strings.TrimSpace(tool.Name), "search_symbol") {
			searchSymbol = tool
			break
		}
	}
	if searchSymbol == nil {
		result.Reason = "search_symbol_missing"
		return result
	}

	inputSchema, err := toolSchemaObject(searchSymbol.InputSchema)
	if err != nil {
		result.Reason = "search_symbol_input_schema_invalid"
		return result
	}
	result.InputFields = toolSchemaPropertyNames(inputSchema)
	if !jetBrainsSearchSymbolInputCompatible(inputSchema) {
		result.Reason = "search_symbol_input_schema_incompatible"
		return result
	}
	result.InputCompatible = true

	if searchSymbol.OutputSchema == nil {
		result.Reason = "search_symbol_output_schema_missing"
		return result
	}
	outputSchema, err := toolSchemaObject(searchSymbol.OutputSchema)
	if err != nil {
		result.Reason = "search_symbol_output_schema_invalid"
		return result
	}
	result.OutputSchemaAvailable = true
	result.OutputFields = toolSchemaPropertyNames(outputSchema)
	result.Reason = "search_symbol_schema_ready"
	return result
}

const jetBrainsNativeProbeMaxOutputBytes = 64 * 1024

type jetBrainsNativeSearchProbeResult struct {
	ProviderID            string `json:"provider_id"`
	MCPID                 string `json:"mcp_id"`
	Tool                  string `json:"tool"`
	Query                 string `json:"query"`
	ProjectPath           string `json:"project_path"`
	Limit                 int    `json:"limit"`
	IncludeExternal       bool   `json:"include_external"`
	IsError               bool   `json:"is_error"`
	TextPreview           string `json:"text_preview,omitempty"`
	StructuredJSONPreview string `json:"structured_json_preview,omitempty"`
	Truncated             bool   `json:"truncated"`
}

func (o *runtimeOwner) probeJetBrainsNativeSearch(ctx context.Context, environmentID, query string, limit int) (jetBrainsNativeSearchProbeResult, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return jetBrainsNativeSearchProbeResult{}, fmt.Errorf("query is required")
	}
	if utf8.RuneCountInString(query) > 256 {
		return jetBrainsNativeSearchProbeResult{}, fmt.Errorf("query is too long")
	}
	if limit <= 0 {
		limit = 20
	}
	if limit > 50 {
		limit = 50
	}

	compatibility := o.inspectJetBrainsNativeCompatibility(ctx, environmentID)
	if compatibility.MCPID == "" {
		return jetBrainsNativeSearchProbeResult{}, fmt.Errorf("JetBrains native provider is unavailable: %s", compatibility.Reason)
	}
	if !compatibility.InputCompatible {
		return jetBrainsNativeSearchProbeResult{}, fmt.Errorf("JetBrains search_symbol input schema is incompatible: %s", compatibility.Reason)
	}

	rt, _, err := o.service.Runtime(environmentID)
	if err != nil {
		return jetBrainsNativeSearchProbeResult{}, err
	}
	projectPath := rt.Root()
	result := jetBrainsNativeSearchProbeResult{
		ProviderID:      app.InvestigationProviderJetBrainsNative,
		MCPID:           compatibility.MCPID,
		Tool:            "search_symbol",
		Query:           query,
		ProjectPath:     projectPath,
		Limit:           limit,
		IncludeExternal: false,
	}
	callResult, err := o.CallTool(ctx, environmentID, compatibility.MCPID, "search_symbol", map[string]any{
		"q":                query,
		"projectPath":      projectPath,
		"limit":            limit,
		"include_external": false,
	})
	if err != nil {
		return result, err
	}
	if callResult == nil {
		return result, fmt.Errorf("JetBrains search_symbol returned no result")
	}
	result.IsError = callResult.IsError

	var textParts []string
	for _, content := range callResult.Content {
		if item, ok := content.(*mcp.TextContent); ok && strings.TrimSpace(item.Text) != "" {
			textParts = append(textParts, item.Text)
		}
	}
	if len(textParts) > 0 {
		result.TextPreview, result.Truncated = truncateUTF8Preview(strings.Join(textParts, "\n"), jetBrainsNativeProbeMaxOutputBytes)
	}
	if callResult.StructuredContent != nil {
		raw, marshalErr := json.Marshal(callResult.StructuredContent)
		if marshalErr == nil {
			preview, truncated := truncateUTF8Preview(string(raw), jetBrainsNativeProbeMaxOutputBytes)
			result.StructuredJSONPreview = preview
			result.Truncated = result.Truncated || truncated
		}
	}
	return result, nil
}

func truncateUTF8Preview(value string, maxBytes int) (string, bool) {
	if maxBytes <= 0 || len(value) <= maxBytes {
		return value, false
	}
	cut := maxBytes
	for cut > 0 && !utf8.ValidString(value[:cut]) {
		cut--
	}
	if cut <= 0 {
		return "", true
	}
	return value[:cut], true
}

func toolSchemaObject(value any) (map[string]any, error) {
	if value == nil {
		return nil, fmt.Errorf("schema is missing")
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	var schema map[string]any
	if err := json.Unmarshal(raw, &schema); err != nil {
		return nil, err
	}
	if len(schema) == 0 {
		return nil, fmt.Errorf("schema is empty")
	}
	if schemaType, ok := schema["type"].(string); ok && schemaType != "" && schemaType != "object" {
		return nil, fmt.Errorf("schema type %q is not object", schemaType)
	}
	return schema, nil
}

func toolSchemaPropertyNames(schema map[string]any) []string {
	properties, _ := schema["properties"].(map[string]any)
	names := make([]string, 0, len(properties))
	for name := range properties {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func jetBrainsSearchSymbolInputCompatible(schema map[string]any) bool {
	properties, ok := schema["properties"].(map[string]any)
	if !ok {
		return false
	}
	for _, name := range []string{"q", "projectPath", "limit"} {
		property, ok := properties[name]
		if !ok || !schemaPropertyAllowsType(property, map[string]bool{"string": name != "limit", "integer": name == "limit", "number": name == "limit"}) {
			return false
		}
	}

	required := schemaRequiredNames(schema)
	return required["q"]
}

func schemaRequiredNames(schema map[string]any) map[string]bool {
	result := map[string]bool{}
	switch values := schema["required"].(type) {
	case []any:
		for _, value := range values {
			if name, ok := value.(string); ok {
				result[name] = true
			}
		}
	case []string:
		for _, name := range values {
			result[name] = true
		}
	}
	return result
}

func schemaPropertyAllowsType(value any, allowed map[string]bool) bool {
	property, ok := value.(map[string]any)
	if !ok {
		raw, err := json.Marshal(value)
		if err != nil {
			return false
		}
		if err := json.Unmarshal(raw, &property); err != nil {
			return false
		}
	}
	typeValue, exists := property["type"]
	if !exists {
		return true
	}
	switch typed := typeValue.(type) {
	case string:
		return allowed[typed]
	case []any:
		for _, item := range typed {
			if name, ok := item.(string); ok && allowed[name] {
				return true
			}
		}
	}
	return false
}
