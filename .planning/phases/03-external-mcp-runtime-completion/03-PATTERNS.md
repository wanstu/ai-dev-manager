# Phase 03: External MCP Runtime Completion - Pattern Map

**Mapped:** 2026-09-07
**Files analyzed:** 12 (7 modified, 3 new, 2 extended tests)
**Analogs found:** 12 / 12

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|-------------------|------|-----------|----------------|---------------|
| `internal/model/types.go` | model | transform | `internal/model/types.go` (self) | exact — add fields to existing struct |
| `internal/catalog/service.go` | service | CRUD | `internal/catalog/service.go` (self) | exact — add method + config struct |
| `internal/catalog/service_test.go` | test | CRUD | `internal/catalog/service_test.go` (self) | exact — extend existing test |
| `internal/app/mcp_health.go` | service | request-response | `internal/verifier/service.go` | role-match — typed result + probe |
| `internal/app/mcp_health_test.go` | test | request-response | `internal/app/service_test.go` | role-match — service unit tests |
| `internal/app/service.go` | service | request-response | `internal/app/service.go` (self) | exact — add method to existing service |
| `internal/app/service_test.go` | test | request-response | `internal/app/service_test.go` (self) | exact — extend existing test |
| `internal/gateway/server.go` | handler | request-response | `internal/gateway/server.go` (self) | exact — enrich existing handlers |
| `internal/gateway/mcp_acceptance_test.go` | test | request-response | `internal/gateway/verifier_acceptance_test.go` | role-match — real HTTP acceptance test |
| `internal/management/service.go` | service | request-response | `internal/management/service.go` (self) | exact — thin delegate method |
| `internal/desktop/adapter.go` | handler | request-response | `internal/desktop/adapter.go` (self) | exact — thin adapter passthrough |
| `cmd/ai-dev-manager/main.go` | CLI | request-response | `cmd/ai-dev-manager/main.go:505-612` | exact — add subcommand to runCatalog |

## Pattern Assignments

### `internal/model/types.go` (model, transform) — MODIFY

**Analog:** `internal/model/types.go` lines 46-55 (CatalogEntry struct)

**Pattern — Add fields after existing `Endpoint` field:**
```go
// internal/model/types.go:46-55 — current CatalogEntry
type CatalogEntry struct {
	ID                  string   `json:"id"`
	Name                string   `json:"name"`
	DefaultIncludeInEnv bool     `json:"default_include_in_environment"`
	Endpoint            string   `json:"endpoint,omitempty"`
	// Phase 3 additions (lines 50-51):
	Transport           string            `json:"transport,omitempty"`        // NEW: "streamable-http" (default), future: "stdio"
	HeaderRefs          map[string]string `json:"header_refs,omitempty"`      // NEW: env-var-referenced auth headers
	// ... existing Skill-only fields unchanged below
	Instructions        string   `json:"instructions,omitempty"`
	ArtifactPath        string   `json:"artifact_path,omitempty"`
	SourceRoot          string   `json:"source_root,omitempty"`
	SupportRoots        []string `json:"support_roots,omitempty"`
}
```

**Key convention:** JSON tags with `omitempty`; empty `Transport` defaults to `"streamable-http"` for backward compatibility.

---

### `internal/catalog/service.go` (service, CRUD) — MODIFY

**Analog:** `internal/catalog/service.go` lines 35-45 (AddMCP) and lines 93-118 (add)

**Pattern — Add MCPConfig struct and AddMCPConfig method:**

The existing `AddMCP` signature (lines 35-45):
```go
func (s *Service) AddMCP(name, endpoint string, defaultInclude bool) (model.CatalogEntry, error) {
	if s.kind != KindMCP {
		return model.CatalogEntry{}, fmt.Errorf("catalog kind %q is not MCP", s.kind)
	}
	endpoint = strings.TrimSpace(endpoint)
	parsed, err := url.Parse(endpoint)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return model.CatalogEntry{}, fmt.Errorf("mcp endpoint must be a valid http(s) URL")
	}
	return s.add(name, endpoint, "", defaultInclude)
}
```

The internal `add` method (lines 93-118):
```go
func (s *Service) add(name, endpoint, instructions string, defaultInclude bool) (model.CatalogEntry, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return model.CatalogEntry{}, fmt.Errorf("%s name is required", s.kind)
	}
	var result model.CatalogEntry
	err := s.store.Update(func(state *model.State) error {
		items := s.items(state)
		for _, entry := range *items {
			if strings.EqualFold(entry.Name, name) {
				return fmt.Errorf("%s %q already exists", s.kind, name)
			}
		}
		id, err := identity.New(string(s.kind))
		if err != nil {
			return err
		}
		result = model.CatalogEntry{ID: id, Name: name, DefaultIncludeInEnv: defaultInclude, Endpoint: endpoint, Instructions: instructions}
		*items = append(*items, result)
		sort.Slice(*items, func(i, j int) bool {
			return strings.ToLower((*items)[i].Name) < strings.ToLower((*items)[j].Name)
		})
		return nil
	})
	return result, err
}
```

**New code to add — MCPConfig struct + AddMCPConfig + backward-compat wrapper:**
```go
// MCPConfig holds transport-specific configuration for an MCP catalog entry.
type MCPConfig struct {
	Endpoint       string
	Transport      string            // defaults to "streamable-http"
	HeaderRefs     map[string]string // env-var-referenced headers
	DefaultInclude bool
}

// AddMCPConfig creates a full MCP catalog entry with transport configuration.
// The existing AddMCP wraps this with defaults for backward compatibility.
func (s *Service) AddMCPConfig(name string, config MCPConfig) (model.CatalogEntry, error) {
	if s.kind != KindMCP {
		return model.CatalogEntry{}, fmt.Errorf("catalog kind %q is not MCP", s.kind)
	}
	endpoint := strings.TrimSpace(config.Endpoint)
	parsed, err := url.Parse(endpoint)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return model.CatalogEntry{}, fmt.Errorf("mcp endpoint must be a valid http(s) URL")
	}
	transport := strings.TrimSpace(config.Transport)
	if transport == "" {
		transport = "streamable-http"
	}
	// Copy header refs to avoid aliasing
	var headerRefs map[string]string
	if len(config.HeaderRefs) > 0 {
		headerRefs = make(map[string]string, len(config.HeaderRefs))
		for k, v := range config.HeaderRefs {
			headerRefs[k] = v
		}
	}
	return s.addWithTransport(name, endpoint, transport, headerRefs, "", config.DefaultInclude)
}

// AddMCP retains the existing signature for backward compatibility.
func (s *Service) AddMCP(name, endpoint string, defaultInclude bool) (model.CatalogEntry, error) {
	return s.AddMCPConfig(name, MCPConfig{Endpoint: endpoint, DefaultInclude: defaultInclude})
}

// addWithTransport is the internal add that sets Transport + HeaderRefs.
func (s *Service) addWithTransport(name, endpoint, transport string, headerRefs map[string]string, instructions string, defaultInclude bool) (model.CatalogEntry, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return model.CatalogEntry{}, fmt.Errorf("%s name is required", s.kind)
	}
	var result model.CatalogEntry
	err := s.store.Update(func(state *model.State) error {
		items := s.items(state)
		for _, entry := range *items {
			if strings.EqualFold(entry.Name, name) {
				return fmt.Errorf("%s %q already exists", s.kind, name)
			}
		}
		id, err := identity.New(string(s.kind))
		if err != nil {
			return err
		}
		result = model.CatalogEntry{
			ID:                  id,
			Name:                name,
			DefaultIncludeInEnv: defaultInclude,
			Endpoint:            endpoint,
			Transport:           transport,
			HeaderRefs:          headerRefs,
			Instructions:        instructions,
		}
		*items = append(*items, result)
		sort.Slice(*items, func(i, j int) bool {
			return strings.ToLower((*items)[i].Name) < strings.ToLower((*items)[j].Name)
		})
		return nil
	})
	return result, err
}
```

---

### `internal/app/mcp_health.go` (service, request-response) — NEW FILE

**Analog:** `internal/verifier/service.go` lines 31-41 (typed Result struct) and lines 141-171 (Classify function)

**Imports pattern** (follow `internal/app/service.go` lines 1-20):
```go
package app

import (
	"context"
	"fmt"
	"net"
	"os"
	"strings"
	"time"

	"ai-dev-manager-v2/internal/model"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)
```

**Typed result pattern** (follow `internal/verifier/service.go` lines 31-41):
```go
// MCPHealthState represents the four-state health model from MCP-RUN-03.
type MCPHealthState string

const (
	MCPHealthConfigured MCPHealthState = "configured"
	MCPHealthDisabled   MCPHealthState = "disabled"
	MCPHealthHealthy    MCPHealthState = "healthy"
	MCPHealthError      MCPHealthState = "error"
)

// MCPHealthStatus is the structured health probe result.
// Pattern follows verifier.Result: identity + status + bounded detail.
type MCPHealthStatus struct {
	MCPID     string         `json:"mcp_id"`
	State     MCPHealthState `json:"state"`
	ErrorKind string         `json:"error_kind,omitempty"`
	Message   string         `json:"message,omitempty"`
}
```

**Probe function pattern** (follow `internal/app/service.go` method pattern — receiver on `*Service`, takes `context.Context`, returns typed result + error):
```go
// ProbeMCPHealth performs an on-demand health probe for one MCP in the context
// of an Environment. This is the core MCP-RUN-03 implementation.
//
// Health states:
//   - disabled: MCP not enabled for this Environment
//   - configured: catalog entry exists but has not been probed / no endpoint
//   - healthy: connect + list-tools succeeded
//   - error: connect or list-tools failed (with structured error_kind)
func (s *Service) ProbeMCPHealth(ctx context.Context, environmentID, mcpID string) (MCPHealthStatus, error) {
	// Step 1: Check enablement (deterministic, no probe needed)
	env, err := s.Environments.Get(environmentID)
	if err != nil {
		return MCPHealthStatus{}, err
	}
	enabled := false
	for _, id := range env.EnabledMCPIDs {
		if id == mcpID {
			enabled = true
			break
		}
	}
	if !enabled {
		return MCPHealthStatus{
			MCPID: mcpID,
			State: MCPHealthDisabled,
		}, nil
	}

	// Step 2: Check catalog entry exists and has an endpoint
	entry, err := s.MCPs.Get(mcpID)
	if err != nil {
		return MCPHealthStatus{
			MCPID: mcpID,
			State: MCPHealthConfigured,
			Message: fmt.Sprintf("mcp catalog entry not found: %v", err),
		}, nil
	}
	endpoint := strings.TrimSpace(entry.Endpoint)
	if endpoint == "" {
		return MCPHealthStatus{
			MCPID: mcpID,
			State: MCPHealthConfigured,
			Message: "mcp has no configured endpoint",
		}, nil
	}

	// Step 3: Resolve secrets at activation boundary (D-03, MCP-RUN-05)
	resolvedEndpoint := resolveEndpoint(endpoint)
	resolvedHeaders := resolveHeaders(entry.HeaderRefs)

	// Step 4: Attempt connect + list-tools with bounded timeout (D-04)
	probeCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	_ = resolvedHeaders // Will be used when connectExternalMCP gains header support

	client := mcp.NewClient(&mcp.Implementation{Name: "adm-v2-health-probe", Version: "dev"}, nil)
	session, err := client.Connect(probeCtx, &mcp.StreamableClientTransport{
		Endpoint:            resolvedEndpoint,
		MaxRetries:          -1,
		DisableStandaloneSSE: true,
	}, nil)
	if err != nil {
		// Classify the error kind without exposing the resolved endpoint
		kind := classifyMCPError(err)
		return MCPHealthStatus{
			MCPID:     mcpID,
			State:     MCPHealthError,
			ErrorKind: kind,
			Message:   fmt.Sprintf("mcp health probe failed: %s", kind),
		}, nil
	}
	defer session.Close()

	// Step 5: ListTools as proof of full readiness
	listCtx, listCancel := context.WithTimeout(probeCtx, 10*time.Second)
	defer listCancel()
	_, err = session.ListTools(listCtx, nil)
	if err != nil {
		return MCPHealthStatus{
			MCPID:     mcpID,
			State:     MCPHealthError,
			ErrorKind: "tool_list_failed",
			Message:   "mcp connected but failed to list tools",
		}, nil
	}

	return MCPHealthStatus{
		MCPID: mcpID,
		State: MCPHealthHealthy,
	}, nil
}
```

**Secret resolution function pattern** (follow `os.ExpandEnv` usage — simple, no side effects):
```go
// resolveEndpoint expands environment variable references in an endpoint URL.
// Called ONLY at activation boundaries (MCP-RUN-05).
func resolveEndpoint(endpoint string) string {
	return os.ExpandEnv(endpoint)
}

// resolveHeaders expands environment variable references in header values.
func resolveHeaders(headerRefs map[string]string) map[string]string {
	if len(headerRefs) == 0 {
		return nil
	}
	resolved := make(map[string]string, len(headerRefs))
	for k, v := range headerRefs {
		resolved[k] = os.ExpandEnv(v)
	}
	return resolved
}
```

**Error classification pattern** (follow the project's pattern of `fmt.Errorf` with kind strings):
```go
// classifyMCPError maps raw connection errors to structured error kinds
// without exposing endpoint URLs or secret values.
func classifyMCPError(err error) string {
	if err == nil {
		return ""
	}
	errStr := err.Error()
	switch {
	case strings.Contains(errStr, "connection refused") || strings.Contains(errStr, "dial"):
		return "connection_refused"
	case strings.Contains(errStr, "timeout") || strings.Contains(errStr, "deadline"):
		return "timeout"
	case strings.Contains(errStr, "401") || strings.Contains(errStr, "403") || strings.Contains(errStr, "unauthorized"):
		return "auth_failure"
	default:
		return "connection_failed"
	}
}
```

---

### `internal/gateway/server.go` (handler, request-response) — MODIFY

**Analog:** `internal/gateway/server.go` lines 87-97 (existing input structs) and lines 358-400 (existing MCP handlers)

**Pattern — New input struct (follows lines 87-97):**
```go
// Add alongside existing EnvironmentMCPRuntimeInput (line 87-90):
type EnvironmentMCPStatusInput struct {
	EnvironmentID string `json:"environment_id"`
	MCPID         string `json:"mcp_id"`
}
```

**Pattern — New `environment_mcp_status` tool** (follows existing tool registration at lines 358-400):
```go
// Add after environment_mcp_call tool (after line 400):
mcp.AddTool(server, &mcp.Tool{Name: "environment_mcp_status", Description: "Probe and return the health status of one external MCP in the context of an Environment. Returns configured/disabled/healthy/error state."},
	func(ctx context.Context, _ *mcp.CallToolRequest, in EnvironmentMCPStatusInput) (*mcp.CallToolResult, any, error) {
		status, err := service.ProbeMCPHealth(ctx, in.EnvironmentID, in.MCPID)
		if err != nil {
			return toolResult(nil, err)
		}
		return nil, status, nil  // Follows environment_inspect pattern: structured output with outputSchema
	})
```

**Pattern — Enrich error handling in `environment_mcp_tools`** (enrich lines 364-366):
```go
// Current pattern (line 364-366):
session, err := connectExternalMCP(ctx, endpoint)
if err != nil {
	return toolResult(nil, err)
}

// Enriched pattern:
session, err := connectExternalMCP(ctx, endpoint)
if err != nil {
	return toolResult(nil, &MCPError{
		MCPID:     in.MCPID,
		ErrorKind: classifyConnectError(err),
		Message:   fmt.Sprintf("mcp %q: connection failed", in.MCPID),
	})
}
```

**Pattern — Enrich `connectExternalMCP`** (modify lines 571-578):
```go
// Current:
func connectExternalMCP(ctx context.Context, endpoint string) (*mcp.ClientSession, error) {
	client := mcp.NewClient(&mcp.Implementation{Name: serverName + "-proxy", Version: serverVersion}, nil)
	session, err := client.Connect(ctx, &mcp.StreamableClientTransport{Endpoint: endpoint, MaxRetries: -1, DisableStandaloneSSE: true}, nil)
	if err != nil {
		return nil, fmt.Errorf("connect external MCP %s: %w", endpoint, err)  // NOTE: leaks endpoint URL
	}
	return session, nil
}

// Enriched — do NOT include endpoint in error:
func connectExternalMCP(ctx context.Context, endpoint string) (*mcp.ClientSession, error) {
	client := mcp.NewClient(&mcp.Implementation{Name: serverName + "-proxy", Version: serverVersion}, nil)
	session, err := client.Connect(ctx, &mcp.StreamableClientTransport{Endpoint: endpoint, MaxRetries: -1, DisableStandaloneSSE: true}, nil)
	if err != nil {
		return nil, fmt.Errorf("connect external MCP: %w", err)  // Endpoint removed from error (D-03/MCP-RUN-05)
	}
	return session, nil
}
```

**Pattern — Use `app.MCPError` (canonical location: `internal/app/mcp_health.go`)**:
Gateway handlers import and use `app.MCPError` from the app package. The gateway must NOT define its own MCPError or classifyConnectError types. The `classifyMCPError` function lives in `internal/app/mcp_health.go`.
```go
// Gateway handlers return app.MCPError — imported from ai-dev-manager-v2/internal/app.
// The MCPError type and classifyMCPError function are defined in internal/app/mcp_health.go.
// Gateway does NOT define a local MCPError or classifyConnectError.
return toolResult(nil, &app.MCPError{
    MCPID:     in.MCPID,
    ErrorKind: app.ClassifyMCPError(err),  // or classifyMCPError if same package
    Message:   fmt.Sprintf("mcp %q: connection failed", in.MCPID),
})
```

---

### `internal/gateway/mcp_acceptance_test.go` (test, request-response) — NEW FILE

**Analog:** `internal/gateway/verifier_acceptance_test.go` lines 1-60 (real HTTP acceptance test pattern) and `internal/gateway/server_test.go:572-750` (MCP external test)

**Test setup pattern** (follow `server_test.go:572-581`):
```go
package gateway

import (
	"context"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ai-dev-manager-v2/internal/app"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Mock external MCP server pattern (from server_test.go:573-581):
func newMockExternalMCP(t *testing.T) *httptest.Server {
	t.Helper()
	external := mcp.NewServer(&mcp.Implementation{Name: "external-test", Version: "dev"}, nil)
	mcp.AddTool(external, &mcp.Tool{Name: "external_ping", Description: "Return a test pong."},
		func(context.Context, *mcp.CallToolRequest, EmptyInput) (*mcp.CallToolResult, any, error) {
			return nil, map[string]any{"pong": "external-pong"}, nil
		})
	server := httptest.NewServer(mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server {
		return external
	}, &mcp.StreamableHTTPOptions{Stateless: true, DisableLocalhostProtection: true}))
	t.Cleanup(server.Close)
	return server
}

// Test pattern (from verifier_acceptance_test.go):
func TestMCPHealthStatusEndToEnd(t *testing.T) {
	external := newMockExternalMCP(t)
	service := app.New(filepath.Join(t.TempDir(), "state.json"))
	root := t.TempDir()
	ws, err := service.Workspaces.Add(root, "mcp-health")
	if err != nil {
		t.Fatal(err)
	}
	mcpEntry, err := service.MCPs.AddMCP("external", external.URL, false)
	if err != nil {
		t.Fatal(err)
	}
	env, err := service.Environments.Create(ws.ID, "enabled", "")
	if err != nil {
		t.Fatal(err)
	}

	// Connect through in-memory transport (from server_test.go:1028-1042):
	ctx := context.Background()
	session := connectInMemory(t, ctx, New(service))
	defer session.Close()

	// Test configured state (not yet enabled):
	status, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "environment_mcp_status",
		Arguments: map[string]any{"environment_id": env.ID, "mcp_id": mcpEntry.ID},
	})
	// ... assertions for configured/disabled/healthy/error states

	// Test health gating in tools/call (structured error pattern):
	// ... connect → probe → expect structured error with error_kind
}
```

**Key patterns to replicate from `verifier_acceptance_test.go`:**
- Use `httptest.NewServer` with real MCP mock
- Use `connectInMemory` for in-process Gateway testing
- Assert structured output with `toolText(t, result)` helper
- Test all 4 health states (configured, disabled, healthy, error)
- Verify error messages do NOT contain resolved endpoint URLs

---

### `internal/catalog/service_test.go` (test, CRUD) — EXTEND

**Analog:** `internal/catalog/service_test.go` lines 14-57 (existing AddMCP test)

**Pattern — Add tests after existing test:**
```go
func TestAddMCPConfigSetsTransportAndHeaders(t *testing.T) {
	state := store.New(filepath.Join(t.TempDir(), "state.json"))
	mcps := catalog.New(state, catalog.KindMCP)

	entry, err := mcps.AddMCPConfig("custom-headers", catalog.MCPConfig{
		Endpoint:   "https://example.com/mcp",
		Transport:  "streamable-http",
		HeaderRefs: map[string]string{"Authorization": "Bearer ${MCP_TOKEN}"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if entry.Transport != "streamable-http" || entry.HeaderRefs["Authorization"] != "Bearer ${MCP_TOKEN}" {
		t.Fatalf("AddMCPConfig result = %+v", entry)
	}

	// Verify persistence:
	listed, err := mcps.List()
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, e := range listed {
		if e.ID == entry.ID {
			found = true
			if e.Transport != "streamable-http" || e.HeaderRefs["Authorization"] != "Bearer ${MCP_TOKEN}" {
				t.Fatalf("persisted MCP = %+v", e)
			}
		}
	}
	if !found {
		t.Fatalf("AddMCPConfig entry not found in list")
	}
}

func TestAddMCPDefaultsTransportToStreamableHTTP(t *testing.T) {
	state := store.New(filepath.Join(t.TempDir(), "state.json"))
	mcps := catalog.New(state, catalog.KindMCP)

	entry, err := mcps.AddMCP("default-transport", "http://127.0.0.1:9000/mcp", true)
	if err != nil {
		t.Fatal(err)
	}
	// Backward compat: AddMCP should set Transport to "streamable-http" by default
	if entry.Transport != "streamable-http" {
		t.Fatalf("AddMCP default transport = %q; want %q", entry.Transport, "streamable-http")
	}
}
```

---

### `internal/app/mcp_health_test.go` (test, request-response) — NEW FILE

**Analog:** `internal/app/service_test.go` lines 19-37 (service unit test pattern) and `internal/app/verifier_internal_test.go` (internal test pattern)

**Note:** This can be either `package app_test` (external, like `service_test.go`) or `package app` (internal, like `verifier_internal_test.go`). The probe function is on `*Service` so either works. Recommend `package app_test` for consistency with the main test file.

**Test pattern** (follow `service_test.go:19-37`):
```go
package app_test

import (
	"context"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ai-dev-manager-v2/internal/app"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestProbeMCPHealthConfigured(t *testing.T) {
	service := app.New(filepath.Join(t.TempDir(), "state.json"))
	root := t.TempDir()
	ws, err := service.Workspaces.Add(root, "health-test")
	if err != nil {
		t.Fatal(err)
	}
	mcpEntry, err := service.MCPs.AddMCP("test-mcp", "http://127.0.0.1:19999/mcp", false)
	if err != nil {
		t.Fatal(err)
	}
	env, err := service.Environments.Create(ws.ID, "test", "")
	if err != nil {
		t.Fatal(err)
	}
	// Enable MCP for environment
	if _, err := service.SetEnvironmentMCP(env.ID, mcpEntry.ID, true); err != nil {
		t.Fatal(err)
	}

	// Probe — should return "error" since endpoint is unreachable
	status, err := service.ProbeMCPHealth(context.Background(), env.ID, mcpEntry.ID)
	if err != nil {
		t.Fatalf("ProbeMCPHealth error: %v", err)
	}
	if status.State != app.MCPHealthError {
		t.Fatalf("state = %q; want %q", status.State, app.MCPHealthError)
	}
}

func TestProbeMCPHealthDisabled(t *testing.T) {
	service := app.New(filepath.Join(t.TempDir(), "state.json"))
	root := t.TempDir()
	ws, err := service.Workspaces.Add(root, "disabled-test")
	if err != nil {
		t.Fatal(err)
	}
	mcpEntry, err := service.MCPs.AddMCP("disabled-mcp", "http://127.0.0.1:19999/mcp", false)
	if err != nil {
		t.Fatal(err)
	}
	env, err := service.Environments.Create(ws.ID, "test", "")
	if err != nil {
		t.Fatal(err)
	}
	// DO NOT enable — MCP is disabled

	status, err := service.ProbeMCPHealth(context.Background(), env.ID, mcpEntry.ID)
	if err != nil {
		t.Fatalf("ProbeMCPHealth error: %v", err)
	}
	if status.State != app.MCPHealthDisabled {
		t.Fatalf("state = %q; want %q", status.State, app.MCPHealthDisabled)
	}
}

func TestResolveEndpointExpandsEnvVars(t *testing.T) {
	t.Setenv("MCP_SECRET", "my-secret-token")
	// This tests the internal resolveEndpoint function — may need to be internal test
	// or expose via a test helper. See verifier_internal_test.go pattern.
}

func TestSecretsNotExposedInErrorMessages(t *testing.T) {
	t.Setenv("MCP_SECRET", "my-secret-token")
	service := app.New(filepath.Join(t.TempDir(), "state.json"))
	root := t.TempDir()
	ws, err := service.Workspaces.Add(root, "secret-test")
	if err != nil {
		t.Fatal(err)
	}
	mcpEntry, err := service.MCPs.AddMCPConfig("secret-mcp", catalog.MCPConfig{
		Endpoint: "http://127.0.0.1:19999/mcp?token=${MCP_SECRET}",
	})
	if err != nil {
		t.Fatal(err)
	}
	env, err := service.Environments.Create(ws.ID, "test", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.SetEnvironmentMCP(env.ID, mcpEntry.ID, true); err != nil {
		t.Fatal(err)
	}

	status, err := service.ProbeMCPHealth(context.Background(), env.ID, mcpEntry.ID)
	if err != nil {
		t.Fatalf("ProbeMCPHealth error: %v", err)
	}
	// Error message must NOT contain the resolved secret
	if strings.Contains(status.Message, "my-secret-token") {
		t.Fatalf("health status leaked secret in message: %s", status.Message)
	}
}
```

---

### `internal/app/service.go` (service, request-response) — MODIFY

**Analog:** `internal/app/service.go` lines 98-130 (InspectEnvironment) and lines 417-427 (verifier passthrough methods)

**Pattern — No new methods needed** — the `ProbeMCPHealth` method lives in the new `mcp_health.go` file within the same `app` package, so it automatically becomes part of `Service`.

**However**, verify that the `Service` struct (lines 22-31) already has all needed dependencies:
```go
type Service struct {
	Store                   *store.Store
	Workspaces              *workspace.Service
	Environments            *environment.Service
	MCPs                    *catalog.Service    // ← already present, needed for ProbeMCPHealth
	Skills                  *catalog.Service
	Memory                  *memory.Service
	Verifiers               *verifier.Service
	writerHeartbeatInterval func(time.Duration) time.Duration
}
```
`MCPs` and `Environments` are already fields — `ProbeMCPHealth` can access them directly.

---

### `internal/management/service.go` (service, request-response) — MODIFY

**Analog:** `internal/management/service.go` lines 100-110 (existing MCP delegate methods)

**Pattern — Thin delegate** (follows lines 100-110):
```go
// Add after MCPSetDefault (line 110):
func (s *Service) MCPHealth(ctx context.Context, environmentID, mcpID string) (app.MCPHealthStatus, error) {
	return s.app.ProbeMCPHealth(ctx, environmentID, mcpID)
}
```

---

### `internal/desktop/adapter.go` (handler, request-response) — MODIFY

**Analog:** `internal/desktop/adapter.go` lines 162-174 (existing MCP adapter methods)

**Pattern — Thin adapter passthrough** (follows lines 162-174):
```go
// Add after RemoveMCP (line 181):
func (a *Adapter) ProbeMCPHealth(environmentID, mcpID string) (app.MCPHealthStatus, error) {
	if err := a.ready(); err != nil {
		return app.MCPHealthStatus{}, err
	}
	return a.management.MCPHealth(context.Background(), environmentID, mcpID)
}
```

---

### `cmd/ai-dev-manager/main.go` (CLI, request-response) — MODIFY

**Analog:** `cmd/ai-dev-manager/main.go:505-612` (runCatalog function)

**Pattern — Add `status` subcommand inside runCatalog** (follows the `case "list":` pattern at line 561):

Insert after the `"set-default"` case (before `default:` at line 609):

```go
case "status":
	if kind != "mcp" {
		return fmt.Errorf("%s status is not supported", kind)
	}
	fs := newFlagSet("mcp status", func() {
		fmt.Fprintf(os.Stdout, "用法：ai-dev-manager-v2 mcp status --id MCP_ID --environment-id ENV_ID\n")
		fmt.Fprintln(os.Stdout, "\nProbe and display health status for one MCP in an Environment context.")
	})
	id := fs.String("id", "", "MCP ID")
	envID := fs.String("environment-id", "", "Environment ID")
	if err := fs.Parse(args[1:]); err != nil {
		return flagError(err)
	}
	mcpID := strings.TrimSpace(*id)
	environmentID := strings.TrimSpace(*envID)
	if fs.NArg() != 0 || mcpID == "" || environmentID == "" {
		return fmt.Errorf("必须提供 --id 和 --environment-id；运行 ai-dev-manager-v2 mcp status -h 查看帮助")
	}
	status, err := service.ProbeMCPHealth(context.Background(), environmentID, mcpID)
	if err != nil {
		return err
	}
	return writeJSON(status)
```

**Note:** The `status` command calls `service.ProbeMCPHealth` directly (through `app.Service`), bypassing the management layer, since the CLI has direct access to the app service. This follows the pattern used by other CLI commands that call `service.MCPs.AddMCP(...)` directly (line 532).

---

## Shared Patterns

### Authentication / Secret Handling
**Source:** New `resolveEndpoint` / `resolveHeaders` in `internal/app/mcp_health.go`
**Apply to:** All MCP connection boundaries
```go
// Secrets resolved ONLY at activation boundary (connectExternalMCP / ProbeMCPHealth).
// Never returned in status, inspect, error, or log output.
resolvedEndpoint := os.ExpandEnv(entry.Endpoint)
resolvedHeaders := resolveHeaders(entry.HeaderRefs)
```

### Error Handling
**Source:** Canonical `MCPError` type in `internal/app/mcp_health.go` (per D-07 — structured MCPError with error_kind, no secret leakage)
**Apply to:** `environment_mcp_tools`, `environment_mcp_call`, `environment_mcp_status` handlers in `internal/gateway/server.go`
```go
// MCPError is defined once in internal/app/mcp_health.go.
// Gateway handlers import and use app.MCPError — NOT a gateway-local type.
// Structured error with identity + kind, no secret leakage.
// Errors carry enough identity for diagnosis without exposing token values.
```

### Typed Result Struct
**Source:** `internal/verifier/service.go` lines 31-41 (verifier.Result)
**Apply to:** `internal/app/mcp_health.go` (MCPHealthStatus)
```go
// Typed result pattern: identity (MCPID) + status (State) + bounded detail (ErrorKind, Message).
// JSON-tagged for direct Gateway tool output.
type MCPHealthStatus struct {
	MCPID     string         `json:"mcp_id"`
	State     MCPHealthState `json:"state"`
	ErrorKind string         `json:"error_kind,omitempty"`
	Message   string         `json:"message,omitempty"`
}
```

### Gateway Tool Registration
**Source:** `internal/gateway/server.go` lines 196-550 (mcp.AddTool pattern)
**Apply to:** New `environment_mcp_status` tool
```go
mcp.AddTool(server, &mcp.Tool{
	Name:        "environment_mcp_status",
	Description: "Probe and return the health status of one external MCP.",
},
	func(ctx context.Context, _ *mcp.CallToolRequest, in InputType) (*mcp.CallToolResult, any, error) {
		result, err := service.SomeMethod(ctx, in.Args...)
		return toolResult(result, err)
	})
```

### Test Helper Pattern
**Source:** `internal/gateway/server_test.go:1028-1042` (connectInMemory)
**Apply to:** All gateway acceptance tests
```go
func connectInMemory(t *testing.T, ctx context.Context, server *mcp.Server) *mcp.ClientSession {
	t.Helper()
	clientTransport, serverTransport := mcp.NewInMemoryTransports()
	serverSession, err := server.Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatalf("server.Connect: %v", err)
	}
	t.Cleanup(func() { _ = serverSession.Close() })
	client := mcp.NewClient(&mcp.Implementation{Name: "adm-v2-test", Version: "dev"}, nil)
	clientSession, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("client.Connect: %v", err)
	}
	return clientSession
}
```

### Management Delegation Pattern
**Source:** `internal/management/service.go` lines 100-110
**Apply to:** New `MCPHealth` method
```go
func (s *Service) SomeMethod(args...) (ReturnType, error) {
	return s.app.SomeMethod(args...)
}
```

### Desktop Adapter Pattern
**Source:** `internal/desktop/adapter.go` lines 162-174
**Apply to:** New `ProbeMCPHealth` method
```go
func (a *Adapter) SomeMethod(args...) (ReturnType, error) {
	if err := a.ready(); err != nil {
		return ZeroValue, err
	}
	return a.management.SomeMethod(args...)
}
```

## No Analog Found

Files with no close match in the codebase (planner should use RESEARCH.md patterns instead):

| File | Role | Data Flow | Reason |
|------|------|-----------|--------|
| None | — | — | All Phase 3 files have close analogs in existing code |

## Metadata

**Analog search scope:** `internal/`, `cmd/`
**Files scanned:** 12
**Pattern extraction date:** 2026-09-07
