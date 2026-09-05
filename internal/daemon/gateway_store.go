package daemon

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

const (
	gatewayStateVersion  = 1
	gatewayStateFilename = "gateway.json"
)

var (
	ErrCorruptGatewayState     = errors.New("corrupt gateway state")
	ErrUnsupportedGatewayState = errors.New("unsupported gateway state version")
)

type GatewayDesired struct {
	DesiredRunning bool   `json:"desired_running"`
	Listen         string `json:"listen,omitempty"`
	Exposed        bool   `json:"exposed,omitempty"`
}

type gatewayStateFile struct {
	Version int `json:"version"`
	GatewayDesired
}

type GatewayStore struct {
	mu   sync.Mutex
	root string
}

func NewGatewayStore(root string) *GatewayStore {
	return &GatewayStore{root: filepath.Clean(root)}
}

func gatewayStatePath(root string) string {
	return filepath.Join(root, runtimeDirName, gatewayStateFilename)
}

func (s *GatewayStore) Load() (GatewayDesired, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.loadLocked()
}

func (s *GatewayStore) Save(desired GatewayDesired) error {
	desired.Listen = strings.TrimSpace(desired.Listen)
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.saveLocked(desired)
}

func (s *GatewayStore) loadLocked() (GatewayDesired, error) {
	data, err := os.ReadFile(gatewayStatePath(s.root))
	if errors.Is(err, os.ErrNotExist) {
		return GatewayDesired{}, nil
	}
	if err != nil {
		return GatewayDesired{}, fmt.Errorf("read gateway state: %w", err)
	}
	var state gatewayStateFile
	if err := json.Unmarshal(data, &state); err != nil {
		return GatewayDesired{}, fmt.Errorf("%w: %v", ErrCorruptGatewayState, err)
	}
	if state.Version != gatewayStateVersion {
		return GatewayDesired{}, fmt.Errorf("%w: %d", ErrUnsupportedGatewayState, state.Version)
	}
	state.Listen = strings.TrimSpace(state.Listen)
	return state.GatewayDesired, nil
}

func (s *GatewayStore) saveLocked(desired GatewayDesired) error {
	state := gatewayStateFile{Version: gatewayStateVersion, GatewayDesired: desired}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	dir := filepath.Join(s.root, runtimeDirName)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create gateway state directory: %w", err)
	}
	temp, err := os.CreateTemp(dir, ".gateway-*.tmp")
	if err != nil {
		return fmt.Errorf("create gateway temp file: %w", err)
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
	if err := os.Rename(tempPath, gatewayStatePath(s.root)); err != nil {
		_ = os.Remove(tempPath)
		return fmt.Errorf("replace gateway state: %w", err)
	}
	return nil
}
