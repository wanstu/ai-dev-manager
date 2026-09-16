package app

import (
	"ai-dev-manager-v2/internal/model"
	"ai-dev-manager-v2/internal/workspace"
	"context"
)

func (s *Service) DiscoverWorkspace(ctx context.Context, workspaceID string, request model.DiscoveryRequest) (model.DiscoveryReport, error) {
	ws, err := s.Workspaces.Get(workspaceID)
	if err != nil {
		return model.DiscoveryReport{}, err
	}
	return workspace.ScanDiscovery(ctx, ws.Path, model.DiscoveryScope{WorkspaceID: ws.ID}, request, true)
}

func (s *Service) EnvironmentTreeDigest(ctx context.Context, environmentID string, request model.DiscoveryRequest) (model.DiscoveryReport, error) {
	rt, env, err := s.Runtime(environmentID)
	if err != nil {
		return model.DiscoveryReport{}, err
	}
	return workspace.ScanDiscovery(ctx, rt.Root(), model.DiscoveryScope{WorkspaceID: env.WorkspaceID, EnvironmentID: env.ID}, request, false)
}
