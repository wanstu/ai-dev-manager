package secretvault

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"sync"
)

var validName = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_.-]{0,79}$`)

type Store struct {
	path string
	mu   sync.Mutex
}
type Metadata struct {
	Name string `json:"name"`
}
type disk struct {
	Version int               `json:"version"`
	Values  map[string]string `json:"values"`
}

func New(statePath string) *Store { return &Store{path: statePath + ".secrets.json"} }
func ValidName(name string) bool  { return validName.MatchString(name) }
func (s *Store) load() (disk, error) {
	data, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return disk{Version: 1, Values: map[string]string{}}, nil
	}
	if err != nil {
		return disk{}, err
	}
	var state disk
	if err := json.Unmarshal(data, &state); err != nil {
		return disk{}, fmt.Errorf("secret vault data is invalid: %w", err)
	}
	if state.Version != 1 || state.Values == nil {
		return disk{}, errors.New("unsupported secret vault data format")
	}
	return state, nil
}
func (s *Store) save(state disk) error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0700); err != nil {
		return err
	}
	bytes, err := json.Marshal(state)
	if err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(s.path), ".secrets-*")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	if err := file.Chmod(0600); err != nil {
		file.Close()
		return err
	}
	if _, err := file.Write(bytes); err != nil {
		file.Close()
		return err
	}
	if err := file.Sync(); err != nil {
		file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return os.Rename(file.Name(), s.path)
}
func (s *Store) Set(name, plaintext string) error {
	if !ValidName(name) {
		return errors.New("invalid secret name")
	}
	if plaintext == "" {
		return errors.New("secret value cannot be empty")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	state, err := s.load()
	if err != nil {
		return err
	}
	encrypted, err := protect([]byte(plaintext), s.path)
	if err != nil {
		return fmt.Errorf("protect secret: %w", err)
	}
	state.Values[name] = base64.StdEncoding.EncodeToString(encrypted)
	return s.save(state)
}
func (s *Store) Get(name string) (string, error) {
	if !ValidName(name) {
		return "", errors.New("invalid secret name")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	state, err := s.load()
	if err != nil {
		return "", err
	}
	raw, ok := state.Values[name]
	if !ok {
		return "", fmt.Errorf("secret %q not found", name)
	}
	payload, err := base64.StdEncoding.DecodeString(raw)
	if err != nil {
		return "", errors.New("secret vault ciphertext invalid")
	}
	plain, err := unprotect(payload, s.path)
	if err != nil {
		return "", fmt.Errorf("secret vault decryption failed: %w", err)
	}
	return string(plain), nil
}
func (s *Store) List() ([]Metadata, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	state, err := s.load()
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(state.Values))
	for name := range state.Values {
		names = append(names, name)
	}
	sort.Strings(names)
	items := make([]Metadata, 0, len(names))
	for _, name := range names {
		items = append(items, Metadata{Name: name})
	}
	return items, nil
}
func (s *Store) Delete(name string) error {
	if !ValidName(name) {
		return errors.New("invalid secret name")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	state, err := s.load()
	if err != nil {
		return err
	}
	if _, ok := state.Values[name]; !ok {
		return fmt.Errorf("secret %q not found", name)
	}
	delete(state.Values, name)
	return s.save(state)
}
