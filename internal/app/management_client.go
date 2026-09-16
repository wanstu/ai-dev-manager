package app

import (
	"context"

	"ai-dev-manager-v2/internal/catalog"
	"ai-dev-manager-v2/internal/memory"
	"ai-dev-manager-v2/internal/model"
)

// The methods in this file expose app-level management operations without
// requiring callers to reach through Service fields. They are used by the CLI
// compatibility path and mirror the canonical Admin MCP client contract.
func (s *Service) WorkspaceList() ([]model.Workspace, error) {
	return s.Workspaces.List()
}

func (s *Service) WorkspaceInspect(id string) (model.Workspace, error) {
	return s.Workspaces.Get(id)
}

func (s *Service) WorkspaceAdd(path, name string) (model.Workspace, error) {
	return s.Workspaces.Add(path, name)
}

func (s *Service) WorkspaceRename(id, name string) (model.Workspace, error) {
	return s.Workspaces.Rename(id, name)
}

func (s *Service) WorkspaceRemove(id string) (model.Workspace, error) {
	return s.Workspaces.Remove(id)
}

func (s *Service) WorkspaceMCPSet(workspaceID, mcpID string, enabled bool) (model.Workspace, error) {
	return s.SetWorkspaceMCP(workspaceID, mcpID, enabled)
}

func (s *Service) WorkspaceSkillSet(workspaceID, skillID string, enabled bool) (model.Workspace, error) {
	return s.SetWorkspaceSkill(workspaceID, skillID, enabled)
}

func (s *Service) EnvironmentCreate(workspaceID, name, root string) (EnvironmentSummary, error) {
	env, err := s.Environments.Create(workspaceID, name, root)
	if err != nil {
		return EnvironmentSummary{}, err
	}
	return s.EnvironmentSummary(env.ID)
}

func (s *Service) EnvironmentList() ([]EnvironmentSummary, error) {
	return s.EnvironmentSummaries()
}

func (s *Service) EnvironmentInspect(id string) (EnvironmentInspection, error) {
	return s.InspectEnvironment(context.Background(), id)
}

func (s *Service) CapabilityReport(id string) (model.CapabilityReport, error) {
	return s.EnvironmentCapabilityReport(context.Background(), id)
}

func (s *Service) EnvironmentRename(id, name string) (EnvironmentSummary, error) {
	env, err := s.Environments.Rename(id, name)
	if err != nil {
		return EnvironmentSummary{}, err
	}
	return s.EnvironmentSummary(env.ID)
}

func (s *Service) EnvironmentRemove(id string) error {
	_, err := s.Environments.Remove(id)
	return err
}

func (s *Service) EnvironmentRemoveResult(id string) (model.Environment, error) {
	return s.Environments.Remove(id)
}

func (s *Service) EnvironmentMCPSet(environmentID, mcpID string, enabled bool) (EnvironmentSummary, error) {
	env, err := s.SetEnvironmentMCP(environmentID, mcpID, enabled)
	if err != nil {
		return EnvironmentSummary{}, err
	}
	return s.EnvironmentSummary(env.ID)
}

func (s *Service) EnvironmentSkillSet(environmentID, skillID string, enabled bool) (EnvironmentSummary, error) {
	env, err := s.SetEnvironmentSkill(environmentID, skillID, enabled)
	if err != nil {
		return EnvironmentSummary{}, err
	}
	return s.EnvironmentSummary(env.ID)
}

func (s *Service) VerifierAdd(environmentID string, definition model.VerifierDefinition) (model.VerifierDefinition, error) {
	return s.AddVerifier(environmentID, definition)
}

func (s *Service) VerifierList(environmentID string) ([]model.VerifierDefinition, error) {
	return s.ListVerifiers(environmentID)
}

func (s *Service) VerifierRemove(environmentID, verifierID string) (model.VerifierDefinition, error) {
	return s.RemoveVerifier(environmentID, verifierID)
}

func (s *Service) WriterAcquire(environmentID, owner string) (model.Environment, error) {
	return s.Environments.AcquireWriter(environmentID, owner)
}

func (s *Service) WriterHeartbeat(environmentID, owner string) (model.Environment, error) {
	return s.Environments.HeartbeatWriter(environmentID, owner)
}

func (s *Service) WriterRelease(environmentID, owner string, force bool) (model.Environment, error) {
	return s.Environments.ReleaseWriter(environmentID, owner, force)
}

func (s *Service) ExecAllow(executable string) ([]string, error) {
	if err := s.AllowExecutable(executable); err != nil {
		return nil, err
	}
	return s.AllowedExecutables()
}

func (s *Service) ExecRemove(executable string) ([]string, error) {
	if err := s.RemoveAllowedExecutable(executable); err != nil {
		return nil, err
	}
	return s.AllowedExecutables()
}

func (s *Service) ExecList() ([]string, error) {
	return s.AllowedExecutables()
}

func (s *Service) MCPAddConfig(name string, config catalog.MCPConfig) (model.MCPDefinition, error) {
	return s.MCPs.AddMCPConfig(name, config)
}

func (s *Service) MCPList() ([]model.MCPDefinition, error) {
	return s.MCPs.List()
}

func (s *Service) MCPImportPreview(input MCPImportInput) (MCPImportPreview, error) {
	return s.PreviewMCPImport(input)
}

func (s *Service) MCPImportApply(input MCPImportInput) (MCPImportApplyResult, error) {
	return s.ApplyMCPImport(input)
}

func (s *Service) MCPHealth(ctx context.Context, environmentID, mcpID string) (MCPHealthStatus, error) {
	return s.ProbeMCPHealth(ctx, environmentID, mcpID)
}

func (s *Service) MCPRemove(id string) error {
	return s.MCPs.Remove(id)
}

func (s *Service) MCPSetDefault(id string, enabled bool) (model.MCPDefinition, error) {
	return s.MCPs.SetDefault(id, enabled)
}

func (s *Service) SkillAdd(root, supportRoot string, defaultInclude bool) ([]model.CatalogEntry, error) {
	supportRoots := []string{}
	if supportRoot != "" {
		supportRoots = append(supportRoots, supportRoot)
	}
	return s.Skills.AddSkillRoot(root, supportRoots, defaultInclude)
}

func (s *Service) SkillList() ([]model.CatalogEntry, error) {
	return s.Skills.List()
}

func (s *Service) SkillSourceAdd(root string, supportRoots []string, defaultInclude bool) (model.SkillSource, error) {
	return s.Skills.AddSkillSource(root, supportRoots, defaultInclude)
}

func (s *Service) SkillSourceUpdate(id, root string, supportRoots []string, defaultInclude bool) (model.SkillSource, error) {
	return s.Skills.UpdateSkillSource(id, root, supportRoots, defaultInclude)
}

func (s *Service) SkillSourceList() ([]model.SkillSource, error) {
	return s.Skills.ListSkillSources()
}

func (s *Service) SkillSourceRefresh(id string) (catalog.SkillSourceRefreshResult, error) {
	return s.Skills.RefreshSkillSource(id)
}

func (s *Service) SkillSourceRemove(id string) (catalog.SkillSourceRefreshResult, error) {
	return s.Skills.RemoveSkillSource(id)
}

func (s *Service) SkillRemove(id string) error {
	return s.Skills.Remove(id)
}

func (s *Service) SkillSetDefault(id string, enabled bool) (model.CatalogEntry, error) {
	return s.Skills.SetDefault(id, enabled)
}

func (s *Service) GlobalMemoryList() ([]memory.Entry, error) {
	return s.Memory.GlobalList()
}

func (s *Service) GlobalMemoryRead(key string) (memory.Entry, error) {
	return s.Memory.GlobalRead(key)
}

func (s *Service) GlobalMemoryWrite(key, value string) error {
	return s.Memory.GlobalWrite(key, value)
}

func (s *Service) GlobalMemoryDelete(key string) error {
	return s.Memory.GlobalDelete(key)
}

func (s *Service) EnvironmentMemoryList(environmentID string) ([]memory.Entry, error) {
	return s.Memory.EnvironmentList(environmentID)
}

func (s *Service) EnvironmentMemoryRead(environmentID, key string) (memory.Entry, error) {
	return s.Memory.EnvironmentRead(environmentID, key)
}

func (s *Service) EnvironmentMemoryWrite(environmentID, key, value string) error {
	return s.Memory.EnvironmentWrite(environmentID, key, value)
}

func (s *Service) EnvironmentMemoryDelete(environmentID, key string) error {
	return s.Memory.EnvironmentDelete(environmentID, key)
}
