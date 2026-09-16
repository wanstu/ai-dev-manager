# Phase 14 Priority Map — Evidence-first Investigation Toolkit

## Priority principles

1. Do not build a second planner/orchestrator. Every helper returns evidence, confidence and uncertainty only.
2. Prefer provider integration over rebuilding complex code intelligence engines inside ADM.
3. Every optional provider must degrade locally; absence or failure must not break built-in file/search/runtime capabilities.
4. Prefer one thin, verifiable slice per node, with a commit at each node.
5. Passive inspection must stay side-effect-free unless a user explicitly invokes a mutating or execution-gated operation.

## Ordered slices

### 14-01 — Endpoint evidence resolution ✅

Status: complete at `0e40394`.

Purpose: map a URL/path plus optional HTTP method to static route evidence.

### 14-02 — Optional code intelligence provider + GitNexus integration

Priority: P0 next.

Purpose: add provider-neutral plumbing so ADM can integrate GitNexus or similar graph tools without making them hard dependencies.

Key result: ADM can say whether GitNexus/code graph evidence is configured, available, stale, disabled or unavailable, and can normalize provider evidence into ADM investigation results.

### 14-03 — Symbol / reference / write tracing

Priority: P1 after 14-02.

Purpose: answer where a symbol is defined, referenced, called, assigned or exported.

Provider use: GitNexus/code graph evidence first when available; static textual/heuristic fallback otherwise.

### 14-04 — Impact / blast-radius evidence

Priority: P1 after 14-03.

Purpose: given a file/symbol/change target, return upstream/downstream affected callers, routes, tests and uncertainties.

Provider use: GitNexus impact/call graph evidence can materially improve this slice, but ADM must still return bounded fallback evidence when the provider is absent.

### 14-05 — Data lineage / response field provenance

Priority: P2.

Purpose: trace how a response field or DTO/model value is assembled from handlers, services, repositories, SQL, configs or external calls.

### 14-06 — Debug-SQL reverse mapping and test-data metric explanation

Priority: P2.

Purpose: connect SQL/table/column/test-data symptoms back to code paths, fixtures and verification evidence.

### 14-07 — Semantic consistency checks

Priority: P3.

Purpose: compare evidence from routes, symbols, configs and tests to highlight contradictions. This must remain evidence-based and cannot become an opaque AI reviewer.

## GitNexus priority decision

GitNexus should be integrated before deep symbol/write/impact helpers because it can provide precomputed graph relationships that are expensive for ADM to reproduce. ADM should first build a provider adapter boundary, then use the provider in later helpers opportunistically.

## Out-of-phase items

### Temporary resource lifecycle / retention

Move from deferred note into Phase 15 planning. This is not an investigation helper; it is a lifecycle/management capability for temporary Env/MCP/Skill/resources created through Gateway/MCP flows.

### Desktop Core parity

Move after temporary lifecycle, because Desktop should expose stable lifecycle semantics instead of inventing its own cleanup model.

### Distribution

Keep last and conditional.
