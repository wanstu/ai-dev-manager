package workspace

import (
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

	return config.Resolve(global, profile, &expandedProject, runtime)
}
