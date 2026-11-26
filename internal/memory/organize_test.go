package memory

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"ai-dev-manager-v2/internal/model"
	"ai-dev-manager-v2/internal/store"
)

func str(s string) *string { return &s }

func TestOrganizeSearchAndPersistence(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	st := store.New(path)
	if err := st.Update(func(s *model.State) error {
		s.Environments = []model.Environment{{ID: "a"}, {ID: "b"}}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	s := New(st)
	for _, item := range []struct{ id, key, value string }{
		{"a", "build-old", "旧构建规则"}, {"a", "build-new", "新构建规则"}, {"b", "build-old", "other private"},
		{"a", "large", strings.Repeat("x", 10000)},
	} {
		if err := s.EnvironmentWrite(item.id, item.key, item.value); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.GlobalWrite("global-rule", "全局构建规则"); err != nil {
		t.Fatal(err)
	}
	r, err := s.EnvironmentSearch("a", "构建", 1, 1024)
	if err != nil || r.Total != 2 || r.Omitted != 1 || len(r.Entries) != 1 {
		t.Fatalf("search %+v %v", r, err)
	}
	r, err = s.EnvironmentSearch("a", "", 100, 256)
	b, _ := json.Marshal(r)
	if err != nil || len(b) > 256 || r.Total != 3 || r.Omitted < 1 {
		t.Fatalf("bounded search %s %v", b, err)
	}
	r, err = s.GlobalSearch("全局", 0, 0)
	if err != nil || r.Total != 1 || r.Entries[0].Key != "global-rule" {
		t.Fatalf("global %+v %v", r, err)
	}
	changes := []Change{{Key: "build-old", ExpectedValue: str("旧构建规则")}, {Key: "build-new", ExpectedValue: str("新构建规则"), Value: str("合并构建规则")}, {Key: "empty", Value: str("")}}
	preview, err := s.EnvironmentOrganize("a", changes, true)
	if err != nil || preview.Created != 1 || preview.Updated != 1 || preview.Deleted != 1 || !preview.DryRun {
		t.Fatalf("preview %+v %v", preview, err)
	}
	if _, err := s.EnvironmentRead("a", "build-old"); err != nil {
		t.Fatal("preview changed state")
	}
	bad := append([]Change(nil), changes...)
	bad[1].ExpectedValue = str("stale")
	if _, err := s.EnvironmentOrganize("a", bad, false); err == nil {
		t.Fatal("accepted stale input")
	}
	if _, err := s.EnvironmentRead("a", "build-old"); err != nil {
		t.Fatal("partial commit")
	}
	if _, err := s.EnvironmentOrganize("a", changes, false); err != nil {
		t.Fatal(err)
	}
	reopened := New(store.New(path))
	if _, err := reopened.EnvironmentRead("a", "build-old"); err == nil {
		t.Fatal("old memory persisted")
	}
	for _, want := range []struct{ id, key, value string }{{"a", "build-new", "合并构建规则"}, {"a", "empty", ""}, {"b", "build-old", "other private"}} {
		entry, err := reopened.EnvironmentRead(want.id, want.key)
		if err != nil || entry.Value != want.value {
			t.Fatalf("entry %+v %v", entry, err)
		}
	}
	if _, err := s.EnvironmentOrganize("a", changes, false); err == nil {
		t.Fatal("accepted repeated stale changes")
	}
	if _, err := s.EnvironmentOrganize("", []Change{{Key: "global-rule", Value: str("bad")}}, false); err == nil {
		t.Fatal("empty environment selected global")
	}
	if _, err := s.EnvironmentOrganize("missing", []Change{{Key: "x", Value: str("bad")}}, false); err == nil {
		t.Fatal("missing environment")
	}
	if _, err := s.EnvironmentOrganize("a", []Change{{Key: "dup", Value: str("a")}, {Key: " dup ", Value: str("b")}}, false); err == nil {
		t.Fatal("duplicate keys")
	}
	if _, err := s.GlobalOrganize([]Change{{Key: "global-rule", ExpectedValue: str("全局构建规则"), Value: str("更新")}}, false); err != nil {
		t.Fatal(err)
	}
	entry, err := reopened.GlobalRead("global-rule")
	if err != nil || entry.Value != "更新" {
		t.Fatalf("global %+v %v", entry, err)
	}
}
