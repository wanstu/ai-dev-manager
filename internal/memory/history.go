package memory

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"time"

	"ai-dev-manager-v2/internal/model"
)

const historyLimit = 20

func normalizeMetadata(m model.MemoryMetadata) (model.MemoryMetadata, error) {
	m.Category = strings.ToLower(strings.TrimSpace(m.Category))
	switch m.Category {
	case "", "convention", "decision", "progress", "issue", "environment", "preference", "other":
	default:
		return m, fmt.Errorf("unknown memory category")
	}
	if m.Importance < 0 || m.Importance > 5 || len(m.Tags) > 20 {
		return m, fmt.Errorf("invalid importance or tag count")
	}
	tags := map[string]bool{}
	out := []string{}
	for _, tag := range m.Tags {
		tag = strings.ToLower(strings.TrimSpace(tag))
		if tag == "" || len(tag) > 100 {
			return m, fmt.Errorf("invalid memory tag")
		}
		if !tags[tag] {
			tags[tag] = true
			out = append(out, tag)
		}
	}
	sort.Strings(out)
	if len(out) == 0 {
		out = nil
	}
	m.Tags = out
	return m, nil
}
func snapshot(values map[string]string, key string, r model.MemoryRecord) model.MemoryVersion {
	v, ok := values[key]
	out := model.MemoryVersion{Revision: r.Revision, Metadata: r.Metadata, UpdatedAt: r.UpdatedAt}
	if ok {
		out.Value = &v
	}
	return out
}
func applyVersion(values *map[string]string, records map[string]model.MemoryRecord, key string, value *string, metadata *model.MemoryMetadata) {
	r := records[key]
	old, exists := (*values)[key]
	if value == nil && !exists {
		return
	}
	next := r.Metadata
	if metadata != nil {
		next = *metadata
	}
	if exists && value != nil && old == *value && reflect.DeepEqual(next, r.Metadata) {
		return
	}
	if r.Revision > 0 || exists {
		r.History = append(r.History, snapshot(*values, key, r))
		if len(r.History) > historyLimit {
			r.History = append([]model.MemoryVersion(nil), r.History[len(r.History)-historyLimit:]...)
		}
	}
	r.Revision++
	now := time.Now().UTC()
	r.UpdatedAt = &now
	r.Metadata = next
	records[key] = r
	if value == nil {
		delete(*values, key)
	} else {
		if *values == nil {
			*values = map[string]string{}
		}
		(*values)[key] = *value
	}
}

type VersionSummary struct {
	Revision   int64                `json:"revision"`
	Deleted    bool                 `json:"deleted"`
	Metadata   model.MemoryMetadata `json:"metadata"`
	UpdatedAt  *time.Time           `json:"updated_at,omitempty"`
	ValueBytes int                  `json:"value_bytes"`
}
type HistoryResult struct {
	Key             string               `json:"key"`
	CurrentRevision int64                `json:"current_revision"`
	Versions        []VersionSummary     `json:"versions"`
	Selected        *model.MemoryVersion `json:"selected,omitempty"`
}

func (s *Service) History(id string, global bool, key string, revision *int64) (HistoryResult, error) {
	state, err := s.store.Load()
	if err != nil {
		return HistoryResult{}, err
	}
	values, records, err := scope(&state, id, global, false)
	if err != nil {
		return HistoryResult{}, err
	}
	key = strings.TrimSpace(key)
	r, recorded := records[key]
	_, exists := (*values)[key]
	if !exists && !recorded {
		return HistoryResult{}, fmt.Errorf("memory key %q not found", key)
	}
	all := append(append([]model.MemoryVersion{}, r.History...), snapshot(*values, key, r))
	out := HistoryResult{Key: key, CurrentRevision: r.Revision, Versions: []VersionSummary{}}
	for i := len(all) - 1; i >= 0; i-- {
		v := all[i]
		item := VersionSummary{Revision: v.Revision, Deleted: v.Value == nil, Metadata: v.Metadata, UpdatedAt: v.UpdatedAt}
		if v.Value != nil {
			item.ValueBytes = len(*v.Value)
		}
		out.Versions = append(out.Versions, item)
		if revision != nil && v.Revision == *revision {
			out.Selected = &v
		}
	}
	if revision != nil && out.Selected == nil {
		return HistoryResult{}, fmt.Errorf("memory revision not retained")
	}
	return out, nil
}
func (s *Service) Restore(id string, global bool, key string, revision, expected int64) (Entry, error) {
	key = strings.TrimSpace(key)
	if key == "" {
		return Entry{}, fmt.Errorf("memory key is required")
	}
	var out Entry
	err := s.store.Update(func(state *model.State) error {
		values, records, err := scope(state, id, global, true)
		if err != nil {
			return err
		}
		r, recorded := records[key]
		_, exists := (*values)[key]
		if !recorded && !exists {
			return fmt.Errorf("memory key %q not found", key)
		}
		if r.Revision != expected {
			return fmt.Errorf("memory revision conflict; reread history")
		}
		var selected *model.MemoryVersion
		for _, v := range r.History {
			if v.Revision == revision {
				copy := v
				selected = &copy
				break
			}
		}
		if selected == nil || selected.Value == nil {
			return fmt.Errorf("restorable memory revision not retained")
		}
		applyVersion(values, records, key, selected.Value, &selected.Metadata)
		out = entry(key, *selected.Value, records[key])
		return nil
	})
	return out, err
}

type DuplicateGroup struct {
	Keys []string `json:"keys"`
}
type DuplicateResult struct {
	Groups  []DuplicateGroup `json:"groups"`
	Total   int              `json:"total_groups"`
	Omitted int              `json:"omitted"`
}

func (s *Service) Duplicates(id string, global bool) (DuplicateResult, error) {
	items, err := s.list(id, global)
	if err != nil {
		return DuplicateResult{}, err
	}
	byValue := map[string][]string{}
	for _, item := range items {
		if strings.TrimSpace(item.Value) != "" {
			byValue[item.Value] = append(byValue[item.Value], item.Key)
		}
	}
	groups := []DuplicateGroup{}
	for _, keys := range byValue {
		if len(keys) > 1 {
			groups = append(groups, DuplicateGroup{Keys: keys})
		}
	}
	sort.Slice(groups, func(i, j int) bool { return groups[i].Keys[0] < groups[j].Keys[0] })
	out := DuplicateResult{Groups: []DuplicateGroup{}, Total: len(groups), Omitted: len(groups)}
	for _, g := range groups {
		if len(out.Groups) >= 100 {
			break
		}
		out.Groups = append(out.Groups, g)
		out.Omitted--
		b, _ := json.Marshal(out)
		if len(b) > 65536 {
			out.Groups = out.Groups[:len(out.Groups)-1]
			out.Omitted++
		}
	}
	return out, nil
}

type TrashEntry struct {
	Key                string     `json:"key"`
	Revision           int64      `json:"revision"`
	UpdatedAt          *time.Time `json:"updated_at,omitempty"`
	RestorableRevision *int64     `json:"restorable_revision,omitempty"`
}

func (s *Service) Trash(id string, global bool) ([]TrashEntry, error) {
	state, err := s.store.Load()
	if err != nil {
		return nil, err
	}
	values, records, err := scope(&state, id, global, false)
	if err != nil {
		return nil, err
	}
	out := []TrashEntry{}
	for k, r := range records {
		if _, exists := (*values)[k]; exists {
			continue
		}
		item := TrashEntry{Key: k, Revision: r.Revision, UpdatedAt: r.UpdatedAt}
		for i := len(r.History) - 1; i >= 0; i-- {
			if r.History[i].Value != nil {
				v := r.History[i].Revision
				item.RestorableRevision = &v
				break
			}
		}
		out = append(out, item)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	return out, nil
}

// Purge removes a deleted record and its retained history. Active keys cannot be purged.
func (s *Service) Purge(id string, global bool, key string, expected int64) error {
	return s.store.Update(func(state *model.State) error {
		values, records, err := scope(state, id, global, false)
		if err != nil {
			return err
		}
		if _, exists := (*values)[key]; exists {
			return fmt.Errorf("active memory must be deleted before purge")
		}
		r, exists := records[key]
		if !exists || r.Revision != expected {
			return fmt.Errorf("memory revision conflict; reload trash")
		}
		delete(records, key)
		return nil
	})
}
