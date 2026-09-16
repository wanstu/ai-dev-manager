package isolation

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"ai-dev-manager-v2/internal/environment"
	"ai-dev-manager-v2/internal/identity"
	"ai-dev-manager-v2/internal/model"
	"ai-dev-manager-v2/internal/pathutil"
	"ai-dev-manager-v2/internal/store"
	"ai-dev-manager-v2/internal/workspace"
)

type Service struct {
	store                    *store.Store
	workspaces               *workspace.Service
	environments             *environment.Service
	now                      func() time.Time
	createManagedEnvironment func(string, string, string, model.ManagedWorktree, model.ResourceRetention) (model.Environment, model.ManagedWorktree, error)
}

type CreateResult struct {
	ManagedWorktree model.ManagedWorktree `json:"managed_worktree"`
	Environment     model.Environment     `json:"environment"`
}

type DestroyResult struct {
	ManagedWorktreeID string `json:"managed_worktree_id"`
	EnvironmentID     string `json:"environment_id"`
	Root              string `json:"root"`
	RetainedBranch    string `json:"retained_branch"`
	Forced            bool   `json:"forced"`
}

type SafetyStatus struct {
	Dirty       bool   `json:"dirty"`
	Unpublished bool   `json:"unpublished"`
	Head        string `json:"head"`
}

func New(s *store.Store, workspaces *workspace.Service, environments *environment.Service) *Service {
	return &Service{store: s, workspaces: workspaces, environments: environments, now: time.Now}
}

func (s *Service) List() ([]model.ManagedWorktree, error) {
	state, err := s.store.Load()
	if err != nil {
		return nil, err
	}
	items := append([]model.ManagedWorktree(nil), state.ManagedWorktrees...)
	sort.Slice(items, func(i, j int) bool {
		if items[i].CreatedAt.Equal(items[j].CreatedAt) {
			return items[i].ID < items[j].ID
		}
		return items[i].CreatedAt.Before(items[j].CreatedAt)
	})
	return items, nil
}

func (s *Service) GetByEnvironment(environmentID string) (model.ManagedWorktree, bool, error) {
	environmentID = strings.TrimSpace(environmentID)
	state, err := s.store.Load()
	if err != nil {
		return model.ManagedWorktree{}, false, err
	}
	for _, item := range state.ManagedWorktrees {
		if item.EnvironmentID == environmentID {
			return item, true, nil
		}
	}
	return model.ManagedWorktree{}, false, nil
}

func (s *Service) Create(ctx context.Context, workspaceID, name, baseRef string) (CreateResult, error) {
	return s.CreateWithRetention(ctx, workspaceID, name, baseRef, model.ResourceRetention{})
}

func (s *Service) CreateWithRetention(ctx context.Context, workspaceID, name, baseRef string, retention model.ResourceRetention) (CreateResult, error) {
	return s.createWithRetention(ctx, strings.TrimSpace(workspaceID), "", name, baseRef, retention)
}

func (s *Service) CreateFromEnvironment(ctx context.Context, sourceEnvironmentID, name, baseRef string) (CreateResult, error) {
	return s.CreateFromEnvironmentWithRetention(ctx, sourceEnvironmentID, name, baseRef, model.ResourceRetention{})
}

func (s *Service) CreateFromEnvironmentWithRetention(ctx context.Context, sourceEnvironmentID, name, baseRef string, retention model.ResourceRetention) (CreateResult, error) {
	return s.createWithRetention(ctx, "", strings.TrimSpace(sourceEnvironmentID), name, baseRef, retention)
}

func (s *Service) createWithRetention(ctx context.Context, workspaceID, sourceEnvironmentID, name, baseRef string, retention model.ResourceRetention) (CreateResult, error) {
	var (
		ws         model.Workspace
		sourceRoot string
		err        error
	)
	if sourceEnvironmentID != "" {
		sourceEnvironment, getErr := s.environments.Get(sourceEnvironmentID)
		if getErr != nil {
			return CreateResult{}, getErr
		}
		ws, err = s.workspaces.Get(sourceEnvironment.WorkspaceID)
		if err != nil {
			return CreateResult{}, err
		}
		sourceRoot = sourceEnvironment.Root
		sourceEnvironmentID = sourceEnvironment.ID
	} else {
		ws, err = s.workspaces.Get(workspaceID)
		if err != nil {
			return CreateResult{}, err
		}
		sourceRoot = ws.Path
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return CreateResult{}, fmt.Errorf("environment name is required")
	}
	baseRef = strings.TrimSpace(baseRef)
	if strings.HasPrefix(baseRef, "-") || strings.ContainsRune(baseRef, '\x00') {
		return CreateResult{}, fmt.Errorf("invalid base_ref %q", baseRef)
	}
	if _, err := exec.LookPath("git"); err != nil {
		return CreateResult{}, fmt.Errorf("git worktree isolation is unavailable: %w", err)
	}

	top, err := s.gitOutput(ctx, sourceRoot, "rev-parse", "--show-toplevel")
	if err != nil {
		if sourceEnvironmentID != "" {
			return CreateResult{}, fmt.Errorf("source environment %s is not a usable Git worktree: %w", sourceEnvironmentID, err)
		}
		return CreateResult{}, fmt.Errorf("workspace %s is not a usable Git worktree: %w", ws.ID, err)
	}
	topDir, err := canonicalExistingDir(strings.TrimSpace(top))
	if err != nil {
		return CreateResult{}, fmt.Errorf("resolve Git top-level: %w", err)
	}
	if !samePath(topDir, sourceRoot) {
		if sourceEnvironmentID != "" {
			return CreateResult{}, fmt.Errorf("managed worktree creation requires source Environment root %s to be the Git top-level %s", sourceRoot, topDir)
		}
		return CreateResult{}, fmt.Errorf("managed worktree creation requires workspace root %s to be the Git top-level %s", sourceRoot, topDir)
	}
	commonRaw, err := s.gitOutput(ctx, sourceRoot, "rev-parse", "--git-common-dir")
	if err != nil {
		return CreateResult{}, err
	}
	commonDir, err := canonicalGitPath(sourceRoot, strings.TrimSpace(commonRaw))
	if err != nil {
		return CreateResult{}, fmt.Errorf("resolve Git common directory: %w", err)
	}
	if _, err := s.gitOutput(ctx, sourceRoot, "fetch", "--all", "--prune"); err != nil {
		return CreateResult{}, fmt.Errorf("fetch source Git refs: %w", err)
	}
	resolvedBaseRef := baseRef
	if resolvedBaseRef == "" {
		upstream, upstreamErr := s.gitOutput(ctx, sourceRoot, "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{upstream}")
		if upstreamErr == nil && strings.TrimSpace(upstream) != "" {
			resolvedBaseRef = strings.TrimSpace(upstream)
		} else {
			remotes, remoteErr := s.gitOutput(ctx, sourceRoot, "remote")
			if remoteErr != nil {
				return CreateResult{}, fmt.Errorf("inspect source Git remotes: %w", remoteErr)
			}
			if strings.TrimSpace(remotes) != "" {
				return CreateResult{}, fmt.Errorf("source Git branch has no upstream after fetch; pass base_ref explicitly")
			}
			resolvedBaseRef = "HEAD"
		}
	}
	baseCommit, err := s.gitOutput(ctx, sourceRoot, "rev-parse", "--verify", "--end-of-options", resolvedBaseRef+"^{commit}")
	if err != nil {
		return CreateResult{}, fmt.Errorf("resolve base_ref %q: %w", resolvedBaseRef, err)
	}
	baseCommit = strings.TrimSpace(baseCommit)

	managedID, err := identity.New("wt")
	if err != nil {
		return CreateResult{}, err
	}
	settings, err := s.ensureSettingsWorkspace()
	if err != nil {
		return CreateResult{}, err
	}
	branch := settings.BranchPrefix + managedID
	root := filepath.Join(settings.Root, ws.ID, managedID)
	if err := os.MkdirAll(filepath.Dir(root), 0o755); err != nil {
		return CreateResult{}, err
	}
	if _, err := os.Stat(root); err == nil {
		return CreateResult{}, fmt.Errorf("managed worktree destination already exists: %s", root)
	} else if !os.IsNotExist(err) {
		return CreateResult{}, err
	}

	if _, err := s.gitOutput(ctx, sourceRoot, "worktree", "add", "-b", branch, root, baseCommit); err != nil {
		return CreateResult{}, fmt.Errorf("create managed worktree: %w", err)
	}
	rollback := true
	defer func() {
		if !rollback {
			return
		}
		_, _ = s.gitOutput(context.Background(), sourceRoot, "worktree", "remove", "--force", root)
		_, _ = s.gitOutput(context.Background(), sourceRoot, "branch", "-D", branch)
		_ = os.RemoveAll(root)
	}()

	managed := model.ManagedWorktree{
		ID:                  managedID,
		SourceEnvironmentID: sourceEnvironmentID,
		SourceRoot:          sourceRoot,
		Branch:              branch,
		BaseCommit:          baseCommit,
		GitCommonDir:        commonDir,
		CreatedAt:           s.nowUTC(),
	}
	createManagedEnvironment := s.createManagedEnvironment
	if createManagedEnvironment == nil {
		createManagedEnvironment = s.environments.CreateManagedWithRetention
	}
	env, persisted, err := createManagedEnvironment(ws.ID, name, root, managed, retention)
	if err != nil {
		return CreateResult{}, err
	}
	rollback = false
	return CreateResult{ManagedWorktree: persisted, Environment: env}, nil
}

func (s *Service) ValidateEnvironment(ctx context.Context, env model.Environment) error {
	ws, err := s.workspaces.Get(env.WorkspaceID)
	if err != nil {
		return err
	}
	managed, ok, err := s.GetByEnvironment(env.ID)
	if err != nil {
		return err
	}
	if !ok {
		if within(ws.Path, env.Root) {
			return nil
		}
		return fmt.Errorf("environment %s root %s is outside workspace %s without managed worktree metadata", env.ID, env.Root, ws.Path)
	}
	if managed.EnvironmentID != env.ID || managed.WorkspaceID != env.WorkspaceID || !samePath(managed.Root, env.Root) {
		return fmt.Errorf("managed worktree metadata does not match environment %s", env.ID)
	}
	settings, err := s.Settings()
	if err != nil {
		return err
	}
	if !within(settings.Root, managed.Root) {
		return fmt.Errorf("managed worktree %s root escapes configured ADM worktree root", managed.ID)
	}
	root, err := canonicalExistingDir(managed.Root)
	if err != nil {
		return fmt.Errorf("managed worktree %s root is missing or invalid: %w", managed.ID, err)
	}
	if !samePath(root, managed.Root) {
		return fmt.Errorf("managed worktree %s root identity changed", managed.ID)
	}
	top, err := s.gitOutput(ctx, root, "rev-parse", "--show-toplevel")
	if err != nil {
		return fmt.Errorf("managed worktree %s Git top-level check failed: %w", managed.ID, err)
	}
	topDir, err := canonicalExistingDir(strings.TrimSpace(top))
	if err != nil || !samePath(topDir, root) {
		return fmt.Errorf("managed worktree %s Git top-level no longer matches root", managed.ID)
	}
	commonRaw, err := s.gitOutput(ctx, root, "rev-parse", "--git-common-dir")
	if err != nil {
		return fmt.Errorf("managed worktree %s Git common-dir check failed: %w", managed.ID, err)
	}
	commonDir, err := canonicalGitPath(root, strings.TrimSpace(commonRaw))
	if err != nil || !samePath(commonDir, managed.GitCommonDir) {
		return fmt.Errorf("managed worktree %s Git common directory changed", managed.ID)
	}
	branch, err := s.gitOutput(ctx, root, "branch", "--show-current")
	if err != nil {
		return fmt.Errorf("managed worktree %s branch check failed: %w", managed.ID, err)
	}
	if strings.TrimSpace(branch) != managed.Branch {
		return fmt.Errorf("managed worktree %s branch changed: got %q want %q", managed.ID, strings.TrimSpace(branch), managed.Branch)
	}
	return nil
}

func (s *Service) Safety(ctx context.Context, environmentID string) (SafetyStatus, error) {
	env, err := s.environments.Get(strings.TrimSpace(environmentID))
	if err != nil {
		return SafetyStatus{}, err
	}
	managed, ok, err := s.GetByEnvironment(env.ID)
	if err != nil {
		return SafetyStatus{}, err
	}
	if !ok {
		return SafetyStatus{}, fmt.Errorf("environment %s is not backed by a managed worktree", env.ID)
	}
	if err := s.ValidateEnvironment(ctx, env); err != nil {
		return SafetyStatus{}, err
	}
	status, err := s.gitOutput(ctx, managed.Root, "status", "--porcelain=v1", "--untracked-files=all")
	if err != nil {
		return SafetyStatus{}, err
	}
	head, err := s.gitOutput(ctx, managed.Root, "rev-parse", "HEAD")
	if err != nil {
		return SafetyStatus{}, err
	}
	head = strings.TrimSpace(head)
	unpublished := false
	if head != managed.BaseCommit {
		remoteRefs, err := s.gitOutput(ctx, managed.Root, "branch", "-r", "--contains", head, "--format=%(refname:short)")
		if err != nil {
			return SafetyStatus{}, err
		}
		unpublished = strings.TrimSpace(remoteRefs) == ""
	}
	return SafetyStatus{Dirty: strings.TrimSpace(status) != "", Unpublished: unpublished, Head: head}, nil
}

func (s *Service) Destroy(ctx context.Context, environmentID, writerOwner string, force bool) (DestroyResult, error) {
	env, err := s.environments.RequireWriter(strings.TrimSpace(environmentID), strings.TrimSpace(writerOwner))
	if err != nil {
		return DestroyResult{}, err
	}
	managed, ok, err := s.GetByEnvironment(env.ID)
	if err != nil {
		return DestroyResult{}, err
	}
	if !ok {
		return DestroyResult{}, fmt.Errorf("environment %s is not backed by a managed worktree", env.ID)
	}
	if err := s.ValidateEnvironment(ctx, env); err != nil {
		return DestroyResult{}, err
	}
	safety, err := s.Safety(ctx, env.ID)
	if err != nil {
		return DestroyResult{}, err
	}
	if !force && (safety.Dirty || safety.Unpublished) {
		reasons := make([]string, 0, 2)
		if safety.Dirty {
			reasons = append(reasons, "dirty worktree")
		}
		if safety.Unpublished {
			reasons = append(reasons, "locally advanced HEAD is not present in any remote-tracking ref")
		}
		return DestroyResult{}, fmt.Errorf("managed worktree %s destroy refused: %s; retry with force=true only after review", managed.ID, strings.Join(reasons, ", "))
	}
	sourceRoot := strings.TrimSpace(managed.SourceRoot)
	if sourceRoot == "" {
		ws, getErr := s.workspaces.Get(managed.WorkspaceID)
		if getErr != nil {
			return DestroyResult{}, getErr
		}
		sourceRoot = ws.Path
	}
	args := []string{"worktree", "remove"}
	if force {
		args = append(args, "--force")
	}
	args = append(args, managed.Root)
	if _, err := s.gitOutput(ctx, sourceRoot, args...); err != nil {
		return DestroyResult{}, fmt.Errorf("remove managed worktree %s: %w", managed.ID, err)
	}
	if _, _, err := s.environments.RemoveManaged(env.ID); err != nil {
		return DestroyResult{}, fmt.Errorf("managed worktree removed from Git but ADM metadata cleanup failed: %w", err)
	}
	_ = os.Remove(filepath.Dir(managed.Root))
	return DestroyResult{
		ManagedWorktreeID: managed.ID,
		EnvironmentID:     env.ID,
		Root:              managed.Root,
		RetainedBranch:    managed.Branch,
		Forced:            force,
	}, nil
}

func (s *Service) Settings() (model.WorktreeSettings, error) {
	state, err := s.store.Load()
	if err != nil {
		return model.WorktreeSettings{}, err
	}
	return s.effectiveSettings(state.WorktreeSettings), nil
}

func (s *Service) ensureSettingsWorkspace() (model.WorktreeSettings, error) {
	state, err := s.store.Load()
	if err != nil {
		return model.WorktreeSettings{}, err
	}
	settings := s.effectiveSettings(state.WorktreeSettings)
	if settings.WorkspaceID != "" {
		for _, workspace := range state.Workspaces {
			if workspace.ID == settings.WorkspaceID && workspace.SystemKind == model.WorkspaceSystemKindManagedWorktreeRoot && samePath(workspace.Path, settings.Root) {
				return settings, nil
			}
		}
	}
	return s.UpdateSettings(settings.Root, settings.BranchPrefix)
}

func (s *Service) UpdateSettings(root, branchPrefix string) (model.WorktreeSettings, error) {
	root = strings.TrimSpace(root)
	if root == "" {
		root = s.defaultOwnedRoot()
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return model.WorktreeSettings{}, fmt.Errorf("create worktree root: %w", err)
	}
	resolvedRoot, err := canonicalExistingDir(root)
	if err != nil {
		return model.WorktreeSettings{}, fmt.Errorf("resolve worktree root: %w", err)
	}
	branchPrefix, err = normalizeBranchPrefix(branchPrefix)
	if err != nil {
		return model.WorktreeSettings{}, err
	}

	var result model.WorktreeSettings
	err = s.store.Update(func(state *model.State) error {
		current := s.effectiveSettings(state.WorktreeSettings)
		rootChanging := !samePath(current.Root, resolvedRoot)
		if rootChanging && len(state.ManagedWorktrees) != 0 {
			return fmt.Errorf("worktree root cannot change while %d managed worktree(s) exist", len(state.ManagedWorktrees))
		}
		if rootChanging && current.WorkspaceID != "" {
			for _, environment := range state.Environments {
				if environment.WorkspaceID == current.WorkspaceID {
					return fmt.Errorf("worktree root cannot change while environment %s references the system workspace", environment.ID)
				}
			}
		}

		workspaceIndex := -1
		if current.WorkspaceID != "" {
			for i := range state.Workspaces {
				if state.Workspaces[i].ID == current.WorkspaceID {
					workspaceIndex = i
					break
				}
			}
		}
		if workspaceIndex < 0 {
			for i := range state.Workspaces {
				if state.Workspaces[i].SystemKind == model.WorkspaceSystemKindManagedWorktreeRoot {
					workspaceIndex = i
					break
				}
			}
		}
		for i := range state.Workspaces {
			if !samePath(state.Workspaces[i].Path, resolvedRoot) {
				continue
			}
			if workspaceIndex >= 0 && i != workspaceIndex {
				return fmt.Errorf("configured worktree root is already registered as workspace %s", state.Workspaces[i].ID)
			}
			if workspaceIndex < 0 {
				workspaceIndex = i
			}
		}
		if workspaceIndex < 0 {
			workspaceID, idErr := identity.New("ws")
			if idErr != nil {
				return idErr
			}
			state.Workspaces = append(state.Workspaces, model.Workspace{
				ID:         workspaceID,
				Name:       "ADM Worktrees",
				Path:       resolvedRoot,
				CreatedAt:  s.nowUTC(),
				SystemKind: model.WorkspaceSystemKindManagedWorktreeRoot,
			})
			workspaceIndex = len(state.Workspaces) - 1
		} else {
			workspace := &state.Workspaces[workspaceIndex]
			workspace.Path = resolvedRoot
			workspace.SystemKind = model.WorkspaceSystemKindManagedWorktreeRoot
			if strings.TrimSpace(workspace.Name) == "" {
				workspace.Name = "ADM Worktrees"
			}
		}

		result = model.WorktreeSettings{
			Root:         resolvedRoot,
			BranchPrefix: branchPrefix,
			WorkspaceID:  state.Workspaces[workspaceIndex].ID,
		}
		state.WorktreeSettings = result
		return nil
	})
	return result, err
}

func (s *Service) effectiveSettings(settings model.WorktreeSettings) model.WorktreeSettings {
	if strings.TrimSpace(settings.Root) == "" {
		settings.Root = s.defaultOwnedRoot()
	} else if abs, err := filepath.Abs(settings.Root); err == nil {
		settings.Root = filepath.Clean(abs)
	}
	if strings.TrimSpace(settings.BranchPrefix) == "" {
		settings.BranchPrefix = "adm/"
	}
	return settings
}

func (s *Service) defaultOwnedRoot() string {
	root := filepath.Join(filepath.Dir(s.store.Path()), "worktrees")
	abs, err := filepath.Abs(root)
	if err == nil {
		root = abs
	}
	return filepath.Clean(root)
}

// ownedRoot remains the single validation helper used by isolation tests and
// callers that only need the effective root path.
func (s *Service) ownedRoot() string {
	settings, err := s.Settings()
	if err == nil && strings.TrimSpace(settings.Root) != "" {
		return pathutil.ForCompare(settings.Root)
	}
	return pathutil.ForCompare(s.defaultOwnedRoot())
}

func normalizeBranchPrefix(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		value = "adm/"
	}
	candidate := value + "wt_probe"
	if strings.HasPrefix(candidate, "-") || strings.HasPrefix(candidate, ".") || strings.HasSuffix(candidate, ".") || strings.HasSuffix(candidate, "/") || strings.Contains(candidate, "..") || strings.Contains(candidate, "//") || strings.Contains(candidate, "@{") || strings.ContainsAny(candidate, " ~^:?*[\\") {
		return "", fmt.Errorf("invalid worktree branch prefix %q", value)
	}
	return value, nil
}

func (s *Service) gitOutput(ctx context.Context, dir string, args ...string) (string, error) {
	git, err := exec.LookPath("git")
	if err != nil {
		return "", err
	}
	fullArgs := append([]string{"-C", dir}, args...)
	cmd := exec.CommandContext(ctx, git, fullArgs...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %s: %s", strings.Join(args, " "), strings.TrimSpace(string(out)))
	}
	return string(out), nil
}

func (s *Service) nowUTC() time.Time {
	if s.now == nil {
		return time.Now().UTC()
	}
	return s.now().UTC()
}

func canonicalExistingDir(path string) (string, error) {
	abs, err := filepath.Abs(filepath.Clean(path))
	if err != nil {
		return "", err
	}
	info, err := os.Stat(abs)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", fmt.Errorf("not a directory: %s", abs)
	}
	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return "", err
	}
	return filepath.Clean(resolved), nil
}

func canonicalGitPath(base, value string) (string, error) {
	if !filepath.IsAbs(value) {
		value = filepath.Join(base, value)
	}
	return canonicalExistingDir(value)
}

func within(base, target string) bool {
	rel, err := filepath.Rel(pathutil.ForCompare(base), pathutil.ForCompare(target))
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}

func samePath(a, b string) bool { return pathutil.Same(a, b) }
