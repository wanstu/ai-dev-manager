package app

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"ai-dev-manager-v2/internal/catalog"
	"ai-dev-manager-v2/internal/model"
)

func TestResourceRetentionCreationDefaultsAndCatalogUpdatesPreserveMetadata(t *testing.T) {
	service := New(filepath.Join(t.TempDir(), "state.json"))
	workspace, err := service.Workspaces.Add(t.TempDir(), "retention")
	if err != nil {
		t.Fatal(err)
	}
	environment, err := service.Environments.Create(workspace.ID, "durable-default", "")
	if err != nil {
		t.Fatal(err)
	}
	assertDurableCoreRetention(t, environment.Retention)

	expiresAt := time.Now().UTC().Add(time.Hour)
	mcp, err := service.MCPs.AddMCPConfigWithRetention("temporary-mcp", catalog.MCPConfig{
		Transport: catalog.MCPTransportStreamableHTTP,
		AuthMode:  catalog.MCPAuthNone,
		Endpoint:  "http://127.0.0.1:65534/mcp",
	}, model.ResourceRetention{
		Persistence:    model.PersistenceTemporary,
		CreatorSurface: "gateway",
		OwnerID:        "owner-a",
		ExpiresAt:      &expiresAt,
	})
	if err != nil {
		t.Fatal(err)
	}
	updated, err := service.MCPs.UpdateMCPConfig(mcp.ID, "temporary-mcp", catalog.MCPConfig{
		Transport: catalog.MCPTransportStreamableHTTP,
		AuthMode:  catalog.MCPAuthNone,
		Endpoint:  "http://127.0.0.1:65533/mcp",
	})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Retention.Persistence != model.PersistenceTemporary || updated.Retention.OwnerID != "owner-a" || updated.Retention.ExpiresAt == nil {
		t.Fatalf("mcp update lost retention metadata: before=%+v after=%+v", mcp.Retention, updated.Retention)
	}

	skillRoot := t.TempDir()
	if err := os.WriteFile(filepath.Join(skillRoot, "SKILL.md"), []byte("# Temporary Skill\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	source, err := service.Skills.AddSkillSourceWithRetention(skillRoot, nil, false, model.ResourceRetention{
		Persistence:    model.PersistenceTemporary,
		CreatorSurface: "gateway",
		OwnerID:        "owner-b",
		ExpiresAt:      &expiresAt,
	})
	if err != nil {
		t.Fatal(err)
	}
	refreshed, err := service.Skills.RefreshSkillSource(source.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(refreshed.Skills) != 1 {
		t.Fatalf("refreshed skills = %+v", refreshed.Skills)
	}
	if refreshed.Source.Retention.Persistence != model.PersistenceTemporary || refreshed.Skills[0].Retention.Persistence != model.PersistenceTemporary {
		t.Fatalf("source retention did not propagate to discovered skill: source=%+v skill=%+v", refreshed.Source.Retention, refreshed.Skills[0].Retention)
	}
}

func TestResourceRetentionReportIsConservativeAndDoesNotLeakPrivateMemory(t *testing.T) {
	service := New(filepath.Join(t.TempDir(), "state.json"))
	workspace, err := service.Workspaces.Add(t.TempDir(), "retention-report")
	if err != nil {
		t.Fatal(err)
	}
	expiredAt := time.Now().UTC().Add(-time.Hour)
	environment, err := service.Environments.CreateWithRetention(workspace.ID, "temporary-env", "", model.ResourceRetention{
		Persistence:    model.PersistenceTemporary,
		CreatorSurface: "gateway",
		OwnerID:        "owner-a",
		ExpiresAt:      &expiredAt,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Environments.AcquireWriter(environment.ID, "writer-a"); err != nil {
		t.Fatal(err)
	}
	if err := service.Memory.EnvironmentWrite(environment.ID, "secret", "retention-private-sentinel"); err != nil {
		t.Fatal(err)
	}

	source, err := service.Skills.AddSkillSourceWithRetention(t.TempDir(), nil, false, model.ResourceRetention{
		Persistence:    model.PersistenceTemporary,
		CreatorSurface: "gateway",
		OwnerID:        "owner-b",
		ExpiresAt:      &expiredAt,
	})
	if err != nil {
		t.Fatal(err)
	}
	durableMCP, err := service.MCPs.AddMCP("durable-mcp", "http://127.0.0.1:65534/mcp", false)
	if err != nil {
		t.Fatal(err)
	}

	report, err := service.ResourceRetentionReport(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	envItem := requireRetentionItem(t, report, model.RetentionResourceEnvironment, environment.ID)
	if envItem.CleanupEligible || envItem.CleanupState != model.RetentionCleanupBlocked {
		t.Fatalf("temporary environment should be blocked: %+v", envItem)
	}
	if !hasString(envItem.Blockers, "active_writer") || !hasString(envItem.Blockers, RetentionRuntimeObservationBlocker) {
		t.Fatalf("environment blockers = %+v", envItem.Blockers)
	}
	sourceItem := requireRetentionItem(t, report, model.RetentionResourceSkillSource, source.ID)
	if !sourceItem.CleanupEligible || sourceItem.CleanupState != model.RetentionCleanupEligible {
		t.Fatalf("expired unattached skill source should be a static cleanup candidate: %+v", sourceItem)
	}
	mcpItem := requireRetentionItem(t, report, model.RetentionResourceMCP, durableMCP.ID)
	if mcpItem.CleanupState != model.RetentionCleanupDurable || mcpItem.CleanupEligible || !hasString(mcpItem.Blockers, "resource_is_durable") {
		t.Fatalf("durable mcp retention = %+v", mcpItem)
	}

	encoded, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "retention-private-sentinel") {
		t.Fatalf("retention report leaked Environment-private Memory: %s", encoded)
	}
}

func TestResourceRetentionCleanupExecutesOnlySafeAppCatalogRecords(t *testing.T) {
	service := New(filepath.Join(t.TempDir(), "state.json"))
	expiredAt := time.Now().UTC().Add(-time.Hour)
	skillRoot := t.TempDir()
	if err := os.WriteFile(filepath.Join(skillRoot, "SKILL.md"), []byte("# Temporary Skill\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	source, err := service.Skills.AddSkillSourceWithRetention(skillRoot, nil, false, model.ResourceRetention{
		Persistence:    model.PersistenceTemporary,
		CreatorSurface: "gateway",
		OwnerID:        "owner-cleanup",
		ExpiresAt:      &expiredAt,
	})
	if err != nil {
		t.Fatal(err)
	}
	refreshed, err := service.Skills.RefreshSkillSource(source.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(refreshed.Skills) != 1 {
		t.Fatalf("refreshed skills = %+v", refreshed.Skills)
	}
	mcpEntry, err := service.MCPs.AddMCPConfigWithRetention("temporary-mcp-needs-owner", catalog.MCPConfig{
		Transport: catalog.MCPTransportStreamableHTTP,
		AuthMode:  catalog.MCPAuthNone,
		Endpoint:  "http://127.0.0.1:65534/mcp",
	}, model.ResourceRetention{
		Persistence:    model.PersistenceTemporary,
		CreatorSurface: "gateway",
		OwnerID:        "owner-cleanup",
		ExpiresAt:      &expiredAt,
	})
	if err != nil {
		t.Fatal(err)
	}

	dryRun, err := service.ResourceRetentionCleanup(context.Background(), model.ResourceRetentionCleanupRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if !dryRun.DryRun || !hasRetentionMutation(dryRun.WouldRemove, model.RetentionResourceSkillSource, source.ID) {
		t.Fatalf("dry-run cleanup plan = %+v", dryRun)
	}
	if hasRetentionMutation(dryRun.WouldRemove, model.RetentionResourceMCP, mcpEntry.ID) {
		t.Fatalf("app-only dry-run must not mark MCP removable without runtime owner evidence: %+v", dryRun)
	}
	mcpPreview := requireRetentionItem(t, dryRun.Report, model.RetentionResourceMCP, mcpEntry.ID)
	if !hasString(mcpPreview.Blockers, RetentionRuntimeObservationBlocker) {
		t.Fatalf("app-only mcp preview blockers = %+v", mcpPreview.Blockers)
	}
	if _, err := service.Skills.GetSkillSource(source.ID); err != nil {
		t.Fatalf("dry-run cleanup mutated state: %v", err)
	}

	execute, err := service.ResourceRetentionCleanup(context.Background(), model.ResourceRetentionCleanupRequest{Execute: true})
	if err != nil {
		t.Fatal(err)
	}
	if execute.DryRun || !hasRetentionMutation(execute.Removed, model.RetentionResourceSkillSource, source.ID) || !hasRetentionMutation(execute.Removed, model.RetentionResourceSkill, refreshed.Skills[0].ID) {
		t.Fatalf("execute cleanup result = %+v", execute)
	}
	if _, err := service.Skills.GetSkillSource(source.ID); err == nil {
		t.Fatalf("execute did not remove temporary skill source %s", source.ID)
	}
	if _, err := service.Skills.Get(refreshed.Skills[0].ID); err == nil {
		t.Fatalf("execute did not remove source-owned skill %s", refreshed.Skills[0].ID)
	}
	if _, err := service.MCPs.Get(mcpEntry.ID); err != nil {
		t.Fatalf("app-only execute must not remove MCP without runtime owner evidence: %v", err)
	}
}

func TestResourceRetentionMarkTemporaryAndPromoteDurable(t *testing.T) {
	service := New(filepath.Join(t.TempDir(), "state.json"))
	expiredAt := time.Now().UTC().Add(-time.Hour)

	standalone, err := service.Skills.Add("standalone", false)
	if err != nil {
		t.Fatal(err)
	}
	markedStandalone, err := service.MarkResourceTemporary(model.ResourceRetentionUpdateRequest{
		Kind:           model.RetentionResourceSkill,
		ID:             standalone.ID,
		OwnerID:        "owner-standalone",
		ExpiresAt:      &expiredAt,
		CreatorSurface: "gateway",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(markedStandalone.Updated) != 1 || markedStandalone.Updated[0].Retention.Persistence != model.PersistenceTemporary || markedStandalone.Updated[0].Retention.OwnerID != "owner-standalone" {
		t.Fatalf("standalone mark temporary result = %+v", markedStandalone)
	}

	skillRoot := t.TempDir()
	if err := os.WriteFile(filepath.Join(skillRoot, "SKILL.md"), []byte("# Source Owned\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	source, err := service.Skills.AddSkillSource(skillRoot, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	refreshed, err := service.Skills.RefreshSkillSource(source.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(refreshed.Skills) != 1 {
		t.Fatalf("refreshed source skills = %+v", refreshed.Skills)
	}
	sourceOwned := refreshed.Skills[0]
	if _, err := service.MarkResourceTemporary(model.ResourceRetentionUpdateRequest{
		Kind:      model.RetentionResourceSkill,
		ID:        sourceOwned.ID,
		OwnerID:   "owner-source-owned",
		ExpiresAt: &expiredAt,
	}); err == nil || !strings.Contains(err.Error(), "skill_source boundary") {
		t.Fatalf("direct source-owned skill retention update should be rejected, err=%v", err)
	}

	markedSource, err := service.MarkResourceTemporary(model.ResourceRetentionUpdateRequest{
		Kind:      model.RetentionResourceSkillSource,
		ID:        source.ID,
		OwnerID:   "owner-source",
		ExpiresAt: &expiredAt,
		Policy:    "test-expire",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !hasRetentionUpdate(markedSource.Updated, model.RetentionResourceSkillSource, source.ID, model.PersistenceTemporary) || !hasRetentionUpdate(markedSource.Updated, model.RetentionResourceSkill, sourceOwned.ID, model.PersistenceTemporary) {
		t.Fatalf("skill source temporary update did not cascade: %+v", markedSource)
	}
	storedSkill, err := service.Skills.Get(sourceOwned.ID)
	if err != nil {
		t.Fatal(err)
	}
	if storedSkill.Retention.Persistence != model.PersistenceTemporary || storedSkill.Retention.OwnerID != "owner-source" {
		t.Fatalf("source-owned skill retention did not persist cascade: %+v", storedSkill.Retention)
	}

	promoted, err := service.PromoteResourceRetention(model.ResourceRetentionUpdateRequest{Kind: model.RetentionResourceSkillSource, ID: source.ID})
	if err != nil {
		t.Fatal(err)
	}
	if !hasRetentionUpdate(promoted.Updated, model.RetentionResourceSkillSource, source.ID, model.PersistenceDurable) || !hasRetentionUpdate(promoted.Updated, model.RetentionResourceSkill, sourceOwned.ID, model.PersistenceDurable) {
		t.Fatalf("skill source durable promotion did not cascade: %+v", promoted)
	}
	storedSource, err := service.Skills.GetSkillSource(source.ID)
	if err != nil {
		t.Fatal(err)
	}
	if storedSource.Retention.Persistence != model.PersistenceDurable || storedSource.Retention.ExpiresAt != nil || storedSource.Retention.OwnerID != "" {
		t.Fatalf("promoted source retention = %+v", storedSource.Retention)
	}
}

func assertDurableCoreRetention(t *testing.T, retention model.ResourceRetention) {
	t.Helper()
	if retention.Persistence != model.PersistenceDurable || retention.CreatorSurface != "core" || retention.CreatedAt == nil {
		t.Fatalf("retention = %+v, want durable core with created_at", retention)
	}
}

func requireRetentionItem(t *testing.T, report model.ResourceRetentionReport, kind, id string) model.ResourceRetentionItem {
	t.Helper()
	for _, item := range report.Resources {
		if item.Kind == kind && item.ID == id {
			return item
		}
	}
	t.Fatalf("missing retention item %s/%s in %+v", kind, id, report.Resources)
	return model.ResourceRetentionItem{}
}

func hasRetentionMutation(items []model.ResourceRetentionCleanupMutation, kind, id string) bool {
	for _, item := range items {
		if item.Kind == kind && item.ID == id && item.Action == model.RetentionCleanupActionRemove {
			return true
		}
	}
	return false
}

func hasRetentionUpdate(items []model.ResourceRetentionUpdateItem, kind, id, persistence string) bool {
	for _, item := range items {
		if item.Kind == kind && item.ID == id && item.Retention.Persistence == persistence {
			return true
		}
	}
	return false
}
