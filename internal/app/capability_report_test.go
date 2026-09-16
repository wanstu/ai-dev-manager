package app_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ai-dev-manager-v2/internal/app"
	"ai-dev-manager-v2/internal/catalog"
	"ai-dev-manager-v2/internal/model"
	"ai-dev-manager-v2/internal/runtime"
	"ai-dev-manager-v2/internal/verifier"
)

func TestCapabilityReportSurvivesOptionalFailuresAndDoesNotLeakSentinels(t *testing.T) {
	root := t.TempDir()
	statePath := filepath.Join(t.TempDir(), "state.json")
	service := app.New(statePath)
	ws, err := service.Workspaces.Add(root, "capability")
	if err != nil {
		t.Fatal(err)
	}
	env, err := service.Environments.Create(ws.ID, "diagnostics", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Environments.AcquireWriter(env.ID, "writer-a"); err != nil {
		t.Fatal(err)
	}

	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	if err := service.AllowExecutable(exe); err != nil {
		t.Fatal(err)
	}

	t.Setenv("ADM_CAP_ENDPOINT_SECRET", "adm-secret-endpoint")
	t.Setenv("ADM_CAP_HEADER_SECRET", "adm-secret-header")
	secureMCP, err := service.MCPs.AddMCPConfig("secure-http", catalog.MCPConfig{
		Transport:  catalog.MCPTransportStreamableHTTP,
		AuthMode:   catalog.MCPAuthHeaders,
		Endpoint:   "http://${ADM_CAP_ENDPOINT_SECRET}.example.test/mcp",
		HeaderRefs: map[string]string{"Authorization": "${ADM_CAP_HEADER_SECRET}"},
	})
	if err != nil {
		t.Fatal(err)
	}
	brokenStdioMCP, err := service.MCPs.AddMCPConfig("broken-stdio", catalog.MCPConfig{
		Transport:  catalog.MCPTransportStdio,
		AuthMode:   catalog.MCPAuthNone,
		Executable: "adm-v2-not-allowlisted-mcp",
	})
	if err != nil {
		t.Fatal(err)
	}
	removedMCP, err := service.MCPs.AddMCP("removed-mcp", "http://removed.example.test/mcp", false)
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{secureMCP.ID, brokenStdioMCP.ID, removedMCP.ID} {
		if _, err := service.SetEnvironmentMCP(env.ID, id, true); err != nil {
			t.Fatal(err)
		}
	}
	if err := service.MCPs.Remove(removedMCP.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Environments.SetMCP(env.ID, removedMCP.ID, true); err != nil {
		t.Fatal(err)
	}

	skillRoot := filepath.Join(t.TempDir(), "skills", "broken")
	if err := os.MkdirAll(skillRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	skillArtifact := filepath.Join(skillRoot, "SKILL.md")
	if err := os.WriteFile(skillArtifact, []byte("# Broken Skill\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	skills, err := service.Skills.AddSkillRoot(skillRoot, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(skills) != 1 {
		t.Fatalf("skills = %+v", skills)
	}
	if _, err := service.SetEnvironmentSkill(env.ID, skills[0].ID, true); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(skillArtifact); err != nil {
		t.Fatal(err)
	}
	removedSkill, err := service.Skills.Add("removed-skill", false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.SetEnvironmentSkill(env.ID, removedSkill.ID, true); err != nil {
		t.Fatal(err)
	}
	if err := service.Skills.Remove(removedSkill.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Environments.SetSkill(env.ID, removedSkill.ID, true); err != nil {
		t.Fatal(err)
	}

	sideEffectPath := filepath.Join(t.TempDir(), "verifier-ran.txt")
	t.Setenv("ADM_CAP_VERIFIER_SIDE_EFFECT", sideEffectPath)
	verifierOK, err := service.AddVerifier(env.ID, model.VerifierDefinition{
		Kind:       verifier.KindCustom,
		Enabled:    true,
		Executable: exe,
		Args:       []string{"-test.run=TestCapabilityReportVerifierSideEffectHelper$"},
	})
	if err != nil {
		t.Fatal(err)
	}
	verifierBroken, err := service.AddVerifier(env.ID, model.VerifierDefinition{
		Kind:       verifier.KindCustom,
		Enabled:    true,
		Executable: "adm-v2-not-allowlisted-verifier",
	})
	if err != nil {
		t.Fatal(err)
	}
	verifierDisabled, err := service.AddVerifier(env.ID, model.VerifierDefinition{
		Kind:       verifier.KindCustom,
		Enabled:    false,
		Executable: "adm-v2-disabled-verifier",
	})
	if err != nil {
		t.Fatal(err)
	}

	info, err := service.InspectEnvironment(context.Background(), env.ID)
	if err != nil {
		t.Fatal(err)
	}
	report := info.CapabilityReport
	if report.EnvironmentID != env.ID || len(report.Facts) == 0 {
		t.Fatalf("empty capability report: %+v", report)
	}
	if _, err := os.Stat(sideEffectPath); !os.IsNotExist(err) {
		t.Fatalf("capability report must not execute verifier helper, stat err=%v", err)
	}

	assertFact(t, report, runtime.CapabilityRead, model.CapabilityStateAvailable, "", false)
	writeFact := assertFact(t, report, runtime.CapabilityWrite, model.CapabilityStateAvailable, "", true)
	if !writerEvidenceHasOwner(writeFact, "writer-a") {
		t.Fatalf("write fact lacks current writer evidence: %+v", writeFact)
	}
	assertFact(t, report, runtime.CapabilityGitStatus, model.CapabilityStateUnavailable, "git_unsupported", false)
	assertFact(t, report, "mcp/"+secureMCP.ID, model.CapabilityStateAvailable, "", false)
	assertFact(t, report, "mcp/"+brokenStdioMCP.ID, model.CapabilityStateUnavailable, "executable_not_allowed", false)
	assertFact(t, report, "mcp/"+removedMCP.ID, model.CapabilityStateUnavailable, "unresolved_mcp", false)
	assertFact(t, report, "skill/"+skills[0].ID, model.CapabilityStateUnavailable, app.SkillAvailabilityArtifactMissing, false)
	assertFact(t, report, "skill/"+removedSkill.ID, model.CapabilityStateUnavailable, app.SkillAvailabilityUnresolved, false)
	assertFact(t, report, "verifier/"+verifierOK.ID, model.CapabilityStateAvailable, "", true)
	assertFact(t, report, "verifier/"+verifierBroken.ID, model.CapabilityStateUnavailable, "executable_not_allowed", true)
	assertFact(t, report, "verifier/"+verifierDisabled.ID, model.CapabilityStateDisabled, "verifier_disabled", true)

	encoded, err := json.Marshal(info)
	if err != nil {
		t.Fatal(err)
	}
	for _, sentinel := range []string{"adm-secret-endpoint", "adm-secret-header"} {
		if strings.Contains(string(encoded), sentinel) {
			t.Fatalf("capability report leaked sentinel %q: %s", sentinel, encoded)
		}
	}
}

func TestCapabilityReportSuppressesUnselectedCatalogFacts(t *testing.T) {
	root := t.TempDir()
	statePath := filepath.Join(t.TempDir(), "state.json")
	service := app.New(statePath)
	ws, err := service.Workspaces.Add(root, "bounded-capability-report")
	if err != nil {
		t.Fatal(err)
	}
	env, err := service.Environments.Create(ws.ID, "diagnostics", "")
	if err != nil {
		t.Fatal(err)
	}
	mcpEntry, err := service.MCPs.AddMCP("unselected-mcp", "http://example.test/mcp", false)
	if err != nil {
		t.Fatal(err)
	}
	skillEntry, err := service.Skills.Add("unselected-skill", false)
	if err != nil {
		t.Fatal(err)
	}

	report, err := service.EnvironmentCapabilityReport(context.Background(), env.ID)
	if err != nil {
		t.Fatal(err)
	}
	mcpCatalog := assertFact(t, report, "mcp.catalog", model.CapabilityStateUnconfigured, "no_mcps_enabled", false)
	if mcpCatalog.Evidence[len(mcpCatalog.Evidence)-1].Details["catalog_count"] != "1" {
		t.Fatalf("mcp catalog summary missing count: %+v", mcpCatalog)
	}
	skillCatalog := assertFact(t, report, "skill.catalog", model.CapabilityStateUnconfigured, "no_skills_enabled", false)
	if skillCatalog.Evidence[len(skillCatalog.Evidence)-1].Details["suppressed_disabled_fact_count"] != "1" {
		t.Fatalf("skill catalog summary missing suppression count: %+v", skillCatalog)
	}
	assertMissingFact(t, report, "mcp/"+mcpEntry.ID)
	assertMissingFact(t, report, "skill/"+skillEntry.ID)
}

func TestCapabilityReportHandlesTamperedRootWithoutErasingGlobalDiagnostics(t *testing.T) {
	root := t.TempDir()
	statePath := filepath.Join(t.TempDir(), "state.json")
	service := app.New(statePath)
	ws, err := service.Workspaces.Add(root, "tampered")
	if err != nil {
		t.Fatal(err)
	}
	env, err := service.Environments.Create(ws.ID, "root-missing", "")
	if err != nil {
		t.Fatal(err)
	}
	httpMCP, err := service.MCPs.AddMCP("http-still-diagnosable", "http://example.test/mcp", false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.SetEnvironmentMCP(env.ID, httpMCP.ID, true); err != nil {
		t.Fatal(err)
	}
	missingRoot := filepath.Join(root, "missing-root")
	if err := service.Store.Update(func(state *model.State) error {
		for i := range state.Environments {
			if state.Environments[i].ID == env.ID {
				state.Environments[i].Root = missingRoot
				return nil
			}
		}
		t.Fatalf("environment not found")
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	info, err := service.InspectEnvironment(context.Background(), env.ID)
	if err != nil {
		t.Fatalf("inspect must survive missing root: %v", err)
	}
	report := info.CapabilityReport
	assertFact(t, report, "environment.root", model.CapabilityStateUnavailable, "root_missing", false)
	assertFact(t, report, runtime.CapabilityRead, model.CapabilityStateUnavailable, "root_missing", false)
	assertFact(t, report, runtime.CapabilityExec, model.CapabilityStateUnavailable, "root_missing", true)
	assertFact(t, report, "mcp/"+httpMCP.ID, model.CapabilityStateAvailable, "", false)
	if contains(info.Capabilities, runtime.CapabilityRead) || contains(info.Capabilities, runtime.CapabilityExec) {
		t.Fatalf("legacy capabilities must be derived from available facts only: %v", info.Capabilities)
	}
}

func TestCapabilityReportVerifierSideEffectHelper(t *testing.T) {
	path := os.Getenv("ADM_CAP_VERIFIER_SIDE_EFFECT")
	if path == "" || !strings.Contains(strings.Join(os.Args, " "), "TestCapabilityReportVerifierSideEffectHelper") {
		return
	}
	if err := os.WriteFile(path, []byte("verifier executed"), 0o644); err != nil {
		panic(err)
	}
}

func assertFact(t *testing.T, report model.CapabilityReport, key string, state model.CapabilityState, reason string, requiresWriter bool) model.CapabilityFact {
	t.Helper()
	for _, fact := range report.Facts {
		if fact.Key != key {
			continue
		}
		if fact.State != state || fact.ReasonCode != reason || fact.RequiresWriter != requiresWriter {
			t.Fatalf("fact %s = state:%s reason:%q requires_writer:%v; want state:%s reason:%q requires_writer:%v; fact=%+v", key, fact.State, fact.ReasonCode, fact.RequiresWriter, state, reason, requiresWriter, fact)
		}
		return fact
	}
	t.Fatalf("missing fact %s in %+v", key, report.Facts)
	return model.CapabilityFact{}
}

func assertMissingFact(t *testing.T, report model.CapabilityReport, key string) {
	t.Helper()
	for _, fact := range report.Facts {
		if fact.Key == key {
			t.Fatalf("unexpected fact %s in %+v", key, report.Facts)
		}
	}
}

func writerEvidenceHasOwner(fact model.CapabilityFact, owner string) bool {
	for _, evidence := range fact.Evidence {
		if evidence.Kind == "writer_lease" && evidence.Details["owner"] == owner {
			return true
		}
	}
	return false
}
