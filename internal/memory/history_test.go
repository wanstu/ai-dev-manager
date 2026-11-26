package memory

import (
	"ai-dev-manager-v2/internal/model"
	"ai-dev-manager-v2/internal/store"
	"path/filepath"
	"testing"
)

func TestMemoryHistoryMetadataRestoreAndScope(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	st := store.New(path)
	if err := st.Update(func(s *model.State) error {
		s.Environments = []model.Environment{{ID: "a", PrivateMemory: map[string]string{"decision": "legacy"}}, {ID: "b", PrivateMemory: map[string]string{"decision": "private b"}}}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	s := New(st)
	initial, err := s.EnvironmentRead("a", "decision")
	if err != nil || initial.Revision != 0 || initial.Metadata != nil {
		t.Fatalf("legacy %+v %v", initial, err)
	}
	zero := int64(0)
	change := Change{Key: "decision", ExpectedValue: str("legacy"), Value: str("new"), ExpectedRevision: &zero, Metadata: &model.MemoryMetadata{Category: "Decision", Tags: []string{"Build", "build"}, Importance: 5}}
	if _, err := s.EnvironmentOrganize("a", []Change{change}, true); err != nil {
		t.Fatal(err)
	}
	h, err := s.History("a", false, "decision", nil)
	if err != nil || h.CurrentRevision != 0 || len(h.Versions) != 1 {
		t.Fatalf("dry run %+v %v", h, err)
	}
	if _, err := s.EnvironmentOrganize("a", []Change{change}, false); err != nil {
		t.Fatal(err)
	}
	e, err := s.EnvironmentRead("a", "decision")
	if err != nil || e.Revision != 1 || e.Metadata.Category != "decision" || len(e.Metadata.Tags) != 1 || e.UpdatedAt == nil {
		t.Fatalf("metadata %+v %v", e, err)
	}
	r, err := s.EnvironmentSearch("a", "build", 0, 0)
	if err != nil || r.Total != 1 {
		t.Fatalf("tag search %+v %v", r, err)
	}
	if err := s.EnvironmentWrite("a", "decision", "new"); err != nil {
		t.Fatal(err)
	}
	h, err = s.History("a", false, "decision", &zero)
	if err != nil || h.CurrentRevision != 1 || h.Selected == nil || *h.Selected.Value != "legacy" {
		t.Fatalf("legacy history %+v %v", h, err)
	}
	if err := s.EnvironmentDelete("a", "decision"); err != nil {
		t.Fatal(err)
	}
	h, err = s.History("a", false, "decision", nil)
	if err != nil || h.CurrentRevision != 2 || !h.Versions[0].Deleted {
		t.Fatalf("deleted history %+v %v", h, err)
	}
	if _, err := s.Restore("a", false, "decision", 1, 1); err == nil {
		t.Fatal("stale restore accepted")
	}
	e, err = s.Restore("a", false, "decision", 1, 2)
	if err != nil || e.Revision != 3 || e.Value != "new" || e.Metadata.Importance != 5 {
		t.Fatalf("restore %+v %v", e, err)
	}
	reopened := New(store.New(path))
	e, err = reopened.EnvironmentRead("a", "decision")
	if err != nil || e.Revision != 3 {
		t.Fatalf("reopen %+v %v", e, err)
	}
	e, err = reopened.EnvironmentRead("b", "decision")
	if err != nil || e.Value != "private b" || e.Revision != 0 {
		t.Fatalf("scope %+v %v", e, err)
	}
	if err := s.EnvironmentWrite("a", "duplicate", "new"); err != nil {
		t.Fatal(err)
	}
	d, err := s.Duplicates("a", false)
	if err != nil || d.Total != 1 || len(d.Groups[0].Keys) != 2 {
		t.Fatalf("duplicates %+v %v", d, err)
	}
	for i := 0; i < 25; i++ {
		if err := s.EnvironmentWrite("a", "decision", string(rune('a'+i))); err != nil {
			t.Fatal(err)
		}
	}
	h, err = s.History("a", false, "decision", nil)
	if err != nil || len(h.Versions) != historyLimit+1 {
		t.Fatalf("retention %+v %v", h, err)
	}
	if err := st.Update(func(s *model.State) error { s.Environments = s.Environments[1:]; return nil }); err != nil {
		t.Fatal(err)
	}
	state, err := st.Load()
	if err != nil || state.EnvironmentMemoryRecords["a"] != nil {
		t.Fatalf("removed environment retained history: %v", err)
	}
}
