(function (root, factory) {
  const api = factory();
  if (typeof module === 'object' && module.exports) module.exports = api;
  else root.ADMMCPExport = api;
})(typeof globalThis !== 'undefined' ? globalThis : this, function () {
  function sortedObject(values) {
    const result = {};
    for (const key of Object.keys(values || {}).sort()) result[key] = String(values[key] ?? '');
    return result;
  }

  function genericServer(entry) {
    if (!entry || typeof entry !== 'object') throw new Error('MCP definition is required');
    if (entry.transport === 'stdio') {
      const server = {command: String(entry.executable || '')};
      const args = Array.isArray(entry.args) ? entry.args.map((value) => String(value)) : [];
      const env = sortedObject(entry.env_refs);
      if (args.length) server.args = args;
      if (Object.keys(env).length) server.env = env;
      return server;
    }
    const server = {url: String(entry.endpoint || '')};
    const headers = sortedObject(entry.header_refs);
    if ((entry.auth_mode === 'headers' || Object.keys(headers).length) && Object.keys(headers).length) server.headers = headers;
    return server;
  }

  function genericConfig(entry) {
    const name = String(entry?.name || entry?.id || 'mcp').trim() || 'mcp';
    return {mcpServers: {[name]: genericServer(entry)}};
  }

  function stringifyGenericConfig(entry) {
    return JSON.stringify(genericConfig(entry), null, 2);
  }

  return {genericServer, genericConfig, stringifyGenericConfig};
});
