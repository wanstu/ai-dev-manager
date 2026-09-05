package environment

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
)

const (
	storeVersion  = 1
	storeFilename = "environments.json"
)

type storeFile struct {
	Version      int           `json:"version"`
	Environments []Environment `json:"environments"`
}

type Store struct {
	mu   sync.Mutex
	root string
}

func NewStore(root string) *Store {
	return &Store{root: filepath.Clean(root)}
}

func (s *Store) Path() string { return filepath.Join(s.root, storeFilename) }

func (s *Store) List() ([]Environment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.loadLocked()
}

func (s *Store) Get(id string) (Environment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	envs, err := s.loadLocked()
	if err != nil {
		return Environment{}, err
	}
	for _, env := range envs {
		if env.ID == id {
			return env, nil
		}
	}
	return Environment{}, &Error{Code: ErrNotFound, EnvironmentID: id, Message: fmt.Sprintf("environment %q not found", id)}
}

func (s *Store) Put(env Environment) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	envs, err := s.loadLocked()
	if err != nil {
		return err
	}
	found := false
	for i := range envs {
		if envs[i].ID == env.ID {
			envs[i] = cloneEnvironment(env)
			found = true
			break
		}
	}
	if !found {
		envs = append(envs, cloneEnvironment(env))
	}
	return s.saveLocked(envs)
}

func (s *Store) Remove(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	envs, err := s.loadLocked()
	if err != nil {
		return err
	}
	index := -1
	for i := range envs {
		if envs[i].ID == id {
			index = i
			break
		}
	}
	if index < 0 {
		return &Error{Code: ErrNotFound, EnvironmentID: id, Message: fmt.Sprintf("environment %q not found", id)}
	}
	envs = append(envs[:index], envs[index+1:]...)
	return s.saveLocked(envs)
}

func (s *Store) loadLocked() ([]Environment, error) {
	data, err := os.ReadFile(s.Path())
	if errors.Is(err, os.ErrNotExist) {
		return []Environment{}, nil
	}
	if err != nil {
		return nil, &Error{Code: ErrStore, Message: "read environment store", Err: err}
	}
	var file storeFile
	if err := json.Unmarshal(data, &file); err != nil {
		return nil, &Error{Code: ErrStore, Message: "decode environment store", Err: err}
	}
	if file.Version != storeVersion {
		return nil, &Error{Code: ErrStore, Message: fmt.Sprintf("unsupported environment store version %d", file.Version)}
	}
	result := make([]Environment, len(file.Environments))
	for i := range file.Environments {
		result[i] = cloneEnvironment(file.Environments[i])
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return result, nil
}

func (s *Store) saveLocked(envs []Environment) error {
	copyEnvs := make([]Environment, len(envs))
	for i := range envs {
		copyEnvs[i] = cloneEnvironment(envs[i])
	}
	sort.Slice(copyEnvs, func(i, j int) bool { return copyEnvs[i].ID < copyEnvs[j].ID })
	data, err := json.MarshalIndent(storeFile{Version: storeVersion, Environments: copyEnvs}, "", "  ")
	if err != nil {
		return &Error{Code: ErrStore, Message: "encode environment store", Err: err}
	}
	data = append(data, '\n')
	if err := os.MkdirAll(s.root, 0o700); err != nil {
		return &Error{Code: ErrStore, Message: "create environment store directory", Err: err}
	}
	temp, err := os.CreateTemp(s.root, ".environments-*.tmp")
	if err != nil {
		return &Error{Code: ErrStore, Message: "create environment store temp file", Err: err}
	}
	tempPath := temp.Name()
	cleanup := func() {
		_ = temp.Close()
		_ = os.Remove(tempPath)
	}
	if err := temp.Chmod(0o600); err != nil {
		cleanup()
		return &Error{Code: ErrStore, Message: "chmod environment store temp file", Err: err}
	}
	if _, err := temp.Write(data); err != nil {
		cleanup()
		return &Error{Code: ErrStore, Message: "write environment store temp file", Err: err}
	}
	if err := temp.Sync(); err != nil {
		cleanup()
		return &Error{Code: ErrStore, Message: "sync environment store temp file", Err: err}
	}
	if err := temp.Close(); err != nil {
		_ = os.Remove(tempPath)
		return &Error{Code: ErrStore, Message: "close environment store temp file", Err: err}
	}
	if err := os.Rename(tempPath, s.Path()); err != nil {
		_ = os.Remove(tempPath)
		return &Error{Code: ErrStore, Message: "replace environment store", Err: err}
	}
	return nil
}

func cloneEnvironment(env Environment) Environment {
	clone := env
	if env.Writer != nil {
		writerCopy := *env.Writer
		clone.Writer = &writerCopy
	}
	if env.Metadata != nil {
		clone.Metadata = make(map[string]string, len(env.Metadata))
		for key, value := range env.Metadata {
			clone.Metadata[key] = value
		}
	}
	if env.Error != nil {
		errCopy := *env.Error
		clone.Error = &errCopy
	}
	return clone
}
