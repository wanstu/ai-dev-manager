package model

// ManagedWorktreeRetentionMigrationResult changes retention metadata only.
// No worktree directory, Git branch, or Environment is deleted by migration.
type ManagedWorktreeRetentionMigrationResult struct {
	DryRun       bool     `json:"dry_run"`
	GraceDays    int      `json:"grace_days"`
	ManagedCount int      `json:"managed_count"`
	Candidates   []string `json:"candidates"`
	Converted    []string `json:"converted,omitempty"`
}
