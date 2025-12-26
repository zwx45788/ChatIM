import { defineStore } from 'pinia';
import { api } from './api.js?v=5';

export const useMainStore = defineStore('main', {
    state: () => ({
        user: JSON.parse(localStorage.getItem('chat_user') || 'null'),
        token: localStorage.getItem('chat_token') || null,
        
        friends: [],
        groups: [],
        
        // 消息存储结构: { 'private_1': [], 'group_2': [] }
        messages: {},
        
        // 当前会话
        currentSession: null, // { type: 'private'|'group', id: 1, name: 'Alice' }
        
        // 未读计数 (简单实现，实际应从后端获取或计算)
        unreadCounts: {}, // { 'private_1': 5 }
    }),
    
    getters: {
        isLoggedIn: (state) => !!state.token,
        currentMessages: (state) => {
            if (!state.currentSession) return [];
            const key = `${state.currentSession.type}_${state.currentSession.id}`;
            return state.messages[key] || [];
        }
    },
    
    actions: {
        setUser(user, token) {
            this.user = user;
            this.token = token;
            localStorage.setItem('chat_user', JSON.stringify(user));
            localStorage.setItem('chat_token', token);
        },
        
        logout() {
            this.user = null;
            this.token = null;
            this.currentSession = null;
            localStorage.removeItem('chat_user');
            localStorage.removeItem('chat_token');
        },
        
        async fetchContacts() {
            if (!this.token) return;
            try {
                const friendsRes = await api.getFriends(this.token);
                this.friends = (friendsRes.data || []).map(f => ({
                    id: f.user_id || f.id,
                    username: f.username,
                    nickname: f.nickname
                }));
                
                const groupsRes = await api.getGroups(this.token);
                this.groups = groupsRes.groups || [];
            } catch (e) {
                console.error("Failed to fetch contacts", e);
            }
        },
        
        selectSession(type, id, name) {
            this.currentSession = { type, id, name };
            // 切换会话时，标记已读
            this.markAsRead(type, id);
        },
        
        addMessage(type, id, message) {
            const key = `${type}_${id}`;
            if (!this.messages[key]) {
                this.messages[key] = [];
            }
            // 避免重复 (简单去重)
            const exists = this.messages[key].some(m => m.id === message.id && message.id !== 0);
            if (!exists) {
                this.messages[key].push(message);
            }
            
            // 如果不是当前会话，增加未读计数
            if (!this.currentSession || this.currentSession.type !== type || this.currentSession.id !== id) {
                if (!this.unreadCounts[key]) this.unreadCounts[key] = 0;
                this.unreadCounts[key]++;
            }
        },
        
        async pullMessages() {
            if (!this.token) return;
            try {
                const res = await api.pullMessages(100, false, false, this.token);
                if (res.conversations) {
                    res.conversations.forEach(conv => {
                        const type = conv.type; // 'private' or 'group'
                        const peerId = conv.peer_id;
                        
                        if (conv.messages) {
                            conv.messages.forEach(msg => {
                                this.addMessage(type, peerId, {
                                    id: msg.id,
                                    sender_id: msg.from_user_id,
                                    group_id: msg.group_id,
                                    content: msg.content,
                                    type: 'text', // Default to text
                                    created_at: msg.created_at,
                                    is_self: msg.from_user_id === this.user.id
                                });
                            });
                        }
                    });
                }
            } catch (e) {
                console.error("Pull messages failed", e);
            }
        },
        
        async sendMessage(content, msgType = 'text') {
            if (!this.currentSession || !this.token) return;
            
            const { type, id } = this.currentSession;
            try {
                if (type === 'private') {
                    const res = await api.sendPrivateMessage(id, content, msgType, this.token);
                    if (res.msg) {
                        this.addMessage('private', id, {
                            id: res.msg.id,
                            from_user_id: res.msg.from_user_id,
                            content: res.msg.content,
                            type: msgType,
                            created_at: res.msg.created_at,
                            is_self: true
                        });
                    }
                } else {
                    const res = await api.sendGroupMessage(id, content, msgType, this.token);
                    if (res.msg) {
                        this.addMessage('group', id, {
                            id: res.msg.id,
                            from_user_id: res.msg.from_user_id,
                            group_id: res.msg.group_id,
                            content: res.msg.content,
                            type: msgType,
                            created_at: res.msg.created_at,
                            is_self: true
                        });
                    }
                }
            } catch (e) {
                console.error("Send message failed", e);
                throw e;
            }
        },
        
        async markAsRead(type, id) {
            if (!this.token) return;
            const key = `${type}_${id}`;
            this.unreadCounts[key] = 0;
            
            try {
                const msgs = this.messages[key] || [];
                if (type === 'private') {
                    const messageIds = msgs.filter(m => !m.is_self).map(m => m.id);
                    if (messageIds.length > 0) {
                        // New API is singular, so we loop.
                        await Promise.all(messageIds.map(mid => api.markPrivateMessageRead(mid, this.token)));
                    }
                } else {
                    if (msgs.length > 0) {
                        const lastMsg = msgs[msgs.length - 1];
                        await api.markGroupMessageRead(id, lastMsg.id, this.token);
                    }
                }
            } catch (e) {
                console.error("Mark read failed", e);
            }
        }
    }
});