package hostenv

import (
	"errors"
	"testing"
)

func TestManagerUsesInitialEnvironmentUntilFirstRefresh(t *testing.T) {
	manager := NewWithReader(map[string]string{
		"PROCESS_ONLY": "keep",
		"HOST_VALUE":   "before",
	}, func() (map[string]string, string, error) {
		return map[string]string{
			"HOST_VALUE": "after",
			"NEW_VALUE":  "added",
		}, "test_host", nil
	})

	if got, ok := manager.Lookup("HOST_VALUE"); !ok || got != "before" {
		t.Fatalf("pre-refresh HOST_VALUE = %q, %v; want before", got, ok)
	}
	status := manager.Status()
	if status.LastRefreshStatus != "not_refreshed" || status.Source != "process_environment" {
		t.Fatalf("pre-refresh status = %+v", status)
	}

	status = manager.Refresh()
	if status.LastRefreshStatus != "success" || status.Source != "test_host" || status.LastRefreshAt == nil {
		t.Fatalf("refresh status = %+v", status)
	}
	if got, ok := manager.Lookup("HOST_VALUE"); !ok || got != "after" {
		t.Fatalf("refreshed HOST_VALUE = %q, %v; want after", got, ok)
	}
	if got, ok := manager.Lookup("PROCESS_ONLY"); !ok || got != "keep" {
		t.Fatalf("PROCESS_ONLY = %q, %v; want preserved process-only value", got, ok)
	}
	if got, ok := manager.Lookup("NEW_VALUE"); !ok || got != "added" {
		t.Fatalf("NEW_VALUE = %q, %v; want added", got, ok)
	}
}

func TestManagerRefreshRemovesPreviouslyManagedKeys(t *testing.T) {
	refresh := 0
	manager := NewWithReader(map[string]string{"PROCESS_ONLY": "keep"}, func() (map[string]string, string, error) {
		refresh++
		if refresh == 1 {
			return map[string]string{"HOST_VALUE": "one", "NEW_VALUE": "one"}, "test_host", nil
		}
		return map[string]string{"NEW_VALUE": "two"}, "test_host", nil
	})
	manager.Refresh()
	manager.Refresh()

	if _, ok := manager.Lookup("HOST_VALUE"); ok {
		t.Fatal("HOST_VALUE remained after it was removed from the host source")
	}
	if got, ok := manager.Lookup("NEW_VALUE"); !ok || got != "two" {
		t.Fatalf("NEW_VALUE = %q, %v; want two", got, ok)
	}
	if got, ok := manager.Lookup("PROCESS_ONLY"); !ok || got != "keep" {
		t.Fatalf("PROCESS_ONLY = %q, %v; want preserved", got, ok)
	}
}

func TestManagerRefreshFailureDoesNotReplaceActiveEnvironment(t *testing.T) {
	manager := NewWithReader(map[string]string{"KEEP": "value"}, func() (map[string]string, string, error) {
		return nil, "test_host", errors.New("boom")
	})
	status := manager.Refresh()
	if status.LastRefreshStatus != "error" || status.LastRefreshError == "" {
		t.Fatalf("refresh failure status = %+v", status)
	}
	if got, ok := manager.Lookup("KEEP"); !ok || got != "value" {
		t.Fatalf("KEEP after failed refresh = %q, %v; want original value", got, ok)
	}
}
