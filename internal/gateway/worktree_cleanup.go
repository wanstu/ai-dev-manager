package gateway

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"ai-dev-manager-v2/internal/model"
)

const (
	defaultStaleManagedWorktreeInactiveSeconds int64 = 24 * 60 * 60
	legacyManagedWorktreeTemporaryPolicy              = "managed_worktree_temporary"
)

func (o *runtimeOwner) StaleManagedWorktreeCleanup(ctx context.Context, inactiveSeconds int64, environmentIDs []string, execute, force bool) (model.ManagedWorktreeCleanupResult, error) {
	if o == nil || o.service == nil || o.service.Isolation == nil || o.service.Environments == nil {
		return model.ManagedWorktreeCleanupResult{}, fmt.Errorf("managed worktree cleanup is unavailable")
	}
	if inactiveSeconds <= 0 {
		inactiveSeconds = defaultStaleManagedWorktreeInactiveSeconds
	}
	selected := normalizedEnvironmentIDSet(environmentIDs)
	if execute && len(selected) == 0 {
		return model.ManagedWorktreeCleanupResult{}, fmt.Errorf("environment_ids is required when execute=true; preview first and explicitly select stale managed worktrees")
	}
	report, err := o.staleManagedWorktreeReport(ctx, inactiveSeconds, selected, force)
	if err != nil {
		return model.ManagedWorktreeCleanupResult{}, err
	}
	result := model.ManagedWorktreeCleanupResult{GeneratedAt: time.Now().UTC(), DryRun: !execute, Report: report}
	if !execute {
		return result, nil
	}

	for _, preview := range report.Candidates {
		if !preview.CleanupEligible {
			result.Skipped = append(result.Skipped, preview)
			continue
		}
		fresh, err := o.managedWorktreeCleanupCandidate(ctx, preview.EnvironmentID, time.Now().UTC(), inactiveSeconds, force)
		if err != nil {
			preview.CleanupEligible = false
			preview.Blockers = uniqueGatewayStrings(append(preview.Blockers, "candidate_recheck_failed"))
			result.Skipped = append(result.Skipped, preview)
			continue
		}
		if !fresh.CleanupEligible {
			result.Skipped = append(result.Skipped, fresh)
			continue
		}

		writerOwner := fmt.Sprintf("stale-worktree-cleanup:%s:%d", fresh.EnvironmentID, time.Now().UTC().UnixNano())
		if _, err := o.service.Environments.AcquireWriter(fresh.EnvironmentID, writerOwner); err != nil {
			fresh.CleanupEligible = false
			fresh.Blockers = uniqueGatewayStrings(append(fresh.Blockers, "writer_acquire_failed"))
			result.Skipped = append(result.Skipped, fresh)
			continue
		}
		if blockers := o.runtimeRetentionBlockers(model.RetentionResourceEnvironment, fresh.EnvironmentID); len(blockers) != 0 {
			_, _ = o.service.Environments.ReleaseWriter(fresh.EnvironmentID, writerOwner, false)
			fresh.CleanupEligible = false
			fresh.Blockers = uniqueGatewayStrings(append(fresh.Blockers, blockers...))
			result.Skipped = append(result.Skipped, fresh)
			continue
		}

		legacyTemporary := fresh.Temporary && strings.EqualFold(strings.TrimSpace(fresh.RetentionPolicy), legacyManagedWorktreeTemporaryPolicy)
		if legacyTemporary {
			safety, safetyErr := o.service.Isolation.SafetyForRetentionCleanup(ctx, fresh.EnvironmentID)
			if safetyErr != nil || (safety.Dirty && !force) {
				_, _ = o.service.Environments.ReleaseWriter(fresh.EnvironmentID, writerOwner, false)
				fresh.CleanupEligible = false
				if safetyErr != nil {
					fresh.Blockers = append(fresh.Blockers, "managed_worktree_safety_check_failed")
				}
				if safety.Dirty && !force {
					fresh.Blockers = append(fresh.Blockers, "managed_worktree_dirty")
					if safety.Ignored {
						fresh.Blockers = append(fresh.Blockers, "managed_worktree_ignored_files")
					}
				}
				fresh.Blockers = uniqueGatewayStrings(fresh.Blockers)
				result.Skipped = append(result.Skipped, fresh)
				continue
			}
			destroyed, destroyErr := o.service.Isolation.DestroyForRetentionCleanup(ctx, fresh.EnvironmentID, writerOwner, force)
			if destroyErr != nil {
				_, _ = o.service.Environments.ReleaseWriter(fresh.EnvironmentID, writerOwner, false)
				fresh.CleanupEligible = false
				fresh.Blockers = uniqueGatewayStrings(append(fresh.Blockers, "managed_worktree_cleanup_failed"))
				result.Skipped = append(result.Skipped, fresh)
				continue
			}
			o.DropEnvironment(fresh.EnvironmentID)
			result.Removed = append(result.Removed, model.ManagedWorktreeCleanupMutation{
				ManagedWorktreeID: destroyed.ManagedWorktreeID,
				EnvironmentID:     destroyed.EnvironmentID,
				Name:              fresh.Name,
				Root:              destroyed.Root,
				RetainedBranch:    destroyed.RetainedBranch,
			})
			continue
		}

		safety, safetyErr := o.service.Isolation.Safety(ctx, fresh.EnvironmentID)
		if safetyErr != nil || safety.Dirty || safety.Unpublished {
			_, _ = o.service.Environments.ReleaseWriter(fresh.EnvironmentID, writerOwner, false)
			fresh.CleanupEligible = false
			if safetyErr != nil {
				fresh.Blockers = append(fresh.Blockers, "managed_worktree_safety_check_failed")
			}
			if safety.Dirty {
				fresh.Blockers = append(fresh.Blockers, "managed_worktree_dirty")
				if safety.Ignored {
					fresh.Blockers = append(fresh.Blockers, "managed_worktree_ignored_files")
				}
			}
			if safety.Unpublished {
				fresh.Blockers = append(fresh.Blockers, "managed_worktree_unpublished")
			}
			fresh.Blockers = uniqueGatewayStrings(fresh.Blockers)
			result.Skipped = append(result.Skipped, fresh)
			continue
		}
		destroyed, destroyErr := o.service.Isolation.Destroy(ctx, fresh.EnvironmentID, writerOwner, false)
		if destroyErr != nil {
			_, _ = o.service.Environments.ReleaseWriter(fresh.EnvironmentID, writerOwner, false)
			fresh.CleanupEligible = false
			fresh.Blockers = uniqueGatewayStrings(append(fresh.Blockers, "managed_worktree_cleanup_failed"))
			result.Skipped = append(result.Skipped, fresh)
			continue
		}
		o.DropEnvironment(fresh.EnvironmentID)
		result.Removed = append(result.Removed, model.ManagedWorktreeCleanupMutation{
			ManagedWorktreeID: destroyed.ManagedWorktreeID,
			EnvironmentID:     destroyed.EnvironmentID,
			Name:              fresh.Name,
			Root:              destroyed.Root,
			RetainedBranch:    destroyed.RetainedBranch,
		})
	}
	sort.Slice(result.Removed, func(i, j int) bool { return result.Removed[i].EnvironmentID < result.Removed[j].EnvironmentID })
	sort.Slice(result.Skipped, func(i, j int) bool { return result.Skipped[i].EnvironmentID < result.Skipped[j].EnvironmentID })
	return result, nil
}

func (o *runtimeOwner) staleManagedWorktreeReport(ctx context.Context, inactiveSeconds int64, selected map[string]struct{}, force bool) (model.ManagedWorktreeCleanupReport, error) {
	now := time.Now().UTC()
	report := model.ManagedWorktreeCleanupReport{GeneratedAt: now, InactiveThresholdSeconds: inactiveSeconds, Candidates: []model.ManagedWorktreeCleanupCandidate{}}
	managedItems, err := o.service.ManagedWorktrees()
	if err != nil {
		return report, err
	}
	found := map[string]struct{}{}
	for _, managed := range managedItems {
		if len(selected) != 0 {
			if _, ok := selected[managed.EnvironmentID]; !ok {
				continue
			}
		}
		found[managed.EnvironmentID] = struct{}{}
		candidate, candidateErr := o.managedWorktreeCleanupCandidate(ctx, managed.EnvironmentID, now, inactiveSeconds, force)
		if candidateErr != nil {
			candidate = model.ManagedWorktreeCleanupCandidate{ManagedWorktreeID: managed.ID, EnvironmentID: managed.EnvironmentID, Root: managed.Root, Branch: managed.Branch, CreatedAt: managed.CreatedAt, CleanupEligible: false, Blockers: []string{"candidate_inspection_failed"}}
		}
		if containsGatewayString(candidate.Blockers, "temporary_environment_use_temporary_cleanup") {
			continue
		}
		report.Candidates = append(report.Candidates, candidate)
	}
	for id := range selected {
		if _, ok := found[id]; ok {
			continue
		}
		report.Candidates = append(report.Candidates, model.ManagedWorktreeCleanupCandidate{EnvironmentID: id, CleanupEligible: false, Blockers: []string{"managed_worktree_not_found"}})
	}
	sort.Slice(report.Candidates, func(i, j int) bool {
		left, right := report.Candidates[i], report.Candidates[j]
		if left.LastActivityAt.Equal(right.LastActivityAt) {
			return left.EnvironmentID < right.EnvironmentID
		}
		if left.LastActivityAt.IsZero() {
			return false
		}
		if right.LastActivityAt.IsZero() {
			return true
		}
		return left.LastActivityAt.Before(right.LastActivityAt)
	})
	return report, nil
}

func (o *runtimeOwner) managedWorktreeCleanupCandidate(ctx context.Context, environmentID string, now time.Time, inactiveThresholdSeconds int64, force bool) (model.ManagedWorktreeCleanupCandidate, error) {
	environment, err := o.service.Environments.Get(strings.TrimSpace(environmentID))
	if err != nil {
		return model.ManagedWorktreeCleanupCandidate{}, err
	}
	managed, ok, err := o.service.Isolation.GetByEnvironment(environment.ID)
	if err != nil {
		return model.ManagedWorktreeCleanupCandidate{}, err
	}
	if !ok {
		return model.ManagedWorktreeCleanupCandidate{}, fmt.Errorf("environment %s is not backed by a managed worktree", environment.ID)
	}

	activityAt := environment.LastActivityAt
	if activityAt.IsZero() {
		activityAt = environment.UpdatedAt
	}
	if activityAt.IsZero() {
		activityAt = environment.CreatedAt
	}
	inactiveFor := int64(0)
	if !activityAt.IsZero() && now.After(activityAt) {
		inactiveFor = int64(now.Sub(activityAt).Seconds())
	}
	legacyTemporary := environment.Retention.Persistence == model.PersistenceTemporary &&
		strings.EqualFold(strings.TrimSpace(environment.Retention.Policy), legacyManagedWorktreeTemporaryPolicy)
	candidate := model.ManagedWorktreeCleanupCandidate{
		ManagedWorktreeID: managed.ID, EnvironmentID: environment.ID, Name: environment.Name, Root: managed.Root, Branch: managed.Branch,
		CreatedAt: managed.CreatedAt, LastActivityAt: activityAt, InactiveSeconds: inactiveFor,
		Temporary: environment.Retention.Persistence == model.PersistenceTemporary, RetentionPolicy: environment.Retention.Policy,
	}
	if environment.Retention.Persistence == model.PersistenceTemporary && !legacyTemporary {
		candidate.Blockers = append(candidate.Blockers, "temporary_environment_use_temporary_cleanup")
		return finalizeManagedWorktreeCleanupCandidate(candidate), nil
	}
	if inactiveFor < inactiveThresholdSeconds {
		candidate.Blockers = append(candidate.Blockers, "recent_activity")
	}
	if environment.Writer != nil && now.Before(environment.Writer.ExpiresAt) {
		candidate.Blockers = append(candidate.Blockers, "active_writer")
	}
	if blockers := o.runtimeRetentionBlockers(model.RetentionResourceEnvironment, environment.ID); len(blockers) != 0 {
		candidate.Blockers = append(candidate.Blockers, blockers...)
	}

	var safetyErr error
	var dirty, ignored, unpublished bool
	if legacyTemporary {
		safety, err := o.service.Isolation.SafetyForRetentionCleanup(ctx, environment.ID)
		safetyErr = err
		dirty, ignored = safety.Dirty, safety.Ignored
	} else {
		safety, err := o.service.Isolation.Safety(ctx, environment.ID)
		safetyErr = err
		dirty, ignored, unpublished = safety.Dirty, safety.Ignored, safety.Unpublished
	}
	if safetyErr != nil {
		candidate.Blockers = append(candidate.Blockers, "managed_worktree_safety_check_failed")
	} else {
		candidate.Dirty = dirty
		candidate.Unpublished = unpublished
		if dirty {
			if legacyTemporary && force {
				candidate.ForceRequired = true
			} else {
				candidate.Blockers = append(candidate.Blockers, "managed_worktree_dirty")
				if ignored {
					candidate.Blockers = append(candidate.Blockers, "managed_worktree_ignored_files")
				}
			}
		}
		if unpublished {
			candidate.Blockers = append(candidate.Blockers, "managed_worktree_unpublished")
		}
	}
	return finalizeManagedWorktreeCleanupCandidate(candidate), nil
}

func finalizeManagedWorktreeCleanupCandidate(candidate model.ManagedWorktreeCleanupCandidate) model.ManagedWorktreeCleanupCandidate {
	candidate.Blockers = uniqueGatewayStrings(candidate.Blockers)
	candidate.CleanupEligible = len(candidate.Blockers) == 0
	return candidate
}

func normalizedEnvironmentIDSet(ids []string) map[string]struct{} {
	out := map[string]struct{}{}
	for _, id := range ids {
		if id = strings.TrimSpace(id); id != "" {
			out[id] = struct{}{}
		}
	}
	return out
}

func containsGatewayString(items []string, target string) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}
