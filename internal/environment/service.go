package environment

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
	"ai-dev-manager-v2/internal/workspace"
)

const (
	StateReady            = "ready"
	DefaultWriterLeaseTTL = 5 * time.Minute
)

type Service struct {
	store          *store.Store
	workspaces     *workspace.Service
	now            func() time.Time
	writerLeaseTTL time.Duration
}

func New(s *store.Store, workspaces *workspace.Service) *Service {
	return &Service{
		store:          s,
		workspaces:     workspaces,
		now:            time.Now,
		writerLeaseTTL: DefaultWriterLeaseTTL,
	}
}

func (s *Service) WriterLeaseTTL() time.Duration { return s.leaseTTL() }

func (s *Service) Create(workspaceID, name, root string) (model.Environment, error) {
	return s.CreateWithRetention(workspaceID, name, root, model.ResourceRetention{})
}

func (s *Service) CreateWithRetention(workspaceID, name, root string, retention model.ResourceRetention) (model.Environment, error) {
	return s.createWithRetention(workspaceID, name, root, retention, true)
}

// CreateNewWithRetention creates a fresh Environment and refuses the ordinary
// idempotent duplicate shortcut. Temporary lifecycle creation uses this path so
// an existing durable Environment can never be returned as temporary success.
func (s *Service) CreateNewWithRetention(workspaceID, name, root string, retention model.ResourceRetention) (model.Environment, error) {
	return s.createWithRetention(workspaceID, name, root, retention, false)
}

func (s *Service) createWithRetention(workspaceID, name, root string, retention model.ResourceRetention, allowExisting bool) (model.Environment, error) {
	ws, err := s.workspaces.Get(strings.TrimSpace(workspaceID))
	if err != nil {
		return model.Environment{}, err
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return model.Environment{}, fmt.Errorf("environment name is required")
	}
	if strings.TrimSpace(root) == "" {
		root = ws.Path
	}
	root, err = canonicalDir(root)
	if err != nil {
		return model.Environment{}, err
	}
	if !within(ws.Path, root) {
		return model.Environment{}, fmt.Errorf("environment root must stay inside workspace %s", ws.Path)
	}

	var result model.Environment
	err = s.store.Update(func(state *model.State) error {
		now := s.nowUTC()
		for _, existing := range state.Environments {
			if existing.WorkspaceID == ws.ID && strings.EqualFold(existing.Name, name) && samePath(existing.Root, root) {
				if !allowExisting {
					return fmt.Errorf("environment conflicts with existing environment %s", existing.ID)
				}
				result = s.environmentView(existing, state, now)
				return nil
			}
		}
		created, createErr := s.newEnvironment(state, ws.ID, name, root, now, normalizeCreationRetention(retention, now))
		if createErr != nil {
			return createErr
		}
		state.Environments = append(state.Environments, created)
		result = s.environmentView(created, state, now)
		return nil
	})
	return result, err
}

func (s *Service) CreateManaged(workspaceID, name, root string, managed model.ManagedWorktree) (model.Environment, model.ManagedWorktree, error) {
	return s.CreateManagedWithRetention(workspaceID, name, root, managed, model.ResourceRetention{})
}

func (s *Service) CreateManagedWithRetention(workspaceID, name, root string, managed model.ManagedWorktree, retention model.ResourceRetention) (model.Environment, model.ManagedWorktree, error) {
	ws, err := s.workspaces.Get(strings.TrimSpace(workspaceID))
	if err != nil {
		return model.Environment{}, model.ManagedWorktree{}, err
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return model.Environment{}, model.ManagedWorktree{}, fmt.Errorf("environment name is required")
	}
	root, err = canonicalDir(root)
	if err != nil {
		return model.Environment{}, model.ManagedWorktree{}, err
	}
	managed.ID = strings.TrimSpace(managed.ID)
	if managed.ID == "" {
		return model.Environment{}, model.ManagedWorktree{}, fmt.Errorf("managed worktree id is required")
	}
	if strings.TrimSpace(managed.Branch) == "" || strings.TrimSpace(managed.BaseCommit) == "" || strings.TrimSpace(managed.GitCommonDir) == "" {
		return model.Environment{}, model.ManagedWorktree{}, fmt.Errorf("managed worktree git identity is incomplete")
	}

	var result model.Environment
	var managedResult model.ManagedWorktree
	err = s.store.Update(func(state *model.State) error {
		for _, existing := range state.ManagedWorktrees {
			if existing.ID == managed.ID || samePath(existing.Root, root) {
				return fmt.Errorf("managed worktree %q already exists", managed.ID)
			}
		}
		now := s.nowUTC()
		created, createErr := s.newEnvironment(state, ws.ID, name, root, now, normalizeCreationRetention(retention, now))
		if createErr != nil {
			return createErr
		}
		managed.EnvironmentID = created.ID
		managed.WorkspaceID = ws.ID
		managed.Root = root
		managed.CreatedAt = now
		managedResult = managed
		state.Environments = append(state.Environments, created)
		state.ManagedWorktrees = append(state.ManagedWorktrees, managedResult)
		result = s.environmentView(created, state, now)
		return nil
	})
	return result, managedResult, err
}

func (s *Service) newEnvironment(state *model.State, workspaceID, name, root string, now time.Time, retention model.ResourceRetention) (model.Environment, error) {
	id, err := identity.New("env")
	if err != nil {
		return model.Environment{}, err
	}
	result := model.Environment{
		ID:             id,
		WorkspaceID:    workspaceID,
		Name:           name,
		Root:           root,
		State:          StateReady,
		CreatedAt:      now,
		UpdatedAt:      now,
		LastActivityAt: now,
		Retention:      retention,
		PrivateMemory:  map[string]string{},
	}
	for _, entry := range state.MCPs {
		if entry.DefaultIncludeInEnv {
			result.EnabledMCPIDs = append(result.EnabledMCPIDs, entry.ID)
		}
	}
	for _, entry := range state.Skills {
		if entry.DefaultIncludeInEnv {
			result.EnabledSkillIDs = append(result.EnabledSkillIDs, entry.ID)
		}
	}
	sort.Strings(result.EnabledMCPIDs)
	sort.Strings(result.EnabledSkillIDs)
	return result, nil
}

func (s *Service) List() ([]model.Environment, error) {
	state, err := s.store.Load()
	if err != nil {
		return nil, err
	}
	items := make([]model.Environment, len(state.Environments))
	now := s.nowUTC()
	for i, env := range state.Environments {
		items[i] = s.environmentView(env, &state, now)
	}
	return items, nil
}

func (s *Service) Get(id string) (model.Environment, error) {
	state, err := s.store.Load()
	if err != nil {
		return model.Environment{}, err
	}
	now := s.nowUTC()
	for _, env := range state.Environments {
		if env.ID == id {
			return s.environmentView(env, &state, now), nil
		}
	}
	return model.Environment{}, fmt.Errorf("environment %q not found", id)
}

func (s *Service) Rename(id, name string) (model.Environment, error) {
	id = strings.TrimSpace(id)
	name = strings.TrimSpace(name)
	if name == "" {
		return model.Environment{}, fmt.Errorf("environment name is required")
	}
	var result model.Environment
	err := s.store.Update(func(state *model.State) error {
		idx := findEnvironment(state.Environments, id)
		if idx < 0 {
			return fmt.Errorf("environment %q not found", id)
		}
		env := &state.Environments[idx]
		env.Name = name
		env.UpdatedAt = s.nowUTC()
		result = *env
		return nil
	})
	return result, err
}

func (s *Service) WorkspaceOptions(id string) (model.EnvironmentWorkspaceOptions, error) {
	id = strings.TrimSpace(id)
	state, err := s.store.Load()
	if err != nil {
		return model.EnvironmentWorkspaceOptions{}, err
	}
	idx := findEnvironment(state.Environments, id)
	if idx < 0 {
		return model.EnvironmentWorkspaceOptions{}, fmt.Errorf("environment %q not found", id)
	}
	return s.workspaceOptionsForState(state.Environments[idx], &state), nil
}

func (s *Service) WorkspaceRecommendations() ([]model.EnvironmentWorkspaceRecommendation, error) {
	state, err := s.store.Load()
	if err != nil {
		return nil, err
	}
	items := make([]model.EnvironmentWorkspaceRecommendation, 0)
	for _, env := range state.Environments {
		options := s.workspaceOptionsForState(env, &state)
		if !options.RebindAllowed || options.RecommendedWorkspaceID == "" || options.RecommendedWorkspaceID == options.CurrentWorkspaceID {
			continue
		}
		var current model.Workspace
		var recommended model.Workspace
		for _, workspace := range state.Workspaces {
			switch workspace.ID {
			case options.CurrentWorkspaceID:
				current = workspace
			case options.RecommendedWorkspaceID:
				recommended = workspace
			}
		}
		if current.ID == "" || recommended.ID == "" {
			continue
		}
		items = append(items, model.EnvironmentWorkspaceRecommendation{
			EnvironmentID:        env.ID,
			EnvironmentName:      env.Name,
			Root:                 env.Root,
			CurrentWorkspace:     current,
			RecommendedWorkspace: recommended,
		})
	}
	sort.SliceStable(items, func(i, j int) bool {
		left := pathutil.ForCompare(items[i].Root)
		right := pathutil.ForCompare(items[j].Root)
		if left != right {
			return left < right
		}
		return strings.ToLower(items[i].EnvironmentName) < strings.ToLower(items[j].EnvironmentName)
	})
	if items == nil {
		items = []model.EnvironmentWorkspaceRecommendation{}
	}
	return items, nil
}

func (s *Service) workspaceOptionsForState(env model.Environment, state *model.State) model.EnvironmentWorkspaceOptions {
	options := model.EnvironmentWorkspaceOptions{
		EnvironmentID:      env.ID,
		CurrentWorkspaceID: env.WorkspaceID,
		Candidates:         []model.Workspace{},
		RebindAllowed:      true,
	}
	if findManagedWorktreeByEnvironment(state.ManagedWorktrees, env.ID) >= 0 {
		options.ManagedWorktree = true
		options.RebindAllowed = false
		options.Reason = "managed worktree Environment keeps its logical source Workspace binding"
		for _, ws := range state.Workspaces {
			if ws.ID == env.WorkspaceID {
				options.Candidates = append(options.Candidates, ws)
				options.RecommendedWorkspaceID = ws.ID
				break
			}
		}
		return options
	}
	for _, ws := range state.Workspaces {
		if ws.SystemKind != "" {
			continue
		}
		if pathutil.Within(ws.Path, env.Root) {
			options.Candidates = append(options.Candidates, ws)
		}
	}
	sort.SliceStable(options.Candidates, func(i, j int) bool {
		left := pathutil.ForCompare(options.Candidates[i].Path)
		right := pathutil.ForCompare(options.Candidates[j].Path)
		if len(left) != len(right) {
			return len(left) > len(right)
		}
		return strings.ToLower(options.Candidates[i].Name) < strings.ToLower(options.Candidates[j].Name)
	})
	if len(options.Candidates) == 0 {
		options.RebindAllowed = false
		options.Reason = "no registered non-system Workspace contains the Environment root"
		return options
	}
	options.RecommendedWorkspaceID = options.Candidates[0].ID
	return options
}

func (s *Service) SetWorkspace(id, workspaceID string) (model.Environment, error) {
	id = strings.TrimSpace(id)
	workspaceID = strings.TrimSpace(workspaceID)
	if workspaceID == "" {
		return model.Environment{}, fmt.Errorf("workspace id is required")
	}
	var result model.Environment
	err := s.store.Update(func(state *model.State) error {
		idx := findEnvironment(state.Environments, id)
		if idx < 0 {
			return fmt.Errorf("environment %q not found", id)
		}
		env := &state.Environments[idx]
		now := s.nowUTC()
		if env.WorkspaceID == workspaceID {
			result = s.environmentView(*env, state, now)
			return nil
		}
		if findManagedWorktreeByEnvironment(state.ManagedWorktrees, env.ID) >= 0 {
			return fmt.Errorf("managed worktree environment %s keeps logical source workspace %s and cannot be rebound by path", env.ID, env.WorkspaceID)
		}
		var target *model.Workspace
		for i := range state.Workspaces {
			if state.Workspaces[i].ID == workspaceID {
				target = &state.Workspaces[i]
				break
			}
		}
		if target == nil {
			return fmt.Errorf("workspace %q not found", workspaceID)
		}
		if target.SystemKind != "" {
			return fmt.Errorf("environment cannot be rebound to system workspace %s", target.ID)
		}
		if !pathutil.Within(target.Path, env.Root) {
			return fmt.Errorf("environment root %s is outside workspace %s", env.Root, target.Path)
		}
		env.WorkspaceID = target.ID
		env.UpdatedAt = now
		result = s.environmentView(*env, state, now)
		return nil
	})
	return result, err
}

func (s *Service) Remove(id string) (model.Environment, error) {
	id = strings.TrimSpace(id)
	var removed model.Environment
	err := s.store.Update(func(state *model.State) error {
		idx := findEnvironment(state.Environments, id)
		if idx < 0 {
			return fmt.Errorf("environment %q not found", id)
		}

		now := s.nowUTC()
		target := state.Environments[idx]
		for _, managed := range state.ManagedWorktrees {
			if managed.EnvironmentID == target.ID {
				return fmt.Errorf("environment %s is backed by managed worktree %s; use managed worktree destroy", target.ID, managed.ID)
			}
		}
		if target.State != StateReady {
			return fmt.Errorf("environment %s cannot be removed while state is %q", target.ID, target.State)
		}
		if target.Writer != nil && !s.writerExpired(target.Writer, now) {
			return fmt.Errorf("environment %s cannot be removed while writer %q is active", target.ID, target.Writer.Owner)
		}
		target.Writer = nil
		removed = target
		state.Environments = append(state.Environments[:idx], state.Environments[idx+1:]...)
		return nil
	})
	return removed, err
}

func (s *Service) RemoveManaged(id string) (model.Environment, model.ManagedWorktree, error) {
	id = strings.TrimSpace(id)
	var removed model.Environment
	var managedRemoved model.ManagedWorktree
	err := s.store.Update(func(state *model.State) error {
		envIdx := findEnvironment(state.Environments, id)
		if envIdx < 0 {
			return fmt.Errorf("environment %q not found", id)
		}
		managedIdx := findManagedWorktreeByEnvironment(state.ManagedWorktrees, id)
		if managedIdx < 0 {
			return fmt.Errorf("environment %s is not backed by a managed worktree", id)
		}
		target := state.Environments[envIdx]
		if target.State != StateReady {
			return fmt.Errorf("environment %s cannot be removed while state is %q", target.ID, target.State)
		}
		removed = target
		removed.Writer = nil
		managedRemoved = state.ManagedWorktrees[managedIdx]
		state.Environments = append(state.Environments[:envIdx], state.Environments[envIdx+1:]...)
		state.ManagedWorktrees = append(state.ManagedWorktrees[:managedIdx], state.ManagedWorktrees[managedIdx+1:]...)
		return nil
	})
	return removed, managedRemoved, err
}

func (s *Service) SetMCP(id, mcpID string, enabled bool) (model.Environment, error) {
	return s.setSelection(id, mcpID, enabled, true)
}

func (s *Service) SetSkill(id, skillID string, enabled bool) (model.Environment, error) {
	return s.setSelection(id, skillID, enabled, false)
}

func (s *Service) setSelection(id, value string, enabled, isMCP bool) (model.Environment, error) {
	var result model.Environment
	err := s.store.Update(func(state *model.State) error {
		idx := findEnvironment(state.Environments, id)
		if idx < 0 {
			return fmt.Errorf("environment %q not found", id)
		}
		env := &state.Environments[idx]
		values := &env.EnabledSkillIDs
		if isMCP {
			values = &env.EnabledMCPIDs
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
		now := s.nowUTC()
		env.UpdatedAt = now
		result = s.environmentView(*env, state, now)
		return nil
	})
	return result, err
}

func (s *Service) AcquireWriter(id, owner string) (model.Environment, error) {
	owner = strings.TrimSpace(owner)
	if owner == "" {
		return model.Environment{}, fmt.Errorf("writer owner is required")
	}
	var result model.Environment
	err := s.store.Update(func(state *model.State) error {
		idx := findEnvironment(state.Environments, id)
		if idx < 0 {
			return fmt.Errorf("environment %q not found", id)
		}
		target := &state.Environments[idx]
		now := s.nowUTC()
		if target.Writer != nil && s.writerExpired(target.Writer, now) {
			target.Writer = nil
			target.UpdatedAt = now
		}
		if target.Writer != nil && target.Writer.Owner != owner {
			return writerConflictError(target)
		}
		if conflict := s.activeWriterOverlap(state, idx, now); conflict != nil {
			return writerConflictError(conflict)
		}
		if target.Writer == nil {
			target.Writer = &model.WriterLease{Owner: owner, AcquiredAt: now}
		}
		s.renewWriter(target, now, false)
		result = *target
		return nil
	})
	return result, err
}

func (s *Service) ReleaseWriter(id, owner string, force bool) (model.Environment, error) {
	var result model.Environment
	err := s.store.Update(func(state *model.State) error {
		idx := findEnvironment(state.Environments, id)
		if idx < 0 {
			return fmt.Errorf("environment %q not found", id)
		}
		target := &state.Environments[idx]
		if target.Writer == nil {
			result = *target
			return nil
		}
		now := s.nowUTC()
		if s.writerExpired(target.Writer, now) {
			target.Writer = nil
			target.UpdatedAt = now
			result = *target
			return nil
		}
		if !force && target.Writer.Owner != strings.TrimSpace(owner) {
			return fmt.Errorf("writer is held by %q", target.Writer.Owner)
		}
		target.Writer = nil
		target.UpdatedAt = now
		result = *target
		return nil
	})
	return result, err
}

func (s *Service) RequireWriter(id, owner string) (model.Environment, error) {
	owner = strings.TrimSpace(owner)
	var result model.Environment
	err := s.store.Update(func(state *model.State) error {
		idx := findEnvironment(state.Environments, id)
		if idx < 0 {
			return fmt.Errorf("environment %q not found", id)
		}
		env := &state.Environments[idx]
		now := s.nowUTC()
		if env.Writer == nil || s.writerExpired(env.Writer, now) || env.Writer.Owner != owner {
			return fmt.Errorf("environment %s is not owned by writer %q", id, owner)
		}
		if conflict := s.activeWriterOverlap(state, idx, now); conflict != nil {
			return writerConflictError(conflict)
		}
		s.renewWriter(env, now, false)
		result = *env
		return nil
	})
	return result, err
}

func (s *Service) HeartbeatWriter(id, owner string) (model.Environment, error) {
	return s.RequireWriter(id, owner)
}

func (s *Service) Touch(id, owner string) error {
	return s.store.Update(func(state *model.State) error {
		idx := findEnvironment(state.Environments, id)
		if idx < 0 {
			return fmt.Errorf("environment %q not found", id)
		}
		env := &state.Environments[idx]
		now := s.nowUTC()
		if env.Writer == nil || s.writerExpired(env.Writer, now) || env.Writer.Owner != strings.TrimSpace(owner) {
			return fmt.Errorf("environment %s is not owned by writer %q", id, owner)
		}
		if conflict := s.activeWriterOverlap(state, idx, now); conflict != nil {
			return writerConflictError(conflict)
		}
		s.renewWriter(env, now, true)
		return nil
	})
}

func (s *Service) nowUTC() time.Time {
	if s.now == nil {
		return time.Now().UTC()
	}
	return s.now().UTC()
}

func (s *Service) writerExpired(lease *model.WriterLease, now time.Time) bool {
	if lease == nil {
		return false
	}
	if lease.ExpiresAt.IsZero() {
		return true
	}
	return !now.Before(lease.ExpiresAt)
}

func (s *Service) environmentView(env model.Environment, state *model.State, now time.Time) model.Environment {
	env.ExplicitMCPIDs = append([]string{}, env.EnabledMCPIDs...)
	env.ExplicitSkillIDs = append([]string{}, env.EnabledSkillIDs...)
	env.InheritedMCPIDs = []string{}
	env.InheritedSkillIDs = []string{}
	for _, workspace := range state.Workspaces {
		if workspace.ID != env.WorkspaceID {
			continue
		}
		env.InheritedMCPIDs = append([]string{}, workspace.EnabledMCPIDs...)
		env.InheritedSkillIDs = append([]string{}, workspace.EnabledSkillIDs...)
		env.EnabledMCPIDs = unionSorted(env.EnabledMCPIDs, workspace.EnabledMCPIDs)
		env.EnabledSkillIDs = unionSorted(env.EnabledSkillIDs, workspace.EnabledSkillIDs)
		break
	}
	if env.Writer == nil {
		return env
	}
	lease := *env.Writer
	if s.writerExpired(&lease, now) {
		env.Writer = nil
		return env
	}
	env.Writer = &lease
	return env
}

func unionSorted(left, right []string) []string {
	seen := make(map[string]struct{}, len(left)+len(right))
	values := make([]string, 0, len(left)+len(right))
	for _, group := range [][]string{left, right} {
		for _, value := range group {
			if _, ok := seen[value]; ok {
				continue
			}
			seen[value] = struct{}{}
			values = append(values, value)
		}
	}
	sort.Strings(values)
	return values
}

func (s *Service) renewWriter(env *model.Environment, now time.Time, activity bool) {
	env.Writer.LastSeenAt = now
	env.Writer.ExpiresAt = now.Add(s.leaseTTL())
	env.UpdatedAt = now
	if activity {
		env.LastActivityAt = now
	}
}

func (s *Service) leaseTTL() time.Duration {
	if s.writerLeaseTTL <= 0 {
		return DefaultWriterLeaseTTL
	}
	return s.writerLeaseTTL
}

func normalizeCreationRetention(retention model.ResourceRetention, now time.Time) model.ResourceRetention {
	if strings.TrimSpace(retention.Persistence) == "" {
		retention.Persistence = model.PersistenceDurable
	}
	if strings.TrimSpace(retention.CreatorSurface) == "" {
		retention.CreatorSurface = "core"
	}
	if retention.CreatedAt == nil {
		createdAt := now.UTC()
		retention.CreatedAt = &createdAt
	}
	return retention
}

func canonicalDir(path string) (string, error) {
	abs, err := filepath.Abs(filepath.Clean(path))
	if err != nil {
		return "", err
	}
	info, err := os.Stat(abs)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", fmt.Errorf("environment root is not a directory: %s", abs)
	}
	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return "", err
	}
	return filepath.Clean(resolved), nil
}

func within(base, target string) bool {
	rel, err := filepath.Rel(pathutil.ForCompare(base), pathutil.ForCompare(target))
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}

func findEnvironment(values []model.Environment, id string) int {
	for i := range values {
		if values[i].ID == id {
			return i
		}
	}
	return -1
}

func findManagedWorktreeByEnvironment(values []model.ManagedWorktree, environmentID string) int {
	for i := range values {
		if values[i].EnvironmentID == environmentID {
			return i
		}
	}
	return -1
}

func (s *Service) activeWriterOverlap(state *model.State, targetIndex int, now time.Time) *model.Environment {
	target := &state.Environments[targetIndex]
	for i := range state.Environments {
		if i == targetIndex {
			continue
		}
		other := &state.Environments[i]
		if other.Writer == nil || !writerRootsOverlap(other.Root, target.Root) {
			continue
		}
		if s.writerExpired(other.Writer, now) {
			other.Writer = nil
			other.UpdatedAt = now
			continue
		}
		return other
	}
	return nil
}

func writerConflictError(env *model.Environment) error {
	return fmt.Errorf("physical root overlaps active writer %q through environment %s (%s)", env.Writer.Owner, env.ID, env.Root)
}

func writerRootsOverlap(a, b string) bool {
	return pathutil.Within(a, b) || pathutil.Within(b, a)
}

func samePath(a, b string) bool { return pathutil.Same(a, b) }
