import { defineStore } from 'pinia'
import { ref } from 'vue'
import type { Conversation, Message, User, Group } from '@/types'
import { messageApi } from '@/api'
import { useUserStore } from './user'

export const useChatStore = defineStore('chat', () => {
  const conversations = ref<Conversation[]>([])
  const currentConversation = ref<Conversation | null>(null)
  const messages = ref<Record<string, Message[]>>({})
  const unreadCount = ref(0)

  async function fetchConversations() {
    const res = await messageApi.getConversations()
    const newConvs = res.conversations || []
    
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

  function handleNewMessage(msg: Message) {
    const userStore = useUserStore()
    console.log('Handling new message:', msg, 'Current User:', userStore.currentUser)
    
    const currentUserId = userStore.currentUser?.id || userStore.currentUser?.user_id
    
    // Determine conversation ID
    let conversationId = ''
    if (msg.type === 'private') {
        const isMe = msg.from_user_id === currentUserId
        const otherId = isMe ? msg.to_user_id : msg.from_user_id
        conversationId = `private:${otherId}`
    } else {
        conversationId = `group:${msg.group_id}`
    }
    
    console.log('Derived Conversation ID:', conversationId)

    if (!messages.value[conversationId]) {
      messages.value[conversationId] = []
    }
    messages.value[conversationId].push(msg)

    // Update conversation list
    const conv = conversations.value.find(c => c.conversation_id === conversationId)
    if (conv) {
      console.log('Updating existing conversation:', conv.conversation_id)
      conv.last_message = msg.content
      conv.last_message_time = msg.created_at
      if (currentConversation.value?.conversation_id !== conversationId) {
        conv.unread_count++
      }
      // Move to top
      const index = conversations.value.indexOf(conv)
      conversations.value.splice(index, 1)
      conversations.value.unshift(conv)
    } else {
      console.log('Conversation not found, fetching list...')
      // Fetch conversations again to get the new one
      fetchConversations()
    }
  }

  async function syncMessages() {
    try {
      const res = await messageApi.getMessages({ limit: 50, include_read: true })
      if (res.conversations) {
        res.conversations.forEach(c => {
          if (c.messages && c.messages.length > 0) {
            const existing = messages.value[c.conversation_id] || []
            const incoming = c.messages
            
            // Simple deduplication
            const map = new Map()
            existing.forEach(m => map.set(m.id, m))
            incoming.forEach(m => map.set(m.id, m))
            
            messages.value[c.conversation_id] = Array.from(map.values()).sort((a, b) => {
                const t1 = typeof a.created_at === 'number' ? a.created_at : new Date(a.created_at).getTime()
                const t2 = typeof b.created_at === 'number' ? b.created_at : new Date(b.created_at).getTime()
                return t1 - t2
            })
          }
        })
      }
    } catch (e) {
      console.error('Failed to sync messages', e)
    }
  }

  return {
    conversations,
    currentConversation,
    messages,
    unreadCount,
    fetchConversations,
    handleNewMessage,
    syncMessages,
    startConversation,
    enterGroupChat
  }
})
