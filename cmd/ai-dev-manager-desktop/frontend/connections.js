let connectionProfiles = {profiles: [], active_id: ''};
let connectionProfilesLoaded = false;
let connectionSwitching = false;
let desktopPendingRequests = 0;
let desktopRequestQueue = Promise.resolve();
const desktopRequestQueueIdleDelay = 25;
const desktopAdapterProxies = new WeakMap();
function trackDesktopAdapter(adapter) {
 if (!desktopAdapterProxies.has(adapter)) desktopAdapterProxies.set(adapter, new Proxy(adapter, {
  get(target, key) {
   const method = target[key];
   if (typeof method !== 'function') return method;
   return (...args) => {
    desktopPendingRequests++;
    const result = desktopRequestQueue.then(() => method.apply(target, args));
    desktopRequestQueue = result.catch(() => {});
    return result.finally(() => { desktopPendingRequests--; });
   };
  }
 }));
 return desktopAdapterProxies.get(adapter);
}
function activeConnectionProfile() { return connectionProfiles.profiles.find(p => p.id === connectionProfiles.active_id) || null; }
function renderConnectionProfiles() {
 const select = document.getElementById('connectionSelect');
 select.replaceChildren(new Option('选择管理连接', ''));
 for (const profile of connectionProfiles.profiles) {
  const suffix = profile.start_service_on_desktop_launch ? ' · 启动 Desktop 时启动' : '';
  select.add(new Option(profile.name + ' · ' + profile.base_url + suffix, profile.id));
 }
 select.value = connectionProfiles.active_id;
 const active = activeConnectionProfile();
 elements.gatewayBaseURL.value = active?.base_url || '';
 document.getElementById('editConnection').disabled = !active;
 document.getElementById('deleteConnection').disabled = !active;
 if (!active) {
  elements.gatewayState.textContent = '未选择连接';
  elements.gatewayState.dataset.state = 'stopped';
  for (const item of [elements.gatewayHealthURL, elements.gatewayURL, elements.gatewayAdminURL]) item.textContent = '—';
  elements.gatewayProcess.textContent = 'PID — · Version —';
  elements.gatewayDetail.textContent = '添加或选择一个 ADM 连接以管理。';
  elements.gatewayStartButton.disabled = true; elements.gatewayStopButton.disabled = true;
 }
 elements.gatewayRefreshButton.disabled = !active;
 elements.refreshButton.disabled = !active;
}
async function withConnectionTransition(action) {
 if (connectionSwitching) return;
 connectionSwitching = true;
 const main = document.querySelector('main'); main.inert = true;
 try {
  // Let old request continuations and their follow-up reads finish on the old target.
  do { await new Promise(resolve => setTimeout(resolve, desktopRequestQueueIdleDelay)); } while (desktopPendingRequests);
  await action();
 } catch (error) {
  setStatus('连接配置失败：' + (error?.message || String(error)), 'error');
 } finally {
  connectionSwitching = false; main.inert = false; renderConnectionProfiles();
 }
}
async function connectSelectedProfile(options = {}) {
 clearManagementData();
 await desktopAdapter().DisconnectADM();
 renderConnectionProfiles();
 const profile = activeConnectionProfile();
 if (!profile) return;
 if (options.desktopStartup && profile.start_service_on_desktop_launch) {
  setStatus('正在随 Desktop 启动本地 ADM Service…', 'loading');
  try {
   const status = await desktopAdapter().StartLocalADM({base_url: profile.base_url});
   renderGatewayStatus(status);
   if (status?.state === 'running') await refreshSnapshot('本地 ADM Service 已随 Desktop 启动');
   else { clearManagementData(); setStatus('ADM Service 未运行；管理数据未加载', 'error'); }
  } catch (error) {
   clearManagementData();
   setStatus('随 Desktop 启动 ADM Service 失败：' + (error?.message || String(error)), 'error');
   try { await refreshGatewayStatus(false); } catch (_) {}
  }
  return;
 }
 await refreshConnectedADM(false);
}
async function initializeConnectionProfiles() {
 await withConnectionTransition(async () => {
  connectionProfiles = await desktopAdapter().GetConnectionProfiles();
  connectionProfilesLoaded = true;
  await connectSelectedProfile({desktopStartup: true});
 });
}
function editConnectionProfile(profile) {
 document.getElementById('connectionID').value = profile?.id || '';
 document.getElementById('connectionName').value = profile?.name || '';
 document.getElementById('connectionURL').value = profile?.base_url || '';
 const apiKeyInput = document.getElementById('connectionAPIKey');
 const apiKeyState = document.getElementById('connectionAPIKeyState');
 const apiKeyConfigured = Boolean(profile?.api_key_configured);
 apiKeyInput.value = '';
 apiKeyInput.placeholder = apiKeyConfigured ? '已配置；留空保留，输入新 Key 可替换' : '远端管理连接必填 Admin Key';
 apiKeyState.dataset.state = apiKeyConfigured ? 'configured' : 'unconfigured';
 apiKeyState.textContent = apiKeyConfigured ? '已保存' : '未配置';
 document.getElementById('connectionAPIKeyHint').textContent = apiKeyConfigured
  ? '客户端 Admin Key 已保存；不会回显。留空会保留原密钥，输入新值会替换。它必须与服务端 Remote access 的 Admin Key 一致。'
  : '远端 Desktop/CLI 管理连接必须配置 Admin Key，并且必须与服务端 Remote access 的 Admin Key 一致。Agent Key 仅供 /mcp 使用。';
 document.getElementById('connectionStartOnDesktopLaunch').checked = Boolean(profile?.start_service_on_desktop_launch);
 document.getElementById('connectionDialogTitle').textContent = profile ? '编辑连接' : '添加连接';
 openEditorDialog('connectionDialog');
}
let renameAction = null;
function openRenameDialog(kind, name, action) {
 renameAction = action;
 document.getElementById('renameDialogTitle').textContent = '重命名 ' + kind;
 document.getElementById('renameName').value = name;
 openEditorDialog('renameDialog');
}
document.addEventListener('DOMContentLoaded', () => {
 document.getElementById('addConnection').addEventListener('click', () => editConnectionProfile(null));
 document.getElementById('editConnection').addEventListener('click', () => editConnectionProfile(activeConnectionProfile()));
 document.getElementById('connectionSelect').addEventListener('change', (event) => {
  const id = event.target.value;
  if (!id) { renderConnectionProfiles(); return; }
  withConnectionTransition(async () => {
   connectionProfiles = await desktopAdapter().SelectConnectionProfile(id);
   await connectSelectedProfile();
  });
 });
 document.getElementById('connectionForm').addEventListener('submit', async (event) => {
  event.preventDefault();
  const form = event.currentTarget;
  const button = form.querySelector('[type="submit"]');
  if (button.disabled) return;
  button.disabled = true;
  const profile = {
   id: document.getElementById('connectionID').value,
   name: document.getElementById('connectionName').value,
   base_url: document.getElementById('connectionURL').value,
   api_key: document.getElementById('connectionAPIKey').value,
   start_service_on_desktop_launch: document.getElementById('connectionStartOnDesktopLaunch').checked,
  };
  try {
   await withConnectionTransition(async () => {
    const prior = activeConnectionProfile();
    connectionProfiles = await desktopAdapter().SaveConnectionProfile(profile);
    connectionProfilesLoaded = true;
    closeFormDialog(form);
    const active = activeConnectionProfile();
    if (active?.base_url !== prior?.base_url) await connectSelectedProfile();
    setStatus('连接已保存', 'success');
   });
  } finally { button.disabled = false; }
 });
 document.getElementById('deleteConnection').addEventListener('click', () => {
  const profile = activeConnectionProfile();
  if (!profile || !window.confirm('删除保存的连接“' + profile.name + '”？不会停止 ADM 服务或删除服务数据。')) return;
  withConnectionTransition(async () => {
   connectionProfiles = await desktopAdapter().DeleteConnectionProfile(profile.id);
   await connectSelectedProfile();
  });
 });
 document.getElementById('renameForm').addEventListener('submit', async (event) => {
  event.preventDefault();
  if (!renameAction) return;
  const action = renameAction;
  const name = document.getElementById('renameName').value.trim();
  if (!name) return;
  await runMutation('保存名称', async () => { await action(name); closeEditorDialog('renameDialog'); });
 });
});
