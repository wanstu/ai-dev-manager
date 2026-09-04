package workspace

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"ai-dev-manager/internal/config"
	"ai-dev-manager/internal/model"
)

// Input contains the persistent workspace fields callers may set. IDs are
// generated and owned by the registry.
type Input struct {
	Path      string
	ProfileID string
	RuntimeID string
}

type ErrorKind string

const (
	ErrInvalidPath   ErrorKind = "invalid_path"
	ErrDuplicatePath ErrorKind = "duplicate_path"
	ErrNotFound      ErrorKind = "not_found"
	ErrIDGeneration  ErrorKind = "id_generation"
)

type RegistryError struct {
	Kind ErrorKind
	ID   string
	Path string
	Err  error
}

func (e *RegistryError) Error() string {
	switch e.Kind {
	case ErrInvalidPath:
		return fmt.Sprintf("invalid workspace path %q", e.Path)
	case ErrDuplicatePath:
		return fmt.Sprintf("workspace path already registered: %q", e.Path)
	case ErrNotFound:
		return fmt.Sprintf("workspace %q not found", e.ID)
	case ErrIDGeneration:
		return fmt.Sprintf("generate workspace id: %v", e.Err)
	default:
		return "workspace registry error"
	}
}

func (e *RegistryError) Unwrap() error { return e.Err }

// Registry persists workspace identity through the user-level config store.
// A registry instance serializes read-modify-write operations so concurrent
// callers cannot overwrite one another within the same process.
type Registry struct {
	store *config.Store
	mu    sync.Mutex
}

func NewRegistry(store *config.Store) *Registry {
	return &Registry{store: store}
}

func (r *Registry) Add(input Input) (model.Workspace, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	canonical, err := canonicalWorkspacePath(input.Path)
	if err != nil {
		return model.Workspace{}, err
	}
	cfg, err := r.store.LoadUserConfig()
	if err != nil {
		return model.Workspace{}, err
	}
	if duplicateWorkspacePath(cfg.Workspaces, canonical, "") {
		return model.Workspace{}, &RegistryError{Kind: ErrDuplicatePath, Path: canonical}
	}

	id, err := newWorkspaceID()
	if err != nil {
		return model.Workspace{}, err
	}
	workspace := model.Workspace{
		ID:        id,
		Path:      canonical,
		ProfileID: input.ProfileID,
		RuntimeID: input.RuntimeID,
	}
	cfg.Workspaces = append(cfg.Workspaces, workspace)
	if err := r.store.SaveUserConfig(cfg); err != nil {
		return model.Workspace{}, err
	}
	return workspace, nil
}

func (r *Registry) Get(id string) (model.Workspace, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	cfg, err := r.store.LoadUserConfig()
	if err != nil {
		return model.Workspace{}, err
	}
	for _, workspace := range cfg.Workspaces {
		if workspace.ID == id {
			return workspace, nil
		}
	}
	return model.Workspace{}, &RegistryError{Kind: ErrNotFound, ID: id}
}

func (r *Registry) List() ([]model.Workspace, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	cfg, err := r.store.LoadUserConfig()
	if err != nil {
		return nil, err
	}
	workspaces := make([]model.Workspace, len(cfg.Workspaces))
	copy(workspaces, cfg.Workspaces)
	return workspaces, nil
}

func (r *Registry) Update(id string, input Input) (model.Workspace, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	canonical, err := canonicalWorkspacePath(input.Path)
	if err != nil {
		return model.Workspace{}, err
	}
	cfg, err := r.store.LoadUserConfig()
	if err != nil {
		return model.Workspace{}, err
	}
	index := -1
	for i, workspace := range cfg.Workspaces {
		if workspace.ID == id {
			index = i
			break
		}
	}
	if index < 0 {
		return model.Workspace{}, &RegistryError{Kind: ErrNotFound, ID: id}
	}
	if duplicateWorkspacePath(cfg.Workspaces, canonical, id) {
		return model.Workspace{}, &RegistryError{Kind: ErrDuplicatePath, Path: canonical}
	}

	updated := model.Workspace{
		ID:        id,
		Path:      canonical,
		ProfileID: input.ProfileID,
		RuntimeID: input.RuntimeID,
	}
	cfg.Workspaces[index] = updated
	if err := r.store.SaveUserConfig(cfg); err != nil {
		return model.Workspace{}, err
	}
	return updated, nil
}

func (r *Registry) Remove(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	cfg, err := r.store.LoadUserConfig()
	if err != nil {
		return err
	}
	index := -1
	for i, workspace := range cfg.Workspaces {
		if workspace.ID == id {
			index = i
			break
		}
	}
	if index < 0 {
		return &RegistryError{Kind: ErrNotFound, ID: id}
	}
	cfg.Workspaces = append(cfg.Workspaces[:index], cfg.Workspaces[index+1:]...)
	return r.store.SaveUserConfig(cfg)
}

func canonicalWorkspacePath(path string) (string, error) {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" || !filepath.IsAbs(trimmed) {
		return "", &RegistryError{Kind: ErrInvalidPath, Path: path}
	}
	absolute, err := filepath.Abs(filepath.Clean(trimmed))
	if err != nil {
		return "", &RegistryError{Kind: ErrInvalidPath, Path: path, Err: err}
	}
	info, err := os.Stat(absolute)
	if err != nil {
		return "", &RegistryError{Kind: ErrInvalidPath, Path: absolute, Err: err}
	}
	if !info.IsDir() {
		return "", &RegistryError{Kind: ErrInvalidPath, Path: absolute, Err: errors.New("path is not a directory")}
	}
	return filepath.Clean(absolute), nil
}

// Workspace identity follows Windows path semantics for the current product:
// drive/path casing differences must not create duplicate registrations.
func sameWorkspacePath(left, right string) bool {
	return strings.EqualFold(filepath.Clean(left), filepath.Clean(right))
}

func duplicateWorkspacePath(workspaces []model.Workspace, path, excludeID string) bool {
	for _, workspace := range workspaces {
		if workspace.ID == excludeID {
			continue
		}
		if sameWorkspacePath(workspace.Path, path) {
			return true
		}
	}
	return false
}

func newWorkspaceID() (string, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", &RegistryError{Kind: ErrIDGeneration, Err: err}
	}
	return "ws_" + hex.EncodeToString(bytes), nil
}
