(function (root, factory) {
  const api = factory();
  if (typeof module === 'object' && module.exports) module.exports = api;
  else root.ADMEnvironmentBulk = api;
})(typeof globalThis !== 'undefined' ? globalThis : this, function () {
  function uniqueIDs(values) {
    return [...new Set(Array.isArray(values) ? values : [])]
      .map((value) => String(value || '').trim())
      .filter(Boolean)
      .sort();
  }

  function retainExistingSelection(selectedIDs, entries, key = 'id') {
    const existing = new Set((Array.isArray(entries) ? entries : []).map((entry) => String(entry?.[key] || '').trim()).filter(Boolean));
    return uniqueIDs(selectedIDs).filter((id) => existing.has(id));
  }

  function selectedIDs(environment, kind, scope = 'effective') {
    if (scope === 'explicit') {
      const explicit = kind === 'mcp' ? environment?.explicit_mcp_ids : environment?.explicit_skill_ids;
      if (Array.isArray(explicit)) return new Set(uniqueIDs(explicit));
    }
    if (scope === 'inherited') {
      const inherited = kind === 'mcp' ? environment?.inherited_mcp_ids : environment?.inherited_skill_ids;
      return new Set(uniqueIDs(inherited));
    }
    const values = kind === 'mcp' ? environment?.enabled_mcp_ids : environment?.enabled_skill_ids;
    return new Set(uniqueIDs(values));
  }

  function environmentSummary(environment, mcpIDs, skillIDs) {
    const mcps = uniqueIDs(mcpIDs), skills = uniqueIDs(skillIDs);
    const enabledMCP = selectedIDs(environment, 'mcp'), enabledSkill = selectedIDs(environment, 'skill');
    const explicitMCP = selectedIDs(environment, 'mcp', 'explicit'), explicitSkill = selectedIDs(environment, 'skill', 'explicit');
    const inheritedMCP = selectedIDs(environment, 'mcp', 'inherited'), inheritedSkill = selectedIDs(environment, 'skill', 'inherited');
    return {
      environmentID: String(environment?.environment_id || ''),
      environmentName: String(environment?.name || environment?.environment_id || ''),
      mcpEnabled: mcps.filter((id) => enabledMCP.has(id)).length,
      mcpExplicit: mcps.filter((id) => explicitMCP.has(id)).length,
      mcpInherited: mcps.filter((id) => inheritedMCP.has(id)).length,
      mcpTotal: mcps.length,
      skillEnabled: skills.filter((id) => enabledSkill.has(id)).length,
      skillExplicit: skills.filter((id) => explicitSkill.has(id)).length,
      skillInherited: skills.filter((id) => inheritedSkill.has(id)).length,
      skillTotal: skills.length,
    };
  }

  function buildOperations(environments, environmentIDs, mcpIDs, skillIDs, enabled) {
    const environmentByID = new Map((Array.isArray(environments) ? environments : []).map((environment) => [String(environment?.environment_id || ''), environment]));
    const targets = uniqueIDs(environmentIDs), mcps = uniqueIDs(mcpIDs), skills = uniqueIDs(skillIDs), desired = Boolean(enabled);
    const operations = [];
    for (const environmentID of targets) {
      const environment = environmentByID.get(environmentID);
      if (!environment) continue;
      const currentMCP = selectedIDs(environment, 'mcp', 'explicit'), currentSkill = selectedIDs(environment, 'skill', 'explicit');
      const inheritedMCP = selectedIDs(environment, 'mcp', 'inherited'), inheritedSkill = selectedIDs(environment, 'skill', 'inherited');
      for (const resourceID of mcps) {
        const currentEnabled = currentMCP.has(resourceID), inherited = inheritedMCP.has(resourceID);
        operations.push({environmentID, environmentName: environment.name || environmentID, kind: 'mcp', resourceID, enabled: desired, currentEnabled, inherited, effectiveEnabled: currentEnabled || inherited, noop: currentEnabled === desired});
      }
      for (const resourceID of skills) {
        const currentEnabled = currentSkill.has(resourceID), inherited = inheritedSkill.has(resourceID);
        operations.push({environmentID, environmentName: environment.name || environmentID, kind: 'skill', resourceID, enabled: desired, currentEnabled, inherited, effectiveEnabled: currentEnabled || inherited, noop: currentEnabled === desired});
      }
    }
    return operations;
  }

  function missingSelections(environments, entries, kind) {
    const existing = new Set((Array.isArray(entries) ? entries : []).map((entry) => String(entry?.id || '').trim()).filter(Boolean));
    const operations = [];
    for (const environment of Array.isArray(environments) ? environments : []) {
      const environmentID = String(environment?.environment_id || '').trim();
      if (!environmentID) continue;
      for (const resourceID of selectedIDs(environment, kind, 'explicit')) {
        if (existing.has(resourceID)) continue;
        operations.push({
          scope: 'environment',
          environmentID,
          environmentName: String(environment?.name || environmentID),
          kind,
          resourceID,
          enabled: false,
          currentEnabled: true,
          noop: false,
        });
      }
    }
    return operations.sort((a, b) => a.environmentID.localeCompare(b.environmentID) || a.resourceID.localeCompare(b.resourceID));
  }

  function missingWorkspaceSelections(workspaces, entries, kind) {
    const existing = new Set((Array.isArray(entries) ? entries : []).map((entry) => String(entry?.id || '').trim()).filter(Boolean));
    const operations = [];
    for (const workspace of Array.isArray(workspaces) ? workspaces : []) {
      const workspaceID = String(workspace?.workspace_id || '').trim();
      if (!workspaceID) continue;
      const values = kind === 'mcp' ? workspace?.enabled_mcp_ids : workspace?.enabled_skill_ids;
      for (const resourceID of uniqueIDs(values)) {
        if (existing.has(resourceID)) continue;
        operations.push({
          scope: 'workspace',
          workspaceID,
          workspaceName: String(workspace?.name || workspaceID),
          kind,
          resourceID,
          enabled: false,
          currentEnabled: true,
          noop: false,
        });
      }
    }
    return operations.sort((a, b) => a.workspaceID.localeCompare(b.workspaceID) || a.resourceID.localeCompare(b.resourceID));
  }

  function summarizeResults(results) {
    const list = Array.isArray(results) ? results : [];
    const summary = {total: list.length, changed: 0, unchanged: 0, failed: 0, environments: 0};
    const environmentIDs = new Set();
    for (const result of list) {
      if (result?.environmentID) environmentIDs.add(result.environmentID);
      if (result?.status === 'changed') summary.changed++;
      else if (result?.status === 'unchanged') summary.unchanged++;
      else if (result?.status === 'failed') summary.failed++;
    }
    summary.environments = environmentIDs.size;
    return summary;
  }

  return {uniqueIDs, retainExistingSelection, environmentSummary, buildOperations, missingSelections, missingWorkspaceSelections, summarizeResults};
});
