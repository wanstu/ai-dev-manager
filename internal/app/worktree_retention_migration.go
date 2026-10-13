package app

import (
	"fmt"
	"sort"
	"time"

	"ai-dev-manager-v2/internal/model"
)

const managedWorktreeGraceDays = 30
const managedWorktreePolicy = "managed_worktree_temporary"
const managedWorktreeLifecycleOwner = "adm:managed-worktree"

func newAdminWorktreeRetention() model.ResourceRetention {
	now := time.Now().UTC()
	expiresAt := now.Add(managedWorktreeGraceDays * 24 * time.Hour)
	return model.ResourceRetention{
		Persistence:    model.PersistenceTemporary,
		CreatorSurface: "admin",
		OwnerID:        managedWorktreeLifecycleOwner,
		CreatedAt:      &now,
		ExpiresAt:      &expiresAt,
		Policy:         managedWorktreePolicy,
	}
}

// MigrateManagedWorktreeRetention changes only ADM metadata. It does not
// inspect or remove any worktree. A fresh grace period prevents past-due
// cleanup immediately after converting legacy durable records.
func (s *Service) MigrateManagedWorktreeRetention(execute bool) (model.ManagedWorktreeRetentionMigrationResult, error) {
	now := time.Now().UTC()
	result := model.ManagedWorktreeRetentionMigrationResult{
		DryRun: !execute, GraceDays: managedWorktreeGraceDays,
		Candidates: []string{}, Converted: []string{},
	}
	convert := func(state *model.State) error {
		managedIDs := make(map[string]struct{}, len(state.ManagedWorktrees))
		for _, managed := range state.ManagedWorktrees {
			managedIDs[managed.EnvironmentID] = struct{}{}
		}
		result.ManagedCount = len(managedIDs)
		found := make(map[string]struct{}, len(state.Environments))
		for i := range state.Environments {
			env := &state.Environments[i]
			if _, ok := managedIDs[env.ID]; !ok {
				continue
			}
			found[env.ID] = struct{}{}
			if env.Retention.Persistence == model.PersistenceTemporary {
				continue
			}
			if env.Retention.Persistence != model.PersistenceDurable {
				return fmt.Errorf("managed worktree %s has invalid retention %q", env.ID, env.Retention.Persistence)
			}
			result.Candidates = append(result.Candidates, env.ID)
			if !execute {
				continue
			}
			originalCreator := env.Retention.CreatorSurface
			if originalCreator == "" {
				originalCreator = "legacy"
			}
			createdAt := env.Retention.CreatedAt
			if createdAt == nil {
				created := env.CreatedAt.UTC()
				createdAt = &created
			}
			expiry := now.Add(managedWorktreeGraceDays * 24 * time.Hour)
			env.Retention = model.ResourceRetention{
				Persistence:    model.PersistenceTemporary,
				CreatorSurface: originalCreator,
				OwnerID:        managedWorktreeLifecycleOwner,
				CreatedAt:      createdAt,
				ExpiresAt:      &expiry,
				Policy:         managedWorktreePolicy,
			}
			env.UpdatedAt = now
			result.Converted = append(result.Converted, env.ID)
		}
		for id := range managedIDs {
			if _, ok := found[id]; !ok {
				return fmt.Errorf("managed worktree %s has no Environment record", id)
			}
		}
		sort.Strings(result.Candidates)
		sort.Strings(result.Converted)
		return nil
	}
	if execute {
		if err := s.Store.Update(convert); err != nil {
			return model.ManagedWorktreeRetentionMigrationResult{}, err
		}
	} else {
		state, err := s.Store.Load()
		if err != nil {
			return model.ManagedWorktreeRetentionMigrationResult{}, err
		}
		if err := convert(&state); err != nil {
			return model.ManagedWorktreeRetentionMigrationResult{}, err
		}
	}
	return result, nil
}
