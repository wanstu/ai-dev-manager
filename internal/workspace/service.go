package workspace

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"ai-dev-manager-v2/internal/identity"
	"ai-dev-manager-v2/internal/model"
	"ai-dev-manager-v2/internal/pathutil"
	"ai-dev-manager-v2/internal/store"
)

type Service struct{ store *store.Store }

func New(s *store.Store) *Service { return &Service{store: s} }

func (s *Service) Add(path, name string) (model.Workspace, error) {
	root, err := canonicalDir(path)
	if err != nil {
		return model.Workspace{}, err
	}
	name = strings.TrimSpace(name)
	if name == "" {
		name = filepath.Base(root)
		if name == "." || name == string(filepath.Separator) || name == "" {
			name = root
		}
	}
	var result model.Workspace
	err = s.store.Update(func(state *model.State) error {
		for _, ws := range state.Workspaces {
			if samePath(ws.Path, root) {
				return fmt.Errorf("workspace path already registered as %s", ws.ID)
			}
		}
		id, err := identity.New("ws")
		if err != nil {
			return err
		}
		result = model.Workspace{ID: id, Name: name, Path: root, CreatedAt: time.Now().UTC()}
		state.Workspaces = append(state.Workspaces, result)
		return nil
	})
	return result, err
}

func (s *Service) List() ([]model.Workspace, error) {
	state, err := s.store.Load()
	if err != nil {
		return nil, err
	}
	items := make([]model.Workspace, len(state.Workspaces))
	copy(items, state.Workspaces)
	return items, nil
}

func (s *Service) Get(id string) (model.Workspace, error) {
	id = strings.TrimSpace(id)
	state, err := s.store.Load()
	if err != nil {
		return model.Workspace{}, err
	}
	idx := findWorkspace(state.Workspaces, id)
	if idx < 0 {
		return model.Workspace{}, fmt.Errorf("workspace %q not found", id)
	}
	return state.Workspaces[idx], nil
}

func (s *Service) Rename(id, name string) (model.Workspace, error) {
	id = strings.TrimSpace(id)
	name = strings.TrimSpace(name)
	if name == "" {
		return model.Workspace{}, fmt.Errorf("workspace name is required")
	}
	var result model.Workspace
	err := s.store.Update(func(state *model.State) error {
		idx := findWorkspace(state.Workspaces, id)
		if idx < 0 {
			return fmt.Errorf("workspace %q not found", id)
		}
		state.Workspaces[idx].Name = name
		result = state.Workspaces[idx]
		return nil
	})
	return result, err
}

func (s *Service) SetMCP(id, mcpID string, enabled bool) (model.Workspace, error) {
	return s.setSelection(id, mcpID, enabled, true)
}

func (s *Service) SetSkill(id, skillID string, enabled bool) (model.Workspace, error) {
	return s.setSelection(id, skillID, enabled, false)
}

func (s *Service) setSelection(id, value string, enabled, isMCP bool) (model.Workspace, error) {
	id = strings.TrimSpace(id)
	value = strings.TrimSpace(value)
	if value == "" {
		return model.Workspace{}, fmt.Errorf("selection id is required")
	}
	var result model.Workspace
	err := s.store.Update(func(state *model.State) error {
		idx := findWorkspace(state.Workspaces, id)
		if idx < 0 {
			return fmt.Errorf("workspace %q not found", id)
		}
		workspace := &state.Workspaces[idx]
		values := &workspace.EnabledSkillIDs
		if isMCP {
			values = &workspace.EnabledMCPIDs
		}
		found := -1
		for i, current := range *values {
			if current == value {
				found = i
				break
			}
		}
		if enabled && found < 0 {
			*values = append(*values, value)
			sort.Strings(*values)
		}
		if !enabled && found >= 0 {
			*values = append((*values)[:found], (*values)[found+1:]...)
		}
		result = *workspace
		return nil
	})
	return result, err
}

func (s *Service) Remove(id string) (model.Workspace, error) {
	id = strings.TrimSpace(id)
	var removed model.Workspace
	err := s.store.Update(func(state *model.State) error {
		idx := findWorkspace(state.Workspaces, id)
		if idx < 0 {
			return fmt.Errorf("workspace %q not found", id)
		}
		if state.Workspaces[idx].SystemKind != "" {
			return fmt.Errorf("workspace %s is system-managed (%s) and cannot be removed manually", id, state.Workspaces[idx].SystemKind)
		}
		for _, env := range state.Environments {
			if env.WorkspaceID == id {
				return fmt.Errorf("workspace %s cannot be removed while environment %s references it", id, env.ID)
			}
		}
		removed = state.Workspaces[idx]
		state.Workspaces = append(state.Workspaces[:idx], state.Workspaces[idx+1:]...)
		return nil
	})
	return removed, err
}

func findWorkspace(values []model.Workspace, id string) int {
	for i := range values {
		if values[i].ID == id {
			return i
		}
	}
	return -1
}

func canonicalDir(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", fmt.Errorf("workspace path is required")
	}
	abs, err := filepath.Abs(filepath.Clean(path))
	if err != nil {
		return "", err
	}
	info, err := os.Stat(abs)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", fmt.Errorf("workspace path is not a directory: %s", abs)
	}
	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return "", err
	}
	return filepath.Clean(resolved), nil
}

func samePath(a, b string) bool {
	return pathutil.Same(a, b)
}
