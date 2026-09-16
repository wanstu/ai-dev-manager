# Phase 01 — Git-independent development core

## Requirement IDs

This phase implements or establishes the first executable slice of:

- ADM-GOAL-001
- ADM-CORE-001
- ADM-CORE-002
- ADM-CORE-003
- ADM-CORE-004
- ADM-CORE-005
- ADM-CORE-006
- ADM-CORE-007
- ADM-CORE-008
- ADM-CORE-009
- ADM-CORE-010
- ADM-GW-001
- ADM-GW-002
- ADM-GW-003
- ADM-DEV-001
- ADM-DEV-003

## Delivered behavior

- Workspace admission validates only that the requested path is an existing directory.
- Environment creation defaults directly to the Workspace directory and has no Git/worktree fields or Git admission check.
- Core file development operations are available on ordinary directories.
- Command execution is enabled only by an explicit executable allowlist.
- Git status/diff/branch are detected and executed as optional per-root capabilities.
- A missing Git repository causes only Git operations to fail.
- Writer coordination is keyed by physical Environment root, not by branch/worktree identity.
- A stdio MCP Gateway routes operations by explicit `environment_id`.
- State is persisted independently of Git.

## Negative acceptance tests

Automated tests prove that:

- an empty directory can be registered and opened
- a directory with no `.git` can be registered and developed
- Environment creation does not require Git
- file read/write/edit/search/delete does not require Git
- `shell.exec` is absent until an executable is explicitly allowlisted
- Git capabilities are absent on a plain directory
- invoking `git_status` through MCP on a plain directory fails only that tool
- two Environments sharing one physical root cannot concurrently acquire different writers
- persisted Environment/writer state survives constructing a new service instance
- the non-Git development path works through a real in-memory MCP client/server session

## Explicit non-goals

This phase does not implement:

- Git worktree lifecycle
- branch-per-task semantics
- GSD workflow execution
- Docker isolation
- clone/copy/VM isolation
- parallel Agent orchestration
- verifier configuration
- process/log/port management
- V1 state migration or compatibility
- UI

## New prerequisites

None for Workspace or Environment admission.

The MCP transport depends on `github.com/modelcontextprotocol/go-sdk`. Local command execution depends only on executables explicitly added to the ADM allowlist. Git functionality depends on Git only when a Git operation is requested.
