const test = require('node:test');
const assert = require('node:assert/strict');
const {dashboardModel, snapshotCounts, scopeMatches} = require('../../cmd/ai-dev-manager-desktop/frontend/dashboard.js');

const emptySnapshot = {workspaces: [], environments: [], allowed_executables: [], mcps: [], skills: [], global_memory_count: 0};

test('unloaded/error dashboard never invents zero counts', () => {
  for (const state of ['unloaded', 'error']) {
    const model = dashboardModel({state});
    assert.deepEqual(model.counts, {
      workspaceCount: '—', environmentCount: '—', execCount: '—', mcpCount: '—', skillCount: '—', memoryCount: '—',
    });
  }
});

test('only a trusted successful empty snapshot establishes zeros', () => {
  const counts = snapshotCounts(emptySnapshot);
  assert.deepEqual(counts, {
    workspaceCount: 0, environmentCount: 0, execCount: 0, mcpCount: 0, skillCount: 0, memoryCount: 0,
  });
  assert.deepEqual(dashboardModel({state: 'success', snapshot: emptySnapshot}).counts, counts);
});

test('loading/stale may retain counts only from an existing successful snapshot', () => {
  const snapshot = {
    workspaces: [{}, {}], environments: [{}], allowed_executables: ['go'], mcps: [{}, {}, {}], skills: [{}, {}], global_memory_count: 4,
  };
  assert.equal(dashboardModel({state: 'loading', snapshot}).counts.workspaceCount, 2);
  assert.equal(dashboardModel({state: 'stale', snapshot, error: 'refresh failed'}).counts.memoryCount, 4);
  assert.equal(dashboardModel({state: 'loading'}).counts.workspaceCount, '—');
});

test('scope guard rejects late connection or Environment results', () => {
  const captured = {connectionGeneration: 4, environmentGeneration: 7, environmentID: 'env-a'};
  assert.equal(scopeMatches(captured, {connectionGeneration: 4, environmentGeneration: 7, environmentID: 'env-a'}), true);
  assert.equal(scopeMatches(captured, {connectionGeneration: 5, environmentGeneration: 7, environmentID: 'env-a'}), false);
  assert.equal(scopeMatches(captured, {connectionGeneration: 4, environmentGeneration: 8, environmentID: 'env-b'}), false);
});
