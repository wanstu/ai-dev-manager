package controlplane

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"ai-dev-manager/internal/adapter/runtimeadapter"
	"ai-dev-manager/internal/config"
	"ai-dev-manager/internal/host"
	admmcp "ai-dev-manager/internal/mcp"
	"ai-dev-manager/internal/model"
	admruntime "ai-dev-manager/internal/runtime"
	"ai-dev-manager/internal/workspace"
)

const NativeRuntimeID = "native"

type ErrorKind string

const (
	ErrUnsupportedRuntime ErrorKind = "unsupported_runtime"
)

type Error struct {
	Kind      ErrorKind
	RuntimeID string
	Err       error
}

func (e *Error) Error() string {
	switch e.Kind {
	case ErrUnsupportedRuntime:
		return fmt.Sprintf("unsupported runtime %q", e.RuntimeID)
	default:
		return "control plane error"
	}
}

func (e *Error) Unwrap() error { return e.Err }

type Service struct {
	store         *config.Store
	registry      *workspace.Registry
	configService *workspace.ConfigService
	host          *host.Manager
	activator     *admmcp.Activator
}

func New(root string) (*Service, error) {
	if strings.TrimSpace(root) == "" {
		resolvedRoot, err := config.DefaultRoot()
		if err != nil {
			return nil, err
		}
		root = resolvedRoot
	}
	store := config.NewStore(root)
	registry := workspace.NewRegistry(store)
	return &Service{
		store:         store,
		registry:      registry,
		configService: workspace.NewConfigService(store, registry),
		host:          host.NewManager(),
		activator:     admmcp.NewActivator(nil),
	}, nil
}

func (s *Service) Store() *config.Store { return s.store }

func (s *Service) Registry() *workspace.Registry { return s.registry }

func (s *Service) Resolve(workspaceID string, runtimeOverride *model.ConfigLayer) (model.Workspace, model.EffectiveConfig, error) {
	ws, err := s.registry.Get(workspaceID)
	if err != nil {
		return model.Workspace{}, model.EffectiveConfig{}, err
	}
	cfg, err := s.configService.ResolveWorkspace(workspaceID, runtimeOverride)
	if err != nil {
		return model.Workspace{}, model.EffectiveConfig{}, err
	}
	return ws, cfg, nil
}

func (s *Service) BuildRuntime(workspaceID string, runtimeOverride *model.ConfigLayer) (runtimeadapter.Runtime, error) {
	ws, cfg, err := s.Resolve(workspaceID, runtimeOverride)
	if err != nil {
		return nil, err
	}
	return buildRuntime(ws, cfg)
}

func buildRuntime(ws model.Workspace, cfg model.EffectiveConfig) (runtimeadapter.Runtime, error) {
	runtimeID := selectedRuntimeID(ws.RuntimeID)
	if runtimeID != NativeRuntimeID {
		return nil, &Error{Kind: ErrUnsupportedRuntime, RuntimeID: runtimeID}
	}
	native, err := admruntime.NewNative(ws, cfg)
	if err != nil {
		return nil, err
	}
	return runtimeadapter.NewNative(NativeRuntimeID+":"+ws.ID, native)
}

func selectedRuntimeID(configured string) string {
	configured = strings.TrimSpace(configured)
	if configured == "" {
		return NativeRuntimeID
	}
	return configured
}

type Snapshot struct {
	Workspace WorkspaceSnapshot  `json:"workspace"`
	Runtime   RuntimeSnapshot    `json:"runtime"`
	MCPs      []MCPSnapshot      `json:"mcps"`
	Skills    []SkillSnapshot    `json:"skills"`
	Verifiers []VerifierSnapshot `json:"verifiers"`
}

type WorkspaceSnapshot struct {
	ID        string `json:"id"`
	Path      string `json:"path"`
	ProfileID string `json:"profile_id,omitempty"`
	RuntimeID string `json:"runtime_id"`
}

type RuntimeSnapshot struct {
	AdapterID    string      `json:"adapter_id"`
	RuntimeID    string      `json:"runtime_id"`
	PolicyMode   string      `json:"policy_mode"`
	PolicySource model.Scope `json:"policy_source,omitempty"`
	Capabilities []string    `json:"capabilities"`
}

type MCPSnapshot struct {
	ID            string      `json:"id"`
	Enabled       *bool       `json:"enabled,omitempty"`
	Source        model.Scope `json:"source"`
	EnabledSource model.Scope `json:"enabled_source,omitempty"`
	Transport     string      `json:"transport,omitempty"`
	Command       string      `json:"command,omitempty"`
	Args          []string    `json:"args,omitempty"`
	URL           string      `json:"url,omitempty"`
	EnvRefNames   []string    `json:"env_ref_names,omitempty"`
}

type SkillSnapshot struct {
	ID            string      `json:"id"`
	Enabled       *bool       `json:"enabled,omitempty"`
	Source        model.Scope `json:"source"`
	EnabledSource model.Scope `json:"enabled_source,omitempty"`
	Path          string      `json:"path,omitempty"`
}

type VerifierSnapshot struct {
	ID             string      `json:"id"`
	Kind           string      `json:"kind,omitempty"`
	Enabled        *bool       `json:"enabled,omitempty"`
	Source         model.Scope `json:"source"`
	EnabledSource  model.Scope `json:"enabled_source,omitempty"`
	Executable     string      `json:"executable,omitempty"`
	Args           []string    `json:"args,omitempty"`
	Cwd            string      `json:"cwd,omitempty"`
	TimeoutSeconds int         `json:"timeout_seconds,omitempty"`
}

func (s *Service) Inspect(workspaceID string, runtimeOverride *model.ConfigLayer) (Snapshot, error) {
	ws, cfg, err := s.Resolve(workspaceID, runtimeOverride)
	if err != nil {
		return Snapshot{}, err
	}
	adapter, err := buildRuntime(ws, cfg)
	if err != nil {
		return Snapshot{}, err
	}

	policyMode := string(admruntime.ModeReadOnly)
	var policySource model.Scope
	if cfg.Policy != nil {
		if strings.TrimSpace(cfg.Policy.Policy.Mode) != "" {
			policyMode = cfg.Policy.Policy.Mode
		}
		policySource = cfg.Policy.Source
	}

	snapshot := Snapshot{
		Workspace: WorkspaceSnapshot{
			ID: ws.ID, Path: ws.Path, ProfileID: ws.ProfileID, RuntimeID: selectedRuntimeID(ws.RuntimeID),
		},
		Runtime: RuntimeSnapshot{
			AdapterID: adapter.ID(), RuntimeID: selectedRuntimeID(ws.RuntimeID), PolicyMode: policyMode,
			PolicySource: policySource, Capabilities: adapter.Capabilities(),
		},
		MCPs:      mcpSnapshots(cfg.MCPs),
		Skills:    skillSnapshots(cfg.Skills),
		Verifiers: verifierSnapshots(cfg.Verifiers),
	}
	return snapshot, nil
}

func (s *Service) StartMCP(instanceID, workspaceID string, runtimeOverride *model.ConfigLayer, addr string) (host.Instance, error) {
	adapter, err := s.BuildRuntime(workspaceID, runtimeOverride)
	if err != nil {
		return host.Instance{}, err
	}
	return s.host.StartHTTP(instanceID, adapter, addr)
}

func (s *Service) GetMCP(instanceID string) (host.Instance, bool) { return s.host.Get(instanceID) }
func (s *Service) ListMCP() []host.Instance                       { return s.host.List() }
func (s *Service) StopMCP(ctx context.Context, instanceID string) error {
	return s.host.Stop(ctx, instanceID)
}
func (s *Service) StopAll(ctx context.Context) error {
	var errs []error
	if err := s.activator.StopAll(); err != nil {
		errs = append(errs, err)
	}
	if err := s.host.StopAll(ctx); err != nil {
		errs = append(errs, err)
	}
	return errors.Join(errs...)
}

func (s *Service) ActivateConfiguredMCPs(ctx context.Context, workspaceID string, runtimeOverride *model.ConfigLayer) ([]admmcp.ActivationStatus, error) {
	_, cfg, err := s.Resolve(workspaceID, runtimeOverride)
	if err != nil {
		return nil, err
	}
	registry := admmcp.FromEffective(cfg)
	entries := registry.Enabled()
	statuses := make([]admmcp.ActivationStatus, 0, len(entries))
	var errs []error
	for _, entry := range entries {
		status, activateErr := s.activator.Activate(ctx, workspaceID, entry)
		statuses = append(statuses, status)
		if activateErr != nil {
			errs = append(errs, activateErr)
		}
	}
	return statuses, errors.Join(errs...)
}

func (s *Service) MCPStatuses(workspaceID string, runtimeOverride *model.ConfigLayer) ([]admmcp.ActivationStatus, error) {
	_, cfg, err := s.Resolve(workspaceID, runtimeOverride)
	if err != nil {
		return nil, err
	}
	entries := admmcp.FromEffective(cfg).List()
	statuses := make([]admmcp.ActivationStatus, 0, len(entries))
	for _, entry := range entries {
		if current, ok := s.activator.Get(workspaceID, entry.Definition.ID); ok {
			current.Source = entry.Source
			current.EnabledSource = entry.EnabledSource
			statuses = append(statuses, current)
			continue
		}
		health := admmcp.HealthUnprobed
		if entry.Definition.Enabled != nil && !*entry.Definition.Enabled {
			health = admmcp.HealthDisabled
		}
		statuses = append(statuses, admmcp.ActivationStatus{
			WorkspaceID:   workspaceID,
			ID:            entry.Definition.ID,
			Transport:     entry.Definition.Transport,
			Health:        health,
			Source:        entry.Source,
			EnabledSource: entry.EnabledSource,
		})
	}
	return statuses, nil
}

func (s *Service) StopConfiguredMCP(workspaceID, mcpID string) error {
	return s.activator.Stop(workspaceID, mcpID)
}

func (s *Service) StopConfiguredMCPs(workspaceID string) error {
	return s.activator.StopWorkspace(workspaceID)
}

func mcpSnapshots(values map[string]model.ResolvedMCP) []MCPSnapshot {
	ids := sortedKeys(values)
	result := make([]MCPSnapshot, 0, len(ids))
	for _, id := range ids {
		value := values[id]
		result = append(result, MCPSnapshot{
			ID: id, Enabled: cloneBool(value.Enabled), Source: value.Source, EnabledSource: value.EnabledSource,
			Transport: value.Transport, Command: value.Command, Args: append([]string(nil), value.Args...), URL: value.URL, EnvRefNames: sortedStringKeys(value.EnvRefs),
		})
	}
	return result
}

func skillSnapshots(values map[string]model.ResolvedSkill) []SkillSnapshot {
	ids := sortedKeys(values)
	result := make([]SkillSnapshot, 0, len(ids))
	for _, id := range ids {
		value := values[id]
		result = append(result, SkillSnapshot{
			ID: id, Enabled: cloneBool(value.Enabled), Source: value.Source, EnabledSource: value.EnabledSource, Path: value.Path,
		})
	}
	return result
}

func verifierSnapshots(values map[string]model.ResolvedVerifier) []VerifierSnapshot {
	ids := sortedKeys(values)
	result := make([]VerifierSnapshot, 0, len(ids))
	for _, id := range ids {
		value := values[id]
		result = append(result, VerifierSnapshot{
			ID: id, Kind: value.Kind, Enabled: cloneBool(value.Enabled), Source: value.Source, EnabledSource: value.EnabledSource,
			Executable: value.Executable, Args: append([]string(nil), value.Args...), Cwd: value.Cwd, TimeoutSeconds: value.TimeoutSeconds,
		})
	}
	return result
}

func sortedKeys[T any](values map[string]T) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func sortedStringKeys(values map[string]string) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func cloneBool(value *bool) *bool {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}
