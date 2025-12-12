const state = {
  baseUrl: localStorage.getItem('chatim_base') || 'http://localhost:8081/api/v1',
  token: localStorage.getItem('chatim_token') || '',
};

const tokenDisplay = document.querySelector('#tokenDisplay');
const logBox = document.querySelector('#log');
const baseInput = document.querySelector('#baseUrl');

baseInput.value = state.baseUrl;
updateToken(state.token);

function log(title, data) {
  const entry = document.createElement('div');
  entry.className = 'log-entry';
  const now = new Date().toLocaleTimeString();
  entry.innerHTML = `<div class="time">${now} · ${title}</div>`;
  if (data !== undefined) {
    const pre = document.createElement('pre');
    pre.textContent = typeof data === 'string' ? data : JSON.stringify(data, null, 2);
    entry.appendChild(pre);
  }
  logBox.appendChild(entry);
  logBox.scrollTop = logBox.scrollHeight;
}

function updateToken(token) {
  state.token = token || '';
  tokenDisplay.textContent = state.token || '未登录';
  if (token) localStorage.setItem('chatim_token', token); else localStorage.removeItem('chatim_token');
}

function saveBaseUrl() {
  const val = baseInput.value.trim();
  if (!val) return log('保存失败', '基础地址不能为空');
  state.baseUrl = val;
  localStorage.setItem('chatim_base', val);
  log('基础地址已保存', val);
}

document.querySelector('#saveBase').addEventListener('click', saveBaseUrl);

async function request(path, { method = 'GET', body, auth = true } = {}) {
  const headers = { 'Content-Type': 'application/json' };
  if (auth && state.token) headers.Authorization = `Bearer ${state.token}`;
  const res = await fetch(`${state.baseUrl}${path}`, {
    method,
    headers,
    body: body ? JSON.stringify(body) : undefined,
  });
  let data = {};
  try { data = await res.json(); } catch (_) {}
  if (!res.ok || data.code && data.code !== 0) {
    throw new Error(data.message || data.error || res.statusText);
  }
  return data;
}

async function handleLogin() {
  const username = document.querySelector('#username').value.trim();
  const password = document.querySelector('#password').value.trim();
  if (!username || !password) return log('登录失败', '用户名和密码不能为空');
  try {
    const res = await request('/login', { method: 'POST', body: { username, password }, auth: false });
    const token = res.token || res.Token || res.data?.token || res.data?.Token;
    updateToken(token);
    log('登录成功', res);
  } catch (err) {
    log('登录失败', err.message);
  }
}

async function handleMe() {
  try {
    const res = await request('/users/me');
    log('当前用户', res);
  } catch (err) {
    log('获取用户失败', err.message);
  }
}

async function handleSend() {
  const to = document.querySelector('#toUserId').value.trim();
  const content = document.querySelector('#messageContent').value.trim();
  if (!to || !content) return log('发送失败', '收件人或内容为空');
  try {
    const res = await request('/messages/send', { method: 'POST', body: { to_user_id: to, content } });
    log('消息已发送', res);
  } catch (err) {
    log('发送失败', err.message);
  }
}

async function handleUnread() {
  try {
    const res = await request('/unread/all');
    log('未读消息', res);
  } catch (err) {
    log('拉取未读失败', err.message);
  }
}

async function handleMarkRead() {
  const raw = document.querySelector('#readIds').value.trim();
  if (!raw) return log('标记失败', '请填写消息ID');
  const ids = raw.split(',').map((s) => s.trim()).filter(Boolean);
  try {
    const res = await request('/messages/read', { method: 'POST', body: { message_ids: ids } });
    log('私聊已读', res);
  } catch (err) {
    log('标记失败', err.message);
  }
}

async function handleGroupUnread() {
  const gid = document.querySelector('#groupId').value.trim();
  if (!gid) return log('群未读提示', '填写群ID用于筛选查看');
  try {
    const res = await request('/unread/all');
    const groups = res.group_messages || res.groupMessages || {};
    const one = groups[gid] || { notice: '后端未暴露单群接口，当前从聚合里取指定群' };
    log('群未读', one);
  } catch (err) {
    log('群未读失败', err.message);
  }
}

window.addEventListener('DOMContentLoaded', () => {
  document.querySelector('#btnLogin').addEventListener('click', handleLogin);
  document.querySelector('#btnMe').addEventListener('click', handleMe);
  document.querySelector('#btnSend').addEventListener('click', handleSend);
  document.querySelector('#btnUnread').addEventListener('click', handleUnread);
  document.querySelector('#btnMarkRead').addEventListener('click', handleMarkRead);
  document.querySelector('#btnGroupUnread').addEventListener('click', handleGroupUnread);
});
