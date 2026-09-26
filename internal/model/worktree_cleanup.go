package model

import "time"

type ManagedWorktreeCleanupCandidate struct {
	ManagedWorktreeID string    `json:"managed_worktree_id"`
	EnvironmentID     string    `json:"environment_id"`
	Name              string    `json:"name"`
	Root              string    `json:"root"`
	Branch            string    `json:"branch"`
	CreatedAt         time.Time `json:"created_at"`
	LastActivityAt    time.Time `json:"last_activity_at"`
	InactiveSeconds   int64     `json:"inactive_seconds"`
	CleanupEligible   bool      `json:"cleanup_eligible"`
	Blockers          []string  `json:"blockers,omitempty"`
	Dirty             bool      `json:"dirty,omitempty"`
	Unpublished       bool      `json:"unpublished,omitempty"`
}

type ManagedWorktreeCleanupReport struct {
	GeneratedAt              time.Time                         `json:"generated_at"`
	InactiveThresholdSeconds int64                             `json:"inactive_threshold_seconds"`
	Candidates               []ManagedWorktreeCleanupCandidate `json:"candidates"`
}

type ManagedWorktreeCleanupMutation struct {
	ManagedWorktreeID string `json:"managed_worktree_id"`
	EnvironmentID     string `json:"environment_id"`
	Name              string `json:"name"`
	Root              string `json:"root"`
	RetainedBranch    string `json:"retained_branch"`
}

type ManagedWorktreeCleanupResult struct {
	GeneratedAt time.Time                         `json:"generated_at"`
	DryRun      bool                              `json:"dry_run"`
	Report      ManagedWorktreeCleanupReport      `json:"report"`
	Removed     []ManagedWorktreeCleanupMutation  `json:"removed,omitempty"`
	Skipped     []ManagedWorktreeCleanupCandidate `json:"skipped,omitempty"`
}
