# Phase 14 Validation

Phase 14 helpers must prove investigation is evidence-only, bounded and side-effect-free.

## Required gates for every Phase 14 slice

- focused app tests for the slice;
- focused Gateway tests for the slice when an Agent-facing tool is added;
- `go test -count=1 ./...`;
- `go vet ./...`;
- `git diff --check`.

## Safety checks for every Phase 14 slice

- no writer lease acquisition unless the explicit operation is already writer-gated;
- no process start, verifier run, shell command, endpoint call, browser probe, MCP tool call, or provider indexing during passive inspection;
- no private Memory values in evidence;
- evidence must be bounded by file/match/byte limits;
- every result must include confidence and uncertainties when evidence is partial, stale or heuristic;
- provider failures must be local and must not break built-in fallback investigation.

## 14-01 gates

14-01 endpoint evidence resolution proved:

- exact route literal evidence;
- conservative dynamic route candidates;
- optional same-line HTTP method evidence;
- bounded results;
- private Memory non-leakage;
- Gateway `investigate_endpoint` output;
- full test/vet/diff gates at `0e40394`.

## 14-02 gates

14-02 optional code intelligence provider + GitNexus integration proved:

- GitNexus/provider absence reports `unconfigured` rather than failing ordinary investigation;
- provider failure reports structured unavailable/degraded state while static endpoint fallback remains available;
- passive provider inspection uses desired configuration and existing Gateway-owner observations without connecting, refreshing inventory, indexing, calling MCP tools, calling endpoints or mutating files;
- provider evidence carries source/provenance, confidence, freshness and uncertainty;
- GitNexus remains optional and uses existing Environment MCP/runtime authorization rather than a new privileged path;
- Gateway inventory normalization exposes only recognized read-only provider capabilities;
- focused app and Gateway gates execute real `TestInvestigationProvider*` and `TestGatewayInvestigationProvider*` coverage;
- full test, vet and diff-check gates pass on the working tree.
