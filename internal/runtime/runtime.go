package runtime

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"ai-dev-manager/internal/model"
)

type Mode string

const (
	ModeReadOnly       Mode = "read-only"
	ModeWorkspaceWrite Mode = "workspace-write"
	ModeStandard       Mode = "standard"
	ModeFull           Mode = "full"
)

type Capability string

const (
	CapabilityTree        Capability = "files.tree"
	CapabilityRead        Capability = "files.read"
	CapabilityWrite       Capability = "files.write"
	CapabilityEdit        Capability = "files.edit"
	CapabilitySearch      Capability = "search.text"
	CapabilityExec        Capability = "shell.exec"
	CapabilityGitStatus   Capability = "git.status"
	CapabilityGitDiff     Capability = "git.diff"
	CapabilityGitBranch   Capability = "git.branch"
	CapabilityGitWorktree Capability = "git.worktree"
	CapabilityVerify      Capability = "verify.run"
)

type ErrorKind string

const (
	ErrInvalidPolicy        ErrorKind = "invalid_policy"
	ErrPathOutsideWorkspace ErrorKind = "path_outside_workspace"
	ErrPathBlocked          ErrorKind = "path_blocked"
	ErrReadOnly             ErrorKind = "read_only"
	ErrExecutionDenied      ErrorKind = "execution_denied"
	ErrToolNotAllowed       ErrorKind = "tool_not_allowed"
	ErrToolNotFound         ErrorKind = "tool_not_found"
	ErrTimeout              ErrorKind = "timeout"
	ErrNotFound             ErrorKind = "not_found"
	ErrInvalidEdit          ErrorKind = "invalid_edit"
	ErrInvalidVerifier      ErrorKind = "invalid_verifier"
	ErrLimitExceeded        ErrorKind = "limit_exceeded"
	ErrInvalidPath          ErrorKind = "invalid_path"
	ErrIO                   ErrorKind = "io"
)

type RuntimeError struct {
	Kind ErrorKind
	Path string
	Tool string
	Err  error
}

func (e *RuntimeError) Error() string {
	switch e.Kind {
	case ErrInvalidPolicy:
		return "invalid runtime policy"
	case ErrPathOutsideWorkspace:
		return fmt.Sprintf("path is outside workspace: %q", e.Path)
	case ErrPathBlocked:
		return fmt.Sprintf("path is blocked for write: %q", e.Path)
	case ErrReadOnly:
		return "runtime policy does not allow writes"
	case ErrExecutionDenied:
		return "runtime policy does not allow command execution"
	case ErrToolNotAllowed:
		return fmt.Sprintf("tool is not allowed: %q", e.Tool)
	case ErrToolNotFound:
		return fmt.Sprintf("tool not found: %q", e.Tool)
	case ErrTimeout:
		return "command timed out"
	case ErrNotFound:
		return fmt.Sprintf("path not found: %q", e.Path)
	case ErrInvalidEdit:
		return fmt.Sprintf("invalid exact edit for %q", e.Path)
	case ErrInvalidVerifier:
		return fmt.Sprintf("invalid verifier %q", e.Path)
	case ErrLimitExceeded:
		return "runtime limit exceeded"
	case ErrInvalidPath:
		return fmt.Sprintf("invalid path: %q", e.Path)
	case ErrIO:
		return fmt.Sprintf("runtime IO error for %q", e.Path)
	default:
		return "runtime error"
	}
}

func (e *RuntimeError) Unwrap() error { return e.Err }

type Native struct {
	workspace model.Workspace
	root      string
	policy    model.Policy
	verifiers map[string]model.ResolvedVerifier
	guard     *PathGuard
}

func NewNative(workspace model.Workspace, cfg model.EffectiveConfig) (*Native, error) {
	if strings.TrimSpace(workspace.Path) == "" {
		return nil, &RuntimeError{Kind: ErrInvalidPath, Path: workspace.Path}
	}
	root, err := filepath.Abs(filepath.Clean(workspace.Path))
	if err != nil {
		return nil, &RuntimeError{Kind: ErrInvalidPath, Path: workspace.Path, Err: err}
	}
	info, err := os.Stat(root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, &RuntimeError{Kind: ErrNotFound, Path: root, Err: err}
		}
		return nil, &RuntimeError{Kind: ErrIO, Path: root, Err: err}
	}
	if !info.IsDir() {
		return nil, &RuntimeError{Kind: ErrInvalidPath, Path: root}
	}
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return nil, &RuntimeError{Kind: ErrIO, Path: root, Err: err}
	}
	resolvedRoot, err = filepath.Abs(resolvedRoot)
	if err != nil {
		return nil, &RuntimeError{Kind: ErrInvalidPath, Path: root, Err: err}
	}

	policy := model.Policy{Mode: string(ModeReadOnly)}
	if cfg.Policy != nil {
		policy = clonePolicy(cfg.Policy.Policy)
		if strings.TrimSpace(policy.Mode) == "" {
			policy.Mode = string(ModeReadOnly)
		}
	}
	if !validMode(Mode(policy.Mode)) {
		return nil, &RuntimeError{Kind: ErrInvalidPolicy}
	}

	verifiers := make(map[string]model.ResolvedVerifier, len(cfg.Verifiers))
	for id, verifier := range cfg.Verifiers {
		verifiers[id] = cloneResolvedVerifier(verifier)
	}

	guard := &PathGuard{root: filepath.Clean(resolvedRoot)}
	return &Native{
		workspace: workspace,
		root:      filepath.Clean(resolvedRoot),
		policy:    policy,
		verifiers: verifiers,
		guard:     guard,
	}, nil
}

func (r *Native) Root() string { return r.root }

func (r *Native) WorkspaceID() string { return r.workspace.ID }

func (r *Native) Mode() Mode { return Mode(r.policy.Mode) }

func (r *Native) Capabilities() []Capability {
	capabilities := []Capability{CapabilityRead, CapabilitySearch, CapabilityTree}
	switch r.Mode() {
	case ModeWorkspaceWrite:
		capabilities = append(capabilities, CapabilityEdit, CapabilityWrite)
	case ModeStandard, ModeFull:
		capabilities = append(capabilities, CapabilityEdit, CapabilityExec, CapabilityWrite)
		if r.Mode() == ModeFull || r.allowedExecutable("git") {
			capabilities = append(capabilities, CapabilityGitStatus, CapabilityGitDiff, CapabilityGitBranch, CapabilityGitWorktree)
		}
		if len(r.verifiers) > 0 {
			capabilities = append(capabilities, CapabilityVerify)
		}
	}
	sort.Slice(capabilities, func(i, j int) bool { return capabilities[i] < capabilities[j] })
	return capabilities
}

func (r *Native) canWrite() bool {
	return r.Mode() == ModeWorkspaceWrite || r.Mode() == ModeStandard || r.Mode() == ModeFull
}

func (r *Native) canExec() bool {
	return r.Mode() == ModeStandard || r.Mode() == ModeFull
}

func validMode(mode Mode) bool {
	switch mode {
	case ModeReadOnly, ModeWorkspaceWrite, ModeStandard, ModeFull:
		return true
	default:
		return false
	}
}

func clonePolicy(source model.Policy) model.Policy {
	clone := source
	clone.AllowedExecutables = append([]string(nil), source.AllowedExecutables...)
	if source.ToolPaths != nil {
		clone.ToolPaths = make(map[string]string, len(source.ToolPaths))
		for key, value := range source.ToolPaths {
			clone.ToolPaths[key] = value
		}
	}
	return clone
}

func cloneResolvedVerifier(source model.ResolvedVerifier) model.ResolvedVerifier {
	clone := source
	clone.VerifierDefinition.Enabled = cloneBool(source.VerifierDefinition.Enabled)
	clone.VerifierDefinition.Args = append([]string(nil), source.VerifierDefinition.Args...)
	return clone
}

func cloneBool(source *bool) *bool {
	if source == nil {
		return nil
	}
	value := *source
	return &value
}
