---
phase: 3
slug: external-mcp-runtime-completion
status: passed
nyquist_compliant: true
wave_0_complete: true
created: 2026-09-07
---

# Phase 3 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test (stdlib) |
| **Config file** | go.mod (module root) |
| **Quick run command** | `go test ./internal/catalog/... ./internal/app/... ./internal/gateway/...` |
| **Full suite command** | `go test ./...` |
| **Estimated runtime** | ~15 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go test ./internal/catalog/... ./internal/app/... ./internal/gateway/...`
- **After every plan wave:** Run `go test ./...`
- **Before `/gsd-verify-work`:** Full suite must be green
- **Max feedback latency:** 15 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Test Type | Automated Command | Status |
|---------|------|------|-------------|-----------|-------------------|--------|
| 03-01-01 | 03-01 | 1 | MCP-RUN-01 | unit | `go test ./internal/catalog/...` | ✅ green |
| 03-01-02 | 03-01 | 1 | MCP-RUN-03, MCP-RUN-05 | unit | `go test ./internal/app/...` | ✅ green |
| 03-01-03 | 03-01 | 1 | MCP-RUN-03, MCP-RUN-04 | integration | `go test ./internal/gateway/...` | ✅ green |
| 03-02-01 | 03-02 | 2 | MCP-RUN-03 | integration | `go test ./internal/management/... ./internal/desktop/... ./cmd/ai-dev-manager/...` | ✅ green |
| 03-02-02 | 03-02 | 2 | MCP-RUN-01..05 | validation | `go test ./... && go vet ./...` | ✅ green |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [x] `internal/app/mcp_health.go` — health probe logic
- [x] `internal/app/mcp_health_test.go` — health probe unit tests
- [x] `internal/gateway/mcp_acceptance_test.go` — real HTTP acceptance tests

*Existing test infrastructure (go test) covers all phase requirements. No framework install needed.*

---

## Manual-Only Verifications

None. The originally manual candidates are covered by automated real Streamable HTTP fixtures and secret non-exposure assertions:

- `TestMCPHealthLifecycleEndToEnd` starts a real local MCP server and proves health, list, call, disable, broken-upstream error, and unrelated-tool isolation.
- `TestProbeMCPHealthAuthFailureDoesNotExposeSecrets` and related app tests prove resolved endpoint/header secret values are absent from normal health/error output.

No human-only Phase 3 exit criterion remains.

---

## Validation Sign-Off

- [x] All tasks have `<automated>` verify commands
- [x] Sampling continuity: no 3 consecutive tasks without automated verify
- [x] Wave 0 covers all MISSING references
- [x] No watch-mode flags
- [x] Feedback latency < 15s
- [x] `nyquist_compliant: true` set in frontmatter

**Approval:** passed by deterministic local verification on 2026-09-07; no external LLM verifier used per user instruction.
