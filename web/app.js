/* ChatIM Web - User UI (no build) */

const storageKeys = {
  apiBase: 'chatim.apiBase',
  token: 'chatim.token',
  me: 'chatim.me',
};

const $ = (id) => document.getElementById(id);

const state = {
  view: 'chats',
  me: null,
  conversations: [],
  friends: [],
  groups: [],
  selectedConversationId: null,
  selectedConversation: null,
  messages: new Map(), // conversation_id -> array
};

let ws = null;

function setBanner(kind, text) {
  const banner = $('banner');
  if (!text) {
    banner.style.display = 'none';
    banner.textContent = '';
    banner.className = 'banner';
    return;
  }
  banner.style.display = 'block';
  banner.textContent = text;
  banner.className = `banner ${kind === 'ok' ? 'ok' : kind === 'bad' ? 'bad' : ''}`;
}

function normalizeApiBase(value) {
  let base = (value || '').trim();
  if (!base) base = '/api/v1';
  base = base.replace(/\/+$/, '');
  return base;
}

function getApiBase() {
  return normalizeApiBase($('apiBase').value);
}

function setApiBase(base) {
  const b = normalizeApiBase(base);
  $('apiBase').value = b;
  $('apiBaseLabel').textContent = b;
  localStorage.setItem(storageKeys.apiBase, b);
}

function getTokenRaw() {
  return ($('token').value || '').trim();
}

function getTokenBearer() {
  const raw = getTokenRaw();
  if (!raw) return '';
  return raw.startsWith('Bearer ') ? raw : `Bearer ${raw}`;
}

function setToken(rawOrToken) {
  const t = (rawOrToken || '').trim();
  $('token').value = t;
  localStorage.setItem(storageKeys.token, t);
}

function clearAuth() {
  setToken('');
  localStorage.removeItem(storageKeys.me);
  state.me = null;
  state.conversations = [];
  state.messages.clear();
  state.selectedConversationId = null;
  state.selectedConversation = null;
  wsDisconnect();
  renderAuth();
}

function buildUrl(path, query) {
  const apiBase = getApiBase();
  let url = `${apiBase}${path.startsWith('/') ? '' : '/'}${path}`;
  if (query) {
    const q = query.toString().replace(/^\?/, '');
    if (q) url += `?${q}`;
  }
  return url;
}

async function apiFetch(path, options = {}) {
  const headers = new Headers(options.headers || {});
  if (!headers.has('Content-Type') && options.body && typeof options.body === 'string') {
    headers.set('Content-Type', 'application/json');
  }

  const token = getTokenBearer();
  if (token) headers.set('Authorization', token);

  const resp = await fetch(buildUrl(path, options.query), {
    method: options.method || 'GET',
    headers,
    body: options.body,
  });

  const contentType = resp.headers.get('content-type') || '';
  const text = await resp.text();

  let data;
  if (contentType.includes('application/json')) {
    try { data = text ? JSON.parse(text) : null; } catch { data = { _raw: text }; }
  } else {
    data = { _raw: text };
  }

  return { ok: resp.ok, status: resp.status, data };
}

function extractApiError(res) {
  const data = res?.data;
  if (!data) return `HTTP ${res?.status || ''}`.trim();
  if (typeof data === 'string') return data;
  if (data.error) return String(data.error);
  if (data.message && data.code && data.code !== 0) return String(data.message);
  if (data.msg && data.code && data.code !== 0) return String(data.msg);
  try {
    return JSON.stringify(data);
  } catch {
    return String(data);
  }
}

function setWsState(label, kind) {
  $('wsState').textContent = label;
  const dot = $('wsDot');
  dot.className = 'dot';
  if (kind === 'ok') dot.classList.add('ok');
  if (kind === 'bad') dot.classList.add('bad');
}

function wsConnect() {
  if (ws && ws.readyState === WebSocket.OPEN) return;

  const t = getTokenBearer().replace(/^Bearer\s+/i, '');
  if (!t) {
    setBanner('bad', '请先登录后再连接 WebSocket');
    return;
  }

  const scheme = location.protocol === 'https:' ? 'wss' : 'ws';
  const url = `${scheme}://${location.host}/ws?token=${encodeURIComponent(t)}`;
  ws = new WebSocket(url);

  setWsState('CONNECTING', null);

  ws.onopen = () => {
    setWsState('CONNECTED', 'ok');
  };

  ws.onclose = () => {
    setWsState('DISCONNECTED', null);
  };

  ws.onerror = () => {
    setWsState('ERROR', 'bad');
  };

  ws.onmessage = (evt) => {
    let payload = evt.data;
    try { payload = JSON.parse(evt.data); } catch { /* keep string */ }
    handleWsMessage(payload);
  };
}

function wsDisconnect() {
  if (ws) {
    try { ws.close(); } catch { /* noop */ }
  }
  ws = null;
  setWsState('DISCONNECTED', null);
}

function toConversationIdFromPush(push) {
  if (!push || typeof push !== 'object') return null;
  if (push.type === 'group' && push.group_id) return `group:${push.group_id}`;
  if (push.type === 'private' && push.from_user_id) return `private:${push.from_user_id}`;
  return null;
}

function handleWsMessage(push) {
  const convId = toConversationIdFromPush(push);
  if (!convId) return;

  const list = state.messages.get(convId) || [];
  const item = {
    id: push.id,
    type: push.type,
    from_user_id: push.from_user_id,
    to_user_id: push.to_user_id,
    group_id: push.group_id,
    content: push.content,
    created_at: push.created_at,
  };
  list.push(item);
  state.messages.set(convId, list);

  if (state.selectedConversationId === convId) {
    renderMessages(convId);
  }

  // best effort: refresh conversation list unread/last message
  refreshConversations().catch(() => {});
}

function formatTime(value) {
  if (!value) return '';
  // Accept seconds, milliseconds, or RFC3339 string
  if (typeof value === 'string') {
    const d = new Date(value);
    if (!Number.isNaN(d.getTime())) return d.toLocaleString();
    return value;
  }
  const n = Number(value);
  if (!Number.isFinite(n)) return '';
  const ms = n > 1e12 ? n : n * 1000;
  const d = new Date(ms);
  return d.toLocaleString();
}

function setMeLabel() {
  if (!state.me) {
    $('meLabel').textContent = '(未登录)';
    $('chipMe').style.display = 'none';
    return;
  }
  $('chipMe').style.display = 'inline-flex';
  const username = state.me.username || '';
  const id = state.me.user_id || state.me.userId || '';
  $('meLabel').textContent = `${username}${id ? `(${id.slice(0, 8)})` : ''}`;
}

function renderAuth() {
  const authed = !!getTokenRaw();
  $('authView').style.display = authed ? 'none' : 'flex';
  $('appView').style.display = authed ? 'grid' : 'none';
  setMeLabel();

  $('btnPin').disabled = !state.selectedConversationId;
  $('btnDeleteConv').disabled = !state.selectedConversationId;
  $('btnSendMsg').disabled = !state.selectedConversationId;
}

function setView(view) {
  state.view = view;
  for (const b of document.querySelectorAll('.sideTab')) {
    b.setAttribute('aria-selected', b.dataset.view === view ? 'true' : 'false');
  }

  $('chatView').style.display = view === 'chats' ? 'flex' : 'none';
  $('friendsView').style.display = view === 'friends' ? 'block' : 'none';
  $('groupsView').style.display = view === 'groups' ? 'block' : 'none';
  $('searchView').style.display = view === 'search' ? 'block' : 'none';

  $('sideSearchLabel').textContent = view === 'chats' ? '筛选会话' : view === 'friends' ? '筛选好友' : view === 'groups' ? '筛选群组' : '筛选';
  $('sideSearch').value = '';
  renderSideList();

  if (view === 'friends') {
    refreshFriends().catch(() => {});
    refreshFriendRequests().catch(() => {});
  }
  if (view === 'groups') {
    refreshGroups().catch(() => {});
  }
}

function renderSideList() {
  const root = $('sideList');
  root.innerHTML = '';
  const q = ($('sideSearch').value || '').trim().toLowerCase();

  if (state.view === 'chats') {
    const items = state.conversations
      .filter(c => !q || String(c.title || '').toLowerCase().includes(q) || String(c.peer_id || '').toLowerCase().includes(q))
      .sort((a, b) => (b.is_pinned === true) - (a.is_pinned === true) || (b.last_message_time || 0) - (a.last_message_time || 0));

    for (const c of items) {
      const el = document.createElement('div');
      el.className = 'sideItem';
      el.dataset.id = c.conversation_id;
      if (c.conversation_id === state.selectedConversationId) {
        el.style.borderColor = 'rgba(79,140,255,.6)';
      }

      const top = document.createElement('div');
      top.className = 'sideItemTop';
      const title = document.createElement('div');
      title.className = 'sideItemTitle';
      title.textContent = c.title || c.peer_id || c.conversation_id;

      const right = document.createElement('div');
      right.style.display = 'flex';
      right.style.gap = '6px';
      right.style.alignItems = 'center';

      if (c.is_pinned) {
        const b = document.createElement('span');
        b.className = 'badge';
        b.textContent = '置顶';
        right.appendChild(b);
      }
      if (c.unread_count && c.unread_count > 0) {
        const b = document.createElement('span');
        b.className = 'badge red';
        b.textContent = String(c.unread_count);
        right.appendChild(b);
      }

      top.appendChild(title);
      top.appendChild(right);

      const sub = document.createElement('div');
      sub.className = 'sideItemSub';
      sub.textContent = c.last_message ? `${c.last_message}` : (c.type === 'group' ? '群聊' : '私聊');

      el.appendChild(top);
      el.appendChild(sub);

      el.addEventListener('click', () => selectConversation(c.conversation_id));
      root.appendChild(el);
    }

    if (!items.length) {
      const empty = document.createElement('div');
      empty.className = 'small muted';
      empty.textContent = '暂无会话';
      root.appendChild(empty);
    }
    return;
  }

  if (state.view === 'friends') {
    const items = state.friends.filter(f => !q || String(f.nickname || '').toLowerCase().includes(q) || String(f.username || '').toLowerCase().includes(q));
    for (const f of items) {
      const el = document.createElement('div');
      el.className = 'sideItem';
      const top = document.createElement('div');
      top.className = 'sideItemTop';
      const title = document.createElement('div');
      title.className = 'sideItemTitle';
      title.textContent = f.nickname ? `${f.nickname}（${f.username}）` : f.username;
      const right = document.createElement('div');
      right.className = 'mono';
      right.textContent = (f.user_id || '').slice(0, 8);
      top.appendChild(title);
      top.appendChild(right);
      const sub = document.createElement('div');
      sub.className = 'sideItemSub';
      sub.textContent = '点击开始私聊';
      el.appendChild(top);
      el.appendChild(sub);
      el.addEventListener('click', () => {
        setView('chats');
        selectConversation(`private:${f.user_id}`);
      });
      root.appendChild(el);
    }
    if (!items.length) {
      const empty = document.createElement('div');
      empty.className = 'small muted';
      empty.textContent = '暂无好友';
      root.appendChild(empty);
    }
    return;
  }

  if (state.view === 'groups') {
    const items = state.groups.filter(g => !q || String(g.name || '').toLowerCase().includes(q) || String(g.id || '').toLowerCase().includes(q));
    for (const g of items) {
      const el = document.createElement('div');
      el.className = 'sideItem';
      const top = document.createElement('div');
      top.className = 'sideItemTop';
      const title = document.createElement('div');
      title.className = 'sideItemTitle';
      title.textContent = g.name || g.id;
      const right = document.createElement('div');
      right.className = 'badge';
      right.textContent = `${g.member_count || 0}人`;
      top.appendChild(title);
      top.appendChild(right);
      const sub = document.createElement('div');
      sub.className = 'sideItemSub';
      sub.textContent = '点击进入群聊';
      el.appendChild(top);
      el.appendChild(sub);
      el.addEventListener('click', () => {
        setView('chats');
        selectConversation(`group:${g.id}`);
      });
      root.appendChild(el);
    }
    if (!items.length) {
      const empty = document.createElement('div');
      empty.className = 'small muted';
      empty.textContent = '暂无群组';
      root.appendChild(empty);
    }
    return;
  }

  const hint = document.createElement('div');
  hint.className = 'small muted';
  hint.textContent = '使用右侧搜索面板进行查询';
  root.appendChild(hint);
}

function renderMainHeader() {
  if (!state.selectedConversationId) {
    $('mainTitle').textContent = '选择一个会话';
    $('mainSubtitle').textContent = '消息会在这里显示';
    $('btnPin').disabled = true;
    $('btnDeleteConv').disabled = true;
    $('btnSendMsg').disabled = true;
    return;
  }

  const c = state.selectedConversation;
  $('btnPin').disabled = false;
  $('btnDeleteConv').disabled = false;
  $('btnSendMsg').disabled = false;

  const title = c?.title || c?.peer_id || state.selectedConversationId;
  $('mainTitle').textContent = title;
  $('mainSubtitle').textContent = c?.type === 'group' ? `群聊：${c?.peer_id || ''}` : `私聊：${c?.peer_id || ''}`;
}

function renderMessages(conversationId) {
  const root = $('chatMessages');
  root.innerHTML = '';
  if (!conversationId) return;

  const msgs = state.messages.get(conversationId) || [];
  if (!msgs.length) {
    const empty = document.createElement('div');
    empty.className = 'small muted';
    empty.textContent = '暂无消息（或当前接口未返回历史消息）';
    root.appendChild(empty);
    return;
  }

  const myId = state.me?.user_id || state.me?.userId;
  for (const m of msgs) {
    const row = document.createElement('div');
    row.className = 'msgRow' + (myId && m.from_user_id === myId ? ' me' : '');

    const bubble = document.createElement('div');
    bubble.className = 'bubble';

    const meta = document.createElement('div');
    meta.className = 'bubbleMeta';

    const left = document.createElement('div');
    const from = m.from_user_name || (m.from_user_id ? String(m.from_user_id).slice(0, 8) : '');
    left.textContent = from;

    const right = document.createElement('div');
    right.textContent = formatTime(m.created_at);

    meta.appendChild(left);
    meta.appendChild(right);

    const text = document.createElement('div');
    text.className = 'bubbleText';
    text.textContent = m.content || '';

    bubble.appendChild(meta);
    bubble.appendChild(text);
    row.appendChild(bubble);
    root.appendChild(row);
  }

  // scroll to bottom
  root.scrollTop = root.scrollHeight;
}

function pickConversationFromId(conversationId) {
  const c = state.conversations.find(x => x.conversation_id === conversationId);
  if (c) return c;

  // build a fallback object
  if (conversationId.startsWith('private:')) {
    return { conversation_id: conversationId, type: 'private', peer_id: conversationId.slice('private:'.length), title: conversationId.slice('private:'.length) };
  }
  if (conversationId.startsWith('group:')) {
    return { conversation_id: conversationId, type: 'group', peer_id: conversationId.slice('group:'.length), title: conversationId.slice('group:'.length) };
  }
  return { conversation_id: conversationId, type: 'private', peer_id: '', title: conversationId };
}

async function selectConversation(conversationId) {
  state.selectedConversationId = conversationId;
  state.selectedConversation = pickConversationFromId(conversationId);

  renderSideList();
  renderMainHeader();
  await refreshMessagesFor(conversationId);
}

async function refreshConversations() {
  const res = await apiFetch('/conversations');
  if (!res.ok) {
    if (res.status === 401) {
      clearAuth();
      return;
    }
    setBanner('bad', `获取会话列表失败（HTTP ${res.status}）`);
    return;
  }

  if (res.data && res.data.code && res.data.code !== 0) {
    setBanner('bad', `获取会话列表失败：${res.data.message || 'error'}`);
    return;
  }

  const list = res.data?.conversations || [];
  state.conversations = list;

  // keep selected conversation object fresh
  if (state.selectedConversationId) {
    state.selectedConversation = pickConversationFromId(state.selectedConversationId);
  }

  renderSideList();
  renderMainHeader();
}

async function refreshMessagesFor(conversationId) {
  if (!conversationId) return;

  const res = await apiFetch('/messages', {
    query: new URLSearchParams({ limit: '50', auto_mark: 'false', include_read: 'true' }),
  });

  if (!res.ok) {
    if (res.status === 401) {
      clearAuth();
      return;
    }
    setBanner('bad', `拉取消息失败（HTTP ${res.status}）`);
    return;
  }

  if (res.data && res.data.code && res.data.code !== 0) {
    setBanner('bad', `拉取消息失败：${res.data.message || 'error'}`);
    return;
  }

  const groups = res.data?.conversations || [];
  const target = groups.find(x => x.conversation_id === conversationId);
  if (target && Array.isArray(target.messages)) {
    state.messages.set(conversationId, target.messages);
  } else if (!state.messages.has(conversationId)) {
    state.messages.set(conversationId, []);
  }

  renderMessages(conversationId);
}

async function refreshFriends() {
  const res = await apiFetch('/friends');
  if (!res.ok) return;
  if (res.data && res.data.code && res.data.code !== 0) return;
  state.friends = res.data?.data || [];
  renderSideList();
}

async function refreshGroups() {
  const res = await apiFetch('/groups', { query: new URLSearchParams({ limit: '50', offset: '0' }) });
  if (!res.ok) return;
  if (res.data && res.data.code && res.data.code !== 0) return;
  state.groups = res.data?.groups || [];
  renderSideList();
}

function renderFriendRequests(list) {
  const root = $('friendRequests');
  root.innerHTML = '';
  if (!Array.isArray(list) || !list.length) {
    const empty = document.createElement('div');
    empty.className = 'small muted';
    empty.textContent = '暂无待处理请求';
    root.appendChild(empty);
    return;
  }

  for (const r of list) {
    const el = document.createElement('div');
    el.className = 'miniItem';

    const top = document.createElement('div');
    top.className = 'miniItemTop';

    const title = document.createElement('div');
    title.className = 'miniItemTitle';
    title.textContent = `${r.from_nickname || ''}${r.from_username ? `（${r.from_username}）` : ''}`.trim() || r.from_user_id;

    const time = document.createElement('div');
    time.className = 'small muted';
    time.textContent = formatTime(r.created_at);

    top.appendChild(title);
    top.appendChild(time);

    const sub = document.createElement('div');
    sub.className = 'miniItemSub';
    sub.textContent = r.message || '';

    const actions = document.createElement('div');
    actions.className = 'miniActions';
    const okBtn = document.createElement('button');
    okBtn.className = 'primary';
    okBtn.textContent = '接受';
    okBtn.addEventListener('click', () => onProcessFriendRequest(r.id, true));
    const noBtn = document.createElement('button');
    noBtn.className = 'danger';
    noBtn.textContent = '拒绝';
    noBtn.addEventListener('click', () => onProcessFriendRequest(r.id, false));
    actions.appendChild(okBtn);
    actions.appendChild(noBtn);

    el.appendChild(top);
    el.appendChild(sub);
    el.appendChild(actions);
    root.appendChild(el);
  }
}

async function refreshFriendRequests() {
  const res = await apiFetch('/friends/requests', { query: new URLSearchParams({ status: 'pending', limit: '50', offset: '0' }) });
  if (!res.ok) return;
  if (res.data && res.data.code && res.data.code !== 0) return;
  renderFriendRequests(res.data?.requests || []);
}

function parseCsvIds(text) {
  return (text || '')
    .split(',')
    .map(s => s.trim())
    .filter(Boolean);
}

function looksLikeUuid(text) {
  const s = (text || '').trim();
  return /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i.test(s);
}

async function resolveUserId(input) {
  const raw = (input || '').trim();
  if (!raw) return '';
  if (looksLikeUuid(raw)) return raw;

  const res = await apiFetch('/search/users', {
    query: new URLSearchParams({ keyword: raw, limit: '20', offset: '0' }),
  });

  if (!res.ok) {
    setBanner('bad', `解析用户失败（HTTP ${res.status}）：${extractApiError(res)}`);
    return '';
  }

  if (res.data && typeof res.data.code === 'number' && res.data.code !== 0) {
    setBanner('bad', `解析用户失败：${res.data.message || 'error'}`);
    return '';
  }

  const users = Array.isArray(res.data?.users) ? res.data.users : [];
  if (!users.length) return '';

  const exact = users.find(u => u?.username === raw) || users.find(u => u?.nickname === raw);
  const picked = exact || (users.length === 1 ? users[0] : users[0]);
  return (picked?.id || '').trim();
}

async function resolveGroupId(input) {
  const raw = (input || '').trim();
  if (!raw) return '';
  if (looksLikeUuid(raw)) return raw;

  const res = await apiFetch('/search/groups', {
    query: new URLSearchParams({ keyword: raw, limit: '20', offset: '0' }),
  });

  if (!res.ok) {
    setBanner('bad', `解析群组失败（HTTP ${res.status}）：${extractApiError(res)}`);
    return '';
  }

  if (res.data && typeof res.data.code === 'number' && res.data.code !== 0) {
    setBanner('bad', `解析群组失败：${res.data.message || 'error'}`);
    return '';
  }

  const groups = Array.isArray(res.data?.groups) ? res.data.groups : [];
  if (!groups.length) return '';

  const exact = groups.find(g => g?.name === raw);
  if (exact?.id) return String(exact.id).trim();

  if (groups.length === 1 && groups[0]?.id) return String(groups[0].id).trim();

  // 多结果且没有精确匹配时，避免“猜错群”
  setBanner('bad', `找到多个群匹配“${raw}”，请在“搜索群组”里点“申请加入”自动填充`);
  return '';
}

async function onRegister() {
  const body = {
    username: $('regUsername').value.trim(),
    password: $('regPassword').value,
    nickname: $('regNickname').value.trim(),
  };
  const res = await apiFetch('/users', { method: 'POST', body: JSON.stringify(body) });
  if (!res.ok || (res.data && res.data.code && res.data.code !== 0)) {
    setBanner('bad', `注册失败（HTTP ${res.status}）`);
    return;
  }
  setBanner('ok', '注册成功，请登录');
}

async function onLogin() {
  const body = {
    username: $('loginUsername').value.trim(),
    password: $('loginPassword').value,
  };
  const res = await apiFetch('/login', { method: 'POST', body: JSON.stringify(body) });
  if (!res.ok || (res.data && res.data.code && res.data.code !== 0)) {
    setBanner('bad', `登录失败（HTTP ${res.status}）`);
    return;
  }
  if (res.data && res.data.token) {
    setToken(res.data.token);
  }

  await loadMe();
  renderAuth();
  setView('chats');
  await refreshConversations();
  wsConnect();
}

async function loadMe() {
  const res = await apiFetch('/users/me');
  if (!res.ok || (res.data && res.data.code && res.data.code !== 0)) {
    if (res.status === 401) clearAuth();
    return;
  }
  state.me = res.data?.data || null;
  localStorage.setItem(storageKeys.me, JSON.stringify(state.me));
  setMeLabel();
}

async function onSendMessage() {
  const conversationId = state.selectedConversationId;
  if (!conversationId) return;

  const text = ($('composerText').value || '').trim();
  if (!text) return;

  const c = state.selectedConversation;
  const peerId = c?.peer_id || (conversationId.includes(':') ? conversationId.split(':')[1] : '');
  if (!peerId) {
    setBanner('bad', '缺少目标 ID');
    return;
  }

  $('btnSendMsg').disabled = true;
  try {
    if (conversationId.startsWith('private:')) {
      const res = await apiFetch('/messages/send', { method: 'POST', body: JSON.stringify({ to_user_id: peerId, content: text }) });
      if (!res.ok || (res.data && res.data.code && res.data.code !== 0)) {
        setBanner('bad', `发送失败（HTTP ${res.status}）`);
        return;
      }
    } else if (conversationId.startsWith('group:')) {
      const res = await apiFetch('/groups/messages', { method: 'POST', body: JSON.stringify({ group_id: peerId, content: text }) });
      if (!res.ok || (res.data && res.data.code && res.data.code !== 0)) {
        setBanner('bad', `发送失败（HTTP ${res.status}）`);
        return;
      }
    }

    $('composerText').value = '';
    await refreshMessagesFor(conversationId);
    await refreshConversations();
  } finally {
    $('btnSendMsg').disabled = !state.selectedConversationId;
  }
}

async function onPinToggle() {
  if (!state.selectedConversationId) return;
  const c = pickConversationFromId(state.selectedConversationId);
  const pinned = !!c.is_pinned;
  const path = `/conversations/${encodeURIComponent(state.selectedConversationId)}/pin`;
  const res = await apiFetch(path, { method: pinned ? 'DELETE' : 'POST' });
  if (!res.ok || (res.data && res.data.code && res.data.code !== 0)) {
    setBanner('bad', `操作失败（HTTP ${res.status}）`);
    return;
  }
  await refreshConversations();
}

async function onDeleteConversation() {
  if (!state.selectedConversationId) return;
  const res = await apiFetch(`/conversations/${encodeURIComponent(state.selectedConversationId)}`, { method: 'DELETE' });
  if (!res.ok || (res.data && res.data.code && res.data.code !== 0)) {
    setBanner('bad', `删除失败（HTTP ${res.status}）`);
    return;
  }
  state.selectedConversationId = null;
  state.selectedConversation = null;
  renderMainHeader();
  $('chatMessages').innerHTML = '';
  await refreshConversations();
}

async function onSendFriendRequest() {
  const input = $('friendToUserId').value.trim();
  if (!input) {
    setBanner('bad', '请填写 to_user_id（或 username）');
    return;
  }

  const toUserId = await resolveUserId(input);
  if (!toUserId) {
    setBanner('bad', `未找到用户：${input}（请用“搜索用户”确认对方存在，并复制 user_id）`);
    return;
  }

  const body = {
    to_user_id: toUserId,
    message: $('friendReqMsg').value.trim(),
  };

  const res = await apiFetch('/friends/requests', { method: 'POST', body: JSON.stringify(body) });
  if (!res.ok) {
    if (res.status === 401) {
      clearAuth();
      setBanner('bad', '登录已失效，请重新登录');
      return;
    }
    setBanner('bad', `发送好友请求失败（HTTP ${res.status}）：${extractApiError(res)}`);
    return;
  }

  if (res.data && typeof res.data.code === 'number' && res.data.code !== 0) {
    setBanner('bad', `发送好友请求失败：${res.data.message || 'error'}`);
    return;
  }

  setBanner('ok', '好友请求已发送');
}

async function onProcessFriendRequest(requestId, accept) {
  const res = await apiFetch('/friends/requests/handle', { method: 'POST', body: JSON.stringify({ request_id: requestId, accept }) });
  if (!res.ok || (res.data && res.data.code && res.data.code !== 0)) {
    setBanner('bad', `处理失败（HTTP ${res.status}）`);
    return;
  }
  setBanner('ok', accept ? '已接受好友请求' : '已拒绝好友请求');
  await refreshFriendRequests();
  await refreshFriends();
}

async function onCreateGroup() {
  const body = {
    name: $('groupName').value.trim(),
    description: $('groupDesc').value.trim(),
    member_ids: parseCsvIds($('groupMemberIds').value),
  };
  const res = await apiFetch('/groups', { method: 'POST', body: JSON.stringify(body) });
  if (!res.ok || (res.data && res.data.code && res.data.code !== 0)) {
    setBanner('bad', `创建群失败（HTTP ${res.status}）`);
    return;
  }
  setBanner('ok', '群已创建');
  if (res.data?.group_id) {
    $('groupId').value = res.data.group_id;
  }
  await refreshGroups();
}

async function onGroupJoinRequest() {
  const input = $('groupId').value.trim();
  if (!input) {
    setBanner('bad', '请填写 group_id（或群名称）');
    return;
  }

  const groupId = await resolveGroupId(input);
  if (!groupId) {
    if (looksLikeUuid(input)) {
      setBanner('bad', `群组不存在：${input}`);
    } else {
      setBanner('bad', `未找到群组：${input}（请用“搜索群组”确认名称/ID）`);
    }
    return;
  }

  const body = {
    group_id: groupId,
    message: $('groupJoinMsg').value.trim(),
  };
  const res = await apiFetch('/groups/join-requests', { method: 'POST', body: JSON.stringify(body) });
  if (!res.ok) {
    if (res.status === 401) {
      clearAuth();
      setBanner('bad', '登录已失效，请重新登录');
      return;
    }
    setBanner('bad', `发送加群申请失败（HTTP ${res.status}）：${extractApiError(res)}`);
    return;
  }
  if (res.data && typeof res.data.code === 'number' && res.data.code !== 0) {
    setBanner('bad', `发送加群申请失败：${res.data.message || 'error'}`);
    return;
  }
  setBanner('ok', '加群申请已发送');
}

async function onMyGroupJoinRequests() {
  const res = await apiFetch('/groups/join-requests/my', { query: new URLSearchParams({ status: '0', limit: '50', offset: '0' }) });
  if (!res.ok || (res.data && res.data.code && res.data.code !== 0)) {
    setBanner('bad', `获取失败（HTTP ${res.status}）`);
    return;
  }
  const list = res.data?.requests || [];
  setBanner('ok', `已加载我的申请（${list.length}）`);
}

function renderSearchResults(containerId, items, type) {
  const root = $(containerId);
  root.innerHTML = '';
  if (!items.length) {
    const empty = document.createElement('div');
    empty.className = 'small muted';
    empty.textContent = '暂无结果';
    root.appendChild(empty);
    return;
  }

  for (const it of items) {
    const el = document.createElement('div');
    el.className = 'miniItem';
    const top = document.createElement('div');
    top.className = 'miniItemTop';

    const title = document.createElement('div');
    title.className = 'miniItemTitle';
    title.textContent = type === 'user'
      ? `${it.nickname || it.username}${it.username ? `（${it.username}）` : ''}`
      : `${it.name || it.id}`;

    const right = document.createElement('div');
    right.className = 'mono';
    right.textContent = String(it.id || '');
    top.appendChild(title);
    top.appendChild(right);

    const sub = document.createElement('div');
    sub.className = 'miniItemSub';
    sub.textContent = type === 'group' ? (it.description || '') : '';

    const actions = document.createElement('div');
    actions.className = 'miniActions';
    if (type === 'user') {
      const chat = document.createElement('button');
      chat.className = 'ghost';
      chat.textContent = '私聊';
      chat.addEventListener('click', () => {
        setView('chats');
        selectConversation(`private:${it.id}`);
      });
      const add = document.createElement('button');
      add.className = 'primary';
      add.textContent = '加好友';
      add.addEventListener('click', async () => {
        $('friendToUserId').value = it.id;
        setView('friends');
        setBanner(null, '');
      });
      actions.appendChild(chat);
      actions.appendChild(add);
    } else {
      const join = document.createElement('button');
      join.className = 'primary';
      join.textContent = '申请加入';
      join.addEventListener('click', () => {
        $('groupId').value = it.id;
        setView('groups');
      });
      actions.appendChild(join);
    }

    el.appendChild(top);
    if (sub.textContent) el.appendChild(sub);
    el.appendChild(actions);
    root.appendChild(el);
  }
}

async function onSearchUsers() {
  const keyword = ($('searchKeyword').value || '').trim();
  const res = await apiFetch('/search/users', { query: new URLSearchParams({ keyword, limit: '20', offset: '0' }) });
  if (!res.ok || (res.data && res.data.code && res.data.code !== 0)) {
    setBanner('bad', `搜索失败（HTTP ${res.status}）`);
    return;
  }
  renderSearchResults('searchUsers', res.data?.users || [], 'user');
}

async function onSearchGroups() {
  const keyword = ($('searchKeyword2').value || '').trim();
  const res = await apiFetch('/search/groups', { query: new URLSearchParams({ keyword, limit: '20', offset: '0' }) });
  if (!res.ok || (res.data && res.data.code && res.data.code !== 0)) {
    setBanner('bad', `搜索失败（HTTP ${res.status}）`);
    return;
  }
  renderSearchResults('searchGroups', res.data?.groups || [], 'group');
}

function openSettings() {
  $('settings').style.display = 'flex';
}

function closeSettings() {
  $('settings').style.display = 'none';
}

function init() {
  setApiBase(localStorage.getItem(storageKeys.apiBase) || '/api/v1');
  $('token').value = (localStorage.getItem(storageKeys.token) || '').trim();
  setWsState('DISCONNECTED', null);

  try {
    state.me = JSON.parse(localStorage.getItem(storageKeys.me) || 'null');
  } catch {
    state.me = null;
  }
  setMeLabel();

  $('btnRegister').addEventListener('click', () => onRegister().catch(e => setBanner('bad', String(e))));
  $('btnLogin').addEventListener('click', () => onLogin().catch(e => setBanner('bad', String(e))));

  $('btnOpenSettings').addEventListener('click', () => openSettings());
  $('btnCloseSettings').addEventListener('click', () => closeSettings());
  $('settings').addEventListener('click', (e) => {
    if (e.target === $('settings')) closeSettings();
  });

  $('btnSaveCfg').addEventListener('click', () => {
    setApiBase($('apiBase').value);
    setToken($('token').value);
    setBanner('ok', '设置已保存');
  });

  $('btnLogout').addEventListener('click', () => {
    clearAuth();
    closeSettings();
    setBanner('ok', '已退出登录');
  });

  $('btnWsConnect').addEventListener('click', () => wsConnect());
  $('btnWsDisconnect').addEventListener('click', () => wsDisconnect());

  for (const b of document.querySelectorAll('.sideTab')) {
    b.addEventListener('click', () => setView(b.dataset.view));
  }

  $('sideSearch').addEventListener('input', () => renderSideList());

  $('btnRefresh').addEventListener('click', () => {
    if (!getTokenRaw()) return;
    refreshConversations().catch(() => {});
    if (state.selectedConversationId) refreshMessagesFor(state.selectedConversationId).catch(() => {});
  });
  $('btnPin').addEventListener('click', () => onPinToggle().catch(e => setBanner('bad', String(e))));
  $('btnDeleteConv').addEventListener('click', () => onDeleteConversation().catch(e => setBanner('bad', String(e))));
  $('btnSendMsg').addEventListener('click', () => onSendMessage().catch(e => setBanner('bad', String(e))));

  $('composerText').addEventListener('keydown', (e) => {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault();
      onSendMessage().catch(err => setBanner('bad', String(err)));
    }
  });

  $('btnFriendReq').addEventListener('click', () => onSendFriendRequest().catch(e => setBanner('bad', String(e))));
  $('btnFriendReqList').addEventListener('click', () => refreshFriendRequests().catch(() => {}));

  $('btnGroupCreate').addEventListener('click', () => onCreateGroup().catch(e => setBanner('bad', String(e))));
  $('btnGroupJoinReq').addEventListener('click', () => onGroupJoinRequest().catch(e => setBanner('bad', String(e))));
  $('btnMyGroupJoin').addEventListener('click', () => onMyGroupJoinRequests().catch(e => setBanner('bad', String(e))));

  $('btnSearchUsers').addEventListener('click', () => onSearchUsers().catch(e => setBanner('bad', String(e))));
  $('btnSearchGroups').addEventListener('click', () => onSearchGroups().catch(e => setBanner('bad', String(e))));

  // Bootstrap
  renderAuth();

  if (getTokenRaw()) {
    loadMe().catch(() => {});
    renderAuth();
    setView('chats');
    refreshConversations().catch(() => {});
    wsConnect();
  }
}

init();
