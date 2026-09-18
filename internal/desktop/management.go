package desktop

import (
	"context"
	"errors"

	"ai-dev-manager-v2/internal/app"
	"ai-dev-manager-v2/internal/catalog"
	"ai-dev-manager-v2/internal/hostenv"
	"ai-dev-manager-v2/internal/logging"
	"ai-dev-manager-v2/internal/management"
	"ai-dev-manager-v2/internal/memory"
	"ai-dev-manager-v2/internal/model"
)

func (a *Adapter) GetSnapshot() (management.Snapshot, error) {
	if err := a.ready(); err != nil {
		return management.Snapshot{}, err
	}
	return a.management.Snapshot()
}

func (a *Adapter) GetWorktreeSettings() (model.WorktreeSettings, error) {
	if err := a.ready(); err != nil {
		return model.WorktreeSettings{}, err
	}
	return a.management.WorktreeSettings()
}

func (a *Adapter) UpdateWorktreeSettings(root, branchPrefix string) (model.WorktreeSettings, error) {
	if err := a.ready(); err != nil {
		return model.WorktreeSettings{}, err
	}
	return a.management.WorktreeSettingsUpdate(root, branchPrefix)
}

func (a *Adapter) GetHostEnvironmentStatus() (hostenv.Status, error) {
	if err := a.ready(); err != nil {
		return hostenv.Status{}, err
	}
	return a.management.HostEnvironmentStatus()
}

func (a *Adapter) RefreshHostEnvironment() (hostenv.Status, error) {
	if err := a.ready(); err != nil {
		return hostenv.Status{}, err
	}
	return a.management.HostEnvironmentRefresh()
}

func (a *Adapter) InspectWorkspace(id string) (model.Workspace, error) {
	if err := a.ready(); err != nil {
		return model.Workspace{}, err
	}
	return a.management.WorkspaceInspect(id)
}

func (a *Adapter) DiscoverWorkspace(id string, request model.DiscoveryRequest) (model.DiscoveryReport, error) {
	if err := a.ready(); err != nil {
		return model.DiscoveryReport{}, err
	}
	return a.management.WorkspaceDiscover(id, request)
}

func (a *Adapter) AddWorkspace(input WorkspaceInput) (model.Workspace, error) {
	if err := a.ready(); err != nil {
		return model.Workspace{}, err
	}
	return a.management.WorkspaceAdd(input.Path, input.Name)
}

func (a *Adapter) RenameWorkspace(id, name string) (model.Workspace, error) {
	if err := a.ready(); err != nil {
		return model.Workspace{}, err
	}
	return a.management.WorkspaceRename(id, name)
}

func (a *Adapter) RemoveWorkspace(id string) (model.Workspace, error) {
	if err := a.ready(); err != nil {
		return model.Workspace{}, err
	}
	return a.management.WorkspaceRemove(id)
}

func (a *Adapter) InspectEnvironment(id string) (app.EnvironmentInspection, error) {
	if err := a.ready(); err != nil {
		return app.EnvironmentInspection{}, err
	}
	return a.management.EnvironmentInspect(id)
}

func (a *Adapter) EnvironmentTreeDigest(id string, request model.DiscoveryRequest) (model.DiscoveryReport, error) {
	if err := a.ready(); err != nil {
		return model.DiscoveryReport{}, err
	}
	return a.management.EnvironmentTreeDigest(id, request)
}

func (a *Adapter) CreateEnvironment(input EnvironmentInput) (app.EnvironmentSummary, error) {
	if err := a.ready(); err != nil {
		return app.EnvironmentSummary{}, err
	}
	return a.management.EnvironmentCreate(input.WorkspaceID, input.Name, input.Root)
}

func (a *Adapter) RenameEnvironment(id, name string) (app.EnvironmentSummary, error) {
	if err := a.ready(); err != nil {
		return app.EnvironmentSummary{}, err
	}
	return a.management.EnvironmentRename(id, name)
}

func (a *Adapter) EnvironmentWorkspaceOptions(id string) (model.EnvironmentWorkspaceOptions, error) {
	if err := a.ready(); err != nil {
		return model.EnvironmentWorkspaceOptions{}, err
	}
	return a.management.EnvironmentWorkspaceOptions(id)
}

func (a *Adapter) EnvironmentWorkspaceRecommendations() ([]model.EnvironmentWorkspaceRecommendation, error) {
	if err := a.ready(); err != nil {
		return nil, err
	}
	return a.management.EnvironmentWorkspaceRecommendations()
}

func (a *Adapter) SetEnvironmentWorkspace(id, workspaceID string) (app.EnvironmentSummary, error) {
	if err := a.ready(); err != nil {
		return app.EnvironmentSummary{}, err
	}
	return a.management.EnvironmentWorkspaceSet(id, workspaceID)
}

func (a *Adapter) RemoveEnvironment(id string) error {
	if err := a.ready(); err != nil {
		return err
	}
	return a.management.EnvironmentRemove(id)
}

func (a *Adapter) GetTemporaryEnvironmentStatus(id string) (model.TemporaryEnvironmentStatus, error) {
	if err := a.ready(); err != nil {
		return model.TemporaryEnvironmentStatus{}, err
	}
	backend, ok := a.management.(temporaryEnvironmentBackend)
	if !ok {
		return model.TemporaryEnvironmentStatus{}, errors.New("temporary Environment lifecycle requires ADM Admin MCP")
	}
	return backend.EnvironmentTemporaryStatus(id)
}

func (a *Adapter) PromoteTemporaryEnvironment(id, ownerID string) (model.TemporaryEnvironmentStatus, error) {
	if err := a.ready(); err != nil {
		return model.TemporaryEnvironmentStatus{}, err
	}
	backend, ok := a.management.(temporaryEnvironmentBackend)
	if !ok {
		return model.TemporaryEnvironmentStatus{}, errors.New("temporary Environment lifecycle requires ADM Admin MCP")
	}
	return backend.EnvironmentTemporaryPromote(id, ownerID)
}

func (a *Adapter) CleanupExpiredTemporaryEnvironments(execute bool) (model.ResourceRetentionCleanupResult, error) {
	if err := a.ready(); err != nil {
		return model.ResourceRetentionCleanupResult{}, err
	}
	backend, ok := a.management.(temporaryEnvironmentBackend)
	if !ok {
		return model.ResourceRetentionCleanupResult{}, errors.New("temporary Environment lifecycle requires ADM Admin MCP")
	}
	return backend.EnvironmentTemporaryCleanupExpired(execute)
}

func (a *Adapter) CleanupTemporaryEnvironment(id, ownerID string, execute bool) (model.ResourceRetentionCleanupResult, error) {
	if err := a.ready(); err != nil {
		return model.ResourceRetentionCleanupResult{}, err
	}
	backend, ok := a.management.(temporaryEnvironmentBackend)
	if !ok {
		return model.ResourceRetentionCleanupResult{}, errors.New("temporary Environment lifecycle requires ADM Admin MCP")
	}
	return backend.EnvironmentTemporaryCleanup(id, ownerID, execute)
}

func (a *Adapter) AllowExecutable(executable string) ([]string, error) {
	if err := a.ready(); err != nil {
		return nil, err
	}
	return a.management.ExecAllow(executable)
}

func (a *Adapter) RemoveExecutable(executable string) ([]string, error) {
	if err := a.ready(); err != nil {
		return nil, err
	}
	return a.management.ExecRemove(executable)
}

func (a *Adapter) GetExecAuthorizationStatus() (app.ExecAuthorizationStatus, error) {
	if err := a.ready(); err != nil {
		return app.ExecAuthorizationStatus{}, err
	}
	return a.management.ExecAuthorizationStatus()
}

func (a *Adapter) SetExecFullAuthorization(enabled bool) (app.ExecAuthorizationStatus, error) {
	if err := a.ready(); err != nil {
		return app.ExecAuthorizationStatus{}, err
	}
	return a.management.ExecFullAuthorizationSet(enabled)
}

func (a *Adapter) GetLoggingStatus() (logging.Status, error) {
	if err := a.ready(); err != nil {
		return logging.Status{}, err
	}
	return a.management.LoggingStatus()
}

func (a *Adapter) GetGatewayAccessStatus() (app.GatewayAccessStatus, error) {
	if err := a.ready(); err != nil {
		return app.GatewayAccessStatus{}, err
	}
	return a.management.GatewayAccessStatus()
}

func (a *Adapter) SetGatewayAllowedHosts(hosts []string) (app.GatewayAccessStatus, error) {
	if err := a.ready(); err != nil {
		return app.GatewayAccessStatus{}, err
	}
	return a.management.GatewayAllowedHostsSet(hosts)
}

func (a *Adapter) SetGatewayAdminAPIKey(apiKey string) (app.GatewayAccessStatus, error) {
	if err := a.ready(); err != nil {
		return app.GatewayAccessStatus{}, err
	}
	return a.management.GatewayAdminAPIKeySet(apiKey)
}

func (a *Adapter) ClearGatewayAdminAPIKey() (app.GatewayAccessStatus, error) {
	if err := a.ready(); err != nil {
		return app.GatewayAccessStatus{}, err
	}
	return a.management.GatewayAdminAPIKeyClear()
}

func (a *Adapter) SetGatewayAgentAPIKey(apiKey string) (app.GatewayAccessStatus, error) {
	if err := a.ready(); err != nil {
		return app.GatewayAccessStatus{}, err
	}
	return a.management.GatewayAgentAPIKeySet(apiKey)
}

func (a *Adapter) ClearGatewayAgentAPIKey() (app.GatewayAccessStatus, error) {
	if err := a.ready(); err != nil {
		return app.GatewayAccessStatus{}, err
	}
	return a.management.GatewayAgentAPIKeyClear()
}

func (a *Adapter) ListExecDenials() ([]model.ExecDenial, error) {
	if err := a.ready(); err != nil {
		return nil, err
	}
	return a.management.ExecDenyList()
}

func (a *Adapter) ClearExecDenial(executable string) ([]model.ExecDenial, error) {
	if err := a.ready(); err != nil {
		return nil, err
	}
	return a.management.ExecDenyClear(executable)
}

func (a *Adapter) ClearAllExecDenials() error {
	if err := a.ready(); err != nil {
		return err
	}
	return a.management.ExecDenyClearAll()
}

func (a *Adapter) AddMCP(input MCPInput) (model.MCPDefinition, error) {
	if err := a.ready(); err != nil {
		return model.MCPDefinition{}, err
	}
	return a.management.MCPAddConfig(input.Name, catalog.MCPConfig{
		Transport:      input.Transport,
		AuthMode:       input.AuthMode,
		Endpoint:       input.Endpoint,
		HeaderRefs:     input.HeaderRefs,
		Executable:     input.Executable,
		Args:           input.Args,
		EnvRefs:        input.EnvRefs,
		HealthPolicy:   input.HealthPolicy,
		DefaultInclude: input.DefaultInclude,
	})
}

func (a *Adapter) UpdateMCP(id string, input MCPInput) (model.MCPDefinition, error) {
	if err := a.ready(); err != nil {
		return model.MCPDefinition{}, err
	}
	return a.management.MCPUpdateConfig(id, input.Name, catalog.MCPConfig{
		Transport:      input.Transport,
		AuthMode:       input.AuthMode,
		Endpoint:       input.Endpoint,
		HeaderRefs:     input.HeaderRefs,
		Executable:     input.Executable,
		Args:           input.Args,
		EnvRefs:        input.EnvRefs,
		HealthPolicy:   input.HealthPolicy,
		DefaultInclude: input.DefaultInclude,
	})
}

func (a *Adapter) PreviewMCPImport(input MCPImportInput) (app.MCPImportPreview, error) {
	if err := a.ready(); err != nil {
		return app.MCPImportPreview{}, err
	}
	return a.management.MCPImportPreview(app.MCPImportInput{
		Format: input.Format, Content: input.Content, SelectedNames: input.SelectedNames,
		ConflictPolicy: input.ConflictPolicy, DefaultInclude: input.DefaultInclude, SourceScope: input.SourceScope,
	})
}

func (a *Adapter) ApplyMCPImport(input MCPImportInput) (app.MCPImportApplyResult, error) {
	if err := a.ready(); err != nil {
		return app.MCPImportApplyResult{}, err
	}
	return a.management.MCPImportApply(app.MCPImportInput{
		Format: input.Format, Content: input.Content, SelectedNames: input.SelectedNames,
		ConflictPolicy: input.ConflictPolicy, DefaultInclude: input.DefaultInclude, SourceScope: input.SourceScope,
	})
}

func (a *Adapter) SetMCPDefault(id string, enabled bool) (model.MCPDefinition, error) {
	if err := a.ready(); err != nil {
		return model.MCPDefinition{}, err
	}
	return a.management.MCPSetDefault(id, enabled)
}

func (a *Adapter) ProbeMCPHealth(mcpID string) (app.MCPHealthStatus, error) {
	if err := a.ready(); err != nil {
		return app.MCPHealthStatus{}, err
	}
	return a.management.MCPProbe(context.Background(), mcpID)
}

func (a *Adapter) RemoveMCP(id string) error {
	if err := a.ready(); err != nil {
		return err
	}
	return a.management.MCPRemove(id)
}

func (a *Adapter) AddSkill(input SkillInput) ([]model.CatalogEntry, error) {
	if err := a.ready(); err != nil {
		return nil, err
	}
	return a.management.SkillAdd(input.Root, input.SupportRoot, input.DefaultInclude)
}

func (a *Adapter) AddSkillSource(input SkillSourceInput) (model.SkillSource, error) {
	if err := a.ready(); err != nil {
		return model.SkillSource{}, err
	}
	return a.management.SkillSourceAdd(input.Root, input.SupportRoots, input.DefaultInclude)
}

func (a *Adapter) UpdateSkillSource(id string, input SkillSourceInput) (model.SkillSource, error) {
	if err := a.ready(); err != nil {
		return model.SkillSource{}, err
	}
	return a.management.SkillSourceUpdate(id, input.Root, input.SupportRoots, input.DefaultInclude)
}

func (a *Adapter) ListSkillSources() ([]model.SkillSource, error) {
	if err := a.ready(); err != nil {
		return nil, err
	}
	return a.management.SkillSourceList()
}

func (a *Adapter) RefreshSkillSource(id string) (catalog.SkillSourceRefreshResult, error) {
	if err := a.ready(); err != nil {
		return catalog.SkillSourceRefreshResult{}, err
	}
	return a.management.SkillSourceRefresh(id)
}

func (a *Adapter) RemoveSkillSource(id string) (catalog.SkillSourceRefreshResult, error) {
	if err := a.ready(); err != nil {
		return catalog.SkillSourceRefreshResult{}, err
	}
	return a.management.SkillSourceRemove(id)
}

func (a *Adapter) ListSkillAvailability() (app.SkillAvailabilityList, error) {
	if err := a.ready(); err != nil {
		return app.SkillAvailabilityList{}, err
	}
	return a.management.SkillAvailabilityList()
}

func (a *Adapter) ListEnvironmentSkills(environmentID string) (app.SkillAvailabilityList, error) {
	if err := a.ready(); err != nil {
		return app.SkillAvailabilityList{}, err
	}
	return a.management.EnvironmentSkillList(environmentID)
}

func (a *Adapter) InspectEnvironmentSkill(environmentID, skillID string) (app.SkillAvailability, error) {
	if err := a.ready(); err != nil {
		return app.SkillAvailability{}, err
	}
	return a.management.EnvironmentSkillInspect(environmentID, skillID)
}

func (a *Adapter) SetSkillDefault(id string, enabled bool) (model.CatalogEntry, error) {
	if err := a.ready(); err != nil {
		return model.CatalogEntry{}, err
	}
	return a.management.SkillSetDefault(id, enabled)
}

func (a *Adapter) RemoveSkill(id string) error {
	if err := a.ready(); err != nil {
		return err
	}
	return a.management.SkillRemove(id)
}

func (a *Adapter) SetWorkspaceMCP(workspaceID, mcpID string, enabled bool) (model.Workspace, error) {
	if err := a.ready(); err != nil {
		return model.Workspace{}, err
	}
	return a.management.WorkspaceMCPSet(workspaceID, mcpID, enabled)
}

func (a *Adapter) SetWorkspaceSkill(workspaceID, skillID string, enabled bool) (model.Workspace, error) {
	if err := a.ready(); err != nil {
		return model.Workspace{}, err
	}
	return a.management.WorkspaceSkillSet(workspaceID, skillID, enabled)
}

func (a *Adapter) SetEnvironmentMCP(environmentID, mcpID string, enabled bool) (app.EnvironmentSummary, error) {
	if err := a.ready(); err != nil {
		return app.EnvironmentSummary{}, err
	}
	return a.management.EnvironmentMCPSet(environmentID, mcpID, enabled)
}

func (a *Adapter) SetEnvironmentSkill(environmentID, skillID string, enabled bool) (app.EnvironmentSummary, error) {
	if err := a.ready(); err != nil {
		return app.EnvironmentSummary{}, err
	}
	return a.management.EnvironmentSkillSet(environmentID, skillID, enabled)
}

func (a *Adapter) ListGlobalMemory() ([]memory.Entry, error) {
	if err := a.ready(); err != nil {
		return nil, err
	}
	return a.management.GlobalMemoryList()
}

func (a *Adapter) ReadGlobalMemory(key string) (memory.Entry, error) {
	if err := a.ready(); err != nil {
		return memory.Entry{}, err
	}
	return a.management.GlobalMemoryRead(key)
}

func (a *Adapter) WriteGlobalMemory(key, value string) error {
	if err := a.ready(); err != nil {
		return err
	}
	return a.management.GlobalMemoryWrite(key, value)
}

func (a *Adapter) DeleteGlobalMemory(key string) error {
	if err := a.ready(); err != nil {
		return err
	}
	return a.management.GlobalMemoryDelete(key)
}

func (a *Adapter) ListEnvironmentMemory(environmentID string) ([]memory.Entry, error) {
	if err := a.ready(); err != nil {
		return nil, err
	}
	return a.management.EnvironmentMemoryList(environmentID)
}

func (a *Adapter) ReadEnvironmentMemory(environmentID, key string) (memory.Entry, error) {
	if err := a.ready(); err != nil {
		return memory.Entry{}, err
	}
	return a.management.EnvironmentMemoryRead(environmentID, key)
}

func (a *Adapter) WriteEnvironmentMemory(environmentID, key, value string) error {
	if err := a.ready(); err != nil {
		return err
	}
	return a.management.EnvironmentMemoryWrite(environmentID, key, value)
}

func (a *Adapter) DeleteEnvironmentMemory(environmentID, key string) error {
	if err := a.ready(); err != nil {
		return err
	}
	return a.management.EnvironmentMemoryDelete(environmentID, key)
}
