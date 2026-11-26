package memory

import (
	"ai-dev-manager-v2/internal/model"
	"ai-dev-manager-v2/internal/store"
	"path/filepath"
	"testing"
	"time"
)

func TestMemoryValidityRecallReviewAndHistory(t *testing.T) {
	s := New(store.New(filepath.Join(t.TempDir(), "state.json")))
	past := time.Now().UTC().Add(-time.Hour)
	future := time.Now().UTC().Add(time.Hour)
	zero := int64(0)
	entries := map[string]model.MemoryMetadata{
		"active":   {ValidUntil: &future},
		"due":      {ReviewAfter: &past},
		"expired":  {ValidUntil: &past},
		"obsolete": {Status: "obsolete", ReviewNote: "old policy"},
		"archived": {Status: "archived"},
	}
	changes := []Change{}
	for k, m := range entries {
		changes = append(changes, Change{Key: k, Value: str("writer lock design"), Metadata: &m, ExpectedRevision: &zero})
	}
	if _, err := s.GlobalOrganize(changes, false); err != nil {
		t.Fatal(err)
	}
	if err := s.GlobalWrite("legacy-style", "writer lock design"); err != nil {
		t.Fatal(err)
	}
	r, err := s.Recall("", true, "writer lock", 20, 8192)
	if err != nil || r.Total != 3 || len(r.Entries) != 3 || r.Entries[2].Key != "due" {
		t.Fatalf("default recall %+v %v", r, err)
	}
	r, err = s.Recall("", true, "writer lock", 20, 8192, true)
	if err != nil || r.Total != 6 {
		t.Fatalf("inclusive recall %+v %v", r, err)
	}
	exact, err := s.GlobalSearch("writer", 20, 8192)
	if err != nil || exact.Total != 6 {
		t.Fatalf("explicit search %+v %v", exact, err)
	}
	queue, err := s.ReviewQueue("", true, "", 20)
	if err != nil || queue.Total != 4 || queue.Items[0].Validity != "expired" {
		t.Fatalf("queue %+v %v", queue, err)
	}
	queue, err = s.ReviewQueue("", true, "review_due", 20)
	if err != nil || queue.Total != 1 || queue.Items[0].Key != "due" {
		t.Fatalf("due queue %+v %v", queue, err)
	}
	e, err := s.GlobalRead("obsolete")
	if err != nil || e.Validity != "obsolete" {
		t.Fatalf("obsolete read %+v %v", e, err)
	}
	m := *e.Metadata
	m.Status = "active"
	m.ReviewAfter = &future
	m.ReviewNote = "verified again"
	if _, err := s.GlobalOrganize([]Change{{Key: e.Key, ExpectedRevision: &e.Revision, ExpectedValue: &e.Value, Value: &e.Value, Metadata: &m}}, false); err != nil {
		t.Fatal(err)
	}
	e, err = s.GlobalRead("obsolete")
	if err != nil || e.Validity != "active" || e.Revision != 2 {
		t.Fatalf("reviewed %+v %v", e, err)
	}
	e, err = s.Restore("", true, "obsolete", 1, 2)
	if err != nil || e.Validity != "obsolete" || e.Revision != 3 {
		t.Fatalf("restored validity %+v %v", e, err)
	}
	m.Status = "invalid"
	if _, err := s.GlobalOrganize([]Change{{Key: e.Key, ExpectedRevision: &e.Revision, ExpectedValue: &e.Value, Value: &e.Value, Metadata: &m}}, false); err == nil {
		t.Fatal("invalid status accepted")
	}
}
