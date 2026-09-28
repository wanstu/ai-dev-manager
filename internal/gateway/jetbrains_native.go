package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"ai-dev-manager-v2/internal/app"
	"ai-dev-manager-v2/internal/codeintel"
	"ai-dev-manager-v2/internal/model"
	"ai-dev-manager-v2/internal/projectanalysis"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type jetBrainsNativeCompatibility struct {
	ProviderID            string   `json:"provider_id"`
	MCPID                 string   `json:"mcp_id,omitempty"`
	Tool                  string   `json:"tool,omitempty"`
	InputCompatible       bool     `json:"input_compatible"`
	OutputSchemaAvailable bool     `json:"output_schema_available"`
	OutputCompatible      bool     `json:"output_compatible"`
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
	if !jetBrainsSearchSymbolOutputCompatible(outputSchema) {
		result.Reason = "search_symbol_output_schema_incompatible"
		return result
	}
	result.OutputCompatible = true
	result.AutoRouteEnabled = true
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
	args := map[string]any{
		"q":           query,
		"projectPath": projectPath,
		"limit":       limit,
	}
	if stringSliceContains(compatibility.InputFields, "include_external") {
		args["include_external"] = false
	}
	callResult, err := o.CallTool(ctx, environmentID, compatibility.MCPID, "search_symbol", args)
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

const (
	jetBrainsNativeAdapterContract        = "jetbrains.native.search_symbol"
	jetBrainsNativeAdapterProtocolVersion = 1
)

func jetBrainsNativeProviderInfo(mcpID string) codeintel.ProviderInfo {
	return codeintel.ProviderInfo{
		Contract:               jetBrainsNativeAdapterContract,
		ProtocolVersion:        jetBrainsNativeAdapterProtocolVersion,
		ID:                     app.InvestigationProviderJetBrainsNative,
		Name:                   "JetBrains Native IDE Index",
		Source:                 "mcp:" + strings.TrimSpace(mcpID) + "#search_symbol",
		RequiresGeneratedIndex: false,
		Capabilities: codeintel.Capabilities{
			Definitions: true,
			References:  false,
			Hierarchy:   false,
		},
	}
}

func (o *runtimeOwner) queryJetBrainsNativeCodeIntelligence(ctx context.Context, environmentID string, query projectanalysis.IndexQuery) (codeintel.ProviderInfo, projectanalysis.IndexQueryResult, string, bool) {
	compatibility := o.inspectJetBrainsNativeCompatibility(ctx, environmentID)
	if compatibility.MCPID == "" {
		return codeintel.ProviderInfo{}, projectanalysis.IndexQueryResult{}, compatibility.Reason, false
	}
	provider := jetBrainsNativeProviderInfo(compatibility.MCPID)
	if strings.TrimSpace(query.Query) == "" {
		return provider, projectanalysis.IndexQueryResult{}, "jetbrains_native_query_requires_text", false
	}
	if strings.TrimSpace(query.Kind) != "" || strings.TrimSpace(query.Language) != "" {
		return provider, projectanalysis.IndexQueryResult{}, "jetbrains_native_filter_not_supported", false
	}
	if !compatibility.AutoRouteEnabled {
		reason := compatibility.Reason
		if reason == "" {
			reason = "jetbrains_native_schema_incompatible"
		}
		return provider, projectanalysis.IndexQueryResult{}, reason, false
	}

	rt, _, err := o.service.Runtime(environmentID)
	if err != nil {
		return provider, projectanalysis.IndexQueryResult{}, "jetbrains_native_environment_unavailable", false
	}
	limit := query.MaxResults
	if limit <= 0 {
		limit = 50
	}
	if limit > 50 {
		limit = 50
	}
	args := map[string]any{
		"q":           strings.TrimSpace(query.Query),
		"projectPath": rt.Root(),
		"limit":       limit,
	}
	if stringSliceContains(compatibility.InputFields, "include_external") {
		args["include_external"] = false
	}
	callResult, err := o.CallTool(ctx, environmentID, compatibility.MCPID, "search_symbol", args)
	if err != nil {
		return provider, projectanalysis.IndexQueryResult{}, "jetbrains_native_call_failed", false
	}
	value, err := decodeJetBrainsNativeSearchResult(rt.Root(), query, limit, callResult)
	if err != nil {
		return provider, projectanalysis.IndexQueryResult{}, "jetbrains_native_invalid_result", false
	}
	return provider, value, "", true
}

func decodeJetBrainsNativeSearchResult(root string, query projectanalysis.IndexQuery, nativeLimit int, result *mcp.CallToolResult) (projectanalysis.IndexQueryResult, error) {
	if result == nil {
		return projectanalysis.IndexQueryResult{}, fmt.Errorf("JetBrains search_symbol returned no result")
	}
	if result.IsError {
		return projectanalysis.IndexQueryResult{}, fmt.Errorf("JetBrains search_symbol returned tool error")
	}
	if result.StructuredContent == nil {
		return projectanalysis.IndexQueryResult{}, fmt.Errorf("JetBrains search_symbol returned no structured content")
	}

	raw, err := json.Marshal(result.StructuredContent)
	if err != nil {
		return projectanalysis.IndexQueryResult{}, fmt.Errorf("encode JetBrains structured result: %w", err)
	}
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return projectanalysis.IndexQueryResult{}, fmt.Errorf("decode JetBrains structured result: %w", err)
	}
	rowsValue, ok := firstMapValue(payload, "results", "matches", "items")
	if !ok {
		return projectanalysis.IndexQueryResult{}, fmt.Errorf("JetBrains structured result has no results array")
	}
	rows, ok := rowsValue.([]any)
	if !ok {
		return projectanalysis.IndexQueryResult{}, fmt.Errorf("JetBrains results is not an array")
	}

	pathFilter := strings.ToLower(filepath.ToSlash(strings.TrimSpace(query.Path)))
	queryText := strings.ToLower(strings.TrimSpace(query.Query))
	matches := make([]projectanalysis.IndexQueryMatch, 0, len(rows))
	unusableRows := 0
	for _, rowValue := range rows {
		row, ok := schemaMap(rowValue)
		if !ok {
			unusableRows++
			continue
		}
		name := firstStringValue(row, "name", "symbolName", "displayName")
		nameProvided := name != ""
		pathValue := firstStringValue(row, "filePath", "path", "pathInProject", "relativePath")
		if pathValue == "" {
			unusableRows++
			continue
		}
		if name == "" {
			name = strings.TrimSpace(query.Query)
		}
		relativePath, ok := normalizeJetBrainsNativePath(root, pathValue)
		if !ok {
			unusableRows++
			continue
		}
		if pathFilter != "" && !strings.Contains(strings.ToLower(relativePath), pathFilter) {
			continue
		}
		qualifiedName := firstStringValue(row, "qualifiedName", "qualified_name", "fqn")
		if query.Exact {
			if !nameProvided && qualifiedName == "" {
				unusableRows++
				continue
			}
			if !strings.EqualFold(name, queryText) && !strings.EqualFold(qualifiedName, queryText) {
				continue
			}
		}
		line := firstIntValue(row, "line", "startLine", "start_line")
		if line < 0 {
			line = 0
		}
		matches = append(matches, projectanalysis.IndexQueryMatch{
			Path:          relativePath,
			Language:      firstStringValue(row, "language", "languageId", "language_id"),
			Namespace:     firstStringValue(row, "namespace"),
			Kind:          firstStringValue(row, "kind", "symbolKind", "type"),
			Name:          name,
			QualifiedName: qualifiedName,
			Line:          line,
			Match:         "provider",
		})
	}
	if len(rows) > 0 && len(matches) == 0 && unusableRows == len(rows) {
		return projectanalysis.IndexQueryResult{}, fmt.Errorf("JetBrains results contain no mappable symbol rows")
	}

	truncated := firstBoolValue(payload, "truncated", "incomplete", "more")
	if strings.TrimSpace(firstStringValue(payload, "partialResultReason")) != "" {
		truncated = true
	}
	for _, content := range result.Content {
		if item, ok := content.(*mcp.TextContent); ok && strings.Contains(strings.ToUpper(item.Text), "INCOMPLETE RESULTS") {
			truncated = true
			break
		}
	}
	if query.MaxResults > nativeLimit {
		truncated = true
	}
	return projectanalysis.IndexQueryResult{
		IndexPath:        "provider://jetbrains_native/search_symbol",
		ArtifactVerified: false,
		Matches:          matches,
		Returned:         len(matches),
		Truncated:        truncated,
	}, nil
}

func normalizeJetBrainsNativePath(root, value string) (string, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", false
	}
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return "", false
	}
	candidate := filepath.FromSlash(value)
	if filepath.IsAbs(candidate) {
		rel, err := filepath.Rel(rootAbs, filepath.Clean(candidate))
		if err != nil || pathEscapesRoot(rel) {
			return "", false
		}
		return filepath.ToSlash(rel), rel != "."
	}
	cleaned := filepath.Clean(candidate)
	if pathEscapesRoot(cleaned) || cleaned == "." {
		return "", false
	}
	return filepath.ToSlash(cleaned), true
}

func pathEscapesRoot(path string) bool {
	path = filepath.Clean(path)
	return path == ".." || strings.HasPrefix(path, ".."+string(filepath.Separator)) || filepath.IsAbs(path)
}

func stringSliceContains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func firstMapValue(values map[string]any, names ...string) (any, bool) {
	for _, name := range names {
		if value, ok := values[name]; ok {
			return value, true
		}
	}
	return nil, false
}

func firstStringValue(values map[string]any, names ...string) string {
	value, ok := firstMapValue(values, names...)
	if !ok || value == nil {
		return ""
	}
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case json.Number:
		return typed.String()
	default:
		return ""
	}
}

func firstIntValue(values map[string]any, names ...string) int {
	value, ok := firstMapValue(values, names...)
	if !ok || value == nil {
		return 0
	}
	switch typed := value.(type) {
	case float64:
		return int(typed)
	case float32:
		return int(typed)
	case int:
		return typed
	case int64:
		return int(typed)
	case json.Number:
		n, _ := strconv.Atoi(typed.String())
		return n
	case string:
		n, _ := strconv.Atoi(strings.TrimSpace(typed))
		return n
	default:
		return 0
	}
}

func firstBoolValue(values map[string]any, names ...string) bool {
	value, ok := firstMapValue(values, names...)
	if !ok || value == nil {
		return false
	}
	switch typed := value.(type) {
	case bool:
		return typed
	case string:
		value, _ := strconv.ParseBool(strings.TrimSpace(typed))
		return value
	default:
		return false
	}
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

func jetBrainsSearchSymbolOutputCompatible(schema map[string]any) bool {
	properties := schemaObjectProperties(schema)
	if len(properties) == 0 {
		return false
	}
	resultsValue, ok := firstSchemaProperty(properties, "results", "matches", "items")
	if !ok || !schemaPropertyAllowsType(resultsValue, map[string]bool{"array": true}) {
		return false
	}
	resultsSchema, ok := schemaMap(resultsValue)
	if !ok {
		return false
	}
	itemValue, ok := resultsSchema["items"]
	if !ok {
		return false
	}
	itemSchema, ok := schemaMap(itemValue)
	if !ok {
		return false
	}
	itemProperties := schemaObjectProperties(itemSchema)
	if len(itemProperties) == 0 {
		return false
	}
	pathValue, ok := firstSchemaProperty(itemProperties, "filePath", "path", "pathInProject", "relativePath")
	if !ok || !schemaPropertyAllowsType(pathValue, map[string]bool{"string": true}) {
		return false
	}
	if nameValue, hasName := firstSchemaProperty(itemProperties, "name", "symbolName", "displayName"); hasName &&
		!schemaPropertyAllowsType(nameValue, map[string]bool{"string": true}) {
		return false
	}
	return true
}

func schemaObjectProperties(schema map[string]any) map[string]any {
	value, ok := schema["properties"]
	if !ok {
		return nil
	}
	properties, ok := schemaMap(value)
	if !ok {
		return nil
	}
	return properties
}

func firstSchemaProperty(properties map[string]any, names ...string) (any, bool) {
	for _, name := range names {
		if value, ok := properties[name]; ok {
			return value, true
		}
	}
	return nil, false
}

func schemaMap(value any) (map[string]any, bool) {
	if value == nil {
		return nil, false
	}
	if mapped, ok := value.(map[string]any); ok {
		return mapped, true
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, false
	}
	var mapped map[string]any
	if err := json.Unmarshal(raw, &mapped); err != nil {
		return nil, false
	}
	return mapped, len(mapped) > 0
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
