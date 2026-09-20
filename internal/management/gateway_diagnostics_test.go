package management

import (
	"path/filepath"
	"runtime"
	"testing"

	"ai-dev-manager-v2/internal/app"
	"ai-dev-manager-v2/internal/gatewayservice"
)

func TestGatewayDiagnosticsReportsRuntimeAndSafeAccessState(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "state.json")
	service := New(app.New(statePath))

	diagnostics, err := service.GatewayDiagnostics()
	if err != nil {
		t.Fatal(err)
	}
	if diagnostics.ProcessPID <= 0 {
		t.Fatalf("expected process pid, got %d", diagnostics.ProcessPID)
	}
	if diagnostics.GOOS != runtime.GOOS || diagnostics.GOARCH != runtime.GOARCH {
		t.Fatalf("unexpected runtime %s/%s", diagnostics.GOOS, diagnostics.GOARCH)
	}
	if diagnostics.StatePath != statePath {
		t.Fatalf("state path = %q, want %q", diagnostics.StatePath, statePath)
	}
	if diagnostics.Readiness.Ready {
		t.Fatalf("fresh state should not be remote-ready: %+v", diagnostics.Readiness)
	}
	if diagnostics.Access.AdminAPIKeyConfigured || diagnostics.Access.AgentAPIKeyConfigured {
		t.Fatalf("fresh state should not report configured keys: %+v", diagnostics.Access)
	}
	if runtime.GOOS != "linux" && diagnostics.Service.Supported {
		t.Fatalf("non-linux runtime unexpectedly reports systemd support: %+v", diagnostics.Service)
	}
	if len(diagnostics.Issues) == 0 || diagnostics.Issues[0].Code != "remote_access_not_ready" {
		t.Fatalf("fresh diagnostics should explain remote readiness issue: %+v", diagnostics.Issues)
	}
}

func TestGatewayDiagnosticIssuesDetectSystemdRuntimeMismatch(t *testing.T) {
	issues := gatewayDiagnosticIssues(GatewayDiagnostics{
		ProcessUser: "admin",
		StatePath:   "/home/admin/.config/ai-dev-manager-v2/state.json",
		Executable:  "/usr/local/bin/adm",
		Readiness:   app.GatewayReadiness{Ready: true},
		Service: gatewayservice.Status{
			Supported:   true,
			Installed:   true,
			Managed:     true,
			UnitVersion: gatewayservice.UnitVersion,
			Active:      false,
			Enabled:     true,
			User:        "root",
			StatePath:   "/root/.config/ai-dev-manager-v2/state.json",
			Executable:  "/opt/adm/adm",
		},
	})
	codes := make(map[string]bool, len(issues))
	for _, issue := range issues {
		codes[issue.Code] = true
	}
	for _, required := range []string{
		"service_inactive",
		"service_user_mismatch",
		"service_state_path_mismatch",
		"service_executable_mismatch",
	} {
		if !codes[required] {
			t.Fatalf("diagnostic issues missing %q: %+v", required, issues)
		}
	}
}

func TestGatewayDiagnosticIssuesDetectOutdatedUnitVersion(t *testing.T) {
	issues := gatewayDiagnosticIssues(GatewayDiagnostics{
		Readiness: app.GatewayReadiness{Ready: true},
		Service: gatewayservice.Status{
			Supported:   true,
			Installed:   true,
			Managed:     true,
			UnitVersion: 0,
			Active:      true,
			Enabled:     true,
		},
	})
	for _, issue := range issues {
		if issue.Code == "service_unit_outdated" {
			return
		}
	}
	t.Fatalf("expected service_unit_outdated issue: %+v", issues)
}

func TestGatewayDiagnosticIssuesDetectUnmanagedService(t *testing.T) {
	issues := gatewayDiagnosticIssues(GatewayDiagnostics{
		Readiness: app.GatewayReadiness{Ready: true},
		Service: gatewayservice.Status{
			Supported: true,
			Installed: true,
			Managed:   false,
			UnitPath:  gatewayservice.UnitPath,
		},
	})
	if len(issues) != 1 || issues[0].Code != "service_unmanaged" {
		t.Fatalf("expected service_unmanaged issue: %+v", issues)
	}
}

func TestGatewayDiagnosticIssuesDetectDisabledService(t *testing.T) {
	issues := gatewayDiagnosticIssues(GatewayDiagnostics{
		Readiness: app.GatewayReadiness{Ready: true},
		Service: gatewayservice.Status{
			Supported:   true,
			Installed:   true,
			Managed:     true,
			UnitVersion: gatewayservice.UnitVersion,
			Active:      true,
			Enabled:     false,
		},
	})
	for _, issue := range issues {
		if issue.Code == "service_disabled" {
			return
		}
	}
	t.Fatalf("expected service_disabled issue: %+v", issues)
}

func TestGatewayDiagnosticIssuesDetectServicePIDMismatch(t *testing.T) {
	issues := gatewayDiagnosticIssues(GatewayDiagnostics{
		ProcessPID: 2222,
		Readiness:  app.GatewayReadiness{Ready: true},
		Service: gatewayservice.Status{
			Supported:   true,
			Installed:   true,
			Managed:     true,
			UnitVersion: gatewayservice.UnitVersion,
			Active:      true,
			Enabled:     true,
			PID:         1111,
		},
	})
	for _, issue := range issues {
		if issue.Code == "service_pid_mismatch" {
			return
		}
	}
	t.Fatalf("expected service_pid_mismatch issue: %+v", issues)
}
