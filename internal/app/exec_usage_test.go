package app

import (
	"path/filepath"
	"testing"
	"time"

	"ai-dev-manager-v2/internal/model"
)

func TestExecUsageRecordsCountsWithoutCommandArguments(t *testing.T) {
	service := New(filepath.Join(t.TempDir(), "state.json"))
	if err := service.RecordExecUsage("env_a", "go", "exec"); err != nil {
		t.Fatal(err)
	}
	if err := service.RecordExecUsage("env_b", "GO", "verifier"); err != nil {
		t.Fatal(err)
	}
	items, err := service.ExecUsages()
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Fatalf("ExecUsages()=%+v", items)
	}
	item := items[0]
	if item.Count != 2 || item.LastEnvironmentID != "env_b" || item.LastSurface != "verifier" {
		t.Fatalf("usage=%+v", item)
	}
	if item.SurfaceCounts["exec"] != 1 || item.SurfaceCounts["verifier"] != 1 {
		t.Fatalf("surface counts=%+v", item.SurfaceCounts)
	}
	if len(item.HourlyCounts) != 1 ||
		item.HourlyCounts[0].Count != 2 ||
		item.HourlyCounts[0].SurfaceCounts["exec"] != 1 ||
		item.HourlyCounts[0].SurfaceCounts["verifier"] != 1 {
		t.Fatalf("hourly counts=%+v", item.HourlyCounts)
	}
	if item.FirstExecutedAt.IsZero() || item.LastExecutedAt.IsZero() {
		t.Fatalf("usage timestamps missing: %+v", item)
	}
}

func TestExecUsageLegacyCountsRemainUnattributed(t *testing.T) {
	service := New(filepath.Join(t.TempDir(), "state.json"))
	if err := service.Store.Update(func(state *model.State) error {
		state.ExecUsages = []model.ExecUsage{{
			Executable:  "go",
			Count:       5,
			LastSurface: "exec",
		}}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	items, err := service.ExecUsages()
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Fatalf("ExecUsages()=%+v", items)
	}
	if items[0].SurfaceCounts[execUsageUnattributedSurface] != 5 {
		t.Fatalf("legacy usage should remain unattributed: %+v", items[0])
	}
	if items[0].SurfaceCounts["exec"] != 0 {
		t.Fatalf("legacy usage was incorrectly inferred as exec: %+v", items[0])
	}

	if err := service.RecordExecUsage("env_a", "go", "exec"); err != nil {
		t.Fatal(err)
	}
	items, err = service.ExecUsages()
	if err != nil {
		t.Fatal(err)
	}
	if items[0].Count != 6 ||
		items[0].SurfaceCounts[execUsageUnattributedSurface] != 5 ||
		items[0].SurfaceCounts["exec"] != 1 {
		t.Fatalf("legacy + new usage merge=%+v", items[0])
	}
	if len(items[0].HourlyCounts) != 1 ||
		items[0].HourlyCounts[0].Count != 1 ||
		items[0].HourlyCounts[0].SurfaceCounts["exec"] != 1 {
		t.Fatalf("legacy history should not fabricate hourly buckets: %+v", items[0].HourlyCounts)
	}
}

func TestNormalizeExecUsageHoursMergesAndPrunes(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Hour)
	items := normalizeExecUsageHours([]model.ExecUsageHour{
		{Hour: now, Count: 2, SurfaceCounts: map[string]int{"exec": 2}},
		{Hour: now.Add(10 * time.Minute), Count: 2, SurfaceCounts: map[string]int{"verifier": 1}},
		{Hour: now.Add(-2 * time.Hour), Count: 3, SurfaceCounts: map[string]int{"run_start": 3}},
		{Hour: now.Add(-49 * time.Hour), Count: 9, SurfaceCounts: map[string]int{"old": 9}},
		{Hour: now.Add(time.Hour), Count: 7, SurfaceCounts: map[string]int{"future": 7}},
	}, now)
	if len(items) != 2 {
		t.Fatalf("hourly normalization=%+v", items)
	}
	if !items[0].Hour.Equal(now) ||
		items[0].Count != 4 ||
		items[0].SurfaceCounts["exec"] != 2 ||
		items[0].SurfaceCounts["verifier"] != 1 ||
		items[0].SurfaceCounts[execUsageUnattributedSurface] != 1 {
		t.Fatalf("current hourly bucket=%+v", items[0])
	}
	if !items[1].Hour.Equal(now.Add(-2*time.Hour)) || items[1].Count != 3 {
		t.Fatalf("previous hourly bucket=%+v", items[1])
	}
}

func TestNormalizeExecUsagesMergesSurfaceCounts(t *testing.T) {
	items := normalizeExecUsages([]model.ExecUsage{
		{Executable: "go", Count: 2, SurfaceCounts: map[string]int{"exec": 2}},
		{Executable: "GO", Count: 3, SurfaceCounts: map[string]int{"verifier": 2}},
	})
	if len(items) != 1 {
		t.Fatalf("normalized=%+v", items)
	}
	item := items[0]
	if item.Count != 5 ||
		item.SurfaceCounts["exec"] != 2 ||
		item.SurfaceCounts["verifier"] != 2 ||
		item.SurfaceCounts[execUsageUnattributedSurface] != 1 {
		t.Fatalf("normalized usage=%+v", item)
	}
}
