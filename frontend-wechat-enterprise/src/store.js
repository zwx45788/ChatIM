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
                // 拉取私聊
                const privateRes = await api.pullPrivateUnread(this.token);
                if (privateRes.messages) {
                    privateRes.messages.forEach(msg => {
                        // 对方发给我的，ID是 from_user_id
                        // 我发给对方的（多端同步），ID是 to_user_id
                        // 这里简化，假设拉取到的都是别人发给我的
                        this.addMessage('private', msg.from_user_id, {
                            id: msg.msg_id,
                            sender_id: msg.from_user_id,
                            content: msg.content,
                            type: msg.msg_type,
                            created_at: msg.created_at,
                            is_self: false
                        });
                    });
                }
                
                // 拉取群聊
                const groupRes = await api.pullGroupUnread(this.token);
                if (groupRes.group_messages) {
                    groupRes.group_messages.forEach(msg => {
                        this.addMessage('group', msg.group_id, {
                            id: msg.msg_id,
                            sender_id: msg.from_user_id,
                            group_id: msg.group_id,
                            content: msg.content,
                            type: msg.msg_type,
                            created_at: msg.created_at,
                            is_self: msg.from_user_id === this.user.id
                        });
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
                    await api.sendPrivateMessage(id, content, msgType, this.token);
                    // 乐观更新
                    this.addMessage('private', id, {
                        id: Date.now().toString(), // 临时ID
                        from_user_id: this.user.id, // Backend uses from_user_id
                        content: content,
                        type: msgType,
                        created_at: new Date().toISOString(),
                        is_self: true
                    });
                } else {
                    await api.sendGroupMessage(id, content, msgType, this.token);
                    // 群聊消息通常等待拉取或推送回显，但也可以乐观更新
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
                if (type === 'private') {
                    // Find unread messages for this session
                    const msgs = this.messages[key] || [];
                    // Assuming message object has 'is_read' property or we just send all IDs
                    // Since we don't track is_read locally perfectly, we might need to fetch or just send all IDs
                    // For now, let's assume we send IDs of messages we have locally that are not from self
                    const messageIds = msgs.filter(m => !m.is_self).map(m => m.id);
                    if (messageIds.length > 0) {
                        await api.markPrivateRead(messageIds, this.token);
                    }
                } else {
                    await api.markGroupRead(id, this.token);
                }
            } catch (e) {
                console.error("Mark read failed", e);
            }
        }
    }
});