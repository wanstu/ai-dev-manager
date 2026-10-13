const test = require('node:test');
const assert = require('node:assert/strict');
const pages = require('../../cmd/ai-dev-manager-desktop/frontend/project-pages.js');

const workspaces = [
  {workspace_id:'ws-a', name:'Alpha Project', path:'C:\\projects\\alpha'},
  {workspace_id:'ws-b', name:'Beta Project', path:'D:\\work\\beta'},
];
const environments = [
  {environment_id:'env-a1', workspace_id:'ws-a', name:'Alpha Dev', root:'C:\\projects\\alpha'},
  {environment_id:'env-a2', workspace_id:'ws-a', name:'Alpha Test', root:'C:\\projects\\alpha-test'},
  {environment_id:'env-b1', workspace_id:'ws-b', name:'Beta Plain', root:'D:\\work\\beta'},
];


test('Workspace occupancy and count sorting compose with search without changing snapshot order', () => {
  const rows = [
    {workspace_id:'ws-c', name:'Empty', path:'C:/empty'},
    ...workspaces
  ];
  assert.deepEqual(pages.workspaceListModel(rows, environments, {occupancy:'empty'}).items.map(item => item.workspace_id), ['ws-c']);
  assert.deepEqual(pages.workspaceListModel(rows, environments, {occupancy:'with', sort:'count-desc'}).items.map(item => item.workspace_id), ['ws-a', 'ws-b']);
  assert.deepEqual(pages.workspaceListModel(rows, environments, {sort:'count-asc'}).items.map(item => item.workspace_id), ['ws-c','ws-b','ws-a']);
  assert.equal(rows[0].workspace_id, 'ws-c');
});

test('Environment lifecycle, expiry, writer and owner filters are combined and deterministic', () => {
  const now = Date.parse('2026-10-13T12:00:00Z');
  const source = [
    {environment_id:'d', name:'Durable', workspace_id:'ws-a', retention:{persistence:'durable'}, last_activity_at:'2026-09-20T00:00:00Z'},
    {environment_id:'a', name:'Expired', workspace_id:'ws-a', retention:{persistence:'temporary', owner_id:'agent-old', expires_at:'2026-10-10T00:00:00Z'}, last_activity_at:'2026-09-25T00:00:00Z'},
    {environment_id:'b', name:'Due soon', workspace_id:'ws-a', writer:{owner:'active'}, retention:{persistence:'temporary', owner_id:'agent-due', expires_at:'2026-10-15T00:00:00Z'}, last_activity_at:'2026-10-13T11:00:00Z'},
    {environment_id:'c', name:'Later', workspace_id:'ws-b', retention:{persistence:'temporary', expires_at:'2026-11-13T00:00:00Z'}, last_activity_at:'2026-10-01T00:00:00Z'},
    {environment_id:'e', name:'No expiry', workspace_id:'ws-b', retention:{persistence:'temporary'}, last_activity_at:'2026-10-04T00:00:00Z'}
  ];
  const ids = opts => pages.environmentListModel(source, workspaces, {...opts, now}).items.map(item=>item.environment_id);
  assert.deepEqual(ids({retention:'durable'}), ['d']);
  assert.deepEqual(ids({kind:'managed', managedIDs:['a','b','c']}), ['b','c','a']);
  assert.deepEqual(ids({kind:'ordinary', managedIDs:['a','b','c']}), ['e','d']);
  assert.deepEqual(ids({kind:'managed', managedIDs:['a','b','c'],retention:'expired'}), ['a']);
  assert.deepEqual(ids({retention:'expired'}), ['a']);
  assert.deepEqual(ids({retention:'due7'}), ['b']);
  assert.deepEqual(ids({retention:'noexpiry'}), ['e']);
  assert.deepEqual(ids({retention:'temporary',writer:'active'}), ['b']);
  assert.deepEqual(ids({retention:'temporary',writer:'none',workspaceID:'ws-a'}), ['a']);
  assert.deepEqual(ids({query:'agent-due'}), ['b']);
  assert.deepEqual(ids({sort:'expiry'}), ['a','b','c','d','e']);
  assert.deepEqual(ids({sort:'name'}), ['b','d','a','c','e']);
  assert.deepEqual(ids({sort:'recent'}), ['b','e','c','a','d']);
  assert.equal(source[0].environment_id, 'd');
});

test('workspace model filters loaded rows and counts Environments from the same snapshot', () => {
  const model = pages.workspaceListModel(workspaces, environments, 'alpha');
  assert.equal(model.total, 2);
  assert.equal(model.visible, 1);
  assert.equal(model.items[0].workspace_id, 'ws-a');
  assert.deepEqual(model.environmentCounts, {'ws-a':2, 'ws-b':1});
});

test('workspace search matches stable ID and full path without scanning disk', () => {
  assert.deepEqual(pages.workspaceListModel(workspaces, environments, 'd:\\work\\beta').items.map(item => item.workspace_id), ['ws-b']);
  assert.deepEqual(pages.workspaceListModel(workspaces, environments, 'ws-a').items.map(item => item.workspace_id), ['ws-a']);
});

test('Environment model joins Workspace names and combines Workspace/search filters', () => {
  const byWorkspace = pages.environmentListModel(environments, workspaces, {workspaceID:'ws-a'});
  assert.deepEqual(byWorkspace.items.map(item => item.environment_id), ['env-a1','env-a2']);
  assert.equal(byWorkspace.workspaceNames['ws-a'], 'Alpha Project');
  const searched = pages.environmentListModel(environments, workspaces, {query:'beta project'});
  assert.deepEqual(searched.items.map(item => item.environment_id), ['env-b1']);
});

test('removed Workspace filter remains invalid instead of silently retargeting', () => {
  const model = pages.environmentListModel(environments, workspaces, {workspaceID:'ws-missing'});
  assert.equal(model.workspaceFilterValid, false);
  assert.equal(model.visible, 0);
  assert.deepEqual(model.items, []);
});

test('empty loaded data and filter miss remain distinguishable by total/visible counts', () => {
  assert.deepEqual(pages.workspaceListModel([], [], 'anything'), {items:[], total:0, visible:0, environmentCounts:{}});
  const miss = pages.workspaceListModel(workspaces, environments, 'not-present');
  assert.equal(miss.total, 2);
  assert.equal(miss.visible, 0);
});
