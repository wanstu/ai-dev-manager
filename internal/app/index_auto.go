package app

import (
	"sort"

	"ai-dev-manager-v2/internal/model"
)

type IndexAutoStatus struct {
	EnvironmentID string `json:"environment_id"`
	Enabled       bool   `json:"enabled"`
	State         string `json:"state"`
	Message       string `json:"message,omitempty"`
}

func (s *Service) IndexAutoStatus(environmentID string) (IndexAutoStatus, error) {
	if _, err := s.Environments.Get(environmentID); err != nil {
		return IndexAutoStatus{}, err
	}
	state, err := s.Store.Load()
	if err != nil {
		return IndexAutoStatus{}, err
	}
	enabled := false
	for _, id := range state.AutoIndexEnvironmentIDs {
		if id == environmentID {
			enabled = true
			break
		}
	}
	status := "disabled"
	if enabled {
		status = "configured"
	}
	return IndexAutoStatus{EnvironmentID: environmentID, Enabled: enabled, State: status}, nil
}
func (s *Service) SetIndexAuto(environmentID string, enabled bool) (IndexAutoStatus, error) {
	if _, err := s.Environments.Get(environmentID); err != nil {
		return IndexAutoStatus{}, err
	}
	err := s.Store.Update(func(state *model.State) error {
		kept := make([]string, 0, len(state.AutoIndexEnvironmentIDs)+1)
		for _, id := range state.AutoIndexEnvironmentIDs {
			if id != environmentID {
				kept = append(kept, id)
			}
		}
		if enabled {
			kept = append(kept, environmentID)
		}
		sort.Strings(kept)
		state.AutoIndexEnvironmentIDs = kept
		return nil
	})
	if err != nil {
		return IndexAutoStatus{}, err
	}
	return s.IndexAutoStatus(environmentID)
}

// removeAutoIndexID cleans the persisted watcher setting after deletion.
func (s *Service) removeAutoIndexID(id string) error {
	return s.Store.Update(func(state *model.State) error {
		filtered := make([]string, 0, len(state.AutoIndexEnvironmentIDs))
		for _, old := range state.AutoIndexEnvironmentIDs {
			if old != id {
				filtered = append(filtered, old)
			}
		}
		state.AutoIndexEnvironmentIDs = filtered
		return nil
	})
}
func (s *Service) AutoIndexIDs() ([]string, error) {
	state, err := s.Store.Load()
	if err != nil {
		return nil, err
	}
	return append([]string(nil), state.AutoIndexEnvironmentIDs...), nil
}
