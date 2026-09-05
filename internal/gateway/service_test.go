package gateway

import (
	"context"
	"testing"
	"time"

	"ai-dev-manager/internal/environment"
	"ai-dev-manager/internal/model"
)

type fakeWorkspaceLister struct {
	items []model.Workspace
}

func (f fakeWorkspaceLister) List() ([]model.Workspace, error) {
	return append([]model.Workspace(nil), f.items...), nil
}

type fakeEnvironmentSource struct {
	items      []environment.Environment
	inspection environment.InspectResult
	inspectID  string
}

func (f *fakeEnvironmentSource) List() ([]environment.Environment, error) {
	return append([]environment.Environment(nil), f.items...), nil
}

func (f *fakeEnvironmentSource) Inspect(_ context.Context, id string) (environment.InspectResult, error) {
	f.inspectID = id
	return f.inspection, nil
}

func (f *fakeEnvironmentSource) Create(_ context.Context, request environment.CreateRequest) (environment.InspectResult, error) {
	return environment.InspectResult{Environment: environment.Environment{ID: "env_created", WorkspaceID: request.WorkspaceID, Name: request.Name}}, nil
}

func (f *fakeEnvironmentSource) Destroy(_ context.Context, id string, _ bool) (environment.Environment, error) {
	return environment.Environment{ID: id}, nil
}

func (f *fakeEnvironmentSource) AcquireWriter(_ context.Context, id, owner string) (environment.Environment, error) {
	return environment.Environment{ID: id, Writer: &environment.WriterLease{Owner: owner}}, nil
}

func (f *fakeEnvironmentSource) ReleaseWriter(id, _ string, _ bool) (environment.Environment, error) {
	return environment.Environment{ID: id}, nil
}

func (f *fakeEnvironmentSource) InvokeRead(_ context.Context, id, operation string, input map[string]any) (any, error) {
	return map[string]any{"environment_id": id, "operation": operation, "input": input}, nil
}

func (f *fakeEnvironmentSource) InvokeMutation(_ context.Context, id, owner, operation string, input map[string]any) (any, error) {
	return map[string]any{"environment_id": id, "writer_owner": owner, "operation": operation, "input": input}, nil
}

func TestServiceDiscoveryUsesRegistryAndEnvironmentSourceWithoutMutatingActivity(t *testing.T) {
	activity := time.Date(2026, 9, 5, 1, 2, 3, 0, time.UTC)
	envSource := &fakeEnvironmentSource{
		items: []environment.Environment{
			{ID: "env_b", WorkspaceID: "ws_b", LastActivityAt: activity},
			{ID: "env_a", WorkspaceID: "ws_a", LastActivityAt: activity},
		},
		inspection: environment.InspectResult{Environment: environment.Environment{ID: "env_a", LastActivityAt: activity}},
	}
	service, err := NewService(fakeWorkspaceLister{items: []model.Workspace{
		{ID: "ws_b", Path: `D:\b`, RuntimeID: "native"},
		{ID: "ws_a", Path: `D:\a`, ProfileID: "default"},
	}}, envSource)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	workspaces, err := service.Workspaces()
	if err != nil {
		t.Fatalf("Workspaces() error = %v", err)
	}
	if len(workspaces) != 2 || workspaces[0].WorkspaceID != "ws_a" || workspaces[1].WorkspaceID != "ws_b" {
		t.Fatalf("workspace summaries = %+v", workspaces)
	}

	environments, err := service.Environments()
	if err != nil {
		t.Fatalf("Environments() error = %v", err)
	}
	if len(environments) != 2 || environments[0].ID != "env_a" || environments[1].ID != "env_b" {
		t.Fatalf("environment summaries = %+v", environments)
	}
	if !environments[0].LastActivityAt.Equal(activity) || !envSource.items[0].LastActivityAt.Equal(activity) {
		t.Fatal("environment_list mutated activity")
	}

	inspection, err := service.InspectEnvironment(context.Background(), "env_a")
	if err != nil {
		t.Fatalf("InspectEnvironment() error = %v", err)
	}
	if envSource.inspectID != "env_a" || inspection.Environment.ID != "env_a" || !inspection.Environment.LastActivityAt.Equal(activity) {
		t.Fatalf("inspection delegation = %+v id=%q", inspection, envSource.inspectID)
	}

	info := service.Info()
	if info.Role != ProductRole || info.APIVersion != APIVersion || len(info.Tools) != 20 {
		t.Fatalf("gateway info = %+v", info)
	}
}
