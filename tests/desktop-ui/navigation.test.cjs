const test = require('node:test');
const assert = require('node:assert/strict');
const navigation = require('../../cmd/ai-dev-manager-desktop/frontend/navigation.js');

test('Phase 18 navigation exposes management routes including standalone diagnostics', () => {
  assert.deepEqual(navigation.routes, [
    'overview',
    'workspaces',
    'environments',
    'runtime',
    'mcp',
    'skills',
    'memory',
    'gateway',
    'diagnostics',
    'exec-allowlist',
    'settings',
  ]);
});

test('normalizeRoute accepts canonical hashes and falls back to overview', () => {
  assert.equal(navigation.normalizeRoute('#/runtime'), 'runtime');
  assert.equal(navigation.normalizeRoute('/mcp'), 'mcp');
  assert.equal(navigation.normalizeRoute('skills'), 'skills');
  assert.equal(navigation.normalizeRoute('#/diagnostics'), 'diagnostics');
  assert.equal(navigation.normalizeRoute('#/unknown'), 'overview');
  assert.equal(navigation.normalizeRoute(''), 'overview');
  assert.equal(navigation.routeHash('workspaces'), '#/workspaces');
  assert.equal(navigation.routeHash('not-a-route'), '#/overview');
});
