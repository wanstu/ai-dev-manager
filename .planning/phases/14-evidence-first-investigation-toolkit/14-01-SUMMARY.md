# 14-01 Summary — Endpoint Evidence Resolution

## Result

Implemented the first Phase 14 evidence-first investigation helper: `investigate_endpoint`.

Given an Environment, target URL/path, optional HTTP method, and optional search root, ADM now returns bounded static route evidence with explicit confidence and uncertainties.

## Implementation

- Added shared model structs:
  - `EndpointInvestigationRequest`
  - `EndpointInvestigationReport`
  - `EndpointInvestigationEvidence`
- Added app service method:
  - `Service.InvestigateEndpoint`
- Added Agent-facing Gateway tool:
  - `investigate_endpoint`
- Added tests for:
  - full URL normalization to path-only targets;
  - exact route literal evidence;
  - conservative dynamic route candidates such as `{id}`;
  - method evidence on the same line;
  - no-evidence uncertainty reporting;
  - Gateway tool exposure.

## Boundary

The helper is static and side-effect-free. It does not:

- start project servers;
- call endpoints;
- execute commands;
- run verifiers;
- call MCP tools;
- start processes or runs;
- acquire writer leases;
- infer a task plan or fix strategy.

It is intentionally not a framework interpreter. The report includes uncertainties such as `static_literal_search_only` and `framework_specific_registration_may_be_indirect`.

## Validation

Passed before implementation commit `0e40394`:

- `go test -count=1 ./internal/app -run TestInvestigateEndpoint`
- `go test -count=1 ./internal/gateway -run TestGatewayInvestigateEndpoint`
- `go test -count=1 ./...`
- `go vet ./...`
- `git diff --check`

`git diff --check` emitted only the existing Windows LF→CRLF warning for `internal/gateway/server.go`.

## Follow-up

Phase 14 remains open for additional evidence-first slices. Candidate next slices remain symbol/reference/write tracing, data lineage, symbol-scoped Git history/diff, debug-SQL reverse mapping, test-data metric explanation, and semantic consistency checks. Each future slice needs its own plan and node commit.
