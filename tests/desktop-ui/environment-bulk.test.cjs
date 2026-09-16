const test = require('node:test');
const assert = require('node:assert/strict');
const bulk = require('../../cmd/ai-dev-manager-desktop/frontend/environment-bulk.js');

test('selection is deduplicated, sorted, and pruned to existing stable IDs', () => {
  assert.deepEqual(bulk.uniqueIDs(['b', 'a', 'b', '', null]), ['a', 'b']);
  assert.deepEqual(bulk.retainExistingSelection(['b', 'missing', 'a'], [{id:'a'}, {id:'b'}]), ['a', 'b']);
  assert.deepEqual(bulk.retainExistingSelection(['env-b', 'missing'], [{environment_id:'env-a'}, {environment_id:'env-b'}], 'environment_id'), ['env-b']);
});

test('environment summary exposes mixed current state for selected MCPs and Skills', () => {
  const summary = bulk.environmentSummary({environment_id:'env-a', name:'A', enabled_mcp_ids:['m1'], enabled_skill_ids:['s2']}, ['m1','m2'], ['s1','s2']);
  assert.deepEqual(summary, {environmentID:'env-a', environmentName:'A', mcpEnabled:1, mcpExplicit:1, mcpInherited:0, mcpTotal:2, skillEnabled:1, skillExplicit:1, skillInherited:0, skillTotal:2});
});

test('operation plan is resource x environment and marks already-desired pairs as no-op', () => {
  const environments = [
    {environment_id:'env-a', name:'A', enabled_mcp_ids:['m1'], enabled_skill_ids:[]},
    {environment_id:'env-b', name:'B', enabled_mcp_ids:[], enabled_skill_ids:['s1']},
  ];
  const plan = bulk.buildOperations(environments, ['env-b','env-a'], ['m1'], ['s1'], true);
  assert.equal(plan.length, 4);
  assert.deepEqual(plan.map((op) => [op.environmentID, op.kind, op.resourceID, op.noop]), [
    ['env-a','mcp','m1',true],
    ['env-a','skill','s1',false],
    ['env-b','mcp','m1',false],
    ['env-b','skill','s1',true],
  ]);
});

test('Environment assignment plans explicit selections separately from Workspace inheritance', () => {
  const environments = [
    {environment_id:'env-a', name:'A', enabled_mcp_ids:['m1'], explicit_mcp_ids:[], inherited_mcp_ids:['m1'], enabled_skill_ids:['s1'], explicit_skill_ids:['s1'], inherited_skill_ids:[]},
  ];
  const disable = bulk.buildOperations(environments, ['env-a'], ['m1'], ['s1'], false);
  assert.deepEqual(disable.map((op) => [op.kind, op.currentEnabled, op.inherited, op.effectiveEnabled, op.noop]), [
    ['mcp', false, true, true, true],
    ['skill', true, false, true, false],
  ]);
  const summary = bulk.environmentSummary(environments[0], ['m1'], ['s1']);
  assert.equal(summary.mcpEnabled, 1);
  assert.equal(summary.mcpExplicit, 0);
  assert.equal(summary.mcpInherited, 1);
});

test('result summary keeps changed, unchanged, and partial failures distinct', () => {
  const summary = bulk.summarizeResults([
    {environmentID:'env-a', status:'changed'},
    {environmentID:'env-a', status:'unchanged'},
    {environmentID:'env-b', status:'failed'},
  ]);
  assert.deepEqual(summary, {total:3, changed:1, unchanged:1, failed:1, environments:2});
});

test('missing selections identify only explicit Environment references and Workspace references absent from the catalog', () => {
  const environments = [
    {environment_id:'env-b', name:'B', enabled_mcp_ids:['m1','missing-mcp','workspace-missing'], explicit_mcp_ids:['m1','missing-mcp'], inherited_mcp_ids:['workspace-missing'], enabled_skill_ids:['missing-skill'], explicit_skill_ids:['missing-skill'], inherited_skill_ids:[]},
    {environment_id:'env-a', name:'A', enabled_mcp_ids:['missing-mcp'], explicit_mcp_ids:['missing-mcp'], inherited_mcp_ids:[], enabled_skill_ids:['s1'], explicit_skill_ids:['s1'], inherited_skill_ids:[]},
  ];
  assert.deepEqual(bulk.missingSelections(environments, [{id:'m1'}], 'mcp').map((op) => [op.environmentID, op.resourceID]), [
    ['env-a','missing-mcp'],
    ['env-b','missing-mcp'],
  ]);
  assert.deepEqual(bulk.missingSelections(environments, [{id:'s1'}], 'skill').map((op) => [op.environmentID, op.resourceID]), [
    ['env-b','missing-skill'],
  ]);
  assert.deepEqual(bulk.missingSelections(environments, [{id:'m1'}], 'mcp').map((op) => op.resourceID).includes('workspace-missing'), false);
  assert.deepEqual(bulk.missingWorkspaceSelections([
    {workspace_id:'ws-b', name:'B', enabled_mcp_ids:['m1','workspace-missing'], enabled_skill_ids:[]},
    {workspace_id:'ws-a', name:'A', enabled_mcp_ids:[], enabled_skill_ids:['workspace-skill-missing']},
  ], [{id:'m1'}], 'mcp').map((op) => [op.workspaceID, op.resourceID]), [
    ['ws-b','workspace-missing'],
  ]);
  assert.deepEqual(bulk.missingWorkspaceSelections([
    {workspace_id:'ws-a', name:'A', enabled_skill_ids:['workspace-skill-missing']},
  ], [{id:'s1'}], 'skill').map((op) => [op.workspaceID, op.resourceID]), [
    ['ws-a','workspace-skill-missing'],
  ]);
});
