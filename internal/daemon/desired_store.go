package daemon

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

const (
	desiredStateVersion       = 2
	desiredStateLegacyVersion = 1
	desiredStateFilename      = "desired-runtimes.json"

	DefaultRuntimeListen = "127.0.0.1:0"
	DockerRuntimeListen  = "0.0.0.0:0"
)

var (
	ErrCorruptDesiredState     = errors.New("corrupt desired runtime state")
	ErrUnsupportedDesiredState = errors.New("unsupported desired runtime state version")
)

type DesiredRuntime struct {
	WorkspaceID string `json:"workspace_id"`
	Listen      string `json:"listen,omitempty"`
	Exposed     bool   `json:"exposed,omitempty"`
}

type desiredStateFile struct {
	Version      int              `json:"version"`
	WorkspaceIDs []string         `json:"workspace_ids,omitempty"`
	Runtimes     []DesiredRuntime `json:"runtimes,omitempty"`
}

type DesiredStore struct {
	mu   sync.Mutex
	root string
}

func NewDesiredStore(root string) *DesiredStore {
	return &DesiredStore{root: filepath.Clean(root)}
}

func desiredStatePath(root string) string {
	return filepath.Join(root, runtimeDirName, desiredStateFilename)
}

// Load preserves the original v0.3 API for callers that only need Workspace
// IDs. New lifecycle code should use LoadRuntimes to retain listen intent.
func (s *DesiredStore) Load() ([]string, error) {
	runtimes, err := s.LoadRuntimes()
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(runtimes))
	for _, runtime := range runtimes {
		ids = append(ids, runtime.WorkspaceID)
	}
	return ids, nil
}

func (s *DesiredStore) LoadRuntimes() ([]DesiredRuntime, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.loadLocked()
}

func (s *DesiredStore) Add(workspaceID string) error {
	return s.Set(DesiredRuntime{WorkspaceID: workspaceID, Listen: DefaultRuntimeListen})
}

func (s *DesiredStore) Set(runtime DesiredRuntime) error {
	runtime = normalizeDesiredRuntime(runtime)
	if runtime.WorkspaceID == "" {
		return errors.New("workspace id is required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	runtimes, err := s.loadLocked()
	if err != nil {
		return err
	}
	found := false
	for i := range runtimes {
		if runtimes[i].WorkspaceID == runtime.WorkspaceID {
			runtimes[i] = runtime
			found = true
			break
		}
	}
	if !found {
		runtimes = append(runtimes, runtime)
	}
	return s.saveLocked(runtimes)
}

func (s *DesiredStore) Remove(workspaceID string) error {
	workspaceID = strings.TrimSpace(workspaceID)
	if workspaceID == "" {
		return errors.New("workspace id is required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	runtimes, err := s.loadLocked()
	if err != nil {
		return err
	}
	filtered := runtimes[:0]
	for _, runtime := range runtimes {
		if runtime.WorkspaceID != workspaceID {
			filtered = append(filtered, runtime)
		}
	}
	return s.saveLocked(filtered)
}

func (s *DesiredStore) Replace(workspaceIDs []string) error {
	runtimes := make([]DesiredRuntime, 0, len(workspaceIDs))
	for _, id := range workspaceIDs {
		runtimes = append(runtimes, DesiredRuntime{WorkspaceID: id, Listen: DefaultRuntimeListen})
	}
	return s.ReplaceRuntimes(runtimes)
}

func (s *DesiredStore) ReplaceRuntimes(runtimes []DesiredRuntime) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.saveLocked(runtimes)
}

func (s *DesiredStore) loadLocked() ([]DesiredRuntime, error) {
	path := desiredStatePath(s.root)
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return []DesiredRuntime{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read desired runtime state: %w", err)
	}
	var state desiredStateFile
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrCorruptDesiredState, err)
	}
	switch state.Version {
	case desiredStateLegacyVersion:
		runtimes := make([]DesiredRuntime, 0, len(state.WorkspaceIDs))
		for _, id := range state.WorkspaceIDs {
			runtimes = append(runtimes, DesiredRuntime{WorkspaceID: id, Listen: DefaultRuntimeListen})
		}
		return normalizeDesiredRuntimes(runtimes), nil
	case desiredStateVersion:
		return normalizeDesiredRuntimes(state.Runtimes), nil
	default:
		return nil, fmt.Errorf("%w: %d", ErrUnsupportedDesiredState, state.Version)
	}
}

func (s *DesiredStore) saveLocked(runtimes []DesiredRuntime) error {
	state := desiredStateFile{
		Version:  desiredStateVersion,
		Runtimes: normalizeDesiredRuntimes(runtimes),
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')

	dir := filepath.Join(s.root, runtimeDirName)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create desired runtime directory: %w", err)
	}
	temp, err := os.CreateTemp(dir, ".desired-runtimes-*.tmp")
	if err != nil {
		return fmt.Errorf("create desired runtime temp file: %w", err)
	}
	tempPath := temp.Name()
	cleanup := func() {
		_ = temp.Close()
		_ = os.Remove(tempPath)
	}
	if err := temp.Chmod(0o600); err != nil {
		cleanup()
		return err
	}
	if _, err := temp.Write(data); err != nil {
		cleanup()
		return err
	}
	if err := temp.Sync(); err != nil {
		cleanup()
		return err
	}
	if err := temp.Close(); err != nil {
		_ = os.Remove(tempPath)
		return err
	}
	if err := os.Rename(tempPath, desiredStatePath(s.root)); err != nil {
		_ = os.Remove(tempPath)
		return fmt.Errorf("replace desired runtime state: %w", err)
	}
	return nil
}

func normalizeDesiredRuntime(runtime DesiredRuntime) DesiredRuntime {
	runtime.WorkspaceID = strings.TrimSpace(runtime.WorkspaceID)
	runtime.Listen = strings.TrimSpace(runtime.Listen)
	if runtime.Listen == "" {
		runtime.Listen = DefaultRuntimeListen
	}
	return runtime
}

func normalizeDesiredRuntimes(runtimes []DesiredRuntime) []DesiredRuntime {
	set := make(map[string]DesiredRuntime, len(runtimes))
	for _, runtime := range runtimes {
		runtime = normalizeDesiredRuntime(runtime)
		if runtime.WorkspaceID != "" {
			set[runtime.WorkspaceID] = runtime
		}
	}
	ids := make([]string, 0, len(set))
	for id := range set {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	result := make([]DesiredRuntime, 0, len(ids))
	for _, id := range ids {
		result = append(result, set[id])
	}
	return result
}
