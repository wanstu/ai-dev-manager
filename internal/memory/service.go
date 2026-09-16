package memory

import (
	"fmt"
	"sort"
	"strings"

	"ai-dev-manager-v2/internal/model"
	"ai-dev-manager-v2/internal/store"
)

type Entry struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type Service struct{ store *store.Store }

func New(s *store.Store) *Service { return &Service{store: s} }

func (s *Service) GlobalList() ([]Entry, error) {
	state, err := s.store.Load()
	if err != nil {
		return nil, err
	}
	return entries(state.GlobalMemory), nil
}

func (s *Service) GlobalRead(key string) (Entry, error) {
	state, err := s.store.Load()
	if err != nil {
		return Entry{}, err
	}
	key = strings.TrimSpace(key)
	value, ok := state.GlobalMemory[key]
	if !ok {
		return Entry{}, fmt.Errorf("global memory key %q not found", key)
	}
	return Entry{Key: key, Value: value}, nil
}

func (s *Service) GlobalWrite(key, value string) error {
	key = strings.TrimSpace(key)
	if key == "" {
		return fmt.Errorf("memory key is required")
	}
	return s.store.Update(func(state *model.State) error {
		if state.GlobalMemory == nil {
			state.GlobalMemory = map[string]string{}
		}
		state.GlobalMemory[key] = value
		return nil
	})
}

func (s *Service) GlobalDelete(key string) error {
	return s.store.Update(func(state *model.State) error {
		delete(state.GlobalMemory, strings.TrimSpace(key))
		return nil
	})
}

func (s *Service) EnvironmentList(environmentID string) ([]Entry, error) {
	state, err := s.store.Load()
	if err != nil {
		return nil, err
	}
	for _, env := range state.Environments {
		if env.ID == environmentID {
			return entries(env.PrivateMemory), nil
		}
	}
	return nil, fmt.Errorf("environment %q not found", environmentID)
}

func (s *Service) EnvironmentRead(environmentID, key string) (Entry, error) {
	state, err := s.store.Load()
	if err != nil {
		return Entry{}, err
	}
	key = strings.TrimSpace(key)
	for _, env := range state.Environments {
		if env.ID == environmentID {
			value, ok := env.PrivateMemory[key]
			if !ok {
				return Entry{}, fmt.Errorf("environment memory key %q not found", key)
			}
			return Entry{Key: key, Value: value}, nil
		}
	}
	return Entry{}, fmt.Errorf("environment %q not found", environmentID)
}

func (s *Service) EnvironmentWrite(environmentID, key, value string) error {
	key = strings.TrimSpace(key)
	if key == "" {
		return fmt.Errorf("memory key is required")
	}
	return s.store.Update(func(state *model.State) error {
		for i := range state.Environments {
			if state.Environments[i].ID == environmentID {
				if state.Environments[i].PrivateMemory == nil {
					state.Environments[i].PrivateMemory = map[string]string{}
				}
				state.Environments[i].PrivateMemory[key] = value
				return nil
			}
		}
		return fmt.Errorf("environment %q not found", environmentID)
	})
}

func (s *Service) EnvironmentDelete(environmentID, key string) error {
	return s.store.Update(func(state *model.State) error {
		for i := range state.Environments {
			if state.Environments[i].ID == environmentID {
				delete(state.Environments[i].PrivateMemory, strings.TrimSpace(key))
				return nil
			}
		}
		return fmt.Errorf("environment %q not found", environmentID)
	})
}

func entries(values map[string]string) []Entry {
	out := make([]Entry, 0, len(values))
	for key, value := range values {
		out = append(out, Entry{Key: key, Value: value})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	return out
}
