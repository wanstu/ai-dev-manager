# 15-02 Validation — Temporary Environment Cleanup and Bounded Inspection

Date: 2026-09-11
Status: complete; rebuilt Gateway live dogfood passed.

## Scope validated

- Temporary ordinary Environment cleanup can remove only the ADM Environment state record.
- Workspace records and ordinary project directories are preserved during ordinary Environment cleanup.
- Temporary ADM-managed worktree cleanup is delegated through existing isolation destroy safety instead of duplicating deletion logic.
- Dirty managed worktrees are blocked from retention cleanup.
- Runtime-owner evidence is required for Environment and MCP cleanup execution paths that depend on Gateway-owned activity observations.
- Environment capability reporting now bounds default output by returning catalog summary facts plus selected MCP/Skill detail facts only.

## Commands run before `48b0858`

```text
go test -count=1 ./internal/app -run TestCapabilityReport
go test -count=1 ./internal/gateway -run TestGatewayCapabilityReport
go test -count=1 ./internal/app -run TestResourceRetention
go test -count=1 ./internal/gateway -run TestGatewayResourceRetention
go test -count=1 ./internal/adminmcp ./internal/management
go test -count=1 ./cmd/...
go vet ./internal/app ./internal/gateway ./internal/adminmcp ./internal/management ./cmd/...
git diff --check
```

## Live dogfood validation after `48b0858`

Built a fresh CLI/Gateway binary into ignored `dist/` output and started it on an alternate loopback port so the active `pjadm` MCP connection was not disrupted.

```text
go build -o dist/adm-live-verify.exe ./cmd/ai-dev-manager
.\dist\adm-live-verify.exe gateway start --listen 127.0.0.1:43138 --detach
.\dist\adm-live-verify.exe --adm-url http://127.0.0.1:43138 environment inspect --environment-id env_43a2d0ca74fbc0f1
.\dist\adm-live-verify.exe gateway stop --listen 127.0.0.1:43138
```

Observed acceptance evidence from the rebuilt Gateway:

- `mcp.catalog` is emitted as one summary fact with `catalog_count=3` and `selected_count=3`.
- Only the three Environment-selected MCP detail facts are emitted.
- `skill.catalog` is emitted as one summary fact with `catalog_count=207`, `selected_count=0`, `available_selected_count=0`, `unavailable_selected_count=0` and `suppressed_disabled_fact_count=207`.
- No per-Skill disabled facts are emitted for unselected catalog Skills.
- The alternate Gateway stopped cleanly after validation.

## Closeout

Phase 15 15-02 is accepted. Future lifecycle changes must preserve the safety boundary: no ordinary workspace/project/host-file deletion, no cleanup without explicit temporary ownership and expiry evidence, and no managed worktree deletion except through the existing managed worktree destroy safety path.
