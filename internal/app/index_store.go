package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"ai-dev-manager-v2/internal/pathutil"
	"ai-dev-manager-v2/internal/projectanalysis"
)

var indexIDRE = regexp.MustCompile("^[A-Za-z0-9_-]{1,100}$")

type indexPointer struct {
	Version    int    `json:"version"`
	SourceRoot string `json:"source_root"`
	Generation string `json:"generation"`
}

// projectIndexStore is separate from all project worktrees and uses its own
// per-Environment RW lock, never the source-write lease. Each generation
// remains immutable; an atomically renamed pointer publishes a whole snapshot.
type projectIndexStore struct {
	root  string
	mu    sync.Mutex
	locks map[string]*sync.RWMutex
}

func newProjectIndexStore(statePath string) *projectIndexStore {
	return &projectIndexStore{root: filepath.Join(filepath.Dir(statePath), "indexes"), locks: map[string]*sync.RWMutex{}}
}
func (s *projectIndexStore) lock(id string) (*sync.RWMutex, error) {
	if !indexIDRE.MatchString(id) {
		return nil, fmt.Errorf("invalid Environment ID")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.locks[id] == nil {
		s.locks[id] = &sync.RWMutex{}
	}
	return s.locks[id], nil
}
func (s *projectIndexStore) environmentDir(id string) string { return filepath.Join(s.root, id) }
func (s *projectIndexStore) current(id, sourceRoot string) (string, error) {
	data, err := os.ReadFile(filepath.Join(s.environmentDir(id), "current.json"))
	if err != nil {
		return "", err
	}
	var p indexPointer
	if err := json.Unmarshal(data, &p); err != nil {
		return "", fmt.Errorf("invalid index pointer: %w", err)
	}
	if p.Version != 1 || p.Generation == "" || filepath.Base(p.Generation) != p.Generation || strings.Contains(p.Generation, "..") {
		return "", fmt.Errorf("invalid project index pointer")
	}
	if !pathutil.Same(p.SourceRoot, sourceRoot) {
		return "", fmt.Errorf("index belongs to a different Environment root; run project_analyze")
	}
	return filepath.Join(s.environmentDir(id), "generations", p.Generation), nil
}
func (s *projectIndexStore) publish(id, sourceRoot string, result projectanalysis.Result) error {
	envRoot := s.environmentDir(id)
	genRoot := filepath.Join(envRoot, "generations")
	if err := os.MkdirAll(genRoot, 0700); err != nil {
		return err
	}
	stage, err := os.MkdirTemp(genRoot, "gen-")
	if err != nil {
		return err
	}
	created := false
	defer func() {
		if !created {
			_ = os.RemoveAll(stage)
		}
	}()
	items := []struct{ path, content string }{
		{projectanalysis.OverviewRelativePath, result.Markdown},
		{projectanalysis.IndexFilesRelativePath, result.FilesJSONL},
		{projectanalysis.IndexSymbolsRelativePath, result.SymbolsJSONL},
		{projectanalysis.IndexCallsRelativePath, result.CallsJSONL},
		{projectanalysis.IndexManifestRelativePath, result.ManifestJSON},
	}
	for _, item := range items {
		path := filepath.Join(stage, filepath.FromSlash(item.path))
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			return err
		}
		if err := os.WriteFile(path, []byte(item.content), 0600); err != nil {
			return err
		}
	}
	canonicalRoot := pathutil.ForCompare(sourceRoot)
	if canonicalRoot == "" {
		return fmt.Errorf("source root cannot be empty")
	}
	p := indexPointer{Version: 1, SourceRoot: canonicalRoot, Generation: filepath.Base(stage)}
	pointerData, err := json.Marshal(p)
	if err != nil {
		return err
	}
	file, err := os.CreateTemp(envRoot, ".current-")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	if err := file.Chmod(0600); err != nil {
		file.Close()
		return err
	}
	if _, err := file.Write(pointerData); err != nil {
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
	// Readers see either the previous complete generation or the next one.
	if err := os.Rename(file.Name(), filepath.Join(envRoot, "current.json")); err != nil {
		return err
	}
	created = true
	// Project queries hold RLock, while this publish holds Lock. Retain
	// the published snapshot and the immediately previous generation.
	entries, err := os.ReadDir(genRoot)
	if err == nil {
		var previous string
		var previousTime int64
		for _, entry := range entries {
			if !entry.IsDir() || !strings.HasPrefix(entry.Name(), "gen-") || entry.Name() == p.Generation {
				continue
			}
			info, infoErr := entry.Info()
			if infoErr == nil && info.ModTime().UnixNano() >= previousTime {
				previous, previousTime = entry.Name(), info.ModTime().UnixNano()
			}
		}
		for _, entry := range entries {
			if entry.IsDir() && strings.HasPrefix(entry.Name(), "gen-") && entry.Name() != p.Generation && entry.Name() != previous {
				_ = os.RemoveAll(filepath.Join(genRoot, entry.Name()))
			}
		}
	}
	return nil
}
func (s *projectIndexStore) remove(id string) error {
	if !indexIDRE.MatchString(id) {
		return fmt.Errorf("invalid Environment ID")
	}
	if err := os.RemoveAll(s.environmentDir(id)); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}
