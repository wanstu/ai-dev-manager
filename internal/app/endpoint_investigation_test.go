package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ai-dev-manager-v2/internal/model"
)

func TestInvestigateEndpointFindsExactAndMethodEvidence(t *testing.T) {
	service, environmentID := endpointInvestigationService(t, map[string]string{
		"routes/api.go": "package routes\n\nfunc register() {\n\trouter.GET(\"/api/users\", listUsers)\n}\n",
	})
	report, err := service.InvestigateEndpoint(environmentID, model.EndpointInvestigationRequest{Target: "https://example.test/api/users?debug=true", Method: "get"})
	if err != nil {
		t.Fatal(err)
	}
	if report.NormalizedPath != "/api/users" || report.Method != "GET" {
		t.Fatalf("normalized report = %+v", report)
	}
	if report.Confidence != "high" {
		t.Fatalf("confidence = %q, want high; report=%+v", report.Confidence, report)
	}
	if len(report.Evidence) == 0 || report.Evidence[0].Path != "routes/api.go" || report.Evidence[0].Line != 4 {
		t.Fatalf("evidence = %+v", report.Evidence)
	}
	if !hasEndpointReason(report.Evidence[0], "method_on_same_line") {
		t.Fatalf("method evidence missing: %+v", report.Evidence[0])
	}
}

func TestInvestigateEndpointFindsDynamicRouteCandidate(t *testing.T) {
	service, environmentID := endpointInvestigationService(t, map[string]string{
		"server/routes.go": "package server\n\nfunc register() {\n\tr.Handle(\"GET\", \"/api/users/{id}\", showUser)\n}\n",
	})
	report, err := service.InvestigateEndpoint(environmentID, model.EndpointInvestigationRequest{Target: "/api/users/123", Method: "GET"})
	if err != nil {
		t.Fatal(err)
	}
	if report.Confidence != "high" && report.Confidence != "medium" {
		t.Fatalf("confidence = %q; report=%+v", report.Confidence, report)
	}
	if len(report.Evidence) == 0 || report.Evidence[0].Matched != "/api/users/{id}" {
		t.Fatalf("dynamic evidence = %+v queries=%+v", report.Evidence, report.Queries)
	}
	if report.Evidence[0].Kind != "route_dynamic_candidate" {
		t.Fatalf("dynamic evidence kind = %+v", report.Evidence[0])
	}
}

func TestInvestigateEndpointReportsUncertaintyWhenNoEvidence(t *testing.T) {
	service, environmentID := endpointInvestigationService(t, map[string]string{
		"main.go": "package main\nfunc main() {}\n",
	})
	report, err := service.InvestigateEndpoint(environmentID, model.EndpointInvestigationRequest{Target: "/missing", Method: "POST"})
	if err != nil {
		t.Fatal(err)
	}
	if report.Confidence != "none" || len(report.Evidence) != 0 {
		t.Fatalf("expected no evidence, got %+v", report)
	}
	if !hasString(report.Uncertainties, "no_route_literal_evidence_found") || !hasString(report.Uncertainties, "method_not_confirmed") {
		t.Fatalf("uncertainties = %+v", report.Uncertainties)
	}
}

func endpointInvestigationService(t *testing.T, files map[string]string) (*Service, string) {
	t.Helper()
	root := t.TempDir()
	for path, content := range files {
		target := filepath.Join(root, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(target, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	service := New(filepath.Join(t.TempDir(), "state.json"))
	workspace, err := service.Workspaces.Add(root, "endpoint-investigation")
	if err != nil {
		t.Fatal(err)
	}
	environment, err := service.Environments.Create(workspace.ID, "endpoint-investigation", "")
	if err != nil {
		t.Fatal(err)
	}
	return service, environment.ID
}

func hasEndpointReason(evidence model.EndpointInvestigationEvidence, reason string) bool {
	return hasString(evidence.Reasons, reason)
}

func hasString(values []string, target string) bool {
	for _, value := range values {
		if strings.EqualFold(value, target) {
			return true
		}
	}
	return false
}
