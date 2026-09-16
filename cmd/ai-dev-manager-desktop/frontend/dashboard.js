(function attachManagementDashboard(global) {
  const metricKeys = ['workspaces', 'environments', 'allowed_executables', 'mcps', 'skills'];

  function count(value) {
    return Array.isArray(value) ? value.length : 0;
  }

  function snapshotCounts(snapshot) {
    return {
      workspaceCount: count(snapshot?.workspaces),
      environmentCount: count(snapshot?.environments),
      execCount: count(snapshot?.allowed_executables),
      mcpCount: count(snapshot?.mcps),
      skillCount: count(snapshot?.skills),
      memoryCount: Number.isFinite(Number(snapshot?.global_memory_count)) ? Number(snapshot.global_memory_count) : 0,
    };
  }

  function dashboardModel(input = {}) {
    const state = String(input.state || 'unloaded');
    const snapshot = input.snapshot || null;
    const hasTrustedSnapshot = Boolean(snapshot) && ['success', 'stale', 'loading'].includes(state);
    const counts = hasTrustedSnapshot ? snapshotCounts(snapshot) : {
      workspaceCount: '—', environmentCount: '—', execCount: '—', mcpCount: '—', skillCount: '—', memoryCount: '—',
    };
    const labels = {
      unloaded: '未加载',
      loading: snapshot ? '刷新中' : '正在加载',
      success: '已加载',
      stale: '数据过期',
      error: '不可用',
    };
    const detail = input.error
      ? String(input.error)
      : state === 'unloaded'
        ? '选择并连接 ADM 后读取管理数据。'
        : state === 'loading'
          ? (snapshot ? '正在刷新；当前数字来自上一次成功快照。' : '正在读取 Admin MCP 管理快照。')
          : state === 'success'
            ? '数字来自最近一次成功的管理快照；配置数量不代表 Runtime 健康。'
            : state === 'stale'
              ? '最近一次刷新失败；当前数字来自上一次成功快照。'
              : '管理快照当前不可用。';
    return {
      state,
      stateLabel: labels[state] || labels.unloaded,
      counts,
      lastSuccessLabel: input.lastSuccessAt ? `上次成功 ${new Date(input.lastSuccessAt).toLocaleTimeString()}` : '尚无成功快照',
      detail,
    };
  }

  function scopeMatches(scope, current) {
    return Boolean(scope) && Boolean(current)
      && scope.connectionGeneration === current.connectionGeneration
      && scope.environmentGeneration === current.environmentGeneration
      && scope.environmentID === current.environmentID;
  }

  function renderDashboard(elements, input) {
    const model = dashboardModel(input);
    elements.state.textContent = model.stateLabel;
    elements.state.dataset.state = model.state;
    elements.lastSuccess.textContent = model.lastSuccessLabel;
    elements.detail.textContent = model.detail;
    for (const [key, value] of Object.entries(model.counts)) {
      if (elements.metrics[key]) elements.metrics[key].textContent = String(value);
    }
    return model;
  }

  const api = {metricKeys: [...metricKeys], snapshotCounts, dashboardModel, scopeMatches, renderDashboard};
  global.ADMDashboard = api;
  if (typeof module !== 'undefined' && module.exports) module.exports = api;
})(typeof window !== 'undefined' ? window : globalThis);
