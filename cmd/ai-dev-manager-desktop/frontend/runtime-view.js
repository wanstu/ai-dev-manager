(function attachRuntimeView(global) {
  const SUBVIEWS = ['verifiers', 'processes', 'runs'];

  function normalizeSubview(value) {
    const token = String(value || '').trim().toLowerCase();
    return SUBVIEWS.includes(token) ? token : 'verifiers';
  }

  function resourceID(kind, item) {
    if (!item) return '';
    if (kind === 'verifiers') return String(item.verifier_id || item.id || '');
    return String(item.id || '');
  }

  function makeOutputBinding(scope = {}, kind, id, viewGeneration = 0) {
    return {
      connectionGeneration: Number(scope.connectionGeneration || 0),
      environmentGeneration: Number(scope.environmentGeneration || 0),
      environmentID: String(scope.environmentID || ''),
      kind: normalizeSubview(kind),
      id: String(id || ''),
      viewGeneration: Number(viewGeneration || 0),
    };
  }

  function outputBindingMatches(binding, current = {}) {
    if (!binding) return false;
    return binding.connectionGeneration === Number(current.connectionGeneration || 0)
      && binding.environmentGeneration === Number(current.environmentGeneration || 0)
      && binding.environmentID === String(current.environmentID || '')
      && binding.kind === normalizeSubview(current.kind)
      && binding.id === String(current.id || '')
      && binding.viewGeneration === Number(current.viewGeneration || 0);
  }

  function resourceStillKnown(binding, lists = {}, states = {}) {
    if (!binding?.id) return false;
    const state = String(states?.[binding.kind] || 'unloaded');
    if (state !== 'success') return true;
    return (Array.isArray(lists?.[binding.kind]) ? lists[binding.kind] : []).some((item) => resourceID(binding.kind, item) === binding.id);
  }

  function truncationSummary(item = {}) {
    const parts = [];
    if (item.stdout_truncated) parts.push('stdout truncated');
    if (item.stderr_truncated) parts.push('stderr truncated');
    return parts.join(' · ');
  }

  const api = {SUBVIEWS: [...SUBVIEWS], normalizeSubview, resourceID, makeOutputBinding, outputBindingMatches, resourceStillKnown, truncationSummary};
  global.ADMRuntimeView = api;
  if (typeof module !== 'undefined' && module.exports) module.exports = api;
})(typeof window !== 'undefined' ? window : globalThis);
