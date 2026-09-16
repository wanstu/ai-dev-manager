# Global MCP probe correction

Baseline: be591d4. User explicitly corrected the management boundary: a global MCP definition can be probed without selecting or enabling an Environment.

Delivered:
- Admin-only mcp_probe with shared catalog reference resolution and bounded transient connection/tool-list lifecycle. No business tool calls, persistent health or implicit Environment creation.
- Stdio uses existing global executable allowlist, denial observations and an ADM-created temporary working directory cleaned after the session; no project cwd is inferred. Agent environment_mcp_* authorization is unchanged.
- Shared Admin client/management/Desktop bridge; CLI adm mcp probe --id MCP_ID. Existing mcp status remains Environment-specific runtime diagnostics.
- Desktop explicit global probe remains available with no Environment or disabled selection. Results are keyed by definition and connection, survive Environment switching, and reject late results after definition changes/removal or connection switching. Duplicate in-flight probes are disabled; request failures replace prior healthy evidence.
- Product contract and CLI documentation updated.

Validation:
- App tests passed in full run: zero-Environment HTTP health, desired-state unchanged, disabled Agent activation, missing IDs, secret redaction, missing references, timeout, real stdio test subprocess, allowlist denial and temporary-directory cleanup.
- Admin-only Gateway focused test run_3a916ba2883b946b PASS: global probe succeeds, Agent inventory/call excludes it, disabled environment_mcp_tools still denied.
- Browser run_332e94fe1d8355d0 PASS: 153 checks x 3, including no-Environment/disabled probes, Environment transitions, definition/profile invalidation and prior layout/scope interactions.
- vet run_89539e70556df480 PASS.
- Initial Go build caught a CatalogInput typo; corrected to CatalogIDInput. Initial full suite run_62b43cd3d4a983cf failed only the stale frontend text marker (including nested verifier copy); changed expected text from runtime to global probe. Command-package rerun run_57939897fe8ee17d PASS.
- Final full Go run_0d807dad79377756: PASS, including nested verifier repository-copy acceptance.
- Wails build run_f3ee50c6a27dc6db PASS; dist/adm-desktop-global-mcp-probe-windows-amd64.exe, SHA-256 561A09C333BD3BA9FE38C52D12FC194B7ECBC5FDDA72E562484AE8983B65904A.
- CLI build run_09c0269551f02759: PASS; dist/adm-global-mcp-probe-windows-amd64.exe; SHA-256 C6F4C769F31BBD329EDAE4AA2B71EE5EFFE436420212A46FCA4AD0030A1AA101.
- git diff --check PASS; staged check before commit.

Operational handoff: both Desktop and the connected ADM Gateway must use the updated build; the running pjadm Gateway was not restarted during this development session. An old Gateway has no mcp_probe tool and will return an explicit error. Native window acceptance is pending, not claimed by browser/build evidence. No push/tag/release. Phase 19 remains unstarted.
