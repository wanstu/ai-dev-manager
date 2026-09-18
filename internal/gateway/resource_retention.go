package gateway

import (
	"context"
	"fmt"
	"strings"
	"time"

	"ai-dev-manager-v2/internal/app"
	"ai-dev-manager-v2/internal/model"
)

func (o *runtimeOwner) ResourceRetentionReport(ctx context.Context) (model.ResourceRetentionReport, error) {
	report, err := o.service.ResourceRetentionReport(ctx)
	if err != nil {
		return model.ResourceRetentionReport{}, err
	}
	now := time.Now().UTC()
	for i := range report.Resources {
		item := &report.Resources[i]
		changed := false
		if hasRetentionBlocker(item.Blockers, app.RetentionRuntimeObservationBlocker) {
			item.Blockers = removeRetentionBlocker(item.Blockers, app.RetentionRuntimeObservationBlocker)
			item.Blockers = append(item.Blockers, o.runtimeRetentionBlockers(item.Kind, item.ID)...)
			changed = true
		}
		if item.Kind == model.RetentionResourceEnvironment && hasRetentionBlocker(item.Blockers, "managed_worktree_requires_destroy_safety_check") {
			o.enrichManagedWorktreeRetention(ctx, item)
			changed = true
		}
		if changed {
			item.Blockers = uniqueGatewayStrings(item.Blockers)
			refreshRetentionCleanupState(item, now)
		}
	}
	report.GeneratedAt = now
	return report, nil
}

func (o *runtimeOwner) TemporaryEnvironmentStatus(ctx context.Context, environmentID string) (model.TemporaryEnvironmentStatus, error) {
	environment, err := o.service.Environments.Get(strings.TrimSpace(environmentID))
	if err != nil {
		return model.TemporaryEnvironmentStatus{}, err
	}
	report, err := o.ResourceRetentionReport(ctx)
	if err != nil {
		return model.TemporaryEnvironmentStatus{}, err
	}
	item, err := gatewayRetentionEnvironmentItem(report, environment.ID)
	if err != nil {
		return model.TemporaryEnvironmentStatus{}, err
	}
	managed := false
	if o.service.Isolation != nil {
		_, managed, err = o.service.Isolation.GetByEnvironment(environment.ID)
		if err != nil {
			return model.TemporaryEnvironmentStatus{}, err
		}
	}
	return model.TemporaryEnvironmentStatus{
		EnvironmentID:   environment.ID,
		WorkspaceID:     environment.WorkspaceID,
		Name:            environment.Name,
		Root:            environment.Root,
		ManagedWorktree: managed,
		Retention:       item.Retention,
		CleanupState:    item.CleanupState,
		CleanupEligible: item.CleanupEligible,
		Blockers:        append([]string(nil), item.Blockers...),
		Uncertainties:   append([]string(nil), item.Uncertainties...),
	}, nil
}

func (o *runtimeOwner) PromoteTemporaryEnvironment(ctx context.Context, environmentID, ownerID string) (model.TemporaryEnvironmentStatus, error) {
	if _, err := o.service.PromoteTemporaryEnvironment(environmentID, ownerID); err != nil {
		return model.TemporaryEnvironmentStatus{}, err
	}
	return o.TemporaryEnvironmentStatus(ctx, environmentID)
}

// TemporaryEnvironmentCleanup previews or executes cleanup for exactly one
// Environment. Execution requires the matching lifecycle owner and a currently
// eligible runtime-enriched retention item; no unrelated resource is swept.
func (o *runtimeOwner) TemporaryEnvironmentCleanup(ctx context.Context, environmentID, ownerID string, execute bool) (model.ResourceRetentionCleanupResult, error) {
	environmentID = strings.TrimSpace(environmentID)
	ownerID = strings.TrimSpace(ownerID)
	environment, err := o.service.Environments.Get(environmentID)
	if err != nil {
		return model.ResourceRetentionCleanupResult{}, err
	}
	report, err := o.ResourceRetentionReport(ctx)
	if err != nil {
		return model.ResourceRetentionCleanupResult{}, err
	}
	item, err := gatewayRetentionEnvironmentItem(report, environment.ID)
	if err != nil {
		return model.ResourceRetentionCleanupResult{}, err
	}
	target := model.ResourceRetentionReport{GeneratedAt: report.GeneratedAt, Resources: []model.ResourceRetentionItem{item}}
	request := model.ResourceRetentionCleanupRequest{Execute: execute}
	if !execute {
		return o.service.ResourceRetentionCleanupFromReport(target, request)
	}
	if environment.Retention.Persistence != model.PersistenceTemporary {
		return model.ResourceRetentionCleanupResult{}, fmt.Errorf("environment %s is not temporary", environment.ID)
	}
	if ownerID == "" {
		return model.ResourceRetentionCleanupResult{}, fmt.Errorf("owner_id is required")
	}
	if strings.TrimSpace(environment.Retention.OwnerID) != ownerID {
		return model.ResourceRetentionCleanupResult{}, fmt.Errorf("temporary environment %s belongs to owner %q", environment.ID, environment.Retention.OwnerID)
	}
	if !item.CleanupEligible {
		return model.ResourceRetentionCleanupResult{}, fmt.Errorf("temporary environment %s is not cleanup eligible: %s", environment.ID, strings.Join(item.Blockers, ", "))
	}
	if blockers := o.runtimeRetentionBlockers(model.RetentionResourceEnvironment, environment.ID); len(blockers) != 0 {
		return model.ResourceRetentionCleanupResult{}, fmt.Errorf("temporary environment %s became runtime-active before cleanup: %s", environment.ID, strings.Join(blockers, ", "))
	}
	result, err := o.service.ResourceRetentionCleanupFromRuntimeReport(ctx, target, request)
	if err != nil {
		return model.ResourceRetentionCleanupResult{}, err
	}
	o.cleanupManagedWorktreeRetention(ctx, target, &result)
	for _, mutation := range result.Removed {
		if mutation.Kind == model.RetentionResourceEnvironment && mutation.ID == environment.ID {
			o.DropEnvironment(environment.ID)
			break
		}
	}
	return result, nil
}

func gatewayRetentionEnvironmentItem(report model.ResourceRetentionReport, environmentID string) (model.ResourceRetentionItem, error) {
	for _, item := range report.Resources {
		if item.Kind == model.RetentionResourceEnvironment && item.ID == environmentID {
			return item, nil
		}
	}
	return model.ResourceRetentionItem{}, fmt.Errorf("retention environment %q not found", environmentID)
}

func (o *runtimeOwner) enrichManagedWorktreeRetention(ctx context.Context, item *model.ResourceRetentionItem) {
	if item == nil {
		return
	}
	item.Blockers = removeRetentionBlocker(item.Blockers, "managed_worktree_requires_destroy_safety_check")
	if o == nil || o.service == nil || o.service.Isolation == nil {
		item.Blockers = append(item.Blockers, "managed_worktree_safety_unavailable")
		item.Uncertainties = uniqueGatewayStrings(append(item.Uncertainties, "cleanup_requires_managed_worktree_safety_evidence"))
		return
	}
	safety, err := o.service.Isolation.Safety(ctx, item.ID)
	if err != nil {
		item.Blockers = append(item.Blockers, "managed_worktree_safety_check_failed")
		item.Uncertainties = uniqueGatewayStrings(append(item.Uncertainties, "cleanup_requires_managed_worktree_safety_evidence"))
		return
	}
	if safety.Dirty {
		item.Blockers = append(item.Blockers, "managed_worktree_dirty")
	}
	if safety.Unpublished {
		item.Blockers = append(item.Blockers, "managed_worktree_unpublished")
	}
}

func (o *runtimeOwner) ExpiredTemporaryEnvironmentsCleanup(ctx context.Context, execute bool) (model.ResourceRetentionCleanupResult, error) {
	report, err := o.ResourceRetentionReport(ctx)
	if err != nil {
		return model.ResourceRetentionCleanupResult{}, err
	}
	now := time.Now().UTC()
	filtered := model.ResourceRetentionReport{GeneratedAt: report.GeneratedAt, Resources: []model.ResourceRetentionItem{}}
	for _, item := range report.Resources {
		if item.Kind != model.RetentionResourceEnvironment || item.Retention.Persistence != model.PersistenceTemporary || item.Retention.ExpiresAt == nil {
			continue
		}
		if now.Before(item.Retention.ExpiresAt.UTC()) {
			continue
		}
		filtered.Resources = append(filtered.Resources, item)
	}
	request := model.ResourceRetentionCleanupRequest{Execute: execute}
	if !execute {
		return o.service.ResourceRetentionCleanupFromReport(filtered, request)
	}
	result, err := o.service.ResourceRetentionCleanupFromRuntimeReport(ctx, filtered, request)
	if err != nil {
		return model.ResourceRetentionCleanupResult{}, err
	}
	o.cleanupManagedWorktreeRetention(ctx, filtered, &result)
	for _, mutation := range result.Removed {
		if mutation.Kind == model.RetentionResourceEnvironment {
			o.DropEnvironment(mutation.ID)
		}
	}
	return result, nil
}

func (o *runtimeOwner) ResourceRetentionCleanup(ctx context.Context, request model.ResourceRetentionCleanupRequest) (model.ResourceRetentionCleanupResult, error) {
	report, err := o.ResourceRetentionReport(ctx)
	if err != nil {
		return model.ResourceRetentionCleanupResult{}, err
	}
	if !request.Execute {
		return o.service.ResourceRetentionCleanupFromReport(report, request)
	}
	result, err := o.service.ResourceRetentionCleanupFromRuntimeReport(ctx, report, request)
	if err != nil {
		return model.ResourceRetentionCleanupResult{}, err
	}
	for _, mutation := range result.Removed {
		if mutation.Kind == model.RetentionResourceMCP {
			o.DropMCP(mutation.ID)
		}
	}
	o.cleanupManagedWorktreeRetention(ctx, report, &result)
	return result, nil
}

func (o *runtimeOwner) cleanupManagedWorktreeRetention(ctx context.Context, report model.ResourceRetentionReport, result *model.ResourceRetentionCleanupResult) {
	if o == nil || o.service == nil || o.service.Isolation == nil || o.service.Environments == nil || result == nil {
		return
	}
	for _, item := range report.Resources {
		if item.Kind != model.RetentionResourceEnvironment || !item.CleanupEligible {
			continue
		}
		_, managed, err := o.service.Isolation.GetByEnvironment(item.ID)
		if err != nil || !managed {
			continue
		}
		if blockers := o.runtimeRetentionBlockers(model.RetentionResourceEnvironment, item.ID); len(blockers) != 0 {
			markRetentionExecutionBlocked(result, item.ID, blockers...)
			continue
		}
		environment, err := o.service.Environments.Get(item.ID)
		if err != nil || environment.Retention.Persistence != model.PersistenceTemporary || strings.TrimSpace(environment.Retention.OwnerID) == "" || environment.Retention.ExpiresAt == nil || time.Now().UTC().Before(environment.Retention.ExpiresAt.UTC()) {
			markRetentionExecutionBlocked(result, item.ID, "retention_changed_before_cleanup")
			continue
		}
		writerOwner := fmt.Sprintf("retention-cleanup:%s:%d", item.ID, time.Now().UTC().UnixNano())
		if _, err := o.service.Environments.AcquireWriter(item.ID, writerOwner); err != nil {
			markRetentionExecutionBlocked(result, item.ID, "writer_acquire_failed")
			continue
		}
		if blockers := o.runtimeRetentionBlockers(model.RetentionResourceEnvironment, item.ID); len(blockers) != 0 {
			_, _ = o.service.Environments.ReleaseWriter(item.ID, writerOwner, false)
			markRetentionExecutionBlocked(result, item.ID, blockers...)
			continue
		}
		destroyed, err := o.service.Isolation.Destroy(ctx, item.ID, writerOwner, false)
		if err != nil {
			_, _ = o.service.Environments.ReleaseWriter(item.ID, writerOwner, false)
			markRetentionExecutionBlocked(result, item.ID, "managed_worktree_cleanup_failed")
			continue
		}
		result.Removed = append(result.Removed, model.ResourceRetentionCleanupMutation{
			Kind:   model.RetentionResourceEnvironment,
			ID:     destroyed.EnvironmentID,
			Name:   item.Name,
			Action: model.RetentionCleanupActionRemove,
		})
		removeRetentionSkipped(result, model.RetentionResourceEnvironment, item.ID)
	}
}

func markRetentionExecutionBlocked(result *model.ResourceRetentionCleanupResult, environmentID string, blockers ...string) {
	for i := range result.Skipped {
		item := &result.Skipped[i]
		if item.Kind != model.RetentionResourceEnvironment || item.ID != environmentID {
			continue
		}
		item.Blockers = uniqueGatewayStrings(append(item.Blockers, blockers...))
		refreshRetentionCleanupState(item, time.Now().UTC())
		return
	}
}

func removeRetentionSkipped(result *model.ResourceRetentionCleanupResult, kind, id string) {
	kept := result.Skipped[:0]
	for _, item := range result.Skipped {
		if item.Kind == kind && item.ID == id {
			continue
		}
		kept = append(kept, item)
	}
	result.Skipped = kept
}

func (o *runtimeOwner) runtimeRetentionBlockers(kind, id string) []string {
	o.mu.Lock()
	defer o.mu.Unlock()
	blockers := []string{}
	switch kind {
	case model.RetentionResourceEnvironment:
		for key, session := range o.sessions {
			if key.environmentID == id && session != nil {
				blockers = append(blockers, "active_mcp_session")
				break
			}
		}
		for key, inFlight := range o.inFlight {
			if key.environmentID == id && inFlight {
				blockers = append(blockers, "mcp_operation_in_flight")
				break
			}
		}
		for _, process := range o.processes {
			if process.environmentID != id {
				continue
			}
			process.mu.Lock()
			running := process.state == devProcessRunning
			process.mu.Unlock()
			if running {
				blockers = append(blockers, "active_process")
				break
			}
		}
		for _, run := range o.runs {
			if run.environmentID != id {
				continue
			}
			run.mu.Lock()
			running := run.state == agentRunRunning
			run.mu.Unlock()
			if running {
				blockers = append(blockers, "active_run")
				break
			}
		}
		for _, run := range o.verifierRuns {
			if run.environmentID != id {
				continue
			}
			run.mu.Lock()
			running := run.state == verifierRunRunning
			run.mu.Unlock()
			if running {
				blockers = append(blockers, "active_verifier_run")
				break
			}
		}
	case model.RetentionResourceMCP:
		for key, session := range o.sessions {
			if key.mcpID == id && session != nil {
				blockers = append(blockers, "active_mcp_session")
				break
			}
		}
		for key, inFlight := range o.inFlight {
			if key.mcpID == id && inFlight {
				blockers = append(blockers, "mcp_operation_in_flight")
				break
			}
		}
	}
	return blockers
}

func refreshRetentionCleanupState(item *model.ResourceRetentionItem, now time.Time) {
	if item == nil {
		return
	}
	item.CleanupEligible = false
	switch item.Retention.Persistence {
	case model.PersistenceDurable:
		item.CleanupState = model.RetentionCleanupDurable
		return
	case model.PersistenceTemporary:
	default:
		item.CleanupState = model.RetentionCleanupBlocked
		return
	}
	if item.Retention.ExpiresAt == nil {
		item.CleanupState = model.RetentionCleanupBlocked
		return
	}
	if now.Before(item.Retention.ExpiresAt.UTC()) {
		item.CleanupState = model.RetentionCleanupNotDue
		return
	}
	if len(item.Blockers) != 0 {
		item.CleanupState = model.RetentionCleanupBlocked
		return
	}
	item.CleanupState = model.RetentionCleanupEligible
	item.CleanupEligible = true
}

func hasRetentionBlocker(values []string, target string) bool {
	for _, value := range values {
		if strings.EqualFold(strings.TrimSpace(value), target) {
			return true
		}
	}
	return false
}

func removeRetentionBlocker(values []string, target string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		if strings.EqualFold(strings.TrimSpace(value), target) {
			continue
		}
		result = append(result, value)
	}
	return result
}
