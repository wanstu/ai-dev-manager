package app

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"ai-dev-manager-v2/internal/model"
)

const RetentionRuntimeObservationBlocker = "gateway_runtime_activity_not_observed"

const retentionCleanupExecuteUnsupportedBlocker = "cleanup_execute_not_supported_for_resource_kind"

// ResourceRetentionReport is a read-only cleanup preview. Application-level
// inspection is deliberately conservative: runtime-owner activity and managed
// worktree publication/dirty state are never guessed.
func (s *Service) ResourceRetentionReport(_ context.Context) (model.ResourceRetentionReport, error) {
	state, err := s.Store.Load()
	if err != nil {
		return model.ResourceRetentionReport{}, err
	}
	return resourceRetentionReportFromState(state, time.Now().UTC()), nil
}

// PromoteResourceRetention makes one temporary resource durable without
// deleting files or changing runtime selections. Skill sources are the boundary
// for source-owned Skills, so promoting a source also promotes its discovered
// Skills to keep retention metadata coherent.
func (s *Service) PromoteResourceRetention(request model.ResourceRetentionUpdateRequest) (model.ResourceRetentionUpdateResult, error) {
	normalized, err := normalizeRetentionUpdateRequest(request, false)
	if err != nil {
		return model.ResourceRetentionUpdateResult{}, err
	}
	now := time.Now().UTC()
	result := model.ResourceRetentionUpdateResult{Action: model.RetentionUpdateActionPromoteDurable, GeneratedAt: now}
	err = s.Store.Update(func(state *model.State) error {
		updated, updateErr := updateResourceRetentionState(state, normalized, now, func(current model.ResourceRetention) model.ResourceRetention {
			return promoteRetentionDurable(current, normalized, now)
		})
		if updateErr != nil {
			return updateErr
		}
		result.Updated = updated
		return nil
	})
	if err != nil {
		return model.ResourceRetentionUpdateResult{}, err
	}
	sortRetentionUpdateItems(result.Updated)
	return result, nil
}

// MarkResourceTemporary explicitly marks one resource as temporary. It requires
// owner evidence and mutates only ADM retention metadata. Omitting ExpiresAt is
// allowed, but such resources remain blocked from cleanup until an explicit
// expiry is recorded.
func (s *Service) MarkResourceTemporary(request model.ResourceRetentionUpdateRequest) (model.ResourceRetentionUpdateResult, error) {
	normalized, err := normalizeRetentionUpdateRequest(request, true)
	if err != nil {
		return model.ResourceRetentionUpdateResult{}, err
	}
	now := time.Now().UTC()
	result := model.ResourceRetentionUpdateResult{Action: model.RetentionUpdateActionMarkTemporary, GeneratedAt: now}
	err = s.Store.Update(func(state *model.State) error {
		if err := validateAttachedEnvironment(state, normalized.AttachedEnvironmentID); err != nil {
			return err
		}
		updated, updateErr := updateResourceRetentionState(state, normalized, now, func(current model.ResourceRetention) model.ResourceRetention {
			return markRetentionTemporary(current, normalized, now)
		})
		if updateErr != nil {
			return updateErr
		}
		result.Updated = updated
		return nil
	})
	if err != nil {
		return model.ResourceRetentionUpdateResult{}, err
	}
	sortRetentionUpdateItems(result.Updated)
	return result, nil
}

// ResourceRetentionCleanup builds a cleanup plan and, when Execute is true,
// removes only state-only Skill catalog resources that remain eligible after a
// fresh in-transaction report. It never deletes project directories or host
// files. Environment, managed worktree and MCP destructive cleanup require a
// Gateway runtime owner, so app-only execution keeps MCPs blocked.
func (s *Service) ResourceRetentionCleanup(ctx context.Context, request model.ResourceRetentionCleanupRequest) (model.ResourceRetentionCleanupResult, error) {
	if !request.Execute {
		report, err := s.ResourceRetentionReport(ctx)
		if err != nil {
			return model.ResourceRetentionCleanupResult{}, err
		}
		return s.ResourceRetentionCleanupFromReport(report, request)
	}
	if err := ctx.Err(); err != nil {
		return model.ResourceRetentionCleanupResult{}, err
	}

	var result model.ResourceRetentionCleanupResult
	err := s.Store.Update(func(state *model.State) error {
		now := time.Now().UTC()
		report := resourceRetentionReportFromState(*state, now)
		result = executeRetentionCleanup(state, report, now, nil, nil)
		return nil
	})
	if err != nil {
		return model.ResourceRetentionCleanupResult{}, err
	}
	return result, nil
}

// ResourceRetentionCleanupFromReport builds a non-mutating cleanup preview from
// a supplied report. Gateway runtime owners pass their enriched report here so
// dry-run output can account for owner-local sessions, processes and runs.
func (s *Service) ResourceRetentionCleanupFromReport(report model.ResourceRetentionReport, request model.ResourceRetentionCleanupRequest) (model.ResourceRetentionCleanupResult, error) {
	_ = s
	_ = request
	result := model.ResourceRetentionCleanupResult{GeneratedAt: time.Now().UTC(), DryRun: true, Report: report}
	for _, item := range report.Resources {
		if item.CleanupEligible {
			result.WouldRemove = append(result.WouldRemove, retentionMutationForItem(item))
			continue
		}
		result.Skipped = append(result.Skipped, item)
	}
	sortRetentionMutations(result.WouldRemove)
	sortRetentionItems(result.Skipped)
	return result, nil
}

// ResourceRetentionCleanupFromRuntimeReport executes the state-only subset of
// cleanup using a Gateway-owner enriched report as runtime evidence. The store
// is still re-read and static blockers are revalidated in the update closure;
// only MCPs that were runtime-eligible in the owner report and still have no
// persisted blockers are removed.
func (s *Service) ResourceRetentionCleanupFromRuntimeReport(ctx context.Context, report model.ResourceRetentionReport, request model.ResourceRetentionCleanupRequest) (model.ResourceRetentionCleanupResult, error) {
	if !request.Execute {
		return s.ResourceRetentionCleanupFromReport(report, request)
	}
	if err := ctx.Err(); err != nil {
		return model.ResourceRetentionCleanupResult{}, err
	}
	runtimeEligibleEnvironments := eligibleRetentionIDs(report, model.RetentionResourceEnvironment)
	runtimeEligibleMCPs := eligibleRetentionIDs(report, model.RetentionResourceMCP)

	var result model.ResourceRetentionCleanupResult
	err := s.Store.Update(func(state *model.State) error {
		now := time.Now().UTC()
		currentReport := resourceRetentionReportFromState(*state, now)
		currentReport = intersectRetentionReport(currentReport, report)
		result = executeRetentionCleanup(state, currentReport, now, runtimeEligibleEnvironments, runtimeEligibleMCPs)
		return nil
	})
	if err != nil {
		return model.ResourceRetentionCleanupResult{}, err
	}
	result.Report = report
	result.Skipped = skippedRetentionItems(report, result.Removed)
	sortRetentionItems(result.Skipped)
	return result, nil
}

func resourceRetentionReportFromState(state model.State, now time.Time) model.ResourceRetentionReport {
	environments := make(map[string]model.Environment, len(state.Environments))
	selectedMCPs := map[string]bool{}
	selectedSkills := map[string]bool{}
	managedByEnvironment := map[string]bool{}
	for _, env := range state.Environments {
		environments[env.ID] = env
		for _, id := range env.EnabledMCPIDs {
			selectedMCPs[id] = true
		}
		for _, id := range env.EnabledSkillIDs {
			selectedSkills[id] = true
		}
	}
	for _, managed := range state.ManagedWorktrees {
		managedByEnvironment[managed.EnvironmentID] = true
	}

	items := make([]model.ResourceRetentionItem, 0, len(state.Environments)+len(state.MCPs)+len(state.SkillSources)+len(state.Skills))
	for _, env := range state.Environments {
		blockers := []string{}
		if strings.TrimSpace(env.State) != "ready" {
			blockers = append(blockers, "environment_state_not_ready")
		}
		if writerActiveAt(env.Writer, now) {
			blockers = append(blockers, "active_writer")
		}
		if managedByEnvironment[env.ID] {
			blockers = append(blockers, "managed_worktree_requires_destroy_safety_check")
		}
		blockers = append(blockers, RetentionRuntimeObservationBlocker)
		items = append(items, buildRetentionItem(model.RetentionResourceEnvironment, env.ID, env.Name, env.Retention, now, blockers, nil))
	}
	for _, definition := range state.MCPs {
		blockers := []string{RetentionRuntimeObservationBlocker}
		if selectedMCPs[definition.ID] {
			blockers = append(blockers, "environment_selection_exists")
		}
		if attachedEnvironmentExists(definition.Retention, environments) {
			blockers = append(blockers, "attached_environment_exists")
		}
		items = append(items, buildRetentionItem(model.RetentionResourceMCP, definition.ID, definition.Name, definition.Retention, now, blockers, nil))
	}
	for _, source := range state.SkillSources {
		blockers := []string{}
		for _, skill := range state.Skills {
			if skill.SourceID == source.ID && selectedSkills[skill.ID] {
				blockers = append(blockers, "environment_selection_exists")
				break
			}
		}
		if attachedEnvironmentExists(source.Retention, environments) {
			blockers = append(blockers, "attached_environment_exists")
		}
		items = append(items, buildRetentionItem(model.RetentionResourceSkillSource, source.ID, source.Root, source.Retention, now, blockers, nil))
	}
	for _, skill := range state.Skills {
		blockers := []string{}
		uncertainties := []string{}
		if skill.SourceID != "" {
			blockers = append(blockers, "source_owned_skill")
			uncertainties = append(uncertainties, "cleanup_at_skill_source_boundary")
		}
		if selectedSkills[skill.ID] {
			blockers = append(blockers, "environment_selection_exists")
		}
		if attachedEnvironmentExists(skill.Retention, environments) {
			blockers = append(blockers, "attached_environment_exists")
		}
		items = append(items, buildRetentionItem(model.RetentionResourceSkill, skill.ID, skill.Name, skill.Retention, now, blockers, uncertainties))
	}
	sortRetentionItems(items)
	return model.ResourceRetentionReport{GeneratedAt: now, Resources: items}
}

func executeRetentionCleanup(state *model.State, report model.ResourceRetentionReport, now time.Time, runtimeEligibleEnvironmentIDs, runtimeEligibleMCPIDs map[string]struct{}) model.ResourceRetentionCleanupResult {
	result := model.ResourceRetentionCleanupResult{GeneratedAt: now, DryRun: false, Report: report}
	removableEnvironments := map[string]struct{}{}
	removableSources := map[string]struct{}{}
	removableStandaloneSkills := map[string]struct{}{}
	removableMCPs := map[string]struct{}{}
	for _, item := range report.Resources {
		if item.CleanupEligible {
			switch item.Kind {
			case model.RetentionResourceSkillSource:
				removableSources[item.ID] = struct{}{}
			case model.RetentionResourceSkill:
				if skillIsStandalone(*state, item.ID) {
					removableStandaloneSkills[item.ID] = struct{}{}
				}
			}
		}
		if runtimeEnvironmentStillSafe(*state, item, now, runtimeEligibleEnvironmentIDs) {
			removableEnvironments[item.ID] = struct{}{}
		}
		if runtimeMCPStillSafe(item, now, runtimeEligibleMCPIDs) {
			removableMCPs[item.ID] = struct{}{}
		}
	}

	for _, item := range report.Resources {
		switch item.Kind {
		case model.RetentionResourceEnvironment:
			if _, ok := removableEnvironments[item.ID]; ok {
				continue
			}
		case model.RetentionResourceSkillSource:
			if _, ok := removableSources[item.ID]; ok {
				continue
			}
		case model.RetentionResourceSkill:
			if _, ok := removableStandaloneSkills[item.ID]; ok {
				continue
			}
			if skillBelongsToRemovedSource(*state, item.ID, removableSources) {
				continue
			}
		case model.RetentionResourceMCP:
			if _, ok := removableMCPs[item.ID]; ok {
				continue
			}
		}
		if item.CleanupEligible {
			item.CleanupEligible = false
			item.CleanupState = model.RetentionCleanupBlocked
			item.Blockers = uniqueStrings(append(item.Blockers, retentionCleanupExecuteUnsupportedBlocker))
		}
		result.Skipped = append(result.Skipped, item)
	}

	for _, item := range report.Resources {
		if item.Kind != model.RetentionResourceEnvironment {
			continue
		}
		if _, ok := removableEnvironments[item.ID]; !ok {
			continue
		}
		environment, removed := removeEnvironmentState(state, item.ID, now)
		if !removed {
			continue
		}
		result.Removed = append(result.Removed, model.ResourceRetentionCleanupMutation{Kind: model.RetentionResourceEnvironment, ID: environment.ID, Name: environment.Name, Action: model.RetentionCleanupActionRemove})
	}
	for _, item := range report.Resources {
		if item.Kind != model.RetentionResourceSkillSource {
			continue
		}
		if _, ok := removableSources[item.ID]; !ok {
			continue
		}
		source, skills, removed := removeSkillSourceState(state, item.ID)
		if !removed {
			continue
		}
		result.Removed = append(result.Removed, model.ResourceRetentionCleanupMutation{Kind: model.RetentionResourceSkillSource, ID: source.ID, Name: source.Root, Action: model.RetentionCleanupActionRemove})
		for _, skill := range skills {
			result.Removed = append(result.Removed, model.ResourceRetentionCleanupMutation{Kind: model.RetentionResourceSkill, ID: skill.ID, Name: skill.Name, Action: model.RetentionCleanupActionRemove})
		}
	}
	for _, item := range report.Resources {
		if item.Kind != model.RetentionResourceSkill {
			continue
		}
		if _, ok := removableStandaloneSkills[item.ID]; !ok {
			continue
		}
		skill, removed := removeStandaloneSkillState(state, item.ID)
		if !removed {
			continue
		}
		result.Removed = append(result.Removed, model.ResourceRetentionCleanupMutation{Kind: model.RetentionResourceSkill, ID: skill.ID, Name: skill.Name, Action: model.RetentionCleanupActionRemove})
	}
	for _, item := range report.Resources {
		if item.Kind != model.RetentionResourceMCP {
			continue
		}
		if _, ok := removableMCPs[item.ID]; !ok {
			continue
		}
		definition, removed := removeMCPState(state, item.ID)
		if !removed {
			continue
		}
		result.Removed = append(result.Removed, model.ResourceRetentionCleanupMutation{Kind: model.RetentionResourceMCP, ID: definition.ID, Name: definition.Name, Action: model.RetentionCleanupActionRemove})
	}
	sortRetentionMutations(result.Removed)
	sortRetentionItems(result.Skipped)
	return result
}

func buildRetentionItem(kind, id, name string, retention model.ResourceRetention, now time.Time, staticBlockers, uncertainties []string) model.ResourceRetentionItem {
	item := model.ResourceRetentionItem{
		Kind:          kind,
		ID:            id,
		Name:          name,
		Retention:     retention,
		Blockers:      uniqueStrings(staticBlockers),
		Uncertainties: uniqueStrings(uncertainties),
	}
	switch retention.Persistence {
	case model.PersistenceDurable:
		item.CleanupState = model.RetentionCleanupDurable
		item.CleanupEligible = false
		item.Blockers = uniqueStrings(append(item.Blockers, "resource_is_durable"))
		return item
	case model.PersistenceTemporary:
	default:
		item.CleanupState = model.RetentionCleanupBlocked
		item.CleanupEligible = false
		item.Blockers = uniqueStrings(append(item.Blockers, "invalid_persistence_class"))
		item.Uncertainties = uniqueStrings(append(item.Uncertainties, "retention_metadata_invalid"))
		return item
	}
	if strings.TrimSpace(retention.OwnerID) == "" {
		item.Blockers = uniqueStrings(append(item.Blockers, "retention_owner_unknown"))
		item.Uncertainties = uniqueStrings(append(item.Uncertainties, "cleanup_requires_owner_evidence"))
	}
	if retention.ExpiresAt == nil {
		item.CleanupState = model.RetentionCleanupBlocked
		item.Blockers = uniqueStrings(append(item.Blockers, "retention_expiry_not_set"))
		item.Uncertainties = uniqueStrings(append(item.Uncertainties, "cleanup_requires_explicit_retention_evidence"))
		return item
	}
	if now.Before(retention.ExpiresAt.UTC()) {
		item.CleanupState = model.RetentionCleanupNotDue
		item.Blockers = uniqueStrings(append(item.Blockers, "retention_not_expired"))
		return item
	}
	if len(item.Blockers) != 0 {
		item.CleanupState = model.RetentionCleanupBlocked
		return item
	}
	item.CleanupState = model.RetentionCleanupEligible
	item.CleanupEligible = true
	return item
}

func retentionMutationForItem(item model.ResourceRetentionItem) model.ResourceRetentionCleanupMutation {
	return model.ResourceRetentionCleanupMutation{
		Kind:   item.Kind,
		ID:     item.ID,
		Name:   item.Name,
		Action: model.RetentionCleanupActionRemove,
	}
}

func eligibleRetentionIDs(report model.ResourceRetentionReport, kind string) map[string]struct{} {
	ids := map[string]struct{}{}
	for _, item := range report.Resources {
		if item.Kind == kind && item.CleanupEligible {
			ids[item.ID] = struct{}{}
		}
	}
	return ids
}

func intersectRetentionReport(current, selected model.ResourceRetentionReport) model.ResourceRetentionReport {
	keys := map[string]struct{}{}
	for _, item := range selected.Resources {
		keys[item.Kind+"\x00"+item.ID] = struct{}{}
	}
	result := model.ResourceRetentionReport{GeneratedAt: current.GeneratedAt}
	for _, item := range current.Resources {
		if _, ok := keys[item.Kind+"\x00"+item.ID]; ok {
			result.Resources = append(result.Resources, item)
		}
	}
	return result
}

func runtimeEnvironmentStillSafe(state model.State, item model.ResourceRetentionItem, now time.Time, runtimeEligibleEnvironmentIDs map[string]struct{}) bool {
	if len(runtimeEligibleEnvironmentIDs) == 0 || item.Kind != model.RetentionResourceEnvironment {
		return false
	}
	if _, ok := runtimeEligibleEnvironmentIDs[item.ID]; !ok {
		return false
	}
	if item.Retention.Persistence != model.PersistenceTemporary || strings.TrimSpace(item.Retention.OwnerID) == "" {
		return false
	}
	if item.Retention.ExpiresAt == nil || now.Before(item.Retention.ExpiresAt.UTC()) {
		return false
	}
	for _, managed := range state.ManagedWorktrees {
		if managed.EnvironmentID == item.ID {
			return false
		}
	}
	found := false
	for _, environment := range state.Environments {
		if environment.ID != item.ID {
			continue
		}
		found = true
		if strings.TrimSpace(environment.State) != "ready" || writerActiveAt(environment.Writer, now) {
			return false
		}
		break
	}
	if !found {
		return false
	}
	blockers := retentionBlockersExcept(item.Blockers, RetentionRuntimeObservationBlocker)
	return len(blockers) == 0
}

func runtimeMCPStillSafe(item model.ResourceRetentionItem, now time.Time, runtimeEligibleMCPIDs map[string]struct{}) bool {
	if len(runtimeEligibleMCPIDs) == 0 || item.Kind != model.RetentionResourceMCP {
		return false
	}
	if _, ok := runtimeEligibleMCPIDs[item.ID]; !ok {
		return false
	}
	if item.Retention.Persistence != model.PersistenceTemporary || strings.TrimSpace(item.Retention.OwnerID) == "" {
		return false
	}
	if item.Retention.ExpiresAt == nil || now.Before(item.Retention.ExpiresAt.UTC()) {
		return false
	}
	blockers := retentionBlockersExcept(item.Blockers, RetentionRuntimeObservationBlocker)
	return len(blockers) == 0
}

func retentionBlockersExcept(values []string, ignored string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		if strings.EqualFold(strings.TrimSpace(value), ignored) {
			continue
		}
		result = append(result, value)
	}
	return result
}

func skippedRetentionItems(report model.ResourceRetentionReport, removed []model.ResourceRetentionCleanupMutation) []model.ResourceRetentionItem {
	removedKeys := map[string]struct{}{}
	for _, mutation := range removed {
		removedKeys[mutation.Kind+"\x00"+mutation.ID] = struct{}{}
	}
	items := make([]model.ResourceRetentionItem, 0, len(report.Resources))
	for _, item := range report.Resources {
		if _, ok := removedKeys[item.Kind+"\x00"+item.ID]; ok {
			continue
		}
		items = append(items, item)
	}
	return items
}

func sortRetentionItems(items []model.ResourceRetentionItem) {
	sort.SliceStable(items, func(i, j int) bool {
		left := strings.ToLower(items[i].Kind + "/" + items[i].Name + "/" + items[i].ID)
		right := strings.ToLower(items[j].Kind + "/" + items[j].Name + "/" + items[j].ID)
		return left < right
	})
}

func sortRetentionMutations(items []model.ResourceRetentionCleanupMutation) {
	sort.SliceStable(items, func(i, j int) bool {
		left := strings.ToLower(items[i].Kind + "/" + items[i].Name + "/" + items[i].ID)
		right := strings.ToLower(items[j].Kind + "/" + items[j].Name + "/" + items[j].ID)
		return left < right
	})
}

func writerActiveAt(writer *model.WriterLease, now time.Time) bool {
	if writer == nil || writer.ExpiresAt.IsZero() {
		return false
	}
	return now.Before(writer.ExpiresAt)
}

func attachedEnvironmentExists(retention model.ResourceRetention, environments map[string]model.Environment) bool {
	id := strings.TrimSpace(retention.AttachedEnvironmentID)
	if id == "" {
		return false
	}
	_, ok := environments[id]
	return ok
}

func skillIsStandalone(state model.State, skillID string) bool {
	for _, skill := range state.Skills {
		if skill.ID == skillID {
			return strings.TrimSpace(skill.SourceID) == ""
		}
	}
	return false
}

func skillBelongsToRemovedSource(state model.State, skillID string, sourceIDs map[string]struct{}) bool {
	for _, skill := range state.Skills {
		if skill.ID != skillID || strings.TrimSpace(skill.SourceID) == "" {
			continue
		}
		_, ok := sourceIDs[skill.SourceID]
		return ok
	}
	return false
}

func removeEnvironmentState(state *model.State, id string, now time.Time) (model.Environment, bool) {
	for _, managed := range state.ManagedWorktrees {
		if managed.EnvironmentID == id {
			return model.Environment{}, false
		}
	}
	for i := range state.Environments {
		if state.Environments[i].ID != id {
			continue
		}
		environment := state.Environments[i]
		if strings.TrimSpace(environment.State) != "ready" || writerActiveAt(environment.Writer, now) {
			return model.Environment{}, false
		}
		environment.Writer = nil
		state.Environments = append(state.Environments[:i], state.Environments[i+1:]...)
		return environment, true
	}
	return model.Environment{}, false
}

func removeSkillSourceState(state *model.State, id string) (model.SkillSource, []model.CatalogEntry, bool) {
	index := -1
	for i := range state.SkillSources {
		if state.SkillSources[i].ID == id {
			index = i
			break
		}
	}
	if index < 0 {
		return model.SkillSource{}, nil, false
	}
	source := state.SkillSources[index]
	state.SkillSources = append(state.SkillSources[:index], state.SkillSources[index+1:]...)
	removedSkills := []model.CatalogEntry{}
	keptSkills := state.Skills[:0]
	for _, skill := range state.Skills {
		if skill.SourceID == id {
			removedSkills = append(removedSkills, skill)
			continue
		}
		keptSkills = append(keptSkills, skill)
	}
	state.Skills = keptSkills
	return source, removedSkills, true
}

func removeStandaloneSkillState(state *model.State, id string) (model.CatalogEntry, bool) {
	for i := range state.Skills {
		if state.Skills[i].ID != id || strings.TrimSpace(state.Skills[i].SourceID) != "" {
			continue
		}
		skill := state.Skills[i]
		state.Skills = append(state.Skills[:i], state.Skills[i+1:]...)
		return skill, true
	}
	return model.CatalogEntry{}, false
}

func removeMCPState(state *model.State, id string) (model.MCPDefinition, bool) {
	for i := range state.MCPs {
		if state.MCPs[i].ID != id {
			continue
		}
		definition := state.MCPs[i]
		state.MCPs = append(state.MCPs[:i], state.MCPs[i+1:]...)
		return definition, true
	}
	return model.MCPDefinition{}, false
}

func normalizeRetentionUpdateRequest(request model.ResourceRetentionUpdateRequest, temporary bool) (model.ResourceRetentionUpdateRequest, error) {
	request.Kind = strings.TrimSpace(request.Kind)
	request.ID = strings.TrimSpace(request.ID)
	request.CreatorSurface = strings.TrimSpace(request.CreatorSurface)
	request.OwnerID = strings.TrimSpace(request.OwnerID)
	request.SessionID = strings.TrimSpace(request.SessionID)
	request.RunID = strings.TrimSpace(request.RunID)
	request.Policy = strings.TrimSpace(request.Policy)
	request.AttachedEnvironmentID = strings.TrimSpace(request.AttachedEnvironmentID)
	if request.CreatorSurface == "" {
		request.CreatorSurface = "admin"
	}
	switch request.Kind {
	case model.RetentionResourceEnvironment, model.RetentionResourceMCP, model.RetentionResourceSkillSource, model.RetentionResourceSkill:
	default:
		return model.ResourceRetentionUpdateRequest{}, fmt.Errorf("unsupported retention resource kind %q", request.Kind)
	}
	if request.ID == "" {
		return model.ResourceRetentionUpdateRequest{}, fmt.Errorf("retention resource id is required")
	}
	if temporary && request.OwnerID == "" {
		return model.ResourceRetentionUpdateRequest{}, fmt.Errorf("owner_id is required when marking a resource temporary")
	}
	return request, nil
}

func updateResourceRetentionState(state *model.State, request model.ResourceRetentionUpdateRequest, now time.Time, update func(model.ResourceRetention) model.ResourceRetention) ([]model.ResourceRetentionUpdateItem, error) {
	updated := []model.ResourceRetentionUpdateItem{}
	switch request.Kind {
	case model.RetentionResourceEnvironment:
		for i := range state.Environments {
			if state.Environments[i].ID != request.ID {
				continue
			}
			state.Environments[i].Retention = update(state.Environments[i].Retention)
			state.Environments[i].UpdatedAt = now
			updated = append(updated, retentionUpdateItem(model.RetentionResourceEnvironment, state.Environments[i].ID, state.Environments[i].Name, state.Environments[i].Retention))
			return updated, nil
		}
	case model.RetentionResourceMCP:
		for i := range state.MCPs {
			if state.MCPs[i].ID != request.ID {
				continue
			}
			state.MCPs[i].Retention = update(state.MCPs[i].Retention)
			updated = append(updated, retentionUpdateItem(model.RetentionResourceMCP, state.MCPs[i].ID, state.MCPs[i].Name, state.MCPs[i].Retention))
			return updated, nil
		}
	case model.RetentionResourceSkillSource:
		for i := range state.SkillSources {
			if state.SkillSources[i].ID != request.ID {
				continue
			}
			retention := update(state.SkillSources[i].Retention)
			state.SkillSources[i].Retention = cloneResourceRetention(retention)
			state.SkillSources[i].UpdatedAt = now
			updated = append(updated, retentionUpdateItem(model.RetentionResourceSkillSource, state.SkillSources[i].ID, state.SkillSources[i].Root, state.SkillSources[i].Retention))
			for j := range state.Skills {
				if state.Skills[j].SourceID != request.ID {
					continue
				}
				state.Skills[j].Retention = cloneResourceRetention(retention)
				updated = append(updated, retentionUpdateItem(model.RetentionResourceSkill, state.Skills[j].ID, state.Skills[j].Name, state.Skills[j].Retention))
			}
			return updated, nil
		}
	case model.RetentionResourceSkill:
		for i := range state.Skills {
			if state.Skills[i].ID != request.ID {
				continue
			}
			if strings.TrimSpace(state.Skills[i].SourceID) != "" {
				return nil, fmt.Errorf("source-owned skill %q retention must be changed at the skill_source boundary", request.ID)
			}
			state.Skills[i].Retention = update(state.Skills[i].Retention)
			updated = append(updated, retentionUpdateItem(model.RetentionResourceSkill, state.Skills[i].ID, state.Skills[i].Name, state.Skills[i].Retention))
			return updated, nil
		}
	}
	return nil, fmt.Errorf("retention resource %s/%s not found", request.Kind, request.ID)
}

func promoteRetentionDurable(current model.ResourceRetention, request model.ResourceRetentionUpdateRequest, now time.Time) model.ResourceRetention {
	createdAt := current.CreatedAt
	if createdAt == nil {
		created := now.UTC()
		createdAt = &created
	}
	return model.ResourceRetention{
		Persistence:    model.PersistenceDurable,
		CreatorSurface: request.CreatorSurface,
		CreatedAt:      cloneTime(createdAt),
		LastUsedAt:     cloneTime(current.LastUsedAt),
	}
}

func markRetentionTemporary(current model.ResourceRetention, request model.ResourceRetentionUpdateRequest, now time.Time) model.ResourceRetention {
	createdAt := current.CreatedAt
	if createdAt == nil {
		created := now.UTC()
		createdAt = &created
	}
	return model.ResourceRetention{
		Persistence:           model.PersistenceTemporary,
		CreatorSurface:        request.CreatorSurface,
		OwnerID:               request.OwnerID,
		SessionID:             request.SessionID,
		RunID:                 request.RunID,
		CreatedAt:             cloneTime(createdAt),
		LastUsedAt:            cloneTime(current.LastUsedAt),
		ExpiresAt:             cloneTime(request.ExpiresAt),
		Policy:                request.Policy,
		AttachedEnvironmentID: request.AttachedEnvironmentID,
	}
}

func validateAttachedEnvironment(state *model.State, environmentID string) error {
	if strings.TrimSpace(environmentID) == "" {
		return nil
	}
	for _, env := range state.Environments {
		if env.ID == environmentID {
			return nil
		}
	}
	return fmt.Errorf("attached environment %q not found", environmentID)
}

func retentionUpdateItem(kind, id, name string, retention model.ResourceRetention) model.ResourceRetentionUpdateItem {
	return model.ResourceRetentionUpdateItem{Kind: kind, ID: id, Name: name, Retention: cloneResourceRetention(retention)}
}

func sortRetentionUpdateItems(items []model.ResourceRetentionUpdateItem) {
	sort.SliceStable(items, func(i, j int) bool {
		left := strings.ToLower(items[i].Kind + "/" + items[i].Name + "/" + items[i].ID)
		right := strings.ToLower(items[j].Kind + "/" + items[j].Name + "/" + items[j].ID)
		return left < right
	})
}

func cloneResourceRetention(input model.ResourceRetention) model.ResourceRetention {
	input.CreatedAt = cloneTime(input.CreatedAt)
	input.LastUsedAt = cloneTime(input.LastUsedAt)
	input.ExpiresAt = cloneTime(input.ExpiresAt)
	return input
}

func cloneTime(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	copy := value.UTC()
	return &copy
}
