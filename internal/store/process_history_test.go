package store

import (
	"path/filepath"
	"testing"
	"time"
)

func TestProcessHistoryOwnershipAndRestart(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	a, b := New(path), New(path)
	now := time.Now().UTC()
	r := ProcessRecord{ID: "proc_test", EnvironmentID: "env_test", Executable: "test", StartedAt: now}
	h, err := a.BeginProcessRecord(r)
	if err != nil {
		t.Fatal(err)
	}
	defer h.Close()
	records, err := b.ProcessRecords()
	if err != nil || len(records) != 1 || !records[0].OwnerLive || records[0].ExitedAt != nil {
		t.Fatalf("active %+v %v", records, err)
	}
	h.Close()
	records, err = b.ProcessRecords()
	if err != nil || len(records) != 1 || records[0].OwnerLive || records[0].ExitedAt != nil {
		t.Fatalf("unconfirmed %+v %v", records, err)
	}
	r.ID = "proc_done"
	h2, err := a.BeginProcessRecord(r)
	if err != nil {
		t.Fatal(err)
	}
	defer h2.Close()
	end := now.Add(time.Second)
	code := 0
	r.ExitedAt = &end
	r.ExitCode = &code
	if err := h2.Update(r); err != nil {
		t.Fatal(err)
	}
	h2.Close()
	records, err = b.ProcessRecords()
	if err != nil || len(records) != 2 {
		t.Fatalf("history %+v %v", records, err)
	}
	if err := b.RemoveTerminalProcessRecords("env_test"); err != nil {
		t.Fatal(err)
	}
	records, err = a.ProcessRecords()
	if err != nil || len(records) != 1 || records[0].ID != "proc_test" {
		t.Fatalf("unconfirmed removed %+v %v", records, err)
	}
}
