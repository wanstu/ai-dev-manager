# Phase 14 Context — Evidence-first Investigation Toolkit

## Product boundary

ADM remains a local development control plane. Phase 14 may add investigation helpers only when they return concrete evidence and improve on plain text search. ADM must not infer tasks, plan work, advance `.planning` state, or choose orchestration policy.

## Phase 14 goal

Expose high-value code/runtime investigation helpers that an external Agent can call to gather bounded evidence. Every helper must return evidence, confidence, and uncertainties; no opaque AI guesses.

## Completed first slice

14-01 implements endpoint evidence resolution. It takes one URL/path and optional HTTP method, searches the Environment root for route-like literals and nearby method evidence, and returns bounded file/line evidence with confidence and uncertainties.

## Next slice direction

14-02 plans a provider-neutral code intelligence integration layer, with GitNexus as the first optional provider candidate. GitNexus or similar tools may already compute symbol, dependency, call-chain and impact evidence. ADM should integrate such providers through existing MCP/runtime authorization and normalize their output, not rebuild a full code graph engine inside ADM.

## Priority order

1. 14-02 optional code intelligence provider + GitNexus integration boundary.
2. 14-03 symbol/reference/write tracing using provider evidence when available and static fallback otherwise.
3. 14-04 impact/blast-radius evidence.
4. 14-05 data lineage / response-field provenance.
5. 14-06 debug-SQL reverse mapping and test-data metric explanation.
6. 14-07 semantic consistency checks.

## Non-goals

- No server startup, network request, browser probe, verifier run, command execution, or MCP tool call during passive investigation/capability inspection.
- No semantic code execution or framework-specific interpreter as a hidden dependency.
- No hard dependency on GitNexus or any external provider.
- No automatic provider indexing without explicit user/provider configuration.
- No task planning or automatic fix suggestion.
- No `.planning` state advancement beyond normal plan/summary docs.

## Current development rule

After Phase 13 was merged back, active development continues directly on `master` in `D:\projects\ai-dev-manager-v2`. Old rebaseline ADM workspace/env metadata has been cleaned; the physical Git worktree is not deleted by ADM metadata cleanup.
