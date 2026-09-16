# AI Dev Manager V2 — v1.0.0-rc.3

## Why RC3 exists

RC2 dogfood exposed one MCP import usability blocker: a standard top-level `mcpServers` wrapper without vendor-specific markers was rejected as `ambiguous_format` because the same wrapper shape is accepted by several supported adapters.

RC3 makes generic `mcpServers` a first-class import shape instead of forcing the user to guess a vendor format.

## Generic mcpServers import

- Added canonical format `generic-mcpservers`.
- `auto` now detects top-level `mcpServers` without vendor-specific markers as `generic-mcpservers`.
- WorkBuddy/CodeBuddy-specific markers still take precedence and select the WorkBuddy adapter.
- Direct unwrapped server maps remain handled by the existing Codex-plugin-compatible adapter.
- The generic adapter uses the existing standard stdio / Streamable HTTP normalization rules; no new runtime transport semantics were introduced.
- Desktop import format picker now exposes `Generic mcpServers` explicitly as well as `auto`.
- CLI import help documents `generic-mcpservers`.

## Credential and environment-value safety

Generic imports keep the existing ADM credential boundary:

- literal `env` / header values from imported source content are not persisted in MCP definitions;
- they are converted to generated environment-reference requirements;
- preview output contains the generated reference names but not the original literal values;
- Desktop preview now lists each required reference so the operator knows which ADM process environment variables must be provided before the imported MCP can activate;
- unsupported source-only fields such as vendor ownership/options/visibility extensions remain warnings rather than silently becoming ADM runtime semantics.

## Validation

- `node --check cmd/ai-dev-manager-desktop/frontend/app.js` passed.
- Focused app/CLI/Desktop/Admin-MCP tests passed.
- Admin MCP acceptance verifies `mcp_import_preview` accepts generic `mcpServers` with `format=auto` and does not expose imported literal env values.
- `go test -count=1 ./...` passed; Gateway suite completed in 83.271s.
- `go vet ./...` passed.
- `git diff --check` passed with only Windows line-ending warnings.
- `scripts/build-rc.ps1 -Version v1.0.0-rc.3` produced the versioned CLI and real Wails Desktop artifacts.
- Exact RC3 Desktop artifact remained running after a 3-second launch smoke.

## Note about Windows CLI inline JSON

A release smoke attempt that passed JSON directly through Windows PowerShell to the native CLI hit PowerShell/native argument quoting and reached the importer as invalid JSON. This is not the generic-import regression: the same Admin MCP path is covered by the passing acceptance test and Desktop does not use native-shell JSON quoting. A future CLI convenience option such as import-from-file can be considered separately if dogfood shows it is needed.

## SHA-256

- `6fefc13802a5562109f23c1fe910e603e762220a72a14622019cbf81f74f8387` — `ai-dev-manager-v2-v1.0.0-rc.3-windows-amd64.exe`
- `b03825b21f80966f401ff23f0c58c1bb31b43e40541e5d88e7040705d831c93c` — `ai-dev-manager-v2-desktop-v1.0.0-rc.3-windows-amd64.exe`
