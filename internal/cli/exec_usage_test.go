package cli

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"ai-dev-manager-v2/internal/app"
	"ai-dev-manager-v2/internal/model"
)

func TestExecUsageDefaultOutputRemainsFullArray(t *testing.T) {
	service := app.New(filepath.Join(t.TempDir(), "state.json"))
	if err := service.RecordExecUsage("env_a", "go", "exec"); err != nil {
		t.Fatal(err)
	}
	output := captureStdout(t, func() {
		if err := runExec(service, []string{"usage"}); err != nil {
			t.Fatal(err)
		}
	})
	var items []model.ExecUsage
	if err := json.Unmarshal([]byte(output), &items); err != nil {
		t.Fatalf("decode default exec usage: %v\n%s", err, output)
	}
	if len(items) != 1 || items[0].Executable != "go" {
		t.Fatalf("default exec usage=%+v", items)
	}
	if items[0].SurfaceCounts["exec"] != 1 || len(items[0].HourlyCounts) != 1 {
		t.Fatalf("default exec usage missing audit detail: %+v", items[0])
	}
}

func TestExecUsageRecentViewFiltersSurfaceAndSummarizes(t *testing.T) {
	service := app.New(filepath.Join(t.TempDir(), "state.json"))
	for _, entry := range []struct {
		executable string
		surface    string
	}{
		{"go", "exec"},
		{"go", "exec"},
		{"go", "verifier"},
		{"node", "verifier"},
	} {
		if err := service.RecordExecUsage("env_a", entry.executable, entry.surface); err != nil {
			t.Fatal(err)
		}
	}

	output := captureStdout(t, func() {
		if err := runExec(service, []string{"usage", "--recent-hours", "24", "--surface", "EXEC"}); err != nil {
			t.Fatal(err)
		}
	})
	var report execUsageRecentReport
	if err := json.Unmarshal([]byte(output), &report); err != nil {
		t.Fatalf("decode recent exec usage: %v\n%s", err, output)
	}
	if report.RecentHours != 24 || report.Surface != "exec" || report.ExecutableCount != 1 {
		t.Fatalf("recent report=%+v", report)
	}
	if report.RecentCount != 2 || report.CurrentHourCount != 2 {
		t.Fatalf("surface-filtered recent totals=%+v", report)
	}
	if len(report.Items) != 1 || report.Items[0].Executable != "go" {
		t.Fatalf("recent items=%+v", report.Items)
	}
	if report.Items[0].RecentCount != 3 ||
		report.Items[0].SelectedSurfaceCount != 2 ||
		report.Items[0].RecentSurfaceCounts["verifier"] != 1 {
		t.Fatalf("recent go item=%+v", report.Items[0])
	}
}

func TestExecUsageSurfaceOnlyKeepsCompatibleItemShape(t *testing.T) {
	service := app.New(filepath.Join(t.TempDir(), "state.json"))
	if err := service.RecordExecUsage("env_a", "go", "exec"); err != nil {
		t.Fatal(err)
	}
	if err := service.RecordExecUsage("env_a", "node", "verifier"); err != nil {
		t.Fatal(err)
	}
	output := captureStdout(t, func() {
		if err := runExec(service, []string{"stats", "--surface", "verifier"}); err != nil {
			t.Fatal(err)
		}
	})
	var items []model.ExecUsage
	if err := json.Unmarshal([]byte(output), &items); err != nil {
		t.Fatalf("decode surface-filtered usage: %v\n%s", err, output)
	}
	if len(items) != 1 || items[0].Executable != "node" {
		t.Fatalf("surface-filtered items=%+v", items)
	}
}

func TestExecUsageRecentHoursValidationAndHelp(t *testing.T) {
	service := app.New(filepath.Join(t.TempDir(), "state.json"))
	if err := runExec(service, []string{"usage", "--recent-hours", "49"}); err == nil || !strings.Contains(err.Error(), "0..48") {
		t.Fatalf("invalid recent-hours error=%v", err)
	}
	output := captureStdout(t, func() {
		if err := runExec(nil, []string{"usage", "-h"}); err != nil {
			t.Fatal(err)
		}
	})
	for _, required := range []string{"--recent-hours", "--surface", "保持兼容"} {
		if !strings.Contains(output, required) {
			t.Fatalf("exec usage help missing %q:\n%s", required, output)
		}
	}
}
