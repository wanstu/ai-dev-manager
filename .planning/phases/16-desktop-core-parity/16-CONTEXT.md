# Phase 16 Context — Desktop Core Parity + 1.0 RC Readiness

## Why Phase 16 moves ahead

After Phase 13, ADM Core has the foundation needed for a usable release candidate: Workspace/Environment lifecycle, safe files/search/write/edit/delete, allowlisted exec, verifier, process/run lifecycle, MCP runtime/import/health, Skill source/availability, optional Git/worktree isolation and canonical capability diagnostics.

Phase 14 and Phase 15 remain useful, but they are primarily usability/depth improvements rather than base capability blockers. Distribution polish also should not block a local 1.0 RC. The next release-critical need is a human-manageable Desktop surface over the validated Core, plus CI that proves the project builds automatically.

## Product boundary

Desktop is a management surface over Core. It must not create Desktop-only state, authorization rules or lifecycle semantics.

Desktop may:

- display Core state and diagnostics;
- call existing application services / Gateway-compatible operations;
- provide safer human workflows for configuration, inspection and local smoke testing;
- expose known limitations for RC.

Desktop must not:

- interpret `.planning` or orchestrate tasks;
- add Planner/Executor/Reviewer or GSD phase semantics;
- bypass writer leases, executable allowlists, Environment selection or capability diagnostics;
- leak private Memory values or secret-backed MCP configuration values;
- implement cleanup semantics that Core does not own.

## CI boundary

GitHub Actions is release infrastructure. The RC baseline may run tests, vet and builds, and upload short-retention artifacts. It must not publish releases, push commits, deploy, or require secrets until an explicit release/publishing phase.

The CI baseline should verify Windows daily-use behavior first because ADM Desktop and local dogfood currently target Windows. CLI artifacts for Linux/macOS are useful, but Desktop installer/package work remains Phase 17 by default.

## First UI priority

16-02 starts with MCP and Skill visual management because these are the main user-configured Core capabilities needed for a credible local 1.0 RC.

A user should be able to configure/import MCPs, manage Skill sources, enable/disable MCPs and Skills for one Environment, and see health/availability reasons without using the CLI. CapabilityReport rendering should support this journey by explaining MCP/Skill availability and selected Environment problems, rather than appearing as a disconnected diagnostic dump.

This moves MCP/Skill Desktop usability ahead of broader verifier/process/run dashboards and ahead of Phase 14/15/17 work.

## RC framing

1.0 RC means a local release candidate suitable for dogfooding. It does not require installer/tray/autostart/updater/signing. Those remain Phase 17 post-RC unless daily use proves they are necessary.

The RC gate is functional parity, safety, smokeability, automated CI/build evidence and honest known limitations, not broad feature expansion.
