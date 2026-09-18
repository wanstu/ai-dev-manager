package app

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"ai-dev-manager-v2/internal/model"
	"ai-dev-manager-v2/internal/runtime"
)

const (
	defaultContextMaxDepth         = 3
	defaultContextMaxEntries       = 1000
	defaultContextMaxDigestEntries = 80
	defaultContextMaxOutputBytes   = 65536

	hardContextMaxDepth         = 8
	hardContextMaxEntries       = 10000
	hardContextMaxDigestEntries = 500
	hardContextMaxOutputBytes   = 262144
	minContextMaxOutputBytes    = 4096

	maxContextAvailableCapabilities = 128
	maxContextCapabilityIssues      = 64
	maxContextMCPs                  = 64
	maxContextMCPToolNames          = 128
	maxContextSkills                = 64
	maxContextVerifiers             = 32
)

func (s *Service) EnvironmentContextBundle(ctx context.Context, environmentID string, request model.EnvironmentContextRequest) (model.EnvironmentContextBundle, error) {
	limits, err := normalizeEnvironmentContextRequest(request)
	if err != nil {
		return model.EnvironmentContextBundle{}, err
	}
	env, err := s.Environments.Get(environmentID)
	if err != nil {
		return model.EnvironmentContextBundle{}, err
	}
	ws, err := s.Workspaces.Get(env.WorkspaceID)
	if err != nil {
		return model.EnvironmentContextBundle{}, err
	}

	tree, err := s.EnvironmentTreeDigest(ctx, environmentID, model.DiscoveryRequest{
		Path:             request.Path,
		MaxDepth:         limits.MaxDepth,
		MaxEntries:       limits.MaxEntries,
		MaxDigestEntries: limits.MaxDigestEntries,
		MaxOutputBytes:   limits.MaxOutputBytes,
	})
	if err != nil {
		return model.EnvironmentContextBundle{}, err
	}
	capabilityReport, err := s.environmentCapabilityReportPassive(ctx, env, ws)
	if err != nil {
		return model.EnvironmentContextBundle{}, err
	}
	injectionPlan, err := s.buildEnvironmentInjectionPlan(env, ws, capabilityReport)
	if err != nil {
		return model.EnvironmentContextBundle{}, err
	}

	bundle := model.EnvironmentContextBundle{
		Environment: model.EnvironmentContextIdentity{
			EnvironmentID:      env.ID,
			EnvironmentName:    env.Name,
			WorkspaceID:        ws.ID,
			WorkspaceName:      ws.Name,
			Root:               env.Root,
			State:              env.State,
			PrivateMemoryCount: len(env.PrivateMemory),
			WriterPresent:      env.Writer != nil,
		},
		GeneratedAt: time.Now().UTC(),
		Limits:      limits,
		Tree: model.EnvironmentContextTree{
			ScanPath:             tree.ScanPath,
			ObservedAt:           tree.ObservedAt,
			Limits:               tree.Limits,
			VisitedEntries:       tree.VisitedEntries,
			ReadBatches:          tree.ReadBatches,
			Digest:               append([]model.DirectoryDigestEntry{}, tree.Digest...),
			Truncated:            tree.Truncated,
			StopReasons:          append([]string{}, tree.StopReasons...),
			Coverage:             tree.Coverage,
			SkippedDirectories:   tree.SkippedDirectories,
			SkippedLinks:         tree.SkippedLinks,
			OmittedDigestEntries: tree.OmittedDigestEntries,
			OmittedDiagnostics:   tree.OmittedDiagnostics + len(tree.Diagnostics),
		},
		AvailableCapabilities: []model.EnvironmentContextCapability{},
		CapabilityIssues:      []model.EnvironmentContextCapability{},
		MCPs:                  []model.EnvironmentContextMCP{},
		Skills:                []model.EnvironmentContextSkill{},
		Verifiers:             []model.EnvironmentContextVerifier{},
		Guidance:              []model.EnvironmentContextGuidance{},
		Coverage:              "complete",
		CoverageReasons:       []string{},
	}
	if env.Writer != nil {
		expires := env.Writer.ExpiresAt.UTC()
		bundle.Environment.WriterExpiresAt = &expires
	}
	if tree.Truncated || tree.OmittedDigestEntries > 0 || len(tree.Diagnostics) > 0 || tree.OmittedDiagnostics > 0 {
		markEnvironmentContextPartial(&bundle, "tree_partial")
	}

	facts := environmentContextCapabilityFacts(capabilityReport)

	for _, planned := range injectionPlan.MCPs {
		bundle.MCPs = append(bundle.MCPs, model.EnvironmentContextMCP{
			ID:                          planned.ID,
			Name:                        planned.Name,
			SelectionSources:            append([]string{}, planned.SelectionSources...),
			DefaultIncludeInEnvironment: planned.DefaultIncludeInEnvironment,
			Selected:                    planned.Selected,
			Injectable:                  planned.Injectable,
			NextAction:                  planned.NextAction,
			State:                       planned.State,
			ReasonCode:                  planned.ReasonCode,
			ObservationState:            "not_observed",
			ToolInventoryObserved:       false,
			ToolNames:                   []string{},
		})
	}

	for _, planned := range injectionPlan.Skills {
		bundle.Skills = append(bundle.Skills, model.EnvironmentContextSkill{
			ID:                          planned.ID,
			Name:                        planned.Name,
			SelectionSources:            append([]string{}, planned.SelectionSources...),
			DefaultIncludeInEnvironment: planned.DefaultIncludeInEnvironment,
			Selected:                    planned.Selected,
			Injectable:                  planned.Injectable,
			NextAction:                  planned.NextAction,
			State:                       string(planned.State),
			ReasonCode:                  planned.ReasonCode,
			Reason:                      planned.Message,
			RelativeArtifactPath:        planned.RelativeArtifactPath,
			SupportRootCount:            planned.SupportRootCount,
			MissingSupportRootCount:     planned.MissingSupportRootCount,
		})
	}

	verifiers := append([]model.VerifierDefinition{}, env.Verifiers...)
	sort.Slice(verifiers, func(i, j int) bool {
		left := verifiers[i].Name + "/" + verifiers[i].ID
		right := verifiers[j].Name + "/" + verifiers[j].ID
		return stableLess(left, right)
	})
	for _, definition := range verifiers {
		item := model.EnvironmentContextVerifier{
			ID:             definition.ID,
			Name:           definition.Name,
			Kind:           definition.Kind,
			Enabled:        definition.Enabled,
			State:          model.CapabilityStateUnavailable,
			ReasonCode:     "capability_fact_missing",
			RequiresWriter: true,
		}
		if fact, ok := facts["verifier/"+definition.ID]; ok {
			item.State = fact.State
			item.ReasonCode = fact.ReasonCode
			item.RequiresWriter = fact.RequiresWriter
		}
		bundle.Verifiers = append(bundle.Verifiers, item)
	}

	ApplyEnvironmentContextCapabilityReport(&bundle, capabilityReport)
	if err := CompactEnvironmentContextBundle(&bundle); err != nil {
		return model.EnvironmentContextBundle{}, err
	}
	return bundle, nil
}

func normalizeEnvironmentContextRequest(request model.EnvironmentContextRequest) (model.EnvironmentContextLimits, error) {
	values := []int{request.MaxDepth, request.MaxEntries, request.MaxDigestEntries, request.MaxOutputBytes}
	defaults := []int{defaultContextMaxDepth, defaultContextMaxEntries, defaultContextMaxDigestEntries, defaultContextMaxOutputBytes}
	caps := []int{hardContextMaxDepth, hardContextMaxEntries, hardContextMaxDigestEntries, hardContextMaxOutputBytes}
	for i := range values {
		if values[i] < 0 || values[i] > caps[i] {
			return model.EnvironmentContextLimits{}, fmt.Errorf("environment context budget out of range")
		}
		if values[i] == 0 {
			values[i] = defaults[i]
		}
	}
	if values[3] < minContextMaxOutputBytes {
		return model.EnvironmentContextLimits{}, fmt.Errorf("max_output_bytes must be at least %d", minContextMaxOutputBytes)
	}
	return model.EnvironmentContextLimits{
		MaxDepth:         values[0],
		MaxEntries:       values[1],
		MaxDigestEntries: values[2],
		MaxOutputBytes:   values[3],
	}, nil
}

func environmentContextCapabilityFacts(report model.CapabilityReport) map[string]model.CapabilityFact {
	facts := make(map[string]model.CapabilityFact, len(report.Facts))
	for _, fact := range report.Facts {
		facts[fact.Key] = fact
	}
	return facts
}

// ApplyEnvironmentContextCapabilityReport projects one canonical capability
// report into the compact context schema and refreshes factual guidance. Gateway
// owner enrichment uses this after applying passive owner-local observations.
func ApplyEnvironmentContextCapabilityReport(bundle *model.EnvironmentContextBundle, report model.CapabilityReport) {
	if bundle == nil {
		return
	}
	bundle.AvailableCapabilities = []model.EnvironmentContextCapability{}
	bundle.CapabilityIssues = []model.EnvironmentContextCapability{}
	facts := environmentContextCapabilityFacts(report)
	for _, fact := range report.Facts {
		item := contextCapability(fact)
		if fact.State == model.CapabilityStateAvailable {
			bundle.AvailableCapabilities = append(bundle.AvailableCapabilities, item)
		} else {
			bundle.CapabilityIssues = append(bundle.CapabilityIssues, item)
		}
	}
	sortContextCapabilities(bundle.AvailableCapabilities)
	sortContextCapabilities(bundle.CapabilityIssues)
	bundle.Guidance = environmentContextGuidance(facts, bundle.MCPs, bundle.Skills, bundle.Verifiers)
}

func contextCapability(fact model.CapabilityFact) model.EnvironmentContextCapability {
	message := fact.Message
	if fact.Kind == capabilityKindMCP && strings.HasPrefix(fact.Key, "mcp/") {
		message = "Environment-selected MCP static capability state is " + string(fact.State) + "."
		if fact.ReasonCode != "" {
			message += " Reason: " + fact.ReasonCode + "."
		}
	}
	if fact.Kind == capabilityKindSkill && strings.HasPrefix(fact.Key, "skill/") {
		message = "Environment-selected Skill availability state is " + string(fact.State) + "."
		if fact.ReasonCode != "" {
			message += " Reason: " + fact.ReasonCode + "."
		}
	}
	return model.EnvironmentContextCapability{
		Key:            fact.Key,
		Kind:           fact.Kind,
		State:          fact.State,
		ReasonCode:     fact.ReasonCode,
		Message:        message,
		RequiresWriter: fact.RequiresWriter,
		Source:         fact.Source,
		ObservedAt:     fact.ObservedAt,
		Freshness:      fact.Freshness,
	}
}

func sortContextCapabilities(items []model.EnvironmentContextCapability) {
	sort.Slice(items, func(i, j int) bool {
		left := items[i].Kind + "/" + items[i].Key
		right := items[j].Kind + "/" + items[j].Key
		return stableLess(left, right)
	})
}

func environmentContextGuidance(facts map[string]model.CapabilityFact, mcps []model.EnvironmentContextMCP, skills []model.EnvironmentContextSkill, verifiers []model.EnvironmentContextVerifier) []model.EnvironmentContextGuidance {
	guidance := []model.EnvironmentContextGuidance{}
	inspectState := combinedCapabilityState(facts, runtime.CapabilityTree, runtime.CapabilityRead, runtime.CapabilitySearch)
	guidance = append(guidance, model.EnvironmentContextGuidance{
		Operation: "files.inspect", State: inspectState, RequiresWriter: false,
		Message: "Use Environment-scoped tree/read/search operations for project inspection; these reads do not require the writer lease.",
	})
	mutationState := combinedCapabilityState(facts, runtime.CapabilityWrite, runtime.CapabilityEdit, runtime.CapabilityDelete)
	guidance = append(guidance, model.EnvironmentContextGuidance{
		Operation: "files.mutate", State: mutationState, RequiresWriter: true,
		Message: "Environment file mutation uses write/edit/delete and requires the active writer lease for the same physical root.",
	})
	verifierState := model.CapabilityStateUnconfigured
	if fact, ok := facts["verifier.definitions"]; ok {
		verifierState = fact.State
	}
	for _, verifier := range verifiers {
		if verifier.State == model.CapabilityStateAvailable {
			verifierState = model.CapabilityStateAvailable
			break
		}
	}
	guidance = append(guidance, model.EnvironmentContextGuidance{
		Operation: "verifier.run", State: verifierState, RequiresWriter: true,
		Message: "Run only an explicitly configured verifier ID when verification is relevant; use environment_verifier_run_start/status/cancel for long or heavy verification, while environment_verifier_run remains blocking for short/direct use. Verifier execution requires the writer lease and does not define task success policy.",
	})
	runState := model.CapabilityStateUnconfigured
	if fact, ok := facts["run.lifecycle"]; ok {
		runState = fact.State
	}
	guidance = append(guidance, model.EnvironmentContextGuidance{
		Operation: "run.lifecycle", State: runState, RequiresWriter: true,
		Message: "Generic async run lifecycle is for one allowlisted Environment-scoped command; task planning and multi-step orchestration remain outside ADM.",
	})
	mcpState := summaryMCPState(mcps)
	guidance = append(guidance, model.EnvironmentContextGuidance{
		Operation: "mcp.access", State: mcpState, RequiresWriter: false,
		Message: "Use explicit Environment MCP inventory/call operations for an enabled MCP ID; this Core context does not probe, connect, or select a business tool automatically.",
	})
	skillState := summarySkillState(skills)
	guidance = append(guidance, model.EnvironmentContextGuidance{
		Operation: "skill.read", State: skillState, RequiresWriter: false,
		Message: "Use the explicit Environment Skill read operation for an enabled available Skill when its instructions are needed; Skill contents are not embedded in this bundle.",
	})
	return guidance
}

func combinedCapabilityState(facts map[string]model.CapabilityFact, keys ...string) model.CapabilityState {
	state := model.CapabilityStateAvailable
	for _, key := range keys {
		fact, ok := facts[key]
		if !ok {
			return model.CapabilityStateUnavailable
		}
		if fact.State == model.CapabilityStateAvailable {
			continue
		}
		if fact.State == model.CapabilityStateUnavailable || fact.State == model.CapabilityStateDisabled {
			return fact.State
		}
		state = fact.State
	}
	return state
}

func summaryMCPState(items []model.EnvironmentContextMCP) model.CapabilityState {
	if len(items) == 0 {
		return model.CapabilityStateUnconfigured
	}
	state := model.CapabilityStateUnavailable
	for _, item := range items {
		if item.State == model.CapabilityStateAvailable {
			return model.CapabilityStateAvailable
		}
		if item.State == model.CapabilityStateDegraded {
			state = model.CapabilityStateDegraded
		}
	}
	return state
}

func summarySkillState(items []model.EnvironmentContextSkill) model.CapabilityState {
	if len(items) == 0 {
		return model.CapabilityStateUnconfigured
	}
	for _, item := range items {
		if item.State == SkillAvailabilityAvailable {
			return model.CapabilityStateAvailable
		}
	}
	return model.CapabilityStateUnavailable
}

func contextSkillReason(state string) string {
	switch state {
	case SkillAvailabilityAvailable:
		return ""
	case SkillAvailabilityDisabled:
		return "Skill is not enabled for this Environment."
	case SkillAvailabilityUnresolved:
		return "Environment selects a Skill ID that is not present in the current catalog."
	case SkillAvailabilitySourceMissing:
		return "Configured Skill source is unavailable."
	case SkillAvailabilityArtifactMissing:
		return "Configured Skill artifact is missing."
	case SkillAvailabilityArtifactUnreadable:
		return "Configured Skill artifact is unreadable."
	case SkillAvailabilitySupportRootMissing:
		return "One or more configured Skill support roots are unavailable."
	case SkillAvailabilityUnconfigured:
		return "Skill catalog entry is not backed by a configured artifact."
	default:
		return "Skill availability is unavailable."
	}
}

// CompactEnvironmentContextBundle applies deterministic section limits and the
// final serialized byte budget. Stable identities are never string-truncated;
// whole lower-priority items are omitted and counted instead.
func CompactEnvironmentContextBundle(bundle *model.EnvironmentContextBundle) error {
	if bundle == nil {
		return fmt.Errorf("environment context bundle is required")
	}
	if bundle.Limits.MaxOutputBytes < minContextMaxOutputBytes || bundle.Limits.MaxOutputBytes > hardContextMaxOutputBytes {
		return fmt.Errorf("environment context output budget is invalid")
	}
	enforceEnvironmentContextSectionLimits(bundle)
	for {
		sort.Strings(bundle.CoverageReasons)
		data, err := json.Marshal(bundle)
		if err != nil {
			return err
		}
		if len(data) <= bundle.Limits.MaxOutputBytes {
			return nil
		}
		markEnvironmentContextPartial(bundle, "output_byte_limit")
		if !omitOneEnvironmentContextItem(bundle) {
			return fmt.Errorf("environment context identity metadata exceeds max_output_bytes=%d", bundle.Limits.MaxOutputBytes)
		}
	}
}

func enforceEnvironmentContextSectionLimits(bundle *model.EnvironmentContextBundle) {
	if omitted := trimContextCapabilities(&bundle.AvailableCapabilities, maxContextAvailableCapabilities); omitted > 0 {
		bundle.Omissions.AvailableCapabilities += omitted
		markEnvironmentContextPartial(bundle, "available_capability_limit")
	}
	if omitted := trimContextCapabilities(&bundle.CapabilityIssues, maxContextCapabilityIssues); omitted > 0 {
		bundle.Omissions.CapabilityIssues += omitted
		markEnvironmentContextPartial(bundle, "capability_issue_limit")
	}
	if len(bundle.MCPs) > maxContextMCPs {
		bundle.Omissions.MCPs += len(bundle.MCPs) - maxContextMCPs
		bundle.MCPs = bundle.MCPs[:maxContextMCPs]
		markEnvironmentContextPartial(bundle, "mcp_limit")
	}
	toolCount := 0
	for i := range bundle.MCPs {
		sort.Slice(bundle.MCPs[i].ToolNames, func(a, b int) bool { return stableLess(bundle.MCPs[i].ToolNames[a], bundle.MCPs[i].ToolNames[b]) })
		if toolCount >= maxContextMCPToolNames {
			bundle.Omissions.MCPToolNames += len(bundle.MCPs[i].ToolNames)
			bundle.MCPs[i].ToolNames = []string{}
			continue
		}
		remaining := maxContextMCPToolNames - toolCount
		if len(bundle.MCPs[i].ToolNames) > remaining {
			bundle.Omissions.MCPToolNames += len(bundle.MCPs[i].ToolNames) - remaining
			bundle.MCPs[i].ToolNames = bundle.MCPs[i].ToolNames[:remaining]
		}
		toolCount += len(bundle.MCPs[i].ToolNames)
	}
	if bundle.Omissions.MCPToolNames > 0 {
		markEnvironmentContextPartial(bundle, "mcp_tool_name_limit")
	}
	if len(bundle.Skills) > maxContextSkills {
		bundle.Omissions.Skills += len(bundle.Skills) - maxContextSkills
		bundle.Skills = bundle.Skills[:maxContextSkills]
		markEnvironmentContextPartial(bundle, "skill_limit")
	}
	if len(bundle.Verifiers) > maxContextVerifiers {
		bundle.Omissions.Verifiers += len(bundle.Verifiers) - maxContextVerifiers
		bundle.Verifiers = bundle.Verifiers[:maxContextVerifiers]
		markEnvironmentContextPartial(bundle, "verifier_limit")
	}
}

func trimContextCapabilities(items *[]model.EnvironmentContextCapability, max int) int {
	if len(*items) <= max {
		return 0
	}
	omitted := len(*items) - max
	*items = (*items)[:max]
	return omitted
}

func omitOneEnvironmentContextItem(bundle *model.EnvironmentContextBundle) bool {
	for i := len(bundle.MCPs) - 1; i >= 0; i-- {
		if n := len(bundle.MCPs[i].ToolNames); n > 0 {
			bundle.MCPs[i].ToolNames = bundle.MCPs[i].ToolNames[:n-1]
			bundle.Omissions.MCPToolNames++
			return true
		}
	}
	if n := len(bundle.Tree.Digest); n > 0 {
		bundle.Tree.Digest = bundle.Tree.Digest[:n-1]
		bundle.Tree.OmittedDigestEntries++
		bundle.Omissions.TreeDigestEntries++
		if !strings.HasPrefix(bundle.Tree.Coverage, "partial context projection; ") {
			bundle.Tree.Coverage = "partial context projection; " + bundle.Tree.Coverage
		}
		return true
	}
	if n := len(bundle.CapabilityIssues); n > 0 {
		bundle.CapabilityIssues = bundle.CapabilityIssues[:n-1]
		bundle.Omissions.CapabilityIssues++
		return true
	}
	if n := len(bundle.AvailableCapabilities); n > 0 {
		bundle.AvailableCapabilities = bundle.AvailableCapabilities[:n-1]
		bundle.Omissions.AvailableCapabilities++
		return true
	}
	if n := len(bundle.Skills); n > 0 {
		bundle.Skills = bundle.Skills[:n-1]
		bundle.Omissions.Skills++
		return true
	}
	if n := len(bundle.MCPs); n > 0 {
		bundle.MCPs = bundle.MCPs[:n-1]
		bundle.Omissions.MCPs++
		return true
	}
	if n := len(bundle.Verifiers); n > 0 {
		bundle.Verifiers = bundle.Verifiers[:n-1]
		bundle.Omissions.Verifiers++
		return true
	}
	if n := len(bundle.Guidance); n > 0 {
		bundle.Guidance = bundle.Guidance[:n-1]
		bundle.Omissions.Guidance++
		return true
	}
	return false
}

func markEnvironmentContextPartial(bundle *model.EnvironmentContextBundle, reason string) {
	bundle.Coverage = "partial"
	for _, existing := range bundle.CoverageReasons {
		if existing == reason {
			return
		}
	}
	bundle.CoverageReasons = append(bundle.CoverageReasons, reason)
}

func stableLess(left, right string) bool {
	leftLower, rightLower := strings.ToLower(left), strings.ToLower(right)
	if leftLower != rightLower {
		return leftLower < rightLower
	}
	return left < right
}
