package main

import (
	"io/fs"
	"strings"
	"testing"
)

func TestExecUsageSurfaceFrontendBindings(t *testing.T) {
	assets, err := frontendAssets()
	if err != nil {
		t.Fatal(err)
	}
	index, err := fs.ReadFile(assets, "index.html")
	if err != nil {
		t.Fatal(err)
	}
	for _, required := range []string{
		"execUsageSurfaceSummary",
		"execUsageFilter",
		"execUsageSurfaceFilter",
		"execUsageTimeFilter",
		"execUsageVisibleCount",
		"execUsageTotalCount",
	} {
		if !strings.Contains(string(index), required) {
			t.Fatalf("desktop index missing exec usage marker %q", required)
		}
	}

	javascript, err := fs.ReadFile(assets, "app.js")
	if err != nil {
		t.Fatal(err)
	}
	for _, required := range []string{
		"execSurfaceEntries",
		"execSurfaceText",
		"surface_counts",
		"hourly_counts",
		"execRecentUsage",
		"execUsageMatchesSurface",
		"renderExecUsageFromSnapshot",
		"elements.execUsageFilter.addEventListener('input'",
		"elements.execUsageSurfaceFilter.addEventListener('change'",
		"elements.execUsageTimeFilter.addEventListener('change'",
		"本小时",
		"近24小时",
		"近期突增",
		"历史未归类",
		"execUsageSurfaceSummary",
	} {
		if !strings.Contains(string(javascript), required) {
			t.Fatalf("desktop app.js missing exec usage surface marker %q", required)
		}
	}
}
