package model

import "time"

const WorkspaceSystemKindManagedWorktreeRoot = "managed_worktree_root"

type Workspace struct {
	ID              string    `json:"workspace_id"`
	Name            string    `json:"name"`
	Path            string    `json:"path"`
	CreatedAt       time.Time `json:"created_at"`
	SystemKind      string    `json:"system_kind,omitempty"`
	EnabledMCPIDs   []string  `json:"enabled_mcp_ids,omitempty"`
	EnabledSkillIDs []string  `json:"enabled_skill_ids,omitempty"`
}

type WorktreeSettings struct {
	Root         string `json:"root"`
	BranchPrefix string `json:"branch_prefix"`
	WorkspaceID  string `json:"workspace_id,omitempty"`
}

type GatewayAccessSettings struct {
	AllowedHosts    []string `json:"allowed_hosts,omitempty"`
	AdminAPIKeyHash string   `json:"admin_api_key_hash,omitempty"`
	AgentAPIKeyHash string   `json:"agent_api_key_hash,omitempty"`
}

type EnvironmentWorkspaceOptions struct {
	EnvironmentID          string      `json:"environment_id"`
	CurrentWorkspaceID     string      `json:"current_workspace_id"`
	RecommendedWorkspaceID string      `json:"recommended_workspace_id,omitempty"`
	Candidates             []Workspace `json:"candidates"`
	RebindAllowed          bool        `json:"rebind_allowed"`
	ManagedWorktree        bool        `json:"managed_worktree"`
	Reason                 string      `json:"reason,omitempty"`
}

type EnvironmentWorkspaceRecommendation struct {
	EnvironmentID        string    `json:"environment_id"`
	EnvironmentName      string    `json:"environment_name"`
	Root                 string    `json:"root"`
	CurrentWorkspace     Workspace `json:"current_workspace"`
	RecommendedWorkspace Workspace `json:"recommended_workspace"`
}

type WriterLease struct {
	Owner      string    `json:"owner,omitempty"`
	AcquiredAt time.Time `json:"acquired_at"`
	LastSeenAt time.Time `json:"last_seen_at"`
	ExpiresAt  time.Time `json:"expires_at"`
}

type VerifierDefinition struct {
	ID             string   `json:"verifier_id"`
	Kind           string   `json:"kind"`
	Enabled        bool     `json:"enabled"`
	Executable     string   `json:"executable"`
	Args           []string `json:"args,omitempty"`
	Cwd            string   `json:"cwd,omitempty"`
	TimeoutSeconds int64    `json:"timeout_seconds,omitempty"`
	Name           string   `json:"name,omitempty"`
}

type Environment struct {
	ID                string               `json:"environment_id"`
	WorkspaceID       string               `json:"workspace_id"`
	Name              string               `json:"name"`
	Root              string               `json:"root"`
	State             string               `json:"state"`
	CreatedAt         time.Time            `json:"created_at"`
	UpdatedAt         time.Time            `json:"updated_at"`
	LastActivityAt    time.Time            `json:"last_activity_at"`
	Retention         ResourceRetention    `json:"retention"`
	Writer            *WriterLease         `json:"writer,omitempty"`
	EnabledMCPIDs     []string             `json:"enabled_mcp_ids,omitempty"`
	EnabledSkillIDs   []string             `json:"enabled_skill_ids,omitempty"`
	ExplicitMCPIDs    []string             `json:"explicit_mcp_ids"`
	ExplicitSkillIDs  []string             `json:"explicit_skill_ids"`
	InheritedMCPIDs   []string             `json:"inherited_mcp_ids"`
	InheritedSkillIDs []string             `json:"inherited_skill_ids"`
	Verifiers         []VerifierDefinition `json:"verifiers,omitempty"`
	PrivateMemory     map[string]string    `json:"private_memory,omitempty"`
}

type MCPHealthPolicy struct {
	HealthCheckEnabled       bool  `json:"health_check_enabled"`
	CheckIntervalSeconds     int64 `json:"check_interval_seconds,omitempty"`
	ProbeTimeoutSeconds      int64 `json:"probe_timeout_seconds,omitempty"`
	AutoReconnect            bool  `json:"auto_reconnect"`
	ReconnectIntervalSeconds int64 `json:"reconnect_interval_seconds,omitempty"`
}

type MCPDefinition struct {
	ID                  string            `json:"id"`
	Name                string            `json:"name"`
	DefaultIncludeInEnv bool              `json:"default_include_in_environment"`
	Transport           string            `json:"transport"`
	AuthMode            string            `json:"auth_mode"`
	Endpoint            string            `json:"endpoint,omitempty"`
	HeaderRefs          map[string]string `json:"header_refs,omitempty"`
	Executable          string            `json:"executable,omitempty"`
	Args                []string          `json:"args,omitempty"`
	EnvRefs             map[string]string `json:"env_refs,omitempty"`
	HealthPolicy        MCPHealthPolicy   `json:"health_policy"`
	Retention           ResourceRetention `json:"retention"`
}

type SkillSource struct {
	ID                  string            `json:"skill_source_id"`
	Root                string            `json:"root"`
	SupportRoots        []string          `json:"support_roots,omitempty"`
	DefaultIncludeInEnv bool              `json:"default_include_in_environment"`
	CreatedAt           time.Time         `json:"created_at"`
	UpdatedAt           time.Time         `json:"updated_at"`
	LastRefreshAt       *time.Time        `json:"last_refresh_at,omitempty"`
	LastRefreshStatus   string            `json:"last_refresh_status,omitempty"`
	LastRefreshError    string            `json:"last_refresh_error,omitempty"`
	Retention           ResourceRetention `json:"retention"`
}

type CatalogEntry struct {
	ID                   string            `json:"id"`
	SourceID             string            `json:"source_id,omitempty"`
	Name                 string            `json:"name"`
	DefaultIncludeInEnv  bool              `json:"default_include_in_environment"`
	Instructions         string            `json:"instructions,omitempty"`
	ArtifactPath         string            `json:"artifact_path,omitempty"`
	RelativeArtifactPath string            `json:"relative_artifact_path,omitempty"`
	SourceRoot           string            `json:"source_root,omitempty"`
	SupportRoots         []string          `json:"support_roots,omitempty"`
	Retention            ResourceRetention `json:"retention"`
}

type ManagedWorktree struct {
	ID                  string    `json:"managed_worktree_id"`
	EnvironmentID       string    `json:"environment_id"`
	WorkspaceID         string    `json:"workspace_id"`
	SourceEnvironmentID string    `json:"source_environment_id,omitempty"`
	SourceRoot          string    `json:"source_root,omitempty"`
	Root                string    `json:"root"`
	Branch              string    `json:"branch"`
	BaseCommit          string    `json:"base_commit"`
	GitCommonDir        string    `json:"git_common_dir"`
	CreatedAt           time.Time `json:"created_at"`
}

type ExecDenial struct {
	Executable        string    `json:"executable"`
	Count             int       `json:"count"`
	FirstBlockedAt    time.Time `json:"first_blocked_at"`
	LastBlockedAt     time.Time `json:"last_blocked_at"`
	LastEnvironmentID string    `json:"last_environment_id,omitempty"`
	LastSurface       string    `json:"last_surface,omitempty"`
	LastReason        string    `json:"last_reason,omitempty"`
}

type State struct {
	Version               int                   `json:"version"`
	Workspaces            []Workspace           `json:"workspaces"`
	Environments          []Environment         `json:"environments"`
	ManagedWorktrees      []ManagedWorktree     `json:"managed_worktrees,omitempty"`
	WorktreeSettings      WorktreeSettings      `json:"worktree_settings"`
	GatewayAccess         GatewayAccessSettings `json:"gateway_access"`
	AllowedExecutables    []string              `json:"allowed_executables,omitempty"`
	ExecFullAuthorization bool                  `json:"exec_full_authorization,omitempty"`
	ExecDenials           []ExecDenial          `json:"exec_denials,omitempty"`
	MCPs                  []MCPDefinition       `json:"mcps,omitempty"`
	SkillSources          []SkillSource         `json:"skill_sources,omitempty"`
	Skills                []CatalogEntry        `json:"skills,omitempty"`
	GlobalMemory          map[string]string     `json:"global_memory,omitempty"`
}
