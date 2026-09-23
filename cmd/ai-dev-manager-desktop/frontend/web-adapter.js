(() => {
  if (
    !['http:', 'https:'].includes(window.location.protocol) ||
    window.location.hostname === 'wails.localhost' ||
    window.go?.desktop?.Adapter
  ) return;

  const genericMethods = [
    'GetSnapshot','UpdateWorktreeSettings','RefreshHostEnvironment','GetLoggingStatus',
    'GetGatewayAccessStatus','GetGatewayDiagnostics','SetGatewayAllowedHosts',
    'RotateGatewayAdminAPIKey','RotateGatewayAgentAPIKey','ClearGatewayAdminAPIKey',
    'SetGatewayAgentAPIKey','ClearGatewayAgentAPIKey','GetExecAuthorizationStatus',
    'SetExecFullAuthorization','ListSkillSources','ListSkillAvailability','BrowseHostDirectories','AddWorkspace',
    'RenameWorkspace','RemoveWorkspace','DiscoverWorkspace','CreateEnvironment',
    'RenameEnvironment','RemoveEnvironment','InspectEnvironment','EnvironmentTreeDigest',
    'EnvironmentWorkspaceOptions','EnvironmentWorkspaceRecommendations','SetEnvironmentWorkspace',
    'AllowExecutable','RemoveExecutable','BlockExecutable','UnblockExecutable','ClearExecDenial',
    'ClearAllExecDenials','AddMCP','UpdateMCP','RemoveMCP','SetMCPDefault','ProbeMCPHealth',
    'PreviewMCPImport','ApplyMCPImport','AddSkillSource','UpdateSkillSource','RefreshSkillSource',
    'RemoveSkillSource','RemoveSkill','SetSkillDefault','SetWorkspaceMCP','SetWorkspaceSkill',
    'SetEnvironmentMCP','SetEnvironmentSkill','ListEnvironmentSkills','ListGlobalMemory',
    'WriteGlobalMemory','DeleteGlobalMemory','ListEnvironmentMemory','WriteEnvironmentMemory',
    'DeleteEnvironmentMemory','AcquireRuntimeWriter','ReleaseRuntimeWriter','ListVerifiers',
    'RunVerifier','ListProcesses','GetProcessLogs','StopProcess','ListRuns','CancelRun',
    'GetTemporaryEnvironmentStatus','PromoteTemporaryEnvironment','CleanupTemporaryEnvironment',
    'CleanupExpiredTemporaryEnvironments'
  ];

  async function call(method, ...args) {
    const response = await fetch('/api/web/manage', {
      method: 'POST',
      credentials: 'same-origin',
      headers: {'Content-Type': 'application/json', 'X-ADM-Web': '1'},
      body: JSON.stringify({method, args}),
    });
    const body = await response.json().catch(() => ({}));
    if (response.status === 401) {
      location.replace('/');
      throw new Error('Web session 已失效，请重新登录');
    }
    if (!response.ok) throw new Error(body?.error || ('HTTP ' + response.status));
    return body?.result;
  }

  async function health() {
    const response = await fetch('/healthz', {credentials: 'same-origin', cache: 'no-store'});
    if (!response.ok) throw new Error('Gateway health check failed: HTTP ' + response.status);
    return response.json();
  }

  const profile = () => ({
    id: 'web',
    name: '当前 Web Gateway',
    base_url: location.origin,
    api_key_configured: true,
    start_service_on_desktop_launch: false,
  });

  function webPreferences() {
    return {
      launch_at_login_supported: false,
      launch_at_login: false,
      theme_mode: localStorage.getItem('adm-web-theme-mode') || 'system',
      theme_pack: localStorage.getItem('adm-web-theme-pack') || '',
    };
  }

  const adapter = {};
  for (const method of genericMethods) adapter[method] = (...args) => call(method, ...args);

  adapter.GetConnectionProfiles = async () => ({profiles: [profile()], active_id: 'web'});
  adapter.SelectConnectionProfile = async () => ({profiles: [profile()], active_id: 'web'});
  adapter.SaveConnectionProfile = async () => { throw new Error('Web 管理台固定连接当前 Gateway，不需要保存连接配置'); };
  adapter.DeleteConnectionProfile = async () => { throw new Error('Web 管理台不能删除当前 Gateway 连接'); };
  adapter.DisconnectADM = async () => null;
  adapter.ConfigureGatewayAdminAPIKey = async (apiKey) => call('ConfigureGatewayAdminAPIKey', apiKey);
  adapter.RotateGatewayAdminAPIKey = async () => {
    const result = await call('RotateGatewayAdminAPIKey');
    return String(result?.admin_api_key || '').trim();
  };
  adapter.RotateGatewayAgentAPIKey = async () => {
    const result = await call('RotateGatewayAgentAPIKey');
    return String(result?.agent_api_key || '').trim();
  };

  adapter.ConnectADM = async () => {
    const info = await health();
    return {
      state: 'running',
      base_url: location.origin,
      health_url: location.origin + '/healthz',
      agent_mcp_url: location.origin + '/mcp',
      admin_mcp_url: location.origin + '/admin/mcp',
      pid: info.pid || 0,
      version: info.version || '',
      management_api_version: info.management_api_version ?? 0,
      recognized_adm_gateway: true,
      management_compatible: true,
      local_bootstrap_eligible: false,
      listen: location.host,
      detail: '当前页面直接管理这个 Gateway；浏览器登录 Session 与 Agent/Admin API Key 相互独立。',
    };
  };

  adapter.GetAboutInfo = async () => {
    const info = await health();
    return {version: info.version || '', go_version: '', goos: 'web', goarch: ''};
  };

  adapter.GetDesktopPreferences = async () => webPreferences();
  adapter.SetLaunchAtLogin = async () => webPreferences();
  adapter.SetDesktopTheme = async (mode, pack) => {
    if (mode) localStorage.setItem('adm-web-theme-mode', mode);
    if (pack) localStorage.setItem('adm-web-theme-pack', pack);
    else localStorage.removeItem('adm-web-theme-pack');
    return webPreferences();
  };
  adapter.StartLocalADM = async () => { throw new Error('Web 管理台不负责启动 Gateway；请使用 systemd 或 adm gateway start'); };
  adapter.StopLocalADM = async () => { throw new Error('Web 管理台不提供停止自身 Gateway 的操作'); };

  window.ADMWebAdapter = adapter;
  window.ADMWebSurface = true;

  document.addEventListener('DOMContentLoaded', () => {
    document.body.dataset.surface = 'web';
    const subtitle = document.querySelector('.subtitle');
    if (subtitle) subtitle.textContent = 'Web management';
    for (const id of ['addConnection','editConnection','deleteConnection','gatewayStartButton','gatewayStopButton']) {
      const element = document.getElementById(id);
      if (element) element.hidden = true;
    }
    const launch = document.getElementById('launchAtLogin')?.closest('label');
    if (launch) launch.hidden = true;
    const actions = document.querySelector('.topbar-actions');
    if (actions && !document.getElementById('webLogoutButton')) {
      const button = document.createElement('button');
      button.id = 'webLogoutButton';
      button.type = 'button';
      button.className = 'secondary-button';
      button.textContent = '退出登录';
      button.addEventListener('click', async () => {
        button.disabled = true;
        try {
          await fetch('/api/web/auth/logout', {method: 'POST', credentials: 'same-origin', headers: {'X-ADM-Web': '1'}});
        } finally {
          location.replace('/');
        }
      });
      actions.append(button);
    }
  });
})();
