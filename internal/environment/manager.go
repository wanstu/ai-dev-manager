package environment

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode"

	"ai-dev-manager/internal/adapter/runtimeadapter"
	"ai-dev-manager/internal/model"
)

type WorkspaceGetter func(string) (model.Workspace, error)
type RuntimeBuilder func(string) (runtimeadapter.Runtime, error)
type DerivedRuntimeBuilder func(string, string, string) (runtimeadapter.Runtime, error)
type IDGenerator func() (string, error)

type Manager struct {
	mu           sync.RWMutex
	store        *Store
	getWorkspace WorkspaceGetter
	buildRuntime RuntimeBuilder
	buildDerived DerivedRuntimeBuilder
	now          func() time.Time
	newID        IDGenerator
}

func NewManager(store *Store, getWorkspace WorkspaceGetter, buildRuntime RuntimeBuilder, buildDerived DerivedRuntimeBuilder) (*Manager, error) {
	if store == nil || getWorkspace == nil || buildRuntime == nil || buildDerived == nil {
		return nil, errors.New("environment manager dependencies are required")
	}
	return &Manager{
		store:        store,
		getWorkspace: getWorkspace,
		buildRuntime: buildRuntime,
		buildDerived: buildDerived,
		now:          func() time.Time { return time.Now().UTC() },
		newID:        newEnvironmentID,
	}, nil
}

func (m *Manager) List() ([]Environment, error) {
	return m.store.List()
}

func (m *Manager) Create(ctx context.Context, req CreateRequest) (InspectResult, error) {
	req.WorkspaceID = strings.TrimSpace(req.WorkspaceID)
	req.Name = strings.TrimSpace(req.Name)
	req.Base = strings.TrimSpace(req.Base)
	req.Branch = strings.TrimSpace(req.Branch)
	if req.WorkspaceID == "" || req.Name == "" {
		return InspectResult{}, &Error{Code: ErrInvalidInput, Message: "workspace_id and name are required"}
	}
	if _, err := m.getWorkspace(req.WorkspaceID); err != nil {
		return InspectResult{}, &Error{Code: ErrInvalidInput, Message: "base workspace is not registered", Err: err}
	}
	baseRuntime, err := m.buildRuntime(req.WorkspaceID)
	if err != nil {
		return InspectResult{}, &Error{Code: ErrRuntime, Message: "build base runtime", Err: err}
	}
	if !hasCapability(baseRuntime.Capabilities(), "git.worktree") {
		return InspectResult{}, &Error{Code: ErrCapabilityMissing, Message: "base runtime does not support managed git worktrees"}
	}

	baseRef := req.Base
	if baseRef == "" {
		branch, err := invokeString(ctx, baseRuntime, runtimeadapter.OpGitBranch, nil)
		if err != nil {
			return InspectResult{}, &Error{Code: ErrRuntime, Message: "read current git branch", Err: err}
		}
		if strings.TrimSpace(branch) == "" {
			baseRef = "HEAD"
		} else {
			baseRef = strings.TrimSpace(branch)
		}
	}
	baseCommit, err := invokeString(ctx, baseRuntime, runtimeadapter.OpGitResolveRef, map[string]any{"ref": baseRef})
	if err != nil {
		return InspectResult{}, &Error{Code: ErrInvalidInput, Message: fmt.Sprintf("cannot resolve base %q", baseRef), Err: err}
	}

	branch := req.Branch
	if branch == "" {
		branch = "adm/" + sanitizeBranchName(req.Name)
	}
	exists, err := invokeBool(ctx, baseRuntime, runtimeadapter.OpGitBranchExists, map[string]any{"branch": branch})
	if err != nil {
		return InspectResult{}, &Error{Code: ErrInvalidInput, Message: fmt.Sprintf("invalid environment branch %q", branch), Err: err}
	}
	if exists {
		return InspectResult{}, &Error{Code: ErrBranchExists, Message: fmt.Sprintf("branch %q already exists", branch)}
	}

	statusCount, err := invokeSliceCount(ctx, baseRuntime, runtimeadapter.OpGitStatus, nil)
	if err != nil {
		return InspectResult{}, &Error{Code: ErrRuntime, Message: "read base git status", Err: err}
	}
	var exportedChanges map[string]any
	if req.IncludeChanges && statusCount > 0 {
		value, err := baseRuntime.Invoke(ctx, runtimeadapter.OpGitChangesExport, nil)
		if err != nil {
			return InspectResult{}, &Error{Code: ErrRuntime, Message: "export base checkout changes", Err: err}
		}
		if err := decodeValue(value, &exportedChanges); err != nil {
			return InspectResult{}, &Error{Code: ErrRuntime, Message: "decode exported checkout changes", Err: err}
		}
	}

	id, err := m.newID()
	if err != nil {
		return InspectResult{}, &Error{Code: ErrStore, Message: "generate environment id", Err: err}
	}
	now := m.now()
	env := Environment{
		ID:             id,
		WorkspaceID:    req.WorkspaceID,
		Name:           req.Name,
		Branch:         branch,
		BaseRef:        baseRef,
		BaseCommit:     baseCommit,
		WorktreeName:   id,
		State:          StateCreating,
		CreatedAt:      now,
		UpdatedAt:      now,
		LastActivityAt: now,
		Metadata:       map[string]string{},
	}
	if statusCount > 0 {
		env.Metadata["base_dirty_at_create"] = "true"
		env.Metadata["base_dirty_change_count"] = fmt.Sprintf("%d", statusCount)
	}
	if req.IncludeChanges {
		env.Metadata["include_changes_requested"] = "true"
	}
	if err := m.store.Put(env); err != nil {
		return InspectResult{}, err
	}

	createdAny, err := baseRuntime.Invoke(ctx, runtimeadapter.OpGitWorktreeCreate, map[string]any{
		"name":        env.WorktreeName,
		"branch":      env.Branch,
		"start_point": env.BaseCommit,
	})
	if err != nil {
		return m.failCreate(env, "worktree_create_failed", "create managed worktree", err)
	}
	var created worktreeInfo
	if err := decodeValue(createdAny, &created); err != nil {
		return m.failCreate(env, "worktree_create_decode_failed", "decode managed worktree result", err)
	}

	actual, err := m.getManagedWorktree(ctx, baseRuntime, env.WorktreeName)
	if err != nil {
		return m.failCreate(env, "worktree_validation_failed", "validate managed worktree", err)
	}
	if !samePath(created.Path, actual.Path) {
		return m.failCreate(env, "worktree_mismatch", "created worktree did not match managed worktree inventory", nil)
	}
	env.WorktreePath = actual.Path
	env.UpdatedAt = m.now()
	if err := m.store.Put(env); err != nil {
		return InspectResult{}, err
	}
	derived, err := m.buildDerived(env.WorkspaceID, env.ID, actual.Path)
	if err != nil {
		return m.failCreate(env, "derived_runtime_failed", "build derived environment runtime", err)
	}
	derivedStatus := derived.Status(ctx)
	if derivedStatus.State == runtimeadapter.StateError {
		return m.failCreate(env, "derived_runtime_error", "derived environment runtime is not ready", nil)
	}
	if req.IncludeChanges && exportedChanges != nil {
		if _, err := derived.Invoke(ctx, runtimeadapter.OpGitChangesApply, exportedChanges); err != nil {
			return m.failCreate(env, "change_transfer_failed", "apply base checkout changes", err)
		}
	}
	if req.IncludeChanges {
		env.Metadata["changes_included"] = "true"
	}

	env.State = StateReady
	env.Error = nil
	env.UpdatedAt = m.now()
	env.LastActivityAt = env.UpdatedAt
	if err := m.store.Put(env); err != nil {
		return InspectResult{}, err
	}
	return m.inspectReady(ctx, env, baseRuntime, derived, actual), nil
}

func (m *Manager) Inspect(ctx context.Context, id string) (InspectResult, error) {
	id = strings.TrimSpace(id)
	env, err := m.store.Get(id)
	if err != nil {
		return InspectResult{}, err
	}
	baseRuntime, derived, worktree, err := m.validatedRuntimes(ctx, env)
	if err != nil {
		now := m.now()
		env.State = StateError
		env.UpdatedAt = now
		code := "worktree_validation_failed"
		errCode := ErrWorktreeMissing
		if errors.As(err, new(*Error)) {
			var envErr *Error
			if errors.As(err, &envErr) {
				errCode = envErr.Code
				code = string(envErr.Code)
			}
		}
		env.Error = &EnvironmentError{Code: code, Message: err.Error()}
		_ = m.store.Put(env)
		return InspectResult{
			Environment: env,
			Facts:       []Fact{{Code: "worktree_available", Message: "Managed worktree is not currently available", Value: false}},
		}, &Error{Code: errCode, EnvironmentID: env.ID, Message: err.Error(), Err: err}
	}
	_ = baseRuntime
	now := m.now()
	if env.State != StateReady || env.Error != nil || !samePath(env.WorktreePath, worktree.Path) {
		env.State = StateReady
		env.Error = nil
		env.WorktreePath = worktree.Path
		env.UpdatedAt = now
	}
	if err := m.store.Put(env); err != nil {
		return InspectResult{}, err
	}
	return m.inspectReady(ctx, env, baseRuntime, derived, worktree), nil
}

func (m *Manager) InvokeRead(ctx context.Context, id, operation string, input map[string]any) (any, error) {
	id = strings.TrimSpace(id)
	operation = strings.TrimSpace(operation)
	if id == "" {
		return nil, &Error{Code: ErrInvalidInput, Message: "environment_id is required"}
	}
	requiredCapability, ok := readOperationCapability(operation)
	if !ok {
		return nil, &Error{Code: ErrInvalidInput, EnvironmentID: id, Message: fmt.Sprintf("operation %q is not an allowed Environment read operation", operation)}
	}
	env, err := m.store.Get(id)
	if err != nil {
		return nil, err
	}
	_, derived, _, err := m.validatedRuntimes(ctx, env)
	if err != nil {
		return nil, err
	}
	if !hasCapability(derived.Capabilities(), requiredCapability) {
		return nil, &Error{Code: ErrCapabilityMissing, EnvironmentID: env.ID, Message: fmt.Sprintf("environment runtime does not support %s", requiredCapability)}
	}
	value, err := derived.Invoke(ctx, operation, input)
	if err != nil {
		return nil, &Error{Code: ErrRuntime, EnvironmentID: env.ID, Message: fmt.Sprintf("invoke environment read operation %q", operation), Err: err}
	}
	return value, nil
}

func readOperationCapability(operation string) (string, bool) {
	switch operation {
	case runtimeadapter.OpTree:
		return "files.tree", true
	case runtimeadapter.OpRead:
		return "files.read", true
	case runtimeadapter.OpSearch:
		return "search.text", true
	case runtimeadapter.OpGitStatus:
		return "git.status", true
	case runtimeadapter.OpGitDiff:
		return "git.diff", true
	case runtimeadapter.OpGitBranch:
		return "git.branch", true
	default:
		return "", false
	}
}

func (m *Manager) InvokeMutation(ctx context.Context, id, owner, operation string, input map[string]any) (any, error) {
	id = strings.TrimSpace(id)
	owner = strings.TrimSpace(owner)
	operation = strings.TrimSpace(operation)
	if id == "" || !validWriterOwner(owner) {
		return nil, &Error{Code: ErrInvalidInput, EnvironmentID: id, Message: "environment_id and valid writer_owner are required"}
	}
	requiredCapability, ok := mutationOperationCapability(operation)
	if !ok {
		return nil, &Error{Code: ErrInvalidInput, EnvironmentID: id, Message: fmt.Sprintf("operation %q is not an allowed Environment mutation operation", operation)}
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	env, err := m.store.Get(id)
	if err != nil {
		return nil, err
	}
	if env.Writer == nil {
		return nil, &Error{Code: ErrWriterNotOwner, EnvironmentID: env.ID, Message: "environment has no active writer"}
	}
	if env.Writer.Owner != owner {
		return nil, &Error{Code: ErrWriterNotOwner, EnvironmentID: env.ID, Message: fmt.Sprintf("environment writer is held by %q", env.Writer.Owner)}
	}
	_, derived, _, err := m.validatedRuntimes(ctx, env)
	if err != nil {
		return nil, err
	}
	if !hasCapability(derived.Capabilities(), requiredCapability) {
		return nil, &Error{Code: ErrCapabilityMissing, EnvironmentID: env.ID, Message: fmt.Sprintf("environment runtime does not support %s", requiredCapability)}
	}
	value, err := derived.Invoke(ctx, operation, input)
	if err != nil {
		return nil, &Error{Code: ErrRuntime, EnvironmentID: env.ID, Message: fmt.Sprintf("invoke environment mutation operation %q", operation), Err: err}
	}

	now := m.now()
	env.Writer.LastSeenAt = now
	env.UpdatedAt = now
	env.LastActivityAt = now
	if err := m.store.Put(env); err != nil {
		return nil, &Error{Code: ErrStore, EnvironmentID: env.ID, Message: "persist environment mutation activity", Err: err}
	}
	return value, nil
}

func mutationOperationCapability(operation string) (string, bool) {
	switch operation {
	case runtimeadapter.OpWrite:
		return "files.write", true
	case runtimeadapter.OpEdit:
		return "files.edit", true
	case runtimeadapter.OpExec:
		return "shell.exec", true
	case runtimeadapter.OpVerifierRun, runtimeadapter.OpVerifierRunMany:
		return "verify.run", true
	default:
		return "", false
	}
}

func (m *Manager) Destroy(ctx context.Context, id string, force bool) (Environment, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	env, err := m.store.Get(strings.TrimSpace(id))
	if err != nil {
		return Environment{}, err
	}
	baseRuntime, derived, _, err := m.validatedRuntimes(ctx, env)
	if err != nil {
		var envErr *Error
		if force && errors.As(err, &envErr) && envErr.Code == ErrWorktreeMissing {
			if removeErr := m.store.Remove(env.ID); removeErr != nil {
				return env, removeErr
			}
			return env, nil
		}
		return env, err
	}
	if !force {
		if env.Writer != nil {
			return env, &Error{Code: ErrUnsafeDestroy, EnvironmentID: env.ID, Message: fmt.Sprintf("environment has an active writer %q", env.Writer.Owner)}
		}
		count, err := invokeSliceCount(ctx, derived, runtimeadapter.OpGitStatus, nil)
		if err != nil {
			return env, &Error{Code: ErrRuntime, EnvironmentID: env.ID, Message: "read environment git status", Err: err}
		}
		if count != 0 {
			return env, &Error{Code: ErrUnsafeDestroy, EnvironmentID: env.ID, Message: "environment has uncommitted changes"}
		}
		value, err := derived.Invoke(ctx, runtimeadapter.OpGitPushStatus, nil)
		if err != nil {
			return env, &Error{Code: ErrRuntime, EnvironmentID: env.ID, Message: "read environment push status", Err: err}
		}
		var push gitPushStatus
		if err := decodeValue(value, &push); err != nil {
			return env, &Error{Code: ErrRuntime, EnvironmentID: env.ID, Message: "decode environment push status", Err: err}
		}
		if push.Head != env.BaseCommit && (!push.HasUpstream || push.Ahead > 0) {
			return env, &Error{Code: ErrUnsafeDestroy, EnvironmentID: env.ID, Message: fmt.Sprintf("environment has unpushed commits (ahead=%d, upstream=%t)", push.Ahead, push.HasUpstream)}
		}
	}
	if _, err := baseRuntime.Invoke(ctx, runtimeadapter.OpGitWorktreeRemove, map[string]any{"name": env.WorktreeName, "force": force}); err != nil {
		return env, &Error{Code: ErrRuntime, EnvironmentID: env.ID, Message: "remove managed worktree", Err: err}
	}
	if err := m.store.Remove(env.ID); err != nil {
		return env, err
	}
	return env, nil
}

func (m *Manager) validatedRuntimes(ctx context.Context, env Environment) (runtimeadapter.Runtime, runtimeadapter.Runtime, worktreeInfo, error) {
	if _, err := m.getWorkspace(env.WorkspaceID); err != nil {
		return nil, nil, worktreeInfo{}, &Error{Code: ErrRuntime, EnvironmentID: env.ID, Message: "base workspace is unavailable", Err: err}
	}
	baseRuntime, err := m.buildRuntime(env.WorkspaceID)
	if err != nil {
		return nil, nil, worktreeInfo{}, &Error{Code: ErrRuntime, EnvironmentID: env.ID, Message: "build base runtime", Err: err}
	}
	actual, err := m.getManagedWorktree(ctx, baseRuntime, env.WorktreeName)
	if err != nil {
		return nil, nil, worktreeInfo{}, &Error{Code: ErrWorktreeMissing, EnvironmentID: env.ID, Message: "managed worktree is missing", Err: err}
	}
	if env.WorktreePath != "" && !samePath(env.WorktreePath, actual.Path) {
		return nil, nil, worktreeInfo{}, &Error{Code: ErrWorktreeMismatch, EnvironmentID: env.ID, Message: "persisted worktree path does not match managed worktree inventory"}
	}
	derived, err := m.buildDerived(env.WorkspaceID, env.ID, actual.Path)
	if err != nil {
		return nil, nil, worktreeInfo{}, &Error{Code: ErrRuntime, EnvironmentID: env.ID, Message: "build derived environment runtime", Err: err}
	}
	return baseRuntime, derived, actual, nil
}

func (m *Manager) getManagedWorktree(ctx context.Context, baseRuntime runtimeadapter.Runtime, name string) (worktreeInfo, error) {
	value, err := baseRuntime.Invoke(ctx, runtimeadapter.OpGitWorktreeGet, map[string]any{"name": name})
	if err != nil {
		return worktreeInfo{}, err
	}
	var result worktreeInfo
	if err := decodeValue(value, &result); err != nil {
		return worktreeInfo{}, err
	}
	if strings.TrimSpace(result.Path) == "" {
		return worktreeInfo{}, errors.New("managed worktree path is empty")
	}
	return result, nil
}

func (m *Manager) inspectReady(ctx context.Context, env Environment, baseRuntime, derived runtimeadapter.Runtime, worktree worktreeInfo) InspectResult {
	return m.buildInspectResult(ctx, env, baseRuntime, derived, worktree)
}

func (m *Manager) failCreate(env Environment, code, message string, cause error) (InspectResult, error) {
	now := m.now()
	env.State = StateError
	env.UpdatedAt = now
	env.LastActivityAt = now
	env.Error = &EnvironmentError{Code: code, Message: message}
	if cause != nil {
		env.Error.Message = message + ": " + cause.Error()
	}
	_ = m.store.Put(env)
	return InspectResult{Environment: env}, &Error{Code: ErrRuntime, EnvironmentID: env.ID, Message: env.Error.Message, Err: cause}
}

type worktreeInfo struct {
	Path   string `json:"Path"`
	Head   string `json:"Head"`
	Branch string `json:"Branch"`
	Bare   bool   `json:"Bare"`
}

type gitPushStatus struct {
	Head        string `json:"head"`
	Branch      string `json:"branch,omitempty"`
	HasUpstream bool   `json:"has_upstream"`
	Upstream    string `json:"upstream,omitempty"`
	Ahead       int    `json:"ahead"`
}

func invokeString(ctx context.Context, runtime runtimeadapter.Runtime, operation string, input map[string]any) (string, error) {
	value, err := runtime.Invoke(ctx, operation, input)
	if err != nil {
		return "", err
	}
	if text, ok := value.(string); ok {
		return strings.TrimSpace(text), nil
	}
	var text string
	if err := decodeValue(value, &text); err != nil {
		return "", err
	}
	return strings.TrimSpace(text), nil
}

func invokeBool(ctx context.Context, runtime runtimeadapter.Runtime, operation string, input map[string]any) (bool, error) {
	value, err := runtime.Invoke(ctx, operation, input)
	if err != nil {
		return false, err
	}
	if result, ok := value.(bool); ok {
		return result, nil
	}
	var result bool
	if err := decodeValue(value, &result); err != nil {
		return false, err
	}
	return result, nil
}

func invokeSliceCount(ctx context.Context, runtime runtimeadapter.Runtime, operation string, input map[string]any) (int, error) {
	value, err := runtime.Invoke(ctx, operation, input)
	if err != nil {
		return 0, err
	}
	data, err := json.Marshal(value)
	if err != nil {
		return 0, err
	}
	var values []json.RawMessage
	if err := json.Unmarshal(data, &values); err != nil {
		return 0, err
	}
	return len(values), nil
}

func decodeValue(value any, target any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, target)
}

func hasCapability(capabilities []string, capability string) bool {
	for _, current := range capabilities {
		if current == capability {
			return true
		}
	}
	return false
}

func sanitizeBranchName(name string) string {
	name = strings.TrimSpace(strings.ToLower(name))
	var builder strings.Builder
	lastDash := false
	for _, r := range name {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			builder.WriteRune(r)
			lastDash = false
		case r == '.' || r == '_' || r == '-':
			builder.WriteRune(r)
			lastDash = r == '-'
		default:
			if !lastDash && builder.Len() > 0 {
				builder.WriteByte('-')
				lastDash = true
			}
		}
	}
	result := strings.Trim(builder.String(), ".-_")
	if result == "" {
		return "task"
	}
	return result
}

func samePath(left, right string) bool {
	return strings.EqualFold(filepath.Clean(left), filepath.Clean(right))
}

func newEnvironmentID() (string, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return "env_" + hex.EncodeToString(bytes), nil
}
