import request from '@/utils/request'
import type { LoginResponse, ApiResponse, FlatResponse, User, Conversation, Message, Group, GroupMember } from '@/types'

export const authApi = {
  login(data: any) {
    return request.post<any, LoginResponse>('/login', data)
  },
  register(data: any) {
    return request.post<any, FlatResponse<{ user_id: string }>>('/users', data)
  },
  getCurrentUser() {
    return request.get<any, ApiResponse<User>>('/users/me')
  }
}

export const userApi = {
  getUser(userId: string) {
    return request.get<any, ApiResponse<User>>(`/users/${userId}`)
  },
  searchUsers(keyword: string) {
    return request.get<any, FlatResponse<{ users: User[], total: number }>>(`/search/users`, { params: { keyword } })
  }
}

export const messageApi = {
  sendPrivateMessage(data: { to_user_id: string, content: string }) {
    return request.post<any, FlatResponse<{ msg: Message }>>('/messages/send', data)
  },
  getConversations() {
    return request.get<any, FlatResponse<{ conversations: Conversation[], total: number }>>('/conversations')
  },
  createConversation(conversationId: string) {
    return request.post<any, FlatResponse<{}>>('/conversations', { conversation_id: conversationId })
  },
  getMessages(params: { from_stream_id?: string, limit?: number }) {
    return request.get<any, FlatResponse<{ conversations: Conversation[], total_unread: number }>>('/messages', { params })
  },
  updateLastSeenCursor(data: { last_seen_stream_id: string, conversation_type?: string, peer_id?: string }) {
    return request.post<any, FlatResponse<{ cursor: string }>>('/messages/cursor', data)
  },
  markPrivateMessageAsRead(messageId: string) {
    return request.post<any, FlatResponse<{}>>('/messages/read', { message_id: messageId })
  },
  pinConversation(conversationId: string) {
    return request.post<any, FlatResponse<{}>>(`/conversations/${conversationId}/pin`)
  },
  unpinConversation(conversationId: string) {
    return request.delete<any, FlatResponse<{}>>(`/conversations/${conversationId}/pin`)
  },
  deleteConversation(conversationId: string) {
    return request.delete<any, FlatResponse<{}>>(`/conversations/${conversationId}`)
  }
}

export const groupApi = {
  createGroup(data: { name: string, description?: string, member_ids?: string[] }) {
    return request.post<any, FlatResponse<{ group_id: string }>>('/groups', data)
  },
  getMyGroups() {
    return request.get<any, FlatResponse<{ groups: Group[], total: number }>>('/groups')
  },
  getGroupMembers(groupId: string) {
    return request.get<any, FlatResponse<{ members: GroupMember[], total: number }>>(`/groups/${groupId}/members`)
  },
  sendGroupMessage(data: { group_id: string, content: string }) {
    return request.post<any, FlatResponse<{ msg: Message }>>('/groups/messages', data)
  },
  joinGroup(groupId: string, message: string) {
    return request.post<any, FlatResponse<{}>>('/groups/join-requests', { group_id: groupId, message })
  },
  getGroupJoinRequests(groupId: string, params?: { status?: number, limit?: number, offset?: number }) {
    return request.get<any, FlatResponse<{ requests: any[], total: number }>>(`/groups/${groupId}/join-requests`, { params })
  },
  getMyGroupJoinRequests(params?: { status?: number, limit?: number, offset?: number }) {
    return request.get<any, FlatResponse<{ requests: any[], total: number }>>('/groups/join-requests/my', { params })
  },
  handleGroupJoinRequest(data: { request_id: string, action: 1 | 2 }) {
    return request.post<any, FlatResponse<{}>>('/groups/join-requests/handle', data)
  },
  markGroupMessageAsRead(groupId: string, lastReadMessageId: string) {
    return request.post<any, FlatResponse<{}>>(`/groups/${groupId}/read`, { last_read_message_id: lastReadMessageId })
  },
  searchGroups(keyword: string) {
    return request.get<any, FlatResponse<{ groups: Group[], total: number }>>(`/search/groups`, { params: { keyword } })
  }
}

export const friendApi = {
  getFriends() {
    return request.get<any, ApiResponse<User[]>>('/friends')
  },
  addFriend(data: { to_user_id: string, message?: string }) {
    return request.post<any, ApiResponse>('/friends/requests', data)
  },
  getFriendRequests() {
    return request.get<any, ApiResponse<{ requests: any[], total: number }>>('/friends/requests')
  },
  handleFriendRequest(data: { request_id: string, accept: boolean }) {
    return request.post<any, ApiResponse>('/friends/requests/handle', data)
  }
}
