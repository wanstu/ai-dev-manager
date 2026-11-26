package store

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// ProcessRecord contains only lifecycle metadata, never arguments, environment
// variables, or captured logs. The record's lock is not a Writer lease.
type ProcessRecord struct {
	ID            string     `json:"id"`
	EnvironmentID string     `json:"environment_id"`
	Executable    string     `json:"executable"`
	Cwd           string     `json:"cwd,omitempty"`
	GatewayPID    int        `json:"gateway_pid"`
	PID           int        `json:"pid,omitempty"`
	StartedAt     time.Time  `json:"started_at"`
	ExitedAt      *time.Time `json:"exited_at,omitempty"`
	ExitCode      *int       `json:"exit_code,omitempty"`
	ExitReason    string     `json:"exit_reason,omitempty"`
	ErrorKind     string     `json:"error_kind,omitempty"`
	OwnerLive     bool       `json:"-"`
}

const maxTerminalProcessRecords = 256

type ProcessRecordHandle struct {
	store  *Store
	record ProcessRecord
	lock   *os.File
}

func (s *Store) processHistoryDir() string { return s.path + ".process-history" }
func validProcessRecordID(id string) bool {
	if !strings.HasPrefix(id, "proc_") || len(id) > 128 {
		return false
	}
	for _, r := range strings.TrimPrefix(id, "proc_") {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '_') {
			return false
		}
	}
	return len(id) > 5
}
func (s *Store) BeginProcessRecord(record ProcessRecord) (*ProcessRecordHandle, error) {
	if !validProcessRecordID(record.ID) || record.EnvironmentID == "" {
		return nil, fmt.Errorf("invalid Process record identity")
	}
	dir := s.processHistoryDir()
	registry, err := acquireStateLock(filepath.Join(dir, "registry"))
	if err != nil {
		return nil, err
	}
	defer registry.Close()
	path := filepath.Join(dir, record.ID)
	f, err := os.OpenFile(path+".lock", os.O_CREATE|os.O_EXCL|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	ok, err := tryStateLock(f)
	if err != nil || !ok {
		f.Close()
		os.Remove(path + ".lock")
		return nil, fmt.Errorf("cannot lock Process record: %v", err)
	}
	if err := writeProcessRecord(path+".json", record); err != nil {
		f.Close()
		os.Remove(path + ".lock")
		return nil, err
	}
	return &ProcessRecordHandle{s, record, f}, nil
}
func writeProcessRecord(path string, record ProcessRecord) error {
	raw, err := json.Marshal(record)
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".record-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	defer f.Close()
	if _, err := f.Write(raw); err != nil {
		return err
	}
	if err := f.Sync(); err != nil {
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), path)
}
func (h *ProcessRecordHandle) Update(record ProcessRecord) error {
	if record.ID != h.record.ID || record.EnvironmentID != h.record.EnvironmentID {
		return fmt.Errorf("Process record identity changed")
	}
	dir := h.store.processHistoryDir()
	registry, err := acquireStateLock(filepath.Join(dir, "registry"))
	if err != nil {
		return err
	}
	defer registry.Close()
	if err := writeProcessRecord(filepath.Join(dir, record.ID+".json"), record); err != nil {
		return err
	}
	h.record = record
	return nil
}
func (h *ProcessRecordHandle) Close() {
	if h != nil && h.lock != nil {
		h.lock.Close()
		h.lock = nil
	}
}
func (s *Store) ProcessRecords() ([]ProcessRecord, error) {
	dir := s.processHistoryDir()
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return []ProcessRecord{}, nil
	} else if err != nil {
		return nil, err
	}
	registry, err := acquireStateLock(filepath.Join(dir, "registry"))
	if err != nil {
		return nil, err
	}
	defer registry.Close()
	return readProcessRecords(dir)
}
func readProcessRecords(dir string) ([]ProcessRecord, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	out := []ProcessRecord{}
	for _, e := range entries {
		id := strings.TrimSuffix(e.Name(), ".json")
		if !strings.HasSuffix(e.Name(), ".json") || !validProcessRecordID(id) {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			return nil, err
		}
		var r ProcessRecord
		if err := json.Unmarshal(raw, &r); err != nil || r.ID != id || r.EnvironmentID == "" {
			return nil, fmt.Errorf("invalid Process history record %s", id)
		}
		f, err := os.OpenFile(filepath.Join(dir, id+".lock"), os.O_RDWR, 0600)
		if err != nil {
			return nil, err
		}
		free, err := tryStateLock(f)
		f.Close()
		if err != nil {
			return nil, err
		}
		r.OwnerLive = !free
		out = append(out, r)
	}
	return out, nil
}

// Prune only confirmed terminal records without a live holder. Unconfirmed
// records are retained for diagnosis, not silently treated as exited.
func (s *Store) PruneProcessRecords() error {
	dir := s.processHistoryDir()
	registry, err := acquireStateLock(filepath.Join(dir, "registry"))
	if err != nil {
		return err
	}
	defer registry.Close()
	records, err := readProcessRecords(dir)
	if err != nil {
		return err
	}
	terminal := []ProcessRecord{}
	for _, r := range records {
		if r.ExitedAt != nil && !r.OwnerLive {
			terminal = append(terminal, r)
		}
	}
	sort.Slice(terminal, func(i, j int) bool { return terminal[i].ExitedAt.After(*terminal[j].ExitedAt) })
	for _, r := range terminal[min(len(terminal), maxTerminalProcessRecords):] {
		if err := os.Remove(filepath.Join(dir, r.ID+".json")); err != nil {
			return err
		}
		if err := os.Remove(filepath.Join(dir, r.ID+".lock")); err != nil {
			return err
		}
	}
	return nil
}
func (s *Store) RemoveTerminalProcessRecords(environmentID string) error {
	dir := s.processHistoryDir()
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return nil
	} else if err != nil {
		return err
	}
	registry, err := acquireStateLock(filepath.Join(dir, "registry"))
	if err != nil {
		return err
	}
	defer registry.Close()
	records, err := readProcessRecords(dir)
	if err != nil {
		return err
	}
	for _, r := range records {
		if r.EnvironmentID != environmentID || r.ExitedAt == nil || r.OwnerLive {
			continue
		}
		if err := os.Remove(filepath.Join(dir, r.ID+".json")); err != nil {
			return err
		}
		if err := os.Remove(filepath.Join(dir, r.ID+".lock")); err != nil {
			return err
		}
	}
	return nil
}
