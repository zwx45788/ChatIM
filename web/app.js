const API_BASE = '/api/v1';
const state = { token: null };

const tokenDisplay = document.querySelector('#tokenDisplay');
const logBox = document.querySelector('#log');

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

async function request(path, { method = 'GET', body, auth = true } = {}) {
  const headers = { 'Content-Type': 'application/json' };
  if (auth && state.token) headers.Authorization = `Bearer ${state.token}`;
  const res = await fetch(`${API_BASE}${path}`, {
    method,
    headers,
    body: body ? JSON.stringify(body) : undefined,
  });
  const data = await res.json().catch(() => ({}));
  if (!res.ok) throw new Error(data.message || data.error || res.statusText);
  return data;
}

function updateToken(token) {
  state.token = token;
  tokenDisplay.textContent = token || '未登录';
}

// 登录
async function handleLogin() {
  const username = document.querySelector('#username').value.trim();
  const password = document.querySelector('#password').value.trim();
  if (!username || !password) {
    log('登录失败', '用户名和密码不能为空');
    return;
  }
  try {
    const res = await request('/login', {
      method: 'POST',
      body: { username, password },
      auth: false,
    });
    const token = res.token || res.data?.token || res.Token || res.access_token;
    updateToken(token);
    log('登录成功', res);
  } catch (err) {
    log('登录失败', err.message);
  }
}

// 当前用户
async function handleMe() {
  try {
    const res = await request('/users/me');
    log('当前用户', res);
  } catch (err) {
    log('获取用户失败', err.message);
  }
}

// 发送私聊
async function handleSend() {
  const to = document.querySelector('#toUserId').value.trim();
  const content = document.querySelector('#messageContent').value.trim();
  if (!to || !content) return log('发送失败', '收件人或内容为空');
  try {
    const res = await request('/messages/send', {
      method: 'POST',
      body: { to_user_id: to, content },
    });
    log('消息已发送', res);
  } catch (err) {
    log('发送失败', err.message);
  }
}

// 拉取未读（私聊+群）
async function handleUnread() {
  try {
    const res = await request('/unread/all');
    log('未读消息', res);
  } catch (err) {
    log('拉取未读失败', err.message);
  }
}

// 标记私聊已读
async function handleMarkRead() {
  const raw = document.querySelector('#readIds').value.trim();
  if (!raw) return log('标记失败', '请填写消息ID');
  const ids = raw.split(',').map((s) => s.trim()).filter(Boolean);
  try {
    const res = await request('/messages/read', {
      method: 'POST',
      body: { message_ids: ids },
    });
    log('私聊已读', res);
  } catch (err) {
    log('标记失败', err.message);
  }
}

// 群聊未读（简单拉取单群）
async function handleGroupUnread() {
  const gid = document.querySelector('#groupId').value.trim();
  if (!gid) return log('群未读失败', '群ID不能为空');
  try {
    // 后端有 PullAllGroupsUnreadMessages，API 网关未暴露单群接口，这里调用统一未读接口
    const res = await request('/unread/all');
    log('群未读', res.group_messages || res);
  } catch (err) {
    log('群未读失败', err.message);
  }
}

// 事件绑定
window.addEventListener('DOMContentLoaded', () => {
  document.querySelector('#btnLogin').addEventListener('click', handleLogin);
  document.querySelector('#btnMe').addEventListener('click', handleMe);
  document.querySelector('#btnSend').addEventListener('click', handleSend);
  document.querySelector('#btnUnread').addEventListener('click', handleUnread);
  document.querySelector('#btnMarkRead').addEventListener('click', handleMarkRead);
  document.querySelector('#btnGroupUnread').addEventListener('click', handleGroupUnread);
});
