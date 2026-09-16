package catalog

import (
	"fmt"
	"net/url"
	"os"
	"sort"
	"strings"
	"time"

	"ai-dev-manager-v2/internal/identity"
	"ai-dev-manager-v2/internal/model"
	"ai-dev-manager-v2/internal/store"
)

const (
	MCPTransportStreamableHTTP = "streamable-http"
	MCPTransportStdio          = "stdio"

	MCPAuthNone    = "none"
	MCPAuthHeaders = "headers"

	MCPConflictError        = "error"
	MCPConflictSkip         = "skip"
	MCPConflictUpdateByName = "update_by_name"
)

type MCPConfig struct {
	Transport      string
	AuthMode       string
	Endpoint       string
	HeaderRefs     map[string]string
	Executable     string
	Args           []string
	EnvRefs        map[string]string
	HealthPolicy   model.MCPHealthPolicy
	DefaultInclude bool
}

type MCPNamedConfig struct {
	Name   string
	Config MCPConfig
}

type MCPBatchMutation struct {
	Action     string              `json:"action"`
	Definition model.MCPDefinition `json:"definition"`
}

type MCPBatchResult struct {
	Mutations []MCPBatchMutation `json:"mutations"`
}

type MCPService struct {
	store *store.Store
	now   func() time.Time
}

func NewMCP(s *store.Store) *MCPService { return &MCPService{store: s, now: time.Now} }

func (s *MCPService) AddMCP(name, endpoint string, defaultInclude bool) (model.MCPDefinition, error) {
	return s.AddMCPConfig(name, MCPConfig{
		Transport:      MCPTransportStreamableHTTP,
		AuthMode:       MCPAuthNone,
		Endpoint:       endpoint,
		DefaultInclude: defaultInclude,
	})
}

func (s *MCPService) AddMCPConfig(name string, config MCPConfig) (model.MCPDefinition, error) {
	return s.AddMCPConfigWithRetention(name, config, model.ResourceRetention{})
}

func (s *MCPService) AddMCPConfigWithRetention(name string, config MCPConfig, retention model.ResourceRetention) (model.MCPDefinition, error) {
	definition, err := ValidateMCPConfig(name, config)
	if err != nil {
		return model.MCPDefinition{}, err
	}

	var result model.MCPDefinition
	err = s.store.Update(func(state *model.State) error {
		for _, existing := range state.MCPs {
			if strings.EqualFold(existing.Name, definition.Name) {
				return fmt.Errorf("mcp %q already exists", definition.Name)
			}
		}
		id, err := identity.New("mcp")
		if err != nil {
			return err
		}
		result = cloneMCPDefinition(definition)
		result.ID = id
		result.Retention = normalizeCatalogRetention(retention, s.nowUTC())
		state.MCPs = append(state.MCPs, result)
		sort.Slice(state.MCPs, func(i, j int) bool {
			return strings.ToLower(state.MCPs[i].Name) < strings.ToLower(state.MCPs[j].Name)
		})
		return nil
	})
	return cloneMCPDefinition(result), err
}

func (s *MCPService) UpdateMCPConfig(id, name string, config MCPConfig) (model.MCPDefinition, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return model.MCPDefinition{}, fmt.Errorf("mcp id is required")
	}
	definition, err := ValidateMCPConfig(name, config)
	if err != nil {
		return model.MCPDefinition{}, err
	}
	definition.ID = id

	var result model.MCPDefinition
	err = s.store.Update(func(state *model.State) error {
		index := -1
		for i, existing := range state.MCPs {
			if existing.ID == id {
				index = i
				continue
			}
			if strings.EqualFold(existing.Name, definition.Name) {
				return fmt.Errorf("mcp %q already exists", definition.Name)
			}
		}
		if index < 0 {
			return fmt.Errorf("mcp %q not found", id)
		}
		definition.Retention = cloneRetention(state.MCPs[index].Retention)
		result = cloneMCPDefinition(definition)
		state.MCPs[index] = result
		sort.Slice(state.MCPs, func(i, j int) bool {
			return strings.ToLower(state.MCPs[i].Name) < strings.ToLower(state.MCPs[j].Name)
		})
		return nil
	})
	return cloneMCPDefinition(result), err
}

func ValidateMCPConfig(name string, config MCPConfig) (model.MCPDefinition, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return model.MCPDefinition{}, fmt.Errorf("mcp name is required")
	}
	return validateMCPDefinition(model.MCPDefinition{
		Name:                name,
		DefaultIncludeInEnv: config.DefaultInclude,
		Transport:           config.Transport,
		AuthMode:            config.AuthMode,
		Endpoint:            config.Endpoint,
		HeaderRefs:          cloneStringMap(config.HeaderRefs),
		Executable:          config.Executable,
		Args:                append([]string(nil), config.Args...),
		EnvRefs:             cloneStringMap(config.EnvRefs),
		HealthPolicy:        config.HealthPolicy,
	})
}

func (s *MCPService) ApplyBatch(items []MCPNamedConfig, conflictPolicy string) (MCPBatchResult, error) {
	conflictPolicy = strings.TrimSpace(conflictPolicy)
	if conflictPolicy == "" {
		conflictPolicy = MCPConflictError
	}
	switch conflictPolicy {
	case MCPConflictError, MCPConflictSkip, MCPConflictUpdateByName:
	default:
		return MCPBatchResult{}, fmt.Errorf("unsupported mcp conflict policy %q", conflictPolicy)
	}

	validated := make([]model.MCPDefinition, 0, len(items))
	seen := make(map[string]struct{}, len(items))
	for _, item := range items {
		definition, err := ValidateMCPConfig(item.Name, item.Config)
		if err != nil {
			return MCPBatchResult{}, err
		}
		key := strings.ToLower(definition.Name)
		if _, ok := seen[key]; ok {
			return MCPBatchResult{}, fmt.Errorf("duplicate mcp name %q in batch", definition.Name)
		}
		seen[key] = struct{}{}
		validated = append(validated, definition)
	}

	result := MCPBatchResult{Mutations: make([]MCPBatchMutation, 0, len(validated))}
	err := s.store.Update(func(state *model.State) error {
		byName := make(map[string]int, len(state.MCPs))
		for i, existing := range state.MCPs {
			byName[strings.ToLower(existing.Name)] = i
		}
		for _, definition := range validated {
			key := strings.ToLower(definition.Name)
			if index, exists := byName[key]; exists {
				switch conflictPolicy {
				case MCPConflictError:
					return fmt.Errorf("mcp %q already exists", definition.Name)
				case MCPConflictSkip:
					result.Mutations = append(result.Mutations, MCPBatchMutation{Action: MCPConflictSkip, Definition: cloneMCPDefinition(state.MCPs[index])})
					continue
				case MCPConflictUpdateByName:
					definition.ID = state.MCPs[index].ID
					definition.Retention = cloneRetention(state.MCPs[index].Retention)
					state.MCPs[index] = cloneMCPDefinition(definition)
					result.Mutations = append(result.Mutations, MCPBatchMutation{Action: MCPConflictUpdateByName, Definition: cloneMCPDefinition(definition)})
					continue
				}
			}

			id, err := identity.New("mcp")
			if err != nil {
				return err
			}
			definition.ID = id
			definition.Retention = normalizeCatalogRetention(definition.Retention, s.nowUTC())
			state.MCPs = append(state.MCPs, cloneMCPDefinition(definition))
			byName[key] = len(state.MCPs) - 1
			result.Mutations = append(result.Mutations, MCPBatchMutation{Action: "add", Definition: cloneMCPDefinition(definition)})
		}
		sort.Slice(state.MCPs, func(i, j int) bool {
			return strings.ToLower(state.MCPs[i].Name) < strings.ToLower(state.MCPs[j].Name)
		})
		return nil
	})
	if err != nil {
		return MCPBatchResult{}, err
	}
	return result, nil
}

func (s *MCPService) List() ([]model.MCPDefinition, error) {
	state, err := s.store.Load()
	if err != nil {
		return nil, err
	}
	items := make([]model.MCPDefinition, len(state.MCPs))
	for i, item := range state.MCPs {
		items[i] = cloneMCPDefinition(item)
	}
	return items, nil
}

func (s *MCPService) Get(id string) (model.MCPDefinition, error) {
	items, err := s.List()
	if err != nil {
		return model.MCPDefinition{}, err
	}
	for _, item := range items {
		if item.ID == id {
			return item, nil
		}
	}
	return model.MCPDefinition{}, fmt.Errorf("mcp %q not found", id)
}

func (s *MCPService) SetDefault(id string, value bool) (model.MCPDefinition, error) {
	var result model.MCPDefinition
	err := s.store.Update(func(state *model.State) error {
		for i := range state.MCPs {
			if state.MCPs[i].ID == id {
				state.MCPs[i].DefaultIncludeInEnv = value
				result = cloneMCPDefinition(state.MCPs[i])
				return nil
			}
		}
		return fmt.Errorf("mcp %q not found", id)
	})
	return result, err
}

func (s *MCPService) Remove(id string) error {
	return s.store.Update(func(state *model.State) error {
		for i := range state.MCPs {
			if state.MCPs[i].ID == id {
				state.MCPs = append(state.MCPs[:i], state.MCPs[i+1:]...)
				removed := map[string]struct{}{id: {}}
				for envIndex := range state.Environments {
					state.Environments[envIndex].EnabledMCPIDs = removeCatalogSelections(state.Environments[envIndex].EnabledMCPIDs, removed)
				}
				for workspaceIndex := range state.Workspaces {
					state.Workspaces[workspaceIndex].EnabledMCPIDs = removeCatalogSelections(state.Workspaces[workspaceIndex].EnabledMCPIDs, removed)
				}
				return nil
			}
		}
		return fmt.Errorf("mcp %q not found", id)
	})
}

func (s *MCPService) Exists(id string) (bool, error) {
	items, err := s.List()
	if err != nil {
		return false, err
	}
	for _, item := range items {
		if item.ID == id {
			return true, nil
		}
	}
	return false, nil
}

func validateMCPDefinition(definition model.MCPDefinition) (model.MCPDefinition, error) {
	definition.Name = strings.TrimSpace(definition.Name)
	definition.Transport = strings.TrimSpace(definition.Transport)
	if definition.Transport == "" {
		definition.Transport = MCPTransportStreamableHTTP
	}
	definition.AuthMode = strings.TrimSpace(definition.AuthMode)
	if definition.AuthMode == "" {
		definition.AuthMode = MCPAuthNone
	}
	definition.Endpoint = strings.TrimSpace(definition.Endpoint)
	definition.Executable = strings.TrimSpace(definition.Executable)
	definition.HeaderRefs = trimStringMap(definition.HeaderRefs)
	definition.EnvRefs = trimStringMap(definition.EnvRefs)
	definition.Args = append([]string(nil), definition.Args...)

	if err := validateMCPHealthPolicy(definition.HealthPolicy); err != nil {
		return model.MCPDefinition{}, err
	}

	switch definition.Transport {
	case MCPTransportStreamableHTTP:
		if definition.Executable != "" || len(definition.Args) != 0 || len(definition.EnvRefs) != 0 {
			return model.MCPDefinition{}, fmt.Errorf("streamable-http mcp cannot configure stdio executable, args, or env_refs")
		}
		if !validHTTPURLTemplate(definition.Endpoint) {
			return model.MCPDefinition{}, fmt.Errorf("mcp endpoint must be a valid http(s) URL or environment-reference template")
		}
		switch definition.AuthMode {
		case MCPAuthNone:
			if len(definition.HeaderRefs) != 0 {
				return model.MCPDefinition{}, fmt.Errorf("mcp auth_mode %q cannot configure header_refs; select auth_mode %q (Secret-backed headers) or remove all header_refs", MCPAuthNone, MCPAuthHeaders)
			}
		case MCPAuthHeaders:
			if len(definition.HeaderRefs) == 0 {
				return model.MCPDefinition{}, fmt.Errorf("mcp auth_mode %q requires header_refs", MCPAuthHeaders)
			}
			for header, value := range definition.HeaderRefs {
				if !containsEnvironmentReference(value) {
					return model.MCPDefinition{}, fmt.Errorf("mcp header %q must use an environment reference", header)
				}
			}
		default:
			return model.MCPDefinition{}, fmt.Errorf("unsupported mcp auth_mode %q", definition.AuthMode)
		}
	case MCPTransportStdio:
		if definition.AuthMode != MCPAuthNone {
			return model.MCPDefinition{}, fmt.Errorf("stdio mcp auth_mode must be %q", MCPAuthNone)
		}
		if definition.Endpoint != "" || len(definition.HeaderRefs) != 0 {
			return model.MCPDefinition{}, fmt.Errorf("stdio mcp cannot configure endpoint or header_refs")
		}
		if definition.Executable == "" {
			return model.MCPDefinition{}, fmt.Errorf("stdio mcp executable is required")
		}
		for key, value := range definition.EnvRefs {
			if !containsEnvironmentReference(value) {
				return model.MCPDefinition{}, fmt.Errorf("stdio mcp env_refs[%q] must use an environment reference", key)
			}
		}
	default:
		return model.MCPDefinition{}, fmt.Errorf("unsupported mcp transport %q", definition.Transport)
	}
	return definition, nil
}

func validateMCPHealthPolicy(policy model.MCPHealthPolicy) error {
	if policy.CheckIntervalSeconds < 0 || policy.ProbeTimeoutSeconds < 0 || policy.ReconnectIntervalSeconds < 0 {
		return fmt.Errorf("mcp health policy intervals must be nonnegative")
	}
	if policy.HealthCheckEnabled {
		if policy.CheckIntervalSeconds <= 0 {
			return fmt.Errorf("mcp health_check_enabled requires positive check_interval_seconds")
		}
		if policy.ProbeTimeoutSeconds <= 0 {
			return fmt.Errorf("mcp health_check_enabled requires positive probe_timeout_seconds")
		}
	}
	if policy.AutoReconnect && policy.ReconnectIntervalSeconds <= 0 {
		return fmt.Errorf("mcp auto_reconnect requires positive reconnect_interval_seconds")
	}
	return nil
}

func validHTTPURLTemplate(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" {
		return false
	}
	expanded := os.Expand(value, func(string) string { return "adm-reference" })
	parsed, err := url.Parse(expanded)
	return err == nil && (parsed.Scheme == "http" || parsed.Scheme == "https") && parsed.Host != ""
}

func containsEnvironmentReference(value string) bool {
	found := false
	_ = os.Expand(value, func(string) string {
		found = true
		return ""
	})
	return found
}

func trimStringMap(input map[string]string) map[string]string {
	if len(input) == 0 {
		return nil
	}
	result := make(map[string]string, len(input))
	for key, value := range input {
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if key != "" {
			result[key] = value
		}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func (s *MCPService) nowUTC() time.Time {
	if s.now == nil {
		return time.Now().UTC()
	}
	return s.now().UTC()
}

func normalizeCatalogRetention(retention model.ResourceRetention, now time.Time) model.ResourceRetention {
	if strings.TrimSpace(retention.Persistence) == "" {
		retention.Persistence = model.PersistenceDurable
	}
	if strings.TrimSpace(retention.CreatorSurface) == "" {
		retention.CreatorSurface = "core"
	}
	if retention.CreatedAt == nil {
		createdAt := now.UTC()
		retention.CreatedAt = &createdAt
	}
	return retention
}

func cloneMCPDefinition(input model.MCPDefinition) model.MCPDefinition {
	input.HeaderRefs = cloneStringMap(input.HeaderRefs)
	input.EnvRefs = cloneStringMap(input.EnvRefs)
	input.Args = append([]string(nil), input.Args...)
	input.Retention = cloneRetention(input.Retention)
	return input
}

func cloneRetention(input model.ResourceRetention) model.ResourceRetention {
	cloneTime := func(value *time.Time) *time.Time {
		if value == nil {
			return nil
		}
		copy := value.UTC()
		return &copy
	}
	input.CreatedAt = cloneTime(input.CreatedAt)
	input.LastUsedAt = cloneTime(input.LastUsedAt)
	input.ExpiresAt = cloneTime(input.ExpiresAt)
	return input
}
