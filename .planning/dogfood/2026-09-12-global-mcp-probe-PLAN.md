# Global MCP management probe

User correction: MCP availability belongs to the global definition; Environment selection only grants Agent usage. Baseline be591d4. Phase 19 remains unstarted.

Requirements: ADM-CORE-012/018, ADM-MGMT-001, ADM-DESKTOP-001. Preserve ADM-CORE-007 executable allowlist and environment_mcp_* authorization. This explicitly authorized correction supersedes earlier management-probe Environment coupling.

Plan:
1. Add Admin-only mcp_probe using global catalog resolution, bounded transient connection/ListTools/close, sanitized diagnostics. Reuse reference resolution/transport policy. Stdio uses an isolated temporary working directory plus the existing global allowlist, with denial recording; no Environment creation or selection mutation. Relative project paths are not inferred.
2. Add shared management/client/Desktop bridge. Global UI state is scoped to ADM connection and MCP definition, survives Environment changes, and invalidates on definition edits/removal/profile changes. Ignore outdated/in-flight superseded responses. Keep Environment enable controls separate.
3. Test zero-Environment global HTTP and stdio probes, disabled Agent access, denylist/secret failures, Admin-vs-Agent exposure and Desktop no-Environment/disabled/transition behavior. Run relevant Go, full test/vet, browser and Wails build.
4. Update contract/evidence/STATE; commit on master and release writer. No push/tag/release.

Non-goals: automatic probes/reconnect, Agent authorization changes, persistent probe state, MCP business tool calls, Phase 19. New prerequisites: none.

Status: complete. Automated acceptance and builds passed; see 2026-09-12-global-mcp-probe-SUMMARY.md. Native window acceptance and updating the active Gateway are separate operational handoff steps.
