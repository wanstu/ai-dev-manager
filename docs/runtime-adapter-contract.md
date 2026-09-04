# Runtime Adapter Contract

## Purpose

`ai-dev-manager` owns workspace configuration, inheritance, policy and lifecycle. Runtime implementations execute capabilities. Protocol adapters such as MCP consume a protocol-neutral Runtime Adapter rather than depending on Native Runtime internals.

This boundary allows Native, CodexPro, DevSpace and future runtimes to coexist without moving configuration inheritance into those runtimes.

## Interface

The v0.1 contract is represented by `internal/adapter/runtimeadapter.Runtime`:

```go
type Runtime interface {
    ID() string
    WorkspaceID() string
    Capabilities() []string
    Status(context.Context) Status
    Invoke(context.Context, string, map[string]any) (any, error)
}
```

### Identity

- `ID()` identifies the runtime instance/adapter.
- `WorkspaceID()` identifies the workspace bound to that runtime.
- One hosted MCP HTTP instance is bound to exactly one Runtime Adapter.
- A caller cannot select another workspace by passing a workspace path/id to the MCP endpoint.

### Status

`Status` contains:

- runtime ID
- workspace ID
- state
- capabilities

It intentionally does not expose secrets, effective MCP environment values, tokens or full config objects.

### Capability vocabulary

Current Native capability names:

```text
files.tree
files.read
files.write
files.edit
search.text
shell.exec
git.status
git.diff
git.branch
git.worktree
verify.run
```

Capabilities describe what a runtime instance can currently expose after policy is applied. For example, a ReadOnly Native runtime does not advertise write/exec/Git capabilities.

Unknown external capabilities may appear in `runtime_info`, but the MCP adapter does not automatically turn arbitrary capability names into executable tools. A new host tool mapping must be explicitly designed and reviewed.

## Invoke operations

The Native Adapter currently understands:

```text
files.tree
files.read
files.write
files.edit
search.text
shell.exec
git.status
git.diff
git.branch
git.worktree.list
git.worktree.create
git.worktree.remove
verify.run
verify.run_many
```

`Invoke` receives JSON-compatible arguments and returns JSON-compatible structured output.

The Runtime implementation remains responsible for enforcing its own filesystem containment, command policy, tool resolution, worktree constraints and verifier execution rules. Protocol adapters must not reimplement or weaken those checks.

## Configuration ownership

Configuration inheritance always belongs to the ai-dev-manager Control Plane:

```text
Global
  ↓
Profile
  ↓
Project
  ↓
Runtime Override
  ↓
EffectiveConfig
  ↓
Runtime
```

An external Runtime Adapter receives the already-resolved configuration/capabilities it needs. It must not become a second source of truth for Global/Profile/Project precedence.

An external runtime may have its own native config format, but its adapter is responsible for mapping the effective ai-dev-manager intent into that runtime without changing the ai-dev-manager precedence model.

## MCP ownership

The MCP adapter lives outside Core and currently uses the official Go MCP SDK.

- stdio: local MCP clients/agents.
- Streamable HTTP: one stateless endpoint per hosted Workspace runtime.
- v0.1 HTTP manager listens on loopback only.
- HTTPS tunnel/reverse proxy/auth are deployment/control-plane concerns and are not automatically enabled by the Runtime Adapter.

## Native mapping

| Adapter operation | Native API |
| --- | --- |
| `files.tree` | `Native.Tree` |
| `files.read` | `Native.Read` |
| `files.write` | `Native.Write` |
| `files.edit` | `Native.Edit` |
| `search.text` | `Native.Search` |
| `shell.exec` | `Native.Exec` |
| `git.status` | `Native.GitStatus` |
| `git.diff` | `Native.GitDiff` |
| `git.branch` | `Native.GitBranch` |
| `git.worktree.*` | Native managed worktree APIs |
| `verify.*` | Native verifier APIs |

## CodexPro compatibility direction

A future CodexPro adapter should map existing CodexPro workspace-oriented primitives into the same contract rather than making CodexPro the Control Plane.

Expected mapping areas:

| ai-dev-manager | CodexPro-style capability |
| --- | --- |
| `files.tree` | tree/workspace inspection |
| `files.read` | bounded read |
| `files.write` / `files.edit` | workspace write/edit |
| `search.text` | repository search |
| `shell.exec` | bounded/allowlisted command execution |
| `git.diff` | change review/show changes |
| runtime status | workspace/runtime server status |

Important constraints:

- CodexPro remains independently usable.
- `codexprov4` is not modified by v0.1.
- ai-dev-manager Global/Profile/Project inheritance remains authoritative.
- An adapter should translate capability/status/errors; it should not require forking CodexPro.

## DevSpace compatibility direction

A future DevSpace adapter should similarly map its coding-runtime functions into this contract.

Expected mapping areas:

| ai-dev-manager | DevSpace-style capability |
| --- | --- |
| files/search/edit | workspace coding tools |
| `shell.exec` | bounded command execution |
| Git/worktree | workspace/worktree operations |
| skill exposure | remains Control Plane metadata unless a specific runtime bridge is required |
| runtime status | DevSpace server/workspace state |

Important constraints:

- DevSpace is an optional Runtime Provider, not the owner of ai-dev-manager configuration inheritance.
- Project-private MCP/Skill state remains resolvable before adapter invocation.
- Unknown DevSpace-specific features can be advertised as capabilities first; they are not automatically exposed as MCP tools until a stable mapping exists.

## External runtime minimum implementation

A third-party adapter only needs to implement the `Runtime` interface. It may be:

- an in-process runtime,
- an MCP client talking to another local server,
- a process bridge,
- a future Codex/Claude/OpenCode executor adapter.

The v0.1 host acceptance test includes a fake external runtime to prove that the HTTP lifecycle/MCP status path does not require `*runtime.Native`.

## Error contract

- Runtime/adapter errors must be safe for tool callers.
- MCP adapter converts ordinary runtime failures into MCP tool errors.
- Unknown external errors are sanitized to an operation-level failure message.
- Secrets and complete environment/config values must not be inserted into returned error strings.

## v0.1 non-goals

The contract does not yet provide:

- public tunnel lifecycle,
- OAuth/auth policy,
- Docker structured capabilities,
- debugger/DAP capabilities,
- Agent/Subagent orchestration,
- complete CodexPro or DevSpace adapters,
- remote multi-machine runtimes.

Those can extend capability vocabulary and adapter implementations without changing the Core configuration inheritance model.
