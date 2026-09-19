package gateway

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"ai-dev-manager-v2/internal/app"
	"ai-dev-manager-v2/internal/catalog"
	"ai-dev-manager-v2/internal/isolation"
	"ai-dev-manager-v2/internal/management"
	"ai-dev-manager-v2/internal/model"
	productversion "ai-dev-manager-v2/internal/version"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	serverName         = "adm"
	runtimeOwnerHeader = "X-ADM-Runtime-Owner"
)

var serverVersion = productversion.Current()

type EmptyInput struct{}

type EnvironmentInput struct {
	EnvironmentID string `json:"environment_id"`
}

type EnvironmentRenameInput struct {
	EnvironmentID string `json:"environment_id"`
	Name          string `json:"name"`
}

type EnvironmentWorkspaceSetInput struct {
	EnvironmentID string `json:"environment_id"`
	WorkspaceID   string `json:"workspace_id"`
}

type WorkspaceAddInput struct {
	Path string `json:"path"`
	Name string `json:"name,omitempty"`
}

type WorkspaceInput struct {
	WorkspaceID string `json:"workspace_id"`
}

type WorkspaceDiscoveryInput struct {
	WorkspaceID      string `json:"workspace_id"`
	Path             string `json:"path,omitempty"`
	Query            string `json:"query,omitempty"`
	MaxDepth         int    `json:"max_depth,omitempty"`
	MaxEntries       int    `json:"max_entries,omitempty"`
	MaxCandidates    int    `json:"max_candidates,omitempty"`
	MaxDigestEntries int    `json:"max_digest_entries,omitempty"`
	MaxOutputBytes   int    `json:"max_output_bytes,omitempty"`
}

type EnvironmentTreeDigestInput struct {
	EnvironmentID    string `json:"environment_id"`
	Path             string `json:"path,omitempty"`
	MaxDepth         int    `json:"max_depth,omitempty"`
	MaxEntries       int    `json:"max_entries,omitempty"`
	MaxCandidates    int    `json:"max_candidates,omitempty"`
	MaxDigestEntries int    `json:"max_digest_entries,omitempty"`
	MaxOutputBytes   int    `json:"max_output_bytes,omitempty"`
}

type EnvironmentContextInput struct {
	EnvironmentID    string `json:"environment_id"`
	Path             string `json:"path,omitempty"`
	MaxDepth         int    `json:"max_depth,omitempty"`
	MaxEntries       int    `json:"max_entries,omitempty"`
	MaxDigestEntries int    `json:"max_digest_entries,omitempty"`
	MaxOutputBytes   int    `json:"max_output_bytes,omitempty"`
}

func discoveryRequest(path, query string, maxDepth, maxEntries, maxCandidates, maxDigestEntries, maxOutputBytes int) model.DiscoveryRequest {
	return model.DiscoveryRequest{
		Path: path, Query: query, MaxDepth: maxDepth, MaxEntries: maxEntries,
		MaxCandidates: maxCandidates, MaxDigestEntries: maxDigestEntries, MaxOutputBytes: maxOutputBytes,
	}
}

type WorkspaceRenameInput struct {
	WorkspaceID string `json:"workspace_id"`
	Name        string `json:"name"`
}

type WorktreeSettingsInput struct {
	Root         string `json:"root,omitempty" jsonschema:"managed worktree storage directory; empty resets to ADM default"`
	BranchPrefix string `json:"branch_prefix,omitempty" jsonschema:"managed worktree branch prefix; empty defaults to adm/"`
}

type ExecutableInput struct {
	Executable string `json:"executable"`
}

type ExecDenyClearInput struct {
	Executable string `json:"executable"`
}

type EnvironmentCreateInput struct {
	WorkspaceID string `json:"workspace_id"`
	Name        string `json:"name"`
	Root        string `json:"root,omitempty" jsonschema:"optional workspace-contained directory; defaults to workspace root"`
}

type TemporaryEnvironmentCreateInput struct {
	WorkspaceID               string `json:"workspace_id,omitempty" jsonschema:"Workspace source; required for existing_root and optional alternative to source_environment_id for managed_worktree"`
	SourceEnvironmentID       string `json:"source_environment_id,omitempty" jsonschema:"existing Git Environment source; only valid for managed_worktree and alternative to workspace_id"`
	Name                      string `json:"name"`
	OwnerID                   string `json:"owner_id" jsonschema:"stable lifecycle owner provenance; not an ADM task owner"`
	TTLSeconds                int64  `json:"ttl_seconds" jsonschema:"positive temporary retention TTL in seconds"`
	SessionID                 string `json:"session_id,omitempty" jsonschema:"optional lifecycle provenance only"`
	RunID                     string `json:"run_id,omitempty" jsonschema:"optional provenance only; the Run need not exist and is not started or parented"`
	Mode                      string `json:"mode,omitempty" jsonschema:"existing_root or managed_worktree; defaults to existing_root"`
	Root                      string `json:"root,omitempty" jsonschema:"existing workspace-contained directory; only valid for existing_root mode"`
	BranchName                string `json:"branch_name,omitempty" jsonschema:"required managed_worktree branch fragment supplied by the agent; ADM prepends configured branch_prefix"`
	BaseRef                   string `json:"base_ref,omitempty" jsonschema:"optional Git ref; managed_worktree defaults to the fetched upstream commit"`
	MigrateUncommittedChanges bool   `json:"migrate_uncommitted_changes,omitempty" jsonschema:"copy tracked/staged/unstaged/untracked non-ignored source changes into the new managed worktree; conflicts fail creation"`
}

type TemporaryEnvironmentOwnerInput struct {
	EnvironmentID string `json:"environment_id"`
	OwnerID       string `json:"owner_id" jsonschema:"matching temporary lifecycle owner provenance"`
}

type TemporaryEnvironmentCleanupExpiredInput struct {
	Execute bool `json:"execute,omitempty"`
}

type TemporaryEnvironmentCleanupInput struct {
	EnvironmentID string `json:"environment_id"`
	OwnerID       string `json:"owner_id" jsonschema:"matching temporary lifecycle owner required for execute"`
	Execute       bool   `json:"execute,omitempty" jsonschema:"false or omitted previews only; true executes after a fresh safety check"`
}

type EnvironmentWorktreeCreateInput struct {
	WorkspaceID               string `json:"workspace_id,omitempty" jsonschema:"Workspace Git root source; alternative to source_environment_id"`
	SourceEnvironmentID       string `json:"source_environment_id,omitempty" jsonschema:"existing Git Environment source; alternative to workspace_id"`
	Name                      string `json:"name"`
	BranchName                string `json:"branch_name" jsonschema:"required ASCII branch fragment such as fix/check-ref; ADM prepends configured branch_prefix"`
	BaseRef                   string `json:"base_ref,omitempty" jsonschema:"optional Git ref; defaults to the fetched upstream commit"`
	MigrateUncommittedChanges bool   `json:"migrate_uncommitted_changes,omitempty" jsonschema:"migrate tracked staged/unstaged/deleted/renamed and untracked non-ignored changes from the source checkout"`
}

type EnvironmentWorktreeDestroyInput struct {
	EnvironmentID string `json:"environment_id"`
	WriterOwner   string `json:"writer_owner"`
	Force         bool   `json:"force,omitempty"`
}

type WriterAcquireInput struct {
	EnvironmentID string `json:"environment_id"`
	Owner         string `json:"owner" jsonschema:"stable agent/session owner identifier"`
}

type MCPAddInput struct {
	Name           string                `json:"name"`
	Transport      string                `json:"transport"`
	AuthMode       string                `json:"auth_mode"`
	Endpoint       string                `json:"endpoint,omitempty"`
	HeaderRefs     map[string]string     `json:"header_refs,omitempty"`
	Executable     string                `json:"executable,omitempty"`
	Args           []string              `json:"args,omitempty"`
	EnvRefs        map[string]string     `json:"env_refs,omitempty"`
	HealthPolicy   model.MCPHealthPolicy `json:"health_policy"`
	DefaultInclude bool                  `json:"default_include_in_environment,omitempty"`
}

type MCPUpdateInput struct {
	ID             string                `json:"id"`
	Name           string                `json:"name"`
	Transport      string                `json:"transport"`
	AuthMode       string                `json:"auth_mode"`
	Endpoint       string                `json:"endpoint,omitempty"`
	HeaderRefs     map[string]string     `json:"header_refs,omitempty"`
	Executable     string                `json:"executable,omitempty"`
	Args           []string              `json:"args,omitempty"`
	EnvRefs        map[string]string     `json:"env_refs,omitempty"`
	HealthPolicy   model.MCPHealthPolicy `json:"health_policy"`
	DefaultInclude bool                  `json:"default_include_in_environment,omitempty"`
}

type CatalogAddInput struct {
	Name           string   `json:"name,omitempty"`
	Endpoint       string   `json:"endpoint,omitempty"`
	Instructions   string   `json:"instructions,omitempty"`
	Root           string   `json:"root,omitempty"`
	SupportRoots   []string `json:"support_roots,omitempty"`
	DefaultInclude bool     `json:"default_include_in_environment,omitempty"`
}

type CatalogIDInput struct {
	ID string `json:"id"`
}

type SkillSourceUpdateInput struct {
	ID             string   `json:"id"`
	Root           string   `json:"root"`
	SupportRoots   []string `json:"support_roots,omitempty"`
	DefaultInclude bool     `json:"default_include_in_environment,omitempty"`
}

type CatalogDefaultInput struct {
	ID             string `json:"id"`
	DefaultInclude bool   `json:"default_include_in_environment"`
}

type WorkspaceSelectionInput struct {
	WorkspaceID string `json:"workspace_id"`
	ID          string `json:"id"`
	Enabled     bool   `json:"enabled"`
}

type EnvironmentSelectionInput struct {
	EnvironmentID string `json:"environment_id"`
	ID            string `json:"id"`
	Enabled       bool   `json:"enabled"`
}

type EnvironmentMCPRuntimeInput struct {
	EnvironmentID string `json:"environment_id"`
	MCPID         string `json:"mcp_id"`
}

type EnvironmentMCPStatusInput struct {
	EnvironmentID string `json:"environment_id"`
	MCPID         string `json:"mcp_id"`
}

type EnvironmentMCPCallInput struct {
	EnvironmentID string         `json:"environment_id"`
	MCPID         string         `json:"mcp_id"`
	Tool          string         `json:"tool"`
	Arguments     map[string]any `json:"arguments,omitempty"`
}

type EnvironmentSkillReadInput struct {
	EnvironmentID string `json:"environment_id"`
	SkillID       string `json:"skill_id"`
	Path          string `json:"path,omitempty"`
	MaxBytes      int    `json:"max_bytes,omitempty"`
}

type EnvironmentSkillFilesInput struct {
	EnvironmentID string `json:"environment_id"`
	SkillID       string `json:"skill_id"`
	RootKind      string `json:"root_kind,omitempty" jsonschema:"artifact, support, or support:<index>; defaults to artifact"`
	MaxEntries    int    `json:"max_entries,omitempty"`
}

type EnvironmentVerifierRunInput struct {
	EnvironmentID  string `json:"environment_id"`
	WriterOwner    string `json:"writer_owner"`
	VerifierID     string `json:"verifier_id"`
	MaxOutputBytes int    `json:"max_output_bytes,omitempty"`
}

type EnvironmentVerifierRunStatusInput struct {
	EnvironmentID string `json:"environment_id"`
	VerifierRunID string `json:"verifier_run_id"`
}

type EnvironmentVerifierRunCancelInput struct {
	EnvironmentID string `json:"environment_id"`
	WriterOwner   string `json:"writer_owner"`
	VerifierRunID string `json:"verifier_run_id"`
}

type EnvironmentVerifierAddInput struct {
	EnvironmentID string                   `json:"environment_id"`
	Definition    model.VerifierDefinition `json:"definition"`
}

type EnvironmentVerifierRemoveInput struct {
	EnvironmentID string `json:"environment_id"`
	VerifierID    string `json:"verifier_id"`
}

type MemoryKeyInput struct {
	Key string `json:"key"`
}

type MemoryWriteInput struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type EnvironmentMemoryKeyInput struct {
	EnvironmentID string `json:"environment_id"`
	Key           string `json:"key"`
}

type EnvironmentMemoryWriteInput struct {
	EnvironmentID string `json:"environment_id"`
	Key           string `json:"key"`
	Value         string `json:"value"`
}

type WriterReleaseInput struct {
	EnvironmentID string `json:"environment_id"`
	Owner         string `json:"owner,omitempty"`
	Force         bool   `json:"force,omitempty"`
}

type TreeInput struct {
	EnvironmentID string `json:"environment_id"`
	Path          string `json:"path,omitempty"`
	MaxDepth      int    `json:"max_depth,omitempty"`
	MaxEntries    int    `json:"max_entries,omitempty"`
}

type ReadInput struct {
	EnvironmentID string `json:"environment_id"`
	Path          string `json:"path"`
	MaxBytes      int    `json:"max_bytes,omitempty"`
}

type SearchInput struct {
	EnvironmentID   string `json:"environment_id"`
	Path            string `json:"path,omitempty"`
	Query           string `json:"query"`
	MaxFiles        int    `json:"max_files,omitempty"`
	MaxMatches      int    `json:"max_matches,omitempty"`
	MaxBytesPerFile int    `json:"max_bytes_per_file,omitempty"`
}

type EndpointInvestigateInput struct {
	EnvironmentID   string `json:"environment_id"`
	Target          string `json:"target" jsonschema:"URL or path to investigate, for example /api/users/123"`
	Method          string `json:"method,omitempty" jsonschema:"optional HTTP method such as GET or POST"`
	Path            string `json:"path,omitempty" jsonschema:"optional environment-relative search root"`
	MaxFiles        int    `json:"max_files,omitempty"`
	MaxMatches      int    `json:"max_matches,omitempty"`
	MaxBytesPerFile int    `json:"max_bytes_per_file,omitempty"`
}

type WriteInput struct {
	EnvironmentID string `json:"environment_id"`
	WriterOwner   string `json:"writer_owner"`
	Path          string `json:"path"`
	Content       string `json:"content"`
	CreateParents bool   `json:"create_parents,omitempty"`
}

type EditInput struct {
	EnvironmentID        string `json:"environment_id"`
	WriterOwner          string `json:"writer_owner"`
	Path                 string `json:"path"`
	OldText              string `json:"old_text"`
	NewText              string `json:"new_text"`
	ExpectedReplacements int    `json:"expected_replacements,omitempty"`
}

type DeleteInput struct {
	EnvironmentID string `json:"environment_id"`
	WriterOwner   string `json:"writer_owner"`
	Path          string `json:"path"`
}

type ExecInput struct {
	EnvironmentID  string   `json:"environment_id"`
	WriterOwner    string   `json:"writer_owner"`
	Executable     string   `json:"executable"`
	Args           []string `json:"args,omitempty"`
	Cwd            string   `json:"cwd,omitempty"`
	TimeoutMS      int64    `json:"timeout_ms,omitempty"`
	MaxOutputBytes int      `json:"max_output_bytes,omitempty"`
}

type ProcessStartInput struct {
	EnvironmentID string   `json:"environment_id"`
	WriterOwner   string   `json:"writer_owner"`
	Executable    string   `json:"executable"`
	Args          []string `json:"args,omitempty"`
	Cwd           string   `json:"cwd,omitempty"`
	MaxLogBytes   int      `json:"max_log_bytes,omitempty"`
}

type ProcessInput struct {
	EnvironmentID string `json:"environment_id"`
	ProcessID     string `json:"process_id"`
}

type ProcessStopInput struct {
	EnvironmentID string `json:"environment_id"`
	WriterOwner   string `json:"writer_owner"`
	ProcessID     string `json:"process_id"`
}

type RunStartInput struct {
	EnvironmentID  string   `json:"environment_id"`
	WriterOwner    string   `json:"writer_owner"`
	Executable     string   `json:"executable"`
	Args           []string `json:"args,omitempty"`
	Cwd            string   `json:"cwd,omitempty"`
	TimeoutMS      int64    `json:"timeout_ms,omitempty"`
	MaxOutputBytes int      `json:"max_output_bytes,omitempty"`
}

type RunInput struct {
	EnvironmentID string `json:"environment_id"`
	RunID         string `json:"run_id"`
}

type RunCancelInput struct {
	EnvironmentID string `json:"environment_id"`
	WriterOwner   string `json:"writer_owner"`
	RunID         string `json:"run_id"`
}

type EnvironmentInfoOutput = app.EnvironmentInspection
type EnvironmentCapabilityReportOutput = model.CapabilityReport
type InvestigationProviderReportOutput = model.InvestigationProviderReport
type ResourceRetentionReportOutput = model.ResourceRetentionReport
type ResourceRetentionCleanupOutput = model.ResourceRetentionCleanupResult
type ResourceRetentionUpdateOutput = model.ResourceRetentionUpdateResult

type serverSurface string

const (
	serverSurfaceAgent serverSurface = "agent"
	serverSurfaceAdmin serverSurface = "admin"
)

func New(service *app.Service) *mcp.Server {
	return newServer(service, nil)
}

func NewAdmin(service *app.Service) *mcp.Server {
	return newServerForSurface(service, nil, serverSurfaceAdmin)
}

func newServer(service *app.Service, owner *runtimeOwner) *mcp.Server {
	return newServerForSurface(service, owner, serverSurfaceAgent)
}

func addScopedTool[In, Out any](server *mcp.Server, surface serverSurface, tool *mcp.Tool, handler mcp.ToolHandlerFor[In, Out]) {
	if surface == serverSurfaceAgent && isAdminOnlyTool(tool.Name) {
		return
	}
	mcp.AddTool(server, tool, handler)
}

func isAdminOnlyTool(name string) bool {
	switch name {
	case "management_snapshot", "worktree_settings_get", "worktree_settings_set", "host_environment_status", "host_environment_refresh",
		"workspace_add", "workspace_rename", "workspace_remove", "workspace_mcp_set", "workspace_skill_set",
		"environment_create", "environment_rename", "environment_workspace_options", "environment_workspace_recommendations", "environment_workspace_set", "environment_remove", "environment_verifier_add", "environment_verifier_remove", "environment_temporary_cleanup_expired",
		"exec_allow", "exec_allow_remove", "exec_deny_list", "exec_deny_clear", "exec_deny_clear_all", "exec_authorization_status", "exec_full_authorization_set",
		"logging_status", "gateway_access_status", "gateway_allowed_hosts_set", "gateway_admin_api_key_set", "gateway_admin_api_key_clear", "gateway_agent_api_key_set", "gateway_agent_api_key_clear",
		"mcp_list", "mcp_add", "mcp_update", "mcp_remove", "mcp_set_default", "mcp_probe", "mcp_import_preview", "mcp_import_apply",
		"environment_mcp_set",
		"skill_list", "skill_add", "skill_remove", "skill_set_default", "skill_availability_list", "skill_source_list", "skill_source_add", "skill_source_update", "skill_source_refresh", "skill_source_remove",
		"environment_skill_set",
		"resource_retention_inspect", "resource_retention_cleanup", "resource_retention_mark_temporary", "resource_retention_promote",
		"memory_global_write", "memory_global_delete":
		return true
	default:
		return false
	}
}

func newServerForSurface(service *app.Service, owner *runtimeOwner, surface serverSurface) *mcp.Server {
	implementationName := serverName
	if surface == serverSurfaceAdmin {
		implementationName += "-admin"
	}
	server := mcp.NewServer(&mcp.Implementation{Name: implementationName, Version: serverVersion}, &mcp.ServerOptions{Instructions: gatewayAgentInstructions})

	addScopedTool(server, surface, &mcp.Tool{Name: "gateway_info", Description: "Describe the ADM V2 Agent Gateway and its core semantics."},
		func(context.Context, *mcp.CallToolRequest, EmptyInput) (*mcp.CallToolResult, any, error) {
			role := "local AI development gateway"
			if surface == serverSurfaceAdmin {
				role = "local ADM management gateway"
			}
			info := map[string]any{
				"name":        implementationName,
				"api_version": "v2-dev",
				"surface":     string(surface),
				"role":        role,
				"notes": []string{
					"Workspace is a registered local directory; Git is optional.",
					"Environment is a persistent development context; worktree is not a prerequisite.",
					"MCP/Skill catalogs and Memory are optional development-context capabilities.",
				},
			}
			if owner != nil {
				info["runtime_owner"] = owner.Info()
			}
			return toolResult(info, nil)
		})

	registerGlobalTools(server, surface, service)

	addScopedTool(server, surface, &mcp.Tool{Name: "management_snapshot", Description: "Return a safe Desktop/CLI management overview without exposing Global or Environment-private Memory values."},
		func(context.Context, *mcp.CallToolRequest, EmptyInput) (*mcp.CallToolResult, management.Snapshot, error) {
			snapshot, err := management.New(service).Snapshot()
			return nil, snapshot, err
		})

	addScopedTool(server, surface, &mcp.Tool{Name: "logging_status", Description: "Return persistent ADM runtime log directory and fixed rotation/retention limits. No log contents or secrets are returned."},
		func(context.Context, *mcp.CallToolRequest, EmptyInput) (*mcp.CallToolResult, any, error) {
			return toolResult(service.LoggingStatus(), nil)
		})
	addScopedTool(server, surface, &mcp.Tool{Name: "gateway_access_status", Description: "Return configured remote Host allowlist and whether separate Admin MCP and Agent MCP API keys are configured; key values are never returned."},
		func(context.Context, *mcp.CallToolRequest, EmptyInput) (*mcp.CallToolResult, any, error) {
			status, err := service.GatewayAccessStatus()
			return toolResult(status, err)
		})
	addScopedTool(server, surface, &mcp.Tool{Name: "gateway_allowed_hosts_set", Description: "Replace the remote ADM Host/IP allowlist. Entries are DNS names or IP addresses without scheme or port; '*' allows any Host/IP while API-key authentication remains mandatory."},
		func(_ context.Context, _ *mcp.CallToolRequest, in GatewayAllowedHostsInput) (*mcp.CallToolResult, any, error) {
			status, err := service.SetGatewayAllowedHosts(in.Hosts)
			return toolResult(status, err)
		})
	addScopedTool(server, surface, &mcp.Tool{Name: "gateway_admin_api_key_set", Description: "Set the API key accepted only by /admin/mcp. The key is stored only as a SHA-256 hash and is never returned."},
		func(_ context.Context, _ *mcp.CallToolRequest, in GatewayAPIKeyInput) (*mcp.CallToolResult, any, error) {
			status, err := service.SetGatewayAdminAPIKey(in.APIKey)
			return toolResult(status, err)
		})
	addScopedTool(server, surface, &mcp.Tool{Name: "gateway_admin_api_key_clear", Description: "Clear the /admin/mcp API key. Remote Gateway listening remains refused until both Admin and Agent keys are configured."},
		func(context.Context, *mcp.CallToolRequest, EmptyInput) (*mcp.CallToolResult, any, error) {
			status, err := service.ClearGatewayAdminAPIKey()
			return toolResult(status, err)
		})
	addScopedTool(server, surface, &mcp.Tool{Name: "gateway_agent_api_key_set", Description: "Set the API key accepted only by /mcp. The key is stored only as a SHA-256 hash and is never returned."},
		func(_ context.Context, _ *mcp.CallToolRequest, in GatewayAPIKeyInput) (*mcp.CallToolResult, any, error) {
			status, err := service.SetGatewayAgentAPIKey(in.APIKey)
			return toolResult(status, err)
		})
	addScopedTool(server, surface, &mcp.Tool{Name: "gateway_agent_api_key_clear", Description: "Clear the /mcp API key. Remote Gateway listening remains refused until both Admin and Agent keys are configured."},
		func(context.Context, *mcp.CallToolRequest, EmptyInput) (*mcp.CallToolResult, any, error) {
			status, err := service.ClearGatewayAgentAPIKey()
			return toolResult(status, err)
		})
	addScopedTool(server, surface, &mcp.Tool{Name: "exec_authorization_status", Description: "Return whether command execution is in strict allowlist mode or explicit full-authorization mode."},
		func(context.Context, *mcp.CallToolRequest, EmptyInput) (*mcp.CallToolResult, any, error) {
			status, err := service.ExecAuthorizationStatus()
			return toolResult(status, err)
		})
	addScopedTool(server, surface, &mcp.Tool{Name: "exec_full_authorization_set", Description: "Enable or disable full command authorization. When enabled, unlisted executables may run but are still recorded as authorization-bypass observations."},
		func(_ context.Context, _ *mcp.CallToolRequest, in ExecFullAuthorizationInput) (*mcp.CallToolResult, any, error) {
			status, err := service.SetExecFullAuthorization(in.Enabled)
			return toolResult(status, err)
		})

	addScopedTool(server, surface, &mcp.Tool{Name: "worktree_settings_get", Description: "Return managed worktree storage root, branch prefix, and the system-managed Workspace ID. No project files are read."},
		func(context.Context, *mcp.CallToolRequest, EmptyInput) (*mcp.CallToolResult, model.WorktreeSettings, error) {
			settings, err := service.WorktreeSettings()
			return nil, settings, err
		})
	addScopedTool(server, surface, &mcp.Tool{Name: "worktree_settings_set", Description: "Update managed worktree storage root and branch prefix. The root is registered as a protected system Workspace; changing root is refused while managed worktrees exist."},
		func(_ context.Context, _ *mcp.CallToolRequest, in WorktreeSettingsInput) (*mcp.CallToolResult, model.WorktreeSettings, error) {
			settings, err := service.UpdateWorktreeSettings(in.Root, in.BranchPrefix)
			return nil, settings, err
		})

	addScopedTool(server, surface, &mcp.Tool{Name: "host_environment_status", Description: "Return safe metadata for ADM's host-environment snapshot. Variable values are never returned."},
		func(context.Context, *mcp.CallToolRequest, EmptyInput) (*mcp.CallToolResult, any, error) {
			status, err := service.HostEnvironmentStatus()
			return toolResult(status, err)
		})
	addScopedTool(server, surface, &mcp.Tool{Name: "host_environment_refresh", Description: "Explicitly refresh ADM's host-environment snapshot. On Windows this re-reads machine/user environment registry values; existing MCP sessions are discarded and future MCP activations/process starts use the refreshed snapshot. Already-running OS processes are unchanged."},
		func(context.Context, *mcp.CallToolRequest, EmptyInput) (*mcp.CallToolResult, any, error) {
			status, err := service.RefreshHostEnvironment()
			if err == nil && owner != nil {
				owner.DropAllMCP()
			}
			return toolResult(status, err)
		})

	addScopedTool(server, surface, &mcp.Tool{Name: "resource_retention_inspect", Description: "Inspect durable/temporary retention metadata and conservative cleanup eligibility. Uses persisted state plus existing Gateway-owner activity only; it does not delete resources, probe Git worktrees, start processes, or call MCP tools."},
		func(ctx context.Context, _ *mcp.CallToolRequest, _ EmptyInput) (*mcp.CallToolResult, ResourceRetentionReportOutput, error) {
			var (
				report model.ResourceRetentionReport
				err    error
			)
			if owner != nil {
				report, err = owner.ResourceRetentionReport(ctx)
			} else {
				report, err = service.ResourceRetentionReport(ctx)
			}
			if err != nil {
				return nil, ResourceRetentionReportOutput{}, err
			}
			return nil, report, nil
		})

	addScopedTool(server, surface, &mcp.Tool{Name: "resource_retention_cleanup", Description: "Return a cleanup plan for eligible temporary resources and execute only safe state-only cleanup when requested. It never deletes project directories, probes Git worktrees, starts processes, or calls MCP tools."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in model.ResourceRetentionCleanupRequest) (*mcp.CallToolResult, ResourceRetentionCleanupOutput, error) {
			var (
				result model.ResourceRetentionCleanupResult
				err    error
			)
			if owner != nil {
				result, err = owner.ResourceRetentionCleanup(ctx, in)
			} else {
				result, err = service.ResourceRetentionCleanup(ctx, in)
			}
			if err != nil {
				return nil, ResourceRetentionCleanupOutput{}, err
			}
			return nil, result, nil
		})

	addScopedTool(server, surface, &mcp.Tool{Name: "resource_retention_mark_temporary", Description: "Explicitly mark one Environment, MCP, Skill Source, or standalone Skill as temporary by updating retention metadata only. Source-owned Skills must be changed through their Skill Source."},
		func(_ context.Context, _ *mcp.CallToolRequest, in model.ResourceRetentionUpdateRequest) (*mcp.CallToolResult, ResourceRetentionUpdateOutput, error) {
			result, err := service.MarkResourceTemporary(in)
			if err != nil {
				return nil, ResourceRetentionUpdateOutput{}, err
			}
			return nil, result, nil
		})

	addScopedTool(server, surface, &mcp.Tool{Name: "resource_retention_promote", Description: "Promote one temporary Environment, MCP, Skill Source, or standalone Skill to durable retention metadata only. Skill Source promotion also updates source-owned Skills."},
		func(_ context.Context, _ *mcp.CallToolRequest, in model.ResourceRetentionUpdateRequest) (*mcp.CallToolResult, ResourceRetentionUpdateOutput, error) {
			result, err := service.PromoteResourceRetention(in)
			if err != nil {
				return nil, ResourceRetentionUpdateOutput{}, err
			}
			return nil, result, nil
		})

	addScopedTool(server, surface, &mcp.Tool{Name: "workspace_list", Description: "List local directories explicitly registered as ADM Workspaces."},
		func(context.Context, *mcp.CallToolRequest, EmptyInput) (*mcp.CallToolResult, any, error) {
			items, err := service.Workspaces.List()
			return toolResult(items, err)
		})

	addScopedTool(server, surface, &mcp.Tool{Name: "workspace_add", Description: "Register an existing local directory as an ADM Workspace. Git is not required."},
		func(_ context.Context, _ *mcp.CallToolRequest, in WorkspaceAddInput) (*mcp.CallToolResult, any, error) {
			item, err := service.Workspaces.Add(in.Path, in.Name)
			return toolResult(item, err)
		})

	addScopedTool(server, surface, &mcp.Tool{Name: "workspace_inspect", Description: "Inspect one registered ADM Workspace by stable ID."},
		func(_ context.Context, _ *mcp.CallToolRequest, in WorkspaceInput) (*mcp.CallToolResult, any, error) {
			item, err := service.Workspaces.Get(in.WorkspaceID)
			return toolResult(item, err)
		})

	addScopedTool(server, surface, &mcp.Tool{Name: "workspace_discover", Description: "Explicitly scan bounded directory metadata under one registered Workspace and return project candidates plus a compact directory digest. No writer, Git, executable, MCP, Skill, verifier, file-content read, or Environment creation is required."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in WorkspaceDiscoveryInput) (*mcp.CallToolResult, model.DiscoveryReport, error) {
			report, err := service.DiscoverWorkspace(ctx, in.WorkspaceID, discoveryRequest(in.Path, in.Query, in.MaxDepth, in.MaxEntries, in.MaxCandidates, in.MaxDigestEntries, in.MaxOutputBytes))
			if err != nil {
				return nil, model.DiscoveryReport{}, err
			}
			return nil, report, nil
		})

	addScopedTool(server, surface, &mcp.Tool{Name: "workspace_rename", Description: "Change one Workspace display name without moving or renaming its directory."},
		func(_ context.Context, _ *mcp.CallToolRequest, in WorkspaceRenameInput) (*mcp.CallToolResult, any, error) {
			item, err := service.Workspaces.Rename(in.WorkspaceID, in.Name)
			return toolResult(item, err)
		})

	addScopedTool(server, surface, &mcp.Tool{Name: "workspace_remove", Description: "Remove one ADM Workspace record without deleting project files. Existing Environment references block removal."},
		func(_ context.Context, _ *mcp.CallToolRequest, in WorkspaceInput) (*mcp.CallToolResult, any, error) {
			item, err := service.Workspaces.Remove(in.WorkspaceID)
			return toolResult(map[string]any{"removed": item}, err)
		})

	addScopedTool(server, surface, &mcp.Tool{Name: "exec_allow", Description: "Allow one executable for Environment command execution."},
		func(_ context.Context, _ *mcp.CallToolRequest, in ExecutableInput) (*mcp.CallToolResult, any, error) {
			err := service.AllowExecutable(in.Executable)
			if err != nil {
				return toolResult(nil, err)
			}
			items, err := service.AllowedExecutables()
			return toolResult(items, err)
		})

	addScopedTool(server, surface, &mcp.Tool{Name: "exec_allow_remove", Description: "Remove one executable from the Environment command execution allowlist."},
		func(_ context.Context, _ *mcp.CallToolRequest, in ExecutableInput) (*mcp.CallToolResult, any, error) {
			err := service.RemoveAllowedExecutable(in.Executable)
			if err != nil {
				return toolResult(nil, err)
			}
			items, err := service.AllowedExecutables()
			return toolResult(items, err)
		})

	addScopedTool(server, surface, &mcp.Tool{Name: "exec_allow_list", Description: "List executables allowed for Environment command execution."},
		func(context.Context, *mcp.CallToolRequest, EmptyInput) (*mcp.CallToolResult, any, error) {
			items, err := service.AllowedExecutables()
			return toolResult(items, err)
		})

	addScopedTool(server, surface, &mcp.Tool{Name: "exec_deny_list", Description: "List executable names recently blocked by the Runtime allowlist, sorted by denial count."},
		func(context.Context, *mcp.CallToolRequest, EmptyInput) (*mcp.CallToolResult, any, error) {
			items, err := service.ExecDenials()
			return toolResult(items, err)
		})

	addScopedTool(server, surface, &mcp.Tool{Name: "exec_deny_clear", Description: "Clear one blocked executable observation without changing the allowlist."},
		func(_ context.Context, _ *mcp.CallToolRequest, in ExecDenyClearInput) (*mcp.CallToolResult, any, error) {
			items, err := service.ClearExecDenial(in.Executable)
			return toolResult(items, err)
		})

	addScopedTool(server, surface, &mcp.Tool{Name: "exec_deny_clear_all", Description: "Clear all blocked executable observations without changing the allowlist."},
		func(context.Context, *mcp.CallToolRequest, EmptyInput) (*mcp.CallToolResult, any, error) {
			err := service.ClearExecDenials()
			return toolResult(map[string]any{"cleared": err == nil}, err)
		})

	addScopedTool(server, surface, &mcp.Tool{Name: "environment_list", Description: "List lightweight Environment summaries. Private Memory values are omitted; only the entry count is exposed."},
		func(context.Context, *mcp.CallToolRequest, EmptyInput) (*mcp.CallToolResult, any, error) {
			items, err := service.EnvironmentSummaries()
			return toolResult(items, err)
		})

	addScopedTool(server, surface, &mcp.Tool{Name: "environment_create", Description: "Create a development Environment for a registered Workspace. Git, worktree and verifier are not prerequisites."},
		func(_ context.Context, _ *mcp.CallToolRequest, in EnvironmentCreateInput) (*mcp.CallToolResult, any, error) {
			env, err := service.Environments.Create(in.WorkspaceID, in.Name, in.Root)
			return toolResult(env, err)
		})

	addScopedTool(server, surface, &mcp.Tool{Name: "environment_temporary_create", Description: "Create one fresh temporary Environment with explicit lifecycle owner and positive TTL. owner_id/session_id/run_id are retention provenance only, not ADM task orchestration. existing_root mode uses a registered Workspace. managed_worktree mode also requires branch_name, applies the configured ADM branch prefix/root, optionally migrates non-ignored uncommitted source changes, and never pulls/resets/reuses the source checkout."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in TemporaryEnvironmentCreateInput) (*mcp.CallToolResult, any, error) {
			value, err := service.CreateTemporaryEnvironment(ctx, string(surface), model.TemporaryEnvironmentCreateRequest{
				WorkspaceID:               in.WorkspaceID,
				SourceEnvironmentID:       in.SourceEnvironmentID,
				Name:                      in.Name,
				OwnerID:                   in.OwnerID,
				TTLSeconds:                in.TTLSeconds,
				SessionID:                 in.SessionID,
				RunID:                     in.RunID,
				Mode:                      in.Mode,
				Root:                      in.Root,
				BranchName:                in.BranchName,
				BaseRef:                   in.BaseRef,
				MigrateUncommittedChanges: in.MigrateUncommittedChanges,
			})
			return toolResult(value, err)
		})

	addScopedTool(server, surface, &mcp.Tool{Name: "environment_temporary_status", Description: "Inspect one temporary Environment lifecycle by stable environment_id, including retention provenance and current cleanup blockers. Read-only: it does not acquire or renew a writer and never returns Environment-private Memory values."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in EnvironmentInput) (*mcp.CallToolResult, any, error) {
			if owner != nil {
				value, err := owner.TemporaryEnvironmentStatus(ctx, in.EnvironmentID)
				return toolResult(value, err)
			}
			value, err := service.TemporaryEnvironmentStatus(ctx, in.EnvironmentID)
			return toolResult(value, err)
		})

	addScopedTool(server, surface, &mcp.Tool{Name: "environment_temporary_promote", Description: "Promote one temporary Environment to durable retention without changing its stable Environment/root/context. Requires the matching lifecycle owner_id; owner metadata is lifecycle provenance, not task orchestration."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in TemporaryEnvironmentOwnerInput) (*mcp.CallToolResult, any, error) {
			if owner != nil {
				value, err := owner.PromoteTemporaryEnvironment(ctx, in.EnvironmentID, in.OwnerID)
				return toolResult(value, err)
			}
			value, err := service.PromoteTemporaryEnvironment(in.EnvironmentID, in.OwnerID)
			return toolResult(value, err)
		})

	addScopedTool(server, surface, &mcp.Tool{Name: "environment_temporary_cleanup", Description: "Preview or execute safe cleanup for exactly one temporary Environment. execute=false/omitted is read-only preview; execute=true requires the matching lifecycle owner and a fresh runtime/Git safety check. There is no force path and unrelated resources are never swept."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in TemporaryEnvironmentCleanupInput) (*mcp.CallToolResult, any, error) {
			if owner == nil {
				return toolResult(nil, fmt.Errorf("persistent runtime owner is unavailable"))
			}
			value, err := owner.TemporaryEnvironmentCleanup(ctx, in.EnvironmentID, in.OwnerID, in.Execute)
			return toolResult(value, err)
		})

	addScopedTool(server, surface, &mcp.Tool{Name: "environment_temporary_cleanup_expired", Description: "Preview or execute cleanup of all expired temporary Environments only. Runtime blockers and managed-worktree dirty/unpublished safety checks remain enforced."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in TemporaryEnvironmentCleanupExpiredInput) (*mcp.CallToolResult, any, error) {
			if owner == nil {
				return toolResult(nil, fmt.Errorf("persistent runtime owner is unavailable"))
			}
			value, err := owner.ExpiredTemporaryEnvironmentsCleanup(ctx, in.Execute)
			return toolResult(value, err)
		})

	addScopedTool(server, surface, &mcp.Tool{Name: "environment_worktree_create", Description: "Create a fresh ADM-managed Git worktree Environment under the configured worktree root. The agent must provide branch_name; ADM prepends the configured branch_prefix, validates the final ASCII Git ref, resolves base_ref to an exact commit, and never reuses/resets an old managed worktree. migrate_uncommitted_changes optionally carries tracked staged/unstaged/deleted/renamed plus untracked non-ignored source changes; conflicts fail and roll back creation."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in EnvironmentWorktreeCreateInput) (*mcp.CallToolResult, any, error) {
			workspaceID := strings.TrimSpace(in.WorkspaceID)
			sourceEnvironmentID := strings.TrimSpace(in.SourceEnvironmentID)
			if (workspaceID == "") == (sourceEnvironmentID == "") {
				return toolResult(isolation.CreateResult{}, fmt.Errorf("environment_worktree_create requires exactly one of workspace_id or source_environment_id"))
			}
			options := isolation.CreateOptions{
				BranchName:                in.BranchName,
				BaseRef:                   in.BaseRef,
				MigrateUncommittedChanges: in.MigrateUncommittedChanges,
			}
			if sourceEnvironmentID != "" {
				value, err := service.CreateManagedWorktreeFromEnvironment(ctx, sourceEnvironmentID, in.Name, options)
				return toolResult(value, err)
			}
			value, err := service.CreateManagedWorktree(ctx, workspaceID, in.Name, options)
			return toolResult(value, err)
		})

	addScopedTool(server, surface, &mcp.Tool{Name: "environment_worktree_list", Description: "List persisted ADM-managed Git worktree records. Ordinary Environments are not included."},
		func(context.Context, *mcp.CallToolRequest, EmptyInput) (*mcp.CallToolResult, any, error) {
			items, err := service.ManagedWorktrees()
			return toolResult(items, err)
		})

	addScopedTool(server, surface, &mcp.Tool{Name: "environment_worktree_destroy", Description: "Destroy one ADM-managed Git worktree Environment. Requires the matching writer_owner. Dirty or unpublished work is refused unless force=true; the managed branch is always retained."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in EnvironmentWorktreeDestroyInput) (*mcp.CallToolResult, any, error) {
			value, err := service.DestroyManagedWorktree(ctx, in.EnvironmentID, in.WriterOwner, in.Force)
			if err == nil && owner != nil {
				owner.DropEnvironment(in.EnvironmentID)
			}
			return toolResult(value, err)
		})

	addScopedTool(server, surface, &mcp.Tool{Name: "environment_inspect", Description: "Inspect Workspace relation, structured capability facts, resolved/unresolved MCP and Skill selections, and private Memory entry count without exposing Memory values."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in EnvironmentInput) (*mcp.CallToolResult, EnvironmentInfoOutput, error) {
			info, err := service.InspectEnvironment(ctx, in.EnvironmentID)
			if err != nil {
				return nil, EnvironmentInfoOutput{}, err
			}
			return nil, info, nil
		})

	addScopedTool(server, surface, &mcp.Tool{Name: "environment_tree_digest", Description: "Explicitly scan bounded directory metadata under one Runtime-authorized Environment root and return a compact directory digest. It never expands authority to Workspace siblings and requires no writer or optional capability."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in EnvironmentTreeDigestInput) (*mcp.CallToolResult, model.DiscoveryReport, error) {
			report, err := service.EnvironmentTreeDigest(ctx, in.EnvironmentID, discoveryRequest(in.Path, "", in.MaxDepth, in.MaxEntries, in.MaxCandidates, in.MaxDigestEntries, in.MaxOutputBytes))
			if err != nil {
				return nil, model.DiscoveryReport{}, err
			}
			return nil, report, nil
		})

	addScopedTool(server, surface, &mcp.Tool{Name: "environment_injection_plan", Description: "Return the canonical passive MCP/Skill injection plan for one explicit Environment: selection provenance, default catalog flag, injectability, reason, and the next explicit Agent action. It does not connect MCPs, read Skill contents, acquire a writer, or expand authority."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in EnvironmentInput) (*mcp.CallToolResult, model.EnvironmentInjectionPlan, error) {
			var (
				plan model.EnvironmentInjectionPlan
				err  error
			)
			if owner != nil {
				plan, err = owner.InjectionPlan(ctx, in.EnvironmentID)
			} else {
				plan, err = service.EnvironmentInjectionPlan(ctx, in.EnvironmentID)
			}
			if err != nil {
				return nil, model.EnvironmentInjectionPlan{}, err
			}
			return nil, plan, nil
		})

	addScopedTool(server, surface, &mcp.Tool{Name: "environment_context_bundle", Description: "Return one explicit bounded read-only Environment context snapshot: root/tree, capabilities, selected MCP/Skill injection summaries, verifier summaries, and factual operation guidance. MCP/Skill fields reuse the canonical injection-plan semantics. It does not probe/connect MCPs, execute Git/verifiers/Runs/processes, read Memory values, or grant new authority."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in EnvironmentContextInput) (*mcp.CallToolResult, model.EnvironmentContextBundle, error) {
			request := model.EnvironmentContextRequest{Path: in.Path, MaxDepth: in.MaxDepth, MaxEntries: in.MaxEntries, MaxDigestEntries: in.MaxDigestEntries, MaxOutputBytes: in.MaxOutputBytes}
			var (
				bundle model.EnvironmentContextBundle
				err    error
			)
			if owner != nil {
				bundle, err = owner.ContextBundle(ctx, in.EnvironmentID, request)
			} else {
				bundle, err = service.EnvironmentContextBundle(ctx, in.EnvironmentID, request)
			}
			if err != nil {
				return nil, model.EnvironmentContextBundle{}, err
			}
			return nil, bundle, nil
		})

	addScopedTool(server, surface, &mcp.Tool{Name: "environment_capability_report", Description: "Return the canonical side-effect-free Environment capability report. With a Gateway runtime owner, owner-local MCP/process/run observations enrich the same CapabilityFact schema without reconnecting, probing, calling tools, or running verifiers."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in EnvironmentInput) (*mcp.CallToolResult, EnvironmentCapabilityReportOutput, error) {
			var (
				report model.CapabilityReport
				err    error
			)
			if owner != nil {
				report, err = owner.CapabilityReport(ctx, in.EnvironmentID)
			} else {
				report, err = service.EnvironmentCapabilityReport(ctx, in.EnvironmentID)
			}
			if err != nil {
				return nil, EnvironmentCapabilityReportOutput{}, err
			}
			return nil, report, nil
		})

	addScopedTool(server, surface, &mcp.Tool{Name: "investigation_provider_inspect", Description: "Inspect optional Environment-authorized code intelligence providers such as GitNexus. Uses desired configuration and existing Gateway-owner observations only; does not connect, refresh, index, call provider tools, run verifiers, or mutate files."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in EnvironmentInput) (*mcp.CallToolResult, InvestigationProviderReportOutput, error) {
			var (
				report model.InvestigationProviderReport
				err    error
			)
			if owner != nil {
				report, err = owner.InvestigationProviderReport(ctx, in.EnvironmentID)
			} else {
				report, err = service.InvestigationProviderReport(ctx, in.EnvironmentID)
			}
			if err != nil {
				return nil, InvestigationProviderReportOutput{}, err
			}
			return nil, report, nil
		})

	addScopedTool(server, surface, &mcp.Tool{Name: "environment_rename", Description: "Rename one Environment in ADM metadata only. The root directory, selections, private memory, writer state, and project files are unchanged."},
		func(_ context.Context, _ *mcp.CallToolRequest, in EnvironmentRenameInput) (*mcp.CallToolResult, any, error) {
			env, err := service.Environments.Rename(in.EnvironmentID, in.Name)
			return toolResult(env, err)
		})

	addScopedTool(server, surface, &mcp.Tool{Name: "environment_workspace_options", Description: "List registered non-system Workspaces that contain one ordinary Environment root, ordered from most specific to broadest, and report the recommended/current binding. Managed-worktree Environments keep their logical source Workspace binding."},
		func(_ context.Context, _ *mcp.CallToolRequest, in EnvironmentInput) (*mcp.CallToolResult, any, error) {
			options, err := service.Environments.WorkspaceOptions(in.EnvironmentID)
			return toolResult(options, err)
		})

	addScopedTool(server, surface, &mcp.Tool{Name: "environment_workspace_recommendations", Description: "List ordinary Environments whose current Workspace is broader than the most specific registered non-system Workspace containing their root. Read-only preview; it never changes Environment bindings."},
		func(context.Context, *mcp.CallToolRequest, EmptyInput) (*mcp.CallToolResult, any, error) {
			items, err := service.Environments.WorkspaceRecommendations()
			return toolResult(items, err)
		})

	addScopedTool(server, surface, &mcp.Tool{Name: "environment_workspace_set", Description: "Rebind one ordinary Environment to a registered non-system Workspace that contains its root. Explicit MCP/Skill selections are preserved and inherited Workspace selections are recalculated. Managed-worktree Environments cannot be rebound by physical path."},
		func(_ context.Context, _ *mcp.CallToolRequest, in EnvironmentWorkspaceSetInput) (*mcp.CallToolResult, any, error) {
			env, err := service.Environments.SetWorkspace(in.EnvironmentID, in.WorkspaceID)
			return toolResult(env, err)
		})

	addScopedTool(server, surface, &mcp.Tool{Name: "environment_remove", Description: "Remove one ADM Environment record without deleting its root directory or project files. Active writers block removal."},
		func(_ context.Context, _ *mcp.CallToolRequest, in EnvironmentInput) (*mcp.CallToolResult, any, error) {
			env, err := service.Environments.Remove(in.EnvironmentID)
			if err == nil && owner != nil {
				owner.DropEnvironment(in.EnvironmentID)
			}
			return toolResult(map[string]any{"removed": env}, err)
		})

	addScopedTool(server, surface, &mcp.Tool{Name: "environment_writer_acquire", Description: "Acquire or renew the single writer lease for an Environment physical root."},
		func(_ context.Context, _ *mcp.CallToolRequest, in WriterAcquireInput) (*mcp.CallToolResult, any, error) {
			env, err := service.Environments.AcquireWriter(in.EnvironmentID, in.Owner)
			return toolResult(env, err)
		})

	addScopedTool(server, surface, &mcp.Tool{Name: "environment_writer_heartbeat", Description: "Renew an active writer lease without performing a file mutation."},
		func(_ context.Context, _ *mcp.CallToolRequest, in WriterAcquireInput) (*mcp.CallToolResult, any, error) {
			env, err := service.Environments.HeartbeatWriter(in.EnvironmentID, in.Owner)
			return toolResult(env, err)
		})

	addScopedTool(server, surface, &mcp.Tool{Name: "environment_writer_release", Description: "Release an Environment writer lease."},
		func(_ context.Context, _ *mcp.CallToolRequest, in WriterReleaseInput) (*mcp.CallToolResult, any, error) {
			if in.Force && surface != serverSurfaceAdmin {
				return toolResult(nil, fmt.Errorf("force writer release is only available on the administrative surface"))
			}
			env, err := service.Environments.ReleaseWriter(in.EnvironmentID, in.Owner, in.Force)
			return toolResult(env, err)
		})

	addScopedTool(server, surface, &mcp.Tool{Name: "environment_verifier_add", Description: "Add one structured verifier definition for an Environment. Admin surface only; this does not modify the executable allowlist."},
		func(_ context.Context, _ *mcp.CallToolRequest, in EnvironmentVerifierAddInput) (*mcp.CallToolResult, any, error) {
			definition, err := service.AddVerifier(in.EnvironmentID, in.Definition)
			return toolResult(definition, err)
		})

	addScopedTool(server, surface, &mcp.Tool{Name: "environment_verifier_remove", Description: "Remove one structured verifier definition from an Environment. Admin surface only."},
		func(_ context.Context, _ *mcp.CallToolRequest, in EnvironmentVerifierRemoveInput) (*mcp.CallToolResult, any, error) {
			definition, err := service.RemoveVerifier(in.EnvironmentID, in.VerifierID)
			return toolResult(map[string]any{"removed": definition}, err)
		})

	addScopedTool(server, surface, &mcp.Tool{Name: "environment_verifier_list", Description: "List structured verifier definitions configured for one Environment. No writer is required."},
		func(_ context.Context, _ *mcp.CallToolRequest, in EnvironmentInput) (*mcp.CallToolResult, any, error) {
			items, err := service.ListVerifiers(in.EnvironmentID)
			return toolResult(items, err)
		})

	addScopedTool(server, surface, &mcp.Tool{Name: "environment_verifier_run", Description: "Run one configured Environment verifier synchronously through the existing Runtime execution policy. This call blocks until verification completes; use environment_verifier_run_start/status/cancel for long or heavy verification. Requires the matching writer_owner."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in EnvironmentVerifierRunInput) (*mcp.CallToolResult, any, error) {
			result, err := service.RunVerifier(ctx, in.EnvironmentID, in.WriterOwner, in.VerifierID, in.MaxOutputBytes)
			return toolResult(result, err)
		})

	addScopedTool(server, surface, &mcp.Tool{Name: "environment_verifier_run_start", Description: "Start one configured Environment verifier asynchronously under the persistent Gateway owner. Preferred for long or heavy verifier/test/build work; returns immediately with a stable owner-local verifier_run identity. Requires the matching writer_owner."},
		func(_ context.Context, _ *mcp.CallToolRequest, in EnvironmentVerifierRunInput) (*mcp.CallToolResult, any, error) {
			if owner == nil {
				return toolResult(nil, fmt.Errorf("persistent runtime owner is unavailable"))
			}
			value, err := owner.StartVerifierRun(in.EnvironmentID, in.WriterOwner, in.VerifierID, in.MaxOutputBytes)
			return toolResult(value, err)
		})

	addScopedTool(server, surface, &mcp.Tool{Name: "environment_verifier_run_list", Description: "List owner-local asynchronous verifier runs for one Environment. Read-only; does not acquire or renew the writer lease and observations are not persisted across Gateway restart."},
		func(_ context.Context, _ *mcp.CallToolRequest, in EnvironmentInput) (*mcp.CallToolResult, any, error) {
			if owner == nil {
				return toolResult(nil, fmt.Errorf("persistent runtime owner is unavailable"))
			}
			value, err := owner.ListVerifierRuns(in.EnvironmentID)
			return toolResult(value, err)
		})

	addScopedTool(server, surface, &mcp.Tool{Name: "environment_verifier_run_status", Description: "Inspect one owner-local asynchronous verifier run by stable verifier_run_id, including bounded live output and terminal verifier result when available. Read-only; does not renew the writer lease."},
		func(_ context.Context, _ *mcp.CallToolRequest, in EnvironmentVerifierRunStatusInput) (*mcp.CallToolResult, any, error) {
			if owner == nil {
				return toolResult(nil, fmt.Errorf("persistent runtime owner is unavailable"))
			}
			value, err := owner.VerifierRunStatus(in.EnvironmentID, in.VerifierRunID)
			return toolResult(value, err)
		})

	addScopedTool(server, surface, &mcp.Tool{Name: "environment_verifier_run_cancel", Description: "Cancel one running owner-local verifier run by stable verifier_run_id. Requires the matching current writer and the same writer owner that launched the run; raw PIDs are not accepted."},
		func(_ context.Context, _ *mcp.CallToolRequest, in EnvironmentVerifierRunCancelInput) (*mcp.CallToolResult, any, error) {
			if owner == nil {
				return toolResult(nil, fmt.Errorf("persistent runtime owner is unavailable"))
			}
			value, err := owner.CancelVerifierRun(in.EnvironmentID, in.WriterOwner, in.VerifierRunID)
			return toolResult(value, err)
		})

	addScopedTool(server, surface, &mcp.Tool{Name: "mcp_list", Description: "List global MCP catalog entries."},
		func(context.Context, *mcp.CallToolRequest, EmptyInput) (*mcp.CallToolResult, any, error) {
			items, err := service.MCPs.List()
			return toolResult(items, err)
		})
	addScopedTool(server, surface, &mcp.Tool{Name: "mcp_add", Description: "Add one typed global MCP definition using streamable-http or stdio configuration."},
		func(_ context.Context, _ *mcp.CallToolRequest, in MCPAddInput) (*mcp.CallToolResult, any, error) {
			item, err := service.MCPs.AddMCPConfig(in.Name, catalog.MCPConfig{
				Transport:      in.Transport,
				AuthMode:       in.AuthMode,
				Endpoint:       in.Endpoint,
				HeaderRefs:     in.HeaderRefs,
				Executable:     in.Executable,
				Args:           in.Args,
				EnvRefs:        in.EnvRefs,
				HealthPolicy:   in.HealthPolicy,
				DefaultInclude: in.DefaultInclude,
			})
			return toolResult(item, err)
		})
	addScopedTool(server, surface, &mcp.Tool{Name: "mcp_update", Description: "Atomically update one global MCP definition while preserving its stable ID; owned runtime state is invalidated so new configuration takes effect without a Gateway restart."},
		func(_ context.Context, _ *mcp.CallToolRequest, in MCPUpdateInput) (*mcp.CallToolResult, any, error) {
			item, err := service.MCPs.UpdateMCPConfig(in.ID, in.Name, catalog.MCPConfig{
				Transport:      in.Transport,
				AuthMode:       in.AuthMode,
				Endpoint:       in.Endpoint,
				HeaderRefs:     in.HeaderRefs,
				Executable:     in.Executable,
				Args:           in.Args,
				EnvRefs:        in.EnvRefs,
				HealthPolicy:   in.HealthPolicy,
				DefaultInclude: in.DefaultInclude,
			})
			if err == nil && owner != nil {
				owner.DropMCP(in.ID)
			}
			return toolResult(item, err)
		})
	addScopedTool(server, surface, &mcp.Tool{Name: "mcp_import_preview", Description: "Preview sanitized MCP JSON/JSONC import candidates without persisting source content or changing Environment selections."},
		func(_ context.Context, _ *mcp.CallToolRequest, in app.MCPImportInput) (*mcp.CallToolResult, any, error) {
			preview, err := service.PreviewMCPImport(in)
			return toolResult(preview, err)
		})
	addScopedTool(server, surface, &mcp.Tool{Name: "mcp_import_apply", Description: "Reparse and atomically apply selected MCP JSON/JSONC candidates to the global MCP catalog using explicit conflict policy."},
		func(_ context.Context, _ *mcp.CallToolRequest, in app.MCPImportInput) (*mcp.CallToolResult, any, error) {
			result, err := service.ApplyMCPImport(in)
			if err == nil && owner != nil {
				for _, mutation := range result.Result.Mutations {
					if mutation.Action == catalog.MCPConflictUpdateByName {
						owner.DropMCP(mutation.Definition.ID)
					}
				}
			}
			return toolResult(result, err)
		})
	addScopedTool(server, surface, &mcp.Tool{Name: "mcp_remove", Description: "Remove one global MCP catalog entry and clear that MCP ID from every Environment selection in the same persisted update."},
		func(_ context.Context, _ *mcp.CallToolRequest, in CatalogIDInput) (*mcp.CallToolResult, any, error) {
			err := service.MCPs.Remove(in.ID)
			if err == nil && owner != nil {
				owner.DropMCP(in.ID)
			}
			return toolResult(map[string]any{"removed": in.ID}, err)
		})
	addScopedTool(server, surface, &mcp.Tool{Name: "mcp_set_default", Description: "Change whether a global MCP is selected by newly created Environments."},
		func(_ context.Context, _ *mcp.CallToolRequest, in CatalogDefaultInput) (*mcp.CallToolResult, any, error) {
			item, err := service.MCPs.SetDefault(in.ID, in.DefaultInclude)
			return toolResult(item, err)
		})
	addScopedTool(server, surface, &mcp.Tool{Name: "workspace_mcp_set", Description: "Enable or disable one global MCP ID for a Workspace. Enabled Workspace MCPs are inherited by every Environment in that Workspace."},
		func(_ context.Context, _ *mcp.CallToolRequest, in WorkspaceSelectionInput) (*mcp.CallToolResult, any, error) {
			workspace, err := service.SetWorkspaceMCP(in.WorkspaceID, in.ID, in.Enabled)
			if err == nil && owner != nil && !in.Enabled {
				if environments, listErr := service.Environments.List(); listErr == nil {
					for _, environment := range environments {
						if environment.WorkspaceID == in.WorkspaceID && !containsValue(environment.EnabledMCPIDs, in.ID) {
							owner.Drop(environment.ID, in.ID)
						}
					}
				}
			}
			return toolResult(workspace, err)
		})
	addScopedTool(server, surface, &mcp.Tool{Name: "environment_mcp_set", Description: "Enable or disable one global MCP ID explicitly for one Environment. Workspace-enabled MCPs remain inherited."},
		func(_ context.Context, _ *mcp.CallToolRequest, in EnvironmentSelectionInput) (*mcp.CallToolResult, any, error) {
			env, err := service.SetEnvironmentMCP(in.EnvironmentID, in.ID, in.Enabled)
			if err == nil && owner != nil && !in.Enabled {
				owner.Drop(in.EnvironmentID, in.ID)
			}
			return toolResult(env, err)
		})

	addScopedTool(server, surface, &mcp.Tool{Name: "mcp_probe", Description: "Explicitly probe one global MCP definition without an Environment. Stdio obeys the executable allowlist in a temporary working directory; no selections or business tools are changed."},
		func(_ context.Context, _ *mcp.CallToolRequest, in CatalogIDInput) (*mcp.CallToolResult, app.MCPHealthStatus, error) {
			// A real HTTP Admin MCP tool context carries inbound MCP session values.
			// Reusing it for a nested outbound MCP client can make the SDK reject the
			// initialized notification. Start from a clean context; MCPProbe applies
			// its own bounded probe timeout and does not retain the transient session.
			status, err := service.MCPProbe(context.Background(), in.ID)
			return nil, status, err
		})

	addScopedTool(server, surface, &mcp.Tool{Name: "environment_mcp_status", Description: "Probe one MCP selected for an Environment and return configured, disabled, healthy, or error status. No writer is required."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in EnvironmentMCPStatusInput) (*mcp.CallToolResult, app.MCPHealthStatus, error) {
			var (
				status app.MCPHealthStatus
				err    error
			)
			if owner != nil {
				status, err = owner.Status(ctx, in.EnvironmentID, in.MCPID)
			} else {
				status, err = service.ProbeMCPHealth(ctx, in.EnvironmentID, in.MCPID)
			}
			if err != nil {
				return nil, app.MCPHealthStatus{}, err
			}
			return nil, status, nil
		})

	addScopedTool(server, surface, &mcp.Tool{Name: "environment_mcp_inspect", Description: "Inspect sanitized desired MCP configuration and owner-local runtime health, recovery, and tool inventory evidence."},
		func(_ context.Context, _ *mcp.CallToolRequest, in EnvironmentMCPRuntimeInput) (*mcp.CallToolResult, any, error) {
			if owner == nil {
				return toolResult(nil, fmt.Errorf("persistent runtime owner is unavailable"))
			}
			inspection, err := owner.Inspect(in.EnvironmentID, in.MCPID)
			return toolResult(inspection, err)
		})

	addScopedTool(server, surface, &mcp.Tool{Name: "environment_mcp_refresh", Description: "Explicitly discard one MCP session and observation, reconnect, Ping, and refresh its bounded public tool inventory."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in EnvironmentMCPRuntimeInput) (*mcp.CallToolResult, any, error) {
			if owner == nil {
				return toolResult(nil, fmt.Errorf("persistent runtime owner is unavailable"))
			}
			observation, err := owner.Refresh(ctx, in.EnvironmentID, in.MCPID)
			return toolResult(observation, err)
		})

	addScopedTool(server, surface, &mcp.Tool{Name: "environment_mcp_tools", Description: "List tools from one external MCP selected for the Environment after injection checks pass. Use environment_injection_plan first when deciding whether the MCP is currently injectable."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in EnvironmentMCPRuntimeInput) (*mcp.CallToolResult, any, error) {
			if owner != nil {
				tools, err := owner.ListTools(ctx, in.EnvironmentID, in.MCPID)
				return toolResult(map[string]any{"mcp_id": in.MCPID, "tools": tools}, err)
			}
			if err := requireHealthyMCP(ctx, service, in.EnvironmentID, in.MCPID); err != nil {
				return toolResult(nil, err)
			}
			connection, err := enabledMCPConnection(ctx, service, in.EnvironmentID, in.MCPID)
			if err != nil {
				return toolResult(nil, err)
			}
			session, err := connectExternalMCP(ctx, in.MCPID, connection.endpoint, connection.headers)
			if err != nil {
				return toolResult(nil, err)
			}
			defer session.Close()
			params := &mcp.ListToolsParams{}
			var tools []*mcp.Tool
			for {
				page, err := session.ListTools(ctx, params)
				if err != nil {
					kind := app.ClassifyMCPError(err)
					if kind == "connection_failed" {
						kind = "tool_list_failed"
					}
					return toolResult(nil, &app.MCPError{MCPID: in.MCPID, ErrorKind: kind, Message: "external MCP tool listing failed"})
				}
				tools = append(tools, page.Tools...)
				if page.NextCursor == "" {
					break
				}
				params.Cursor = page.NextCursor
			}
			return toolResult(map[string]any{"mcp_id": in.MCPID, "tools": tools}, nil)
		})
	addScopedTool(server, surface, &mcp.Tool{Name: "environment_mcp_call", Description: "Call a tool on one external MCP that is enabled and healthy for the selected Environment."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in EnvironmentMCPCallInput) (*mcp.CallToolResult, any, error) {
			if strings.TrimSpace(in.Tool) == "" {
				return toolResult(nil, &app.MCPError{MCPID: in.MCPID, ErrorKind: "missing_tool_name", Message: "external MCP tool name is required"})
			}
			if owner != nil {
				result, err := owner.CallTool(ctx, in.EnvironmentID, in.MCPID, in.Tool, in.Arguments)
				return toolResult(result, err)
			}
			if err := requireHealthyMCP(ctx, service, in.EnvironmentID, in.MCPID); err != nil {
				return toolResult(nil, err)
			}
			connection, err := enabledMCPConnection(ctx, service, in.EnvironmentID, in.MCPID)
			if err != nil {
				return toolResult(nil, err)
			}
			session, err := connectExternalMCP(ctx, in.MCPID, connection.endpoint, connection.headers)
			if err != nil {
				return toolResult(nil, err)
			}
			defer session.Close()
			result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: in.Tool, Arguments: in.Arguments})
			if err != nil {
				kind := app.ClassifyMCPError(err)
				if kind == "connection_failed" {
					kind = "tool_call_failed"
				}
				return toolResult(nil, &app.MCPError{MCPID: in.MCPID, ErrorKind: kind, Message: "external MCP tool call failed"})
			}
			return toolResult(result, nil)
		})

	addScopedTool(server, surface, &mcp.Tool{Name: "skill_list", Description: "List global Skill catalog entries."},
		func(context.Context, *mcp.CallToolRequest, EmptyInput) (*mcp.CallToolResult, any, error) {
			items, err := service.Skills.List()
			return toolResult(items, err)
		})
	addScopedTool(server, surface, &mcp.Tool{Name: "skill_add", Description: "Compatibility helper: register one explicit Skill source and refresh it once. Optional support roots authorize Skill-owned supporting files."},
		func(_ context.Context, _ *mcp.CallToolRequest, in CatalogAddInput) (*mcp.CallToolResult, any, error) {
			items, err := service.Skills.AddSkillRoot(in.Root, in.SupportRoots, in.DefaultInclude)
			return toolResult(items, err)
		})
	addScopedTool(server, surface, &mcp.Tool{Name: "skill_source_list", Description: "List explicit persisted Skill sources without scanning host paths."},
		func(context.Context, *mcp.CallToolRequest, EmptyInput) (*mcp.CallToolResult, any, error) {
			items, err := service.Skills.ListSkillSources()
			return toolResult(items, err)
		})
	addScopedTool(server, surface, &mcp.Tool{Name: "skill_source_add", Description: "Register one explicit Skill source root and support roots without refreshing or scanning outside the source."},
		func(_ context.Context, _ *mcp.CallToolRequest, in CatalogAddInput) (*mcp.CallToolResult, any, error) {
			item, err := service.Skills.AddSkillSource(in.Root, in.SupportRoots, in.DefaultInclude)
			return toolResult(item, err)
		})
	addScopedTool(server, surface, &mcp.Tool{Name: "skill_source_update", Description: "Edit one persisted Skill source root, support roots, and default include setting without refreshing or scanning host paths."},
		func(_ context.Context, _ *mcp.CallToolRequest, in SkillSourceUpdateInput) (*mcp.CallToolResult, any, error) {
			item, err := service.Skills.UpdateSkillSource(in.ID, in.Root, in.SupportRoots, in.DefaultInclude)
			return toolResult(item, err)
		})
	addScopedTool(server, surface, &mcp.Tool{Name: "skill_source_refresh", Description: "Atomically refresh one Skill source and replace only that source-owned discovered Skill snapshot."},
		func(_ context.Context, _ *mcp.CallToolRequest, in CatalogIDInput) (*mcp.CallToolResult, any, error) {
			result, err := service.Skills.RefreshSkillSource(in.ID)
			return toolResult(result, err)
		})
	addScopedTool(server, surface, &mcp.Tool{Name: "skill_source_remove", Description: "Remove one Skill source and its source-owned discovered Skills, clearing those removed Skill IDs from every Environment selection."},
		func(_ context.Context, _ *mcp.CallToolRequest, in CatalogIDInput) (*mcp.CallToolResult, any, error) {
			result, err := service.Skills.RemoveSkillSource(in.ID)
			return toolResult(result, err)
		})
	addScopedTool(server, surface, &mcp.Tool{Name: "skill_remove", Description: "Remove one global Skill catalog entry and clear that Skill ID from every Environment selection in the same persisted update."},
		func(_ context.Context, _ *mcp.CallToolRequest, in CatalogIDInput) (*mcp.CallToolResult, any, error) {
			err := service.Skills.Remove(in.ID)
			return toolResult(map[string]any{"removed": in.ID}, err)
		})
	addScopedTool(server, surface, &mcp.Tool{Name: "skill_set_default", Description: "Change whether a global Skill is selected by newly created Environments."},
		func(_ context.Context, _ *mcp.CallToolRequest, in CatalogDefaultInput) (*mcp.CallToolResult, any, error) {
			item, err := service.Skills.SetDefault(in.ID, in.DefaultInclude)
			return toolResult(item, err)
		})
	addScopedTool(server, surface, &mcp.Tool{Name: "skill_availability_list", Description: "List global Skill catalog structural availability by checking configured source, artifact and support roots without using Environment selections or interpreting Skill instructions."},
		func(context.Context, *mcp.CallToolRequest, EmptyInput) (*mcp.CallToolResult, any, error) {
			availability, err := service.SkillAvailabilities()
			return toolResult(availability, err)
		})
	addScopedTool(server, surface, &mcp.Tool{Name: "workspace_skill_set", Description: "Enable or disable one global Skill ID for a Workspace. Enabled Workspace Skills are inherited by every Environment in that Workspace."},
		func(_ context.Context, _ *mcp.CallToolRequest, in WorkspaceSelectionInput) (*mcp.CallToolResult, any, error) {
			workspace, err := service.SetWorkspaceSkill(in.WorkspaceID, in.ID, in.Enabled)
			return toolResult(workspace, err)
		})
	addScopedTool(server, surface, &mcp.Tool{Name: "environment_skill_set", Description: "Enable or disable one global Skill ID explicitly for one Environment. Workspace-enabled Skills remain inherited."},
		func(_ context.Context, _ *mcp.CallToolRequest, in EnvironmentSelectionInput) (*mcp.CallToolResult, any, error) {
			env, err := service.SetEnvironmentSkill(in.EnvironmentID, in.ID, in.Enabled)
			return toolResult(env, err)
		})

	addScopedTool(server, surface, &mcp.Tool{Name: "environment_skill_list", Description: "List Environment-specific Skill availability for enabled selections and known disabled Skills without interpreting Skill instructions."},
		func(_ context.Context, _ *mcp.CallToolRequest, in EnvironmentInput) (*mcp.CallToolResult, any, error) {
			availability, err := service.EnvironmentSkillAvailabilities(in.EnvironmentID)
			return toolResult(availability, err)
		})

	addScopedTool(server, surface, &mcp.Tool{Name: "environment_skill_inspect", Description: "Inspect one Skill's Environment-specific availability, source/artifact facts and structured unavailability reason."},
		func(_ context.Context, _ *mcp.CallToolRequest, in EnvironmentSkillReadInput) (*mcp.CallToolResult, any, error) {
			availability, err := service.InspectEnvironmentSkill(in.EnvironmentID, in.SkillID)
			return toolResult(availability, err)
		})

	addScopedTool(server, surface, &mcp.Tool{Name: "environment_skill_files", Description: "List a bounded inventory under an enabled Skill's artifact directory or one explicit support root."},
		func(_ context.Context, _ *mcp.CallToolRequest, in EnvironmentSkillFilesInput) (*mcp.CallToolResult, any, error) {
			inventory, err := service.EnvironmentSkillFiles(in.EnvironmentID, in.SkillID, in.RootKind, in.MaxEntries)
			return toolResult(inventory, err)
		})

	addScopedTool(server, surface, &mcp.Tool{Name: "environment_skill_read", Description: "Read a selected and available Skill's SKILL.md or a file contained by its explicitly configured artifact/support roots. Use environment_injection_plan first when deciding whether the Skill is currently injectable."},
		func(_ context.Context, _ *mcp.CallToolRequest, in EnvironmentSkillReadInput) (*mcp.CallToolResult, any, error) {
			content, err := service.ReadEnvironmentSkill(in.EnvironmentID, in.SkillID, in.Path, in.MaxBytes)
			return toolResult(content, err)
		})

	addScopedTool(server, surface, &mcp.Tool{Name: "memory_global_list", Description: "List global durable memory entries."},
		func(context.Context, *mcp.CallToolRequest, EmptyInput) (*mcp.CallToolResult, any, error) {
			items, err := service.Memory.GlobalList()
			return toolResult(items, err)
		})
	addScopedTool(server, surface, &mcp.Tool{Name: "memory_global_read", Description: "Read one global durable memory entry."},
		func(_ context.Context, _ *mcp.CallToolRequest, in MemoryKeyInput) (*mcp.CallToolResult, any, error) {
			item, err := service.Memory.GlobalRead(in.Key)
			return toolResult(item, err)
		})
	addScopedTool(server, surface, &mcp.Tool{Name: "memory_global_write", Description: "Write one global durable memory entry; scope is explicit."},
		func(_ context.Context, _ *mcp.CallToolRequest, in MemoryWriteInput) (*mcp.CallToolResult, any, error) {
			err := service.Memory.GlobalWrite(in.Key, in.Value)
			if err != nil {
				return toolResult(nil, err)
			}
			item, err := service.Memory.GlobalRead(in.Key)
			return toolResult(item, err)
		})
	addScopedTool(server, surface, &mcp.Tool{Name: "memory_global_delete", Description: "Delete one global durable memory entry."},
		func(_ context.Context, _ *mcp.CallToolRequest, in MemoryKeyInput) (*mcp.CallToolResult, any, error) {
			err := service.Memory.GlobalDelete(in.Key)
			return toolResult(map[string]any{"deleted": in.Key}, err)
		})

	addScopedTool(server, surface, &mcp.Tool{Name: "memory_environment_list", Description: "List private memory entries for one Environment."},
		func(_ context.Context, _ *mcp.CallToolRequest, in EnvironmentInput) (*mcp.CallToolResult, any, error) {
			items, err := service.Memory.EnvironmentList(in.EnvironmentID)
			return toolResult(items, err)
		})
	addScopedTool(server, surface, &mcp.Tool{Name: "memory_environment_read", Description: "Read one private memory entry from one Environment only."},
		func(_ context.Context, _ *mcp.CallToolRequest, in EnvironmentMemoryKeyInput) (*mcp.CallToolResult, any, error) {
			item, err := service.Memory.EnvironmentRead(in.EnvironmentID, in.Key)
			return toolResult(item, err)
		})
	addScopedTool(server, surface, &mcp.Tool{Name: "memory_environment_write", Description: "Write one Environment-private memory entry; scope is explicit."},
		func(_ context.Context, _ *mcp.CallToolRequest, in EnvironmentMemoryWriteInput) (*mcp.CallToolResult, any, error) {
			err := service.Memory.EnvironmentWrite(in.EnvironmentID, in.Key, in.Value)
			if err != nil {
				return toolResult(nil, err)
			}
			item, err := service.Memory.EnvironmentRead(in.EnvironmentID, in.Key)
			return toolResult(item, err)
		})
	addScopedTool(server, surface, &mcp.Tool{Name: "memory_environment_delete", Description: "Delete one private memory entry from one Environment only."},
		func(_ context.Context, _ *mcp.CallToolRequest, in EnvironmentMemoryKeyInput) (*mcp.CallToolResult, any, error) {
			err := service.Memory.EnvironmentDelete(in.EnvironmentID, in.Key)
			return toolResult(map[string]any{"deleted": in.Key}, err)
		})

	addScopedTool(server, surface, &mcp.Tool{Name: "tree", Description: "List files under an Environment root. Works in ordinary non-Git directories."},
		func(_ context.Context, _ *mcp.CallToolRequest, in TreeInput) (*mcp.CallToolResult, any, error) {
			value, err := service.Tree(in.EnvironmentID, in.Path, in.MaxDepth, in.MaxEntries)
			return toolResult(value, err)
		})

	addScopedTool(server, surface, &mcp.Tool{Name: "read", Description: "Read a text file under an Environment root."},
		func(_ context.Context, _ *mcp.CallToolRequest, in ReadInput) (*mcp.CallToolResult, any, error) {
			value, err := service.Read(in.EnvironmentID, in.Path, in.MaxBytes)
			return toolResult(value, err)
		})

	addScopedTool(server, surface, &mcp.Tool{Name: "search", Description: "Search literal text under an Environment root."},
		func(_ context.Context, _ *mcp.CallToolRequest, in SearchInput) (*mcp.CallToolResult, any, error) {
			value, err := service.Search(in.EnvironmentID, in.Path, in.Query, in.MaxFiles, in.MaxMatches, in.MaxBytesPerFile)
			return toolResult(value, err)
		})

	addScopedTool(server, surface, &mcp.Tool{Name: "investigate_endpoint", Description: "Resolve one URL/path to bounded static route evidence with confidence and uncertainties. Does not execute project code, call the endpoint, run verifiers, or mutate state."},
		func(_ context.Context, _ *mcp.CallToolRequest, in EndpointInvestigateInput) (*mcp.CallToolResult, any, error) {
			report, err := service.InvestigateEndpoint(in.EnvironmentID, model.EndpointInvestigationRequest{
				Target:          in.Target,
				Method:          in.Method,
				Path:            in.Path,
				MaxFiles:        in.MaxFiles,
				MaxMatches:      in.MaxMatches,
				MaxBytesPerFile: in.MaxBytesPerFile,
			})
			return toolResult(report, err)
		})
	addScopedTool(server, surface, &mcp.Tool{Name: "write", Description: "Write a text file under an Environment root. Requires the matching writer_owner."},
		func(_ context.Context, _ *mcp.CallToolRequest, in WriteInput) (*mcp.CallToolResult, any, error) {
			value, err := service.Write(in.EnvironmentID, in.WriterOwner, in.Path, in.Content, in.CreateParents)
			return toolResult(value, err)
		})

	addScopedTool(server, surface, &mcp.Tool{Name: "edit", Description: "Apply an exact text replacement. Requires the matching writer_owner."},
		func(_ context.Context, _ *mcp.CallToolRequest, in EditInput) (*mcp.CallToolResult, any, error) {
			value, err := service.Edit(in.EnvironmentID, in.WriterOwner, in.Path, in.OldText, in.NewText, in.ExpectedReplacements)
			return toolResult(value, err)
		})

	addScopedTool(server, surface, &mcp.Tool{Name: "delete", Description: "Delete one file. Requires the matching writer_owner; recursive directory deletion is not exposed."},
		func(_ context.Context, _ *mcp.CallToolRequest, in DeleteInput) (*mcp.CallToolResult, any, error) {
			value, err := service.Delete(in.EnvironmentID, in.WriterOwner, in.Path)
			return toolResult(value, err)
		})

	addScopedTool(server, surface, &mcp.Tool{Name: "exec", Description: "Run one explicitly allowlisted executable inside an Environment root. Requires the matching writer_owner."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in ExecInput) (*mcp.CallToolResult, any, error) {
			value, err := service.Exec(ctx, in.EnvironmentID, in.WriterOwner, in.Executable, in.Args, in.Cwd, in.TimeoutMS, in.MaxOutputBytes)
			return toolResult(value, err)
		})

	addScopedTool(server, surface, &mcp.Tool{Name: "process_start", Description: "Start one allowlisted long-running development process owned by this Gateway. Requires the matching Environment writer_owner."},
		func(_ context.Context, _ *mcp.CallToolRequest, in ProcessStartInput) (*mcp.CallToolResult, any, error) {
			if owner == nil {
				return toolResult(nil, fmt.Errorf("persistent runtime owner is unavailable"))
			}
			value, err := owner.StartDevProcess(in.EnvironmentID, in.WriterOwner, in.Executable, in.Args, in.Cwd, in.MaxLogBytes)
			return toolResult(value, err)
		})

	addScopedTool(server, surface, &mcp.Tool{Name: "process_list", Description: "List development processes owned by this Gateway for one Environment."},
		func(_ context.Context, _ *mcp.CallToolRequest, in EnvironmentInput) (*mcp.CallToolResult, any, error) {
			if owner == nil {
				return toolResult(nil, fmt.Errorf("persistent runtime owner is unavailable"))
			}
			value, err := owner.ListDevProcesses(in.EnvironmentID)
			return toolResult(value, err)
		})

	addScopedTool(server, surface, &mcp.Tool{Name: "process_status", Description: "Inspect one Gateway-owned development process by stable ADM process identity."},
		func(_ context.Context, _ *mcp.CallToolRequest, in ProcessInput) (*mcp.CallToolResult, any, error) {
			if owner == nil {
				return toolResult(nil, fmt.Errorf("persistent runtime owner is unavailable"))
			}
			value, err := owner.DevProcessStatus(in.EnvironmentID, in.ProcessID)
			return toolResult(value, err)
		})

	addScopedTool(server, surface, &mcp.Tool{Name: "process_logs", Description: "Read bounded stdout/stderr tails from one Gateway-owned development process."},
		func(_ context.Context, _ *mcp.CallToolRequest, in ProcessInput) (*mcp.CallToolResult, any, error) {
			if owner == nil {
				return toolResult(nil, fmt.Errorf("persistent runtime owner is unavailable"))
			}
			value, err := owner.DevProcessLogs(in.EnvironmentID, in.ProcessID)
			return toolResult(value, err)
		})

	addScopedTool(server, surface, &mcp.Tool{Name: "process_stop", Description: "Stop one Gateway-owned development process. Requires the matching Environment writer_owner; arbitrary OS PIDs are not accepted."},
		func(_ context.Context, _ *mcp.CallToolRequest, in ProcessStopInput) (*mcp.CallToolResult, any, error) {
			if owner == nil {
				return toolResult(nil, fmt.Errorf("persistent runtime owner is unavailable"))
			}
			value, err := owner.StopDevProcess(in.EnvironmentID, in.WriterOwner, in.ProcessID)
			return toolResult(value, err)
		})

	addScopedTool(server, surface, &mcp.Tool{Name: "run_start", Description: "Start one asynchronous single-command Agent Run owned by this Gateway. Requires the matching Environment writer_owner and reuses the existing Runtime allowlist/cwd policy."},
		func(_ context.Context, _ *mcp.CallToolRequest, in RunStartInput) (*mcp.CallToolResult, any, error) {
			if owner == nil {
				return toolResult(nil, fmt.Errorf("persistent runtime owner is unavailable"))
			}
			value, err := owner.StartAgentRun(in.EnvironmentID, in.WriterOwner, in.Executable, in.Args, in.Cwd, in.TimeoutMS, in.MaxOutputBytes)
			return toolResult(value, err)
		})

	addScopedTool(server, surface, &mcp.Tool{Name: "run_list", Description: "List Agent Runs owned by this Gateway for one Environment. Run observations are owner-local and are not persisted across restart."},
		func(_ context.Context, _ *mcp.CallToolRequest, in EnvironmentInput) (*mcp.CallToolResult, any, error) {
			if owner == nil {
				return toolResult(nil, fmt.Errorf("persistent runtime owner is unavailable"))
			}
			value, err := owner.ListAgentRuns(in.EnvironmentID)
			return toolResult(value, err)
		})

	addScopedTool(server, surface, &mcp.Tool{Name: "run_status", Description: "Inspect one Gateway-owned Agent Run by stable ADM run identity."},
		func(_ context.Context, _ *mcp.CallToolRequest, in RunInput) (*mcp.CallToolResult, any, error) {
			if owner == nil {
				return toolResult(nil, fmt.Errorf("persistent runtime owner is unavailable"))
			}
			value, err := owner.AgentRunStatus(in.EnvironmentID, in.RunID)
			return toolResult(value, err)
		})

	addScopedTool(server, surface, &mcp.Tool{Name: "run_cancel", Description: "Cancel one running Gateway-owned Agent Run. Requires the matching Environment writer_owner and stable run identity."},
		func(_ context.Context, _ *mcp.CallToolRequest, in RunCancelInput) (*mcp.CallToolResult, any, error) {
			if owner == nil {
				return toolResult(nil, fmt.Errorf("persistent runtime owner is unavailable"))
			}
			value, err := owner.CancelAgentRun(in.EnvironmentID, in.WriterOwner, in.RunID)
			return toolResult(value, err)
		})

	addScopedTool(server, surface, &mcp.Tool{Name: "git_status", Description: "Optional Git status tool. Fails locally when the Environment root is not a Git repository."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in EnvironmentInput) (*mcp.CallToolResult, any, error) {
			value, err := service.GitStatus(ctx, in.EnvironmentID)
			return toolResult(value, err)
		})

	addScopedTool(server, surface, &mcp.Tool{Name: "git_diff", Description: "Optional Git diff tool. Git is never required for Environment creation or file development."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in EnvironmentInput) (*mcp.CallToolResult, any, error) {
			value, err := service.GitDiff(ctx, in.EnvironmentID)
			return toolResult(value, err)
		})

	addScopedTool(server, surface, &mcp.Tool{Name: "git_branch", Description: "Optional Git branch tool."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in EnvironmentInput) (*mcp.CallToolResult, any, error) {
			value, err := service.GitBranch(ctx, in.EnvironmentID)
			return toolResult(value, err)
		})

	return server
}

type externalMCPConnection struct {
	endpoint string
	headers  map[string]string
}

func requireHealthyMCP(ctx context.Context, service *app.Service, environmentID, mcpID string) error {
	status, err := service.ProbeMCPHealth(ctx, environmentID, mcpID)
	if err != nil {
		return err
	}
	if status.State == app.MCPHealthHealthy {
		return nil
	}
	kind := status.ErrorKind
	if kind == "" {
		if status.State == app.MCPHealthDisabled {
			kind = "not_enabled"
		} else {
			kind = string(status.State)
		}
	}
	message := status.Message
	if strings.TrimSpace(message) == "" {
		message = fmt.Sprintf("external MCP is %s", status.State)
	}
	return &app.MCPError{MCPID: mcpID, ErrorKind: kind, Message: message}
}

func enabledMCPConnection(_ context.Context, service *app.Service, environmentID, mcpID string) (externalMCPConnection, error) {
	activation, status, err := service.ResolveMCPActivation(environmentID, mcpID)
	if err != nil {
		return externalMCPConnection{}, err
	}
	if activation == nil {
		return externalMCPConnection{}, statusAsMCPError(status)
	}
	return externalMCPConnection{endpoint: activation.Endpoint, headers: activation.Headers}, nil
}

func connectExternalMCP(ctx context.Context, mcpID, endpoint string, headers map[string]string) (*mcp.ClientSession, error) {
	client := mcp.NewClient(&mcp.Implementation{Name: serverName + "-proxy", Version: serverVersion}, nil)
	transport := &mcp.StreamableClientTransport{
		Endpoint:             endpoint,
		MaxRetries:           -1,
		DisableStandaloneSSE: true,
	}
	if len(headers) != 0 {
		transport.HTTPClient = &http.Client{Transport: externalMCPHeaderRoundTripper{base: http.DefaultTransport, headers: headers}}
	}
	session, err := client.Connect(ctx, transport, nil)
	if err != nil {
		return nil, &app.MCPError{MCPID: mcpID, ErrorKind: app.ClassifyMCPError(err), Message: "external MCP connection failed"}
	}
	return session, nil
}

func connectStdioMCP(ctx, commandCtx context.Context, service *app.Service, environmentID string, activation *app.MCPActivation) (*mcp.ClientSession, error) {
	cmd, err := service.MCPCommand(commandCtx, environmentID, activation)
	if err != nil {
		return nil, &app.MCPError{MCPID: activation.MCPID, ErrorKind: "executable_not_allowed", Message: "stdio MCP executable is unavailable under Environment authority"}
	}
	client := mcp.NewClient(&mcp.Implementation{Name: serverName + "-proxy", Version: serverVersion}, nil)
	session, err := client.Connect(ctx, &mcp.CommandTransport{Command: cmd}, nil)
	if err != nil {
		return nil, &app.MCPError{MCPID: activation.MCPID, ErrorKind: app.ClassifyMCPError(err), Message: "external MCP stdio connection failed"}
	}
	return session, nil
}

type externalMCPHeaderRoundTripper struct {
	base    http.RoundTripper
	headers map[string]string
}

func (r externalMCPHeaderRoundTripper) RoundTrip(request *http.Request) (*http.Response, error) {
	clone := request.Clone(request.Context())
	clone.Header = request.Header.Clone()
	for key, value := range r.headers {
		clone.Header.Set(key, value)
	}
	return r.base.RoundTrip(clone)
}

func RunStdio(ctx context.Context, service *app.Service) error {
	owner := newRuntimeOwner(service)
	ownerCtx, cancelOwner := context.WithCancel(ctx)
	defer owner.Close()
	defer cancelOwner()
	go owner.Reconcile(ownerCtx)
	return newServer(service, owner).Run(ctx, &mcp.StdioTransport{})
}

func NewHTTPHandler(service *app.Service) http.Handler {
	return newHTTPHandler(service, nil)
}

func newHTTPHandler(service *app.Service, owner *runtimeOwner) http.Handler {
	return newHTTPHandlerWithShutdown(service, owner, nil)
}

func newHTTPHandlerWithShutdown(service *app.Service, owner *runtimeOwner, shutdown func()) http.Handler {
	agentServer := newServer(service, owner)
	adminServer := newServerForSurface(service, owner, serverSurfaceAdmin)
	newSurfaceHandler := func(server *mcp.Server, surface serverSurface) http.Handler {
		base := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server {
			return server
		}, &mcp.StreamableHTTPOptions{Stateless: true, DisableLocalhostProtection: true})
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !gatewayRequestAllowed(service, r, surface) {
				service.Log("warn", "gateway.access_denied", map[string]string{"host": r.Host, "path": r.URL.Path, "remote_addr": r.RemoteAddr})
				http.Error(w, "Forbidden: gateway access denied", http.StatusForbidden)
				return
			}
			base.ServeHTTP(w, r)
		})
	}
	mux := http.NewServeMux()
	mux.Handle("/mcp", newSurfaceHandler(agentServer, serverSurfaceAgent))
	mux.Handle("/admin/mcp", newSurfaceHandler(adminServer, serverSurfaceAdmin))
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		ownerID := ""
		if owner != nil {
			ownerID = owner.Info().ID
		}
		_, _ = fmt.Fprintf(w, `{"name":%q,"version":%q,"status":"ok","pid":%d,"transport":"http","owner_id":%q}`, serverName, serverVersion, os.Getpid(), ownerID)
	})
	mux.HandleFunc("/shutdown", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			return
		}
		if !gatewayShutdownRequestAllowed(service, r) {
			service.Log("warn", "gateway.shutdown_denied", map[string]string{"host": r.Host, "remote_addr": r.RemoteAddr})
			http.Error(w, "Forbidden: gateway access denied", http.StatusForbidden)
			return
		}
		if owner == nil || shutdown == nil {
			http.NotFound(w, r)
			return
		}
		expectedOwnerID := owner.Info().ID
		if providedOwnerID := strings.TrimSpace(r.Header.Get(runtimeOwnerHeader)); providedOwnerID == "" || providedOwnerID != expectedOwnerID {
			http.Error(w, "runtime owner mismatch", http.StatusConflict)
			return
		}
		w.WriteHeader(http.StatusAccepted)
		go shutdown()
	})
	return mux
}

func allowedGatewayHost(authority string) bool {
	host := strings.TrimSpace(authority)
	if parsed, _, err := net.SplitHostPort(host); err == nil {
		host = parsed
	}
	host = strings.Trim(strings.ToLower(host), "[]")
	switch host {
	case "localhost", "127.0.0.1", "::1", "host.docker.internal":
		return true
	default:
		return false
	}
}

func RunHTTP(ctx context.Context, service *app.Service, listen string) error {
	listen = strings.TrimSpace(listen)
	if listen == "" {
		return fmt.Errorf("gateway listen address is required")
	}
	host, _, err := net.SplitHostPort(listen)
	if err != nil {
		return fmt.Errorf("invalid gateway listen address %q: %w", listen, err)
	}
	if host != "localhost" {
		ip := net.ParseIP(host)
		if ip == nil || !ip.IsLoopback() {
			if !remoteGatewayListenAllowed(service) {
				return fmt.Errorf("remote gateway listen %q requires at least one allowed host plus separate Admin MCP and Agent MCP API keys", listen)
			}
		}
	}

	service.Log("info", "gateway.start", map[string]string{"listen": listen})
	defer service.Log("info", "gateway.stop", map[string]string{"listen": listen})
	runCtx, cancelRun := context.WithCancel(ctx)
	defer cancelRun()
	owner := newRuntimeOwner(service)
	ownerCtx, cancelOwner := context.WithCancel(runCtx)
	defer owner.Close()
	defer cancelOwner()
	go owner.Reconcile(ownerCtx)

	httpServer := &http.Server{
		Addr:              listen,
		Handler:           newHTTPHandlerWithShutdown(service, owner, cancelRun),
		ReadHeaderTimeout: 5 * time.Second,
	}
	go func() {
		<-runCtx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = httpServer.Shutdown(shutdownCtx)
	}()
	if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

func toolResult(value any, err error) (*mcp.CallToolResult, any, error) {
	if err != nil {
		return nil, nil, fmt.Errorf("%w", err)
	}
	return nil, map[string]any{"result": value}, nil
}
