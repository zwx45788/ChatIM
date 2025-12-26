const API_BASE = ''; // 使用相对路径，通过服务器代理转发到后端

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
            
            // 检查响应类型
            const contentType = response.headers.get('content-type');
            if (!contentType || !contentType.includes('application/json')) {
                const text = await response.text();
                console.error('Non-JSON response:', text);
                throw new Error('服务器返回非 JSON 响应，请检查后端服务是否正常运行');
            }
            
            const data = await response.json();
            // Check for code !== 0 as per doc
            if (data.code !== 0) {
                 throw new Error(data.message || data.msg || 'Request failed');
            }
            return data;
        } catch (error) {
            console.error('API Error:', error);
            throw error;
        }
    },

    // 1. 用户管理
    register(username, password, nickname) {
        return this.request('/api/v1/users', 'POST', { username, password, nickname });
    },
    login(username, password) {
        return this.request('/api/v1/login', 'POST', { username, password });
    },
    getCurrentUser(token) {
        return this.request('/api/v1/users/me', 'GET', null, token);
    },
    getUserDetail(userId, token) {
        return this.request(`/api/v1/users/${userId}`, 'GET', null, token);
    },
    checkUserOnline(userId, token) {
        return this.request(`/api/v1/users/${userId}/online`, 'GET', null, token);
    },

    // 2. 消息管理
    sendPrivateMessage(toUserId, content, msgType = 'text', token) {
        return this.request('/api/v1/messages/send', 'POST', { 
            to_user_id: toUserId, 
            content, 
            msg_type: msgType 
        }, token);
    },
    pullMessages(limit = 20, auto_mark = false, include_read = false, token) {
        return this.request(`/api/v1/messages?limit=${limit}&auto_mark=${auto_mark}&include_read=${include_read}`, 'GET', null, token);
    },
    markPrivateMessageRead(messageId, token) {
        return this.request('/api/v1/messages/read', 'POST', { message_id: messageId }, token);
    },
    markGroupMessageRead(groupId, lastReadMessageId, token) {
        return this.request(`/api/v1/groups/${groupId}/read`, 'POST', { last_read_message_id: lastReadMessageId }, token);
    },
    getUnreadCount(token) {
        return this.request('/api/v1/messages/unread', 'GET', null, token);
    },

    // 3. 群组管理
    createGroup(name, description, avatar, token) {
        return this.request('/api/v1/groups', 'POST', { name, description, avatar }, token);
    },
    getGroupInfo(groupId, token) {
        return this.request(`/api/v1/groups/${groupId}`, 'GET', null, token);
    },
    getMyGroups(token) {
        return this.request('/api/v1/groups', 'GET', null, token);
    },
    addGroupMembers(groupId, userIds, token) {
        return this.request(`/api/v1/groups/${groupId}/members`, 'POST', { user_ids: userIds }, token);
    },
    removeGroupMember(groupId, userId, token) {
        return this.request(`/api/v1/groups/${groupId}/members?user_id=${userId}`, 'DELETE', null, token);
    },
    quitGroup(groupId, token) {
        return this.request(`/api/v1/groups/${groupId}`, 'DELETE', null, token);
    },
    sendGroupMessage(groupId, content, msgType = 'text', token) {
        return this.request('/api/v1/groups/messages', 'POST', { 
            group_id: groupId, 
            content, 
            msg_type: msgType 
        }, token);
    },
    getGroupMembers(groupId, token) {
        return this.request(`/api/v1/groups/${groupId}/members`, 'GET', null, token);
    },

    // 4. 群加入请求
    joinGroupRequest(groupId, message, token) {
        return this.request('/api/v1/groups/join-requests', 'POST', { group_id: groupId, message }, token);
    },
    handleJoinRequest(requestId, action, token) {
        return this.request('/api/v1/groups/join-requests/handle', 'POST', { request_id: requestId, action }, token);
    },
    getGroupJoinRequests(groupId, token) {
        return this.request(`/api/v1/groups/${groupId}/join-requests`, 'GET', null, token);
    },
    getMyJoinRequests(token) {
        return this.request('/api/v1/groups/join-requests/my', 'GET', null, token);
    },

    // 5. 群组管理功能
    updateGroupInfo(groupId, data, token) {
        return this.request(`/api/v1/groups/${groupId}/info`, 'PUT', data, token);
    },
    transferGroupOwner(groupId, newOwnerId, token) {
        return this.request(`/api/v1/groups/${groupId}/transfer`, 'POST', { new_owner_id: newOwnerId }, token);
    },
    dismissGroup(groupId, token) {
        return this.request(`/api/v1/groups/${groupId}/dismiss`, 'POST', null, token);
    },
    setGroupAdmin(groupId, userId, isAdmin, token) {
        return this.request(`/api/v1/groups/${groupId}/admin`, 'POST', { user_id: userId, is_admin: isAdmin }, token);
    },

    // 6. 搜索功能
    searchUsers(keyword, limit = 20, offset = 0, token) {
        return this.request(`/api/v1/search/users?keyword=${encodeURIComponent(keyword)}&limit=${limit}&offset=${offset}`, 'GET', null, token);
    },
    searchGroups(keyword, limit = 20, offset = 0, token) {
        return this.request(`/api/v1/search/groups?keyword=${encodeURIComponent(keyword)}&limit=${limit}&offset=${offset}`, 'GET', null, token);
    },

    // 7. 文件上传
    getUploadSignature(type = 'image', token) {
        return this.request(`/api/v1/upload/signature?type=${type}`, 'GET', null, token);
    },

    // 8. 会话管理
    getConversations(token) {
        return this.request('/api/v1/conversations', 'GET', null, token);
    },
    pinConversation(conversationId, token) {
        return this.request(`/api/v1/conversations/${conversationId}/pin`, 'POST', null, token);
    },
    unpinConversation(conversationId, token) {
        return this.request(`/api/v1/conversations/${conversationId}/pin`, 'DELETE', null, token);
    },
    deleteConversation(conversationId, token) {
        return this.request(`/api/v1/conversations/${conversationId}`, 'DELETE', null, token);
    },
    
    // Compatibility aliases
    getGroups(token) { return this.getMyGroups(token); },
    searchUser(keyword, token) { return this.searchUsers(keyword, 20, 0, token); },

    // Friends
    getFriends(token) {
        return this.request('/api/v1/friends', 'GET', null, token);
    },
    addFriend(friendId, remark, token) {
        return this.request('/api/v1/friends/requests', 'POST', { to_user_id: friendId, message: remark }, token);
    },
    
    // Old aliases for compatibility with store.js if needed
    joinGroup(groupId, userId, token) {
        return this.addGroupMembers(groupId, [userId], token);
    },
    pullGroupUnread(token) {
        return this.getAllUnread(token);
    },
    markPrivateRead(messageIds, token) {
        return this.markMessagesRead(messageIds, token);
    },
    markGroupRead(groupId, token) {
        // Not implemented in backend yet, or maybe use markMessagesRead if we have message IDs
        return Promise.resolve();
    }
};