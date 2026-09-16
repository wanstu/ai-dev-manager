package app

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"unicode"

	"ai-dev-manager-v2/internal/catalog"
	"ai-dev-manager-v2/internal/model"
)

const (
	MCPImportAuto        = "auto"
	MCPImportOpenCode    = "opencode"
	MCPImportWorkBuddy   = "workbuddy"
	MCPImportCodexPlugin = "codex-plugin"
	MCPImportClaudeCode  = "claude-code"
	MCPImportMCPHub      = "mcphub"
	MCPImportGeneric     = "generic-mcpservers"
)

type MCPImportInput struct {
	Format         string   `json:"format"`
	Content        string   `json:"json_or_jsonc"`
	SelectedNames  []string `json:"selected_names,omitempty"`
	ConflictPolicy string   `json:"conflict_policy,omitempty"`
	DefaultInclude bool     `json:"default_include,omitempty"`
	SourceScope    string   `json:"source_scope,omitempty"`
}

type MCPImportIssue struct {
	Kind      string `json:"kind"`
	FieldPath string `json:"field_path,omitempty"`
	Message   string `json:"message"`
}

type MCPImportReferenceRequirement struct {
	ReferenceName string `json:"reference_name"`
	FieldPath     string `json:"field_path"`
}

type MCPImportCandidate struct {
	Name                  string                          `json:"name"`
	Format                string                          `json:"format"`
	Transport             string                          `json:"transport,omitempty"`
	AuthMode              string                          `json:"auth_mode,omitempty"`
	EndpointConfigured    bool                            `json:"endpoint_configured"`
	Executable            string                          `json:"executable,omitempty"`
	Args                  []string                        `json:"args,omitempty"`
	HeaderReferenceKeys   []string                        `json:"header_reference_keys,omitempty"`
	EnvReferenceKeys      []string                        `json:"env_reference_keys,omitempty"`
	HealthPolicy          model.MCPHealthPolicy           `json:"health_policy"`
	Warnings              []MCPImportIssue                `json:"warnings,omitempty"`
	Errors                []MCPImportIssue                `json:"errors,omitempty"`
	ReferenceRequirements []MCPImportReferenceRequirement `json:"reference_requirements,omitempty"`
	ExistingMCPID         string                          `json:"existing_mcp_id,omitempty"`
}

type MCPImportPreview struct {
	Format      string               `json:"format"`
	SourceScope string               `json:"source_scope,omitempty"`
	Candidates  []MCPImportCandidate `json:"candidates"`
}

type MCPImportApplyResult struct {
	Format      string                 `json:"format"`
	SourceScope string                 `json:"source_scope,omitempty"`
	Result      catalog.MCPBatchResult `json:"result"`
}

type MCPImportError struct {
	ErrorKind string   `json:"error_kind"`
	Message   string   `json:"message"`
	Formats   []string `json:"formats,omitempty"`
}

func (e *MCPImportError) Error() string {
	if e == nil {
		return ""
	}
	if len(e.Formats) != 0 {
		return fmt.Sprintf("error_kind=%s message=%s formats=%s", e.ErrorKind, e.Message, strings.Join(e.Formats, ","))
	}
	return fmt.Sprintf("error_kind=%s message=%s", e.ErrorKind, e.Message)
}

type normalizedMCPImportCandidate struct {
	name         string
	format       string
	config       catalog.MCPConfig
	warnings     []MCPImportIssue
	errors       []MCPImportIssue
	requirements []MCPImportReferenceRequirement
}

func (s *Service) PreviewMCPImport(input MCPImportInput) (MCPImportPreview, error) {
	format, scope, candidates, err := s.normalizeMCPImport(input)
	if err != nil {
		return MCPImportPreview{}, err
	}
	existing, err := s.MCPs.List()
	if err != nil {
		return MCPImportPreview{}, err
	}
	byName := make(map[string]string, len(existing))
	for _, definition := range existing {
		byName[strings.ToLower(definition.Name)] = definition.ID
	}
	preview := MCPImportPreview{Format: format, SourceScope: scope, Candidates: make([]MCPImportCandidate, 0, len(candidates))}
	for _, candidate := range candidates {
		preview.Candidates = append(preview.Candidates, importPreviewCandidate(candidate, byName[strings.ToLower(candidate.name)]))
	}
	return preview, nil
}

func (s *Service) ApplyMCPImport(input MCPImportInput) (MCPImportApplyResult, error) {
	format, scope, candidates, err := s.normalizeMCPImport(input)
	if err != nil {
		return MCPImportApplyResult{}, err
	}
	selected, err := selectMCPImportCandidates(candidates, input.SelectedNames)
	if err != nil {
		return MCPImportApplyResult{}, err
	}
	items := make([]catalog.MCPNamedConfig, 0, len(selected))
	for _, candidate := range selected {
		if len(candidate.errors) != 0 {
			return MCPImportApplyResult{}, &MCPImportError{ErrorKind: "invalid_candidate", Message: fmt.Sprintf("selected MCP %q has blocking validation errors", candidate.name)}
		}
		items = append(items, catalog.MCPNamedConfig{Name: candidate.name, Config: candidate.config})
	}
	result, err := s.MCPs.ApplyBatch(items, input.ConflictPolicy)
	if err != nil {
		return MCPImportApplyResult{}, err
	}
	return MCPImportApplyResult{Format: format, SourceScope: scope, Result: result}, nil
}

func (s *Service) normalizeMCPImport(input MCPImportInput) (string, string, []normalizedMCPImportCandidate, error) {
	root, err := decodeJSONCObject(input.Content)
	if err != nil {
		return "", "", nil, err
	}
	format := strings.TrimSpace(input.Format)
	if format == "" {
		format = MCPImportAuto
	}
	if format == MCPImportAuto {
		format, err = detectMCPImportFormat(root)
		if err != nil {
			return "", "", nil, err
		}
	} else if !supportedMCPImportFormat(format) {
		return "", "", nil, &MCPImportError{ErrorKind: "unsupported_format", Message: fmt.Sprintf("unsupported MCP import format %q", format)}
	}
	servers, scope, err := importServerMap(format, root, input.SourceScope)
	if err != nil {
		return "", "", nil, err
	}
	names := make([]string, 0, len(servers))
	for name := range servers {
		names = append(names, name)
	}
	sort.Slice(names, func(i, j int) bool { return strings.ToLower(names[i]) < strings.ToLower(names[j]) })
	candidates := make([]normalizedMCPImportCandidate, 0, len(names))
	candidateByName := make(map[string]int, len(names))
	for _, name := range names {
		candidate := normalizeMCPImportServer(format, name, servers[name], input.DefaultInclude)
		if _, err := catalog.ValidateMCPConfig(candidate.name, candidate.config); err != nil {
			candidate.errors = append(candidate.errors, MCPImportIssue{Kind: "invalid_configuration", Message: err.Error()})
		}
		key := strings.ToLower(candidate.name)
		if previous, exists := candidateByName[key]; exists {
			issue := MCPImportIssue{Kind: "duplicate_source_name", Message: fmt.Sprintf("source contains duplicate MCP name %q under case-insensitive ADM identity", candidate.name)}
			candidate.errors = append(candidate.errors, issue)
			candidates[previous].errors = append(candidates[previous].errors, issue)
		} else {
			candidateByName[key] = len(candidates)
		}
		candidates = append(candidates, candidate)
	}
	if len(candidates) == 0 {
		return "", "", nil, &MCPImportError{ErrorKind: "no_candidates", Message: "MCP import source contains no server definitions"}
	}
	return format, scope, candidates, nil
}

func importPreviewCandidate(candidate normalizedMCPImportCandidate, existingID string) MCPImportCandidate {
	return MCPImportCandidate{
		Name:                  candidate.name,
		Format:                candidate.format,
		Transport:             candidate.config.Transport,
		AuthMode:              candidate.config.AuthMode,
		EndpointConfigured:    strings.TrimSpace(candidate.config.Endpoint) != "",
		Executable:            candidate.config.Executable,
		Args:                  append([]string(nil), candidate.config.Args...),
		HeaderReferenceKeys:   sortedImportKeys(candidate.config.HeaderRefs),
		EnvReferenceKeys:      sortedImportKeys(candidate.config.EnvRefs),
		HealthPolicy:          candidate.config.HealthPolicy,
		Warnings:              append([]MCPImportIssue(nil), candidate.warnings...),
		Errors:                append([]MCPImportIssue(nil), candidate.errors...),
		ReferenceRequirements: append([]MCPImportReferenceRequirement(nil), candidate.requirements...),
		ExistingMCPID:         existingID,
	}
}

func selectMCPImportCandidates(candidates []normalizedMCPImportCandidate, selectedNames []string) ([]normalizedMCPImportCandidate, error) {
	if len(selectedNames) == 0 {
		return candidates, nil
	}
	byName := make(map[string]normalizedMCPImportCandidate, len(candidates))
	for _, candidate := range candidates {
		byName[strings.ToLower(candidate.name)] = candidate
	}
	selected := make([]normalizedMCPImportCandidate, 0, len(selectedNames))
	seen := make(map[string]struct{}, len(selectedNames))
	for _, name := range selectedNames {
		key := strings.ToLower(strings.TrimSpace(name))
		if key == "" {
			return nil, &MCPImportError{ErrorKind: "invalid_selection", Message: "selected MCP name cannot be empty"}
		}
		if _, ok := seen[key]; ok {
			return nil, &MCPImportError{ErrorKind: "duplicate_selection", Message: fmt.Sprintf("MCP %q was selected more than once", name)}
		}
		candidate, ok := byName[key]
		if !ok {
			return nil, &MCPImportError{ErrorKind: "unknown_selection", Message: fmt.Sprintf("selected MCP %q is not present in import source", name)}
		}
		seen[key] = struct{}{}
		selected = append(selected, candidate)
	}
	return selected, nil
}

func supportedMCPImportFormat(format string) bool {
	switch format {
	case MCPImportOpenCode, MCPImportWorkBuddy, MCPImportCodexPlugin, MCPImportClaudeCode, MCPImportMCPHub, MCPImportGeneric:
		return true
	default:
		return false
	}
}

func detectMCPImportFormat(root map[string]any) (string, error) {
	if mcp, ok := objectValue(root["mcp"]); ok {
		if _, ok := objectValue(mcp["servers"]); ok {
			return MCPImportOpenCode, nil
		}
	}
	if _, ok := objectValue(root["projects"]); ok {
		return MCPImportClaudeCode, nil
	}
	if _, ok := objectValue(root["servers"]); ok {
		return MCPImportMCPHub, nil
	}
	if servers, ok := objectValue(root["mcpServers"]); ok {
		if hasWorkBuddyMarkers(servers) {
			return MCPImportWorkBuddy, nil
		}
		return MCPImportGeneric, nil
	}
	if looksLikeDirectServerMap(root) {
		return MCPImportCodexPlugin, nil
	}
	return "", &MCPImportError{ErrorKind: "unsupported_format", Message: "MCP import source shape is not recognized"}
}

func hasWorkBuddyMarkers(servers map[string]any) bool {
	for _, value := range servers {
		server, ok := objectValue(value)
		if !ok {
			continue
		}
		for _, key := range []string{"runtime", "npmRegistry", "x-workbuddy", "install"} {
			if _, ok := server[key]; ok {
				return true
			}
		}
		if typeName, _ := stringValue(server["type"]); strings.EqualFold(typeName, "streamableHttp") {
			return true
		}
	}
	return false
}

func looksLikeDirectServerMap(root map[string]any) bool {
	if len(root) == 0 {
		return false
	}
	for _, value := range root {
		server, ok := objectValue(value)
		if !ok {
			return false
		}
		if _, hasCommand := server["command"]; !hasCommand {
			if _, hasURL := server["url"]; !hasURL {
				return false
			}
		}
	}
	return true
}

func importServerMap(format string, root map[string]any, sourceScope string) (map[string]any, string, error) {
	switch format {
	case MCPImportOpenCode:
		mcp, ok := objectValue(root["mcp"])
		if !ok {
			return nil, "", importShapeError(format, "mcp object is required")
		}
		servers, ok := objectValue(mcp["servers"])
		if !ok {
			return nil, "", importShapeError(format, "mcp.servers object is required")
		}
		return servers, "", nil
	case MCPImportWorkBuddy:
		servers, ok := objectValue(root["mcpServers"])
		if !ok {
			return nil, "", importShapeError(format, "mcpServers object is required")
		}
		return servers, "", nil
	case MCPImportCodexPlugin:
		if servers, ok := objectValue(root["mcpServers"]); ok {
			return servers, "", nil
		}
		if !looksLikeDirectServerMap(root) {
			return nil, "", importShapeError(format, "mcpServers wrapper or direct server map is required")
		}
		return root, "", nil
	case MCPImportClaudeCode:
		if servers, ok := objectValue(root["mcpServers"]); ok {
			return servers, "", nil
		}
		projects, ok := objectValue(root["projects"])
		if !ok {
			return nil, "", importShapeError(format, "mcpServers or projects object is required")
		}
		scope := strings.TrimSpace(sourceScope)
		if scope == "" {
			if len(projects) != 1 {
				return nil, "", &MCPImportError{ErrorKind: "source_scope_required", Message: "Claude Code source contains multiple project scopes; choose source_scope explicitly"}
			}
			for key := range projects {
				scope = key
			}
		}
		project, ok := objectValue(projects[scope])
		if !ok {
			return nil, "", &MCPImportError{ErrorKind: "source_scope_not_found", Message: fmt.Sprintf("Claude Code project scope %q was not found", scope)}
		}
		servers, ok := objectValue(project["mcpServers"])
		if !ok {
			return nil, "", importShapeError(format, fmt.Sprintf("projects.%s.mcpServers object is required", scope))
		}
		return servers, scope, nil
	case MCPImportMCPHub:
		if servers, ok := objectValue(root["mcpServers"]); ok {
			return servers, "", nil
		}
		servers, ok := objectValue(root["servers"])
		if !ok {
			return nil, "", importShapeError(format, "mcpServers or servers object is required")
		}
		return servers, "", nil
	case MCPImportGeneric:
		servers, ok := objectValue(root["mcpServers"])
		if !ok {
			return nil, "", importShapeError(format, "mcpServers object is required")
		}
		return servers, "", nil
	default:
		return nil, "", &MCPImportError{ErrorKind: "unsupported_format", Message: fmt.Sprintf("unsupported MCP import format %q", format)}
	}
}

func importShapeError(format, message string) error {
	return &MCPImportError{ErrorKind: "invalid_source_shape", Message: fmt.Sprintf("%s: %s", format, message)}
}

func normalizeMCPImportServer(format, name string, raw any, defaultInclude bool) normalizedMCPImportCandidate {
	candidate := normalizedMCPImportCandidate{name: strings.TrimSpace(name), format: format}
	candidate.config.DefaultInclude = defaultInclude
	server, ok := objectValue(raw)
	if !ok {
		candidate.errors = append(candidate.errors, MCPImportIssue{Kind: "invalid_server", Message: "server definition must be an object"})
		return candidate
	}
	if candidate.name == "" {
		candidate.errors = append(candidate.errors, MCPImportIssue{Kind: "invalid_name", Message: "server name cannot be empty"})
	}
	if format == MCPImportOpenCode {
		normalizeOpenCodeServer(&candidate, server)
	} else {
		normalizeStandardServer(&candidate, server)
	}
	appendSourceWarnings(&candidate, server)
	return candidate
}

func normalizeOpenCodeServer(candidate *normalizedMCPImportCandidate, server map[string]any) {
	typeName, _ := stringValue(server["type"])
	switch strings.ToLower(strings.TrimSpace(typeName)) {
	case "local":
		command, ok := stringSliceValue(server["command"])
		if !ok || len(command) == 0 {
			candidate.errors = append(candidate.errors, MCPImportIssue{Kind: "invalid_command", FieldPath: "command", Message: "OpenCode local MCP requires a non-empty command array"})
			return
		}
		candidate.config.Transport = catalog.MCPTransportStdio
		candidate.config.AuthMode = catalog.MCPAuthNone
		candidate.config.Executable = command[0]
		candidate.config.Args = append([]string(nil), command[1:]...)
		candidate.config.EnvRefs = normalizeImportReferenceMap(candidate, "environment", server["environment"])
	case "remote":
		candidate.config.Transport = catalog.MCPTransportStreamableHTTP
		candidate.config.Endpoint, _ = stringValue(server["url"])
		candidate.config.HeaderRefs = normalizeImportReferenceMap(candidate, "headers", server["headers"])
		if len(candidate.config.HeaderRefs) != 0 {
			candidate.config.AuthMode = catalog.MCPAuthHeaders
		} else {
			candidate.config.AuthMode = catalog.MCPAuthNone
		}
	default:
		candidate.errors = append(candidate.errors, MCPImportIssue{Kind: "unsupported_transport", FieldPath: "type", Message: "OpenCode MCP type must be local or remote"})
	}
}

func normalizeStandardServer(candidate *normalizedMCPImportCandidate, server map[string]any) {
	typeName, _ := stringValue(server["type"])
	normalizedType := strings.ToLower(strings.ReplaceAll(strings.ReplaceAll(strings.TrimSpace(typeName), "-", ""), "_", ""))
	if normalizedType == "sse" {
		candidate.errors = append(candidate.errors, MCPImportIssue{Kind: "unsupported_transport", FieldPath: "type", Message: "legacy SSE transport is not supported by ADM Phase 11"})
		return
	}
	urlValue, _ := stringValue(server["url"])
	commandValue, commandIsString := stringValue(server["command"])
	isHTTP := strings.TrimSpace(urlValue) != "" || normalizedType == "http" || normalizedType == "streamablehttp" || normalizedType == "remote"
	isStdio := strings.TrimSpace(commandValue) != "" || normalizedType == "stdio" || normalizedType == "local"
	if isHTTP && isStdio {
		candidate.errors = append(candidate.errors, MCPImportIssue{Kind: "mixed_transport", Message: "server mixes HTTP and stdio connection fields"})
		return
	}
	if isHTTP {
		candidate.config.Transport = catalog.MCPTransportStreamableHTTP
		candidate.config.Endpoint = urlValue
		candidate.config.HeaderRefs = normalizeImportReferenceMap(candidate, "headers", server["headers"])
		if len(candidate.config.HeaderRefs) != 0 {
			candidate.config.AuthMode = catalog.MCPAuthHeaders
		} else {
			candidate.config.AuthMode = catalog.MCPAuthNone
		}
		return
	}
	if isStdio {
		if !commandIsString || strings.TrimSpace(commandValue) == "" {
			candidate.errors = append(candidate.errors, MCPImportIssue{Kind: "invalid_command", FieldPath: "command", Message: "stdio MCP command must be a non-empty string"})
			return
		}
		candidate.config.Transport = catalog.MCPTransportStdio
		candidate.config.AuthMode = catalog.MCPAuthNone
		candidate.config.Executable = commandValue
		if args, ok := stringSliceValue(server["args"]); ok {
			candidate.config.Args = args
		} else if server["args"] != nil {
			candidate.errors = append(candidate.errors, MCPImportIssue{Kind: "invalid_args", FieldPath: "args", Message: "stdio MCP args must be an array of strings"})
		}
		candidate.config.EnvRefs = normalizeImportReferenceMap(candidate, "env", server["env"])
		return
	}
	candidate.errors = append(candidate.errors, MCPImportIssue{Kind: "unsupported_transport", Message: "server must configure stdio command or Streamable HTTP url"})
}

func appendSourceWarnings(candidate *normalizedMCPImportCandidate, server map[string]any) {
	if _, ok := server["cwd"]; ok {
		candidate.warnings = append(candidate.warnings, MCPImportIssue{Kind: "cwd_ignored", FieldPath: "cwd", Message: "ADM stdio MCP processes always use the selected Environment root as cwd"})
	}
	if _, ok := server["disabled"]; ok {
		candidate.warnings = append(candidate.warnings, MCPImportIssue{Kind: "source_enable_state_ignored", FieldPath: "disabled", Message: "source disabled state does not change ADM Environment selections"})
	}
	if _, ok := server["enabled"]; ok {
		candidate.warnings = append(candidate.warnings, MCPImportIssue{Kind: "source_enable_state_ignored", FieldPath: "enabled", Message: "source enabled state does not change ADM Environment selections"})
	}
	if _, ok := server["timeout"]; ok {
		candidate.warnings = append(candidate.warnings, MCPImportIssue{Kind: "source_timeout_not_mapped", FieldPath: "timeout", Message: "source timeout semantics are not mapped because units/meaning are adapter-specific"})
	}
	known := map[string]struct{}{
		"type": {}, "command": {}, "args": {}, "env": {}, "environment": {}, "cwd": {}, "url": {}, "headers": {},
		"disabled": {}, "enabled": {}, "timeout": {}, "description": {}, "version": {},
	}
	for key := range server {
		if _, ok := known[key]; ok {
			continue
		}
		candidate.warnings = append(candidate.warnings, MCPImportIssue{Kind: "unsupported_source_field", FieldPath: key, Message: fmt.Sprintf("source field %q has no ADM Phase 11 runtime equivalent", key)})
	}
	sort.Slice(candidate.warnings, func(i, j int) bool {
		if candidate.warnings[i].FieldPath == candidate.warnings[j].FieldPath {
			return candidate.warnings[i].Kind < candidate.warnings[j].Kind
		}
		return candidate.warnings[i].FieldPath < candidate.warnings[j].FieldPath
	})
}

func normalizeImportReferenceMap(candidate *normalizedMCPImportCandidate, field string, raw any) map[string]string {
	if raw == nil {
		return nil
	}
	values, ok := objectValue(raw)
	if !ok {
		candidate.errors = append(candidate.errors, MCPImportIssue{Kind: "invalid_reference_map", FieldPath: field, Message: fmt.Sprintf("%s must be an object of string values", field)})
		return nil
	}
	result := make(map[string]string, len(values))
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		value, ok := stringValue(values[key])
		path := field + "." + key
		if !ok {
			candidate.errors = append(candidate.errors, MCPImportIssue{Kind: "invalid_reference_value", FieldPath: path, Message: "reference value must be a string"})
			continue
		}
		normalized, requirement := normalizeImportReference(candidate.name, field, key, value)
		result[key] = normalized
		if requirement != nil {
			requirement.FieldPath = path
			candidate.requirements = append(candidate.requirements, *requirement)
		}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func normalizeImportReference(candidateName, field, key, value string) (string, *MCPImportReferenceRequirement) {
	value = strings.TrimSpace(value)
	value = normalizeBraceEnvReferences(value)
	if strings.Contains(value, "${") {
		return value, nil
	}
	reference := generatedImportReference(candidateName, field, key)
	template := "${" + reference + "}"
	if field == "headers" {
		lower := strings.ToLower(value)
		for _, prefix := range []string{"bearer ", "basic "} {
			if strings.HasPrefix(lower, prefix) {
				template = value[:len(prefix)] + template
				break
			}
		}
	}
	return template, &MCPImportReferenceRequirement{ReferenceName: reference}
}

func normalizeBraceEnvReferences(value string) string {
	for {
		start := strings.Index(value, "{env:")
		if start < 0 {
			return value
		}
		endOffset := strings.IndexByte(value[start:], '}')
		if endOffset < 0 {
			return value
		}
		end := start + endOffset
		name := strings.TrimSpace(value[start+5 : end])
		if name == "" {
			return value
		}
		value = value[:start] + "${" + name + "}" + value[end+1:]
	}
}

func generatedImportReference(candidateName, field, key string) string {
	parts := []string{"ADM", "MCP", "IMPORT", candidateName, field, key}
	var b strings.Builder
	for i, part := range parts {
		if i != 0 {
			b.WriteByte('_')
		}
		lastUnderscore := false
		for _, r := range strings.ToUpper(part) {
			if unicode.IsLetter(r) || unicode.IsDigit(r) {
				b.WriteRune(r)
				lastUnderscore = false
			} else if !lastUnderscore {
				b.WriteByte('_')
				lastUnderscore = true
			}
		}
	}
	return strings.Trim(b.String(), "_")
}

func sortedImportKeys(values map[string]string) []string {
	if len(values) == 0 {
		return nil
	}
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func decodeJSONCObject(content string) (map[string]any, error) {
	cleaned, err := stripJSONC([]byte(content))
	if err != nil {
		return nil, err
	}
	cleaned = removeJSONTrailingCommas(cleaned)
	decoder := json.NewDecoder(bytes.NewReader(cleaned))
	decoder.UseNumber()
	if err := rejectDuplicateJSONKeys(decoder); err != nil {
		return nil, err
	}
	decoder = json.NewDecoder(bytes.NewReader(cleaned))
	decoder.UseNumber()
	var root map[string]any
	if err := decoder.Decode(&root); err != nil {
		return nil, &MCPImportError{ErrorKind: "invalid_json", Message: "MCP import content is not valid JSON/JSONC"}
	}
	if root == nil {
		return nil, &MCPImportError{ErrorKind: "invalid_json", Message: "MCP import content must be a JSON object"}
	}
	return root, nil
}

func stripJSONC(input []byte) ([]byte, error) {
	output := make([]byte, 0, len(input))
	inString := false
	escaped := false
	for i := 0; i < len(input); i++ {
		c := input[i]
		if inString {
			output = append(output, c)
			if escaped {
				escaped = false
				continue
			}
			if c == '\\' {
				escaped = true
			} else if c == '"' {
				inString = false
			}
			continue
		}
		if c == '"' {
			inString = true
			output = append(output, c)
			continue
		}
		if c == '/' && i+1 < len(input) {
			switch input[i+1] {
			case '/':
				i += 2
				for i < len(input) && input[i] != '\n' && input[i] != '\r' {
					i++
				}
				if i < len(input) {
					output = append(output, input[i])
				}
				continue
			case '*':
				i += 2
				closed := false
				for i+1 < len(input) {
					if input[i] == '*' && input[i+1] == '/' {
						i++
						closed = true
						break
					}
					i++
				}
				if !closed {
					return nil, &MCPImportError{ErrorKind: "invalid_jsonc", Message: "MCP import content has an unterminated block comment"}
				}
				continue
			}
		}
		output = append(output, c)
	}
	return output, nil
}

func removeJSONTrailingCommas(input []byte) []byte {
	output := make([]byte, 0, len(input))
	inString := false
	escaped := false
	for i := 0; i < len(input); i++ {
		c := input[i]
		if inString {
			output = append(output, c)
			if escaped {
				escaped = false
			} else if c == '\\' {
				escaped = true
			} else if c == '"' {
				inString = false
			}
			continue
		}
		if c == '"' {
			inString = true
			output = append(output, c)
			continue
		}
		if c == ',' {
			j := i + 1
			for j < len(input) && unicode.IsSpace(rune(input[j])) {
				j++
			}
			if j < len(input) && (input[j] == '}' || input[j] == ']') {
				continue
			}
		}
		output = append(output, c)
	}
	return output
}

func rejectDuplicateJSONKeys(decoder *json.Decoder) error {
	if err := scanJSONValue(decoder); err != nil {
		return err
	}
	if decoder.More() {
		return &MCPImportError{ErrorKind: "invalid_json", Message: "MCP import content contains trailing JSON values"}
	}
	return nil
}

func scanJSONValue(decoder *json.Decoder) error {
	token, err := decoder.Token()
	if err != nil {
		return &MCPImportError{ErrorKind: "invalid_json", Message: "MCP import content is not valid JSON/JSONC"}
	}
	delim, ok := token.(json.Delim)
	if !ok {
		return nil
	}
	switch delim {
	case '{':
		seen := map[string]struct{}{}
		for decoder.More() {
			keyToken, err := decoder.Token()
			if err != nil {
				return &MCPImportError{ErrorKind: "invalid_json", Message: "MCP import content is not valid JSON/JSONC"}
			}
			key, ok := keyToken.(string)
			if !ok {
				return &MCPImportError{ErrorKind: "invalid_json", Message: "MCP import object key is invalid"}
			}
			if _, exists := seen[key]; exists {
				return &MCPImportError{ErrorKind: "duplicate_source_key", Message: fmt.Sprintf("MCP import source contains duplicate object key %q", key)}
			}
			seen[key] = struct{}{}
			if err := scanJSONValue(decoder); err != nil {
				return err
			}
		}
		_, err = decoder.Token()
		return err
	case '[':
		for decoder.More() {
			if err := scanJSONValue(decoder); err != nil {
				return err
			}
		}
		_, err = decoder.Token()
		return err
	default:
		return nil
	}
}

func objectValue(value any) (map[string]any, bool) {
	object, ok := value.(map[string]any)
	return object, ok
}

func stringValue(value any) (string, bool) {
	text, ok := value.(string)
	return text, ok
}

func stringSliceValue(value any) ([]string, bool) {
	if value == nil {
		return nil, false
	}
	values, ok := value.([]any)
	if !ok {
		return nil, false
	}
	result := make([]string, 0, len(values))
	for _, value := range values {
		text, ok := value.(string)
		if !ok {
			return nil, false
		}
		result = append(result, text)
	}
	return result, true
}
