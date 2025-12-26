export interface User {
  id?: string
  user_id?: string
  username: string
  nickname: string
  avatar?: string
}

export interface Message {
  id: string
  type: 'private' | 'group'
  from_user_id: string
  from_user_name?: string
  to_user_id?: string
  group_id?: string
  content: string
  created_at: number | string
  is_read?: boolean
  read_at?: number
  stream_id?: string
  is_sender?: boolean
}

export interface Conversation {
  conversation_id: string
  type: 'private' | 'group'
  peer_id: string
  peer_name: string
  peer_avatar?: string
  unread_count: number
  last_message_time: number | string
  messages: Message[]
  title?: string
  avatar?: string
  last_message?: string
  is_pinned?: boolean
}

export interface Group {
  id: string
  name: string
  description: string
  creator_id: string
  created_at: number
  member_count: number
  avatar?: string
}

export interface GroupMember extends User {
  role: 'admin' | 'member'
  joined_at: number
}

export interface LoginResponse {
  code: number
  message: string
  token: string
  private_unreads: any[]
  private_unread_count: number
  group_unreads: any[]
  group_unread_count: number
  total_unread_count: number
}

export interface ApiResponse<T = any> {
  code: number
  message: string
  data?: T
  [key: string]: any
}

export type FlatResponse<T> = {
  code: number
  message: string
} & T
