package desktop

import (
	"context"

	"ai-dev-manager-v2/internal/adminmcp"
	"ai-dev-manager-v2/internal/app"
	"ai-dev-manager-v2/internal/catalog"
	"ai-dev-manager-v2/internal/hostenv"
	"ai-dev-manager-v2/internal/logging"
	"ai-dev-manager-v2/internal/management"
	"ai-dev-manager-v2/internal/memory"
	"ai-dev-manager-v2/internal/model"
	"ai-dev-manager-v2/internal/verifier"
)

type managementBackend interface {
	Snapshot() (management.Snapshot, error)
	WorktreeSettings() (model.WorktreeSettings, error)
	WorktreeSettingsUpdate(string, string) (model.WorktreeSettings, error)
	HostEnvironmentStatus() (hostenv.Status, error)
	HostEnvironmentRefresh() (hostenv.Status, error)
	WorkspaceInspect(string) (model.Workspace, error)
	WorkspaceDiscover(string, model.DiscoveryRequest) (model.DiscoveryReport, error)
	WorkspaceAdd(string, string) (model.Workspace, error)
	WorkspaceRename(string, string) (model.Workspace, error)
	WorkspaceRemove(string) (model.Workspace, error)
	EnvironmentInspect(string) (app.EnvironmentInspection, error)
	EnvironmentTreeDigest(string, model.DiscoveryRequest) (model.DiscoveryReport, error)
	EnvironmentCreate(string, string, string) (app.EnvironmentSummary, error)
	EnvironmentRename(string, string) (app.EnvironmentSummary, error)
	EnvironmentWorkspaceOptions(string) (model.EnvironmentWorkspaceOptions, error)
	EnvironmentWorkspaceRecommendations() ([]model.EnvironmentWorkspaceRecommendation, error)
	EnvironmentWorkspaceSet(string, string) (app.EnvironmentSummary, error)
	EnvironmentRemove(string) error
	ExecAllow(string) ([]string, error)
	ExecRemove(string) ([]string, error)
	ExecBlock(string) ([]string, error)
	ExecUnblock(string) ([]string, error)
	ExecBlockList() ([]string, error)
	ExecDenyList() ([]model.ExecDenial, error)
	ExecDenyClear(string) ([]model.ExecDenial, error)
	ExecDenyClearAll() error
	ExecAuthorizationStatus() (app.ExecAuthorizationStatus, error)
	ExecFullAuthorizationSet(bool) (app.ExecAuthorizationStatus, error)
	LoggingStatus() (logging.Status, error)
	GatewayAccessStatus() (app.GatewayAccessStatus, error)
	GatewayAllowedHostsSet([]string) (app.GatewayAccessStatus, error)
	GatewayAdminAPIKeySet(string) (app.GatewayAccessStatus, error)
	GatewayAdminAPIKeyClear() (app.GatewayAccessStatus, error)
	GatewayAgentAPIKeySet(string) (app.GatewayAccessStatus, error)
	GatewayAgentAPIKeyClear() (app.GatewayAccessStatus, error)
	MCPAddConfig(string, catalog.MCPConfig) (model.MCPDefinition, error)
	MCPUpdateConfig(string, string, catalog.MCPConfig) (model.MCPDefinition, error)
	MCPImportPreview(app.MCPImportInput) (app.MCPImportPreview, error)
	MCPImportApply(app.MCPImportInput) (app.MCPImportApplyResult, error)
	MCPRemove(string) error
	MCPSetDefault(string, bool) (model.MCPDefinition, error)
	MCPProbe(context.Context, string) (app.MCPHealthStatus, error)
	SkillAdd(string, string, bool) ([]model.CatalogEntry, error)
	SkillSourceAdd(string, []string, bool) (model.SkillSource, error)
	SkillSourceUpdate(string, string, []string, bool) (model.SkillSource, error)
	SkillSourceList() ([]model.SkillSource, error)
	SkillSourceRefresh(string) (catalog.SkillSourceRefreshResult, error)
	SkillSourceRemove(string) (catalog.SkillSourceRefreshResult, error)
	SkillRemove(string) error
	SkillAvailabilityList() (app.SkillAvailabilityList, error)
	SkillSetDefault(string, bool) (model.CatalogEntry, error)
	WorkspaceMCPSet(string, string, bool) (model.Workspace, error)
	WorkspaceSkillSet(string, string, bool) (model.Workspace, error)
	EnvironmentMCPSet(string, string, bool) (app.EnvironmentSummary, error)
	EnvironmentSkillSet(string, string, bool) (app.EnvironmentSummary, error)
	EnvironmentSkillList(string) (app.SkillAvailabilityList, error)
	EnvironmentSkillInspect(string, string) (app.SkillAvailability, error)
	GlobalMemoryList() ([]memory.Entry, error)
	GlobalMemoryRead(string) (memory.Entry, error)
	GlobalMemoryWrite(string, string) error
	GlobalMemoryDelete(string) error
	EnvironmentMemoryList(string) ([]memory.Entry, error)
	EnvironmentMemoryRead(string, string) (memory.Entry, error)
	EnvironmentMemoryWrite(string, string, string) error
	EnvironmentMemoryDelete(string, string) error
}

type temporaryEnvironmentBackend interface {
	EnvironmentTemporaryStatus(string) (model.TemporaryEnvironmentStatus, error)
	EnvironmentTemporaryPromote(string, string) (model.TemporaryEnvironmentStatus, error)
	EnvironmentTemporaryCleanup(string, string, bool) (model.ResourceRetentionCleanupResult, error)
	EnvironmentTemporaryCleanupExpired(bool) (model.ResourceRetentionCleanupResult, error)
}

type runtimeBackend interface {
	WriterAcquire(string, string) (model.Environment, error)
	WriterRelease(string, string, bool) (model.Environment, error)
	VerifierList(string) ([]model.VerifierDefinition, error)
	VerifierRun(string, string, string, int) (verifier.Result, error)
	ProcessList(string) ([]adminmcp.ProcessStatus, error)
	ProcessStatus(string, string) (adminmcp.ProcessStatus, error)
	ProcessLogs(string, string) (adminmcp.ProcessLogs, error)
	ProcessStop(string, string, string) (adminmcp.ProcessStatus, error)
	RunList(string) ([]adminmcp.RunStatus, error)
	RunStatus(string, string) (adminmcp.RunStatus, error)
	RunCancel(string, string, string) (adminmcp.RunStatus, error)
}
