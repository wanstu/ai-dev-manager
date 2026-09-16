(function (root, factory) {
  const api = factory();
  if (typeof module === 'object' && module.exports) module.exports = api;
  else root.ADMSkillBulk = api;
})(typeof globalThis !== 'undefined' ? globalThis : this, function () {
  const CLEANUP_STATES = new Set([
    'unconfigured',
    'source_missing',
    'artifact_missing',
    'artifact_unreadable',
    'support_root_missing',
    'unavailable',
    'error',
  ]);

  function normalizeState(value) {
    return String(value || 'unknown').toLowerCase().replace(/[^a-z0-9_-]+/g, '-');
  }

  function catalogFingerprint(skills) {
    return (Array.isArray(skills) ? skills : [])
      .map((skill) => String(skill?.id || '').trim())
      .filter(Boolean)
      .sort()
      .join('|');
  }

  function availabilityByID(availabilities) {
    const result = new Map();
    for (const item of Array.isArray(availabilities) ? availabilities : []) {
      const id = String(item?.skill_id || '').trim();
      if (id) result.set(id, item);
    }
    return result;
  }

  function isCleanupState(state) {
    return CLEANUP_STATES.has(normalizeState(state));
  }

  function summarizeAvailability(skills, availabilities) {
    const byID = availabilityByID(availabilities);
    const summary = {available: 0, disabled: 0, unavailable: 0, unknown: 0, cleanupIDs: []};
    for (const skill of Array.isArray(skills) ? skills : []) {
      const id = String(skill?.id || '').trim();
      if (!id) continue;
      const item = byID.get(id);
      const state = normalizeState(item?.state || 'unknown');
      if (state === 'available') summary.available++;
      else if (state === 'disabled') summary.disabled++;
      else if (isCleanupState(state)) { summary.unavailable++; summary.cleanupIDs.push(id); }
      else summary.unknown++;
    }
    summary.cleanupIDs.sort();
    return summary;
  }

  function retainExistingSelection(selectedIDs, skills) {
    const existing = new Set((Array.isArray(skills) ? skills : []).map((skill) => String(skill?.id || '')).filter(Boolean));
    return [...new Set(Array.isArray(selectedIDs) ? selectedIDs : [])].filter((id) => existing.has(id)).sort();
  }

  function probeMatches(probe, current) {
    if (!probe || !current) return false;
    if (probe.scope || current.scope) {
      return probe.connectionGeneration === current.connectionGeneration
        && probe.scope === current.scope
        && probe.catalogFingerprint === current.catalogFingerprint;
    }
    return probe.connectionGeneration === current.connectionGeneration
      && probe.environmentGeneration === current.environmentGeneration
      && probe.environmentID === current.environmentID
      && probe.catalogFingerprint === current.catalogFingerprint;
  }

  return {
    CLEANUP_STATES: [...CLEANUP_STATES],
    normalizeState,
    catalogFingerprint,
    isCleanupState,
    summarizeAvailability,
    retainExistingSelection,
    probeMatches,
  };
});
