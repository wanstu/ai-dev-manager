package gatewayservice

import (
	"strings"
	"testing"
)

func TestRenderSystemdUnitPinsUserHomeExecutableAndListen(t *testing.T) {
	options := InstallOptions{
		User: TargetUser{
			Name:      "admin",
			Home:      "/home/admin",
			StatePath: "/home/admin/.config/ai-dev-manager-v2/state.json",
		},
		Listen:     "0.0.0.0:8001",
		Executable: "/usr/bin/adm",
	}
	unit, err := RenderSystemdUnit(options)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"# Managed by AI Dev Manager.",
		"# ADM-Unit-Version: 1",
		"# ADM-User: admin",
		"# ADM-Listen: 0.0.0.0:8001",
		"# ADM-StatePath: /home/admin/.config/ai-dev-manager-v2/state.json",
		"User=admin",
		`Environment="HOME=/home/admin"`,
		`Environment="ADM_V2_HOME=/home/admin/.config/ai-dev-manager-v2"`,
		`WorkingDirectory="/home/admin"`,
		`ExecStart="/usr/bin/adm" gateway start --listen "0.0.0.0:8001"`,
		"Restart=on-failure",
		"UMask=0077",
	} {
		if !strings.Contains(unit, want) {
			t.Fatalf("unit missing %q:\n%s", want, unit)
		}
	}
	parsed := parseManagedUnit(unit)
	if !parsed.Managed || parsed.UnitVersion != UnitVersion {
		t.Fatalf("parsed managed unit metadata = %+v", parsed)
	}
	if parsed.User != "admin" ||
		parsed.Listen != "0.0.0.0:8001" ||
		parsed.Executable != "/usr/bin/adm" ||
		parsed.StatePath != "/home/admin/.config/ai-dev-manager-v2/state.json" {
		t.Fatalf("parsed unit metadata = %+v", parsed)
	}
}

func TestRenderSystemdUnitRejectsInvalidInputs(t *testing.T) {
	base := InstallOptions{
		User: TargetUser{
			Name:      "admin",
			Home:      "/home/admin",
			StatePath: "/home/admin/.config/ai-dev-manager-v2/state.json",
		},
		Listen:     "0.0.0.0:8001",
		Executable: "/usr/bin/adm",
	}
	tests := []InstallOptions{
		{User: TargetUser{Name: "", Home: "/home/admin", StatePath: base.User.StatePath}, Listen: base.Listen, Executable: base.Executable},
		{User: base.User, Listen: "invalid", Executable: base.Executable},
		{User: base.User, Listen: base.Listen, Executable: "adm"},
		{User: TargetUser{Name: "admin", Home: "/home/admin", StatePath: "relative/state.json"}, Listen: base.Listen, Executable: base.Executable},
	}
	for _, input := range tests {
		if _, err := RenderSystemdUnit(input); err == nil {
			t.Fatalf("RenderSystemdUnit(%+v) unexpectedly succeeded", input)
		}
	}
}

func TestValidateManagedServiceStatusRejectsMissingAndForeignUnits(t *testing.T) {
	if err := validateManagedServiceStatus(Status{}); err == nil || !strings.Contains(err.Error(), "not installed") {
		t.Fatalf("missing service error = %v", err)
	}
	foreign := Status{Installed: true, UnitPath: UnitPath}
	if err := validateManagedServiceStatus(foreign); err == nil || !strings.Contains(err.Error(), "not managed by ADM") {
		t.Fatalf("foreign service error = %v", err)
	}
	if err := validateManagedServiceStatus(Status{Installed: true, Managed: true, UnitPath: UnitPath}); err != nil {
		t.Fatalf("managed service rejected: %v", err)
	}
}

func TestSystemdQuoteEscapesSpecifiersAndVariables(t *testing.T) {
	got := systemdQuote(`/opt/%i/$HOME/"adm"`)
	if got != `"/opt/%%i/$$HOME/\"adm\""` {
		t.Fatalf("systemdQuote()=%q", got)
	}
}
