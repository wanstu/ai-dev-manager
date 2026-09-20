package app

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"ai-dev-manager-v2/internal/catalog"
	"ai-dev-manager-v2/internal/model"
	"ai-dev-manager-v2/internal/runtime"
)

const capabilitySourceStatic = "app.static"

const (
	capabilityKindEnvironment = "environment"
	capabilityKindFiles       = "files"
	capabilityKindExec        = "exec"
	capabilityKindVerifier    = "verifier"
	capabilityKindGit         = "git"
	capabilityKindIsolation   = "isolation"
	capabilityKindMCP         = "mcp"
	capabilityKindSkill       = "skill"
	capabilityKindProcess     = "process"
	capabilityKindRun         = "run"
)

func (s *Service) EnvironmentCapabilityReport(ctx context.Context, environmentID string) (model.CapabilityReport, error) {
	env, err := s.Environments.Get(environmentID)
	if err != nil {
		return model.CapabilityReport{}, err
	}
	ws, err := s.Workspaces.Get(env.WorkspaceID)
	if err != nil {
		return model.CapabilityReport{}, err
	}
	return s.environmentCapabilityReport(ctx, env, ws)
}

// EnvironmentCapabilityReportPassive returns the same canonical capability facts
// without executing Git. It exists for compact context composition, where optional
// runtime observations must stay passive.
func (s *Service) EnvironmentCapabilityReportPassive(ctx context.Context, environmentID string) (model.CapabilityReport, error) {
	env, err := s.Environments.Get(environmentID)
	if err != nil {
		return model.CapabilityReport{}, err
	}
	ws, err := s.Workspaces.Get(env.WorkspaceID)
	if err != nil {
		return model.CapabilityReport{}, err
	}
	return s.environmentCapabilityReportPassive(ctx, env, ws)
}

func (s *Service) environmentCapabilityReport(ctx context.Context, env model.Environment, ws model.Workspace) (model.CapabilityReport, error) {
	return s.environmentCapabilityReportWithOptions(ctx, env, ws, true)
}

func (s *Service) environmentCapabilityReportPassive(ctx context.Context, env model.Environment, ws model.Workspace) (model.CapabilityReport, error) {
	return s.environmentCapabilityReportWithOptions(ctx, env, ws, false)
}

func (s *Service) environmentCapabilityReportWithOptions(ctx context.Context, env model.Environment, ws model.Workspace, inspectGit bool) (model.CapabilityReport, error) {
	state, err := s.Store.Load()
	if err != nil {
		return model.CapabilityReport{}, err
	}

	report := model.CapabilityReport{
		EnvironmentID: env.ID,
		GeneratedAt:   time.Now().UTC(),
		Facts:         []model.CapabilityFact{},
	}

	baseEvidence := []model.CapabilityEvidence{environmentEvidence(env, ws)}
	managed, managedOK, isolationErr := s.managedWorktreeFactInput(ctx, env)
	report.Facts = append(report.Facts, s.isolationCapabilityFact(env, ws, managed, managedOK, isolationErr))

	var rt *runtime.Runtime
	var runtimeErr error
	if isolationErr == nil {
		rt, runtimeErr = runtime.NewWithExecutionPolicy(env.Root, state.AllowedExecutables, state.BlockedExecutables, os.Environ(), state.ExecFullAuthorization)
	}

	rootErr := firstError(isolationErr, runtimeErr)
	rootState := model.CapabilityStateAvailable
	rootReason := ""
	rootMessage := "Environment root passed workspace/isolation and Runtime root validation."
	rootEvidence := baseEvidence
	if isolationErr != nil {
		rootEvidence = appendManagedEvidence(baseEvidence, managed, managedOK)
	}
	if rootErr != nil {
		rootState = model.CapabilityStateUnavailable
		rootReason = rootFailureReason(rootErr)
		rootMessage = rootFailureMessage(rootErr)
	}
	report.Facts = append(report.Facts, capabilityFact("environment.root", capabilityKindEnvironment, rootState, rootReason, rootMessage, false, rootEvidence))
	report.Facts = append(report.Facts, fileCapabilityFacts(env, ws, rootErr)...)
	execFacts, execAvailable := execCapabilityFacts(ctx, env, ws, rt, state.AllowedExecutables, state.ExecFullAuthorization, rootErr)
	report.Facts = append(report.Facts, execFacts...)
	report.Facts = append(report.Facts, verifierCapabilityFacts(ctx, env, ws, rt, rootErr)...)
	report.Facts = append(report.Facts, gitCapabilityFacts(ctx, env, ws, rt, rootErr, inspectGit)...)
	report.Facts = append(report.Facts, processRunCapabilityFacts(env, ws, rootErr, execAvailable)...)
	mcpFacts := s.mcpCapabilityFacts(ctx, env, ws, rt, rootErr)
	report.Facts = append(report.Facts, mcpFacts...)
	report.Facts = append(report.Facts, s.investigationProviderCapabilityFacts(env, ws, mcpFacts)...)
	report.Facts = append(report.Facts, s.skillCapabilityFacts(env, ws)...)

	sort.SliceStable(report.Facts, func(i, j int) bool {
		left := report.Facts[i].Kind + "/" + report.Facts[i].Key
		right := report.Facts[j].Kind + "/" + report.Facts[j].Key
		return strings.ToLower(left) < strings.ToLower(right)
	})
	return report, nil
}

func (s *Service) managedWorktreeFactInput(ctx context.Context, env model.Environment) (model.ManagedWorktree, bool, error) {
	if s.Isolation == nil {
		return model.ManagedWorktree{}, false, nil
	}
	managed, ok, err := s.Isolation.GetByEnvironment(env.ID)
	if err != nil {
		return model.ManagedWorktree{}, false, err
	}
	if err := s.Isolation.ValidateEnvironment(ctx, env); err != nil {
		return managed, ok, err
	}
	return managed, ok, nil
}

func (s *Service) isolationCapabilityFact(env model.Environment, ws model.Workspace, managed model.ManagedWorktree, managedOK bool, isolationErr error) model.CapabilityFact {
	evidence := appendManagedEvidence([]model.CapabilityEvidence{environmentEvidence(env, ws)}, managed, managedOK)
	if s.Isolation == nil {
		return capabilityFact("isolation.managed_worktree", capabilityKindIsolation, model.CapabilityStateUnconfigured, "isolation_service_unavailable", "Managed worktree isolation service is not configured.", false, evidence)
	}
	if isolationErr != nil {
		reason := "environment_validation_failed"
		if managedOK {
			reason = "managed_worktree_invalid"
		}
		return capabilityFact("isolation.managed_worktree", capabilityKindIsolation, model.CapabilityStateUnavailable, reason, sanitizeCapabilityMessage(isolationErr.Error()), false, evidence)
	}
	if !managedOK {
		return capabilityFact("isolation.managed_worktree", capabilityKindIsolation, model.CapabilityStateUnconfigured, "not_managed", "Environment uses a workspace-contained root; managed worktree isolation is not configured.", false, evidence)
	}
	return capabilityFact("isolation.managed_worktree", capabilityKindIsolation, model.CapabilityStateAvailable, "", "Managed worktree metadata and root identity are valid.", false, evidence)
}

func fileCapabilityFacts(env model.Environment, ws model.Workspace, rootErr error) []model.CapabilityFact {
	definitions := []struct {
		key            string
		requiresWriter bool
	}{
		{runtime.CapabilityTree, false},
		{runtime.CapabilityRead, false},
		{runtime.CapabilitySearch, false},
		{runtime.CapabilityWrite, true},
		{runtime.CapabilityEdit, true},
		{runtime.CapabilityDelete, true},
	}
	facts := make([]model.CapabilityFact, 0, len(definitions))
	for _, definition := range definitions {
		state := model.CapabilityStateAvailable
		reason := ""
		message := "Environment root is readable through Runtime path containment."
		if definition.requiresWriter {
			message = "Environment root is writable through Runtime path containment when the caller owns the writer lease."
		}
		if rootErr != nil {
			state = model.CapabilityStateUnavailable
			reason = rootFailureReason(rootErr)
			message = rootFailureMessage(rootErr)
		}
		evidence := []model.CapabilityEvidence{environmentEvidence(env, ws)}
		if definition.requiresWriter {
			evidence = append(evidence, writerEvidence(env))
		}
		facts = append(facts, capabilityFact(definition.key, capabilityKindFiles, state, reason, message, definition.requiresWriter, evidence))
	}
	return facts
}

func execCapabilityFacts(ctx context.Context, env model.Environment, ws model.Workspace, rt *runtime.Runtime, allowed []string, fullAuthorization bool, rootErr error) ([]model.CapabilityFact, bool) {
	evidence := []model.CapabilityEvidence{environmentEvidence(env, ws), writerEvidence(env), {
		Kind: capabilityKindExec,
		Details: map[string]string{
			"allowed_executable_count": strconv.Itoa(len(allowed)),
			"full_authorization":       strconv.FormatBool(fullAuthorization),
		},
	}}
	if rootErr != nil {
		return []model.CapabilityFact{capabilityFact(runtime.CapabilityExec, capabilityKindExec, model.CapabilityStateUnavailable, rootFailureReason(rootErr), rootFailureMessage(rootErr), true, evidence)}, false
	}
	if len(allowed) == 0 {
		if fullAuthorization {
			return []model.CapabilityFact{capabilityFact(
				runtime.CapabilityExec,
				capabilityKindExec,
				model.CapabilityStateAvailable,
				"full_authorization",
				"Full authorization permits unlisted executables. Executions outside the allowlist are allowed and recorded as Full authorization bypass audit observations; executable availability is still resolved from PATH at execution time.",
				true,
				evidence,
			)}, true
		}
		return []model.CapabilityFact{capabilityFact(runtime.CapabilityExec, capabilityKindExec, model.CapabilityStateUnconfigured, "no_allowed_executables", "No executable is allowlisted for this Environment runtime.", true, evidence)}, false
	}

	facts := make([]model.CapabilityFact, 0, len(allowed)+1)
	available := 0
	for index, executable := range allowed {
		executable = strings.TrimSpace(executable)
		details := map[string]string{
			"allowed_executable": executable,
			"allowlist_index":    strconv.Itoa(index),
		}
		factEvidence := []model.CapabilityEvidence{environmentEvidence(env, ws), writerEvidence(env), {Kind: capabilityKindExec, Details: details}}
		if err := validatePreparedCommand(ctx, rt, executable, nil, ""); err != nil {
			facts = append(facts, capabilityFact("shell.exec/"+executable, capabilityKindExec, model.CapabilityStateUnavailable, classifyCommandCapabilityError(err), sanitizeCapabilityMessage(err.Error()), true, factEvidence))
			continue
		}
		available++
		facts = append(facts, capabilityFact("shell.exec/"+executable, capabilityKindExec, model.CapabilityStateAvailable, "", "Executable is allowlisted and can be prepared without starting a process.", true, factEvidence))
	}

	summaryState := model.CapabilityStateAvailable
	summaryReason := ""
	summaryMessage := "At least one allowlisted executable can be prepared without starting a process."
	execAvailable := available > 0
	if fullAuthorization {
		summaryReason = "full_authorization"
		summaryMessage = "Full authorization permits unlisted executables in addition to the explicit allowlist. Unlisted executions are recorded as Full authorization bypass audit observations."
		execAvailable = true
	} else if available == 0 {
		summaryState = model.CapabilityStateUnavailable
		summaryReason = "allowed_executables_unavailable"
		summaryMessage = "No allowlisted executable can be prepared under Runtime command authority."
	}
	evidence[len(evidence)-1].Details["available_executable_count"] = strconv.Itoa(available)
	facts = append(facts, capabilityFact(runtime.CapabilityExec, capabilityKindExec, summaryState, summaryReason, summaryMessage, true, evidence))
	return facts, execAvailable
}

func verifierCapabilityFacts(ctx context.Context, env model.Environment, ws model.Workspace, rt *runtime.Runtime, rootErr error) []model.CapabilityFact {
	definitions := append([]model.VerifierDefinition(nil), env.Verifiers...)
	evidence := []model.CapabilityEvidence{environmentEvidence(env, ws), writerEvidence(env), {Kind: capabilityKindVerifier, Details: map[string]string{"verifier_count": strconv.Itoa(len(definitions))}}}
	if len(definitions) == 0 {
		return []model.CapabilityFact{capabilityFact("verifier.definitions", capabilityKindVerifier, model.CapabilityStateUnconfigured, "no_verifiers_configured", "No verifier definitions are configured for this Environment.", true, evidence)}
	}

	facts := make([]model.CapabilityFact, 0, len(definitions)+1)
	available := 0
	for _, definition := range definitions {
		factEvidence := []model.CapabilityEvidence{environmentEvidence(env, ws), writerEvidence(env), verifierEvidence(definition)}
		key := "verifier/" + definition.ID
		switch {
		case !definition.Enabled:
			facts = append(facts, capabilityFact(key, capabilityKindVerifier, model.CapabilityStateDisabled, "verifier_disabled", "Verifier definition is disabled for this Environment.", true, factEvidence))
		case strings.TrimSpace(definition.Executable) == "":
			facts = append(facts, capabilityFact(key, capabilityKindVerifier, model.CapabilityStateUnavailable, "executable_required", "Verifier executable is required.", true, factEvidence))
		case rootErr != nil:
			facts = append(facts, capabilityFact(key, capabilityKindVerifier, model.CapabilityStateUnavailable, rootFailureReason(rootErr), rootFailureMessage(rootErr), true, factEvidence))
		default:
			if err := validatePreparedCommand(ctx, rt, definition.Executable, definition.Args, definition.Cwd); err != nil {
				facts = append(facts, capabilityFact(key, capabilityKindVerifier, model.CapabilityStateUnavailable, classifyCommandCapabilityError(err), sanitizeCapabilityMessage(err.Error()), true, factEvidence))
				continue
			}
			available++
			facts = append(facts, capabilityFact(key, capabilityKindVerifier, model.CapabilityStateAvailable, "", "Verifier command can be prepared without execution; running it still requires the writer lease.", true, factEvidence))
		}
	}

	summaryState := model.CapabilityStateAvailable
	summaryReason := ""
	summaryMessage := "At least one verifier definition can be prepared without execution."
	if available == 0 {
		summaryState = model.CapabilityStateDegraded
		summaryReason = "no_enabled_verifier_available"
		summaryMessage = "Verifier definitions exist, but none is currently runnable without configuration or authority errors."
	}
	facts = append(facts, capabilityFact("verifier.definitions", capabilityKindVerifier, summaryState, summaryReason, summaryMessage, true, evidence))
	return facts
}

func gitCapabilityFacts(ctx context.Context, env model.Environment, ws model.Workspace, rt *runtime.Runtime, rootErr error, inspectGit bool) []model.CapabilityFact {
	keys := []string{runtime.CapabilityGitStatus, runtime.CapabilityGitDiff, runtime.CapabilityGitBranch}
	facts := make([]model.CapabilityFact, 0, len(keys))
	state := model.CapabilityStateAvailable
	reason := ""
	message := "Git is usable for this Environment root."
	if rootErr != nil {
		state = model.CapabilityStateUnavailable
		reason = rootFailureReason(rootErr)
		message = rootFailureMessage(rootErr)
	} else if !inspectGit {
		state = model.CapabilityStateDegraded
		reason = "not_observed"
		message = "Passive capability inspection did not execute Git; use explicit Git operations when repository status is needed."
	} else if _, err := rt.GitBranch(ctx); err != nil {
		state = model.CapabilityStateUnavailable
		reason = "git_unsupported"
		message = "Environment root is not a Git worktree or git is unavailable."
	}
	for _, key := range keys {
		facts = append(facts, capabilityFact(key, capabilityKindGit, state, reason, message, false, []model.CapabilityEvidence{environmentEvidence(env, ws)}))
	}
	return facts
}

func processRunCapabilityFacts(env model.Environment, ws model.Workspace, rootErr error, execAvailable bool) []model.CapabilityFact {
	definitions := []struct {
		key  string
		kind string
	}{
		{"process.lifecycle", capabilityKindProcess},
		{"run.lifecycle", capabilityKindRun},
	}
	facts := make([]model.CapabilityFact, 0, len(definitions))
	for _, definition := range definitions {
		state := model.CapabilityStateDegraded
		reason := "gateway_owner_observation_not_included"
		message := "Application-level static diagnostics do not include Gateway-owner runtime observations; Phase 13-02 may enrich this fact."
		if rootErr != nil {
			state = model.CapabilityStateUnavailable
			reason = rootFailureReason(rootErr)
			message = rootFailureMessage(rootErr)
		} else if !execAvailable {
			state = model.CapabilityStateUnconfigured
			reason = "no_executable_available"
			message = "No allowlisted executable is currently available for Gateway-owned process/run start operations."
		}
		facts = append(facts, capabilityFact(definition.key, definition.kind, state, reason, message, true, []model.CapabilityEvidence{environmentEvidence(env, ws), writerEvidence(env)}))
	}
	return facts
}

func (s *Service) mcpCapabilityFacts(ctx context.Context, env model.Environment, ws model.Workspace, rt *runtime.Runtime, rootErr error) []model.CapabilityFact {
	entries, err := s.MCPs.List()
	if err != nil {
		return []model.CapabilityFact{capabilityFact("mcp.catalog", capabilityKindMCP, model.CapabilityStateUnavailable, "catalog_unavailable", sanitizeCapabilityMessage(err.Error()), false, []model.CapabilityEvidence{environmentEvidence(env, ws)})}
	}
	byID := make(map[string]model.MCPDefinition, len(entries))
	for _, entry := range entries {
		byID[entry.ID] = entry
	}
	facts := make([]model.CapabilityFact, 0, len(env.EnabledMCPIDs)+1)
	facts = append(facts, mcpCatalogCapabilityFact(env, ws, len(entries)))
	for _, id := range env.EnabledMCPIDs {
		entry, ok := byID[id]
		if !ok {
			facts = append(facts, capabilityFact("mcp/"+id, capabilityKindMCP, model.CapabilityStateUnavailable, "unresolved_mcp", "Environment selects an MCP ID that is not present in the current catalog.", false, []model.CapabilityEvidence{environmentEvidence(env, ws), {Kind: capabilityKindMCP, ID: id, State: "selected"}}))
			continue
		}
		facts = append(facts, mcpCapabilityFact(ctx, env, ws, entry, rt, rootErr))
	}
	return facts
}

func mcpCatalogCapabilityFact(env model.Environment, ws model.Workspace, catalogCount int) model.CapabilityFact {
	selectedCount := len(env.EnabledMCPIDs)
	details := map[string]string{
		"catalog_count":  strconv.Itoa(catalogCount),
		"selected_count": strconv.Itoa(selectedCount),
	}
	state := model.CapabilityStateAvailable
	reason := ""
	message := "MCP catalog is configured; detailed capability facts are reported for Environment-selected MCPs only."
	if catalogCount == 0 {
		state = model.CapabilityStateUnconfigured
		reason = "no_mcps_configured"
		message = "No MCP definitions are configured."
	} else if selectedCount == 0 {
		state = model.CapabilityStateUnconfigured
		reason = "no_mcps_enabled"
		message = "No MCP definitions are enabled for this Environment."
	}
	return capabilityFact("mcp.catalog", capabilityKindMCP, state, reason, message, false, []model.CapabilityEvidence{environmentEvidence(env, ws), {Kind: capabilityKindMCP, Details: details}})
}

func mcpCapabilityFact(ctx context.Context, env model.Environment, ws model.Workspace, entry model.MCPDefinition, rt *runtime.Runtime, rootErr error) model.CapabilityFact {
	evidence := []model.CapabilityEvidence{environmentEvidence(env, ws), mcpEvidence(entry, true)}
	key := "mcp/" + entry.ID
	if _, err := catalog.ValidateMCPConfig(entry.Name, catalog.MCPConfig{
		Transport:      entry.Transport,
		AuthMode:       entry.AuthMode,
		Endpoint:       entry.Endpoint,
		HeaderRefs:     entry.HeaderRefs,
		Executable:     entry.Executable,
		Args:           entry.Args,
		EnvRefs:        entry.EnvRefs,
		HealthPolicy:   entry.HealthPolicy,
		DefaultInclude: entry.DefaultIncludeInEnv,
	}); err != nil {
		return capabilityFact(key, capabilityKindMCP, model.CapabilityStateUnavailable, "invalid_mcp_config", sanitizeCapabilityMessage(err.Error()), false, evidence)
	}
	if hasUnresolvedEnvRef(entry.Endpoint) || hasUnresolvedMapRef(entry.HeaderRefs) || hasUnresolvedMapRef(entry.EnvRefs) {
		return capabilityFact(key, capabilityKindMCP, model.CapabilityStateUnavailable, "unresolved_secret_reference", "MCP connection configuration has an unresolved environment reference.", false, evidence)
	}
	if entry.Transport == catalog.MCPTransportStdio {
		if rootErr != nil {
			return capabilityFact(key, capabilityKindMCP, model.CapabilityStateUnavailable, rootFailureReason(rootErr), rootFailureMessage(rootErr), false, evidence)
		}
		if err := validatePreparedCommand(ctx, rt, entry.Executable, entry.Args, ""); err != nil {
			return capabilityFact(key, capabilityKindMCP, model.CapabilityStateUnavailable, classifyCommandCapabilityError(err), "Stdio MCP executable is unavailable under Environment authority.", false, evidence)
		}
		return capabilityFact(key, capabilityKindMCP, model.CapabilityStateAvailable, "", "Stdio MCP desired configuration is enabled and executable authority can be prepared; static inspection did not start the server.", false, evidence)
	}
	return capabilityFact(key, capabilityKindMCP, model.CapabilityStateAvailable, "", "MCP desired configuration is enabled and activatable; static inspection did not probe network/tool health.", false, evidence)
}

func (s *Service) skillCapabilityFacts(env model.Environment, ws model.Workspace) []model.CapabilityFact {
	availability, err := s.EnvironmentSkillAvailabilities(env.ID)
	if err != nil {
		return []model.CapabilityFact{capabilityFact("skill.catalog", capabilityKindSkill, model.CapabilityStateUnavailable, "catalog_unavailable", sanitizeCapabilityMessage(err.Error()), false, []model.CapabilityEvidence{environmentEvidence(env, ws)})}
	}
	facts := make([]model.CapabilityFact, 0, len(env.EnabledSkillIDs)+1)
	facts = append(facts, skillCatalogCapabilityFact(env, ws, availability.Skills))
	for _, item := range availability.Skills {
		if !item.Enabled {
			continue
		}
		facts = append(facts, skillCapabilityFact(env, ws, item))
	}
	return facts
}

func skillCatalogCapabilityFact(env model.Environment, ws model.Workspace, skills []SkillAvailability) model.CapabilityFact {
	totalCount := len(skills)
	selectedCount := 0
	availableCount := 0
	unavailableSelectedCount := 0
	for _, item := range skills {
		if !item.Enabled {
			continue
		}
		selectedCount++
		if item.State == SkillAvailabilityAvailable {
			availableCount++
		} else {
			unavailableSelectedCount++
		}
	}
	details := map[string]string{
		"catalog_count":                  strconv.Itoa(totalCount),
		"selected_count":                 strconv.Itoa(selectedCount),
		"available_selected_count":       strconv.Itoa(availableCount),
		"unavailable_selected_count":     strconv.Itoa(unavailableSelectedCount),
		"suppressed_disabled_fact_count": strconv.Itoa(totalCount - selectedCount),
	}
	state := model.CapabilityStateAvailable
	reason := ""
	message := "Skill catalog is configured; detailed capability facts are reported for Environment-selected Skills only."
	if totalCount == 0 {
		state = model.CapabilityStateUnconfigured
		reason = "no_skills_configured"
		message = "No Skill definitions are configured."
	} else if selectedCount == 0 {
		state = model.CapabilityStateUnconfigured
		reason = "no_skills_enabled"
		message = "No Skills are enabled for this Environment."
	} else if availableCount == 0 {
		state = model.CapabilityStateDegraded
		reason = "no_enabled_skill_available"
		message = "Skills are enabled for this Environment, but none is currently available."
	}
	return capabilityFact("skill.catalog", capabilityKindSkill, state, reason, message, false, []model.CapabilityEvidence{environmentEvidence(env, ws), {Kind: capabilityKindSkill, Details: details}})
}

func skillCapabilityFact(env model.Environment, ws model.Workspace, item SkillAvailability) model.CapabilityFact {
	state := model.CapabilityStateUnavailable
	reason := item.State
	message := item.Reason
	switch item.State {
	case SkillAvailabilityAvailable:
		state = model.CapabilityStateAvailable
		reason = ""
		message = "Skill artifact and support roots are available for this Environment."
	case SkillAvailabilityDisabled:
		state = model.CapabilityStateDisabled
		reason = "skill_disabled"
		if strings.TrimSpace(message) == "" {
			message = "Skill is not enabled for this Environment."
		}
	case SkillAvailabilityUnconfigured:
		state = model.CapabilityStateUnconfigured
		if strings.TrimSpace(message) == "" {
			message = "Skill catalog entry is not backed by a configured artifact."
		}
	}
	return capabilityFact("skill/"+item.SkillID, capabilityKindSkill, state, reason, message, false, []model.CapabilityEvidence{environmentEvidence(env, ws), skillEvidence(item)})
}

func validatePreparedCommand(ctx context.Context, rt *runtime.Runtime, executable string, args []string, cwd string) error {
	if rt == nil {
		return fmt.Errorf("runtime root is unavailable")
	}
	cmd, err := rt.PrepareCommand(ctx, executable, args, cwd)
	if err != nil {
		return err
	}
	if strings.TrimSpace(cmd.Path) == "" {
		return nil
	}
	info, err := os.Stat(cmd.Path)
	if err != nil {
		return fmt.Errorf("allowed executable %q is not available: %w", executable, err)
	}
	if info.IsDir() {
		return fmt.Errorf("allowed executable %q is a directory", executable)
	}
	return nil
}

func legacyCapabilitiesFromFacts(facts []model.CapabilityFact) []string {
	seen := map[string]struct{}{}
	for _, fact := range facts {
		if fact.State != model.CapabilityStateAvailable {
			continue
		}
		switch fact.Key {
		case runtime.CapabilityTree, runtime.CapabilityRead, runtime.CapabilitySearch, runtime.CapabilityWrite, runtime.CapabilityEdit, runtime.CapabilityDelete, runtime.CapabilityExec, runtime.CapabilityGitStatus, runtime.CapabilityGitDiff, runtime.CapabilityGitBranch:
			seen[fact.Key] = struct{}{}
		}
	}
	items := make([]string, 0, len(seen))
	for key := range seen {
		items = append(items, key)
	}
	sort.Strings(items)
	return items
}

func capabilityFact(key, kind string, state model.CapabilityState, reason, message string, requiresWriter bool, evidence []model.CapabilityEvidence) model.CapabilityFact {
	return model.CapabilityFact{
		Key:            key,
		Kind:           kind,
		State:          state,
		ReasonCode:     strings.TrimSpace(reason),
		Message:        sanitizeCapabilityMessage(message),
		RequiresWriter: requiresWriter,
		Evidence:       evidence,
		Source:         capabilitySourceStatic,
	}
}

func environmentEvidence(env model.Environment, ws model.Workspace) model.CapabilityEvidence {
	return model.CapabilityEvidence{
		Kind: capabilityKindEnvironment,
		ID:   env.ID,
		Name: env.Name,
		Path: env.Root,
		Details: map[string]string{
			"workspace_id":   ws.ID,
			"workspace_name": ws.Name,
			"workspace_path": ws.Path,
		},
	}
}

func writerEvidence(env model.Environment) model.CapabilityEvidence {
	details := map[string]string{"lease_state": "absent"}
	if env.Writer != nil {
		details["lease_state"] = "active"

		details["acquired_at"] = env.Writer.AcquiredAt.UTC().Format(time.RFC3339Nano)
		details["last_seen_at"] = env.Writer.LastSeenAt.UTC().Format(time.RFC3339Nano)
		details["expires_at"] = env.Writer.ExpiresAt.UTC().Format(time.RFC3339Nano)
	}
	return model.CapabilityEvidence{Kind: "writer_lease", Details: details}
}

func verifierEvidence(definition model.VerifierDefinition) model.CapabilityEvidence {
	details := map[string]string{
		"kind":        definition.Kind,
		"enabled":     strconv.FormatBool(definition.Enabled),
		"executable":  definition.Executable,
		"args_count":  strconv.Itoa(len(definition.Args)),
		"cwd":         definition.Cwd,
		"timeout_sec": strconv.FormatInt(definition.TimeoutSeconds, 10),
	}
	return model.CapabilityEvidence{Kind: capabilityKindVerifier, ID: definition.ID, Name: definition.Name, Details: compactDetails(details)}
}

func mcpEvidence(entry model.MCPDefinition, enabled bool) model.CapabilityEvidence {
	details := map[string]string{
		"enabled":                    strconv.FormatBool(enabled),
		"transport":                  entry.Transport,
		"auth_mode":                  entry.AuthMode,
		"endpoint_configured":        strconv.FormatBool(strings.TrimSpace(entry.Endpoint) != ""),
		"header_reference_keys":      strings.Join(sortedKeys(entry.HeaderRefs), ","),
		"executable":                 entry.Executable,
		"args_count":                 strconv.Itoa(len(entry.Args)),
		"env_reference_keys":         strings.Join(sortedKeys(entry.EnvRefs), ","),
		"health_check_enabled":       strconv.FormatBool(entry.HealthPolicy.HealthCheckEnabled),
		"auto_reconnect":             strconv.FormatBool(entry.HealthPolicy.AutoReconnect),
		"check_interval_seconds":     strconv.FormatInt(entry.HealthPolicy.CheckIntervalSeconds, 10),
		"probe_timeout_seconds":      strconv.FormatInt(entry.HealthPolicy.ProbeTimeoutSeconds, 10),
		"reconnect_interval_seconds": strconv.FormatInt(entry.HealthPolicy.ReconnectIntervalSeconds, 10),
	}
	return model.CapabilityEvidence{Kind: capabilityKindMCP, ID: entry.ID, Name: entry.Name, Details: compactDetails(details)}
}

func skillEvidence(item SkillAvailability) model.CapabilityEvidence {
	details := map[string]string{
		"enabled":                    strconv.FormatBool(item.Enabled),
		"source_id":                  item.SourceID,
		"source_root":                item.SourceRoot,
		"artifact_path":              item.ArtifactPath,
		"relative_artifact_path":     item.RelativeArtifactPath,
		"support_root_count":         strconv.Itoa(len(item.SupportRoots)),
		"missing_support_root_count": strconv.Itoa(len(item.MissingSupportRoots)),
	}
	return model.CapabilityEvidence{Kind: capabilityKindSkill, ID: item.SkillID, Name: item.Name, Path: item.ArtifactPath, State: item.State, Details: compactDetails(details)}
}

func appendManagedEvidence(base []model.CapabilityEvidence, managed model.ManagedWorktree, ok bool) []model.CapabilityEvidence {
	result := append([]model.CapabilityEvidence(nil), base...)
	if ok {
		result = append(result, model.CapabilityEvidence{
			Kind: capabilityKindIsolation,
			ID:   managed.ID,
			Path: managed.Root,
			Details: compactDetails(map[string]string{
				"environment_id": managed.EnvironmentID,
				"workspace_id":   managed.WorkspaceID,
				"branch":         managed.Branch,
				"base_commit":    managed.BaseCommit,
				"git_common_dir": managed.GitCommonDir,
			}),
		})
	}
	return result
}

func firstError(values ...error) error {
	for _, value := range values {
		if value != nil {
			return value
		}
	}
	return nil
}

func rootFailureReason(err error) string {
	if err == nil {
		return ""
	}
	message := strings.ToLower(err.Error())
	switch {
	case strings.Contains(message, "managed worktree"):
		return "managed_worktree_invalid"
	case strings.Contains(message, "outside workspace"):
		return "root_outside_workspace"
	case strings.Contains(message, "not a directory"), strings.Contains(message, "no such file"), strings.Contains(message, "cannot find"), strings.Contains(message, "not exist"):
		return "root_missing"
	default:
		return "runtime_root_unavailable"
	}
}

func rootFailureMessage(err error) string {
	if err == nil {
		return ""
	}
	return sanitizeCapabilityMessage(err.Error())
}

func classifyCommandCapabilityError(err error) string {
	if err == nil {
		return ""
	}
	message := strings.ToLower(err.Error())
	switch {
	case strings.Contains(message, "cwd") && strings.Contains(message, "escapes"):
		return "cwd_escapes_root"
	case strings.Contains(message, "cwd"):
		return "invalid_cwd"
	case strings.Contains(message, "blocked by the command blacklist"):
		return "executable_blocked"
	case strings.Contains(message, "not allowed"):
		return "executable_not_allowed"
	case strings.Contains(message, "not available"), strings.Contains(message, "executable file not found"), strings.Contains(message, "no such file"), strings.Contains(message, "cannot find"):
		return "missing_executable"
	case strings.Contains(message, "is a directory"):
		return "executable_is_directory"
	default:
		return "runtime_error"
	}
}

func sanitizeCapabilityMessage(message string) string {
	message = strings.TrimSpace(message)
	message = strings.ReplaceAll(message, "\r", " ")
	message = strings.ReplaceAll(message, "\n", " ")
	return strings.Join(strings.Fields(message), " ")
}

func sortedKeys(values map[string]string) []string {
	if len(values) == 0 {
		return nil
	}
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func compactDetails(details map[string]string) map[string]string {
	for key, value := range details {
		if strings.TrimSpace(value) == "" {
			delete(details, key)
		}
	}
	if len(details) == 0 {
		return nil
	}
	return details
}
