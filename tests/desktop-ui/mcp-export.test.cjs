const test = require('node:test');
const assert = require('node:assert/strict');
const {genericServer, stringifyGenericConfig} = require('../../cmd/ai-dev-manager-desktop/frontend/mcp-export.js');

test('exports Streamable HTTP MCP with only URL and header references', () => {
  const server = genericServer({
    transport: 'streamable-http',
    endpoint: 'https://example.test/mcp',
    executable: 'should-not-export',
    args: ['ignored'],
    auth_mode: 'headers',
    header_refs: {Authorization: 'Bearer ${TOKEN}', 'X-Tenant': '${TENANT}'},
    env_refs: {IGNORED: '${IGNORED}'},
  });
  assert.deepEqual(server, {
    url: 'https://example.test/mcp',
    headers: {Authorization: 'Bearer ${TOKEN}', 'X-Tenant': '${TENANT}'},
  });
});

test('exports stdio MCP with only command, args, and environment references', () => {
  const server = genericServer({
    transport: 'stdio',
    endpoint: 'https://ignored.test/mcp',
    executable: 'node',
    args: ['server.js', '--stdio'],
    header_refs: {Ignored: '${IGNORED}'},
    env_refs: {DB_TOKEN: '${DB_TOKEN}'},
  });
  assert.deepEqual(server, {
    command: 'node',
    args: ['server.js', '--stdio'],
    env: {DB_TOKEN: '${DB_TOKEN}'},
  });
});

test('stringifies one MCP as generic mcpServers configuration', () => {
  const output = stringifyGenericConfig({id: 'mcp-a', name: 'MCP A', transport: 'streamable-http', endpoint: 'http://127.0.0.1:9900/mcp'});
  assert.deepEqual(JSON.parse(output), {mcpServers: {'MCP A': {url: 'http://127.0.0.1:9900/mcp'}}});
  assert.match(output, /\n  "mcpServers"/);
});
