# MCP probe badge consistency fix

Baseline: 5ca528f. User screenshot shows unresolved_secret_reference alongside two configured badges. Requirement: truthful global MCP diagnostics (ADM-CORE-018/020, ADM-DESKTOP-001). Existing error_kind must take precedence over the configured lifecycle state in presentation. No backend/authorization changes or new prerequisites.

Plan: map unresolved references to configuration unavailable with a specific label, map error-bearing probes to a blocked/error badge, preserve configured definitions for transport failures, and verify error-to-healthy recovery and issue filtering using production-browser fixtures. Build Wails, record evidence, commit/release writer. No push/tag/release. Phase 19 remains unstarted.

Status: complete; native window visual acceptance pending.


Evidence (2026-09-12):
- Browser run_9b55aa7e0e101775 PASS, 158 checks x 3 (1120x760, 820x560, 125% scale). New checks cover unavailable configuration badge, blocked/error probe badge, matching error toast, issue filter and healthy recovery. Initial run_f08c2d21549506a9 exposed test overlap with the preceding Environment-switch toast; the fixture now lets that preceding operation settle before the probe check.
- Desktop Go run_5ba5a925268b56e8 PASS.
- Wails run_eec60c6b96e4d64f PASS; dist/adm-desktop-mcp-badge-fix-windows-amd64.exe; SHA-256 57D4EF6862F0BF1556D21C833D08601BC94059FDA93509CE3A6BC0B76F6EEBC0.
- git diff --check PASS. No Go/backend change; full Go/vet were not repeated for this frontend mapping change. No runtime secret values read or changed.
- Error-kind takes precedence over lifecycle configured state. Unresolved references show configuration unavailable / reference unresolved and global probe blocked; generic error-kind shows error. A configured probe with no terminal health now says incomplete rather than configured.
