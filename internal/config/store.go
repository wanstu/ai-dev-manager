package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"ai-dev-manager/internal/model"
)

const SchemaVersion = 1

const (
	userConfigFilename = "config.json"
	profilesDirname    = "profiles"
	projectDirname     = ".ai-dev-manager"
)

// UserConfig is the persisted user-level state consumed by the control plane.
// Runtime-only state deliberately does not belong here.
type UserConfig struct {
	Version    int
	Global     model.ConfigLayer
	Workspaces []model.Workspace
}

// StoreErrorKind lets callers distinguish persistence failures without parsing
// user-facing error strings.
type StoreErrorKind string

const (
	StoreErrNotFound           StoreErrorKind = "not_found"
	StoreErrDecode             StoreErrorKind = "decode"
	StoreErrUnsupportedVersion StoreErrorKind = "unsupported_version"
	StoreErrInvalidID          StoreErrorKind = "invalid_id"
	StoreErrInvalidScope       StoreErrorKind = "invalid_scope"
	StoreErrInvalidDefinition  StoreErrorKind = "invalid_definition"
	StoreErrIO                 StoreErrorKind = "io"
)

// StoreError intentionally avoids embedding configuration values so MCP env
// data cannot leak through ordinary error formatting.
type StoreError struct {
	Kind     StoreErrorKind
	Path     string
	Entity   string
	ID       string
	Expected model.Scope
	Actual   model.Scope
	Err      error
}

func (e *StoreError) Error() string {
	switch e.Kind {
	case StoreErrNotFound:
		return fmt.Sprintf("%s %q not found", e.Entity, e.ID)
	case StoreErrDecode:
		return fmt.Sprintf("decode config %q: %v", e.Path, e.Err)
	case StoreErrUnsupportedVersion:
		return fmt.Sprintf("unsupported config version in %q", e.Path)
	case StoreErrInvalidID:
		return fmt.Sprintf("invalid %s id %q", e.Entity, e.ID)
	case StoreErrInvalidScope:
		return fmt.Sprintf("invalid config scope: expected %q, got %q", e.Expected, e.Actual)
	case StoreErrInvalidDefinition:
		return fmt.Sprintf("invalid %s definition %q", e.Entity, e.ID)
	case StoreErrIO:
		return fmt.Sprintf("config IO error for %q: %v", e.Path, e.Err)
	default:
		return "config store error"
	}
}

func (e *StoreError) Unwrap() error { return e.Err }

// Store owns on-disk configuration paths. Tests can inject a temporary root.
type Store struct {
	root string
}

func NewStore(root string) *Store {
	return &Store{root: filepath.Clean(root)}
}

func DefaultRoot() (string, error) {
	root, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "ai-dev-manager"), nil
}

func (s *Store) Root() string { return s.root }

func (s *Store) LoadUserConfig() (UserConfig, error) {
	path := filepath.Join(s.root, userConfigFilename)
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return defaultUserConfig(), nil
	}
	if err != nil {
		return UserConfig{}, ioStoreError(path, err)
	}

	var dto userConfigDTO
	if err := json.Unmarshal(data, &dto); err != nil {
		return UserConfig{}, &StoreError{Kind: StoreErrDecode, Path: path, Err: err}
	}
	if err := checkVersion(path, dto.Version); err != nil {
		return UserConfig{}, err
	}

	cfg := UserConfig{
		Version:    dto.Version,
		Global:     dto.Global.toLayer(model.ScopeGlobal),
		Workspaces: make([]model.Workspace, len(dto.Workspaces)),
	}
	for i, workspace := range dto.Workspaces {
		cfg.Workspaces[i] = workspace.toModel()
	}
	if err := validatePersistentLayer(cfg.Global, model.ScopeGlobal); err != nil {
		return UserConfig{}, err
	}
	if err := validateWorkspaceRecords(cfg.Workspaces); err != nil {
		return UserConfig{}, err
	}
	return cfg, nil
}

func (s *Store) SaveUserConfig(cfg UserConfig) error {
	if cfg.Version == 0 {
		cfg.Version = SchemaVersion
	}
	if cfg.Version != SchemaVersion {
		return &StoreError{Kind: StoreErrUnsupportedVersion, Path: filepath.Join(s.root, userConfigFilename)}
	}
	if err := validatePersistentLayer(cfg.Global, model.ScopeGlobal); err != nil {
		return err
	}
	if err := validateWorkspaceRecords(cfg.Workspaces); err != nil {
		return err
	}

	dto := userConfigDTO{
		Version:    cfg.Version,
		Global:     layerDTOFromModel(cfg.Global),
		Workspaces: make([]workspaceDTO, len(cfg.Workspaces)),
	}
	for i, workspace := range cfg.Workspaces {
		dto.Workspaces[i] = workspaceDTOFromModel(workspace)
	}
	return atomicWriteJSON(filepath.Join(s.root, userConfigFilename), dto)
}

func (s *Store) LoadProfile(id string) (model.ConfigLayer, error) {
	if err := validateProfileID(id); err != nil {
		return model.ConfigLayer{}, err
	}
	path := filepath.Join(s.root, profilesDirname, id+".json")
	return loadLayerFile(path, model.ScopeProfile, true, "profile", id)
}

func (s *Store) SaveProfile(id string, layer model.ConfigLayer) error {
	if err := validateProfileID(id); err != nil {
		return err
	}
	if err := validatePersistentLayer(layer, model.ScopeProfile); err != nil {
		return err
	}
	path := filepath.Join(s.root, profilesDirname, id+".json")
	return atomicWriteJSON(path, layerFileDTOFromModel(layer))
}

func (s *Store) LoadProject(workspacePath string) (model.ConfigLayer, error) {
	path, err := projectConfigPath(workspacePath)
	if err != nil {
		return model.ConfigLayer{}, err
	}
	return loadLayerFile(path, model.ScopeProject, false, "project", workspacePath)
}

func (s *Store) SaveProject(workspacePath string, layer model.ConfigLayer) error {
	path, err := projectConfigPath(workspacePath)
	if err != nil {
		return err
	}
	if err := validatePersistentLayer(layer, model.ScopeProject); err != nil {
		return err
	}
	return atomicWriteJSON(path, layerFileDTOFromModel(layer))
}

func defaultUserConfig() UserConfig {
	return UserConfig{
		Version: SchemaVersion,
		Global: model.ConfigLayer{
			Scope:  model.ScopeGlobal,
			MCPs:   map[string]model.MCPDefinition{},
			Skills: map[string]model.SkillDefinition{},
		},
		Workspaces: []model.Workspace{},
	}
}

func loadLayerFile(path string, scope model.Scope, missingIsError bool, entity, id string) (model.ConfigLayer, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		if missingIsError {
			return model.ConfigLayer{}, &StoreError{Kind: StoreErrNotFound, Path: path, Entity: entity, ID: id}
		}
		return model.ConfigLayer{Scope: scope, MCPs: map[string]model.MCPDefinition{}, Skills: map[string]model.SkillDefinition{}}, nil
	}
	if err != nil {
		return model.ConfigLayer{}, ioStoreError(path, err)
	}

	var dto layerFileDTO
	if err := json.Unmarshal(data, &dto); err != nil {
		return model.ConfigLayer{}, &StoreError{Kind: StoreErrDecode, Path: path, Err: err}
	}
	if err := checkVersion(path, dto.Version); err != nil {
		return model.ConfigLayer{}, err
	}
	layer := dto.layerDTO.toLayer(scope)
	if err := validatePersistentLayer(layer, scope); err != nil {
		return model.ConfigLayer{}, err
	}
	return layer, nil
}

func checkVersion(path string, version int) error {
	if version != SchemaVersion {
		return &StoreError{Kind: StoreErrUnsupportedVersion, Path: path}
	}
	return nil
}

func validateProfileID(id string) error {
	if id == "" || id == "." || id == ".." || strings.ContainsAny(id, `/\\`) {
		return &StoreError{Kind: StoreErrInvalidID, Entity: "profile", ID: id}
	}
	return nil
}

func projectConfigPath(workspacePath string) (string, error) {
	if strings.TrimSpace(workspacePath) == "" || !filepath.IsAbs(workspacePath) {
		return "", &StoreError{Kind: StoreErrInvalidID, Entity: "workspace_path", ID: workspacePath}
	}
	return filepath.Join(filepath.Clean(workspacePath), projectDirname, userConfigFilename), nil
}

func validatePersistentLayer(layer model.ConfigLayer, expected model.Scope) error {
	if layer.Scope != expected {
		return &StoreError{Kind: StoreErrInvalidScope, Expected: expected, Actual: layer.Scope}
	}
	for key, definition := range layer.MCPs {
		if key == "" || definition.ID == "" || key != definition.ID {
			return &StoreError{Kind: StoreErrInvalidDefinition, Entity: "mcp", ID: definition.ID}
		}
	}
	for key, definition := range layer.Skills {
		if key == "" || definition.ID == "" || key != definition.ID {
			return &StoreError{Kind: StoreErrInvalidDefinition, Entity: "skill", ID: definition.ID}
		}
	}
	for key, definition := range layer.Verifiers {
		if key == "" || definition.ID == "" || key != definition.ID {
			return &StoreError{Kind: StoreErrInvalidDefinition, Entity: "verifier", ID: definition.ID}
		}
	}
	return nil
}

func validateWorkspaceRecords(workspaces []model.Workspace) error {
	seenIDs := make(map[string]struct{}, len(workspaces))
	seenPaths := make(map[string]struct{}, len(workspaces))
	for _, workspace := range workspaces {
		if workspace.ID == "" || strings.TrimSpace(workspace.Path) == "" || !filepath.IsAbs(workspace.Path) {
			return &StoreError{Kind: StoreErrInvalidDefinition, Entity: "workspace", ID: workspace.ID}
		}
		if workspace.ProfileID != "" {
			if err := validateProfileID(workspace.ProfileID); err != nil {
				return err
			}
		}
		if _, exists := seenIDs[workspace.ID]; exists {
			return &StoreError{Kind: StoreErrInvalidDefinition, Entity: "workspace", ID: workspace.ID}
		}
		seenIDs[workspace.ID] = struct{}{}

		canonicalPath := strings.ToLower(filepath.Clean(workspace.Path))
		if _, exists := seenPaths[canonicalPath]; exists {
			return &StoreError{Kind: StoreErrInvalidDefinition, Entity: "workspace", ID: workspace.ID}
		}
		seenPaths[canonicalPath] = struct{}{}
	}
	return nil
}

func atomicWriteJSON(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return ioStoreError(path, err)
	}
	data = append(data, '\n')

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return ioStoreError(path, err)
	}

	temp, err := os.CreateTemp(dir, ".ai-dev-manager-*.tmp")
	if err != nil {
		return ioStoreError(path, err)
	}
	tempPath := temp.Name()
	cleanup := func() {
		_ = temp.Close()
		_ = os.Remove(tempPath)
	}

	if _, err := temp.Write(data); err != nil {
		cleanup()
		return ioStoreError(path, err)
	}
	if err := temp.Sync(); err != nil {
		cleanup()
		return ioStoreError(path, err)
	}
	if err := temp.Close(); err != nil {
		_ = os.Remove(tempPath)
		return ioStoreError(path, err)
	}
	if err := os.Rename(tempPath, path); err != nil {
		_ = os.Remove(tempPath)
		return ioStoreError(path, err)
	}
	return nil
}

func ioStoreError(path string, err error) error {
	return &StoreError{Kind: StoreErrIO, Path: path, Err: err}
}

type userConfigDTO struct {
	Version    int            `json:"version"`
	Global     layerDTO       `json:"global"`
	Workspaces []workspaceDTO `json:"workspaces"`
}

type layerFileDTO struct {
	Version int `json:"version"`
	layerDTO
}

type layerDTO struct {
	MCPs       map[string]mcpDTO      `json:"mcps"`
	Skills     map[string]skillDTO    `json:"skills"`
	SkillRoots []string               `json:"skill_roots,omitempty"`
	Verifiers  map[string]verifierDTO `json:"verifiers,omitempty"`
	Policy     *policyDTO             `json:"policy"`
}

type mcpDTO struct {
	ID        string            `json:"id"`
	Enabled   *bool             `json:"enabled,omitempty"`
	Transport string            `json:"transport,omitempty"`
	Command   string            `json:"command,omitempty"`
	URL       string            `json:"url,omitempty"`
	Env       map[string]string `json:"env,omitempty"`
	EnvRefs   map[string]string `json:"env_refs,omitempty"`
}

type skillDTO struct {
	ID      string `json:"id"`
	Enabled *bool  `json:"enabled,omitempty"`
	Path    string `json:"path,omitempty"`
}

type verifierDTO struct {
	ID             string   `json:"id"`
	Kind           string   `json:"kind,omitempty"`
	Enabled        *bool    `json:"enabled,omitempty"`
	Executable     string   `json:"executable,omitempty"`
	Args           []string `json:"args,omitempty"`
	Cwd            string   `json:"cwd,omitempty"`
	TimeoutSeconds int      `json:"timeout_seconds,omitempty"`
}

type policyDTO struct {
	Mode               string            `json:"mode"`
	AllowedExecutables []string          `json:"allowed_executables,omitempty"`
	ToolPaths          map[string]string `json:"tool_paths,omitempty"`
}

type workspaceDTO struct {
	ID        string `json:"id"`
	Path      string `json:"path"`
	ProfileID string `json:"profile_id,omitempty"`
	RuntimeID string `json:"runtime_id,omitempty"`
}

func layerFileDTOFromModel(layer model.ConfigLayer) layerFileDTO {
	return layerFileDTO{Version: SchemaVersion, layerDTO: layerDTOFromModel(layer)}
}

func layerDTOFromModel(layer model.ConfigLayer) layerDTO {
	dto := layerDTO{
		MCPs:       make(map[string]mcpDTO, len(layer.MCPs)),
		Skills:     make(map[string]skillDTO, len(layer.Skills)),
		SkillRoots: append([]string(nil), layer.SkillRoots...),
	}
	if layer.Verifiers != nil {
		dto.Verifiers = make(map[string]verifierDTO, len(layer.Verifiers))
	}
	for key, definition := range layer.MCPs {
		dto.MCPs[key] = mcpDTOFromModel(definition)
	}
	for key, definition := range layer.Skills {
		dto.Skills[key] = skillDTOFromModel(definition)
	}
	for key, definition := range layer.Verifiers {
		dto.Verifiers[key] = verifierDTOFromModel(definition)
	}
	if layer.Policy != nil {
		dto.Policy = &policyDTO{
			Mode:               layer.Policy.Mode,
			AllowedExecutables: append([]string(nil), layer.Policy.AllowedExecutables...),
			ToolPaths:          cloneMap(layer.Policy.ToolPaths),
		}
	}
	return dto
}

func (d layerDTO) toLayer(scope model.Scope) model.ConfigLayer {
	layer := model.ConfigLayer{
		Scope:      scope,
		MCPs:       make(map[string]model.MCPDefinition, len(d.MCPs)),
		Skills:     make(map[string]model.SkillDefinition, len(d.Skills)),
		SkillRoots: append([]string(nil), d.SkillRoots...),
	}
	if d.Verifiers != nil {
		layer.Verifiers = make(map[string]model.VerifierDefinition, len(d.Verifiers))
	}
	for key, definition := range d.MCPs {
		layer.MCPs[key] = definition.toModel()
	}
	for key, definition := range d.Skills {
		layer.Skills[key] = definition.toModel()
	}
	for key, definition := range d.Verifiers {
		layer.Verifiers[key] = definition.toModel()
	}
	if d.Policy != nil {
		layer.Policy = &model.Policy{
			Mode:               d.Policy.Mode,
			AllowedExecutables: append([]string(nil), d.Policy.AllowedExecutables...),
			ToolPaths:          cloneMap(d.Policy.ToolPaths),
		}
	}
	return layer
}

func mcpDTOFromModel(definition model.MCPDefinition) mcpDTO {
	return mcpDTO{
		ID:        definition.ID,
		Enabled:   cloneBool(definition.Enabled),
		Transport: definition.Transport,
		Command:   definition.Command,
		URL:       definition.URL,
		Env:       cloneMap(definition.Env),
		EnvRefs:   cloneMap(definition.EnvRefs),
	}
}

func (d mcpDTO) toModel() model.MCPDefinition {
	return model.MCPDefinition{
		ID:        d.ID,
		Enabled:   cloneBool(d.Enabled),
		Transport: d.Transport,
		Command:   d.Command,
		URL:       d.URL,
		Env:       cloneMap(d.Env),
		EnvRefs:   cloneMap(d.EnvRefs),
	}
}

func skillDTOFromModel(definition model.SkillDefinition) skillDTO {
	return skillDTO{ID: definition.ID, Enabled: cloneBool(definition.Enabled), Path: definition.Path}
}

func (d skillDTO) toModel() model.SkillDefinition {
	return model.SkillDefinition{ID: d.ID, Enabled: cloneBool(d.Enabled), Path: d.Path}
}

func verifierDTOFromModel(definition model.VerifierDefinition) verifierDTO {
	return verifierDTO{
		ID:             definition.ID,
		Kind:           definition.Kind,
		Enabled:        cloneBool(definition.Enabled),
		Executable:     definition.Executable,
		Args:           append([]string(nil), definition.Args...),
		Cwd:            definition.Cwd,
		TimeoutSeconds: definition.TimeoutSeconds,
	}
}

func (d verifierDTO) toModel() model.VerifierDefinition {
	return model.VerifierDefinition{
		ID:             d.ID,
		Kind:           d.Kind,
		Enabled:        cloneBool(d.Enabled),
		Executable:     d.Executable,
		Args:           append([]string(nil), d.Args...),
		Cwd:            d.Cwd,
		TimeoutSeconds: d.TimeoutSeconds,
	}
}

func workspaceDTOFromModel(workspace model.Workspace) workspaceDTO {
	return workspaceDTO{ID: workspace.ID, Path: workspace.Path, ProfileID: workspace.ProfileID, RuntimeID: workspace.RuntimeID}
}

func (d workspaceDTO) toModel() model.Workspace {
	return model.Workspace{ID: d.ID, Path: d.Path, ProfileID: d.ProfileID, RuntimeID: d.RuntimeID}
}

func cloneBool(value *bool) *bool {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}

func cloneMap(values map[string]string) map[string]string {
	if values == nil {
		return nil
	}
	copy := make(map[string]string, len(values))
	for key, value := range values {
		copy[key] = value
	}
	return copy
}
