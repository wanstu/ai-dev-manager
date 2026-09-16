package store

import (
	"os"
	"path/filepath"
	"testing"

	"ai-dev-manager-v2/internal/model"
)

func TestLoadNormalizesLegacyRetentionAsDurable(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	legacy := `{
  "version": 1,
  "workspaces": [],
  "environments": [{
    "environment_id": "env_legacy",
    "workspace_id": "ws_legacy",
    "name": "legacy-env",
    "root": "C:\\legacy",
    "state": "ready",
    "created_at": "2026-09-01T00:00:00Z",
    "updated_at": "2026-09-01T00:00:00Z",
    "last_activity_at": "2026-09-01T00:00:00Z"
  }],
  "mcps": [{
    "id": "mcp_legacy",
    "name": "legacy-mcp",
    "default_include_in_environment": false,
    "transport": "streamable-http",
    "auth_mode": "none",
    "endpoint": "http://127.0.0.1:9999/mcp",
    "health_policy": {}
  }],
  "skill_sources": [{
    "skill_source_id": "source_legacy",
    "root": "C:\\skills",
    "created_at": "2026-09-01T00:00:00Z",
    "updated_at": "2026-09-01T00:00:00Z"
  }],
  "skills": [{
    "id": "skill_legacy",
    "source_id": "source_legacy",
    "name": "legacy-skill"
  }],
  "global_memory": {}
}`
	if err := os.WriteFile(path, []byte(legacy), 0o600); err != nil {
		t.Fatal(err)
	}

	state, err := New(path).Load()
	if err != nil {
		t.Fatal(err)
	}
	for kind, retention := range map[string]model.ResourceRetention{
		"environment":  state.Environments[0].Retention,
		"mcp":          state.MCPs[0].Retention,
		"skill_source": state.SkillSources[0].Retention,
		"skill":        state.Skills[0].Retention,
	} {
		if retention.Persistence != model.PersistenceDurable || retention.CreatorSurface != "legacy" {
			t.Fatalf("%s retention = %+v, want durable legacy", kind, retention)
		}
	}
	if state.Environments[0].Retention.CreatedAt == nil || state.SkillSources[0].Retention.CreatedAt == nil {
		t.Fatalf("legacy created_at was not carried into retention metadata: env=%+v source=%+v", state.Environments[0].Retention, state.SkillSources[0].Retention)
	}
}
