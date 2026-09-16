# Phase 16 UI Plan — MCP and Skill Visual Management

## Design intent

MCP and Skill management should feel like a control panel, not a JSON editor. The user should see what is configured, where it is enabled, whether it works, and what to do next.

This UI plan is intentionally scoped to Desktop RC. It does not change Core semantics and does not introduce Desktop-only state.

## Primary navigation model

Use three related views:

1. **MCPs** — global MCP definitions, import/add flows, defaults, health and Environment selection.
2. **Skills** — Skill sources, refresh results, discovered Skills, defaults, availability and Environment selection.
3. **Environment detail** — per-Environment MCP/Skill enablement and reason-focused diagnostics.

The current dashboard can remain as the home view, but MCPs and Skills should become prominent first-class sections or large tabs.

## Shared visual patterns

### State badges

Use the same visual vocabulary everywhere:

| State | Meaning | Example copy |
|---|---|---|
| Available / healthy | Usable now or statically valid | `Available` / `Healthy` |
| Degraded | Usable partially or missing owner observation | `Degraded` |
| Unavailable / error | Cannot be used | `Unavailable` / `Error` |
| Disabled | Intentionally off | `Disabled` |
| Unconfigured | Definition/source incomplete | `Unconfigured` |
| Unresolved | Selected ID or reference cannot be resolved | `Unresolved` |

Badges must be backed by Core fields, not guessed from UI state.

### Definition vs selection labels

Use precise labels:

- `Global definition` for MCP catalog entries.
- `Skill source` for roots that discover Skills.
- `Default for new Environments` for default include flags.
- `Enabled in this Environment` for current Environment selection.
- `Remove global definition/source` for destructive catalog/source removal.

Avoid vague labels like `Enabled` when it is unclear whether it means default include or selected Environment enablement.

### Detail drawers

MCP and Skill rows should open a detail drawer or side panel instead of expanding into very large inline cards. Details should be organized into sections:

- Status
- Configuration/source
- Environment selection
- Diagnostics/evidence
- Actions

## MCP screen

### Header

- Title: `MCPs`
- Subtitle: `Configure external tools and choose which Environments can use them.`
- Primary actions:
  - `Add MCP`
  - `Import config`
- Secondary actions:
  - `Probe selected Environment`
  - `Refresh view`

### Summary metrics

- Total MCPs
- Enabled by default
- Enabled in selected Environment
- Needs attention

### Filters

- State: all / healthy / degraded / unavailable / disabled / unconfigured
- Transport: all / streamable-http / stdio
- Environment: selected Environment dropdown
- Text search: name/id/transport

### List columns or card fields

- State badge
- Name + ID
- Transport
- Configuration summary:
  - HTTP: endpoint configured, not raw secret headers
  - stdio: executable and arg count, not resolved env secret values
- Default for new Environments toggle
- Enabled in selected Environment toggle
- Last observation or `not observed`
- Actions: inspect, probe, remove

### Add MCP flow

Transport-specific form:

1. Common:
   - Name
   - Default for new Environments
2. Streamable HTTP:
   - Endpoint
   - Optional header reference keys
   - Health-check policy basics if Core supports it in management surface
3. Stdio:
   - Executable
   - Args
   - Env reference keys

Validation should happen before submit when possible, and Core errors should be displayed without rephrasing away important detail.

### Import config flow

A two-step wizard:

1. Preview:
   - paste JSON/JSONC content or select supported source format;
   - show candidates, warnings, errors and reference requirements;
   - disable apply when preview has blocking errors.
2. Apply:
   - confirm created definitions;
   - show reference requirements that must be configured externally;
   - never enable imported MCPs in existing Environments unless Core explicitly does that, which Phase 11 says it does not.

### MCP detail drawer

Sections:

- Summary: name, ID, transport, default include.
- Environment selection: selected Environment enabled/disabled toggle.
- Desired config: sanitized endpoint/executable/config facts.
- Health/status: state, failure stage, error kind, last check, last healthy, consecutive failures.
- Capability fact: `mcp/<id>` state/reason/message/evidence for selected Environment.
- Actions: probe, toggle default, toggle Environment selection, remove global definition.

## Skill screen

### Header

- Title: `Skills`
- Subtitle: `Manage Skill sources and choose which Environments can use discovered Skills.`
- Primary actions:
  - `Add Skill source`
- Secondary actions:
  - `Refresh selected source`
  - `Refresh all sources`
  - `Refresh view`

### Summary metrics

- Total sources
- Discovered Skills
- Enabled in selected Environment
- Needs attention

### Source panel

Fields:

- Source root
- Support roots count
- Default include flag
- Last refresh time/result
- Discovered artifact count
- Actions: refresh, inspect refresh result, remove source

### Skill list fields

- Availability badge
- Skill name + ID
- Source root / artifact path
- Support file count
- Default for new Environments if represented at catalog level
- Enabled in selected Environment toggle
- Reason/message for unavailable/degraded/unconfigured
- Actions: inspect, list support files, remove global Skill when safe

### Add Skill source flow

Form:

- Source root
- Support roots, one per line or addable chips
- Default include checkbox

After submit:

- show refresh result;
- show discovered Skills;
- show per-artifact errors without hiding unrelated valid Skills.

### Skill detail drawer

Sections:

- Summary: name, ID, availability badge.
- Source: root, artifact path, support roots.
- Environment selection: selected Environment enabled/disabled toggle.
- Availability diagnostics: state, reason, message.
- Support inventory: bounded list if already available through Core; support reading can be explicit and post-RC if it makes the UI too large.
- Capability fact: `skill/<id>` state/reason/message/evidence for selected Environment.
- Actions: toggle Environment selection, refresh source, remove global Skill/source where applicable.

## Environment detail integration

The Environment detail panel should add clear tabs or sections:

1. `Capabilities`
2. `MCPs`
3. `Skills`
4. `Memory`

For 16-02, the MCPs and Skills sections are the priority. Each section should show:

- enabled entries;
- available/unavailable/degraded state;
- reason/message;
- direct action to enable/disable;
- link/action to open global MCP/Skill detail.

## Empty states

### No MCPs

Copy:

`No MCPs configured yet. Add one manually or import a config from OpenCode, Claude Code, Codex plugin MCP JSON, WorkBuddy/CodeBuddy, or MCPHub.`

Actions:

- Add MCP
- Import config

### No Skills

Copy:

`No Skill sources configured yet. Add a source root containing SKILL.md files.`

Action:

- Add Skill source

### Selected Environment has none enabled

Copy:

`This Environment has no MCPs/Skills enabled. Toggle entries below to make them available here.`

## Safety copy

### Remove global MCP

`This removes the global MCP definition from ADM. It does not edit project files. Existing Environment references may become unresolved.`

### Remove Skill source

`This removes the Skill source from ADM. It does not delete files from disk. Skills discovered only from this source may no longer be available.`

### Toggle default include

`This affects newly created Environments only. Existing Environments are not changed.`

### Toggle Environment selection

`This affects only the selected Environment.`

## RC acceptance checklist

A 16-02 implementation is acceptable for RC when Desktop supports these happy paths:

1. Add one HTTP MCP and see it in the MCP list.
2. Import an MCP config, preview candidates/errors, then apply valid definitions.
3. Toggle an MCP default without changing existing Environment selections.
4. Toggle an MCP for one Environment and see health/capability status.
5. Add one Skill source and see discovered Skills.
6. Refresh a Skill source and see success/failure results.
7. Toggle a Skill for one Environment and see availability/capability status.
8. Inspect a broken MCP/Skill and see a structured reason.
9. Confirm private Memory values and resolved secret values are not displayed in normal MCP/Skill views.

## Implementation notes

Prefer small, testable UI helpers over a single large script rewrite:

- normalize state badge rendering;
- normalize selected Environment lookup;
- isolate MCP import wizard state;
- isolate Skill source refresh state;
- keep destructive confirmation copy close to actions;
- keep frontend dependent only on Desktop adapter methods backed by Management/Core.
