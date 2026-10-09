// Desktop-only updater. The browser-hosted Gateway management UI must not
// acquire local filesystem or application-install capabilities.
(() => {
  const $ = (id) => document.getElementById(id);
  const panel = $('desktopUpdatePanel');
  if (!panel) return;
  const status = $('desktopUpdateStatus');
  const mode = $('desktopUpdateMode');
  const prerelease = $('desktopUpdatePrerelease');
  const checkButton = $('desktopUpdateCheck');
  const downloadButton = $('desktopUpdateDownload');
  const cancelButton = $('desktopUpdateCancel');
  const openFolderButton = $('desktopUpdateOpenFolder');
  const installButton = $('desktopUpdateInstall');
  const progress = $('desktopUpdateProgress');
  const progressText = $('desktopUpdateProgressText');
  const fileText = $('desktopUpdateFile');
  const releaseLink = $('desktopUpdateRelease');
  let lastState = null;
  let busy = false;
  let activity = '';

  const updateManager = () => window.go?.main?.UpdateManager;
  const failureText = (error) => error?.message || String(error);
  const fmtSize = (bytes) => (Math.max(0, Number(bytes) || 0) / (1024 * 1024)).toFixed(1) + ' MB';

  function showProgress(downloaded, total) {
    progress.hidden = false;
    progressText.hidden = false;
    if (total > 0) {
      const percent = Math.min(100, Math.floor((downloaded / total) * 100));
      progress.value = percent;
      progressText.textContent = `已下载 ${fmtSize(downloaded)} / ${fmtSize(total)}（${percent}%）`;
    } else {
      progress.removeAttribute('value');
      progressText.textContent = `已下载 ${fmtSize(downloaded)}`;
    }
  }

  function render(state) {
    lastState = state;
    mode.textContent = {user: '当前用户安装版', machine: '系统级安装版', portable: '便携版'}[state?.install_mode] || '本机 Desktop';
    const supported = Boolean(state?.supported);
    checkButton.disabled = !supported || busy;
    prerelease.disabled = busy || !supported;
    downloadButton.disabled = !supported || busy || !state?.update_available || state?.phase === 'downloaded';
    cancelButton.hidden = !busy || activity === 'installing';
    openFolderButton.hidden = !(state?.phase === 'downloaded' && state?.download_path && !busy);
    installButton.hidden = !(state?.install_ready && state?.phase === 'downloaded' && !busy);
    if (!supported) {
      status.textContent = state?.current_version === 'dev' ?
        '开发构建不能检查版本更新，请使用发行版。' : '当前平台暂不支持桌面安装器更新。';
    } else if (state?.phase === 'downloaded') {
      status.textContent = state?.install_mode === 'user' ?
        `${state?.latest_version} 已下载并通过 SHA256 校验，可安装并重启。` :
        state?.install_mode === 'machine' ?
          `${state?.latest_version} 已下载并通过 SHA256 校验。系统级安装请手动运行 Setup，可能需要管理员权限。` :
          `${state?.latest_version} 已下载并通过 SHA256 校验。当前为便携版，请手动运行 Setup。`;
    } else if (state?.phase === 'checked') {
      status.textContent = state.update_available ?
        `发现新版本 ${state.latest_version}（当前 ${state.current_version}），可以下载。` :
        `已是当前更新渠道的最新版本：${state.current_version}`;
    } else {
      status.textContent = `当前版本 ${state?.current_version || '—'}。点击“检查更新”查询 GitHub Release。`;
    }
    if (state?.phase === 'downloaded') showProgress(state.downloaded, state.total);
    else if (!busy) {
      progress.hidden = true;
      progressText.hidden = true;
    }
    fileText.hidden = !state?.download_path;
    fileText.textContent = state?.download_path ? `已校验的文件：${state.download_path}` : '';
    const url = String(state?.release_page || '');
    const valid = /^https:\/\/github\.com\/wanstu\/ai-dev-manager\/releases\/tag\//.test(url);
    releaseLink.hidden = !valid;
    if (valid) releaseLink.href = url;
  }

  function setBusy(next, nextActivity = '') {
    busy = next;
    activity = next ? nextActivity : '';
    if (lastState) render(lastState);
    else {
      checkButton.disabled = next;
      downloadButton.disabled = true;
    }
  }

  async function reload() {
    const api = updateManager();
    if (!api || window.ADMWebSurface || document.body.dataset.surface === 'web') {
      panel.hidden = true;
      return;
    }
    const state = await api.GetDesktopUpdateStatus();
    prerelease.checked = /-rc\.|-(beta|alpha)/i.test(state.current_version || '');
    render(state);
  }

  checkButton.addEventListener('click', async () => {
    const api = updateManager();
    if (!api || busy) return;
    setBusy(true, 'checking');
    status.textContent = '正在检查 GitHub Release…';
    try {
      const result = await api.CheckDesktopUpdate(Boolean(prerelease.checked));
      setBusy(false);
      render(result);
    } catch (error) {
      setBusy(false);
      status.textContent = /cancell?ed|context canceled/i.test(failureText(error)) ? '已取消检查更新。' : '检查更新失败：' + failureText(error);
    }
  });

  downloadButton.addEventListener('click', async () => {
    const api = updateManager();
    if (!api || busy || !lastState?.update_available) return;
    setBusy(true, 'downloading');
    showProgress(0, 0);
    status.textContent = '正在安全下载更新安装包…';
    try {
      const result = await api.DownloadDesktopUpdate();
      setBusy(false);
      render(result);
    } catch (error) {
      setBusy(false);
      progress.hidden = true;
      progressText.hidden = true;
      status.textContent = /cancell?ed|context canceled/i.test(failureText(error)) ? '已取消下载，未应用更新。' : '下载失败：' + failureText(error);
    }
  });

  cancelButton.addEventListener('click', async () => {
    try {
      await updateManager()?.CancelDesktopUpdate();
      status.textContent = '正在取消下载…';
    } catch (error) {
      status.textContent = '取消失败：' + failureText(error);
    }
  });

  openFolderButton.addEventListener('click', async () => {
    try {
      await updateManager()?.OpenDesktopUpdateFolder();
    } catch (error) {
      status.textContent = '无法打开安装包目录：' + failureText(error);
    }
  });

  installButton.addEventListener('click', async () => {
    if (busy || !lastState?.install_ready) return;
    if (!window.confirm(`安装 ${lastState.latest_version} 并重启 ADM Desktop？当前 Gateway 后台服务不会停止。`)) return;
    setBusy(true, 'installing');
    status.textContent = '正在启动安装器，完成后将重新启动 Desktop…';
    try {
      await updateManager().InstallDesktopUpdate();
    } catch (error) {
      setBusy(false);
      status.textContent = '安装未启动：' + failureText(error);
    }
  });

  window.addEventListener('DOMContentLoaded', () => {
    window.runtime?.EventsOn?.('desktop:update-progress', (value) => {
      if (!busy) return;
      showProgress(value?.downloaded || 0, value?.total || 0);
    });
    reload().catch((error) => {
      checkButton.disabled = true;
      status.textContent = '读取更新状态失败：' + failureText(error);
    });
  });
})();
