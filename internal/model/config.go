package model

// Scope identifies the configuration layer that contributed a value.
type Scope string

const (
	ScopeGlobal  Scope = "global"
	ScopeProfile Scope = "profile"
	ScopeProject Scope = "project"
	ScopeRuntime Scope = "runtime"
)

// Workspace is the persistent identity needed by the control plane.
// Runtime state such as PID, port, or process handles deliberately does not
// belong here.
type Workspace struct {
	ID        string
	Path      string
	ProfileID string
	RuntimeID string
}

// MCPDefinition is an MCP configuration fragment. Empty string fields mean
// "not specified by this layer" during Phase 1 resolution. Enabled is
// tri-state: nil means inherit, true enables, and false disables.
type MCPDefinition struct {
	ID        string
	Enabled   *bool
	Transport string
	Command   string
	Args      []string
	URL       string
	Env       map[string]string
	EnvRefs   map[string]string
}

// SkillDefinition is a Skill configuration fragment.
type SkillDefinition struct {
	ID      string
	Enabled *bool
	Path    string
}

// VerifierDefinition is a structured project verification command. Enabled is
// tri-state so higher config layers can disable or re-enable an inherited verifier.
type VerifierDefinition struct {
	ID             string
	Kind           string
	Enabled        *bool
	Executable     string
	Args           []string
	Cwd            string
	TimeoutSeconds int
}

// Policy contains execution permissions and explicit local tool resolution.
type Policy struct {
	Mode               string
	AllowedExecutables []string
	ToolPaths          map[string]string
}

// ConfigLayer represents exactly one configuration scope.
type ConfigLayer struct {
	Scope      Scope
	MCPs       map[string]MCPDefinition
	Skills     map[string]SkillDefinition
	SkillRoots []string
	Verifiers  map[string]VerifierDefinition
	Policy     *Policy
}

// ResolvedMCP keeps the merged definition plus enough source information to
// explain both where its effective body came from and which layer most
// recently decided its enabled state.
type ResolvedMCP struct {
	MCPDefinition
	Source        Scope
	EnabledSource Scope
}

// ResolvedSkill is the Skill counterpart of ResolvedMCP.
type ResolvedSkill struct {
	SkillDefinition
	Source        Scope
	EnabledSource Scope
}

// ResolvedVerifier keeps the merged verifier plus source information.
type ResolvedVerifier struct {
	VerifierDefinition
	Source        Scope
	EnabledSource Scope
}

// ResolvedPolicy keeps the winning policy and its source.
type ResolvedPolicy struct {
	Policy Policy
	Source Scope
}

// EffectiveConfig is the only configuration shape later runtimes should
// consume. Resolution details stay in the control-plane side of the boundary.
type EffectiveConfig struct {
	MCPs      map[string]ResolvedMCP
	Skills    map[string]ResolvedSkill
	Verifiers map[string]ResolvedVerifier
	Policy    *ResolvedPolicy
}
