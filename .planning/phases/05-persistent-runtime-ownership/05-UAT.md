# Phase 5 UAT — Persistent Runtime Ownership

**Status:** passed automatically; no human UAT required
**Date:** 2026-09-07

Phase 5 is runtime/lifecycle behavior with machine-observable acceptance. All eight validation items are covered by deterministic tests plus an independent private-Gateway dogfood run.

## Automated coverage

1. Same Gateway owner identity is visible to independent clients — covered.
2. External MCP session reuse under one owner — covered by deterministic connect-count tests; real MCP call path covered independently.
3. Desired state persists while observed owner/session state does not — covered by private `state.json` inspection.
4. Restart creates a new owner and reconciles desired state — covered by cross-process test and independent detached Gateway run.
5. Dead upstream cannot remain healthy — covered; `connection_refused` and zero owned sessions observed.
6. Disable/remove/clean shutdown close owned resources — covered by fake-session close assertions, owner-bound shutdown tests, and independent CLI stop.
7. Optional-capability/non-Git behavior remains green — covered by the full regression suite.
8. No Phase 6 dev-process/log/port API was introduced — covered by added-line diff review.

No manual visual judgment or Desktop interaction is necessary. No OpenCode, CodeBuddy, GSD LLM planner/checker/verifier, or other LLM subprocess was invoked.

This UAT artifact records local acceptance only. It does not advance planning state to Phase 6 and does not claim that `phase uat-passed 05` was run.
