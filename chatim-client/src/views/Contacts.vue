<template>
  <div class="contacts-view">
    <div class="header">
      <h2>Contacts</h2>
      <el-button type="primary" @click="showAddFriendDialog = true">Add Friend</el-button>
    </div>
    <el-tabs v-model="activeTab">
      <el-tab-pane label="Friends" name="friends">
        <el-table :data="friends" style="width: 100%">
          <el-table-column prop="username" label="Username" />
          <el-table-column prop="nickname" label="Nickname" />
          <el-table-column label="Action">
            <template #default="scope">
              <el-button size="small" @click="startChat(scope.row)">Chat</el-button>
            </template>
          </el-table-column>
        </el-table>
      </el-tab-pane>
      <el-tab-pane label="Requests" name="requests">
        <el-table :data="friendRequests" style="width: 100%">
          <el-table-column prop="from_username" label="From" />
          <el-table-column prop="message" label="Message" />
          <el-table-column label="Action">
            <template #default="scope">
              <el-button size="small" type="success" @click="handleRequest(scope.row, true)">Accept</el-button>
              <el-button size="small" type="danger" @click="handleRequest(scope.row, false)">Reject</el-button>
            </template>
          </el-table-column>
        </el-table>
      </el-tab-pane>
    </el-tabs>

    <el-dialog v-model="showAddFriendDialog" title="Add Friend">
      <el-input v-model="searchKeyword" placeholder="Search by username" @keyup.enter="searchUsers">
        <template #append>
          <el-button @click="searchUsers"><el-icon><Search /></el-icon></el-button>
        </template>
      </el-input>
      <div class="search-results" v-if="searchResults.length > 0">
        <div v-for="user in searchResults" :key="user.id" class="search-item">
          <span>{{ user.username }} ({{ user.nickname }})</span>
          <el-button size="small" @click="addFriend(user)">Add</el-button>
        </div>
      </div>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { friendApi, userApi } from '@/api'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import type { User } from '@/types'
import { useChatStore } from '@/stores/chat'

const activeTab = ref('friends')
const friends = ref<User[]>([])
const friendRequests = ref<any[]>([])
const showAddFriendDialog = ref(false)
const searchKeyword = ref('')
const searchResults = ref<User[]>([])
const router = useRouter()
const chatStore = useChatStore()

onMounted(() => {
  fetchFriends()
  fetchFriendRequests()
})

const fetchFriends = async () => {
  const res = await friendApi.getFriends()
  friends.value = res.data || []
}

const fetchFriendRequests = async () => {
  const res = await friendApi.getFriendRequests()
  friendRequests.value = res.requests || []
}

const startChat = async (user: User) => {
  await chatStore.startConversation(user)
  router.push('/')
}

const handleRequest = async (req: any, accept: boolean) => {
  try {
    await friendApi.handleFriendRequest({ request_id: req.id, accept })
    ElMessage.success(accept ? 'Accepted' : 'Rejected')
    fetchFriendRequests()
    if (accept) fetchFriends()
  } catch (e) {
    // Error handled
  }
}

const searchUsers = async () => {
  if (!searchKeyword.value) return
  const res = await userApi.searchUsers(searchKeyword.value)
  searchResults.value = res.users || []
}

const addFriend = async (user: User) => {
  try {
    await friendApi.addFriend({ to_user_id: user.id })
    ElMessage.success('Friend request sent')
    showAddFriendDialog.value = false
  } catch (e) {
    // Error handled
  }
}
</script>

<style scoped>
.contacts-view {
  padding: 20px;
}
.header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 20px;
}
.search-results {
  margin-top: 20px;
}
.search-item {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 10px 0;
  border-bottom: 1px solid #eee;
}
</style>
