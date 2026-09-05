package mcpgateway

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"

	"ai-dev-manager/internal/adapter/runtimeadapter"
	"ai-dev-manager/internal/environment"
	"ai-dev-manager/internal/gateway"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	serverName    = "ai-dev-manager-gateway"
	serverVersion = "v0.6.0"
)

type Discovery interface {
	Info() gateway.Info
	Workspaces() ([]gateway.WorkspaceSummary, error)
	Environments() ([]environment.Environment, error)
	InspectEnvironment(context.Context, string) (environment.InspectResult, error)
	CreateEnvironment(context.Context, environment.CreateRequest) (environment.InspectResult, error)
	DestroyEnvironment(context.Context, string, bool) (environment.Environment, error)
	AcquireEnvironmentWriter(context.Context, string, string) (environment.Environment, error)
	ReleaseEnvironmentWriter(string, string, bool) (environment.Environment, error)
	InvokeEnvironmentRead(context.Context, string, string, map[string]any) (any, error)
	InvokeEnvironmentMutation(context.Context, string, string, string, map[string]any) (any, error)
	DomainError(context.Context, error, *environment.Environment, *environment.InspectResult) gateway.DomainError
}

type EmptyInput struct{}

type GatewayInfoOutput struct {
	Gateway gateway.Info `json:"gateway"`
}

type WorkspaceListOutput struct {
	Workspaces []gateway.WorkspaceSummary `json:"workspaces"`
}

type EnvironmentListOutput struct {
	Environments []environment.Environment `json:"environments"`
}

type EnvironmentInspectInput struct {
	EnvironmentID string `json:"environment_id" jsonschema:"stable env_ identifier returned by environment_list"`
}

type EnvironmentInspectOutput struct {
	OK         bool                       `json:"ok"`
	Inspection *environment.InspectResult `json:"inspection,omitempty"`
	Error      *gateway.DomainError       `json:"error,omitempty"`
}

type EnvironmentCreateInput struct {
	WorkspaceID    string `json:"workspace_id" jsonschema:"stable ws_ identifier returned by workspace_list"`
	Name           string `json:"name" jsonschema:"human-readable task Environment name"`
	Base           string `json:"base,omitempty" jsonschema:"optional branch, tag, or commit base; defaults to current checkout branch/HEAD"`
	Branch         string `json:"branch,omitempty" jsonschema:"optional Environment branch; defaults to adm/<sanitized-name>"`
	IncludeChanges bool   `json:"include_changes,omitempty" jsonschema:"copy staged, unstaged, and untracked non-ignored regular files from the base checkout"`
}

type EnvironmentCreateOutput struct {
	OK         bool                       `json:"ok"`
	Inspection *environment.InspectResult `json:"inspection,omitempty"`
	Error      *gateway.DomainError       `json:"error,omitempty"`
}

type EnvironmentDestroyInput struct {
	EnvironmentID string `json:"environment_id"`
	Force         bool   `json:"force,omitempty" jsonschema:"explicit destructive override for dirty, unpushed, active-writer, or missing-worktree cleanup guards"`
}

type EnvironmentResultOutput struct {
	OK          bool                     `json:"ok"`
	Environment *environment.Environment `json:"environment,omitempty"`
	Error       *gateway.DomainError     `json:"error,omitempty"`
}

type EnvironmentWriterAcquireInput struct {
	EnvironmentID string `json:"environment_id"`
	Owner         string `json:"owner" jsonschema:"stable caller/session coordination identifier"`
}

type EnvironmentWriterReleaseInput struct {
	EnvironmentID string `json:"environment_id"`
	Owner         string `json:"owner,omitempty" jsonschema:"current writer owner; required unless force=true"`
	Force         bool   `json:"force,omitempty" jsonschema:"explicitly clear the current writer lease without matching owner"`
}

type RoutedTreeInput struct {
	EnvironmentID string `json:"environment_id"`
	Path          string `json:"path,omitempty"`
	MaxDepth      int    `json:"max_depth,omitempty"`
	MaxEntries    int    `json:"max_entries,omitempty"`
}

type RoutedReadInput struct {
	EnvironmentID string `json:"environment_id"`
	Path          string `json:"path"`
	MaxBytes      int    `json:"max_bytes,omitempty"`
}

type RoutedSearchInput struct {
	EnvironmentID   string `json:"environment_id"`
	Path            string `json:"path,omitempty"`
	Query           string `json:"query"`
	MaxFiles        int    `json:"max_files,omitempty"`
	MaxMatches      int    `json:"max_matches,omitempty"`
	MaxBytesPerFile int    `json:"max_bytes_per_file,omitempty"`
}

type RoutedEnvironmentInput struct {
	EnvironmentID string `json:"environment_id"`
}

type RoutedReadOutput struct {
	OK     bool                 `json:"ok"`
	Result any                  `json:"result,omitempty"`
	Error  *gateway.DomainError `json:"error,omitempty"`
}

type RoutedWriteInput struct {
	EnvironmentID string `json:"environment_id"`
	WriterOwner   string `json:"writer_owner"`
	Path          string `json:"path"`
	Content       string `json:"content"`
	CreateParents bool   `json:"create_parents,omitempty"`
}

type RoutedEditInput struct {
	EnvironmentID        string `json:"environment_id"`
	WriterOwner          string `json:"writer_owner"`
	Path                 string `json:"path"`
	OldText              string `json:"old_text"`
	NewText              string `json:"new_text"`
	ExpectedReplacements int    `json:"expected_replacements,omitempty"`
}

type RoutedDeleteInput struct {
	EnvironmentID string `json:"environment_id"`
	WriterOwner   string `json:"writer_owner"`
	Path          string `json:"path"`
}

type RoutedExecInput struct {
	EnvironmentID  string   `json:"environment_id"`
	WriterOwner    string   `json:"writer_owner"`
	Executable     string   `json:"executable"`
	Args           []string `json:"args,omitempty"`
	Cwd            string   `json:"cwd,omitempty"`
	TimeoutMS      int64    `json:"timeout_ms,omitempty"`
	MaxOutputBytes int      `json:"max_output_bytes,omitempty"`
}

type RoutedVerifierInput struct {
	EnvironmentID string `json:"environment_id"`
	WriterOwner   string `json:"writer_owner"`
	ID            string `json:"id"`
}

type RoutedVerifiersInput struct {
	EnvironmentID string   `json:"environment_id"`
	WriterOwner   string   `json:"writer_owner"`
	IDs           []string `json:"ids,omitempty"`
}

type RoutedMutationOutput struct {
	OK     bool                 `json:"ok"`
	Result any                  `json:"result,omitempty"`
	Error  *gateway.DomainError `json:"error,omitempty"`
}

func routedOutputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"ok":     map[string]any{"type": "boolean"},
			"result": map[string]any{},
			"error":  map[string]any{},
		},
		"required":             []string{"ok"},
		"additionalProperties": false,
	}
}

func New(discovery Discovery) *mcp.Server {
	server := mcp.NewServer(&mcp.Implementation{Name: serverName, Version: serverVersion}, nil)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "gateway_info",
		Description: "Describe the ADM Agent MCP Gateway and its currently available capabilities.",
	}, func(context.Context, *mcp.CallToolRequest, EmptyInput) (*mcp.CallToolResult, GatewayInfoOutput, error) {
		return nil, GatewayInfoOutput{Gateway: discovery.Info()}, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "workspace_list",
		Description: "List registered development Workspaces that can host isolated Environments.",
	}, func(context.Context, *mcp.CallToolRequest, EmptyInput) (*mcp.CallToolResult, WorkspaceListOutput, error) {
		items, err := discovery.Workspaces()
		if err != nil {
			return nil, WorkspaceListOutput{}, sanitizeDiscoveryError(err)
		}
		return nil, WorkspaceListOutput{Workspaces: items}, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "environment_list",
		Description: "List persistent isolated development Environments without changing their activity state.",
	}, func(context.Context, *mcp.CallToolRequest, EmptyInput) (*mcp.CallToolResult, EnvironmentListOutput, error) {
		items, err := discovery.Environments()
		if err != nil {
			return nil, EnvironmentListOutput{}, sanitizeDiscoveryError(err)
		}
		return nil, EnvironmentListOutput{Environments: items}, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "environment_inspect",
		Description: "Inspect one Environment's validated worktree, Git/base/activity/writer facts, warnings, hints and runtime capabilities.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input EnvironmentInspectInput) (*mcp.CallToolResult, EnvironmentInspectOutput, error) {
		environmentID := strings.TrimSpace(input.EnvironmentID)
		if environmentID == "" {
			return domainToolError(EnvironmentInspectOutput{OK: false, Error: invalidInput("environment_id is required")})
		}
		result, err := discovery.InspectEnvironment(ctx, environmentID)
		if err != nil {
			domain := discovery.DomainError(ctx, err, nil, nil)
			return domainToolError(EnvironmentInspectOutput{OK: false, Error: &domain})
		}
		return nil, EnvironmentInspectOutput{OK: true, Inspection: &result}, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "environment_create",
		Description: "Create a persistent isolated Environment using ADM managed-worktree safety. Dirty base changes are excluded unless include_changes=true.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input EnvironmentCreateInput) (*mcp.CallToolResult, EnvironmentCreateOutput, error) {
		input.WorkspaceID = strings.TrimSpace(input.WorkspaceID)
		input.Name = strings.TrimSpace(input.Name)
		if input.WorkspaceID == "" || input.Name == "" {
			return domainToolError(EnvironmentCreateOutput{OK: false, Error: invalidInput("workspace_id and name are required")})
		}
		result, err := discovery.CreateEnvironment(ctx, environment.CreateRequest{
			WorkspaceID:    input.WorkspaceID,
			Name:           input.Name,
			Base:           strings.TrimSpace(input.Base),
			Branch:         strings.TrimSpace(input.Branch),
			IncludeChanges: input.IncludeChanges,
		})
		if err != nil {
			var inspection *environment.InspectResult
			if result.Environment.ID != "" {
				inspection = &result
			}
			domain := discovery.DomainError(ctx, err, nil, inspection)
			return domainToolError(EnvironmentCreateOutput{OK: false, Inspection: inspection, Error: &domain})
		}
		return nil, EnvironmentCreateOutput{OK: true, Inspection: &result}, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "environment_destroy",
		Description: "Destroy a managed Environment. Normal destroy protects dirty, unpushed and active-writer work; force=true is an explicit destructive override. Branches are never deleted automatically.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input EnvironmentDestroyInput) (*mcp.CallToolResult, EnvironmentResultOutput, error) {
		environmentID := strings.TrimSpace(input.EnvironmentID)
		if environmentID == "" {
			return domainToolError(EnvironmentResultOutput{OK: false, Error: invalidInput("environment_id is required")})
		}
		result, err := discovery.DestroyEnvironment(ctx, environmentID, input.Force)
		if err != nil {
			domain := discovery.DomainError(ctx, err, &result, nil)
			return domainToolError(EnvironmentResultOutput{OK: false, Environment: environmentPointer(result), Error: &domain})
		}
		return nil, EnvironmentResultOutput{OK: true, Environment: &result}, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "environment_writer_acquire",
		Description: "Acquire or renew the single-writer lease for an Environment. A different active owner is never silently stolen.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input EnvironmentWriterAcquireInput) (*mcp.CallToolResult, EnvironmentResultOutput, error) {
		environmentID := strings.TrimSpace(input.EnvironmentID)
		owner := strings.TrimSpace(input.Owner)
		if environmentID == "" || owner == "" {
			return domainToolError(EnvironmentResultOutput{OK: false, Error: invalidInput("environment_id and owner are required")})
		}
		result, err := discovery.AcquireEnvironmentWriter(ctx, environmentID, owner)
		if err != nil {
			domain := discovery.DomainError(ctx, err, &result, nil)
			return domainToolError(EnvironmentResultOutput{OK: false, Environment: environmentPointer(result), Error: &domain})
		}
		return nil, EnvironmentResultOutput{OK: true, Environment: &result}, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "environment_writer_release",
		Description: "Release the current Environment writer lease. Normal release requires matching owner; force=true explicitly clears the lease only.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input EnvironmentWriterReleaseInput) (*mcp.CallToolResult, EnvironmentResultOutput, error) {
		environmentID := strings.TrimSpace(input.EnvironmentID)
		owner := strings.TrimSpace(input.Owner)
		if environmentID == "" || (!input.Force && owner == "") {
			return domainToolError(EnvironmentResultOutput{OK: false, Error: invalidInput("environment_id and owner are required unless force=true")})
		}
		result, err := discovery.ReleaseEnvironmentWriter(environmentID, owner, input.Force)
		if err != nil {
			domain := discovery.DomainError(ctx, err, &result, nil)
			return domainToolError(EnvironmentResultOutput{OK: false, Environment: environmentPointer(result), Error: &domain})
		}
		return nil, EnvironmentResultOutput{OK: true, Environment: &result}, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:         "tree",
		Description:  "List files and directories inside one validated Environment. Requires environment_id but no writer lease.",
		OutputSchema: routedOutputSchema(),
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input RoutedTreeInput) (*mcp.CallToolResult, RoutedReadOutput, error) {
		return invokeRoutedRead(ctx, discovery, input.EnvironmentID, runtimeadapter.OpTree, map[string]any{
			"path": input.Path, "max_depth": input.MaxDepth, "max_entries": input.MaxEntries,
		})
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:         "read",
		Description:  "Read a text file from one validated Environment. Requires environment_id but no writer lease.",
		OutputSchema: routedOutputSchema(),
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input RoutedReadInput) (*mcp.CallToolResult, RoutedReadOutput, error) {
		if strings.TrimSpace(input.Path) == "" {
			return domainToolError(RoutedReadOutput{OK: false, Error: invalidInput("path is required")})
		}
		return invokeRoutedRead(ctx, discovery, input.EnvironmentID, runtimeadapter.OpRead, map[string]any{
			"path": input.Path, "max_bytes": input.MaxBytes,
		})
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:         "search",
		Description:  "Search literal text inside one validated Environment. Requires environment_id but no writer lease.",
		OutputSchema: routedOutputSchema(),
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input RoutedSearchInput) (*mcp.CallToolResult, RoutedReadOutput, error) {
		if strings.TrimSpace(input.Query) == "" {
			return domainToolError(RoutedReadOutput{OK: false, Error: invalidInput("query is required")})
		}
		return invokeRoutedRead(ctx, discovery, input.EnvironmentID, runtimeadapter.OpSearch, map[string]any{
			"path": input.Path, "query": input.Query, "max_files": input.MaxFiles, "max_matches": input.MaxMatches, "max_bytes_per_file": input.MaxBytesPerFile,
		})
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:         "git_status",
		Description:  "Return structured Git status for one validated Environment.",
		OutputSchema: routedOutputSchema(),
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input RoutedEnvironmentInput) (*mcp.CallToolResult, RoutedReadOutput, error) {
		return invokeRoutedRead(ctx, discovery, input.EnvironmentID, runtimeadapter.OpGitStatus, nil)
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:         "git_diff",
		Description:  "Return changed files and Git patch for one validated Environment.",
		OutputSchema: routedOutputSchema(),
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input RoutedEnvironmentInput) (*mcp.CallToolResult, RoutedReadOutput, error) {
		return invokeRoutedRead(ctx, discovery, input.EnvironmentID, runtimeadapter.OpGitDiff, nil)
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:         "git_branch",
		Description:  "Return the current Git branch for one validated Environment.",
		OutputSchema: routedOutputSchema(),
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input RoutedEnvironmentInput) (*mcp.CallToolResult, RoutedReadOutput, error) {
		return invokeRoutedRead(ctx, discovery, input.EnvironmentID, runtimeadapter.OpGitBranch, nil)
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:         "write",
		Description:  "Write a text file inside one Environment. Requires the current writer_owner and remains subject to Runtime write policy.",
		OutputSchema: routedOutputSchema(),
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input RoutedWriteInput) (*mcp.CallToolResult, RoutedMutationOutput, error) {
		if strings.TrimSpace(input.Path) == "" {
			return domainToolError(RoutedMutationOutput{OK: false, Error: invalidInput("path is required")})
		}
		return invokeRoutedMutation(ctx, discovery, input.EnvironmentID, input.WriterOwner, runtimeadapter.OpWrite, map[string]any{
			"path": input.Path, "content": input.Content, "create_parents": input.CreateParents,
		})
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:         "edit",
		Description:  "Apply an exact text replacement inside one Environment. Requires the current writer_owner.",
		OutputSchema: routedOutputSchema(),
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input RoutedEditInput) (*mcp.CallToolResult, RoutedMutationOutput, error) {
		if strings.TrimSpace(input.Path) == "" {
			return domainToolError(RoutedMutationOutput{OK: false, Error: invalidInput("path is required")})
		}
		return invokeRoutedMutation(ctx, discovery, input.EnvironmentID, input.WriterOwner, runtimeadapter.OpEdit, map[string]any{
			"path": input.Path, "old_text": input.OldText, "new_text": input.NewText, "expected_replacements": input.ExpectedReplacements,
		})
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:         "delete",
		Description:  "Delete one file or allowed symlink inside one Environment. Requires the current writer_owner; directories are rejected.",
		OutputSchema: routedOutputSchema(),
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input RoutedDeleteInput) (*mcp.CallToolResult, RoutedMutationOutput, error) {
		if strings.TrimSpace(input.Path) == "" {
			return domainToolError(RoutedMutationOutput{OK: false, Error: invalidInput("path is required")})
		}
		return invokeRoutedMutation(ctx, discovery, input.EnvironmentID, input.WriterOwner, runtimeadapter.OpDelete, map[string]any{
			"path": input.Path,
		})
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:         "exec",
		Description:  "Execute a structured executable plus argv inside one Environment under Runtime policy. Requires the current writer_owner; shell strings are not accepted.",
		OutputSchema: routedOutputSchema(),
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input RoutedExecInput) (*mcp.CallToolResult, RoutedMutationOutput, error) {
		if strings.TrimSpace(input.Executable) == "" {
			return domainToolError(RoutedMutationOutput{OK: false, Error: invalidInput("executable is required")})
		}
		return invokeRoutedMutation(ctx, discovery, input.EnvironmentID, input.WriterOwner, runtimeadapter.OpExec, map[string]any{
			"executable": input.Executable, "args": input.Args, "cwd": input.Cwd, "timeout_ms": input.TimeoutMS, "max_output_bytes": input.MaxOutputBytes,
		})
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:         "run_verifier",
		Description:  "Run one configured project verifier inside an Environment. Requires the current writer_owner because verifier commands may generate files or active runtime state.",
		OutputSchema: routedOutputSchema(),
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input RoutedVerifierInput) (*mcp.CallToolResult, RoutedMutationOutput, error) {
		if strings.TrimSpace(input.ID) == "" {
			return domainToolError(RoutedMutationOutput{OK: false, Error: invalidInput("id is required")})
		}
		return invokeRoutedMutation(ctx, discovery, input.EnvironmentID, input.WriterOwner, runtimeadapter.OpVerifierRun, map[string]any{"id": input.ID})
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:         "run_verifiers",
		Description:  "Run selected configured project verifiers, or all when ids is empty, inside one Environment. Requires the current writer_owner.",
		OutputSchema: routedOutputSchema(),
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input RoutedVerifiersInput) (*mcp.CallToolResult, RoutedMutationOutput, error) {
		return invokeRoutedMutation(ctx, discovery, input.EnvironmentID, input.WriterOwner, runtimeadapter.OpVerifierRunMany, map[string]any{"ids": input.IDs})
	})

	return server
}

func NewHTTPHandler(discovery Discovery) http.Handler {
	server := New(discovery)
	return mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server {
		return server
	}, &mcp.StreamableHTTPOptions{Stateless: true})
}

type ExposedHTTPOptions struct {
	AllowedHosts   []string
	AllowIPLiteral bool
}

// NewExposedHTTPHandler is the explicit non-loopback Gateway transport used by
// local Docker clients. Disabling the SDK localhost protection is paired with a
// narrow Host/Origin allowlist rather than accepting arbitrary authorities.
func NewExposedHTTPHandler(discovery Discovery, options ExposedHTTPOptions) http.Handler {
	server := New(discovery)
	base := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server {
		return server
	}, &mcp.StreamableHTTPOptions{Stateless: true, DisableLocalhostProtection: true})

	allowed := make(map[string]struct{}, len(options.AllowedHosts))
	for _, host := range options.AllowedHosts {
		host = normalizeHTTPHost(host)
		if host != "" {
			allowed[host] = struct{}{}
		}
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !allowedHTTPHost(r.Host, allowed, options.AllowIPLiteral) {
			http.Error(w, "Forbidden: invalid Host header", http.StatusForbidden)
			return
		}
		if origin := strings.TrimSpace(r.Header.Get("Origin")); origin != "" {
			parsed, err := url.Parse(origin)
			if err != nil || !allowedHTTPHost(parsed.Host, allowed, options.AllowIPLiteral) {
				http.Error(w, "Forbidden: invalid Origin header", http.StatusForbidden)
				return
			}
		}
		base.ServeHTTP(w, r)
	})
}

func allowedHTTPHost(authority string, allowed map[string]struct{}, allowIPLiteral bool) bool {
	host := normalizeHTTPHost(authority)
	if host == "" {
		return false
	}
	if host == "localhost" {
		return true
	}
	if ip := net.ParseIP(host); ip != nil {
		return ip.IsLoopback() || allowIPLiteral
	}
	_, ok := allowed[host]
	return ok
}

func normalizeHTTPHost(authority string) string {
	authority = strings.TrimSpace(authority)
	if host, _, err := net.SplitHostPort(authority); err == nil {
		authority = host
	}
	authority = strings.Trim(authority, "[]")
	authority = strings.TrimSuffix(strings.ToLower(authority), ".")
	return authority
}

func invalidInput(message string) *gateway.DomainError {
	return &gateway.DomainError{Code: "invalid_input", Message: message}
}

func environmentPointer(env environment.Environment) *environment.Environment {
	if env.ID == "" {
		return nil
	}
	copyEnv := env
	return &copyEnv
}

func domainToolError[Out any](output Out) (*mcp.CallToolResult, Out, error) {
	return &mcp.CallToolResult{IsError: true}, output, nil
}

func invokeRoutedRead(ctx context.Context, discovery Discovery, environmentID, operation string, input map[string]any) (*mcp.CallToolResult, RoutedReadOutput, error) {
	environmentID = strings.TrimSpace(environmentID)
	if environmentID == "" {
		return domainToolError(RoutedReadOutput{OK: false, Error: invalidInput("environment_id is required")})
	}
	result, err := discovery.InvokeEnvironmentRead(ctx, environmentID, operation, input)
	if err != nil {
		domain := discovery.DomainError(ctx, err, nil, nil)
		return domainToolError(RoutedReadOutput{OK: false, Error: &domain})
	}
	return nil, RoutedReadOutput{OK: true, Result: result}, nil
}

func invokeRoutedMutation(ctx context.Context, discovery Discovery, environmentID, writerOwner, operation string, input map[string]any) (*mcp.CallToolResult, RoutedMutationOutput, error) {
	environmentID = strings.TrimSpace(environmentID)
	writerOwner = strings.TrimSpace(writerOwner)
	if environmentID == "" || writerOwner == "" {
		return domainToolError(RoutedMutationOutput{OK: false, Error: invalidInput("environment_id and writer_owner are required")})
	}
	result, err := discovery.InvokeEnvironmentMutation(ctx, environmentID, writerOwner, operation, input)
	if err != nil {
		domain := discovery.DomainError(ctx, err, nil, nil)
		return domainToolError(RoutedMutationOutput{OK: false, Error: &domain})
	}
	return nil, RoutedMutationOutput{OK: true, Result: result}, nil
}

func sanitizeDiscoveryError(err error) error {
	var envErr *environment.Error
	if errors.As(err, &envErr) {
		message := strings.TrimSpace(envErr.Message)
		if message == "" {
			message = string(envErr.Code)
		}
		return fmt.Errorf("%s: %s", envErr.Code, message)
	}
	return errors.New("gateway_discovery_failed: discovery operation failed")
}
