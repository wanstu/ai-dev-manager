const test = require('node:test');
const assert = require('node:assert/strict');
const runtime = require('../../cmd/ai-dev-manager-desktop/frontend/runtime-view.js');

test('runtime subview normalization stays within the three local views', () => {
  assert.deepEqual(runtime.SUBVIEWS, ['verifiers', 'processes', 'runs']);
  assert.equal(runtime.normalizeSubview('processes'), 'processes');
  assert.equal(runtime.normalizeSubview('unknown'), 'verifiers');
});

test('output binding requires connection, Environment, resource and subview generation identity', () => {
  const binding = runtime.makeOutputBinding({connectionGeneration: 4, environmentGeneration: 7, environmentID: 'env-a'}, 'processes', 'proc-a', 3);
  assert.equal(runtime.outputBindingMatches(binding, {connectionGeneration: 4, environmentGeneration: 7, environmentID: 'env-a', kind: 'processes', id: 'proc-a', viewGeneration: 3}), true);
  assert.equal(runtime.outputBindingMatches(binding, {connectionGeneration: 4, environmentGeneration: 8, environmentID: 'env-b', kind: 'processes', id: 'proc-a', viewGeneration: 3}), false);
  assert.equal(runtime.outputBindingMatches(binding, {connectionGeneration: 4, environmentGeneration: 7, environmentID: 'env-a', kind: 'runs', id: 'proc-a', viewGeneration: 4}), false);
});

test('resource disappearance clears output only after a successful authoritative list', () => {
  const binding = runtime.makeOutputBinding({connectionGeneration: 1, environmentGeneration: 2, environmentID: 'env-a'}, 'processes', 'proc-a', 0);
  const lists = {processes: [{id: 'proc-a'}]};
  assert.equal(runtime.resourceStillKnown(binding, lists, {processes: 'success'}), true);
  assert.equal(runtime.resourceStillKnown(binding, {processes: []}, {processes: 'success'}), false);
  assert.equal(runtime.resourceStillKnown(binding, {processes: []}, {processes: 'error'}), true);
});

test('runtime resource IDs and truncation summary preserve existing result semantics', () => {
  assert.equal(runtime.resourceID('verifiers', {verifier_id: 'vf-a'}), 'vf-a');
  assert.equal(runtime.resourceID('runs', {id: 'run-a'}), 'run-a');
  assert.equal(runtime.truncationSummary({stdout_truncated: true, stderr_truncated: true}), 'stdout truncated · stderr truncated');
  assert.equal(runtime.truncationSummary({}), '');
});
