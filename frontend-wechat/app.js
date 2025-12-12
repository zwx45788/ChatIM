const state = {
  baseUrl: localStorage.getItem('chatim_base') || 'http://localhost:8081/api/v1',
  token: localStorage.getItem('chatim_token') || '',
  me: null,
  chats: {}, // id -> {id,name,unread,count,messages:[]}
  activeId: null,
};

const el = {
  overlay: document.querySelector('#loginOverlay'),
  baseUrl: document.querySelector('#baseUrl'),
  username: document.querySelector('#username'),
  password: document.querySelector('#password'),
  tokenDisplay: document.querySelector('#tokenDisplay'),
  meName: document.querySelector('#meName'),
  meId: document.querySelector('#meId'),
  chatList: document.querySelector('#chatList'),
  activeName: document.querySelector('#activeName'),
  activeId: document.querySelector('#activeId'),
  messageList: document.querySelector('#messageList'),
  messageInput: document.querySelector('#messageInput'),
  contactInput: document.querySelector('#contactInput'),
};

// init base
el.baseUrl.value = state.baseUrl;
updateToken(state.token);
renderChats();
renderMessages();

function log(msg) { console.log('[chatim]', msg); }

function updateToken(token) {
  state.token = token || '';
  if (token) localStorage.setItem('chatim_token', token); else localStorage.removeItem('chatim_token');
  el.tokenDisplay.textContent = token || '未登录';
}

function saveBase() {
  const v = el.baseUrl.value.trim();
  if (!v) return;
  state.baseUrl = v;
  localStorage.setItem('chatim_base', v);
  log('Base saved: ' + v);
}

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
  if (!res.ok || (data.code !== undefined && data.code !== 0)) {
    throw new Error(data.message || data.error || res.statusText);
  }
  return data;
}

async function login() {
  saveBase();
  const u = el.username.value.trim();
  const p = el.password.value.trim();
  if (!u || !p) return alert('请输入用户名/密码');
  try {
    const res = await request('/login', { method: 'POST', body: { username: u, password: p }, auth: false });
    const token = res.token || res.Token || res.data?.token || res.data?.Token;
    if (!token) throw new Error('未返回 token');
    updateToken(token);
    state.me = await getMe();
    el.meName.textContent = state.me?.username || '已登录';
    el.meId.textContent = state.me?.id || '';
    el.overlay.classList.add('hidden');
  } catch (e) {
    alert('登录失败: ' + e.message);
  }
}

async function getMe() {
  const res = await request('/users/me');
  return res.user || res.data || res;
}

function ensureChat(id, name) {
  if (!id) return null;
  if (!state.chats[id]) {
    state.chats[id] = { id, name: name || id, messages: [], unread: 0 };
  }
  return state.chats[id];
}

function setActive(id) {
  state.activeId = id;
  renderChats();
  renderMessages();
}

function renderChats() {
  el.chatList.innerHTML = '';
  const entries = Object.values(state.chats);
  entries.sort((a,b)=> (b.lastTime||0)-(a.lastTime||0));
  entries.forEach(chat => {
    const div = document.createElement('div');
    div.className = 'chat-item' + (chat.id === state.activeId ? ' active' : '');
    div.innerHTML = `
      <div class="top">
        <div class="title">${chat.name || chat.id}</div>
        ${chat.unread ? `<span class="unread">${chat.unread}</span>` : ''}
      </div>
      <div class="preview">${(chat.lastPreview || '...')}</div>
    `;
    div.addEventListener('click', () => {
      chat.unread = 0;
      setActive(chat.id);
    });
    el.chatList.appendChild(div);
  });
}

function renderMessages() {
  const chat = state.chats[state.activeId];
  if (!chat) {
    el.activeName.textContent = '请选择会话';
    el.activeId.textContent = '';
    el.messageList.innerHTML = '';
    return;
  }
  el.activeName.textContent = chat.name || chat.id;
  el.activeId.textContent = chat.id;
  el.messageList.innerHTML = '';
  chat.messages.forEach(m => {
    const div = document.createElement('div');
    div.className = 'msg' + (m.me ? ' me' : '');
    div.innerHTML = `
      <div class="meta">${m.from} · ${new Date(m.ts*1000).toLocaleString()} ${m.id ? '#'+m.id : ''}</div>
      <div class="content">${m.content || ''}</div>
    `;
    el.messageList.appendChild(div);
  });
  el.messageList.scrollTop = el.messageList.scrollHeight;
}

async function sendMessage() {
  const chat = state.chats[state.activeId];
  if (!chat) return alert('请选择会话');
  const text = el.messageInput.value.trim();
  if (!text) return;
  try {
    const res = await request('/messages/send', { method: 'POST', body: { to_user_id: chat.id, content: text } });
    const msg = res.msg || res.data || {};
    chat.messages.push({ id: msg.id, content: text, from: state.me?.id || 'me', ts: Math.floor(Date.now()/1000), me: true });
    chat.lastPreview = text;
    chat.lastTime = Date.now();
    el.messageInput.value = '';
    renderMessages();
    renderChats();
  } catch (e) {
    alert('发送失败: ' + e.message);
  }
}

async function pullUnread() {
  try {
    const res = await request('/unread/all');
    const meId = state.me?.id;
    const privates = res.private_messages || res.PrivateMessages || [];
    privates.forEach(m => {
      const peer = m.from_user_id === meId ? m.to_user_id : m.from_user_id;
      const chat = ensureChat(peer, peer);
      chat.messages.push({ id: m.id, content: m.content, from: m.from_user_id, ts: m.created_at || Math.floor(Date.now()/1000), me: m.from_user_id === meId });
      chat.unread = (chat.unread || 0) + (m.from_user_id === meId ? 0 : 1);
      chat.lastPreview = m.content;
      chat.lastTime = Date.now();
    });
    const groups = res.group_messages || {};
    Object.keys(groups).forEach(gid => {
      const info = groups[gid];
      (info.messages || []).forEach(m => {
        const name = `群 ${gid}`;
        const chat = ensureChat(gid, name);
        chat.messages.push({ id: m.id, content: m.content, from: m.from_user_id, ts: m.created_at || Math.floor(Date.now()/1000), me: m.from_user_id === meId });
        chat.unread = (chat.unread || 0) + (m.from_user_id === meId ? 0 : 1);
        chat.lastPreview = m.content;
        chat.lastTime = Date.now();
      });
    });
    renderChats();
    renderMessages();
  } catch (e) {
    alert('拉取未读失败: ' + e.message);
  }
}

async function markCurrentRead() {
  const chat = state.chats[state.activeId];
  if (!chat) return;
  const ids = chat.messages.filter(m => !m.me && m.id).map(m => m.id);
  if (!ids.length) { alert('没有可标记的消息'); return; }
  try {
    await request('/messages/read', { method: 'POST', body: { message_ids: ids } });
    chat.unread = 0;
    renderChats();
  } catch (e) {
    alert('标记失败: ' + e.message);
  }
}

function addContactFromInput(e) {
  if (e.key === 'Enter') {
    const v = el.contactInput.value.trim();
    if (!v) return;
    ensureChat(v, v);
    el.contactInput.value = '';
    renderChats();
    setActive(v);
  }
}

// events
el.baseUrl.addEventListener('blur', saveBase);
document.querySelector('#btnLogin').addEventListener('click', login);
document.querySelector('#btnShowLogin').addEventListener('click', () => el.overlay.classList.remove('hidden'));
document.querySelector('#btnCloseLogin').addEventListener('click', () => el.overlay.classList.add('hidden'));
document.querySelector('#btnSend').addEventListener('click', sendMessage);
document.querySelector('#btnPullUnread').addEventListener('click', pullUnread);
document.querySelector('#btnMarkRead').addEventListener('click', markCurrentRead);
el.contactInput.addEventListener('keydown', addContactFromInput);
el.messageInput.addEventListener('keydown', (e) => { if (e.key === 'Enter' && !e.shiftKey) { e.preventDefault(); sendMessage(); } });

// 如果已有 token，尝试获取用户信息
(async () => {
  if (state.token) {
    try {
      state.me = await getMe();
      el.meName.textContent = state.me?.username || '已登录';
      el.meId.textContent = state.me?.id || '';
    } catch (_) {
      updateToken('');
    }
  } else {
    el.overlay.classList.remove('hidden');
  }
})();
