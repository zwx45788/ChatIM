const API_BASE = ''; // 使用相对路径，自动适配当前域名和端口

export const api = {
    async request(endpoint, method = 'GET', body = null, token = null) {
        const headers = {
            'Content-Type': 'application/json',
        };
        if (token) {
            headers['Authorization'] = `Bearer ${token}`;
        }

        const config = {
            method,
            headers,
        };

        if (body) {
            config.body = JSON.stringify(body);
        }

        try {
            const response = await fetch(`${API_BASE}${endpoint}`, config);
            const data = await response.json();
            if (!response.ok) {
                throw new Error(data.msg || data.error || 'Request failed');
            }
            return data;
        } catch (error) {
            console.error('API Error:', error);
            throw error;
        }
    },

    // Auth
    login(username, password) {
        return this.request('/api/v1/login', 'POST', { username, password });
    },
    register(username, password, email) {
        // 后端目前只接受 username, password, nickname
        // 我们把 email 当作 nickname 传过去，或者暂时忽略
        return this.request('/api/v1/users', 'POST', { username, password, nickname: username });
    },

    // User
    getCurrentUser(token) {
        return this.request('/api/v1/users/me', 'GET', null, token);
    },
    searchUser(username, token) {
        // 后端暂无按用户名搜索接口，暂时返回空
        // return this.request(`/api/v1/users/search?username=${username}`, 'GET', null, token);
        return Promise.resolve({ data: [] });
    },

    // Friend
    addFriend(friendId, remark, token) {
        // 后端暂无好友接口，暂时模拟成功
        // return this.request('/api/v1/friends', 'POST', { friend_id: parseInt(friendId), remark }, token);
        console.warn("Add friend not implemented in backend");
        return Promise.resolve({ code: 0, msg: "模拟添加成功" });
    },
    getFriends(token) {
        // 后端暂无好友接口，暂时返回空列表
        // return this.request('/api/v1/friends', 'GET', null, token);
        return Promise.resolve({ data: [] });
    },

    // Group
    createGroup(name, token) {
        return this.request('/api/v1/groups', 'POST', { name }, token);
    },
    joinGroup(groupId, userId, token) {
        // Backend expects group_id and user_ids list
        return this.request(`/api/v1/groups/${groupId}/members`, 'POST', { 
            group_id: groupId,
            user_ids: [userId]
        }, token); 
    },
    getGroups(token) {
        return this.request('/api/v1/groups', 'GET', null, token);
    },

    // Message
    sendPrivateMessage(receiverId, content, type, token) {
        // Backend expects to_user_id and content
        return this.request('/api/v1/messages/send', 'POST', { 
            to_user_id: receiverId.toString(), 
            content
        }, token);
    },
    sendGroupMessage(groupId, content, type, token) {
        // Backend currently missing specific group message endpoint in Gateway
        // Using temporary endpoint or placeholder
        return this.request(`/api/v1/groups/${groupId}/messages`, 'POST', { 
            group_id: groupId.toString(), 
            content,
            msg_type: type.toString()
        }, token);
    },
    pullPrivateUnread(token) {
        // main.go: protected.GET("/messages/unread/pull", userHandler.PullUnreadMessages)
        return this.request('/api/v1/messages/unread/pull', 'GET', null, token);
    },
    pullGroupUnread(token) {
        // main.go: protected.GET("/unread/all", userHandler.PullAllUnreadMessages)
        return this.request('/api/v1/unread/all', 'GET', null, token);
    },
    markPrivateRead(messageIds, token) {
        // Backend expects message_ids list
        return this.request('/api/v1/messages/read', 'POST', { message_ids: messageIds }, token);
    },
    markGroupRead(groupId, token) {
        // Not implemented in backend yet
        return Promise.resolve();
    }
};