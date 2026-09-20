package cli

import (
	"encoding/json"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"ai-dev-manager-v2/internal/app"
	"ai-dev-manager-v2/internal/gatewayservice"
	"ai-dev-manager-v2/internal/model"
)

func TestGatewayServiceHelpListsManagedLifecycleCommands(t *testing.T) {
	output := captureStdout(t, func() {
		if err := runGatewayService([]string{"-h"}); err != nil {
			t.Fatal(err)
		}
	})
	for _, required := range []string{
		"gateway service install",
		"gateway service status",
		"gateway service start",
		"gateway service stop",
		"gateway service restart",
		"gateway service enable",
		"gateway service disable",
		"gateway service uninstall",
	} {
		if !strings.Contains(output, required) {
			t.Fatalf("gateway service help missing %q:\n%s", required, output)
		}
	}
}

func TestResolveGatewayInstallTargetFirstInstallDefaults(t *testing.T) {
	user, listen, err := resolveGatewayInstallTarget(gatewayservice.Status{}, "admin", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	if user != "admin" || listen != "0.0.0.0:8001" {
		t.Fatalf("resolved user/listen = %q %q", user, listen)
	}
}

func TestResolveGatewayInstallTargetUpgradePreservesManagedUserAndListen(t *testing.T) {
	existing := gatewayservice.Status{
		Installed: true,
		Managed:   true,
		User:      "admin",
		Listen:    "0.0.0.0:43137",
	}
	user, listen, err := resolveGatewayInstallTarget(existing, "", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	if user != "admin" || listen != "0.0.0.0:43137" {
		t.Fatalf("resolved user/listen = %q %q", user, listen)
	}
}

func TestResolveGatewayInstallTargetExplicitValuesOverrideUpgradeDefaults(t *testing.T) {
	existing := gatewayservice.Status{
		Installed: true,
		Managed:   true,
		User:      "admin",
		Listen:    "0.0.0.0:43137",
	}
	user, listen, err := resolveGatewayInstallTarget(existing, "gateway", "", 9001)
	if err != nil {
		t.Fatal(err)
	}
	if user != "gateway" || listen != "0.0.0.0:9001" {
		t.Fatalf("resolved user/listen = %q %q", user, listen)
	}
}

func TestResolveGatewayInstallTargetRequiresUserOnFirstInstall(t *testing.T) {
	if _, _, err := resolveGatewayInstallTarget(gatewayservice.Status{}, "", "", 0); err == nil {
		t.Fatal("first install unexpectedly accepted an empty user")
	}
}

func TestResolveGatewayInstallTargetRejectsListenAndPortTogether(t *testing.T) {
	if _, _, err := resolveGatewayInstallTarget(gatewayservice.Status{}, "admin", "0.0.0.0:8001", 9001); err == nil {
		t.Fatal("listen + port unexpectedly accepted")
	}
}

func TestResolveGatewayInstallHostsPreservesExistingUnlessExplicit(t *testing.T) {
	previous := []string{"adm.example.com", "10.0.0.8"}
	got := resolveGatewayInstallHosts(previous, "*", false)
	if !reflect.DeepEqual(got, previous) {
		t.Fatalf("preserved hosts = %#v, want %#v", got, previous)
	}
	got[0] = "changed"
	if previous[0] == "changed" {
		t.Fatal("preserved hosts reused the original backing slice")
	}

	explicit := resolveGatewayInstallHosts(previous, "*", true)
	if !reflect.DeepEqual(explicit, []string{"*"}) {
		t.Fatalf("explicit hosts = %#v", explicit)
	}
}

func TestBuildGatewayInstallPlanFirstInstallGeneratesMissingKeys(t *testing.T) {
	target := gatewayservice.TargetUser{
		Name:      "admin",
		StatePath: "/home/admin/.config/ai-dev-manager-v2/state.json",
	}
	plan := buildGatewayInstallPlan(
		gatewayservice.Status{},
		target,
		"0.0.0.0:8001",
		"/usr/bin/adm",
		model.GatewayAccessSettings{},
		[]string{"*"},
		false,
	)
	if plan.Action != "install" || plan.AdminKeyAction != "generate" || plan.AgentKeyAction != "generate" || !plan.ServiceEnabled {
		t.Fatalf("unexpected first install plan: %+v", plan)
	}
	assertGatewayPlanChanges(t, plan.Changes, "create_managed_service", "allowed_hosts", "admin_api_key", "agent_api_key")
}

func TestBuildGatewayInstallPlanUpgradePreservesConfigurationAndRestarts(t *testing.T) {
	statePath := "/home/admin/.config/ai-dev-manager-v2/state.json"
	existing := gatewayservice.Status{
		Installed:   true,
		Managed:     true,
		Enabled:     true,
		UnitVersion: gatewayservice.UnitVersion,
		User:        "admin",
		Listen:      "0.0.0.0:8001",
		StatePath:   statePath,
		Executable:  "/usr/bin/adm",
	}
	target := gatewayservice.TargetUser{Name: "admin", StatePath: statePath}
	previous := model.GatewayAccessSettings{
		AllowedHosts:    []string{"adm.example.com", "10.0.0.8"},
		AdminAPIKeyHash: "sha256:admin",
		AgentAPIKeyHash: "sha256:agent",
	}
	plan := buildGatewayInstallPlan(existing, target, existing.Listen, existing.Executable, previous, previous.AllowedHosts, false)
	if plan.Action != "upgrade" || plan.AdminKeyAction != "preserve" || plan.AgentKeyAction != "preserve" || !plan.ServiceEnabled {
		t.Fatalf("unexpected upgrade plan: %+v", plan)
	}
	if !reflect.DeepEqual(plan.Changes, []string{"restart_service"}) {
		t.Fatalf("upgrade changes = %#v", plan.Changes)
	}
}

func TestBuildGatewayInstallPlanReportsRotationAndUnitUpgrade(t *testing.T) {
	statePath := "/home/admin/.config/ai-dev-manager-v2/state.json"
	existing := gatewayservice.Status{
		Installed:  true,
		Managed:    true,
		User:       "admin",
		Listen:     "0.0.0.0:8001",
		StatePath:  statePath,
		Executable: "/usr/bin/adm",
	}
	target := gatewayservice.TargetUser{Name: "admin", StatePath: statePath}
	previous := model.GatewayAccessSettings{
		AllowedHosts:    []string{"*"},
		AdminAPIKeyHash: "sha256:admin",
		AgentAPIKeyHash: "sha256:agent",
	}
	plan := buildGatewayInstallPlan(existing, target, existing.Listen, existing.Executable, previous, previous.AllowedHosts, true)
	if plan.AdminKeyAction != "rotate" || plan.AgentKeyAction != "rotate" {
		t.Fatalf("rotation plan = %+v", plan)
	}
	if plan.ServiceEnabled {
		t.Fatalf("upgrade should preserve disabled service state: %+v", plan)
	}
	assertGatewayPlanChanges(t, plan.Changes, "unit_version", "restart_service", "admin_api_key", "agent_api_key")
	for _, unexpected := range []string{"enable_service"} {
		for _, change := range plan.Changes {
			if change == unexpected {
				t.Fatalf("upgrade unexpectedly changes enable state: %+v", plan)
			}
		}
	}
}

func assertGatewayPlanChanges(t *testing.T, got []string, required ...string) {
	t.Helper()
	have := make(map[string]bool, len(got))
	for _, item := range got {
		have[item] = true
	}
	for _, item := range required {
		if !have[item] {
			t.Fatalf("plan changes %#v missing %q", got, item)
		}
	}
}

func TestValidateGatewayServiceStartReadinessRejectsMissingForeignAndIncompleteState(t *testing.T) {
	if err := validateGatewayServiceStartReadiness(gatewayservice.Status{}); err == nil {
		t.Fatal("missing service unexpectedly passed readiness")
	}
	if err := validateGatewayServiceStartReadiness(gatewayservice.Status{Installed: true, UnitPath: gatewayservice.UnitPath}); err == nil {
		t.Fatal("foreign service unexpectedly passed readiness")
	}

	statePath := filepath.Join(t.TempDir(), "state.json")
	status := gatewayservice.Status{
		Installed: true,
		Managed:   true,
		UnitPath:  gatewayservice.UnitPath,
		Listen:    "0.0.0.0:8001",
		StatePath: statePath,
	}
	if err := validateGatewayServiceStartReadiness(status); err == nil || !strings.Contains(err.Error(), "Host Policy") {
		t.Fatalf("incomplete remote state error = %v", err)
	}
	service := app.New(statePath)
	if _, err := service.SetupGatewayRemote([]string{"adm.example.com"}, false); err != nil {
		t.Fatal(err)
	}
	if err := validateGatewayServiceStartReadiness(status); err != nil {
		t.Fatalf("ready managed service rejected: %v", err)
	}
}

func TestGatewayInstallPlanDoesNotExposeStoredKeyHashes(t *testing.T) {
	const adminHash = "sha256:ADMIN_SECRET_HASH_MARKER"
	const agentHash = "sha256:AGENT_SECRET_HASH_MARKER"
	statePath := "/home/admin/.config/ai-dev-manager-v2/state.json"
	plan := buildGatewayInstallPlan(
		gatewayservice.Status{
			Installed:   true,
			Managed:     true,
			UnitVersion: gatewayservice.UnitVersion,
			User:        "admin",
			Listen:      "0.0.0.0:8001",
			StatePath:   statePath,
			Executable:  "/usr/bin/adm",
		},
		gatewayservice.TargetUser{Name: "admin", StatePath: statePath},
		"0.0.0.0:8001",
		"/usr/bin/adm",
		model.GatewayAccessSettings{
			AllowedHosts:    []string{"*"},
			AdminAPIKeyHash: adminHash,
			AgentAPIKeyHash: agentHash,
		},
		[]string{"*"},
		false,
	)
	data, err := json.Marshal(plan)
	if err != nil {
		t.Fatal(err)
	}
	output := string(data)
	for _, secret := range []string{adminHash, agentHash, "api_key_hash"} {
		if strings.Contains(output, secret) {
			t.Fatalf("dry-run plan exposed stored key material: %s", output)
		}
	}
}
