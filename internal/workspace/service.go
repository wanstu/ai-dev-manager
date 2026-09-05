package workspace

import (
	"strings"

	"ai-dev-manager/internal/config"
	"ai-dev-manager/internal/model"
	"ai-dev-manager/internal/skill"
)

// ConfigService orchestrates persisted workspace identity and configuration
// loading, then delegates merge semantics to the Phase 1 resolver.
type ConfigService struct {
	store    *config.Store
	registry *Registry
}

func NewConfigService(store *config.Store, registry *Registry) *ConfigService {
	return &ConfigService{store: store, registry: registry}
}

// ResolveWorkspace loads Global, optional Profile, optional Project and the
// caller-supplied in-memory Runtime Override, then returns the EffectiveConfig.
func (s *ConfigService) ResolveWorkspace(id string, runtimeOverride *model.ConfigLayer) (model.EffectiveConfig, error) {
	workspace, err := s.registry.Get(id)
	if err != nil {
		return model.EffectiveConfig{}, err
	}

	userConfig, err := s.store.LoadUserConfig()
	if err != nil {
		return model.EffectiveConfig{}, err
	}
	global, err := skill.ExpandLayer(userConfig.Global, s.store.Root())
	if err != nil {
		return model.EffectiveConfig{}, err
	}

	var profile *model.ConfigLayer
	if workspace.ProfileID != "" {
		loadedProfile, err := s.store.LoadProfile(workspace.ProfileID)
		if err != nil {
			return model.EffectiveConfig{}, err
		}
		expandedProfile, err := skill.ExpandLayer(loadedProfile, s.store.Root())
		if err != nil {
			return model.EffectiveConfig{}, err
		}
		profile = &expandedProfile
	}

	project, err := s.store.LoadProject(workspace.Path)
	if err != nil {
		return model.EffectiveConfig{}, err
	}
	expandedProject, err := skill.ExpandLayer(project, workspace.Path)
	if err != nil {
		return model.EffectiveConfig{}, err
	}

	var runtime *model.ConfigLayer
	if runtimeOverride != nil {
		expandedRuntime, err := skill.ExpandLayer(*runtimeOverride, workspace.Path)
		if err != nil {
			return model.EffectiveConfig{}, err
		}
		runtime = &expandedRuntime
	}

	if workspace.LocalPolicy != nil && (runtime == nil || runtime.Policy == nil) {
		base, err := config.Resolve(global, profile, &expandedProject, nil)
		if err != nil {
			return model.EffectiveConfig{}, err
		}
		localPolicy := overlayWorkspaceLocalPolicy(base.Policy, workspace.LocalPolicy)
		if runtime == nil {
			runtime = &model.ConfigLayer{Scope: model.ScopeRuntime}
		}
		runtime.Policy = localPolicy
	}

	return config.Resolve(global, profile, &expandedProject, runtime)
}

func overlayWorkspaceLocalPolicy(base *model.ResolvedPolicy, local *model.Policy) *model.Policy {
	result := &model.Policy{}
	if base != nil {
		result = clonePolicy(&base.Policy)
	}
	if localMode := strings.TrimSpace(local.Mode); localMode != "" {
		result.Mode = applyMinimumPolicyMode(result.Mode, localMode)
	}
	for _, executable := range local.AllowedExecutables {
		executable = strings.TrimSpace(executable)
		if executable == "" || containsExecutable(result.AllowedExecutables, executable) {
			continue
		}
		result.AllowedExecutables = append(result.AllowedExecutables, executable)
	}
	if local.ToolPaths != nil {
		if result.ToolPaths == nil {
			result.ToolPaths = make(map[string]string, len(local.ToolPaths))
		}
		for key, value := range local.ToolPaths {
			result.ToolPaths[key] = value
		}
	}
	return result
}

func applyMinimumPolicyMode(base, minimum string) string {
	base = strings.TrimSpace(base)
	minimum = strings.TrimSpace(minimum)
	baseRank, baseKnown := policyModeRank(base)
	minimumRank, minimumKnown := policyModeRank(minimum)
	if !minimumKnown {
		return minimum
	}
	if base != "" && !baseKnown {
		return base
	}
	if minimumRank > baseRank {
		return minimum
	}
	return base
}

func policyModeRank(mode string) (int, bool) {
	switch strings.TrimSpace(mode) {
	case "", "read-only":
		return 0, true
	case "workspace-write":
		return 1, true
	case "standard":
		return 2, true
	case "full":
		return 3, true
	default:
		return 0, false
	}
}

func containsExecutable(values []string, target string) bool {
	for _, value := range values {
		if strings.EqualFold(strings.TrimSpace(value), target) {
			return true
		}
	}
	return false
}
