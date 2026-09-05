package daemon

import (
	"context"
	"net"
	"strings"
	"testing"
	"time"

	"ai-dev-manager/internal/environment"
	"ai-dev-manager/internal/gateway"
	"ai-dev-manager/internal/testutil"
)

type daemonGatewayDiscovery struct{}

func (daemonGatewayDiscovery) Info() gateway.Info {
	return gateway.Info{Name: gateway.ProductName, Role: gateway.ProductRole, APIVersion: gateway.APIVersion}
}
func (daemonGatewayDiscovery) Workspaces() ([]gateway.WorkspaceSummary, error)  { return nil, nil }
func (daemonGatewayDiscovery) Environments() ([]environment.Environment, error) { return nil, nil }
func (daemonGatewayDiscovery) InspectEnvironment(context.Context, string) (environment.InspectResult, error) {
	return environment.InspectResult{}, nil
}
func (daemonGatewayDiscovery) CreateEnvironment(_ context.Context, request environment.CreateRequest) (environment.InspectResult, error) {
	return environment.InspectResult{Environment: environment.Environment{ID: "env_test", WorkspaceID: request.WorkspaceID, Name: request.Name}}, nil
}
func (daemonGatewayDiscovery) DestroyEnvironment(_ context.Context, id string, _ bool) (environment.Environment, error) {
	return environment.Environment{ID: id}, nil
}
func (daemonGatewayDiscovery) AcquireEnvironmentWriter(_ context.Context, id, owner string) (environment.Environment, error) {
	return environment.Environment{ID: id, Writer: &environment.WriterLease{Owner: owner}}, nil
}
func (daemonGatewayDiscovery) ReleaseEnvironmentWriter(id, _ string, _ bool) (environment.Environment, error) {
	return environment.Environment{ID: id}, nil
}
func (daemonGatewayDiscovery) InvokeEnvironmentRead(_ context.Context, id, operation string, input map[string]any) (any, error) {
	return map[string]any{"environment_id": id, "operation": operation, "input": input}, nil
}
func (daemonGatewayDiscovery) InvokeEnvironmentMutation(_ context.Context, id, owner, operation string, input map[string]any) (any, error) {
	return map[string]any{"environment_id": id, "writer_owner": owner, "operation": operation, "input": input}, nil
}
func (daemonGatewayDiscovery) DomainError(_ context.Context, _ error, env *environment.Environment, _ *environment.InspectResult) gateway.DomainError {
	result := gateway.DomainError{Code: "test_error", Message: "test error"}
	if env != nil {
		result.EnvironmentID = env.ID
	}
	return result
}

func TestGatewayOwnerPersistsConcreteDynamicListenAcrossReconcile(t *testing.T) {
	testutil.RequireNetworkAcceptance(t)
	root := t.TempDir()
	owner, err := NewGatewayOwner(root, daemonGatewayDiscovery{})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	started, err := owner.Up(ctx, "")
	if err != nil {
		t.Fatalf("Up() error = %v", err)
	}
	if started.State != GatewayRunning || !started.DesiredRunning || started.Listen == "" || isDynamicListen(started.Listen) || !strings.HasSuffix(started.Endpoint, "/mcp") {
		t.Fatalf("started gateway = %+v", started)
	}
	stored, err := NewGatewayStore(root).Load()
	if err != nil {
		t.Fatal(err)
	}
	if !stored.DesiredRunning || stored.Listen != started.Listen {
		t.Fatalf("stored desired = %+v, started=%+v", stored, started)
	}

	if err := owner.CloseObserved(ctx); err != nil {
		t.Fatalf("CloseObserved() error = %v", err)
	}
	restartedOwner, err := NewGatewayOwner(root, daemonGatewayDiscovery{})
	if err != nil {
		t.Fatal(err)
	}
	restarted, err := restartedOwner.Reconcile(ctx)
	if err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}
	if restarted.State != GatewayRunning || restarted.Listen != started.Listen || restarted.Endpoint != started.Endpoint {
		t.Fatalf("restarted gateway = %+v, want endpoint %s", restarted, started.Endpoint)
	}

	stopped, err := restartedOwner.Down(ctx)
	if err != nil {
		t.Fatalf("Down() error = %v", err)
	}
	if stopped.State != GatewayStopped || stopped.DesiredRunning || stopped.Listen != started.Listen {
		t.Fatalf("stopped gateway = %+v", stopped)
	}
	stored, err = NewGatewayStore(root).Load()
	if err != nil || stored.DesiredRunning || stored.Listen != started.Listen {
		t.Fatalf("stored stopped desired = %+v err=%v", stored, err)
	}
}

func TestGatewayOwnerDoesNotMovePersistedPortWhenOccupied(t *testing.T) {
	testutil.RequireNetworkAcceptance(t)
	root := t.TempDir()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	owner, _ := NewGatewayOwner(root, daemonGatewayDiscovery{})
	started, err := owner.Up(ctx, "")
	if err != nil {
		t.Fatal(err)
	}
	if err := owner.CloseObserved(ctx); err != nil {
		t.Fatal(err)
	}

	occupied, err := net.Listen("tcp", started.Listen)
	if err != nil {
		t.Fatalf("occupy persisted gateway port: %v", err)
	}
	defer occupied.Close()

	restartedOwner, _ := NewGatewayOwner(root, daemonGatewayDiscovery{})
	status, err := restartedOwner.Reconcile(ctx)
	if err != nil {
		t.Fatalf("Reconcile() store error = %v", err)
	}
	if status.State != GatewayError || !status.DesiredRunning || status.Listen != started.Listen || status.Endpoint != "" {
		t.Fatalf("port-conflict status = %+v", status)
	}
	stored, err := NewGatewayStore(root).Load()
	if err != nil || !stored.DesiredRunning || stored.Listen != started.Listen {
		t.Fatalf("port conflict changed desired state: %+v err=%v", stored, err)
	}
}

func TestGatewayOwnerRejectsNonLoopbackAndListenChangeWhileRunning(t *testing.T) {
	testutil.RequireNetworkAcceptance(t)
	root := t.TempDir()
	owner, _ := NewGatewayOwner(root, daemonGatewayDiscovery{})
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if _, err := owner.Up(ctx, "0.0.0.0:0"); err == nil {
		t.Fatal("non-loopback gateway unexpectedly accepted")
	}
	started, err := owner.Up(ctx, "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := owner.Up(ctx, "127.0.0.1:65530"); err == nil {
		t.Fatal("running gateway unexpectedly moved listen address")
	}
	if err := owner.CloseObserved(ctx); err != nil {
		t.Fatal(err)
	}
	_ = started
}
