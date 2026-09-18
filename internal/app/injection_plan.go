package app

import (
	"context"
	"sort"
	"time"

	"ai-dev-manager-v2/internal/model"
)

const (
	injectionNextMCPInspect   = "environment_mcp_inspect"
	injectionNextMCPTools     = "environment_mcp_tools"
	injectionNextSkillRead    = "environment_skill_read"
	injectionNextSkillInspect = "environment_skill_inspect"
)

// EnvironmentInjectionPlan returns the canonical passive MCP/Skill injection
// decision for one explicit Environment. It never connects an MCP, reads Skill
// contents, acquires a writer, or expands Environment authority.
func (s *Service) EnvironmentInjectionPlan(ctx context.Context, environmentID string) (model.EnvironmentInjectionPlan, error) {
	env, err := s.Environments.Get(environmentID)
	if err != nil {
		return model.EnvironmentInjectionPlan{}, err
	}
	ws, err := s.Workspaces.Get(env.WorkspaceID)
	if err != nil {
		return model.EnvironmentInjectionPlan{}, err
	}
	report, err := s.environmentCapabilityReportPassive(ctx, env, ws)
	if err != nil {
		return model.EnvironmentInjectionPlan{}, err
	}
	return s.buildEnvironmentInjectionPlan(env, ws, report)
}

func (s *Service) buildEnvironmentInjectionPlan(env model.Environment, ws model.Workspace, report model.CapabilityReport) (model.EnvironmentInjectionPlan, error) {
	mcpCatalog, err := s.MCPs.List()
	if err != nil {
		return model.EnvironmentInjectionPlan{}, err
	}
	skillCatalog, err := s.Skills.List()
	if err != nil {
		return model.EnvironmentInjectionPlan{}, err
	}
	skillAvailability, err := s.EnvironmentSkillAvailabilities(env.ID)
	if err != nil {
		return model.EnvironmentInjectionPlan{}, err
	}

	generatedAt := report.GeneratedAt
	if generatedAt.IsZero() {
		generatedAt = time.Now().UTC()
	}
	plan := model.EnvironmentInjectionPlan{
		EnvironmentID: env.ID,
		WorkspaceID:   ws.ID,
		GeneratedAt:   generatedAt,
		MCPs:          []model.EnvironmentInjectionMCP{},
		Skills:        []model.EnvironmentInjectionSkill{},
	}
	facts := make(map[string]model.CapabilityFact, len(report.Facts))
	for _, fact := range report.Facts {
		facts[fact.Key] = fact
	}

	mcpByID := make(map[string]model.MCPDefinition, len(mcpCatalog))
	for _, entry := range mcpCatalog {
		mcpByID[entry.ID] = entry
	}
	for _, id := range env.EnabledMCPIDs {
		item := model.EnvironmentInjectionMCP{
			ID:               id,
			Selected:         true,
			SelectionSources: injectionSelectionSources(id, env.ExplicitMCPIDs, env.InheritedMCPIDs),
			State:            model.CapabilityStateUnavailable,
			ReasonCode:       "unresolved_mcp",
			Message:          "Environment selects an MCP ID that is not present in the current catalog.",
			NextAction:       injectionNextMCPInspect,
		}
		if entry, ok := mcpByID[id]; ok {
			item.Name = entry.Name
			item.Transport = entry.Transport
			item.DefaultIncludeInEnvironment = entry.DefaultIncludeInEnv
		}
		if fact, ok := facts["mcp/"+id]; ok {
			item.State = fact.State
			item.ReasonCode = fact.ReasonCode
			item.Message = fact.Message
		}
		item.Injectable = item.State == model.CapabilityStateAvailable
		if item.Injectable {
			item.NextAction = injectionNextMCPTools
			plan.Summary.InjectableMCPs++
		} else {
			plan.Summary.BlockedMCPs++
		}
		plan.Summary.SelectedMCPs++
		plan.MCPs = append(plan.MCPs, item)
	}
	sort.Slice(plan.MCPs, func(i, j int) bool {
		return stableLess(plan.MCPs[i].Name+"/"+plan.MCPs[i].ID, plan.MCPs[j].Name+"/"+plan.MCPs[j].ID)
	})

	skillByID := make(map[string]model.CatalogEntry, len(skillCatalog))
	for _, entry := range skillCatalog {
		skillByID[entry.ID] = entry
	}
	for _, availability := range skillAvailability.Skills {
		if !availability.Enabled {
			continue
		}
		item := model.EnvironmentInjectionSkill{
			ID:                      availability.SkillID,
			Name:                    availability.Name,
			Selected:                true,
			SelectionSources:        injectionSelectionSources(availability.SkillID, env.ExplicitSkillIDs, env.InheritedSkillIDs),
			State:                   skillInjectionCapabilityState(availability.State),
			ReasonCode:              availability.State,
			Message:                 contextSkillReason(availability.State),
			RelativeArtifactPath:    availability.RelativeArtifactPath,
			SupportRootCount:        len(availability.SupportRoots),
			MissingSupportRootCount: len(availability.MissingSupportRoots),
			NextAction:              injectionNextSkillInspect,
		}
		if entry, ok := skillByID[availability.SkillID]; ok {
			item.DefaultIncludeInEnvironment = entry.DefaultIncludeInEnv
			if item.Name == "" {
				item.Name = entry.Name
			}
		}
		if fact, ok := facts["skill/"+availability.SkillID]; ok {
			item.State = fact.State
			item.ReasonCode = fact.ReasonCode
			item.Message = fact.Message
		}
		item.Injectable = item.State == model.CapabilityStateAvailable
		if item.Injectable {
			item.NextAction = injectionNextSkillRead
			plan.Summary.InjectableSkills++
		} else {
			plan.Summary.BlockedSkills++
		}
		plan.Summary.SelectedSkills++
		plan.Skills = append(plan.Skills, item)
	}
	sort.Slice(plan.Skills, func(i, j int) bool {
		return stableLess(plan.Skills[i].Name+"/"+plan.Skills[i].ID, plan.Skills[j].Name+"/"+plan.Skills[j].ID)
	})
	return plan, nil
}

func injectionSelectionSources(id string, explicit, inherited []string) []string {
	result := make([]string, 0, 2)
	if containsInjectionString(explicit, id) {
		result = append(result, model.InjectionSelectionEnvironment)
	}
	if containsInjectionString(inherited, id) {
		result = append(result, model.InjectionSelectionWorkspace)
	}
	return result
}

func containsInjectionString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func skillInjectionCapabilityState(state string) model.CapabilityState {
	switch state {
	case SkillAvailabilityAvailable:
		return model.CapabilityStateAvailable
	case SkillAvailabilityDisabled:
		return model.CapabilityStateDisabled
	case SkillAvailabilityUnconfigured:
		return model.CapabilityStateUnconfigured
	default:
		return model.CapabilityStateUnavailable
	}
}

// ApplyEnvironmentInjectionCapabilityReport refreshes passive/live capability
// states without changing selection provenance or catalog metadata.
func ApplyEnvironmentInjectionCapabilityReport(plan *model.EnvironmentInjectionPlan, report model.CapabilityReport) {
	if plan == nil {
		return
	}
	facts := make(map[string]model.CapabilityFact, len(report.Facts))
	for _, fact := range report.Facts {
		facts[fact.Key] = fact
	}
	plan.Summary.InjectableMCPs = 0
	plan.Summary.BlockedMCPs = 0
	for i := range plan.MCPs {
		item := &plan.MCPs[i]
		if fact, ok := facts["mcp/"+item.ID]; ok {
			item.State = fact.State
			item.ReasonCode = fact.ReasonCode
			item.Message = fact.Message
		}
		item.Injectable = item.State == model.CapabilityStateAvailable
		if item.Injectable {
			item.NextAction = injectionNextMCPTools
			plan.Summary.InjectableMCPs++
		} else {
			item.NextAction = injectionNextMCPInspect
			plan.Summary.BlockedMCPs++
		}
	}
	plan.Summary.InjectableSkills = 0
	plan.Summary.BlockedSkills = 0
	for i := range plan.Skills {
		item := &plan.Skills[i]
		if fact, ok := facts["skill/"+item.ID]; ok {
			item.State = fact.State
			item.ReasonCode = fact.ReasonCode
			item.Message = fact.Message
		}
		item.Injectable = item.State == model.CapabilityStateAvailable
		if item.Injectable {
			item.NextAction = injectionNextSkillRead
			plan.Summary.InjectableSkills++
		} else {
			item.NextAction = injectionNextSkillInspect
			plan.Summary.BlockedSkills++
		}
	}
}
