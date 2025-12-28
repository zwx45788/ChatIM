import { defineStore } from 'pinia'
import { ref, watch } from 'vue'
import type { Conversation, Message, User, Group } from '@/types'
import { messageApi, userApi } from '@/api'
import { useUserStore } from './user'

export const useChatStore = defineStore('chat', () => {
  const conversations = ref<Conversation[]>([])
  const currentConversation = ref<Conversation | null>(null)
  const messages = ref<Record<string, Message[]>>({})
  const unreadCount = ref(0)
  const lastStreamId = ref<string>('0-0') // 存储最后的 stream_id
  const userCache = ref<Record<string, { username: string, avatar?: string }>>({})

  // Persistence
  const STORAGE_KEY_MESSAGES = 'chatim_messages'
  const STORAGE_KEY_CONVERSATIONS = 'chatim_conversations'
  const STORAGE_KEY_LAST_STREAM_ID = 'chatim_last_stream_id'

  function loadFromStorage() {
    const storedMessages = localStorage.getItem(STORAGE_KEY_MESSAGES)
    if (storedMessages) {
      try {
        messages.value = JSON.parse(storedMessages)
      } catch (e) {
        console.error('Failed to load messages from storage', e)
      }
    }

    const storedConversations = localStorage.getItem(STORAGE_KEY_CONVERSATIONS)
    if (storedConversations) {
      try {
        conversations.value = JSON.parse(storedConversations)
      } catch (e) {
        console.error('Failed to load conversations from storage', e)
      }
    }

    const storedLastStreamId = localStorage.getItem(STORAGE_KEY_LAST_STREAM_ID)
    if (storedLastStreamId) {
      lastStreamId.value = storedLastStreamId
    }
  }

  // Load immediately
  loadFromStorage()

  // Watch and save
  watch(messages, (newVal) => {
    try {
      localStorage.setItem(STORAGE_KEY_MESSAGES, JSON.stringify(newVal))
    } catch (e) {
      console.error('Failed to save messages to storage (quota exceeded?)', e)
    }
  }, { deep: true })

  watch(conversations, (newVal) => {
    try {
      localStorage.setItem(STORAGE_KEY_CONVERSATIONS, JSON.stringify(newVal))
    } catch (e) {
      console.error('Failed to save conversations to storage', e)
    }
  }, { deep: true })

  async function ensureUserInfo(userId?: string) {
    if (!userId) return { username: 'Unknown', avatar: '' }
    const cached = userCache.value[userId]
    if (cached) return cached
    try {
      const res = await userApi.getUser(userId)
      const info = {
        username: res.data?.nickname || res.data?.username || 'Unknown',
        avatar: res.data?.avatar || ''
      }
      userCache.value[userId] = info
      return info
    } catch (e) {
      return { username: 'Unknown', avatar: '' }
    }
  }

  async function fetchConversations() {
    const res = await messageApi.getConversations()
    const newConvs = res.conversations || []

    // Preserve local unread count (frontend manages unread counts)
    newConvs.forEach((newConv) => {
      const existing = conversations.value.find(c => c.conversation_id === newConv.conversation_id)
      newConv.unread_count = existing ? (existing.unread_count || 0) : 0
    })

    // Preserve current conversation if it's not in the fetched list (e.g. newly created local conversation)
    if (currentConversation.value) {
      const exists = newConvs.find(c => c.conversation_id === currentConversation.value?.conversation_id)
      if (!exists) {
        newConvs.unshift(currentConversation.value)
      }
    }

    conversations.value = newConvs
  }
  async function startConversation(user: User) {
    const userId = user.id || user.user_id
    if (!userId) {
        console.error("User ID is missing", user)
        return
    }
    const conversationId = `private:${userId}`
      try {
        await messageApi.createConversation(conversationId)
      } catch (e) {
        console.error('Failed to create conversation:', e)
    }

    let conv = conversations.value.find(c => c.conversation_id === conversationId)
    
    if (!conv) {
      conv = {
        conversation_id: conversationId,
        type: 'private',
        peer_id: userId,
        peer_name: user.nickname || user.username,
        peer_avatar: user.avatar,
        unread_count: 0,
        last_message_time: Date.now() / 1000,
        messages: [],
        last_message: ''
      }
      conversations.value.unshift(conv)
    }
    
    currentConversation.value = conv
  }

  async function enterGroupChat(group: Group) {
    const conversationId = `group:${group.id}`
    
    try {
      await messageApi.createConversation(conversationId)
    } catch (e) {
      console.error('Failed to create conversation:', e)
    }

    let conv = conversations.value.find(c => c.conversation_id === conversationId)
    
    if (!conv) {
      conv = {
        conversation_id: conversationId,
        type: 'group',
        peer_id: group.id,
        peer_name: group.name,
        peer_avatar: group.avatar,
        unread_count: 0,
        last_message_time: Date.now() / 1000,
        messages: [],
        last_message: ''
      }
      conversations.value.unshift(conv)
    }
    
    currentConversation.value = conv
  }

  async function handleNewMessage(msg: Message) {
    const userStore = useUserStore()
    console.log('Handling new message:', msg, 'Current User:', userStore.currentUser)
    
    const currentUserId = userStore.currentUser?.id || userStore.currentUser?.user_id
    
    // Determine conversation ID based on message type
    let conversationId = ''
    let peerId = ''
    if (msg.type === 'private') {
        const isMe = msg.from_user_id === currentUserId
        peerId = isMe ? msg.to_user_id! : msg.from_user_id
        conversationId = `private:${peerId}`
    } else {
        // group message
        peerId = msg.group_id!
        conversationId = `group:${peerId}`
    }
    
    console.log('Derived Conversation ID:', conversationId)

    if (!messages.value[conversationId]) {
      messages.value[conversationId] = []
    }

    // Check for duplicates
    const existingIndex = messages.value[conversationId].findIndex(m => {
      if (m.id && msg.id) return m.id === msg.id
      if (m.stream_id && msg.stream_id) return m.stream_id === msg.stream_id
      // Fallback: shallow equality by essential fields
      return (
        m.from_user_id === msg.from_user_id &&
        m.content === msg.content &&
        String(m.created_at) === String(msg.created_at)
      )
    })
    if (existingIndex !== -1) {
        console.log('Duplicate message detected, updating:', msg.id)
        // Update existing message (e.g. status change)
        messages.value[conversationId][existingIndex] = { ...messages.value[conversationId][existingIndex], ...msg }
        return
    }

    // Ensure sender name present
    if (!msg.from_user_name || msg.from_user_name === 'Unknown') {
      const info = await ensureUserInfo(msg.from_user_id)
      msg.from_user_name = info.username
    }

    messages.value[conversationId].push(msg)

    // 更新 stream_id 和游标
    if (msg.stream_id) {
      lastStreamId.value = msg.stream_id
      localStorage.setItem(STORAGE_KEY_LAST_STREAM_ID, msg.stream_id)
      
      // 异步更新游标到后端
      messageApi.updateLastSeenCursor({
        last_seen_stream_id: msg.stream_id,
        conversation_type: msg.type === 'group' ? 'group' : 'private',
        peer_id: peerId
      }).catch(e => {
        console.error('Failed to update cursor:', e)
      })
    }

    // Update conversation list
    const conv = conversations.value.find(c => c.conversation_id === conversationId)
    if (conv) {
      console.log('Updating existing conversation:', conv.conversation_id)
      conv.last_message = msg.content
      conv.last_message_time = msg.created_at
      
      // Only increment unread count if user is NOT in this conversation
      if (currentConversation.value?.conversation_id !== conversationId) {
        conv.unread_count = (conv.unread_count || 0) + 1
      }
      
      // Move to top
      const index = conversations.value.indexOf(conv)
      conversations.value.splice(index, 1)
      conversations.value.unshift(conv)
    } else {
      console.log('Conversation not found, creating placeholder...')
      // Create a new conversation entry if it doesn't exist
      let peerName = 'Unknown'
      let peerAvatar = ''
      if (msg.type === 'private') {
        const info = await ensureUserInfo(peerId)
        peerName = info.username
        peerAvatar = info.avatar || ''
      }
      const newConv: Conversation = {
        conversation_id: conversationId,
        type: msg.type,
        peer_id: peerId,
        peer_name: msg.type === 'group' ? (msg.from_user_name || 'Unknown') : peerName,
        peer_avatar: peerAvatar,
        unread_count: currentConversation.value?.conversation_id === conversationId ? 0 : 1,
        last_message_time: msg.created_at,
        messages: [],
        last_message: msg.content
      }
      conversations.value.unshift(newConv)
    }

    // 计算总未读数
    unreadCount.value = conversations.value.reduce((sum, c) => sum + (c.unread_count || 0), 0)
  }

  async function syncMessages() {
    try {
      const res = await messageApi.getMessages({ 
        from_stream_id: lastStreamId.value,
        limit: 50 
      })
      if (res.conversations) {
        for (const c of res.conversations) {
          if (c.messages && c.messages.length > 0) {
            const existing = messages.value[c.conversation_id] || []
            const incoming = c.messages

            const keyOf = (m: Message) => m.stream_id || m.id || `${m.from_user_id}-${m.created_at}-${m.content}`

            const bucket: Record<string, Message> = {}
            existing.forEach(m => {
              bucket[keyOf(m)] = m
            })

            for (const m of incoming) {
              // 补齐昵称（离线拉取可能缺）
              if (!m.from_user_name || m.from_user_name === 'Unknown') {
                const info = await ensureUserInfo(m.from_user_id)
                m.from_user_name = info.username
              }
              bucket[keyOf(m)] = m

              // 更新最后的 stream_id
              if (m.stream_id && m.stream_id > lastStreamId.value) {
                lastStreamId.value = m.stream_id
                localStorage.setItem(STORAGE_KEY_LAST_STREAM_ID, m.stream_id)
              }
            }

            messages.value[c.conversation_id] = Object.values(bucket).sort((a, b) => {
              const t1 = typeof a.created_at === 'number' ? a.created_at : new Date(a.created_at).getTime()
              const t2 = typeof b.created_at === 'number' ? b.created_at : new Date(b.created_at).getTime()
              return t1 - t2
            })
          }
        }

        // 计算总未读数（完全由前端维护，不使用后端返回值）
        unreadCount.value = conversations.value.reduce((sum, c) => sum + (c.unread_count || 0), 0)

        // 同步完成后，上报最新的 stream_id 给服务端，避免下次重复拉取
        if (lastStreamId.value && lastStreamId.value !== '0-0') {
          messageApi.updateLastSeenCursor({
            last_seen_stream_id: lastStreamId.value
          }).catch(e => console.error('Failed to update last seen cursor', e))
        }
      }
    } catch (e) {
      console.error('Failed to sync messages', e)
    }
  }

  async function pinConversation(conversationId: string, isPinned: boolean) {
    try {
      if (isPinned) {
        await messageApi.pinConversation(conversationId)
      } else {
        await messageApi.unpinConversation(conversationId)
      }
      
      const conv = conversations.value.find(c => c.conversation_id === conversationId)
      if (conv) {
        conv.is_pinned = isPinned
        // Re-sort conversations: pinned first, then by time
        conversations.value.sort((a, b) => {
            if (a.is_pinned !== b.is_pinned) return a.is_pinned ? -1 : 1
            const t1 = typeof a.last_message_time === 'number' ? a.last_message_time : new Date(a.last_message_time).getTime()
            const t2 = typeof b.last_message_time === 'number' ? b.last_message_time : new Date(b.last_message_time).getTime()
            return t2 - t1
        })
      }
    } catch (e) {
      console.error('Failed to pin/unpin conversation', e)
    }
  }

  async function deleteConversation(conversationId: string) {
    try {
      await messageApi.deleteConversation(conversationId)
      const index = conversations.value.findIndex(c => c.conversation_id === conversationId)
      if (index !== -1) {
        conversations.value.splice(index, 1)
      }
      if (currentConversation.value?.conversation_id === conversationId) {
        currentConversation.value = null
      }
      // Also remove messages
      delete messages.value[conversationId]
    } catch (e) {
      console.error('Failed to delete conversation', e)
    }
  }

  return {
    conversations,
    currentConversation,
    messages,
    unreadCount,
    lastStreamId,
    fetchConversations,
    handleNewMessage,
    syncMessages,
    startConversation,
    enterGroupChat,
    pinConversation,
    deleteConversation
  }
})
