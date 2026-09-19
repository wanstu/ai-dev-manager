package app

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"ai-dev-manager-v2/internal/catalog"
	"ai-dev-manager-v2/internal/environment"
	"ai-dev-manager-v2/internal/hostenv"
	"ai-dev-manager-v2/internal/isolation"
	"ai-dev-manager-v2/internal/logging"
	"ai-dev-manager-v2/internal/memory"
	"ai-dev-manager-v2/internal/model"
	"ai-dev-manager-v2/internal/runtime"
	skillruntime "ai-dev-manager-v2/internal/skill"
	"ai-dev-manager-v2/internal/store"
	"ai-dev-manager-v2/internal/verifier"
	"ai-dev-manager-v2/internal/workspace"
)

type Service struct {
	Store                   *store.Store
	Workspaces              *workspace.Service
	Environments            *environment.Service
	Isolation               *isolation.Service
	HostEnvironment         *hostenv.Manager
	MCPs                    *catalog.MCPService
	Skills                  *catalog.Service
	Memory                  *memory.Service
	Logger                  *logging.Logger
	Verifiers               *verifier.Service
	writerHeartbeatInterval func(time.Duration) time.Duration
}

type EnvironmentSummary struct {
	model.Environment
	PrivateMemoryCount int `json:"private_memory_count"`
}

type EnvironmentInspection struct {
	Environment        EnvironmentSummary     `json:"environment"`
	Workspace          model.Workspace        `json:"workspace"`
	Capabilities       []string               `json:"capabilities"`
	CapabilityReport   model.CapabilityReport `json:"capability_report"`
	EnabledMCPs        []model.MCPDefinition  `json:"enabled_mcps,omitempty"`
	EnabledSkills      []model.CatalogEntry   `json:"enabled_skills,omitempty"`
	UnresolvedMCPIDs   []string               `json:"unresolved_mcp_ids,omitempty"`
	UnresolvedSkillIDs []string               `json:"unresolved_skill_ids,omitempty"`
}

func New(statePath string) *Service {
	s := store.New(statePath)
	ws := workspace.New(s)
	environments := environment.New(s, ws)
	return &Service{
		Store:                   s,
		Workspaces:              ws,
		Environments:            environments,
		Isolation:               isolation.New(s, ws, environments),
		HostEnvironment:         hostenv.New(),
		MCPs:                    catalog.NewMCP(s),
		Skills:                  catalog.New(s, catalog.KindSkill),
		Memory:                  memory.New(s),
		Logger:                  logging.New(filepath.Join(filepath.Dir(statePath), "logs"), 5*1024*1024, 7),
		Verifiers:               verifier.New(s),
		writerHeartbeatInterval: defaultWriterHeartbeatInterval,
	}
}

func defaultWriterHeartbeatInterval(ttl time.Duration) time.Duration {
	interval := ttl / 3
	if interval <= 0 {
		return time.Second
	}
	return interval
}

func (s *Service) heartbeatInterval() time.Duration {
	if s.writerHeartbeatInterval == nil {
		return defaultWriterHeartbeatInterval(s.Environments.WriterLeaseTTL())
	}
	return s.writerHeartbeatInterval(s.Environments.WriterLeaseTTL())
}

func (s *Service) EnvironmentSummaries() ([]EnvironmentSummary, error) {
	environments, err := s.Environments.List()
	if err != nil {
		return nil, err
	}
	items := make([]EnvironmentSummary, 0, len(environments))
	for _, env := range environments {
		items = append(items, environmentSummary(env))
	}
	return items, nil
}

func (s *Service) EnvironmentWorkspaceOptions(id string) (model.EnvironmentWorkspaceOptions, error) {
	return s.Environments.WorkspaceOptions(id)
}

func (s *Service) EnvironmentWorkspaceRecommendations() ([]model.EnvironmentWorkspaceRecommendation, error) {
	return s.Environments.WorkspaceRecommendations()
}

func (s *Service) EnvironmentWorkspaceSet(id, workspaceID string) (EnvironmentSummary, error) {
	env, err := s.Environments.SetWorkspace(id, workspaceID)
	if err != nil {
		return EnvironmentSummary{}, err
	}
	return s.EnvironmentSummary(env.ID)
}

func (s *Service) EnvironmentSummary(environmentID string) (EnvironmentSummary, error) {
	env, err := s.Environments.Get(environmentID)
	if err != nil {
		return EnvironmentSummary{}, err
	}
	return environmentSummary(env), nil
}

func (s *Service) InspectEnvironment(ctx context.Context, environmentID string) (EnvironmentInspection, error) {
	env, err := s.Environments.Get(environmentID)
	if err != nil {
		return EnvironmentInspection{}, err
	}
	ws, err := s.Workspaces.Get(env.WorkspaceID)
	if err != nil {
		return EnvironmentInspection{}, err
	}
	capabilityReport, err := s.environmentCapabilityReport(ctx, env, ws)
	if err != nil {
		return EnvironmentInspection{}, err
	}
	mcps, err := s.MCPs.List()
	if err != nil {
		return EnvironmentInspection{}, err
	}
	skills, err := s.Skills.List()
	if err != nil {
		return EnvironmentInspection{}, err
	}
	enabledMCPs, unresolvedMCPs := resolveMCPSelections(env.EnabledMCPIDs, mcps)
	enabledSkills, unresolvedSkills := resolveCatalogSelections(env.EnabledSkillIDs, skills)
	return EnvironmentInspection{
		Environment:        environmentSummary(env),
		Workspace:          ws,
		Capabilities:       legacyCapabilitiesFromFacts(capabilityReport.Facts),
		CapabilityReport:   capabilityReport,
		EnabledMCPs:        enabledMCPs,
		EnabledSkills:      enabledSkills,
		UnresolvedMCPIDs:   unresolvedMCPs,
		UnresolvedSkillIDs: unresolvedSkills,
	}, nil
}

func environmentSummary(env model.Environment) EnvironmentSummary {
	count := len(env.PrivateMemory)
	env.PrivateMemory = nil
	if env.Writer != nil {
		lease := *env.Writer
		lease.Owner = ""
		env.Writer = &lease
	}
	return EnvironmentSummary{Environment: env, PrivateMemoryCount: count}
}

func resolveMCPSelections(ids []string, entries []model.MCPDefinition) ([]model.MCPDefinition, []string) {
	byID := make(map[string]model.MCPDefinition, len(entries))
	for _, entry := range entries {
		byID[entry.ID] = entry
	}
	resolved := make([]model.MCPDefinition, 0, len(ids))
	unresolved := make([]string, 0)
	for _, id := range ids {
		if entry, ok := byID[id]; ok {
			resolved = append(resolved, entry)
			continue
		}
		unresolved = append(unresolved, id)
	}
	return resolved, unresolved
}

func resolveCatalogSelections(ids []string, entries []model.CatalogEntry) ([]model.CatalogEntry, []string) {
	byID := make(map[string]model.CatalogEntry, len(entries))
	for _, entry := range entries {
		byID[entry.ID] = entry
	}
	resolved := make([]model.CatalogEntry, 0, len(ids))
	unresolved := make([]string, 0)
	for _, id := range ids {
		if entry, ok := byID[id]; ok {
			resolved = append(resolved, entry)
			continue
		}
		unresolved = append(unresolved, id)
	}
	return resolved, unresolved
}

func (s *Service) SetWorkspaceMCP(workspaceID, mcpID string, enabled bool) (model.Workspace, error) {
	if enabled {
		exists, err := s.MCPs.Exists(mcpID)
		if err != nil {
			return model.Workspace{}, err
		}
		if !exists {
			return model.Workspace{}, fmt.Errorf("mcp %q not found", mcpID)
		}
	}
	return s.Workspaces.SetMCP(workspaceID, mcpID, enabled)
}

func (s *Service) SetWorkspaceSkill(workspaceID, skillID string, enabled bool) (model.Workspace, error) {
	if enabled {
		exists, err := s.Skills.Exists(skillID)
		if err != nil {
			return model.Workspace{}, err
		}
		if !exists {
			return model.Workspace{}, fmt.Errorf("skill %q not found", skillID)
		}
	}
	return s.Workspaces.SetSkill(workspaceID, skillID, enabled)
}

func (s *Service) SetEnvironmentMCP(environmentID, mcpID string, enabled bool) (model.Environment, error) {
	if enabled {
		exists, err := s.MCPs.Exists(mcpID)
		if err != nil {
			return model.Environment{}, err
		}
		if !exists {
			return model.Environment{}, fmt.Errorf("mcp %q not found", mcpID)
		}
	}
	return s.Environments.SetMCP(environmentID, mcpID, enabled)
}

func (s *Service) SetEnvironmentSkill(environmentID, skillID string, enabled bool) (model.Environment, error) {
	if enabled {
		exists, err := s.Skills.Exists(skillID)
		if err != nil {
			return model.Environment{}, err
		}
		if !exists {
			return model.Environment{}, fmt.Errorf("skill %q not found", skillID)
		}
	}
	return s.Environments.SetSkill(environmentID, skillID, enabled)
}

func (s *Service) EnvironmentSkillEntries(environmentID string) ([]model.CatalogEntry, []string, error) {
	env, err := s.Environments.Get(environmentID)
	if err != nil {
		return nil, nil, err
	}
	entries, err := s.Skills.List()
	if err != nil {
		return nil, nil, err
	}
	resolved, unresolved := resolveCatalogSelections(env.EnabledSkillIDs, entries)
	configured := make([]model.CatalogEntry, 0, len(resolved))
	for _, entry := range resolved {
		if skillruntime.Configured(entry) {
			configured = append(configured, entry)
			continue
		}
		unresolved = append(unresolved, entry.ID)
	}
	sort.Strings(unresolved)
	return configured, unresolved, nil
}

func (s *Service) CreateManagedWorktree(ctx context.Context, workspaceID, name string, options isolation.CreateOptions) (isolation.CreateResult, error) {
	if s.Isolation == nil {
		return isolation.CreateResult{}, fmt.Errorf("managed worktree isolation is unavailable")
	}
	return s.Isolation.Create(ctx, workspaceID, name, options)
}

func (s *Service) CreateManagedWorktreeFromEnvironment(ctx context.Context, sourceEnvironmentID, name string, options isolation.CreateOptions) (isolation.CreateResult, error) {
	if s.Isolation == nil {
		return isolation.CreateResult{}, fmt.Errorf("managed worktree isolation is unavailable")
	}
	return s.Isolation.CreateFromEnvironment(ctx, sourceEnvironmentID, name, options)
}

func (s *Service) ManagedWorktrees() ([]model.ManagedWorktree, error) {
	if s.Isolation == nil {
		return nil, fmt.Errorf("managed worktree isolation is unavailable")
	}
	return s.Isolation.List()
}

func (s *Service) HostEnvironmentStatus() (hostenv.Status, error) {
	if s.HostEnvironment == nil {
		return hostenv.Status{Source: "unavailable", LastRefreshStatus: "error", LastRefreshError: "host environment manager is unavailable"}, fmt.Errorf("host environment manager is unavailable")
	}
	return s.HostEnvironment.Status(), nil
}

func (s *Service) RefreshHostEnvironment() (hostenv.Status, error) {
	if s.HostEnvironment == nil {
		status, statusErr := s.HostEnvironmentStatus()
		return status, statusErr
	}
	status := s.HostEnvironment.Refresh()
	if status.LastRefreshStatus == "error" {
		return status, fmt.Errorf("refresh host environment: %s", status.LastRefreshError)
	}
	return status, nil
}

func (s *Service) WorktreeSettings() (model.WorktreeSettings, error) {
	if s.Isolation == nil {
		return model.WorktreeSettings{}, fmt.Errorf("managed worktree isolation is unavailable")
	}
	return s.Isolation.Settings()
}

func (s *Service) UpdateWorktreeSettings(root, branchPrefix string) (model.WorktreeSettings, error) {
	if s.Isolation == nil {
		return model.WorktreeSettings{}, fmt.Errorf("managed worktree isolation is unavailable")
	}
	return s.Isolation.UpdateSettings(root, branchPrefix)
}

func (s *Service) DestroyManagedWorktree(ctx context.Context, environmentID, writerOwner string, force bool) (isolation.DestroyResult, error) {
	if s.Isolation == nil {
		return isolation.DestroyResult{}, fmt.Errorf("managed worktree isolation is unavailable")
	}
	return s.Isolation.Destroy(ctx, environmentID, writerOwner, force)
}

func (s *Service) Runtime(environmentID string) (*runtime.Runtime, model.Environment, error) {
	env, err := s.Environments.Get(environmentID)
	if err != nil {
		return nil, model.Environment{}, err
	}
	if s.Isolation != nil {
		if err := s.Isolation.ValidateEnvironment(context.Background(), env); err != nil {
			return nil, model.Environment{}, err
		}
	}
	state, err := s.Store.Load()
	if err != nil {
		return nil, model.Environment{}, err
	}
	var rt *runtime.Runtime
	if s.HostEnvironment != nil {
		rt, err = runtime.NewWithPolicy(env.Root, state.AllowedExecutables, s.HostEnvironment.Environ(), state.ExecFullAuthorization)
	} else {
		rt, err = runtime.NewWithAuthorization(env.Root, state.AllowedExecutables, state.ExecFullAuthorization)
	}
	if err != nil {
		return nil, model.Environment{}, err
	}
	return rt, env, nil
}

func (s *Service) Capabilities(ctx context.Context, environmentID string) ([]string, error) {
	rt, _, err := s.Runtime(environmentID)
	if err != nil {
		return nil, err
	}
	return rt.Capabilities(ctx), nil
}

func (s *Service) AllowExecutable(executable string) error {
	executable = strings.TrimSpace(executable)
	if executable == "" {
		return fmt.Errorf("executable is required")
	}
	if filepath.IsAbs(executable) {
		executable = filepath.Clean(executable)
	}
	return s.Store.Update(func(state *model.State) error {
		state.ExecDenials = removeExecDenial(state.ExecDenials, executable)
		for _, current := range state.AllowedExecutables {
			if strings.EqualFold(current, executable) {
				return nil
			}
		}
		state.AllowedExecutables = append(state.AllowedExecutables, executable)
		sort.Slice(state.AllowedExecutables, func(i, j int) bool {
			return strings.ToLower(state.AllowedExecutables[i]) < strings.ToLower(state.AllowedExecutables[j])
		})
		return nil
	})
}

func (s *Service) RemoveAllowedExecutable(executable string) error {
	executable = strings.TrimSpace(executable)
	if executable == "" {
		return fmt.Errorf("executable is required")
	}
	if filepath.IsAbs(executable) {
		executable = filepath.Clean(executable)
	}
	return s.Store.Update(func(state *model.State) error {
		for i, current := range state.AllowedExecutables {
			if strings.EqualFold(current, executable) {
				state.AllowedExecutables = append(state.AllowedExecutables[:i], state.AllowedExecutables[i+1:]...)
				return nil
			}
		}
		return fmt.Errorf("executable %q is not allowlisted", executable)
	})
}

func (s *Service) AllowedExecutables() ([]string, error) {
	state, err := s.Store.Load()
	if err != nil {
		return nil, err
	}
	return append([]string(nil), state.AllowedExecutables...), nil
}

func (s *Service) Tree(environmentID, path string, maxDepth, maxEntries int) (any, error) {
	rt, _, err := s.Runtime(environmentID)
	if err != nil {
		return nil, err
	}
	return rt.Tree(path, maxDepth, maxEntries)
}

func (s *Service) Read(environmentID, path string, maxBytes int) (any, error) {
	rt, _, err := s.Runtime(environmentID)
	if err != nil {
		return nil, err
	}
	return rt.Read(path, maxBytes)
}

func (s *Service) Search(environmentID, path, query string, maxFiles, maxMatches, maxBytesPerFile int) (any, error) {
	rt, _, err := s.Runtime(environmentID)
	if err != nil {
		return nil, err
	}
	return rt.Search(path, query, maxFiles, maxMatches, maxBytesPerFile)
}

func (s *Service) Write(environmentID, owner, path, content string, createParents bool) (any, error) {
	if _, err := s.Environments.RequireWriter(environmentID, owner); err != nil {
		return nil, err
	}
	rt, _, err := s.Runtime(environmentID)
	if err != nil {
		return nil, err
	}
	result, err := rt.Write(path, content, createParents)
	if err != nil {
		return nil, err
	}
	if err := s.Environments.Touch(environmentID, owner); err != nil {
		return nil, err
	}
	return result, nil
}

func (s *Service) Edit(environmentID, owner, path, oldText, newText string, expected int) (any, error) {
	if _, err := s.Environments.RequireWriter(environmentID, owner); err != nil {
		return nil, err
	}
	rt, _, err := s.Runtime(environmentID)
	if err != nil {
		return nil, err
	}
	result, err := rt.Edit(path, oldText, newText, expected)
	if err != nil {
		return nil, err
	}
	if err := s.Environments.Touch(environmentID, owner); err != nil {
		return nil, err
	}
	return result, nil
}

func (s *Service) Delete(environmentID, owner, path string) (any, error) {
	if _, err := s.Environments.RequireWriter(environmentID, owner); err != nil {
		return nil, err
	}
	rt, _, err := s.Runtime(environmentID)
	if err != nil {
		return nil, err
	}
	result, err := rt.Delete(path)
	if err != nil {
		return nil, err
	}
	if err := s.Environments.Touch(environmentID, owner); err != nil {
		return nil, err
	}
	return result, nil
}

func (s *Service) Exec(ctx context.Context, environmentID, owner, executable string, args []string, cwd string, timeoutMS int64, maxOutputBytes int) (any, error) {
	if _, err := s.Environments.RequireWriter(environmentID, owner); err != nil {
		return nil, err
	}
	rt, _, err := s.Runtime(environmentID)
	if err != nil {
		return nil, err
	}

	commandCtx, cancel := context.WithCancel(ctx)
	heartbeatDone := make(chan error, 1)
	go func() {
		interval := s.heartbeatInterval()
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-commandCtx.Done():
				heartbeatDone <- nil
				return
			case <-ticker.C:
				if _, heartbeatErr := s.Environments.HeartbeatWriter(environmentID, owner); heartbeatErr != nil {
					heartbeatDone <- heartbeatErr
					cancel()
					return
				}
			}
		}
	}()

	s.recordFullAuthorizationBypass(rt, environmentID, executable, "exec")
	s.Log("info", "exec.start", map[string]string{"environment_id": environmentID, "executable": executable, "surface": "exec"})
	result, execErr := rt.Exec(commandCtx, executable, args, cwd, timeoutMS, maxOutputBytes)
	if execErr != nil {
		s.Log("error", "exec.failed", map[string]string{"environment_id": environmentID, "executable": executable, "surface": "exec", "error": execErr.Error()})
	} else {
		s.Log("info", "exec.completed", map[string]string{"environment_id": environmentID, "executable": executable, "surface": "exec", "exit_code": fmt.Sprint(result.ExitCode)})
	}
	if isExecutableNotAllowedError(execErr) {
		s.recordExecDenial(environmentID, executable, "exec", execErr.Error())
	}
	cancel()
	heartbeatErr := <-heartbeatDone
	if heartbeatErr != nil {
		return nil, fmt.Errorf("writer heartbeat failed: %w", heartbeatErr)
	}
	if execErr != nil {
		return nil, execErr
	}
	if err := s.Environments.Touch(environmentID, owner); err != nil {
		return nil, err
	}
	return result, nil
}

func (s *Service) ListVerifiers(environmentID string) ([]model.VerifierDefinition, error) {
	return s.Verifiers.List(environmentID)
}

func (s *Service) AddVerifier(environmentID string, definition model.VerifierDefinition) (model.VerifierDefinition, error) {
	return s.Verifiers.Add(environmentID, definition)
}

func (s *Service) RemoveVerifier(environmentID, verifierID string) (model.VerifierDefinition, error) {
	return s.Verifiers.Remove(environmentID, verifierID)
}

func (s *Service) RunVerifier(ctx context.Context, environmentID, owner, verifierID string, maxOutputBytes int) (verifier.Result, error) {
	prepared, err := s.PrepareVerifierExecution(ctx, environmentID, owner, verifierID)
	if err != nil {
		return verifier.Result{}, err
	}
	definition := prepared.Definition
	rt := prepared.Runtime

	commandCtx, cancel := context.WithCancel(ctx)
	heartbeatDone := make(chan error, 1)
	go func() {
		interval := s.heartbeatInterval()
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-commandCtx.Done():
				heartbeatDone <- nil
				return
			case <-ticker.C:
				if _, heartbeatErr := s.Environments.HeartbeatWriter(environmentID, owner); heartbeatErr != nil {
					heartbeatDone <- heartbeatErr
					cancel()
					return
				}
			}
		}
	}()

	timeout := prepared.Timeout
	verifierCtx, timeoutCancel := context.WithTimeout(commandCtx, timeout)
	started := time.Now()
	// The app-owned verifier deadline is authoritative. Runtime.Exec keeps its
	// own slightly-later timeout only as a fallback so timeout identity comes
	// from verifierCtx rather than Runtime error text.
	runtimeTimeoutMS := timeout.Milliseconds() + 1000
	commandResult, execErr := rt.Exec(verifierCtx, definition.Executable, definition.Args, definition.Cwd, runtimeTimeoutMS, maxOutputBytes)
	if isExecutableNotAllowedError(execErr) {
		s.recordExecDenial(environmentID, definition.Executable, "verifier", execErr.Error())
	}
	duration := time.Since(started)
	callerInterrupted := ctx.Err() != nil && (execErr != nil || commandResult.ExitCode != 0)
	timedOut := !callerInterrupted && verifierCtx.Err() == context.DeadlineExceeded
	timeoutCancel()
	cancel()
	heartbeatErr := <-heartbeatDone
	if heartbeatErr != nil {
		return verifier.Result{}, fmt.Errorf("writer heartbeat failed: %w", heartbeatErr)
	}
	if callerInterrupted {
		return verifier.Result{}, BlockingVerifierRequestInterruptedError{}
	}

	result, classifyErr := verifier.Classify(definition, commandResult, duration, timedOut, execErr)
	if classifyErr != nil {
		return verifier.Result{}, classifyErr
	}
	if err := s.Environments.Touch(environmentID, owner); err != nil {
		return verifier.Result{}, err
	}
	return result, nil
}

func (s *Service) GitStatus(ctx context.Context, environmentID string) (any, error) {
	rt, _, err := s.Runtime(environmentID)
	if err != nil {
		return nil, err
	}
	return rt.GitStatus(ctx)
}

func (s *Service) GitDiff(ctx context.Context, environmentID string) (any, error) {
	rt, _, err := s.Runtime(environmentID)
	if err != nil {
		return nil, err
	}
	return rt.GitDiff(ctx)
}

func (s *Service) GitBranch(ctx context.Context, environmentID string) (any, error) {
	rt, _, err := s.Runtime(environmentID)
	if err != nil {
		return nil, err
	}
	return rt.GitBranch(ctx)
}
