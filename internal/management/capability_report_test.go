package management_test

import (
	"path/filepath"
	"testing"

	"ai-dev-manager-v2/internal/app"
	"ai-dev-manager-v2/internal/management"
	"ai-dev-manager-v2/internal/model"
)

func TestManagementEnvironmentCapabilityReportDelegatesToApplicationSchema(t *testing.T) {
	service := app.New(filepath.Join(t.TempDir(), "state.json"))
	workspace, err := service.Workspaces.Add(t.TempDir(), "management-capability")
	if err != nil {
		t.Fatal(err)
	}
	environment, err := service.Environments.Create(workspace.ID, "management-capability", "")
	if err != nil {
		t.Fatal(err)
	}
	report, err := management.New(service).EnvironmentCapabilityReport(environment.ID)
	if err != nil {
		t.Fatal(err)
	}
	if report.EnvironmentID != environment.ID || len(report.Facts) == 0 {
		t.Fatalf("capability report = %+v", report)
	}
	if fact := managementCapabilityFact(report, "environment.root"); fact.State != model.CapabilityStateAvailable || fact.Source != "app.static" {
		t.Fatalf("environment.root fact = %+v", fact)
	}
}

func managementCapabilityFact(report model.CapabilityReport, key string) model.CapabilityFact {
	for _, fact := range report.Facts {
		if fact.Key == key {
			return fact
		}
	}
	return model.CapabilityFact{}
}
