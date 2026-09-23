(() => {
  const title = document.getElementById('authTitle');
  const description = document.getElementById('authDescription');
  const form = document.getElementById('authForm');
  const blocked = document.getElementById('authBlocked');
  const message = document.getElementById('authMessage');
  const submit = document.getElementById('submitButton');
  let mode = 'login';

  async function request(path, options = {}) {
    const response = await fetch(path, {
      credentials: 'same-origin',
      ...options,
      headers: {'Content-Type': 'application/json', 'X-ADM-Web': '1', ...(options.headers || {})},
    });
    const body = response.status === 204 ? null : await response.json().catch(() => ({}));
    if (!response.ok) throw new Error(body?.error || ('HTTP ' + response.status));
    return body;
  }

  async function load() {
    try {
      const status = await fetch('/api/web/auth/status', {credentials: 'same-origin'}).then(async (response) => {
        if (!response.ok) throw new Error('HTTP ' + response.status);
        return response.json();
      });
      if (status.authenticated) {
        location.replace('/');
        return;
      }
      if (!status.initialized) {
        mode = 'register';
        title.textContent = '初始化 ADM';
        description.textContent = '创建第一个管理员账号。创建成功后普通注册入口会永久关闭。';
        document.getElementById('password').autocomplete = 'new-password';
        submit.textContent = '创建管理员';
        if (!status.registration_allowed) {
          blocked.hidden = false;
          form.hidden = true;
          return;
        }
      } else {
        mode = 'login';
        title.textContent = '登录 ADM';
        description.textContent = '使用管理员账号进入 Web 管理台。';
        submit.textContent = '登录';
      }
      blocked.hidden = true;
      form.hidden = false;
      document.getElementById('username').focus();
    } catch (error) {
      title.textContent = '无法连接 ADM';
      description.textContent = 'Web 管理状态读取失败。';
      message.textContent = error?.message || String(error);
    }
  }

  form.addEventListener('submit', async (event) => {
    event.preventDefault();
    submit.disabled = true;
    message.textContent = '';
    try {
      await request(mode === 'register' ? '/api/web/auth/register' : '/api/web/auth/login', {
        method: 'POST',
        body: JSON.stringify({
          username: document.getElementById('username').value.trim(),
          password: document.getElementById('password').value,
        }),
      });
      location.replace('/');
    } catch (error) {
      message.textContent = error?.message || String(error);
    } finally {
      submit.disabled = false;
    }
  });

  load();
})();
