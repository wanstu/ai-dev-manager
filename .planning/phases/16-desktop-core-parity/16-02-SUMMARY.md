# 16-02 Summary — Desktop MCP and Skill Visual Management UI

## Result

Phase 16 Plan 16-02 implemented the first release-candidate Desktop user journey: first-class visual management for MCP and Skill.

Implementation commits:

- `374fa12` — `feat(16-02): expose MCP Skill desktop management APIs`
- `74be256` — `feat(16-02): add MCP Skill desktop management UI`

## Desktop adapter additions

Desktop now exposes existing Management/Core operations without adding Desktop-only state or authorization:

- MCP import preview/apply through typed `MCPImportInput`;
- Skill source add/list/refresh/remove;
- Environment Skill availability list/inspect;
- existing typed MCP add, Environment selection, MCP probe and Skill defaults remain authoritative.

Adapter tests prove:

- MCP import preview does not expose the literal credential supplied by the test fixture;
- MCP import apply uses existing Core atomic import behavior;
- Skill source add/list/refresh/remove delegates through Core and discovers the real `SKILL.md` artifact.

## MCP UI

Desktop now has a first-class MCP manager with:

- explicit Management Environment context;
- separate global definition, new-Environment default and current-Environment enabled state;
- typed Streamable HTTP / stdio add form;
- secret/environment reference mapping inputs rather than literal credential fields;
- health policy basics;
- explicit MCP health probe action;
- no automatic probe during page refresh;
- runtime probe state separated from side-effect-free static capability/config state;
- import JSON/JSONC preview before apply;
- candidate warnings/errors/reference requirements;
- explicit global-delete warning that Environment references are not silently rewritten.

## Skill UI

Desktop now has a first-class Skill manager with:

- explicit Skill source add flow;
- source support roots and default inclusion;
- add + explicit Core refresh;
- source refresh status/error display;
- source refresh/remove actions;
- discovered Skill list;
- new-Environment default state separated from current-Environment selection;
- Environment availability state/reason display;
- source-managed Skills visually distinguished from legacy entries;
- destructive source removal warns that Environment references may become unresolved.

## Environment detail

Environment detail keeps the existing explicit selection controls and now adds state/reason information from:

- side-effect-free CapabilityReport facts for MCP configuration;
- Environment Skill availability diagnostics.

Private Memory remains explicit-load only.

## UX and safety decisions

- Global definition/source deletion and Environment enable/disable use different labels and confirmation copy.
- MCP refresh does not trigger network probes automatically.
- Import cannot apply until preview succeeds without candidate errors.
- Desktop does not execute MCP tools.
- Desktop does not resolve or display secret values.
- Skill refresh failures remain local to the source and do not create Desktop-only recovery state.

## Verification

Focused:

- `node --check cmd/ai-dev-manager-desktop/frontend/app.js` — passed.
- `go test -count=1 ./cmd/ai-dev-manager-desktop ./internal/desktop ./internal/management` — passed.

Repository gates:

- `go test -count=1 ./...` — passed.
- `go vet ./...` — passed.
- `go build ./cmd/ai-dev-manager ./cmd/ai-dev-manager-desktop` — passed.
- `git diff --check` — passed; Windows LF→CRLF warnings only.

## Remaining RC work

16-02 automated implementation gates are complete, but the RC manual GUI smoke checklist still needs to be run in a real Wails Desktop session before declaring 1.0 RC.

The next Phase 16 slice should prioritize the remaining daily-use RC gaps after MCP/Skill UI, especially structured capability visibility and whichever verifier/process/run surfaces prove necessary in manual dogfood.
