package model

import "time"

const (
	PersistenceDurable   = "durable"
	PersistenceTemporary = "temporary"

	TemporaryEnvironmentModeExistingRoot    = "existing_root"
	TemporaryEnvironmentModeManagedWorktree = "managed_worktree"

	RetentionResourceEnvironment = "environment"
	RetentionResourceMCP         = "mcp"
	RetentionResourceSkillSource = "skill_source"
	RetentionResourceSkill       = "skill"

	RetentionCleanupDurable  = "durable"
	RetentionCleanupNotDue   = "not_due"
	RetentionCleanupBlocked  = "blocked"
	RetentionCleanupEligible = "eligible"

	RetentionCleanupActionRemove = "remove"

	RetentionUpdateActionPromoteDurable = "promote_durable"
	RetentionUpdateActionMarkTemporary  = "mark_temporary"
)

// ResourceRetention records lifecycle intent separately from the resource's
// operational configuration. Missing persistence metadata is normalized as
// durable so pre-Phase-15 state can never become cleanup-eligible by accident.
type ResourceRetention struct {
	Persistence           string     `json:"persistence"`
	CreatorSurface        string     `json:"creator_surface,omitempty"`
	OwnerID               string     `json:"owner_id,omitempty"`
	SessionID             string     `json:"session_id,omitempty"`
	RunID                 string     `json:"run_id,omitempty"`
	CreatedAt             *time.Time `json:"created_at,omitempty"`
	LastUsedAt            *time.Time `json:"last_used_at,omitempty"`
	ExpiresAt             *time.Time `json:"expires_at,omitempty"`
	Policy                string     `json:"policy,omitempty"`
	AttachedEnvironmentID string     `json:"attached_environment_id,omitempty"`
}

type ResourceRetentionItem struct {
	Kind            string            `json:"kind"`
	ID              string            `json:"id"`
	Name            string            `json:"name,omitempty"`
	Retention       ResourceRetention `json:"retention"`
	CleanupState    string            `json:"cleanup_state"`
	CleanupEligible bool              `json:"cleanup_eligible"`
	Blockers        []string          `json:"blockers,omitempty"`
	Uncertainties   []string          `json:"uncertainties,omitempty"`
}

type ResourceRetentionReport struct {
	GeneratedAt time.Time               `json:"generated_at"`
	Resources   []ResourceRetentionItem `json:"resources"`
}

type ResourceRetentionUpdateRequest struct {
	Kind                  string     `json:"kind" jsonschema:"resource kind: environment, mcp, skill_source, or skill"`
	ID                    string     `json:"id"`
	CreatorSurface        string     `json:"creator_surface,omitempty" jsonschema:"optional creator surface label; defaults to admin"`
	OwnerID               string     `json:"owner_id,omitempty" jsonschema:"required when marking temporary"`
	SessionID             string     `json:"session_id,omitempty"`
	RunID                 string     `json:"run_id,omitempty"`
	ExpiresAt             *time.Time `json:"expires_at,omitempty" jsonschema:"optional expiry; omitted temporary resources remain blocked from cleanup"`
	Policy                string     `json:"policy,omitempty"`
	AttachedEnvironmentID string     `json:"attached_environment_id,omitempty"`
}

type ResourceRetentionUpdateItem struct {
	Kind      string            `json:"kind"`
	ID        string            `json:"id"`
	Name      string            `json:"name,omitempty"`
	Retention ResourceRetention `json:"retention"`
}

type ResourceRetentionUpdateResult struct {
	Action      string                        `json:"action"`
	GeneratedAt time.Time                     `json:"generated_at"`
	Updated     []ResourceRetentionUpdateItem `json:"updated"`
}

type ResourceRetentionCleanupRequest struct {
	Execute bool `json:"execute,omitempty" jsonschema:"false or omitted returns dry-run preview only; true removes eligible temporary metadata records"`
}

type ResourceRetentionCleanupMutation struct {
	Kind   string `json:"kind"`
	ID     string `json:"id"`
	Name   string `json:"name,omitempty"`
	Action string `json:"action"`
}

type ResourceRetentionCleanupResult struct {
	GeneratedAt time.Time                          `json:"generated_at"`
	DryRun      bool                               `json:"dry_run"`
	Report      ResourceRetentionReport            `json:"report"`
	WouldRemove []ResourceRetentionCleanupMutation `json:"would_remove,omitempty"`
	Removed     []ResourceRetentionCleanupMutation `json:"removed,omitempty"`
	Skipped     []ResourceRetentionItem            `json:"skipped,omitempty"`
}

type TemporaryEnvironmentCreateRequest struct {
	WorkspaceID         string `json:"workspace_id,omitempty"`
	SourceEnvironmentID string `json:"source_environment_id,omitempty"`
	Name                string `json:"name"`
	OwnerID             string `json:"owner_id"`
	TTLSeconds          int64  `json:"ttl_seconds"`
	SessionID           string `json:"session_id,omitempty"`
	RunID               string `json:"run_id,omitempty"`
	Mode                string `json:"mode,omitempty"`
	Root                string `json:"root,omitempty"`
	BaseRef             string `json:"base_ref,omitempty"`
}

type TemporaryEnvironmentCreateResult struct {
	Mode            string           `json:"mode"`
	Environment     Environment      `json:"environment"`
	ManagedWorktree *ManagedWorktree `json:"managed_worktree,omitempty"`
}

type TemporaryEnvironmentStatus struct {
	EnvironmentID   string            `json:"environment_id"`
	WorkspaceID     string            `json:"workspace_id"`
	Name            string            `json:"name"`
	Root            string            `json:"root"`
	ManagedWorktree bool              `json:"managed_worktree"`
	Retention       ResourceRetention `json:"retention"`
	CleanupState    string            `json:"cleanup_state"`
	CleanupEligible bool              `json:"cleanup_eligible"`
	Blockers        []string          `json:"blockers,omitempty"`
	Uncertainties   []string          `json:"uncertainties,omitempty"`
}
