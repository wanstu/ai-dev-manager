package app

import (
	"context"
	"strings"
	"testing"

	"ai-dev-manager-v2/internal/catalog"
	"ai-dev-manager-v2/internal/model"
)

func TestInvestigationProviderAbsentKeepsStaticEndpointFallback(t *testing.T) {
	service, environmentID := endpointInvestigationService(t, map[string]string{
		"routes/api.go": "package routes\n\nfunc register() {\n\trouter.GET(\"/api/users\", listUsers)\n}\n",
	})

	providerReport, err := service.InvestigationProviderReport(context.Background(), environmentID)
	if err != nil {
		t.Fatal(err)
	}
	provider := requireInvestigationProviderFact(t, providerReport, InvestigationProviderGitNexusKey)
	if provider.State != model.CapabilityStateUnconfigured || provider.ReasonCode != "provider_not_configured" {
		t.Fatalf("provider fact = %+v", provider)
	}
	if provider.RequiresWriter {
		t.Fatalf("passive provider inspection must not require writer: %+v", provider)
	}
	if provider.Source != capabilitySourceStatic {
		t.Fatalf("provider source = %q, want %q", provider.Source, capabilitySourceStatic)
	}
	if !hasString(provider.Uncertainties, "provider_not_configured") || !hasString(provider.Uncertainties, "static_fallback_available") {
		t.Fatalf("provider uncertainties = %+v", provider.Uncertainties)
	}

	endpointReport, err := service.InvestigateEndpoint(environmentID, model.EndpointInvestigationRequest{Target: "/api/users", Method: "GET"})
	if err != nil {
		t.Fatal(err)
	}
	if len(endpointReport.Evidence) == 0 || endpointReport.Confidence == "none" {
		t.Fatalf("static endpoint fallback unavailable: %+v", endpointReport)
	}
}

func TestInvestigationProviderFailureIsLocalAndKeepsStaticEndpointFallback(t *testing.T) {
	service, environmentID := endpointInvestigationService(t, map[string]string{
		"routes/api.go": "package routes\n\nfunc register() {\n\trouter.POST(\"/api/orders\", createOrder)\n}\n",
	})
	gitNexus, err := service.MCPs.AddMCPConfig("GitNexus", catalog.MCPConfig{
		Transport:  catalog.MCPTransportStdio,
		AuthMode:   catalog.MCPAuthNone,
		Executable: "adm-v2-gitnexus-not-allowlisted",
		Args:       []string{"mcp"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.SetEnvironmentMCP(environmentID, gitNexus.ID, true); err != nil {
		t.Fatal(err)
	}

	providerReport, err := service.InvestigationProviderReport(context.Background(), environmentID)
	if err != nil {
		t.Fatal(err)
	}
	provider := requireInvestigationProviderFact(t, providerReport, InvestigationProviderGitNexusKey)
	if provider.State != model.CapabilityStateUnavailable || provider.ReasonCode != "executable_not_allowed" {
		t.Fatalf("provider failure was not localized: %+v", provider)
	}
	if !hasString(provider.Uncertainties, "provider_unavailable") || !hasString(provider.Uncertainties, "static_fallback_available") {
		t.Fatalf("provider uncertainties = %+v", provider.Uncertainties)
	}

	endpointReport, err := service.InvestigateEndpoint(environmentID, model.EndpointInvestigationRequest{Target: "/api/orders", Method: "POST"})
	if err != nil {
		t.Fatal(err)
	}
	if len(endpointReport.Evidence) == 0 || endpointReport.Confidence == "none" {
		t.Fatalf("provider failure broke static endpoint fallback: %+v", endpointReport)
	}
}

func TestPhpStormInvestigationProviderIsRecognized(t *testing.T) {
	service, environmentID := endpointInvestigationService(t, map[string]string{
		"app/Foo.php": "<?php\nclass Foo {}\n",
	})
	phpStorm, err := service.MCPs.AddMCPConfig("PhpStorm Code Intelligence", catalog.MCPConfig{
		Transport:  catalog.MCPTransportStdio,
		AuthMode:   catalog.MCPAuthNone,
		Executable: "phpstorm-code-intelligence-mcp-not-allowlisted",
		Args:       []string{"mcp"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.SetEnvironmentMCP(environmentID, phpStorm.ID, true); err != nil {
		t.Fatal(err)
	}

	report, err := service.InvestigationProviderReport(context.Background(), environmentID)
	if err != nil {
		t.Fatal(err)
	}
	provider := requireInvestigationProviderFact(t, report, InvestigationProviderPhpStormKey)
	if provider.State != model.CapabilityStateUnavailable || provider.ReasonCode != "executable_not_allowed" {
		t.Fatalf("PhpStorm provider fact = %+v", provider)
	}
	if got := provider.Evidence[0].Details["provider"]; got != InvestigationProviderPhpStorm {
		t.Fatalf("provider evidence = %q, want %q", got, InvestigationProviderPhpStorm)
	}
	tools := provider.Evidence[0].Details["read_only_provider_tools"]
	for _, want := range []string{"code_intelligence_query", "code_intelligence_references", "code_intelligence_hierarchy"} {
		if !strings.Contains(tools, want) {
			t.Fatalf("PhpStorm provider tools missing %q: %s", want, tools)
		}
	}
}

func requireInvestigationProviderFact(t *testing.T, report model.InvestigationProviderReport, key string) model.CapabilityFact {
	t.Helper()
	for _, provider := range report.Providers {
		if provider.Key == key {
			return provider
		}
	}
	t.Fatalf("missing provider fact %q in %+v", key, report.Providers)
	return model.CapabilityFact{}
}
