# 16-01 Summary — Desktop Parity Inventory, GitHub CI Baseline and RC Gate

## Outcome

Phase 16 was moved ahead of the remaining Phase 14/15/17 work as the immediate 1.0 RC path. The decision is that GitNexus/provider investigation, temporary resource lifecycle and distribution polish are useful improvements, but not base-function blockers by default.

This slice added the RC infrastructure and planning baseline:

- GitHub Actions CI workflow for automated test/vet/build.
- Desktop Core parity matrix.
- 1.0 RC gate and smoke checklist.
- Phase 16 context and validation docs.
- ROADMAP update marking Phase 16 as the next priority.

## CI baseline

Added `.github/workflows/ci.yml`.

Workflow triggers:

- `push` to `master`
- `pull_request`
- `workflow_dispatch`

Workflow security posture:

- `permissions: contents: read`
- no deploy
- no release publishing
- no push
- no secret requirement

Workflow jobs:

1. `windows-test-vet-build`
   - checkout
   - setup Go from `go.mod`
   - `go mod download`
   - `go test -count=1 ./...`
   - `go vet ./...`
   - build Windows CLI binary
   - build Windows Desktop smoke binary
   - upload short-retention artifacts
2. `cli-cross-build`
   - build CLI artifact on Linux, macOS and Windows
   - upload short-retention artifacts

Desktop installer/package/signing remains Phase 17 by default and is not an RC blocker.

## Desktop parity inventory

Added `16-PARITY-MATRIX.md`.

Initial RC blocker/target findings:

- P0: Gateway lifecycle, Workspace/Environment lifecycle, capability report display, exec allowlist, Memory privacy, CI auto build.
- P1: MCP typed/import/status coverage, Skill source/availability coverage, verifier/process/run visibility.
- P2: full file manager/editor, Git/worktree UI, GitNexus/provider integration, temporary resource cleanup, installer/tray/autostart/updater/signing.

## RC gate

Added `16-RC-GATE.md`.

RC means a local dogfood release candidate, not a public packaged release. Required gates include local tests/vet/diff, CI workflow presence, binary artifacts, Desktop smoke checklist, known limitations and no secret/private Memory leaks.

## Validation performed

Local validation before commit:

- Windows CLI smoke build passed:
  - `go build -trimpath -o dist/ai-dev-manager-v2-windows-amd64.exe ./cmd/ai-dev-manager`
- Windows Desktop smoke build passed:
  - `go build -trimpath -o dist/ai-dev-manager-v2-desktop-windows-amd64.exe ./cmd/ai-dev-manager-desktop`
- Full tests passed:
  - `go test -count=1 ./...`
- Vet passed:
  - `go vet ./...`
- Diff check passed:
  - `git diff --check`
  - only Windows LF-to-CRLF warnings appeared

## Commit

Implementation/planning baseline commit:

- `6d51e6d` — `ci(16): add GitHub Actions RC build baseline`

## Next action

Continue Phase 16 on `master` with Desktop RC blocker implementation, starting from the parity matrix:

1. render structured CapabilityReport facts in Desktop Environment detail;
2. add or document minimal MCP import/status path;
3. add or document minimal Skill source/availability path;
4. add or document verifier/process/run visibility;
5. run RC smoke checklist.

Do not resume Phase 14/15/17 unless a concrete RC blocker proves one of those items is required before 1.0 RC.
