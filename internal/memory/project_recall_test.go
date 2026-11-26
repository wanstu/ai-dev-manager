package memory

import (
	"ai-dev-manager-v2/internal/model"
	"ai-dev-manager-v2/internal/store"
	"path/filepath"
	"testing"
)

func TestMemoryProjectHandoff(t *testing.T) {
	st := store.New(filepath.Join(t.TempDir(), "state.json"))
	if err := st.Update(func(s *model.State) error {
		s.Environments = []model.Environment{{ID: "project"}, {ID: "other"}, {ID: "wt", Retention: model.ResourceRetention{Persistence: model.PersistenceTemporary}}, {ID: "nested", Retention: model.ResourceRetention{Persistence: model.PersistenceTemporary}}}
		s.ManagedWorktrees = []model.ManagedWorktree{{EnvironmentID: "wt", SourceEnvironmentID: "project"}, {EnvironmentID: "nested", SourceEnvironmentID: "wt"}}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	s := New(st)
	scope, err := s.ProjectScope("nested")
	if err != nil || scope.ProjectEnvironmentID != "project" || len(scope.SourceChain) != 3 {
		t.Fatalf("scope %+v %v", scope, err)
	}
	if err := s.EnvironmentWrite("nested", "decision", "verified"); err != nil {
		t.Fatal(err)
	}
	items := []HandoffItem{{Key: "decision", ExpectedRevision: 1, ExpectedValue: "verified"}}
	if _, err := s.Handoff("nested", "other", items, false); err == nil {
		t.Fatal("unrelated target accepted")
	}
	if _, err := s.Handoff("nested", "project", items, true); err != nil {
		t.Fatal(err)
	}
	if _, err := s.EnvironmentRead("project", "decision"); err == nil {
		t.Fatal("dry run wrote")
	}
	bad := append([]HandoffItem(nil), items...)
	bad = append(bad, HandoffItem{Key: "missing"})
	if _, err := s.Handoff("nested", "project", bad, false); err == nil {
		t.Fatal("invalid batch accepted")
	}
	if _, err := s.EnvironmentRead("project", "decision"); err == nil {
		t.Fatal("partial handoff")
	}
	if _, err := s.Handoff("nested", "project", items, false); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Handoff("nested", "project", items, false); err == nil {
		t.Fatal("overwrote target")
	}
	if err := s.EnvironmentDelete("project", "decision"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Handoff("nested", "project", items, false); err == nil {
		t.Fatal("revived deleted target")
	}
	items[0].TargetKey = "retained"
	if err := s.EnvironmentWrite("nested", "decision", "changed"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Handoff("nested", "project", items, false); err == nil {
		t.Fatal("stale handoff")
	}
	items[0].ExpectedRevision = 2
	items[0].ExpectedValue = "changed"
	if _, err := s.Handoff("nested", "project", items, false); err != nil {
		t.Fatal(err)
	}
	if err := st.Update(func(state *model.State) error {
		state.Environments = state.Environments[:2]
		state.ManagedWorktrees = nil
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	e, err := New(st).EnvironmentRead("project", "retained")
	if err != nil || e.Value != "changed" {
		t.Fatalf("handoff not durable %+v %v", e, err)
	}
}
func TestMemoryLexicalRecallAndSimilar(t *testing.T) {
	s := New(store.New(filepath.Join(t.TempDir(), "state.json")))
	for key, value := range map[string]string{"writer.lock": "常驻进程与锁生命周期独立", "other": "无关图形界面", "notes-a": "process writer lease lifetime independent management", "notes-b": "process writer lease lifetime independent admin management", "notes-c": "process writer lease lifetime independent management"} {
		if err := s.GlobalWrite(key, value); err != nil {
			t.Fatal(err)
		}
	}
	r, err := s.Recall("", true, "进程锁生命周期", 1, 4096)
	if err != nil || len(r.Entries) != 1 || r.Entries[0].Key != "writer.lock" {
		t.Fatalf("recall %+v %v", r, err)
	}
	d, err := s.Similar("", true, .75, 20)
	if err != nil || len(d.Pairs) != 2 {
		t.Fatalf("similar %+v %v", d, err)
	}
	for _, p := range d.Pairs {
		if p.FirstKey == "notes-a" && p.SecondKey == "notes-c" {
			t.Fatal("exact duplicate included")
		}
	}
	if _, err := s.Recall("", true, "", 0, 0); err == nil {
		t.Fatal("empty query accepted")
	}
	if _, err := s.Similar("", true, .2, 0); err == nil {
		t.Fatal("invalid threshold")
	}
}
