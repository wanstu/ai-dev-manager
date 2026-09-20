package desktop

import "ai-dev-manager-v2/internal/model"

type WorkspaceInput struct {
	Path string `json:"path"`
	Name string `json:"name,omitempty"`
}

type EnvironmentInput struct {
	WorkspaceID string `json:"workspace_id"`
	Name        string `json:"name"`
	Root        string `json:"root,omitempty"`
}

type MCPInput struct {
	Name           string                `json:"name"`
	Transport      string                `json:"transport"`
	AuthMode       string                `json:"auth_mode"`
	Endpoint       string                `json:"endpoint,omitempty"`
	HeaderRefs     map[string]string     `json:"header_refs,omitempty"`
	Executable     string                `json:"executable,omitempty"`
	Args           []string              `json:"args,omitempty"`
	EnvRefs        map[string]string     `json:"env_refs,omitempty"`
	HealthPolicy   model.MCPHealthPolicy `json:"health_policy"`
	DefaultInclude bool                  `json:"default_include_in_environment"`
}

type MCPImportInput struct {
	Format         string   `json:"format"`
	Content        string   `json:"json_or_jsonc"`
	SelectedNames  []string `json:"selected_names,omitempty"`
	ConflictPolicy string   `json:"conflict_policy,omitempty"`
	DefaultInclude bool     `json:"default_include,omitempty"`
	SourceScope    string   `json:"source_scope,omitempty"`
}

type SkillInput struct {
	Root           string `json:"root"`
	SupportRoot    string `json:"support_root,omitempty"`
	DefaultInclude bool   `json:"default_include_in_environment"`
}

type SkillSourceInput struct {
	Root           string   `json:"root"`
	SupportRoots   []string `json:"support_roots,omitempty"`
	DefaultInclude bool     `json:"default_include_in_environment"`
}

type ADMConnectionInput struct {
	BaseURL string `json:"base_url"`
	APIKey  string `json:"api_key,omitempty"`
}

type ADMConnectionStatus struct {
	State                  string `json:"state"`
	BaseURL                string `json:"base_url"`
	Listen                 string `json:"listen,omitempty"`
	HealthURL              string `json:"health_url"`
	AgentMCPURL            string `json:"agent_mcp_url"`
	AdminMCPURL            string `json:"admin_mcp_url"`
	PID                    int    `json:"pid,omitempty"`
	Version                string `json:"version,omitempty"`
	OwnerID                string `json:"owner_id,omitempty"`
	Detail                 string `json:"detail,omitempty"`
	LocalBootstrapEligible bool   `json:"local_bootstrap_eligible"`
}
