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

  function compareNames(a, b) {
    return String(a?.name || a?.environment_id || a?.workspace_id || '').localeCompare(
      String(b?.name || b?.environment_id || b?.workspace_id || ''), 'zh-CN',
      {numeric: true, sensitivity: 'base'}
    );
  }

  function timestamp(value) {
    const result = Date.parse(String(value || ''));
    return Number.isFinite(result) ? result : 0;
  }

  function workspaceListModel(workspaces, environments, queryOrOptions = '') {
    const options = typeof queryOrOptions === 'string' ? {query: queryOrOptions} : (queryOrOptions || {});
    const source = safeArray(workspaces);
    const counts = workspaceEnvironmentCounts(source, environments);
    const needle = normalizedQuery(options.query);
    const occupancy = options.occupancy || 'all';
    const sort = options.sort || 'name';
    const items = source.filter((workspace) => {
      const count = counts[workspace?.workspace_id] || 0;
      if (occupancy === 'with' && !count) return false;
      if (occupancy === 'empty' && count) return false;
      if (!needle) return true;
      return [workspace?.name, workspace?.workspace_id, workspace?.path]
        .filter(Boolean).join(' ').toLowerCase().includes(needle);
    }).sort((a, b) => {
      const difference = (counts[b.workspace_id] || 0) - (counts[a.workspace_id] || 0);
      if (sort === 'count-desc' && difference) return difference;
      if (sort === 'count-asc' && difference) return -difference;
      return compareNames(a, b);
    });
    return {items, total: source.length, visible: items.length, environmentCounts: counts};
  }

  function retentionMatches(environment, filter, now) {
    if (!filter || filter === 'all') return true;
    const retention = environment?.retention || {};
    const temporary = retention.persistence === 'temporary';
    const expiry = timestamp(retention.expires_at);
    if (filter === 'durable') return retention.persistence === 'durable';
    if (filter === 'temporary') return temporary;
    if (!temporary) return false;
    if (filter === 'noexpiry') return !expiry;
    if (filter === 'expired') return Boolean(expiry && expiry <= now);
    if (filter === 'due7') return Boolean(expiry && expiry > now && expiry <= now + 7 * 24 * 60 * 60 * 1000);
    return true;
  }

  function environmentListModel(environments, workspaces, options = {}) {
    const source = safeArray(environments);
    const names = workspaceNames(workspaces);
    const workspaceIDs = new Set(safeArray(workspaces).map((workspace) => String(workspace?.workspace_id || '')).filter(Boolean));
    const workspaceID = String(options.workspaceID || '');
    const needle = normalizedQuery(options.query);
    const retentionFilter = options.retention || 'all';
    const kindFilter = options.kind || 'all';
    const managedIDs = new Set(safeArray(options.managedIDs));
    const writerFilter = options.writer || 'all';
    const sort = options.sort || 'recent';
    const now = Number.isFinite(options.now) ? options.now : Date.now();
    const workspaceFilterValid = !workspaceID || workspaceIDs.has(workspaceID);
    const items = source.filter((environment) => {
      if (workspaceID && environment?.workspace_id !== workspaceID) return false;
      if (!retentionMatches(environment, retentionFilter, now)) return false;
      if (kindFilter === 'managed' && !managedIDs.has(environment?.environment_id)) return false;
      if (kindFilter === 'ordinary' && managedIDs.has(environment?.environment_id)) return false;
      if (writerFilter === 'active' && !environment?.writer) return false;
      if (writerFilter === 'none' && environment?.writer) return false;
      if (!needle) return true;
      return [environment?.name, environment?.environment_id, environment?.root, environment?.workspace_id, names[environment?.workspace_id], environment?.retention?.owner_id]
        .filter(Boolean).join(' ').toLowerCase().includes(needle);
    }).sort((a, b) => {
      if (sort === 'name') return compareNames(a, b);
      if (sort === 'expiry') {
        const aExpiry = timestamp(a?.retention?.expires_at) || Number.MAX_SAFE_INTEGER;
        const bExpiry = timestamp(b?.retention?.expires_at) || Number.MAX_SAFE_INTEGER;
        return aExpiry - bExpiry || compareNames(a, b);
      }
      const field = sort === 'created' ? 'created_at' : 'last_activity_at';
      return timestamp(b?.[field]) - timestamp(a?.[field]) || compareNames(a, b);
    });
    return {items, total: source.length, visible: items.length, workspaceNames: names, workspaceFilterValid};
  }

  const api = {normalizedQuery, workspaceEnvironmentCounts, workspaceNames, workspaceListModel, environmentListModel, retentionMatches};
  global.ADMProjectPages = api;
  if (typeof module !== 'undefined' && module.exports) module.exports = api;
})(typeof window !== 'undefined' ? window : globalThis);
