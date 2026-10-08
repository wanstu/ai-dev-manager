package app

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"ai-dev-manager-v2/internal/projectanalysis"
)

func TestProjectIndexBackgroundJobCanBePolledWithoutSourceWriter(t *testing.T) {
	s, id, root := setupIndexEnvironment(t)
	const editingOwner = "agent-editor"
	if _, err := s.Environments.AcquireWriter(id, editingOwner); err != nil {
		t.Fatal(err)
	}
	first, err := s.StartProjectIndexJob(id)
	if err != nil || first.ID == "" {
		t.Fatalf("cannot enqueue: %+v %v", first, err)
	}
	second, err := s.StartProjectIndexJob(id)
	if err != nil {
		t.Fatal(err)
	}
	if second.State == "queued" || second.State == "running" {
		if second.ID != first.ID {
			t.Fatal("duplicate refresh started during existing job")
		}
	}
	var job ProjectIndexJob
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		job, err = s.ProjectIndexJobStatus(id)
		if err != nil {
			t.Fatal(err)
		}
		if job.State == "succeeded" || job.State == "failed" {
			break
		}
		time.Sleep(15 * time.Millisecond)
	}
	if job.State != "succeeded" {
		t.Fatalf("job state=%+v", job)
	}
	if job.FilesScanned == 0 || job.TotalFiles == 0 || job.FilesIndexed == 0 || job.Symbols == 0 || job.FinishedAt == nil {
		t.Fatalf("missing real progress and result counts: %+v", job)
	}
	lease, err := s.Environments.Get(id)
	if err != nil || lease.Writer == nil || lease.Writer.Owner != editingOwner {
		t.Fatalf("background job took source lock: %+v %v", lease.Writer, err)
	}
	if _, err := os.Stat(filepath.Join(root, ".adm")); !os.IsNotExist(err) {
		t.Fatalf("index wrote into project: %v", err)
	}
	q, err := s.ProjectIndexQuery(id, projectanalysis.IndexQuery{Query: "Service::first", Exact: true})
	if err != nil || q.Returned != 1 {
		t.Fatalf("background index query: %+v %v", q, err)
	}
	refreshed, err := s.StartProjectIndexJob(id)
	if err != nil || refreshed.ID == first.ID {
		t.Fatalf("completed job cannot restart: %+v %v", refreshed, err)
	}
}
func TestProjectIndexJobStatusIsIdleBeforeStart(t *testing.T) {
	s, id, _ := setupIndexEnvironment(t)
	st, err := s.ProjectIndexJobStatus(id)
	if err != nil || st.State != "idle" {
		t.Fatalf("idle state: %+v %v", st, err)
	}
}
