---
gsd_state_version: 1.0
milestone: v0.6
milestone_name: Agent MCP Gateway
status: in_progress
last_updated: "2026-09-05T12:17:00+08:00"
last_activity: 2026-09-05
progress:
  total_phases: 4
  completed_phases: 3
  total_plans: 4
  completed_plans: 3
  percent: 75
---

# Project State

## Current Position

Milestone: v0.6 — Agent MCP Gateway
Phase: 25 — Writer-safe Mutation & Verification
Plan: 25-01 — writer-guarded mutation, execution, and verification
Status: In Progress — implementation and unattended gate complete; explicit fixed-path network acceptance pending
Last activity: 2026-09-05 — Phase 25 writer-safe mutation/exec/verify and unattended test split verified; ordinary go test no longer opens real TCP listeners

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

At the v0.1 boundary, MCP SDK imports were isolated to the MCP adapter/host test boundary. v0.2 intentionally adds official MCP SDK client usage in `internal/mcp/activation.go` and its tests for configured MCP probing. Core packages (`model/config/runtime/workspace/skill`) still do not import the MCP SDK.

## v0.2 Completed

### Phase 7 — Control Plane Composition & Introspection

Validated persisted Workspace ID → EffectiveConfig → Native Runtime → Runtime Adapter → MCP Host composition, deterministic safe introspection, unsupported runtime errors, and two-workspace isolation.

### Phase 8 — CLI & Foreground MCP Serve

Validated the real `ai-dev-manager` executable with workspace add/list/show, runtime inspect, foreground loopback MCP serve, official MCP client read, and graceful cancellation.

### Phase 9 — Configured MCP Activation & Health

Validated structured MCP argv inheritance/override/clear, real stdio and Streamable HTTP activation/probe, EnvRefs-at-launch without secret output, health/status, Global inheritance with Project disable isolation, and process-local stop behavior.

## Final v0.2 Verification

```text
go fmt ./...
→ PASS

go test ./...
→ all packages PASS

go vet ./...
→ PASS

go build -o NUL ./cmd/ai-dev-manager
→ PASS
```

Manual Windows acceptance also validated a built `ai-dev-manager.exe` for workspace add/list/inspect and foreground `serve`, resolving the workspace to `native` / `read-only` and binding a loopback `/mcp` endpoint.

## v0.3 Progress

### Phase 10 — Local Daemon & Control API ✅

Validated:

- config-root-scoped `runtime/daemon.json` discovery metadata and heartbeat owner lease.
- loopback-only local Control API with health/stop and client-side endpoint revalidation.
- daemon owns one long-lived `controlplane.Service` and cleans it with `StopAll` on shutdown.
- `start/status/stop` CLI works across independent processes; repeat start is idempotent to the same instance/PID.
- stale lease recovery is bounded by heartbeat freshness rather than trusting a PID file alone.

Acceptance:

```text
go fmt ./... → PASS
go test ./... → PASS
go vet ./... → PASS
go build ./cmd/ai-dev-manager → PASS

real ai-dev-manager-phase10.exe:
start  → daemon_03b25ea27a87132e20c6e7c9a7ccaaf9 pid=10224 running
status → same instance / same pid
start  → same instance / same pid
stop   → same instance stopped
status → stopped
```

### Phase 11 — Persistent Workspace Runtime Ownership ✅

Validated:

- daemon-owned `RuntimeOwner` uses the same long-lived `controlplane.Service` for Workspace MCP Host and configured MCP activation.
- `runtime start/status/stop/list` are local Control API operations; CLI does not locally rebuild a persistent runtime.
- one Workspace has one idempotent runtime Host; two Workspaces receive independent loopback endpoints.
- configured MCP session remains `healthy` across later status calls; activation failure rolls back the Workspace Host/session and records observed error state.
- stop A does not affect B; daemon shutdown closes remaining owned runtimes before final Control Plane cleanup.

Acceptance:

```text
go fmt ./... → PASS
go test ./... → PASS
go vet ./... → PASS
go build ./cmd/ai-dev-manager → PASS

real ai-dev-manager-phase11.exe:
daemon → daemon_dd47bee4bbbdc2c1a94a9527226dfaf7 pid=33064
A → ws_474118cbf1f0dd6535e797dd7e22ab8a endpoint 127.0.0.1:46190
B → ws_35a7c4d98c6a0eedeae14d12b8494256 endpoint 127.0.0.1:46192
repeat A → same endpoint
stop A → A stopped
status B → B still running
stop daemon → stopped
```

### Phase 12 — Restart Reconciliation & Lifecycle Acceptance ✅

Validated:

- persisted `runtime/desired-runtimes.json` stores only schema version + deterministic sorted Workspace IDs.
- explicit `runtime start` persists desired=true before observed startup; activation failure keeps desired=true for later retry.
- explicit `runtime stop` removes desired state before teardown; daemon shutdown preserves desired state while stopping only observed Host/session resources.
- daemon startup reconciles desired Workspaces through the same RuntimeOwner/Control Plane path and never restores old endpoint/session objects.
- corrupt desired state blocks healthy daemon publication rather than silently treating desired state as empty.
- direct daemon kill leaves stale discovery/lease, and a new `start` waits for heartbeat staleness, safely reclaims ownership, creates a new daemon identity and rebuilds the desired runtime.
- old runtime endpoints are unreachable after clean/crash shutdown and are never used as evidence of new health.

Acceptance:

```text
go fmt ./... → PASS
go test ./... → PASS
go vet ./... → PASS
go build ./cmd/ai-dev-manager → PASS

process-level tests:
TestCLIDaemonCleanRestartReconcilesDesiredRuntimes → PASS
TestCLIDaemonCrashRestartReclaimsStaleLeaseAndReconciles → PASS

real ai-dev-manager-phase12.exe clean restart:
first daemon  → daemon_73a5f6bf7ee45fc9e436a60f64981e1c pid=28996
first runtime → ws_e0ac652b045670ebde6d46b3a81b4504 endpoint 127.0.0.1:30924
stop daemon   → observed runtime stopped, desired preserved
second daemon → daemon_a11c0ef8fad9fa7990cd743a1adb8d76 pid=3156
reconciled    → same Workspace running at rebuilt endpoint 127.0.0.1:30927
```

## Final v0.3 Verification

```text
go fmt ./... → PASS
go test ./... → all packages PASS
go vet ./... → PASS
go build ./cmd/ai-dev-manager → PASS
```

v0.3 completes the persistent ownership foundation. Auto-advance stopped there until real-user usability acceptance resumed as v0.3.1.

## v0.3.1 — Usability Acceptance ✅

### Phase 13 — Docker-reachable Workspace MCP

Validated:

- default Workspace MCP path remains loopback-only; explicit exposed path is separate.
- `runtime start --docker` binds IPv4 wildcard intentionally and reports both Local and `host.docker.internal` endpoints.
- desired runtime schema v2 persists listen/exposure intent while reading legacy v1 desired state as loopback defaults.
- changing an already-running Workspace from loopback to Docker exposure rebuilds only that observed Host without requiring a manual stop first.
- daemon restart reconciles the persisted Docker exposure rather than reverting to loopback.
- exposed HTTP disables the SDK localhost-only check only on the explicit exposure path and replaces it with an ai-dev-manager Host/Origin allowlist; unexpected Host/Origin remains HTTP 403.
- MCP tool structured output is normalized to an object envelope (`{"result": ...}`), and the matching output schema uses explicit `{"result": {}}` instead of boolean JSON Schema `true`, preserving MCP data while remaining compatible with the tested MCP Hub/Zod stack.
- daemon Control API remains loopback-only.

Real Docker acceptance using the running `mcphub` container:

```text
Workspace: ws_8062e3c4b7ce9eeae7aa0f8bab45cf42
Docker MCP: http://host.docker.internal:33997/mcp
GET /mcp from mcphub → HTTP 405 + Allow: POST (handler reached)
POST initialize from mcphub → HTTP 200 text/event-stream
serverInfo.name → ai-dev-manager
protocolVersion → 2025-11-25
```

### Phase 14 — Daily CLI UX

Validated:

- `up [path|workspace-id]` defaults to current directory, auto-registers an unknown directory, starts/reuses daemon, and starts Runtime.
- `up --docker` emits directly usable Local and Docker MCP URLs; repeated `up` reuses the same Workspace ID.
- `down [path|workspace-id]` defaults to current directory and explicitly clears that Workspace desired-running state without stopping daemon.
- `ps` merges persisted Workspace/desired state with observed daemon runtime state and remains useful when daemon is stopped.
- `ctl start|status|stop|restart|shutdown` centralizes daemon management.
- `ctl stop`/`restart` preserve desired runtime; `ctl shutdown` clears desired runtimes so a later start does not resurrect them.
- original low-level lifecycle commands remain compatible.

Final verification:

```text
go fmt ./... → PASS
go test ./... → all packages PASS
go vet ./... → PASS
go build -o ai-dev-manager.exe ./cmd/ai-dev-manager → PASS
```

v0.3.1 makes the persistent runtime usable through the daily CLI while keeping advanced low-level commands available. Auto-advance stops at this milestone boundary before v0.4 Agents/GSD Runtime.

## v0.4 Progress

### Phase 15 — Agent Run Lifecycle ✅

Validated:

- daemon owns Agent Runs independently of the CLI invocation that created them.
- stable `run_` identity, Workspace identity, executor identity and running/completed/cancelled/error lifecycle state.
- loopback Control API provides run/list/status/cancel and is not exposed through Workspace MCP Docker/custom listen.
- `agent run [path|workspace-id]` reuses the daily Workspace resolution path, auto-registers unknown paths and ensures daemon running.
- `agent list/status/cancel` are thin Control API clients; cancel is idempotent and Workspace A/B Runs remain isolated.
- production Phase 15 executor is explicitly `lifecycle`; it blocks until cancellation and does not claim LLM/code execution.
- Agent Runs are observed state only. daemon shutdown cancels active Runs and restart starts with an empty Run registry rather than resurrecting old execution objects.

Acceptance:

```text
TestManagerLifecycleCancelAndIsolation → PASS
TestManagerExecutorCompletionAndError → PASS
TestManagerRejectsUnknownWorkspaceAndRun → PASS
TestCLIAgentRunLifecycleAcrossInvocationsAndRestart → PASS

go fmt ./... → PASS
go test ./... → all packages PASS
go vet ./... → PASS
go build -o ai-dev-manager-phase15.exe ./cmd/ai-dev-manager → PASS

real Windows binary, isolated config root:
agent run .      → run_71500556bf93fa429aebd9cb0f1f9595 running
agent list       → same run still running after creator CLI exited
agent status     → same run/workspace/executor
agent cancel     → cancelled + finished_at
ctl stop/start   → new daemon identity
agent list       → [] (old observed run not resurrected)
```

### Phase 16 — Planner / Executor / Reviewer ✅

Validated deterministic `verify` Planner → Executor → Reviewer orchestration with structured plan/steps/review, review-fail vs infrastructure-error separation, cancellation safety and real cross-process configured verifier execution.

### Phase 17 — GSD Phase Executor ✅

Validated `.planning` provenance resolution, allowlisted Execution Spec operations, real edit + verifier execution, forbidden-operation rejection, and pass-gated deterministic STATE advance with blocked/skip semantics when the next plan is unavailable or ambiguous.

### Phase 18 — Parallel Agents / Worktrees ✅

Validated observed-only derived runtimes rooted at managed Git worktrees, 2+ truly concurrent verify lanes, parent review aggregation, default cleanup / explicit preservation, distinct lane roots and unchanged main checkout HEAD/branch. The daemon clean-restart stop budget was also widened from 5s to 10s after full-suite Windows scheduling exposed a false timeout; the restart + parallel acceptance combination then passed 10 consecutive stress runs.

### Phase 19 — Persistent Environment Lifecycle ✅

Validated persistent `env_` identity/registry, real managed worktree + branch creation from current or explicit base, daemon-restart-safe inspect, missing-worktree diagnostics, isolated derived Runtime roots, and conservative branch-preserving destroy.

### Phase 20 — Dirty Change Transfer & Safe Destruction ✅

Validated `--include-changes` with staged/unstaged preservation, binary patches, untracked non-ignored transfer, failed-materialization diagnosis, upstream/unpushed safety, normal/force destroy semantics, and branch preservation across real Git + cross-process CLI tests.

### Phase 21 — Environment State / Writer Guard / Base Facts ✅

Validated structured ahead/behind/diverged/base-moved facts, dirty/upstream/activity/stale observations, restrained non-prescriptive hints, persistent single-writer lease with conflict/renew/release/force-release behavior, and writer survival across daemon restart. Full v0.5 fmt/test/vet/build gate passed.

### Phase 22 — Gateway Host & Discovery ✅

Validated an independent daemon-owned Agent MCP Gateway with persisted concrete loopback listen, stable endpoint across daemon restart, non-loopback rejection, no silent port migration on conflict, and typed `gateway_info/workspace_list/environment_list/environment_inspect` discovery through the official Streamable HTTP MCP client. Existing Direct MCP tests remain green.

## Locked Direction

1. Product positioning is **AI Coding Environment**, not a new built-in AI Agent.
2. Primary pain point is multi-task isolation: one Project can own multiple persistent Environment/worktree/branch contexts without dirty changes mixing together.
3. Tool reliability comes first, Agent MCP experience second, human CLI/Manager/UI experience third.
4. Control Plane and Runtime remain separate boundaries; Runtime consumes EffectiveConfig.
5. Environment worktrees remain managed paths rather than arbitrary filesystem targets.
6. Default Environment base is the creating checkout HEAD; explicit branch/tag/commit base is supported without hard-coding dev/master.
7. `include_changes=true` means staged + unstaged + untracked, excluding ignored files; tracked `.gitignore` remains normal repository content.
8. Environment integration is review-first: no automatic commit/merge/squash/cherry-pick/rebase decisions.
9. stale is informational only; no automatic Environment deletion. dirty/unpushed destructive cleanup requires explicit force.
10. One Environment defaults to one writer; parallel writing uses separate Environments.
11. ADM returns facts/warnings/hints and enforces policy, but development decisions belong to Agent/user.
12. Long-term Agent interface is one ADM MCP Gateway routing by `environment_id`, not one manually configured MCP per Environment.
13. Existing per-Workspace Runtime MCP remains a lower-level/compatibility capability until the Gateway milestone.
14. `codexpro-plus` stable product is not modified for ADM experiments; future Manager integration uses a fork/independent experiment first.
15. CodexPro/DevSpace remain design references and adapter targets; Core does not depend on them.
16. Native security remains enforced below MCP/tool descriptions; structured execution remains executable + argv.

## v0.3 Direction

1. Persistent local Control Plane ownership is the immediate priority; Agent/Subagent/GSD orchestration moves to v0.4.
2. Phase 10 establishes only daemon + local Control API + cross-process CLI lifecycle.
3. Phase 11 moves Workspace Runtime/MCP ownership under the daemon.
4. Phase 12 adds desired-state reconciliation and restart/crash acceptance.
5. v0.3 does not implement generic Process Manager, Docker, Debug/DAP, Desktop UI, remote access, or Agent orchestration.
6. Desired state may persist; observed runtime/session objects never persist and must be rebuilt after restart.

## Environment Findings

- bare `go` is not visible in the current MCP bash PATH.
- host Go is available at `D:\tools\go\go1.26.5\bin\go.exe`.
- `ToolPaths` explicitly solves this mismatch for Runtime execution.
- official MCP Go SDK v1.7.0 changes the module `go` directive to 1.25.0.
- Windows atomic config replacement and symlink containment tests passed on the current host.

## Deferred Beyond v0.3

- final product name.
- desktop UI / tray integration.
- public HTTPS tunnel/reverse proxy/auth lifecycle.
- complete CodexPro adapter implementation.
- complete DevSpace adapter implementation.
- Docker structured API.
- Process manager / Debug / DAP.
- Agent / Subagent orchestration and deeper GSD runtime integration (v0.4).
- generic Process Manager / development service lifecycle beyond the minimum daemon-owned child cleanup primitive.
- multi-machine remote runtime.
- raw shell interpreter capability, if ever justified separately from structured exec.

## Next Action

v0.5 is complete. Stop at this milestone boundary before v0.6 Agent MCP Gateway. Review and commit the full v0.5 change set first; the next milestone should expose one ADM MCP that routes Agent operations by `environment_id` rather than creating one MCP configuration per Environment.
