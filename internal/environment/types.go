package environment

import "time"

type State string

const (
	StateCreating State = "creating"
	StateReady    State = "ready"
	StateError    State = "error"
)

type Environment struct {
	ID             string            `json:"environment_id"`
	WorkspaceID    string            `json:"workspace_id"`
	Name           string            `json:"name"`
	Branch         string            `json:"branch"`
	BaseRef        string            `json:"base_ref"`
	BaseCommit     string            `json:"base_commit"`
	WorktreeName   string            `json:"worktree_name"`
	WorktreePath   string            `json:"worktree_path,omitempty"`
	State          State             `json:"state"`
	CreatedAt      time.Time         `json:"created_at"`
	UpdatedAt      time.Time         `json:"updated_at"`
	LastActivityAt time.Time         `json:"last_activity_at"`
	Writer         *WriterLease      `json:"writer,omitempty"`
	Metadata       map[string]string `json:"metadata,omitempty"`
	Error          *EnvironmentError `json:"error,omitempty"`
}

type WriterLease struct {
	Owner      string    `json:"owner"`
	AcquiredAt time.Time `json:"acquired_at"`
	LastSeenAt time.Time `json:"last_seen_at"`
}

type EnvironmentError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type Fact struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Value   any    `json:"value,omitempty"`
}

type Warning struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type Hint struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type CreateRequest struct {
	WorkspaceID    string `json:"workspace_id"`
	Name           string `json:"name"`
	Base           string `json:"base,omitempty"`
	Branch         string `json:"branch,omitempty"`
	IncludeChanges bool   `json:"include_changes,omitempty"`
}

type InspectResult struct {
	Environment  Environment `json:"environment"`
	Facts        []Fact      `json:"facts,omitempty"`
	Warnings     []Warning   `json:"warnings,omitempty"`
	Hints        []Hint      `json:"hints,omitempty"`
	Capabilities []string    `json:"capabilities,omitempty"`
}

type ErrorCode string

const (
	ErrNotFound          ErrorCode = "environment_not_found"
	ErrInvalidInput      ErrorCode = "invalid_input"
	ErrBranchExists      ErrorCode = "branch_exists"
	ErrCapabilityMissing ErrorCode = "capability_missing"
	ErrWorktreeMissing   ErrorCode = "worktree_missing"
	ErrWorktreeMismatch  ErrorCode = "worktree_mismatch"
	ErrUnsafeDestroy     ErrorCode = "unsafe_destroy"
	ErrWriterConflict    ErrorCode = "writer_conflict"
	ErrWriterNotOwner    ErrorCode = "writer_not_owner"
	ErrStore             ErrorCode = "store_error"
	ErrRuntime           ErrorCode = "runtime_error"
)

type Error struct {
	Code          ErrorCode `json:"code"`
	EnvironmentID string    `json:"environment_id,omitempty"`
	Message       string    `json:"message"`
	Err           error     `json:"-"`
}

func (e *Error) Error() string {
	if e.Message != "" {
		return e.Message
	}
	return string(e.Code)
}

func (e *Error) Unwrap() error { return e.Err }
