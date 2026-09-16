(function attachProjectPages(global) {
  function safeArray(value) {
    return Array.isArray(value) ? value : [];
  }

  function normalizedQuery(value) {
    return String(value || '').trim().toLowerCase();
  }

  function workspaceEnvironmentCounts(workspaces, environments) {
    const counts = {};
    for (const workspace of safeArray(workspaces)) {
      const id = String(workspace?.workspace_id || '');
      if (id) counts[id] = 0;
    }
    for (const environment of safeArray(environments)) {
      const id = String(environment?.workspace_id || '');
      if (id && Object.prototype.hasOwnProperty.call(counts, id)) counts[id]++;
    }
    return counts;
  }

  function workspaceNames(workspaces) {
    const names = {};
    for (const workspace of safeArray(workspaces)) {
      const id = String(workspace?.workspace_id || '');
      if (id) names[id] = String(workspace?.name || id);
    }
    return names;
  }

  function workspaceListModel(workspaces, environments, query) {
    const source = safeArray(workspaces);
    const counts = workspaceEnvironmentCounts(source, environments);
    const needle = normalizedQuery(query);
    const items = source.filter((workspace) => {
      if (!needle) return true;
      return [workspace?.name, workspace?.workspace_id, workspace?.path]
        .filter(Boolean)
        .join(' ')
        .toLowerCase()
        .includes(needle);
    });
    return {items, total: source.length, visible: items.length, environmentCounts: counts};
  }

  function environmentListModel(environments, workspaces, options = {}) {
    const source = safeArray(environments);
    const names = workspaceNames(workspaces);
    const workspaceIDs = new Set(safeArray(workspaces).map((workspace) => String(workspace?.workspace_id || '')).filter(Boolean));
    const workspaceID = String(options.workspaceID || '');
    const needle = normalizedQuery(options.query);
    const workspaceFilterValid = !workspaceID || workspaceIDs.has(workspaceID);
    const items = source.filter((environment) => {
      if (workspaceID && environment?.workspace_id !== workspaceID) return false;
      if (!needle) return true;
      return [environment?.name, environment?.environment_id, environment?.root, environment?.workspace_id, names[environment?.workspace_id]]
        .filter(Boolean)
        .join(' ')
        .toLowerCase()
        .includes(needle);
    });
    return {items, total: source.length, visible: items.length, workspaceNames: names, workspaceFilterValid};
  }

  const api = {normalizedQuery, workspaceEnvironmentCounts, workspaceNames, workspaceListModel, environmentListModel};
  global.ADMProjectPages = api;
  if (typeof module !== 'undefined' && module.exports) module.exports = api;
})(typeof window !== 'undefined' ? window : globalThis);
