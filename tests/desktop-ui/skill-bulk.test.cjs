const test = require('node:test');
const assert = require('node:assert/strict');
const bulk = require('../../cmd/ai-dev-manager-desktop/frontend/skill-bulk.js');

test('cleanup classification excludes disabled/available and includes structural failures', () => {
  assert.equal(bulk.isCleanupState('disabled'), false);
  assert.equal(bulk.isCleanupState('available'), false);
  for (const state of ['unconfigured', 'source_missing', 'artifact_missing', 'artifact_unreadable', 'support_root_missing', 'unavailable', 'error']) {
    assert.equal(bulk.isCleanupState(state), true, state);
  }
});

test('availability summary only returns current catalog structural failures for cleanup', () => {
  const skills = [{id:'available'}, {id:'disabled'}, {id:'broken'}, {id:'unknown'}];
  const summary = bulk.summarizeAvailability(skills, [
    {skill_id:'available', state:'available'},
    {skill_id:'disabled', state:'disabled'},
    {skill_id:'broken', state:'artifact_missing'},
    {skill_id:'orphan-not-in-catalog', state:'source_missing'},
  ]);
  assert.deepEqual(summary, {available:1, disabled:1, unavailable:1, unknown:1, cleanupIDs:['broken']});
});

test('catalog fingerprint is stable across row ordering', () => {
  assert.equal(bulk.catalogFingerprint([{id:'b'}, {id:'a'}]), 'a|b');
  assert.equal(bulk.catalogFingerprint([{id:'a'}, {id:'b'}]), 'a|b');
});

test('probe freshness supports legacy Environment-scoped and current catalog-scoped checks', () => {
  const legacyProbe = {connectionGeneration:4, environmentGeneration:8, environmentID:'env-a', catalogFingerprint:'a|b'};
  assert.equal(bulk.probeMatches(legacyProbe, {...legacyProbe}), true);
  assert.equal(bulk.probeMatches(legacyProbe, {...legacyProbe, connectionGeneration:5}), false);
  assert.equal(bulk.probeMatches(legacyProbe, {...legacyProbe, environmentGeneration:9}), false);
  assert.equal(bulk.probeMatches(legacyProbe, {...legacyProbe, environmentID:'env-b'}), false);
  assert.equal(bulk.probeMatches(legacyProbe, {...legacyProbe, catalogFingerprint:'a|c'}), false);

  const catalogProbe = {connectionGeneration:4, scope:'catalog', catalogFingerprint:'a|b'};
  assert.equal(bulk.probeMatches(catalogProbe, {...catalogProbe, environmentGeneration:99, environmentID:'env-b'}), true);
  assert.equal(bulk.probeMatches(catalogProbe, {...catalogProbe, connectionGeneration:5}), false);
  assert.equal(bulk.probeMatches(catalogProbe, {...catalogProbe, scope:'environment'}), false);
  assert.equal(bulk.probeMatches(catalogProbe, {...catalogProbe, catalogFingerprint:'a|c'}), false);
});

test('selection is pruned to stable IDs still present in the current catalog', () => {
  assert.deepEqual(bulk.retainExistingSelection(['b','missing','a','a'], [{id:'a'}, {id:'b'}]), ['a','b']);
});
