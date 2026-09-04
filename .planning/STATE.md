---
gsd_state_version: 1.0
milestone: v0.1
milestone_name: Core Foundation
status: complete
last_updated: "2026-09-04T21:00:00+08:00"
last_activity: 2026-09-04
progress:
  total_phases: 6
  completed_phases: 6
  total_plans: 6
  completed_plans: 6
  percent: 100
---

# Project State

## Current Position

Milestone: v0.1 — Core Foundation
Phase: 6 of 6 — MCP Host / External Runtime Adapter
Plan: 06-01 — MCP host + multi-instance runtime adapter vertical slice
Status: Completed
Last activity: 2026-09-04 — Phase 6 and v0.1 exit criteria verified; auto-advance stopped at milestone boundary

## Completed

### Phase 1 — Core Domain & EffectiveConfig

Validated:

- Pure Go Core with no UI/runtime coupling.
- Fixed `Global → Profile → Project → Runtime Override` precedence.
- MCP/Skill inherit/override/disable/re-enable semantics.
- entity source + Enabled source trace.
- structured resolver errors and immutable merge behavior.

### Phase 2 — Config Store & Workspace Registry

Validated:

- v1 JSON user/Profile/Project persistence.
- atomic save + malformed/unsupported config protection.
- Workspace Add/Get/List/Update/Remove with stable `ws_` IDs.
- Windows-semantic duplicate path prevention.
- two Workspace records survive reload with stable identity.
- ConfigService orchestrates persisted layers and runtime override without persisting runtime state.
- Project A/B configuration isolation.

### Phase 3 — MCP Catalog & Skill Discovery

Validated:

- optional `SkillRoots` and `MCPDefinition.EnvRefs` persisted without breaking old v1 JSON.
- Skill discovery scans explicitly configured roots only, including relative/home expansion and symlink-safe behavior.
- later discovery roots, explicit same-layer Skill, Project and Runtime precedence are deterministic.
- one Global GSD root resolves into multiple Workspaces without copying GSD per project.
- Project private Skills/MCPs do not leak into other Workspaces.
- MCP Catalog preserves Source/EnabledSource and uses `health=unprobed` until a real runtime exists.

### Phase 4 — Native Runtime & Security Policy

Validated:

- Native Runtime consumes Workspace + EffectiveConfig and snapshots policy/config state.
- capabilities for tree/read/search/write/edit/structured exec.
- `read-only`, `workspace-write`, `standard`, `full` enforced in execution code.
- relative/absolute cross-workspace escape rejected.
- Windows symlink escape test executed and passed (not skipped).
- `.git` and `.ai-dev-manager/runtime` direct writes blocked.
- structured `Executable + Args`, cwd containment, allowlist, timeout and output limits.
- explicit `ToolPaths` works independently of the MCP shell PATH.
- two Native runtimes keep independent roots/policies.

### Phase 5 — Git & Verification

Validated:

- structured Git status/diff/branch.
- managed Git worktree list/create/remove under app-owned sibling root.
- worktree creation does not switch or mutate the main checkout HEAD/branch.
- structured verifier configuration participates in four-layer resolution/persistence.
- test/lint/build/custom verifier runner supports pass/fail/skip/timeout structured results.
- real end-to-end temp repo completed `Edit → GitStatus/GitDiff → test verifier → build verifier`.

Acceptance:

```text
TestManagedGitWorktreeCreateListRemoveDoesNotSwitchMainCheckout → PASS
TestVerifierRunnerPassFailSkipTimeoutAndOrdering → PASS
TestModifyDiffAndVerifyEndToEnd → PASS
```

### Phase 6 — MCP Host & External Runtime Adapter

Validated:

- official `github.com/modelcontextprotocol/go-sdk` v1.7.0 is isolated to the MCP adapter boundary.
- SDK dependency raises project Go baseline from 1.22 to 1.25.0; host verification still uses Go 1.26.5.
- protocol-neutral `runtimeadapter.Runtime` contract supports identity, workspace, capabilities, status and generic invoke.
- NativeAdapter maps existing Native capabilities without moving policy/config inheritance into MCP.
- MCP server dynamically exposes only tools supported by the Runtime capabilities.
- `runtime_info` exposes safe identity/status/capabilities only.
- stdio runner exists via official `mcp.StdioTransport`.
- Streamable HTTP uses stateless handler and is bound to one Runtime/Workspace per instance.
- lifecycle Manager starts/stops deterministic independent HTTP instances and defaults to `127.0.0.1:0`.
- non-loopback listen is rejected in v0.1.
- official MCP in-memory client can list/call real Native read/edit tools.
- two official Streamable HTTP clients can connect to two isolated Workspace endpoints simultaneously.
- A cannot read B through its endpoint; stopping A does not affect B.
- fake non-Native external runtime is hosted through the same manager/runtime_info path.
- unknown external capabilities are visible but are not auto-converted into executable MCP tools.
- CodexPro/DevSpace compatibility boundary is documented in `docs/runtime-adapter-contract.md`; `codexprov4` remains untouched.

Acceptance:

```text
TestInMemoryMCPReadOnlyToolSurfaceAndRead → PASS
TestInMemoryMCPWorkspaceWriteCanEditButDoesNotExposeExec → PASS
TestManagerRunsTwoIsolatedWorkspaceMCPInstances → PASS
TestManagerRejectsNonLoopbackAndDuplicateInstanceID → PASS
TestManagerHostsExternalRuntimeContract → PASS
```

## Final v0.1 Verification

```text
D:/tools/go/go1.26.5/bin/go.exe test ./...
→ all packages PASS

D:/tools/go/go1.26.5/bin/go.exe vet ./...
→ PASS
```

MCP SDK scope search under `internal/` found imports only in:

```text
internal/adapter/mcpserver/server.go
internal/adapter/mcpserver/server_test.go
internal/host/manager_test.go
```

Core packages (`model/config/runtime/workspace/skill`) do not import the MCP SDK.

## Locked Direction

1. Independent project; do not modify or depend on `codexprov4` implementation details.
2. Do not fork CodexPro or DevSpace; future compatibility is adapter-based.
3. Control Plane and Runtime remain separate boundaries.
4. Runtime consumes EffectiveConfig; configuration inheritance remains in Control Plane.
5. Global GSD/Skill installation and project `.planning/` state remain separate.
6. Runtime Override remains memory-only.
7. Skill discovery scans explicit roots only.
8. Native security is enforced below MCP/tool descriptions.
9. Structured execution remains executable + argv; no implicit raw shell wrapper.
10. Git worktrees remain managed paths rather than arbitrary filesystem targets.
11. MCP Host consumes the protocol-neutral Runtime Adapter interface.
12. One HTTP MCP instance binds one Workspace Runtime.
13. v0.1 HTTP listen remains loopback-only.
14. Unknown external capabilities require explicit host-tool mappings before becoming executable MCP tools.
15. Agent/Subagent orchestration is deferred to v0.3 rather than represented by speculative empty interfaces.

## Environment Findings

- bare `go` is not visible in the current MCP bash PATH.
- host Go is available at `D:\tools\go\go1.26.5\bin\go.exe`.
- `ToolPaths` explicitly solves this mismatch for Runtime execution.
- official MCP Go SDK v1.7.0 changes the module `go` directive to 1.25.0.
- Windows atomic config replacement and symlink containment tests passed on the current host.

## Deferred Beyond v0.1

- final product name.
- desktop UI / tray integration.
- public HTTPS tunnel/reverse proxy/auth lifecycle.
- complete CodexPro adapter implementation.
- complete DevSpace adapter implementation.
- Docker structured API.
- Process manager / Debug / DAP.
- Agent / Subagent orchestration and deeper GSD runtime integration.
- multi-machine remote runtime.
- raw shell interpreter capability, if ever justified separately from structured exec.

## Next Action

v0.1 milestone is complete. `workflow.auto_advance=true` stops here because there is no approved next milestone plan. Before code continues, create/approve the v0.2 roadmap and requirements rather than silently expanding scope.
