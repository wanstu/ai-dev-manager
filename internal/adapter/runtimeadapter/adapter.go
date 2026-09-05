package runtimeadapter

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	admruntime "ai-dev-manager/internal/runtime"
)

const (
	OpTree              = "files.tree"
	OpRead              = "files.read"
	OpWrite             = "files.write"
	OpEdit              = "files.edit"
	OpSearch            = "search.text"
	OpExec              = "shell.exec"
	OpGitStatus         = "git.status"
	OpGitDiff           = "git.diff"
	OpGitBranch         = "git.branch"
	OpGitBranchExists   = "git.branch_exists"
	OpGitResolveRef     = "git.resolve_ref"
	OpGitWorktreeList   = "git.worktree.list"
	OpGitWorktreeGet    = "git.worktree.get"
	OpGitWorktreeCreate = "git.worktree.create"
	OpGitWorktreeRemove = "git.worktree.remove"
	OpGitChangesExport  = "git.changes.export"
	OpGitChangesApply   = "git.changes.apply"
	OpGitPushStatus     = "git.push_status"
	OpGitRelation       = "git.relation"
	OpVerifierRun       = "verify.run"
	OpVerifierRunMany   = "verify.run_many"
)

type State string

const (
	StateReady   State = "ready"
	StateRunning State = "running"
	StateStopped State = "stopped"
	StateError   State = "error"
)

type Status struct {
	ID           string   `json:"id"`
	WorkspaceID  string   `json:"workspace_id"`
	State        State    `json:"state"`
	Capabilities []string `json:"capabilities"`
}

// Runtime is the protocol-neutral boundary consumed by MCP/host adapters.
// External runtimes can implement this contract without depending on Native.
type Runtime interface {
	ID() string
	WorkspaceID() string
	Capabilities() []string
	Status(context.Context) Status
	Invoke(context.Context, string, map[string]any) (any, error)
}

type ErrorKind string

const (
	ErrInvalidInput         ErrorKind = "invalid_input"
	ErrUnsupportedOperation ErrorKind = "unsupported_operation"
)

type AdapterError struct {
	Kind      ErrorKind
	Operation string
	Err       error
}

func (e *AdapterError) Error() string {
	switch e.Kind {
	case ErrInvalidInput:
		return fmt.Sprintf("invalid input for operation %q", e.Operation)
	case ErrUnsupportedOperation:
		return fmt.Sprintf("unsupported runtime operation %q", e.Operation)
	default:
		return "runtime adapter error"
	}
}

func (e *AdapterError) Unwrap() error { return e.Err }

type NativeAdapter struct {
	id      string
	runtime *admruntime.Native
}

func NewNative(id string, native *admruntime.Native) (*NativeAdapter, error) {
	if strings.TrimSpace(id) == "" || native == nil {
		return nil, &AdapterError{Kind: ErrInvalidInput, Operation: "new_native"}
	}
	return &NativeAdapter{id: id, runtime: native}, nil
}

func (a *NativeAdapter) ID() string { return a.id }

func (a *NativeAdapter) WorkspaceID() string { return a.runtime.WorkspaceID() }

func (a *NativeAdapter) Capabilities() []string {
	caps := a.runtime.Capabilities()
	result := make([]string, 0, len(caps))
	for _, capability := range caps {
		result = append(result, string(capability))
	}
	sort.Strings(result)
	return result
}

func (a *NativeAdapter) Status(context.Context) Status {
	return Status{
		ID:           a.ID(),
		WorkspaceID:  a.WorkspaceID(),
		State:        StateReady,
		Capabilities: a.Capabilities(),
	}
}

func (a *NativeAdapter) Invoke(_ context.Context, operation string, input map[string]any) (any, error) {
	switch operation {
	case OpTree:
		var args struct {
			Path       string `json:"path"`
			MaxDepth   int    `json:"max_depth"`
			MaxEntries int    `json:"max_entries"`
		}
		if err := decodeInput(operation, input, &args); err != nil {
			return nil, err
		}
		return a.runtime.Tree(args.Path, admruntime.TreeOptions{MaxDepth: args.MaxDepth, MaxEntries: args.MaxEntries})

	case OpRead:
		var args struct {
			Path     string `json:"path"`
			MaxBytes int    `json:"max_bytes"`
		}
		if err := decodeInput(operation, input, &args); err != nil {
			return nil, err
		}
		data, info, err := a.runtime.Read(args.Path, args.MaxBytes)
		if err != nil {
			return nil, err
		}
		return struct {
			Info    admruntime.FileInfo `json:"info"`
			Content string              `json:"content"`
		}{Info: info, Content: string(data)}, nil

	case OpWrite:
		var args struct {
			Path          string `json:"path"`
			Content       string `json:"content"`
			CreateParents bool   `json:"create_parents"`
		}
		if err := decodeInput(operation, input, &args); err != nil {
			return nil, err
		}
		return a.runtime.Write(args.Path, []byte(args.Content), args.CreateParents)

	case OpEdit:
		var args struct {
			Path                 string `json:"path"`
			OldText              string `json:"old_text"`
			NewText              string `json:"new_text"`
			ExpectedReplacements int    `json:"expected_replacements"`
		}
		if err := decodeInput(operation, input, &args); err != nil {
			return nil, err
		}
		return a.runtime.Edit(args.Path, args.OldText, args.NewText, args.ExpectedReplacements)

	case OpSearch:
		var args struct {
			Path            string `json:"path"`
			Query           string `json:"query"`
			MaxFiles        int    `json:"max_files"`
			MaxMatches      int    `json:"max_matches"`
			MaxBytesPerFile int    `json:"max_bytes_per_file"`
		}
		if err := decodeInput(operation, input, &args); err != nil {
			return nil, err
		}
		return a.runtime.Search(admruntime.SearchOptions{
			Path:            args.Path,
			Query:           args.Query,
			MaxFiles:        args.MaxFiles,
			MaxMatches:      args.MaxMatches,
			MaxBytesPerFile: args.MaxBytesPerFile,
		})

	case OpExec:
		var args struct {
			Executable     string   `json:"executable"`
			Args           []string `json:"args"`
			Cwd            string   `json:"cwd"`
			TimeoutMS      int64    `json:"timeout_ms"`
			MaxOutputBytes int      `json:"max_output_bytes"`
		}
		if err := decodeInput(operation, input, &args); err != nil {
			return nil, err
		}
		return a.runtime.Exec(admruntime.Command{
			Executable:     args.Executable,
			Args:           append([]string(nil), args.Args...),
			Cwd:            args.Cwd,
			Timeout:        time.Duration(args.TimeoutMS) * time.Millisecond,
			MaxOutputBytes: args.MaxOutputBytes,
		})

	case OpGitStatus:
		return a.runtime.GitStatus()
	case OpGitDiff:
		return a.runtime.GitDiff()
	case OpGitBranch:
		return a.runtime.GitBranch()
	case OpGitBranchExists:
		var args struct {
			Branch string `json:"branch"`
		}
		if err := decodeInput(operation, input, &args); err != nil {
			return nil, err
		}
		return a.runtime.GitBranchExists(args.Branch)
	case OpGitResolveRef:
		var args struct {
			Ref string `json:"ref"`
		}
		if err := decodeInput(operation, input, &args); err != nil {
			return nil, err
		}
		return a.runtime.GitResolveRef(args.Ref)
	case OpGitWorktreeList:
		return a.runtime.GitWorktrees()
	case OpGitWorktreeGet:
		var args struct {
			Name string `json:"name"`
		}
		if err := decodeInput(operation, input, &args); err != nil {
			return nil, err
		}
		return a.runtime.GitWorktreeGet(args.Name)

	case OpGitWorktreeCreate:
		var args struct {
			Name       string `json:"name"`
			Branch     string `json:"branch"`
			StartPoint string `json:"start_point"`
		}
		if err := decodeInput(operation, input, &args); err != nil {
			return nil, err
		}
		if strings.TrimSpace(args.StartPoint) == "" {
			return a.runtime.GitWorktreeCreate(args.Name, args.Branch)
		}
		return a.runtime.GitWorktreeCreateAt(args.Name, args.Branch, args.StartPoint)

	case OpGitWorktreeRemove:
		var args struct {
			Name  string `json:"name"`
			Force bool   `json:"force"`
		}
		if err := decodeInput(operation, input, &args); err != nil {
			return nil, err
		}
		if err := a.runtime.GitWorktreeRemoveWithOptions(args.Name, args.Force); err != nil {
			return nil, err
		}
		return struct {
			Removed bool `json:"removed"`
		}{Removed: true}, nil

	case OpGitChangesExport:
		return a.runtime.GitExportChanges()
	case OpGitChangesApply:
		var args admruntime.GitChangeSet
		if err := decodeInput(operation, input, &args); err != nil {
			return nil, err
		}
		if err := a.runtime.GitApplyChanges(args); err != nil {
			return nil, err
		}
		return struct {
			Applied bool `json:"applied"`
		}{Applied: true}, nil
	case OpGitPushStatus:
		return a.runtime.GitPushStatus()
	case OpGitRelation:
		var args struct {
			Left  string `json:"left"`
			Right string `json:"right"`
		}
		if err := decodeInput(operation, input, &args); err != nil {
			return nil, err
		}
		return a.runtime.GitRelation(args.Left, args.Right)

	case OpVerifierRun:
		var args struct {
			ID string `json:"id"`
		}
		if err := decodeInput(operation, input, &args); err != nil {
			return nil, err
		}
		return a.runtime.RunVerifier(args.ID)

	case OpVerifierRunMany:
		var args struct {
			IDs []string `json:"ids"`
		}
		if err := decodeInput(operation, input, &args); err != nil {
			return nil, err
		}
		return a.runtime.RunVerifiers(args.IDs...)
	default:
		return nil, &AdapterError{Kind: ErrUnsupportedOperation, Operation: operation}
	}
}

func decodeInput(operation string, input map[string]any, target any) error {
	if input == nil {
		input = map[string]any{}
	}
	data, err := json.Marshal(input)
	if err != nil {
		return &AdapterError{Kind: ErrInvalidInput, Operation: operation, Err: err}
	}
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return &AdapterError{Kind: ErrInvalidInput, Operation: operation, Err: err}
	}
	return nil
}

func IsAdapterErrorKind(err error, kind ErrorKind) bool {
	var adapterErr *AdapterError
	return errors.As(err, &adapterErr) && adapterErr.Kind == kind
}
