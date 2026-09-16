# Phase 13 Validation — Environment Capability Diagnostics

## Validation principle

The report is accepted only if it agrees with the real operation boundaries and remains useful when optional capabilities are broken. It must not be a decorative summary assembled from catalog metadata.

## Required proof matrix

### P1 — healthy Environment report

For a normal non-Git Environment with files/search, one allowlisted executable, one verifier, one enabled healthy MCP and one enabled healthy Skill, prove one capability report contains structured facts for each relevant capability/resource.

Facts include state, stable reason code where applicable, writer requirement, and sanitized evidence.

### P2 — no Git is local

For a plain directory:

- file read/search/write facts remain usable/available subject to writer;
- Git facts report unavailable/not-repository;
- MCP/Skill facts are still returned;
- top-level capability report succeeds.

### P3 — broken managed root does not erase global diagnostics

Tamper/move a managed worktree so Runtime validation fails.

Prove root-dependent file/exec/Git/stdio-MCP facts become unavailable with managed-root evidence while global selection facts and remote HTTP MCP/Skill diagnostics are still returned where their own checks permit.

### P4 — optional MCP failures stay per MCP

Enable one healthy MCP and one broken MCP.

Prove:

- healthy MCP fact stays available;
- broken MCP fact carries Phase 11 reason/stage;
- file/Skill/verifier facts remain present;
- capability inspection does not automatically reconnect/call tools unless an explicit Phase 11 refresh was invoked separately.

### P5 — optional Skill failures stay per Skill

Enable one healthy and one broken/missing Skill selection.

Prove Phase 12 availability states appear directly and unrelated facts remain usable.

### P6 — writer authority is observable, not acquired

With no writer, writer owned by A, and expired/reacquired writer cases, prove mutation capabilities report `requires_writer` and current lease evidence accurately.

Capability inspection itself must not acquire, renew, release or steal the writer lease.

### P7 — verifier status without execution

Configure enabled, disabled and invalid/unavailable verifier definitions. Prove capability facts identify configuration/authority state without running the verifier helper process.

Use a helper side-effect sentinel so accidental execution fails the test.

### P8 — no secret/private Memory leakage

Plant sentinel values in MCP activation secret refs and Environment-private Memory. Prove neither value appears anywhere in the capability report, error facts or evidence.

### P9 — observation provenance/timestamps are honest

For owner-local MCP/process/run observations:

- current observation includes its origin/timestamp;
- after owner restart, stale old observation is absent rather than silently reused;
- absence of observation is not mislabeled as healthy or failed.

## Automated gates

- app-level fact aggregation tests;
- Gateway owner-enrichment tests;
- real non-Git Environment acceptance;
- broken-MCP/broken-Skill/tampered-worktree negative matrix;
- secret/Memory sentinel tests;
- no-side-effect verifier/MCP inspection tests;
- regression MCP/Skill/files/process/run/isolation;
- `go test ./...` or equivalent complete package grouping;
- `go vet ./...`;
- `git diff --check`.

## Review focus

Integration review must prove that capability reporting reuses real validators/status contracts, does not introduce side-effect probes by default, and does not make any optional capability a global Environment prerequisite.
