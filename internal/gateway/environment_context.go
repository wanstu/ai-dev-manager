package gateway

import (
	"context"
	"sort"

	"ai-dev-manager-v2/internal/app"
	"ai-dev-manager-v2/internal/model"
)

const gatewayAgentInstructions = "ADM routes development authority by stable Workspace and Environment IDs; there is no implicit current project. After choosing an Environment, use environment_injection_plan to understand selected MCP/Skill sources, passive injectability, and the next explicit action; use environment_context_bundle when a bounded root/tree/capability snapshot is also useful. Do not treat selected as usable when injectable=false. Optional capability failures are local facts and do not block ordinary file development. Mutations still require the matching Environment writer lease. These read-only summaries do not execute tasks or grant new authority."

func (o *runtimeOwner) InjectionPlan(ctx context.Context, environmentID string) (model.EnvironmentInjectionPlan, error) {
	plan, err := o.service.EnvironmentInjectionPlan(ctx, environmentID)
	if err != nil {
		return model.EnvironmentInjectionPlan{}, err
	}
	passive, err := o.service.EnvironmentCapabilityReportPassive(ctx, environmentID)
	if err != nil {
		return model.EnvironmentInjectionPlan{}, err
	}
	report := o.enrichCapabilityReport(environmentID, passive)
	app.ApplyEnvironmentInjectionCapabilityReport(&plan, report)
	if !report.GeneratedAt.IsZero() {
		plan.GeneratedAt = report.GeneratedAt
	}
	return plan, nil
}

func (o *runtimeOwner) ContextBundle(ctx context.Context, environmentID string, request model.EnvironmentContextRequest) (model.EnvironmentContextBundle, error) {
	bundle, err := o.service.EnvironmentContextBundle(ctx, environmentID, request)
	if err != nil {
		return model.EnvironmentContextBundle{}, err
	}

	passive, err := o.service.EnvironmentCapabilityReportPassive(ctx, environmentID)
	if err != nil {
		return model.EnvironmentContextBundle{}, err
	}
	report := o.enrichCapabilityReport(environmentID, passive)
	facts := make(map[string]model.CapabilityFact, len(report.Facts))
	for _, fact := range report.Facts {
		facts[fact.Key] = fact
	}

	for i := range bundle.MCPs {
		item := &bundle.MCPs[i]
		if fact, ok := facts["mcp/"+item.ID]; ok {
			item.State = fact.State
			item.ReasonCode = fact.ReasonCode
			item.Injectable = fact.State == model.CapabilityStateAvailable
			if item.Injectable {
				item.NextAction = "environment_mcp_tools"
			} else {
				item.NextAction = "environment_mcp_inspect"
			}
		}
		observation, observed := o.observation(runtimeOwnerKey{environmentID: environmentID, mcpID: item.ID})
		if !observed {
			item.ObservationState = "not_observed"
			item.ToolInventoryObserved = false
			item.ToolNames = []string{}
			item.ObservedAt = nil
			continue
		}
		item.ObservationState = string(observation.State)
		item.ToolInventoryObserved = observation.InventoryFetchedAt != nil
		item.ToolNames = []string{}
		if item.ToolInventoryObserved {
			for _, tool := range observation.ToolInventory {
				if tool.Name != "" {
					item.ToolNames = append(item.ToolNames, tool.Name)
				}
			}
			sort.Strings(item.ToolNames)
		}
		if observation.InventoryFetchedAt != nil {
			observedAt := observation.InventoryFetchedAt.UTC()
			item.ObservedAt = &observedAt
		} else if observedAt, ok := mcpObservedAt(observation, true, bundle.GeneratedAt); ok {
			item.ObservedAt = &observedAt
		}
	}

	// Core compaction may already have omitted capability/guidance entries. They
	// are fully re-projected from the enriched canonical report below, so reset
	// only those omission counters before applying the final byte budget again.
	bundle.Omissions.AvailableCapabilities = 0
	bundle.Omissions.CapabilityIssues = 0
	bundle.Omissions.Guidance = 0
	app.ApplyEnvironmentContextCapabilityReport(&bundle, report)
	if err := app.CompactEnvironmentContextBundle(&bundle); err != nil {
		return model.EnvironmentContextBundle{}, err
	}
	return bundle, nil
}
