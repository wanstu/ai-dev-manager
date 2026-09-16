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
