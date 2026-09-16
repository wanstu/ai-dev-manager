# 14-02 Summary — Optional Code Intelligence Provider + GitNexus Boundary

## Result

Closed the Phase 14 optional code-intelligence provider slice around the provider-neutral investigation boundary and GitNexus MCP integration shape.

ADM now has a passive provider inspection path that reports optional GitNexus availability through existing Environment MCP authorization and existing Gateway-owner observations. GitNexus remains optional; absence or failure does not break built-in static endpoint investigation.

## Implementation

The repository already contains the 14-02 production boundary:

- `InvestigationProvider` abstraction in the app layer;
- `code_intelligence.gitnexus` capability fact family;
- GitNexus matching through existing MCP definitions and Environment selection;
- provider provenance, confidence, freshness and uncertainty fields;
- app-level `InvestigationProviderReport` derived from passive capability inspection;
- Gateway-owner enrichment from existing MCP observations only;
- Agent-facing `investigation_provider_inspect` read-only tool;
- recognized read-only GitNexus tool inventory filtering.

This closure slice added the missing acceptance coverage required by the plan:

- provider absence reports `unconfigured` and preserves static endpoint evidence;
- provider configuration/runtime failure remains local and preserves static endpoint evidence;
- Gateway inspection consumes existing owner-local observation without connecting or refreshing the MCP;
- only recognized read-only provider tools are surfaced as available investigation capabilities.

## Boundary

Passive provider inspection does not:

- connect to GitNexus or another MCP;
- refresh MCP tool inventory;
- invoke provider tools;
- index a repository;
- call application endpoints;
- run verifiers;
- start processes or Agent Runs;
- mutate project files;
- require a writer lease as part of the product inspection path.

The writer lease used during this implementation session was only the ADM Gateway requirement for editing files and executing local validation commands.

## Validation

Passed on the working tree before any commit:

- `go test -count=1 ./internal/app -run TestInvestigationProvider`
- `go test -count=1 ./internal/gateway -run TestGatewayInvestigationProvider`
- `go test -count=1 ./...`
- `go vet ./...`
- `git diff --check`

The first two gates now execute real tests rather than succeeding with `[no tests to run]`.

## Follow-up

14-02 is closed at the implementation/validation level. Per current project sequencing, return to Phase 15 next rather than expanding immediately into the later optional Phase 14 helper slices. GitNexus indexing or active provider calls remain out of scope unless explicitly introduced by a later slice.
