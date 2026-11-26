package memory

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"ai-dev-manager-v2/internal/model"
	"ai-dev-manager-v2/internal/store"
)

type Entry struct {
	Validity  string                `json:"validity"`
	Key       string                `json:"key"`
	Value     string                `json:"value"`
	Revision  int64                 `json:"revision,omitempty"`
	Metadata  *model.MemoryMetadata `json:"metadata,omitempty"`
	UpdatedAt *time.Time            `json:"updated_at,omitempty"`
}

type Service struct{ store *store.Store }

func New(s *store.Store) *Service { return &Service{store: s} }

func scope(state *model.State, id string, global bool, create bool) (*map[string]string, map[string]model.MemoryRecord, error) {
	if global {
		if create && state.GlobalMemoryRecords == nil {
			state.GlobalMemoryRecords = map[string]model.MemoryRecord{}
		}
		return &state.GlobalMemory, state.GlobalMemoryRecords, nil
	}
	for i := range state.Environments {
		if state.Environments[i].ID == id {
			if create {
				if state.EnvironmentMemoryRecords == nil {
					state.EnvironmentMemoryRecords = map[string]map[string]model.MemoryRecord{}
				}
				if state.EnvironmentMemoryRecords[id] == nil {
					state.EnvironmentMemoryRecords[id] = map[string]model.MemoryRecord{}
				}
			}
			return &state.Environments[i].PrivateMemory, state.EnvironmentMemoryRecords[id], nil
		}
	}
	return nil, nil, fmt.Errorf("environment %q not found", id)
}
func entry(key, value string, record model.MemoryRecord) Entry {
	e := Entry{Key: key, Value: value, Revision: record.Revision, UpdatedAt: record.UpdatedAt, Validity: validity(record.Metadata, time.Now().UTC())}
	if record.Revision > 0 {
		e.Metadata = &record.Metadata
	}
	return e
}
func (s *Service) list(id string, global bool) ([]Entry, error) {
	state, err := s.store.Load()
	if err != nil {
		return nil, err
	}
	values, records, err := scope(&state, id, global, false)
	if err != nil {
		return nil, err
	}
	out := make([]Entry, 0, len(*values))
	for k, v := range *values {
		out = append(out, entry(k, v, records[k]))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	return out, nil
}
func (s *Service) read(id string, global bool, key string) (Entry, error) {
	state, err := s.store.Load()
	if err != nil {
		return Entry{}, err
	}
	values, records, err := scope(&state, id, global, false)
	if err != nil {
		return Entry{}, err
	}
	key = strings.TrimSpace(key)
	v, ok := (*values)[key]
	if !ok {
		return Entry{}, fmt.Errorf("memory key %q not found", key)
	}
	return entry(key, v, records[key]), nil
}
func (s *Service) mutate(id string, global bool, key string, value *string) error {
	key = strings.TrimSpace(key)
	if key == "" {
		return fmt.Errorf("memory key is required")
	}
	return s.store.Update(func(state *model.State) error {
		values, records, err := scope(state, id, global, true)
		if err != nil {
			return err
		}
		applyVersion(values, records, key, value, nil)
		return nil
	})
}
func (s *Service) GlobalList() ([]Entry, error)                  { return s.list("", true) }
func (s *Service) GlobalRead(key string) (Entry, error)          { return s.read("", true, key) }
func (s *Service) GlobalWrite(key, value string) error           { return s.mutate("", true, key, &value) }
func (s *Service) GlobalDelete(key string) error                 { return s.mutate("", true, key, nil) }
func (s *Service) EnvironmentList(id string) ([]Entry, error)    { return s.list(id, false) }
func (s *Service) EnvironmentRead(id, key string) (Entry, error) { return s.read(id, false, key) }
func (s *Service) EnvironmentWrite(id, key, value string) error {
	return s.mutate(id, false, key, &value)
}
func (s *Service) EnvironmentDelete(id, key string) error { return s.mutate(id, false, key, nil) }
