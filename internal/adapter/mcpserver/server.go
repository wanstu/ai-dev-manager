package mcpserver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"ai-dev-manager/internal/adapter/runtimeadapter"
	admruntime "ai-dev-manager/internal/runtime"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	serverName    = "ai-dev-manager"
	serverVersion = "v0.1.0"
)

type EmptyInput struct{}

type RuntimeInfoOutput struct {
	ID           string   `json:"id"`
	WorkspaceID  string   `json:"workspace_id"`
	State        string   `json:"state"`
	Capabilities []string `json:"capabilities"`
}

type TreeInput struct {
	Path       string `json:"path,omitempty" jsonschema:"workspace-relative directory path; defaults to root"`
	MaxDepth   int    `json:"max_depth,omitempty" jsonschema:"maximum traversal depth"`
	MaxEntries int    `json:"max_entries,omitempty" jsonschema:"maximum returned entries"`
}

type ReadInput struct {
	Path     string `json:"path" jsonschema:"workspace-relative file path"`
	MaxBytes int    `json:"max_bytes,omitempty" jsonschema:"maximum bytes to read"`
}

type SearchInput struct {
	Path            string `json:"path,omitempty" jsonschema:"workspace-relative directory path; defaults to root"`
	Query           string `json:"query" jsonschema:"literal text to search for"`
	MaxFiles        int    `json:"max_files,omitempty"`
	MaxMatches      int    `json:"max_matches,omitempty"`
	MaxBytesPerFile int    `json:"max_bytes_per_file,omitempty"`
}

type WriteInput struct {
	Path          string `json:"path" jsonschema:"workspace-relative file path"`
	Content       string `json:"content" jsonschema:"complete text content"`
	CreateParents bool   `json:"create_parents,omitempty"`
}

type EditInput struct {
	Path                 string `json:"path"`
	OldText              string `json:"old_text"`
	NewText              string `json:"new_text"`
	ExpectedReplacements int    `json:"expected_replacements,omitempty"`
}

type ExecInput struct {
	Executable     string   `json:"executable"`
	Args           []string `json:"args,omitempty"`
	Cwd            string   `json:"cwd,omitempty"`
	TimeoutMS      int64    `json:"timeout_ms,omitempty"`
	MaxOutputBytes int      `json:"max_output_bytes,omitempty"`
}

type WorktreeCreateInput struct {
	Name   string `json:"name"`
	Branch string `json:"branch"`
}

type WorktreeRemoveInput struct {
	Name string `json:"name"`
}

type VerifierInput struct {
	ID string `json:"id"`
}

type VerifiersInput struct {
	IDs []string `json:"ids,omitempty"`
}

func New(adapter runtimeadapter.Runtime) *mcp.Server {
	server := mcp.NewServer(&mcp.Implementation{Name: serverName, Version: serverVersion}, nil)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "runtime_info",
		Description: "Return this MCP instance's runtime identity, workspace identity, state and capabilities.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, _ EmptyInput) (*mcp.CallToolResult, RuntimeInfoOutput, error) {
		status := adapter.Status(ctx)
		return nil, RuntimeInfoOutput{
			ID:           status.ID,
			WorkspaceID:  status.WorkspaceID,
			State:        string(status.State),
			Capabilities: append([]string(nil), status.Capabilities...),
		}, nil
	})

	caps := capabilitySet(adapter.Capabilities())
	if caps[string(admruntime.CapabilityTree)] {
		addInvokeTool(server, adapter, "tree", "List files and directories inside the bound workspace.", runtimeadapter.OpTree, TreeInput{})
	}
	if caps[string(admruntime.CapabilityRead)] {
		addInvokeTool(server, adapter, "read", "Read a text file inside the bound workspace.", runtimeadapter.OpRead, ReadInput{})
	}
	if caps[string(admruntime.CapabilitySearch)] {
		addInvokeTool(server, adapter, "search", "Search literal text inside files in the bound workspace.", runtimeadapter.OpSearch, SearchInput{})
	}
	if caps[string(admruntime.CapabilityWrite)] {
		addInvokeTool(server, adapter, "write", "Write a text file inside the bound workspace.", runtimeadapter.OpWrite, WriteInput{})
	}
	if caps[string(admruntime.CapabilityEdit)] {
		addInvokeTool(server, adapter, "edit", "Apply an exact text replacement inside the bound workspace.", runtimeadapter.OpEdit, EditInput{})
	}
	if caps[string(admruntime.CapabilityExec)] {
		addInvokeTool(server, adapter, "exec", "Execute a structured executable plus argv under the runtime policy.", runtimeadapter.OpExec, ExecInput{})
	}
	if caps[string(admruntime.CapabilityGitStatus)] {
		addInvokeTool(server, adapter, "git_status", "Return structured Git status for the bound workspace.", runtimeadapter.OpGitStatus, EmptyInput{})
	}
	if caps[string(admruntime.CapabilityGitDiff)] {
		addInvokeTool(server, adapter, "git_diff", "Return changed files and Git patch for the bound workspace.", runtimeadapter.OpGitDiff, EmptyInput{})
	}
	if caps[string(admruntime.CapabilityGitBranch)] {
		addInvokeTool(server, adapter, "git_branch", "Return the current Git branch for the bound workspace.", runtimeadapter.OpGitBranch, EmptyInput{})
	}
	if caps[string(admruntime.CapabilityGitWorktree)] {
		addInvokeTool(server, adapter, "git_worktrees", "List Git worktrees for the bound workspace repository.", runtimeadapter.OpGitWorktreeList, EmptyInput{})
		addInvokeTool(server, adapter, "git_worktree_create", "Create a managed Git worktree without switching the main checkout.", runtimeadapter.OpGitWorktreeCreate, WorktreeCreateInput{})
		addInvokeTool(server, adapter, "git_worktree_remove", "Remove a managed Git worktree.", runtimeadapter.OpGitWorktreeRemove, WorktreeRemoveInput{})
	}
	if caps[string(admruntime.CapabilityVerify)] {
		addInvokeTool(server, adapter, "run_verifier", "Run one project verifier and return a structured result.", runtimeadapter.OpVerifierRun, VerifierInput{})
		addInvokeTool(server, adapter, "run_verifiers", "Run selected project verifiers, or all when ids is empty.", runtimeadapter.OpVerifierRunMany, VerifiersInput{})
	}
	return server
}

func RunStdio(ctx context.Context, adapter runtimeadapter.Runtime) error {
	return New(adapter).Run(ctx, &mcp.StdioTransport{})
}

func NewHTTPHandler(adapter runtimeadapter.Runtime) http.Handler {
	server := New(adapter)
	return mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server {
		return server
	}, &mcp.StreamableHTTPOptions{Stateless: true})
}

func addInvokeTool[In any](server *mcp.Server, adapter runtimeadapter.Runtime, name, description, operation string, _ In) {
	mcp.AddTool(server, &mcp.Tool{Name: name, Description: description}, func(ctx context.Context, _ *mcp.CallToolRequest, input In) (*mcp.CallToolResult, any, error) {
		args, err := toMap(input)
		if err != nil {
			return nil, nil, fmt.Errorf("invalid tool input")
		}
		output, err := adapter.Invoke(ctx, operation, args)
		if err != nil {
			return nil, nil, sanitizeError(operation, err)
		}
		return nil, output, nil
	})
}

func toMap(input any) (map[string]any, error) {
	data, err := json.Marshal(input)
	if err != nil {
		return nil, err
	}
	var result map[string]any
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	if result == nil {
		result = map[string]any{}
	}
	return result, nil
}

func sanitizeError(operation string, err error) error {
	var runtimeErr *admruntime.RuntimeError
	if errors.As(err, &runtimeErr) {
		return errors.New(runtimeErr.Error())
	}
	var gitErr *admruntime.GitError
	if errors.As(err, &gitErr) {
		return errors.New(gitErr.Error())
	}
	var adapterErr *runtimeadapter.AdapterError
	if errors.As(err, &adapterErr) {
		return errors.New(adapterErr.Error())
	}
	return fmt.Errorf("runtime operation %q failed", operation)
}

func capabilitySet(values []string) map[string]bool {
	result := make(map[string]bool, len(values))
	for _, value := range values {
		result[value] = true
	}
	return result
}
