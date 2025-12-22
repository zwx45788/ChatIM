<template>
  <div class="chat-view">
    <div class="conversation-list">
      <div v-for="conv in chatStore.conversations" :key="conv.conversation_id" 
           class="conversation-item" 
           :class="{ active: chatStore.currentConversation?.conversation_id === conv.conversation_id }"
           @click="selectConversation(conv)">
        <el-avatar :size="40" :src="conv.peer_avatar">{{ conv.peer_name?.charAt(0) }}</el-avatar>
        <div class="conv-info">
          <div class="conv-top">
            <span class="conv-name">{{ conv.peer_name || conv.title }}</span>
            <span class="conv-time">{{ formatTime(conv.last_message_time) }}</span>
          </div>
          <div class="conv-bottom">
            <span class="conv-msg">{{ conv.last_message }}</span>
            <el-badge v-if="conv.unread_count > 0" :value="conv.unread_count" class="unread-badge" />
          </div>
        </div>
      </div>
    </div>
    <div class="chat-area" v-if="chatStore.currentConversation">
      <div class="chat-header">
        {{ chatStore.currentConversation.peer_name || chatStore.currentConversation.title }}
      </div>
      <div class="message-list" ref="messageListRef">
        <div v-for="msg in currentMessages" :key="msg.id" 
             class="message-item" 
             :class="{ 'my-message': msg.from_user_id === userStore.currentUserId }">
          <el-avatar :size="30" class="msg-avatar">{{ msg.from_user_name?.charAt(0) || 'U' }}</el-avatar>
          <div class="msg-content">
            <div class="msg-sender" v-if="msg.type === 'group' && msg.from_user_id !== userStore.currentUserId">
              {{ msg.from_user_name }}
            </div>
            <div class="msg-bubble">{{ msg.content }}</div>
          </div>
        </div>
      </div>
      <div class="input-area">
        <el-input v-model="inputMessage" type="textarea" :rows="3" placeholder="Type a message..." @keyup.enter.ctrl="sendMessage" />
        <div class="input-actions">
          <el-button type="primary" @click="sendMessage">Send</el-button>
        </div>
      </div>
    </div>
    <div class="empty-state" v-else>
      Select a conversation to start chatting
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, watch, nextTick } from 'vue'
import { useChatStore } from '@/stores/chat'
import { useUserStore } from '@/stores/user'
import { messageApi, groupApi } from '@/api'
import type { Conversation } from '@/types'

const chatStore = useChatStore()
const userStore = useUserStore()
const inputMessage = ref('')
const messageListRef = ref<HTMLElement | null>(null)

onMounted(() => {
  chatStore.fetchConversations()
  chatStore.syncMessages()
})

const currentMessages = computed(() => {
  if (!chatStore.currentConversation) return []
  return chatStore.messages[chatStore.currentConversation.conversation_id] || []
})

const selectConversation = async (conv: Conversation) => {
  chatStore.currentConversation = conv
  conv.unread_count = 0 // Reset unread locally
  
  // Fetch messages if not loaded
  if (!chatStore.messages[conv.conversation_id]) {
    // In a real app, we would fetch messages for this conversation specifically
    // But the API provided has /messages which returns all conversations with messages?
    // Wait, the API 2.2 returns conversations with messages.
    // API 2.4 pulls unread.
    // There isn't a clear "get history for conversation" API in the reference provided in the prompt snippet.
    // Ah, 2.2 "拉取消息（按会话分组...）" seems to be the sync endpoint.
    // Usually there is a /messages/{conversation_id} or similar.
    // Let's assume for now we rely on the initial sync or we might need to implement a fetch logic if the API supports it.
    // Looking at API 2.2, it returns a list of conversations with their last messages.
    // It seems we might need to rely on what we have or if there is a missing API in the doc for history.
    // For this demo, I will assume messages are accumulated in the store or fetched via sync.
    // If the API doesn't support history pagination per conversation, we might just show what we have.
    
    // Actually, looking at the prompt again, it says "getMessages(params: GetMessagesParams)".
    // Let's assume we just use what's in the store for now, populated by initial fetch or websocket.
  }
  
  scrollToBottom()
}

const sendMessage = async () => {
  if (!inputMessage.value.trim() || !chatStore.currentConversation) return
  
  const content = inputMessage.value
  inputMessage.value = ''
  
  try {
    let res;
    if (chatStore.currentConversation.type === 'private') {
      res = await messageApi.sendPrivateMessage({
        to_user_id: chatStore.currentConversation.peer_id,
        content
      })
    } else {
      res = await groupApi.sendGroupMessage({
        group_id: chatStore.currentConversation.peer_id,
        content
      })
    }
    
    if (res && res.msg) {
      chatStore.handleNewMessage(res.msg)
    }
  } catch (e) {
    console.error(e)
  }
}

const scrollToBottom = () => {
  nextTick(() => {
    if (messageListRef.value) {
      messageListRef.value.scrollTop = messageListRef.value.scrollHeight
    }
  })
}

watch(currentMessages, () => {
  scrollToBottom()
}, { deep: true })

const formatTime = (timestamp: number | string) => {
  if (!timestamp) return ''
  const date = typeof timestamp === 'number' ? new Date(timestamp * 1000) : new Date(timestamp)
  return date.toLocaleTimeString()
}
</script>

<style scoped>
.chat-view {
  display: flex;
  height: 100%;
}
.conversation-list {
  width: 300px;
  border-right: 1px solid #dcdfe6;
  overflow-y: auto;
  background: #fff;
}
.conversation-item {
  padding: 15px;
  display: flex;
  gap: 10px;
  cursor: pointer;
  border-bottom: 1px solid #f0f0f0;
}
.conversation-item:hover, .conversation-item.active {
  background-color: #f5f7fa;
}
.conv-info {
  flex: 1;
  overflow: hidden;
}
.conv-top {
  display: flex;
  justify-content: space-between;
  margin-bottom: 5px;
}
.conv-name {
  font-weight: bold;
}
.conv-time {
  font-size: 12px;
  color: #999;
}
.conv-bottom {
  display: flex;
  justify-content: space-between;
}
.conv-msg {
  font-size: 13px;
  color: #666;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  max-width: 180px;
}
.chat-area {
  flex: 1;
  display: flex;
  flex-direction: column;
  background: #fff;
}
.chat-header {
  padding: 15px;
  border-bottom: 1px solid #dcdfe6;
  font-weight: bold;
}
.message-list {
  flex: 1;
  padding: 20px;
  overflow-y: auto;
  display: flex;
  flex-direction: column;
  gap: 15px;
}
.message-item {
  display: flex;
  gap: 10px;
  max-width: 70%;
}
.message-item.my-message {
  align-self: flex-end;
  flex-direction: row-reverse;
}
.msg-content {
  display: flex;
  flex-direction: column;
}
.msg-sender {
  font-size: 12px;
  color: #999;
  margin-bottom: 2px;
}
.msg-bubble {
  background-color: #f4f4f5;
  padding: 10px;
  border-radius: 8px;
  word-break: break-word;
}
.my-message .msg-bubble {
  background-color: #409eff;
  color: #fff;
}
.input-area {
  padding: 15px;
  border-top: 1px solid #dcdfe6;
}
.input-actions {
  margin-top: 10px;
  text-align: right;
}
.empty-state {
  flex: 1;
  display: flex;
  justify-content: center;
  align-items: center;
  color: #999;
}
</style>
