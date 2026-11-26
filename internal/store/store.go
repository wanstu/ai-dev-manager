package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"ai-dev-manager-v2/internal/model"
)

const stateVersion = 1

type Store struct {
	mu   sync.Mutex
	path string
}

func DefaultPath() (string, error) {
	if root := os.Getenv("ADM_V2_HOME"); root != "" {
		return filepath.Join(root, "state.json"), nil
	}
	root, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "ai-dev-manager-v2", "state.json"), nil
}

func New(path string) *Store {
	// Resolve an existing state-file alias before deriving the stable lock path.
	// The parent may not exist yet; a sidecar in that directory will still use
	// the filesystem's real parent once it is created.
	if absolute, err := filepath.Abs(path); err == nil {
		path = absolute
	}
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		path = resolved
	} else if parent, err := filepath.EvalSymlinks(filepath.Dir(path)); err == nil {
		path = filepath.Join(parent, filepath.Base(path))
	}
	return &Store{path: path}
}

func (s *Store) Path() string { return s.path }

func (s *Store) Load() (model.State, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	lock, err := acquireStateLock(s.path)
	if err != nil {
		return model.State{}, err
	}
	defer lock.Close()
	return s.loadLocked()
}

func (s *Store) Update(fn func(*model.State) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	lock, err := acquireStateLock(s.path)
	if err != nil {
		return err
	}
	defer lock.Close()
	state, err := s.loadLocked()
	if err != nil {
		return err
	}
	if err := fn(&state); err != nil {
		return err
	}
	return s.saveLocked(state)
}

func (s *Store) loadLocked() (model.State, error) {
	data, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return defaultState(), nil
	}
	if err != nil {
		return model.State{}, err
	}
	var state model.State
	if err := json.Unmarshal(data, &state); err != nil {
		return model.State{}, fmt.Errorf("decode state: %w", err)
	}
	if state.Version != stateVersion {
		return model.State{}, fmt.Errorf("unsupported state version %d", state.Version)
	}
	if state.GlobalMemory == nil {
		state.GlobalMemory = map[string]string{}
	}
	normalizeRetentionState(&state)
	return state, nil
}

func (s *Store) saveLocked(state model.State) error {
	state.Version = stateVersion
	if state.GlobalMemory == nil {
		state.GlobalMemory = map[string]string{}
	}
	normalizeRetentionState(&state)
	// Private history must follow the owning Environment's lifetime.
	active := make(map[string]bool, len(state.Environments))
	for _, env := range state.Environments {
		active[env.ID] = true
	}
	for id := range state.EnvironmentMemoryRecords {
		if !active[id] {
			delete(state.EnvironmentMemoryRecords, id)
		}
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	// A unique same-directory file avoids stale fixed-name temporaries and
	// ensures the replacement does not cross filesystem boundaries.
	tmp, err := os.CreateTemp(filepath.Dir(s.path), "."+filepath.Base(s.path)+".*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	defer tmp.Close()
	if _, err := tmp.Write(append(data, '\n')); err != nil {
		return err
	}
	if err := tmp.Sync(); err != nil {
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmp.Name(), s.path); err != nil {
		return err
	}
	return nil
}

func defaultState() model.State {
	return model.State{Version: stateVersion, GlobalMemory: map[string]string{}}
}

func normalizeRetentionState(state *model.State) {
	if state == nil {
		return
	}
	for i := range state.Environments {
		createdAt := state.Environments[i].CreatedAt
		normalizeResourceRetention(&state.Environments[i].Retention, &createdAt)
	}
	for i := range state.MCPs {
		normalizeResourceRetention(&state.MCPs[i].Retention, nil)
	}
	sourceRetention := make(map[string]model.ResourceRetention, len(state.SkillSources))
	for i := range state.SkillSources {
		createdAt := state.SkillSources[i].CreatedAt
		normalizeResourceRetention(&state.SkillSources[i].Retention, &createdAt)
		sourceRetention[state.SkillSources[i].ID] = cloneResourceRetention(state.SkillSources[i].Retention)
	}
	for i := range state.Skills {
		if state.Skills[i].Retention.Persistence == "" && state.Skills[i].SourceID != "" {
			if inherited, ok := sourceRetention[state.Skills[i].SourceID]; ok {
				state.Skills[i].Retention = cloneResourceRetention(inherited)
			}
		}
		normalizeResourceRetention(&state.Skills[i].Retention, nil)
	}
}

func normalizeResourceRetention(retention *model.ResourceRetention, createdAt *time.Time) {
	if retention == nil {
		return
	}
	if retention.Persistence == "" {
		retention.Persistence = model.PersistenceDurable
		if retention.CreatorSurface == "" {
			retention.CreatorSurface = "legacy"
		}
	}
	if retention.CreatorSurface == "" {
		retention.CreatorSurface = "unknown"
	}
	if retention.CreatedAt == nil && createdAt != nil && !createdAt.IsZero() {
		value := createdAt.UTC()
		retention.CreatedAt = &value
	}
}

func cloneResourceRetention(input model.ResourceRetention) model.ResourceRetention {
	cloneTime := func(value *time.Time) *time.Time {
		if value == nil {
			return nil
		}
		copy := value.UTC()
		return &copy
	}
	input.CreatedAt = cloneTime(input.CreatedAt)
	input.LastUsedAt = cloneTime(input.LastUsedAt)
	input.ExpiresAt = cloneTime(input.ExpiresAt)
	return input
}
