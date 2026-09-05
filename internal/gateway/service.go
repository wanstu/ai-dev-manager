package gateway

import (
	"context"
	"errors"
	"sort"

	"ai-dev-manager/internal/environment"
	"ai-dev-manager/internal/model"
)

const (
	ProductName = "ai-dev-manager"
	ProductRole = "AI Coding Environment"
	APIVersion  = "v0.6"
)

type WorkspaceLister interface {
	List() ([]model.Workspace, error)
}

type EnvironmentSource interface {
	List() ([]environment.Environment, error)
	Inspect(context.Context, string) (environment.InspectResult, error)
	Create(context.Context, environment.CreateRequest) (environment.InspectResult, error)
	Destroy(context.Context, string, bool) (environment.Environment, error)
	AcquireWriter(context.Context, string, string) (environment.Environment, error)
	ReleaseWriter(string, string, bool) (environment.Environment, error)
	InvokeRead(context.Context, string, string, map[string]any) (any, error)
	InvokeMutation(context.Context, string, string, string, map[string]any) (any, error)
}

type Service struct {
	workspaces   WorkspaceLister
	environments EnvironmentSource
}

type Info struct {
	Name         string   `json:"name"`
	Role         string   `json:"role"`
	APIVersion   string   `json:"api_version"`
	Tools        []string `json:"tools"`
	Capabilities []string `json:"capabilities"`
	Notes        []string `json:"notes,omitempty"`
}

type WorkspaceSummary struct {
	WorkspaceID string `json:"workspace_id"`
	Path        string `json:"path"`
	ProfileID   string `json:"profile_id,omitempty"`
	RuntimeID   string `json:"runtime_id,omitempty"`
}

func NewService(workspaces WorkspaceLister, environments EnvironmentSource) (*Service, error) {
	if workspaces == nil || environments == nil {
		return nil, errors.New("gateway discovery dependencies are required")
	}
	return &Service{workspaces: workspaces, environments: environments}, nil
}

func (s *Service) Info() Info {
	return Info{
		Name:       ProductName,
		Role:       ProductRole,
		APIVersion: APIVersion,
		Tools: []string{
			"gateway_info",
			"workspace_list",
			"environment_list",
			"environment_inspect",
			"environment_create",
			"environment_destroy",
			"environment_writer_acquire",
			"environment_writer_release",
			"tree",
			"read",
			"search",
			"git_status",
			"git_diff",
			"git_branch",
			"write",
			"edit",
			"delete",
			"exec",
			"run_verifier",
			"run_verifiers",
		},
		Capabilities: []string{
			"workspace.discovery",
			"environment.discovery",
			"environment.inspect",
			"environment.lifecycle",
			"environment.writer",
			"environment.read",
			"environment.git.read",
			"environment.mutation",
			"environment.exec",
			"environment.verify",
		},
		Notes: []string{
			"This Gateway is the Agent-facing entry point for multiple isolated Environments.",
			"Per-Workspace Direct MCP remains available as a lower-level compatibility interface.",
		},
	}
}

func (s *Service) Workspaces() ([]WorkspaceSummary, error) {
	items, err := s.workspaces.List()
	if err != nil {
		return nil, err
	}
	result := make([]WorkspaceSummary, 0, len(items))
	for _, item := range items {
		result = append(result, WorkspaceSummary{
			WorkspaceID: item.ID,
			Path:        item.Path,
			ProfileID:   item.ProfileID,
			RuntimeID:   item.RuntimeID,
		})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].WorkspaceID < result[j].WorkspaceID })
	return result, nil
}

func (s *Service) Environments() ([]environment.Environment, error) {
	items, err := s.environments.List()
	if err != nil {
		return nil, err
	}
	result := make([]environment.Environment, len(items))
	copy(result, items)
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return result, nil
}

func (s *Service) InspectEnvironment(ctx context.Context, environmentID string) (environment.InspectResult, error) {
	return s.environments.Inspect(ctx, environmentID)
}

func (s *Service) CreateEnvironment(ctx context.Context, request environment.CreateRequest) (environment.InspectResult, error) {
	return s.environments.Create(ctx, request)
}

func (s *Service) DestroyEnvironment(ctx context.Context, environmentID string, force bool) (environment.Environment, error) {
	return s.environments.Destroy(ctx, environmentID, force)
}

func (s *Service) AcquireEnvironmentWriter(ctx context.Context, environmentID, owner string) (environment.Environment, error) {
	return s.environments.AcquireWriter(ctx, environmentID, owner)
}

func (s *Service) ReleaseEnvironmentWriter(environmentID, owner string, force bool) (environment.Environment, error) {
	return s.environments.ReleaseWriter(environmentID, owner, force)
}

func (s *Service) InvokeEnvironmentRead(ctx context.Context, environmentID, operation string, input map[string]any) (any, error) {
	return s.environments.InvokeRead(ctx, environmentID, operation, input)
}

func (s *Service) InvokeEnvironmentMutation(ctx context.Context, environmentID, writerOwner, operation string, input map[string]any) (any, error) {
	return s.environments.InvokeMutation(ctx, environmentID, writerOwner, operation, input)
}
