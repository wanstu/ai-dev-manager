package model

import "time"

const (
	InjectionSelectionEnvironment = "environment"
	InjectionSelectionWorkspace   = "workspace"
)

type EnvironmentInjectionSummary struct {
	SelectedMCPs     int `json:"selected_mcps"`
	InjectableMCPs   int `json:"injectable_mcps"`
	BlockedMCPs      int `json:"blocked_mcps"`
	SelectedSkills   int `json:"selected_skills"`
	InjectableSkills int `json:"injectable_skills"`
	BlockedSkills    int `json:"blocked_skills"`
}

type EnvironmentInjectionMCP struct {
	ID                          string          `json:"id"`
	Name                        string          `json:"name,omitempty"`
	Transport                   string          `json:"transport,omitempty"`
	SelectionSources            []string        `json:"selection_sources"`
	DefaultIncludeInEnvironment bool            `json:"default_include_in_environment"`
	Selected                    bool            `json:"selected"`
	Injectable                  bool            `json:"injectable"`
	State                       CapabilityState `json:"state"`
	ReasonCode                  string          `json:"reason_code,omitempty"`
	Message                     string          `json:"message,omitempty"`
	NextAction                  string          `json:"next_action"`
}

type EnvironmentInjectionSkill struct {
	ID                          string          `json:"id"`
	Name                        string          `json:"name,omitempty"`
	SelectionSources            []string        `json:"selection_sources"`
	DefaultIncludeInEnvironment bool            `json:"default_include_in_environment"`
	Selected                    bool            `json:"selected"`
	Injectable                  bool            `json:"injectable"`
	State                       CapabilityState `json:"state"`
	ReasonCode                  string          `json:"reason_code,omitempty"`
	Message                     string          `json:"message,omitempty"`
	RelativeArtifactPath        string          `json:"relative_artifact_path,omitempty"`
	SupportRootCount            int             `json:"support_root_count"`
	MissingSupportRootCount     int             `json:"missing_support_root_count"`
	NextAction                  string          `json:"next_action"`
}

type EnvironmentInjectionPlan struct {
	EnvironmentID string                      `json:"environment_id"`
	WorkspaceID   string                      `json:"workspace_id"`
	GeneratedAt   time.Time                   `json:"generated_at"`
	MCPs          []EnvironmentInjectionMCP   `json:"mcps"`
	Skills        []EnvironmentInjectionSkill `json:"skills"`
	Summary       EnvironmentInjectionSummary `json:"summary"`
}
