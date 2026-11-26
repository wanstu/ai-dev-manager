package store

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"ai-dev-manager-v2/internal/pathutil"
)

// OperationIdentity describes an ADM-controlled finite project mutation.
// The opaque lease generation is never a renewal credential.
type OperationIdentity struct {
	Root          string    `json:"root"`
	EnvironmentID string    `json:"environment_id"`
	Generation    string    `json:"generation"`
	Kind          string    `json:"kind,omitempty"`
	HostPID       int       `json:"host_pid,omitempty"`
	StartedAt     time.Time `json:"started_at"`
}
type OperationGuard struct {
	once sync.Once
	file *os.File
}

func (g *OperationGuard) Close() {
	if g != nil {
		g.once.Do(func() { _ = g.file.Close() })
	}
}

// BeginOperation serializes admission across processes, not execution.
// Operations of the same lease may run concurrently. Different generations
// with overlapping physical roots cannot run concurrently even after takeover.
// The lifetime lock is held until the caller has finished the actual operation.
func (s *Store) BeginOperation(identity OperationIdentity) (*OperationGuard, error) {
	identity.Root = pathutil.ForCompare(identity.Root)
	if identity.Root == "" || identity.EnvironmentID == "" || identity.Generation == "" {
		return nil, fmt.Errorf("invalid writer operation identity")
	}
	dir := s.path + ".operations"
	registry, err := acquireStateLock(filepath.Join(dir, "registry"))
	if err != nil {
		return nil, err
	}
	defer registry.Close()
	activeOperations, err := scanOperations(dir)
	if err != nil {
		return nil, err
	}
	for _, active := range activeOperations {
		if active.EnvironmentID == identity.EnvironmentID && active.Generation == identity.Generation {
			continue
		}
		if pathutil.Within(active.Root, identity.Root) || pathutil.Within(identity.Root, active.Root) {
			return nil, fmt.Errorf("writer_handoff_pending: previous writer operation still active in environment %s; retry after it finishes", active.EnvironmentID)
		}
	}
	identity.HostPID = os.Getpid()
	identity.StartedAt = time.Now().UTC()
	f, err := os.CreateTemp(dir, "op-*.lock")
	if err != nil {
		return nil, err
	}
	metadataPath := strings.TrimSuffix(f.Name(), ".lock") + ".json"
	success := false
	defer func() {
		if !success {
			f.Close()
			os.Remove(metadataPath)
			os.Remove(f.Name())
		}
	}()
	acquired, err := tryStateLock(f)
	if err != nil {
		return nil, err
	}
	if !acquired {
		return nil, fmt.Errorf("new writer operation lock unexpectedly busy")
	}
	data, err := json.Marshal(identity)
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(metadataPath, data, 0600); err != nil {
		return nil, err
	}
	success = true
	return &OperationGuard{file: f}, nil
}

// ActiveOperations returns live, cross-process registry entries, cleaning up
// completed guards. It neither changes the Writer nor starts any operation.
func (s *Store) ActiveOperations() ([]OperationIdentity, error) {
	dir := s.path + ".operations"
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return []OperationIdentity{}, nil
	} else if err != nil {
		return nil, err
	}
	registry, err := acquireStateLock(filepath.Join(dir, "registry"))
	if err != nil {
		return nil, err
	}
	defer registry.Close()
	return scanOperations(dir)
}
func scanOperations(dir string) ([]OperationIdentity, error) {
	items := []OperationIdentity{}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasPrefix(name, "op-") || !strings.HasSuffix(name, ".lock") {
			continue
		}
		lockPath := filepath.Join(dir, name)
		metadataPath := strings.TrimSuffix(lockPath, ".lock") + ".json"
		f, err := os.OpenFile(lockPath, os.O_RDWR, 0600)
		if err != nil {
			return nil, fmt.Errorf("open writer operation: %w", err)
		}
		stale, err := tryStateLock(f)
		if err != nil {
			f.Close()
			return nil, err
		}
		if stale {
			// No live holder. Remove metadata first under the admission lock so a
			// crash during cleanup cannot leave metadata pointing at a missing lock.
			f.Close()
			if err := os.Remove(metadataPath); err != nil && !os.IsNotExist(err) {
				return nil, err
			}
			if err := os.Remove(lockPath); err != nil && !os.IsNotExist(err) {
				return nil, err
			}
			continue
		}
		f.Close()
		raw, err := os.ReadFile(metadataPath)
		if err != nil {
			return nil, fmt.Errorf("active writer operation metadata unavailable: %w", err)
		}
		var active OperationIdentity
		if err := json.Unmarshal(raw, &active); err != nil || active.Root == "" || active.EnvironmentID == "" || active.Generation == "" {
			return nil, fmt.Errorf("invalid active writer operation metadata")
		}
		items = append(items, active)
	}
	return items, nil

}
