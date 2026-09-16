# ADM V2 Product Contract

Status: development baseline. Breaking changes are allowed until the user explicitly declares a compatibility boundary.

This file defines product semantics. Implementation plans and phases may refine how requirements are delivered, but must not silently change these requirements.

## Product goal

### ADM-GOAL-001 — Let an Agent develop software on this computer through MCP

ADM V2 is a local development MCP server. Its primary purpose is to let an Agent perform real software-development work against projects on this computer.

A usable ADM must let an Agent, through MCP, select/open a local project, inspect its files, search code, create/edit/delete files, execute permitted development commands, run tests/build/lint when available, inspect the results, and continue iterating until a real development task is complete.

Workspace, Environment, Memory, Skill, external MCP integration, Git integration, isolation, concurrency control, and other mechanisms exist only to support this goal. None of those abstractions may become an unrelated prerequisite that prevents ordinary local development.

The primary product test is therefore an end-to-end Agent development loop, not the existence of management abstractions.

### ADM-GOAL-002 — ADM supplies development capabilities; the Agent supplies task orchestration

ADM owns safe local capability execution, lifecycle, authorization, routing and diagnostics. It does not decide what task an Agent should do, decompose a task into Planner/Executor/Reviewer roles, interpret natural-language plans into commands, advance GSD `.planning` state, select a next phase, or make automatic Git integration decisions.

An external Agent, GSD, or another orchestrator may use ADM files, exec, verifier, MCP, Skill, process, Git/worktree and generic asynchronous Runtime capabilities to implement its own workflow. ADM must not duplicate that orchestration policy inside Core.

## Core model

### ADM-CORE-001 — Workspace is a directory

A Workspace is a development directory explicitly added to ADM and identified by an ADM ID and a filesystem path.

Agents may operate only inside directories that have been added to ADM as Workspaces. ADM must not expose arbitrary filesystem access outside registered Workspace roots.

Acceptance:

- register an empty ordinary directory
- register a directory that is not a Git repository
- inspect/list it successfully

### ADM-CORE-002 — Environment is a development context

An Environment is a development context associated with one Workspace and rooted at a directory.

Minimum semantic fields:

- Environment ID
- Workspace ID
- human-readable name
- root path
- state/activity metadata
- writer lease state
- enabled global MCP IDs
- enabled global Skill IDs
- Environment-private memory

Environment is both a development root and a private AI development context. MCPs and Skills are defined once in the global catalog; an Environment stores only which global MCPs and Skills are enabled for that Environment. Environment-private memory belongs to that Environment and must not leak into other Environments.

Environment is not defined by Git branch, commit, worktree, Docker container, clone, or any other isolation mechanism.

Acceptance:

- create an Environment for a non-Git Workspace
- root defaults to the Workspace directory
- inspect/list survives daemon restart
- Environment MCP/Skill enable selections survive daemon restart
- enabling or disabling an MCP/Skill for one Environment does not affect another Environment or the global catalog
- Environment-private memory is not visible from another Environment by default

### ADM-CORE-003 — Tools are optional capabilities

Git, worktree, Docker, shell execution, verifier, MCP, language toolchains, and future integrations are optional capabilities.

Missing capability blocks only operations that require it.

Acceptance examples:

- no Git: read/search/write still work
- no verifier: Environment creation and file operations still work
- no shell execution: read/search still work
- no worktree: normal Environment development still works

### ADM-CORE-004 — Capability checks are operation-local

Each routed operation checks only the capability needed for that operation.

No workflow or Environment lifecycle may require unrelated capabilities as a global admission check.

### ADM-CORE-005 — Single writer per physical root

ADM must prevent concurrent writers from mutating the same physical development root through multiple Environments.

Read-only operations do not require the writer lease.

Mutation operations require the active writer owner.

A writer is a bounded lease, not a permanent lock. The lease has an explicit expiry time, successful owner activity renews it, and an expired lease must automatically stop blocking the physical root. A persisted writer without an explicit expiry is invalid and must be reacquired rather than inferred through compatibility rules. Force release remains a recovery fallback, not the normal lifecycle.

### ADM-CORE-006 — Core file development operations

The first usable V2 core must support, subject only to the relevant runtime policy:

- tree/list files
- read text files
- literal search
- write files
- exact edit
- delete a file

These operations must work in a plain non-Git directory.

### ADM-CORE-007 — Local command execution is allowlisted

Agents may execute local development commands only when the executable is explicitly allowed by ADM's command whitelist.

Command execution must run within the selected registered Workspace/Environment root and must not provide arbitrary command execution outside that scope.

Commands not permitted by the whitelist must be rejected clearly.

The allowlist is mutable management state. ADM must support removing a previously allowed executable, and that removal must affect subsequent capability checks and command execution without requiring an ADM restart. Removing a value that is not currently allowlisted must fail clearly rather than report a false success.

Acceptance:

- allow an executable and observe `shell.exec` when the Runtime otherwise supports execution
- remove the executable and observe the updated allowlist immediately
- after removing the last allowed executable, `shell.exec` is no longer advertised
- execution of the removed executable is rejected
- removing a non-allowlisted executable returns a clear error

### ADM-CORE-008 — Verifier is optional

Verifier execution is a Runtime capability.

A project without configured verifiers remains a valid development Workspace.

### ADM-CORE-009 — Git is a tool

Git features are optional tool operations.

Potential Git operations include status, diff, branch and later worktree support.

Git must not be part of the intrinsic Workspace or Environment data model.

### ADM-CORE-010 — Isolation is optional

Isolation is an optional capability around Environment and must not become part of the intrinsic Workspace or Environment definition.

An isolation implementation may create/select another root directory and then create an Environment rooted there. Git worktree is the first implemented isolation mechanism, but ordinary non-Git and in-Workspace Environments remain valid without it.

Managed Git worktrees:

- are created only for a registered Workspace whose root is the Git top-level;
- use an ADM-generated branch and an ADM-owned destination under the ADM state directory; callers do not supply arbitrary destination paths or branch names;
- create a normal Environment context rooted at that managed worktree without changing the source checkout branch, HEAD, or files;
- persist separate managed-worktree metadata and revalidate root, Git common-dir, top-level, and branch identity before routed Runtime access;
- require the matching Environment writer for destroy;
- refuse dirty or locally advanced work by default and require explicit force to remove such a worktree;
- retain the managed branch after destroy so committed work is never silently deleted with the worktree directory.

A missing, moved, replaced, or tampered managed worktree must fail locally before routed mutation. Generic Environment removal must not bypass the managed-worktree destroy safety policy.

Other future isolation implementations may include clone, copy, container, VM, or another mechanism. None is privileged in the core Environment model.

Acceptance:

- ordinary non-Git Environment create/read/write remains green when worktree isolation is unavailable;
- two managed worktree Environments from one Git Workspace have distinct roots and branches;
- creating or mutating a managed worktree does not switch or modify the source checkout;
- mutation in one managed root is not visible in another managed root or the source checkout;
- missing/tampered managed worktree identity is rejected before routed Runtime mutation;
- clean destroy removes the managed worktree root but retains its branch;
- dirty or unpublished committed work blocks default destroy; explicit force may remove the worktree root but still retains the branch.

### ADM-CORE-011 — No pre-stable compatibility layer

During active V2 development, old development data may be discarded when a model changes.

Do not add migrations, legacy inference, compatibility fields, or fallback behavior unless the user explicitly introduces a compatibility requirement.

### ADM-CORE-012 — Global MCP/Skill catalog with Environment selection

All MCP and Skill definitions are managed in one global catalog boundary, with type-specific definition models. An MCP definition contains the transport-specific connection information needed for ADM to use that MCP; supported Phase 11 transports are Streamable HTTP and local stdio/command under ADM executable authority. A Skill definition resolves to a real Skill artifact discovered from an explicitly configured root. The initial artifact contract is `SKILL.md`; explicitly configured support roots may authorize supporting files referenced by that Skill without granting arbitrary host-filesystem access.

Each global MCP and Skill definition includes a default-include-in-environment setting that controls whether newly created Environments enable that entry by default.

An Environment does not own duplicate/private MCP or Skill definitions. It stores only the IDs of global MCPs and Skills enabled for that Environment.

When a new Environment is created, its initial enabled MCP/Skill selections are copied from the current global default-include settings. Later changes to a global default must not silently rewrite existing Environment selections.

An Environment may then enable or disable any subset of the global catalog. Entries not selected for that Environment are not exposed in that Environment. MCP runtime access must enforce that selection at call time. Skill discovery/read access must likewise enforce Environment selection and must resolve to the configured real artifact/support roots rather than only the catalog name/ID.

Changing an Environment's enabled selections must not modify the global MCP/Skill definitions, their global defaults, or another Environment's selections.

If a globally defined MCP or Skill is removed, affected Environment selections may become unresolved references; handling of unresolved selections must be explicit and must not silently recreate or copy definitions.

MCP definitions additionally carry an explicit health/recovery policy. Enabled MCPs may be periodically checked through the MCP protocol by the persistent Gateway owner with user-configurable check interval and bounded probe timeout. Automatic reconnect defaults to disabled. When explicitly enabled, unhealthy MCPs reconnect using the configured fixed interval; Phase 11 does not add exponential/adaptive backoff. Health timestamps, failure counters, next-reconnect facts, sessions and tool inventories are observed owner-local state and are not persisted as desired state. Background recovery may reconnect and refresh safe inventory, but must never automatically replay a failed MCP tool call.

ADM may import MCP definitions from external JSON/JSONC configuration formats through source adapters. Import is a preview/apply normalization boundary: OpenCode, WorkBuddy/CodeBuddy, Codex plugin MCP JSON, Claude Code and supported MCPHub source shapes normalize into ADM's canonical MCP definition model. Single and selected-batch import use the same validation path; selected-batch apply is atomic and name conflicts default to error. Phase 11 import writes only global MCP definitions and never changes existing Environment enable selections. Literal credential-bearing source values are converted into generated secret/environment-reference requirements; ADM persists only the references, not the literal credentials, and the caller provisions those references before activation. Import must not persist raw import blobs. Ambiguous automatic format detection must require explicit source format selection.

Acceptance:

- a newly managed MCP definition contains valid transport-specific runtime configuration rather than only a display name
- an MCP not enabled for an Environment cannot be listed or called through that Environment
- enabling a configured MCP makes its tools discoverable/callable through the ADM Gateway; disabling it revokes that access immediately and cancels owned health/reconnect work
- a configured health policy drives bounded protocol health checks; automatic reconnect is disabled by default and, when explicitly enabled, uses the configured fixed retry interval without replaying failed tool calls
- single and batch MCP JSON/JSONC import support preview + atomic apply for the explicitly supported OpenCode, WorkBuddy/CodeBuddy, Codex plugin, Claude Code and MCPHub source adapters; name conflicts default to error, import writes only the global MCP catalog, and literal credentials are converted to reference requirements rather than persisted
- a configured Skill is discovered from an explicit root and records a real `SKILL.md` artifact/source rather than only a display name or copied instruction string
- enabled Skill artifacts and explicitly authorized supporting files can be read through the Environment-scoped Agent Gateway
- the same global Skill installation can be enabled by multiple Environments without copying it into each project
- disabled Skills and paths outside configured artifact/support roots are rejected locally
- legacy/unconfigured metadata-only entries are reported explicitly and are not presented as working integrations

### ADM-CORE-013 — Global memory and Environment-private memory

ADM owns one global memory scope plus one private memory scope per Environment.

Global memory:

- persists independently from transient Agent sessions
- is available across Environments as shared durable context
- is not tied to any Workspace or Environment lifecycle
- is not deleted when an Environment is destroyed

Environment-private memory:

- persists independently from transient Agent sessions
- is available when working inside that Environment
- is isolated from other Environments by default
- is not promoted to global memory implicitly
- can be deleted with the Environment without deleting project files

When working inside an Environment, ADM may expose both Global Memory and that Environment's private Memory. Writes must target one scope explicitly; ADM must not silently copy, promote, merge, or synchronize memory between scopes.

### ADM-CORE-014 — Workspace lifecycle management is metadata-safe

ADM must support inspecting a Workspace by stable ID, changing its human-readable name, and removing its registration.

Renaming a Workspace changes only ADM metadata. It must not move or rename the Workspace directory.

Removing a Workspace removes only the ADM registration. It must not delete the Workspace directory or any project files. A Workspace that is still referenced by any Environment must not be removed; the Environment must be removed first.

Acceptance:

- inspect a registered Workspace by ID
- rename it while preserving its ID and filesystem path
- reject an empty replacement name
- reject removal while any Environment references the Workspace
- after Environment removal, remove the Workspace registration successfully
- Workspace removal leaves the directory and project files untouched

### ADM-CORE-015 — Human management surfaces expose persisted development context

ADM management state must not be usable only through the Agent-facing MCP Gateway. Human-facing management surfaces must expose the same persisted MCP catalog, Skill catalog, Memory scopes, and Environment selections needed to understand and administer the current development context.

The first human management surface is the CLI. It must expose management operations without changing the underlying product model or creating a second persistence path.

Acceptance:

- CLI can add/list/remove global MCP catalog entries and change their default-include setting
- CLI can add/list/remove global Skill catalog entries and change their default-include setting
- CLI can list/read/write/delete Global Memory
- CLI can enable/disable MCP and Skill IDs for one Environment without changing another Environment
- CLI can list/read/write/delete Environment-private Memory by explicit Environment ID
- CLI help makes these capabilities discoverable
- all operations reuse the same persisted state and application services used by MCP

### ADM-CORE-016 — Environment lifecycle management is metadata-safe

ADM must support changing an Environment's human-readable name by stable Environment ID.

Renaming an Environment changes only ADM metadata. It must not move or rename the Environment root directory, change the Workspace association, rewrite MCP/Skill selections, rewrite private memory, release an active writer, or touch project files.

Acceptance:

- rename an Environment by stable ID
- preserve Environment ID, Workspace ID, root path, creation time, MCP selections, Skill selections, private memory, and writer state
- reject an empty replacement name
- leave project directories and files untouched
- expose rename through both CLI and MCP management surfaces

### ADM-CORE-017 — Environment management views are informative without leaking private Memory

Human and Agent management views must make an Environment understandable without requiring callers to manually join Workspace and catalog state, while keeping private Memory values behind explicit Memory operations.

`environment list` must remain a lightweight summary. It may expose the Environment identity, root, writer state, selection IDs, activity metadata, and the count of private Memory entries, but it must not dump private Memory values.

`environment inspect` must additionally expose the referenced Workspace, current Runtime capabilities, resolved selected MCP/Skill catalog entries, unresolved selected MCP/Skill IDs, and the private Memory entry count. It must not dump private Memory values.

Acceptance:

- list exposes `private_memory_count` without private Memory values
- inspect exposes Workspace metadata and current capabilities
- inspect exposes resolved MCP/Skill names alongside stored selection IDs
- inspect keeps removed catalog selections visible as unresolved IDs
- private Memory values remain readable only through explicit Environment-private Memory operations
- CLI and MCP use the same application-level management view

### ADM-CORE-018 — External MCP is a first-class diagnosable Runtime capability

An MCP definition is desired configuration, not observed health. ADM must keep configured transport/auth/secret references separate from live session/health/tool inventory state. Every transport advertised as supported must have a real activation path; an unsupported or broken MCP must fail locally without affecting files, exec, Skill, verifier or other MCPs.

MCP secrets resolve only at activation boundaries and must not be exposed through normal status, logs, tool inventory or error output. Environment selection remains the authorization gate for activation and calls. Disabling an MCP revokes access immediately and owned live sessions must not remain reported healthy.

The Agent must be able to inspect/refresh the actual remote tool inventory and receive structured diagnostics for configuration, transport, authentication, initialization, discovery, call and reconnect failures.

#### Global management probes versus Environment authorization

An explicit administrative `mcp_probe` checks a global MCP definition without selecting or creating an Environment. Configuration/reference resolution and connection health are independent of Environment enablement. A successful probe never enables that MCP or grants an Agent tool access. `environment_mcp_*` operations continue to enforce Environment selection at call time.

The probe performs a bounded transient protocol connection and tool inventory request, then closes it; it does not invoke business tools, schedule reconnects, or persist health. Stdio obeys the global executable allowlist and uses an ADM-created temporary working directory removed after the session closes. Project-relative paths are not inferred from an Environment. Secret-bearing activation values and process output are not returned as diagnostics. Desktop results are scoped to the connected ADM and unchanged definition; Environment changes do not invalidate them, but definition or connection changes do.

### ADM-CORE-019 — Skill is a first-class discoverable and diagnosable Agent context capability

A Skill resolves to a real `SKILL.md` artifact plus only explicitly authorized supporting files. ADM may discover/refresh Skill artifacts from configured roots, report source/support facts, gate access by Environment selection and explain disabled/missing/broken states.

A broken Skill or support reference must not break unrelated Skills or ordinary Environment development. ADM does not interpret a Skill into an internal task workflow and does not execute Skill reasoning policy; the consuming Agent reads and follows the Skill.

### ADM-CORE-020 — Environment capability availability is inspectable

ADM must provide an authoritative Environment capability view that distinguishes usable capabilities from unavailable ones and explains the reason without leaking secrets or private Memory values. Optional-capability failures remain operation-local.

Examples of useful unavailable reasons include disabled, unconfigured, unsupported transport, missing executable, broken Skill artifact, authentication error, connection error and managed-root validation failure.

### ADM-CORE-021 — Long verifier execution has an owner-local async lifecycle

Heavy Environment verifier/test/build execution may outlive one short MCP request. ADM must provide an asynchronous verifier-run lifecycle owned by the persistent Gateway so a launching client can disconnect and a later client on the same owner can observe or cancel the same verification by stable ADM identity.

An asynchronous verifier run is distinct from both the persisted verifier definition and the generic single-command `run_` resource. It must preserve verifier semantics rather than reducing verification to opaque command execution.

The async verifier lifecycle must:

- start only from an existing enabled Environment verifier definition;
- require the active matching Environment writer for start;
- reuse the same Runtime executable allowlist, Environment-contained cwd, managed-root validation, verifier timeout and `verifier.Classify` semantics as synchronous verifier execution;
- return a stable owner-local verifier-run identity and lifecycle state;
- expose bounded live stdout/stderr snapshots with explicit truncation evidence while running;
- retain the terminal structured verifier result while the same Gateway owner remains alive;
- allow list/status as read-only Environment-scoped observations without writer acquisition;
- allow cancel only with the matching current writer and the writer owner that launched the verifier run;
- heartbeat the writer while active and cancel safely if that authority can no longer be renewed;
- cancel active verifier runs on Environment drop or Gateway-owner shutdown and wait boundedly for cleanup;
- never persist verifier-run observations/results to desired state and never resume, infer or resurrect them after Gateway restart;
- never automatically retry/replay a failed, timed-out, canceled or disconnected verification.

The existing synchronous verifier path remains available. ADM must not invent an arbitrary duration threshold that rejects valid blocking verification. If an outer caller/request cancellation interrupts a blocking verifier before completion, diagnostics should distinguish that interruption from the verifier's own configured timeout and direct long-running callers toward the async verifier lifecycle. ADM must not silently convert an interrupted blocking call into background work.

Acceptance:

- a real long verifier returns a stable async identity while still running; after the launching client disconnects, a later client on the same Gateway owner can observe early bounded output and terminal result;
- pass, non-zero exit and configured verifier timeout retain structured verifier result semantics;
- wrong writer cannot start/cancel; matching writer can cancel and the owned command tree stops;
- missing/disabled verifier, forbidden executable, escaped cwd and invalid managed root fail before installing a verifier-run resource;
- live/terminal output remains bounded and reports truncation;
- owner shutdown and Environment drop cancel active verifier runs; Gateway restart begins with no prior verifier-run identities and persisted state contains no verifier-run observation;
- a caller-interrupted synchronous verifier reports a clear blocking-request diagnostic when a response is still possible, while verifier-configured timeout remains a normal failed verifier result;
- ordinary file development and Environment creation remain usable with no verifier configured.

### ADM-CORE-022 — Temporary Environment lifecycle is explicit, scoped, and conservatively cleanable

ADM may provide a first-class temporary Environment workflow for external Agents that need short-lived development contexts. This is lifecycle infrastructure, not task orchestration: ADM does not choose the task, decompose work, schedule Runs, merge branches, or decide when work is complete.

A temporary Environment must be created atomically with explicit retention metadata rather than by creating a durable Environment and silently converting it later. The workflow must require a stable owner identifier and an explicit positive TTL. Optional session/run attachment is provenance only and must not create parent/child execution semantics or make a Run a prerequisite.

Temporary creation supports two operation-local modes:

- an existing Workspace-contained root, which creates only ADM Environment metadata and never creates or deletes the project directory; and
- an optional ADM-managed Git worktree, which reuses the existing managed-worktree identity, validation and destroy safety. Git remains optional and is required only for this mode.

Temporary Environment lifecycle must:

- persist `temporary` retention class, creator surface, owner identity, creation/last-use facts, explicit expiry and optional session/run provenance;
- reject accidental reuse of an existing durable Environment as a temporary creation result;
- expose Environment-scoped status/cleanup evidence with the same conservative retention blockers used by the generic retention system;
- make cleanup preview non-mutating and target only the requested Environment rather than sweeping unrelated temporary resources;
- allow cleanup execution only when the Environment is still temporary, expired, owned by the matching lifecycle owner and free of active writer, MCP operation/session, development process, generic `run_`, async verifier `vfrun_`, or other runtime blockers;
- remove an ordinary temporary Environment only from ADM state, including its private Environment context, while leaving Workspace registration, project directories and host files untouched;
- destroy a temporary managed-worktree Environment only through the existing managed-worktree safety path, refuse dirty or unpublished work, and retain the managed branch after cleanup;
- allow the matching lifecycle owner to promote the temporary Environment to durable retention without moving files, merging/pushing Git, changing Environment identity or rewriting its development context;
- never perform automatic/background garbage collection, forced managed-worktree deletion, automatic merge/push, or cleanup based only on an expired timestamp without current safety evidence.

The existing generic resource-retention management surfaces remain valid administrative tools. The temporary Environment workflow is a narrower Environment-scoped convenience path and must reuse their retention/safety semantics rather than define a second cleanup policy.

Acceptance:

- a plain non-Git Workspace can create a temporary Environment on an existing contained root with explicit owner + TTL, and ordinary files remain usable;
- optional managed-worktree temporary creation succeeds only when Git/worktree capability is available and produces an ADM-owned isolated root with temporary retention recorded at creation;
- missing/invalid owner, non-positive TTL, escaped/missing existing root, invalid managed base ref, or creation collision fails without a half-created temporary Environment;
- cleanup preview is read-only and reports not-due, owner mismatch, active writer/process/run/verifier/MCP, dirty/unpublished managed worktree and other blockers without deleting anything;
- wrong lifecycle owner cannot promote or execute cleanup; matching owner can promote, after which cleanup reports the Environment as durable;
- eligible ordinary cleanup removes only Environment state/private context and preserves the Workspace and underlying directory/files;
- eligible managed-worktree cleanup removes only the ADM-owned worktree through existing destroy safety and retains its branch; dirty/unpublished work blocks cleanup and no force option is exposed by the temporary workflow;
- an active async verifier `vfrun_` blocks cleanup just like active process/generic Run activity;
- restart preserves temporary retention metadata and expiry but does not infer owner-local runtime activity; cleanup still requires fresh current-owner safety evidence;
- no Git, verifier, process, Run, MCP or Skill becomes a prerequisite for ordinary temporary Environment creation on an existing root.

### ADM-CLI-001 — CLI automation and provisioning reuse Admin MCP with explicit stable scope

The normal `adm` CLI is a thin human/automation client over the same Admin MCP and application contracts used by other management surfaces. Phase-23 CLI convenience must not create a second API, state path, authorization model, hidden current Environment, or CLI-only lifecycle semantics.

Normal CLI management/provisioning/context commands must:

- route through the selected Admin MCP target and fail clearly when it is unavailable rather than falling back to writable local `state.json`;
- use explicit stable Workspace/Environment/resource IDs and never infer one implicit current project from the shell directory;
- keep successful management, provisioning, diagnostic, context and lifecycle data machine-readable as JSON on stdout, while help remains human-readable and errors remain non-zero failures reported on stderr;
- preserve the existing Admin-MCP authority boundary for creator/owner provenance rather than accepting a caller-forged `creator_surface` or equivalent authority field.

MCP CLI provisioning must reuse the canonical MCP import/runtime contracts. `mcp import-preview` and `mcp import-apply` may accept content inline, from one explicitly named local file, or from explicitly requested stdin, but exactly one content source is allowed and preview/apply must share the same local input semantics. Reading a local file/stdin is a CLI transport convenience only: raw import blobs are not persisted by the CLI, import still writes only canonical global MCP definitions, existing Environment selections are not changed, and literal credentials retain the reference-only safety defined by ADM-CORE-012/018.

MCP diagnostics must keep distinct semantics discoverable rather than collapsing them into one ambiguous command: global `mcp probe` is a bounded transient configuration/connection probe without Environment selection; Environment `status` is an explicit selected-Environment probe; owner-runtime `inspect` is passive sanitized desired/observed evidence; and `refresh` is an explicit owner-runtime reconnect/Ping/tool-inventory refresh. Refresh must not invoke business tools, change catalog definitions or enable an MCP for an Environment.

Skill CLI provisioning must expose the existing source-aware model rather than invent another installation model. Source configuration update and refresh remain separate operations; support roots remain explicit; catalog structural availability and Environment-specific enabled/disabled/broken availability are distinct; list/availability commands do not read Skill instructions or execute Skill reasoning policy.

Environment Agent-workflow CLI convenience must reuse existing stable Core/Gateway contracts:

- context output calls the same bounded `environment_context_bundle` for one explicit Environment ID, with no hidden selection, writer acquisition, MCP probing, Memory-value injection or task execution;
- temporary Environment create/status/promote/cleanup calls the same ADM-CORE-022 lifecycle, requires explicit lifecycle owner + positive TTL where applicable, treats session/run IDs as provenance only, defaults cleanup to preview unless execution is explicitly requested, exposes no force cleanup path, and never adds task/GSD orchestration.

The existing `gateway`, `doctor`, and `state` commands remain explicit local bootstrap/offline/recovery surfaces. Phase 23 does not require a universal CLI formatter or convert those recovery commands into a peer management API merely to claim JSON coverage.

Acceptance:

- MCP import preview/apply succeeds from a local JSON/JSONC file and redirected stdin without Windows native-shell JSON quoting, while conflicting simultaneous content sources fail before an Admin MCP mutation;
- MCP global probe, Environment status, passive owner-runtime inspect and explicit refresh are individually discoverable and preserve their distinct side-effect boundaries;
- Skill source configuration can be added/updated with explicit support roots, refreshed separately, and inspected through global structural plus Environment-specific availability without reading Skill contents;
- `environment context` returns the canonical bounded Phase-20 bundle for an explicit Environment ID and keeps private Memory values/full Skill contents absent;
- CLI temporary lifecycle can create/status/promote/preview/execute through the Phase-22 Admin MCP tools, with wrong-owner/no-force/ordinary-directory-preservation safety unchanged;
- new CLI management commands still fail when the selected Admin MCP is unavailable and do not silently mutate local persisted state;
- ordinary non-Git Workspace/Environment management remains valid and Git is required only for explicitly requested managed-worktree operations.

### ADM-SURFACE-001 — Desktop and CLI are distinct presentation surfaces over one shared management contract

ADM may optimize Desktop and CLI independently, but presentation separation must not fork product semantics, authority, persistence or transport behavior.

Normal Desktop and CLI management both use the shared Admin MCP management plane and canonical Core/application models. Surface-specific code owns only presentation concerns: CLI command grammar, flags, shell/stdin/file ergonomics and JSON/text presentation; Desktop Wails bindings, connection profiles, local-start UI integration and frontend presentation state.

Shared connection-target semantics — Base URL normalization, derived health/Agent MCP/Admin MCP endpoints and eligibility for local Gateway lifecycle control — must come from one reusable implementation. Surface-specific target selection remains valid (`--adm-url` / `ADM_V2_URL` for CLI and saved connection profiles for Desktop), but those surfaces must not maintain incompatible URL authority rules.

Surface separation must preserve these boundaries:

- one persisted ADM desired state and one Core/application product model;
- one normal Admin MCP management protocol/client implementation;
- no CLI-only or Desktop-only copy of Workspace, Environment, MCP, Skill, Memory, retention, writer or Runtime business rules;
- no automatic writable local-state fallback when Admin MCP is unavailable;
- explicit local CLI `gateway`, `doctor` and `state` bootstrap/offline/recovery behavior remains an exception rather than a peer management plane;
- Desktop and CLI packages do not import each other;
- Core/application/Gateway/Admin-MCP packages do not depend on presentation packages;
- surface-local narrow interfaces may exist for testing, while the production shared Admin MCP client satisfies them;
- user-facing release names remain `adm` and `adm-desktop` unless a separate product decision changes them.

Logical separation does not require a multi-repository split, Go module rename, second state file/cache, second management API, frontend framework migration or duplicated validation layer.

Acceptance:

- equivalent ADM targets normalize to the same Base URL and derived health/Agent/Admin MCP endpoints from CLI and Desktop;
- local lifecycle eligibility is identical for both surfaces and remains limited to supported loopback HTTP root targets;
- disconnected/incompatible normal management does not install or use a writable local fallback in either surface;
- the CLI executable can be reduced to process bootstrap over a dedicated CLI surface package without changing command/JSON/error semantics;
- Desktop adapter responsibilities can be separated internally without changing its Wails/frontend contract or production Admin-MCP-only behavior;
- package dependency tests/review show no CLI↔Desktop coupling and no Core→presentation dependency;
- full CLI/Desktop/Core regression and release builds remain green after the structural split.

## Human management boundary

### ADM-MGMT-001 — Management clients reuse application state through one boundary

Desktop or other human management clients must not read or mutate `state.json` directly and must not introduce a second persistence or product model.

ADM must provide an application-backed management boundary that can summarize the current ADM installation and host explicit management mutations. The boundary reuses the same persisted services used by CLI and MCP.

The read-only snapshot contains registered Workspaces, lightweight Environment summaries, the exec allowlist, MCP catalog, Skill catalog, and Global Memory entry count. Memory values are not part of the snapshot.

Explicit management mutations must be thin named methods for product operations already defined elsewhere, including Workspace/Environment lifecycle, exec allowlist, MCP/Skill catalog and Environment selections, and explicit-scope Memory writes/deletes. These methods delegate to existing application services instead of duplicating validation, persistence, or safety rules.

The desktop-facing adapter must expose exported Go methods and JSON-friendly inputs over this management boundary. Overview responses must remain sanitized; Memory values may be returned only through explicit Memory list/read operations. The adapter must not require direct `state.json` access, a REST layer, or a running Wails runtime merely to exercise management behavior in tests.

Acceptance:

- snapshot data comes from the existing application services/store
- no second state file or cache is introduced
- Environment summaries do not expose private Memory values
- Global Memory values are not exposed by the snapshot
- snapshot works without Git, worktree, verifier, or Gateway process prerequisites
- subsequent persisted changes are visible in the next snapshot
- management mutations preserve existing Workspace/Environment deletion guards and metadata-only rename semantics
- management mutation results do not expose Environment-private Memory values
- allowlist, catalog, selection, and Memory validation behavior remains the same as the existing application services
- management mutations do not write `state.json` directly
- desktop overview/inspect paths do not expose private Memory values
- desktop Memory values are available only through explicit Memory list/read methods
- desktop adapter behavior can be tested without a running Wails runtime
- no REST API is required by the desktop adapter

## Desktop management application

### ADM-DESKTOP-001 — Desktop shell binds the management boundary

ADM may provide a cross-platform desktop management application using Wails. The desktop application is a human management surface, not a replacement for the Agent Gateway.

The first desktop slice must remain deliberately small: start a desktop window, bind the existing desktop management adapter, render the read-only management snapshot, and allow the user to refresh it. It must reuse the same default ADM state path as the CLI and must not introduce a second state store, REST API, Gateway prerequisite, Git prerequisite, or frontend build toolchain.

The first slice may use embedded plain HTML/CSS/JavaScript assets. Later slices may add mutations and explicit detail/Memory panels through the already-defined desktop adapter.

Desktop Gateway controls must reuse the same HTTP Gateway lifecycle semantics as the CLI. The desktop may start the same `gateway.RunHTTP` implementation through an internal detached child mode, but it must not introduce a second daemon or generic process manager. Start succeeds only after the Gateway health endpoint is ready. Incompatible endpoints must be surfaced and refused rather than automatically terminated by the desktop.

Acceptance:

- desktop command is a separate executable entrypoint from the CLI/Gateway command
- desktop startup uses `store.DefaultPath()` and the existing application/management/desktop layers
- Wails binds the existing desktop adapter rather than persistence services directly
- the initial page renders Workspace, Environment, allowlist, MCP, Skill, and Global Memory counts from `GetSnapshot`
- the page can refresh the snapshot without restarting the desktop application
- the desktop shell does not require a running Gateway
- the first desktop slice does not require npm, Vite, Node, or another frontend build system
- existing CLI/Gateway behavior remains unchanged
- desktop Gateway status reports stopped/running/incompatible without requiring the desktop to own the Gateway process
- desktop detached start waits for the same HTTP health contract before reporting success
- desktop stop refuses incompatible endpoints instead of attempting automatic recovery or killing an unknown listener
- CLI and desktop current-version Gateway health/termination paths share lifecycle helpers rather than diverging

## Agent-facing Gateway

### ADM-GW-001 — Gateway routes by Workspace/Environment identity

Agent tools operate against explicit stable IDs rather than assuming one current repository.

### ADM-GW-002 — Gateway must remain usable without Git

Gateway discovery, Environment lifecycle, read/search and permitted file mutation must work for plain directories.

Git-specific tools may return a clear unsupported-capability error when unavailable.

### ADM-GW-003 — Tool failure must be local

A failure or absence of one tool must not make unrelated Gateway tools unavailable.

### ADM-GW-004 — HTTP Gateway supports foreground and detached startup

`gateway start` defaults to foreground execution in the current terminal.

`gateway start -d` / `gateway start --detach` must start the same ADM V2 HTTP Gateway as a process detached from the current terminal, wait until its health endpoint reports ready, and then return control to the caller. The resulting process must remain manageable through the existing `gateway status` and `gateway stop` commands.

Acceptance:

- foreground `gateway start` remains blocking and Ctrl+C-stoppable
- `gateway start -d` and `gateway start --detach` return only after `/healthz` is ready
- closing the launching terminal does not stop the detached Gateway
- `gateway status` reports the detached Gateway PID/version
- `gateway stop` stops the detached Gateway
- detached startup does not introduce a separate daemon or generic process-manager prerequisite

## Development process contract

### PROC-01 — Gateway-owned long-running development processes

A long-running development process is an Environment-scoped Runtime resource owned by the persistent ADM Gateway, not by the short Agent request or client invocation that starts it.

Starting a process must reuse the same executable allowlist, Environment-root cwd containment, writer lease and OS process-tree cancellation policy as ordinary `exec`. ADM returns a stable `proc_` identity and does not expose arbitrary raw-PID control as an Agent capability.

A later client connected to the same running Gateway can list/status/stop that owned process by ADM identity. Clean owner/Gateway shutdown must terminate still-running owned process trees. Gateway restart must not serialize or resurrect process observations from the prior owner.

### PROC-02 — Bounded queryable process logs

ADM-owned development processes retain bounded stdout/stderr tails in owner memory so later clients can inspect recent output without unbounded log growth. Process/log observations are not persisted to `state.json`. Logs may remain queryable for an exited process while that Gateway owner remains alive.

### PROC-03 — Listening ports are owned-process facts

ADM may report listening TCP ports observed for a process it already owns. Port reporting is observational only: Agents do not supply arbitrary PIDs and ADM must not become a generic OS process/port manager.

Acceptance:

- start an allowlisted real local dev server through the HTTP Agent Gateway and return control immediately
- disconnect the launching client; a later client sees the same `proc_` identity, bounded stdout/stderr and listening port
- direct HTTP traffic succeeds on the reported listening port
- wrong writer, forbidden executable and escaped cwd are rejected locally
- explicit stop terminates the owned process tree and releases its port
- clean Gateway shutdown terminates any remaining owned dev process
- after Gateway restart, prior `proc_` identities are absent and no process/log/port observed state was persisted
- ordinary Environment/file development still works without any long-running process configured

## Agent Run contract

### ARUN-01 — Persistent-owner Agent Run lifecycle

An Agent Run is a stable `run_` runtime resource owned by the persistent ADM Gateway rather than by the short MCP request or client connection that starts it. The Run payload is one asynchronous Environment-scoped command. ADM keeps this lifecycle task-semantic-neutral; multi-step task planning, Planner/Executor/Reviewer orchestration, GSD phase policy and parent/child Agent workflows belong to the consuming Agent/orchestrator rather than the Run contract.

Run start must require the active Environment writer and reuse the existing Runtime command authority: executable allowlist, Environment-relative cwd containment, managed-worktree revalidation, bounded output, timeout, and OS process-tree cancellation. ADM must not create a second hidden command-execution policy for Runs.

A running Run remains observable after the launching client disconnects. A later client connected to the same Gateway owner can list and inspect it by stable identity, including bounded stdout/stderr snapshots produced so far, and can cancel it only with the matching writer. Lifecycle state distinguishes `running`, `succeeded`, `failed`, and `canceled`; terminal status may retain the bounded command result while that owner remains alive.

Run observations are owner-local, not desired persisted state. Gateway/owner shutdown must cancel active Runs and wait boundedly for cleanup. A restarted owner starts with no prior Run identities and must not serialize, infer, resume, or resurrect stale Runs from `state.json`.

Acceptance:

- `run_start` returns a stable `run_` identity while a real allowlisted helper command remains running;
- after the launching MCP client disconnects, a later client can `run_list` / `run_status` the same Run, and `run_status` exposes bounded running stdout/stderr snapshots when output has already been produced;
- exit 0 becomes `succeeded`; non-zero exit becomes `failed` with the command result retained;
- wrong writer cannot cancel; matching writer `run_cancel` deterministically reaches `canceled` and stops the command tree;
- forbidden executable and escaped cwd are rejected before a Run is installed;
- owner Environment drop (including cleanup after a successful Environment removal) and Gateway shutdown cancel affected active Runs;
- after Gateway restart, Run list is empty, old Run IDs are invalid, and persisted state contains no Run observation;
- ordinary file/process/Git-optional Environment development remains usable without any Agent Run.

## Orchestration boundary

### ADM-NONGOAL-001 — Task/GSD orchestration is outside ADM Core

ADM does not provide a product-level Planner/Executor/Reviewer workflow, GSD phase executor, `.planning/STATE.md` advancement, next-phase selection, parent/child Agent policy, or automatic Git integration decision engine.

An external Agent, GSD, or another orchestrator may compose ADM's generic capabilities — files, exec, verifier, MCP, Skill, process, Git/worktree and single-command asynchronous Runs — into its own workflow. ADM remains responsible for local authority, lifecycle, safety and diagnostics at each capability boundary.

The previously implemented Phase 9 `run_workflow_start` surface is historical/mis-scoped implementation and was removed from ADM Core by Phase 10 under BOUNDARY-01. Future ADM features must not depend on FLOW-01 semantics.

### ADM-DEV-001 — Requirement traceability

Every implementation phase must name the requirement IDs it changes or implements.

### ADM-DEV-002 — Explicit prerequisites

Every new prerequisite must be listed in the phase plan and requires user approval before implementation.

### ADM-DEV-003 — Negative acceptance tests

Tests must prove not only that features work, but that unrelated tools are not prerequisites.

### ADM-DEV-004 — Dogfood gate

Before expanding into speculative orchestration/isolation features, use V2 through its Agent Gateway to perform real development work on another project.

Real blockers discovered by dogfooding take priority over speculative features.

## Initial non-goals

The first V2 milestone intentionally does NOT include:

- clone-based Environment isolation
- branch management
- automatic Git synchronization
- Planner / Executor / Reviewer task orchestration
- parallel Agent workflow policy / parent aggregation
- GSD automation engine / `.planning` state advancement
- Docker as a prerequisite
- UI / Windows Client
- migration from ADM V1 Environment data
- compatibility with V1 Environment store format
- generic OS process/log/port management outside ADM-owned Environment development processes

These may be added later only from concrete requirements.

## V2 first milestone acceptance gate

The first milestone is accepted only when an Agent can complete a real development loop through ADM MCP against an ordinary local project directory, including a directory with no `.git` directory.

The end-to-end gate is:

1. select/open a local project through ADM
2. inspect the project tree and read files
3. search for relevant code
4. create and edit files
5. delete a file when the task requires it
6. execute a permitted project command
7. run the project's tests/build/lint when those capabilities exist
8. inspect command results and make a follow-up code change
9. preserve the development context across an ADM restart
10. complete the task without Git being required

Supporting capabilities required by the same milestone include Workspace/Environment identity, global MCP/Skill definitions and Environment selections, global Memory, Environment-private Memory, and safe mutation/concurrency control. These supporting abstractions are not themselves the product acceptance goal; they are accepted only insofar as they support the Agent development loop above.
