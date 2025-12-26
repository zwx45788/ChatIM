<template>
  <div class="groups-view">
    <div class="header">
      <h2>My Groups</h2>
      <div class="actions">
        <el-button @click="openMyJoinRequests">My Join Requests</el-button>
        <el-button @click="showJoinGroupDialog = true">Join Group</el-button>
        <el-button type="primary" @click="showCreateGroupDialog = true">Create Group</el-button>
      </div>
    </div>
    <el-table :data="groups" style="width: 100%">
      <el-table-column prop="name" label="Group Name" />
      <el-table-column prop="description" label="Description" />
      <el-table-column prop="member_count" label="Members" width="100" />
      <el-table-column label="Action">
        <template #default="scope">
          <el-button size="small" @click="enterGroup(scope.row)">Chat</el-button>
          <el-button size="small" @click="openJoinRequests(scope.row)">Requests</el-button>
        </template>
      </el-table-column>
    </el-table>

    <el-dialog v-model="showCreateGroupDialog" title="Create Group">
      <el-form :model="createForm">
        <el-form-item label="Name">
          <el-input v-model="createForm.name" />
        </el-form-item>
        <el-form-item label="Description">
          <el-input v-model="createForm.description" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="showCreateGroupDialog = false">Cancel</el-button>
        <el-button type="primary" @click="createGroup">Create</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="showJoinGroupDialog" title="Join Group" width="600px">
      <div class="search-area">
        <el-input v-model="searchKeyword" placeholder="Search groups..." @keyup.enter="searchGroups">
          <template #append>
            <el-button @click="searchGroups">Search</el-button>
          </template>
        </el-input>
      </div>
      <el-table :data="searchResults" style="width: 100%; margin-top: 20px;" v-if="searchResults.length > 0">
        <el-table-column prop="name" label="Name" />
        <el-table-column prop="description" label="Description" />
        <el-table-column label="Action" width="100">
          <template #default="scope">
            <el-button size="small" type="primary" @click="openJoinConfirm(scope.row)">Join</el-button>
          </template>
        </el-table-column>
      </el-table>
      <div v-else-if="hasSearched" class="no-results">
        No groups found
      </div>
    </el-dialog>

    <el-dialog v-model="showJoinRequestsDialog" :title="joinRequestsTitle" width="900px">
      <el-table :data="joinRequests" style="width: 100%" v-loading="joinRequestsLoading">
        <el-table-column prop="from_username" label="Applicant" width="160" />
        <el-table-column prop="message" label="Message" />
        <el-table-column prop="status" label="Status" width="120" />
        <el-table-column label="Created" width="200">
          <template #default="scope">
            {{ formatUnixSeconds(scope.row.created_at) }}
          </template>
        </el-table-column>
        <el-table-column label="Action" width="200">
          <template #default="scope">
            <el-button
              size="small"
              type="success"
              :disabled="scope.row.status !== 'pending'"
              @click="handleJoinRequest(scope.row, 1)"
            >Accept</el-button>
            <el-button
              size="small"
              type="danger"
              :disabled="scope.row.status !== 'pending'"
              @click="handleJoinRequest(scope.row, 2)"
            >Reject</el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-dialog>

    <el-dialog v-model="showMyJoinRequestsDialog" title="My Join Requests" width="900px">
      <el-table :data="myJoinRequests" style="width: 100%" v-loading="myJoinRequestsLoading">
        <el-table-column prop="from_username" label="Group" width="220" />
        <el-table-column prop="message" label="Message" />
        <el-table-column prop="status" label="Status" width="120" />
        <el-table-column label="Created" width="200">
          <template #default="scope">
            {{ formatUnixSeconds(scope.row.created_at) }}
          </template>
        </el-table-column>
      </el-table>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { groupApi } from '@/api'
import { useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import type { Group } from '@/types'
import { useChatStore } from '@/stores/chat'

const groups = ref<Group[]>([])
const showCreateGroupDialog = ref(false)
const createForm = ref({
  name: '',
  description: ''
})
const router = useRouter()
const chatStore = useChatStore()

// Join Group State
const showJoinGroupDialog = ref(false)
const searchKeyword = ref('')
const searchResults = ref<Group[]>([])
const hasSearched = ref(false)

// Join Requests (admin)
const showJoinRequestsDialog = ref(false)
const joinRequestsLoading = ref(false)
const joinRequests = ref<any[]>([])
const selectedGroup = ref<Group | null>(null)

// My Join Requests
const showMyJoinRequestsDialog = ref(false)
const myJoinRequestsLoading = ref(false)
const myJoinRequests = ref<any[]>([])

const joinRequestsTitle = computed(() => {
  if (!selectedGroup.value) return 'Join Requests'
  return `Join Requests - ${selectedGroup.value.name}`
})

onMounted(() => {
  fetchGroups()
})

const fetchGroups = async () => {
  const res = await groupApi.getMyGroups()
  groups.value = res.groups || []
}

const enterGroup = async (group: Group) => {
  await chatStore.enterGroupChat(group)
  router.push('/')
}

const createGroup = async () => {
  try {
    await groupApi.createGroup(createForm.value)
    ElMessage.success('Group created')
    showCreateGroupDialog.value = false
    fetchGroups()
  } catch (e) {
    // Error handled
  }
}

const searchGroups = async () => {
  if (!searchKeyword.value.trim()) return
  try {
    const res = await groupApi.searchGroups(searchKeyword.value)
    searchResults.value = res.groups || []
    hasSearched.value = true
  } catch (e) {
    console.error(e)
  }
}

const openJoinConfirm = async (group: Group) => {
  try {
    const { value } = await ElMessageBox.prompt('Please enter join message', 'Join Group', {
      confirmButtonText: 'Send Request',
      cancelButtonText: 'Cancel',
      inputPlaceholder: 'Hello, I want to join...'
    })
    
    await groupApi.joinGroup(group.id, value)
    ElMessage.success('Join request sent')
    showJoinGroupDialog.value = false
  } catch (e: any) {
    if (e !== 'cancel') {
      console.error(e)
      ElMessage.error(e.message || 'Failed to send join request')
    }
  }
}

const openJoinRequests = async (group: Group) => {
  selectedGroup.value = group
  showJoinRequestsDialog.value = true
  await fetchJoinRequests()
}

const openMyJoinRequests = async () => {
  showMyJoinRequestsDialog.value = true
  await fetchMyJoinRequests()
}

const fetchJoinRequests = async () => {
  if (!selectedGroup.value) return
  joinRequestsLoading.value = true
  try {
    const res = await groupApi.getGroupJoinRequests(selectedGroup.value.id, { status: 1, limit: 50, offset: 0 })
    joinRequests.value = res.requests || []
  } catch (e: any) {
    joinRequests.value = []
    ElMessage.error(e?.message || 'Failed to load join requests')
  } finally {
    joinRequestsLoading.value = false
  }
}

const fetchMyJoinRequests = async () => {
  myJoinRequestsLoading.value = true
  try {
    const res = await groupApi.getMyGroupJoinRequests({ status: 0, limit: 50, offset: 0 })
    myJoinRequests.value = res.requests || []
  } catch (e: any) {
    myJoinRequests.value = []
    ElMessage.error(e?.message || 'Failed to load my join requests')
  } finally {
    myJoinRequestsLoading.value = false
  }
}

const handleJoinRequest = async (req: any, action: 1 | 2) => {
  try {
    await groupApi.handleGroupJoinRequest({ request_id: req.id, action })
    ElMessage.success(action === 1 ? 'Accepted' : 'Rejected')
    fetchJoinRequests()
  } catch (e: any) {
    ElMessage.error(e?.message || 'Failed to handle request')
  }
}

const formatUnixSeconds = (value: any) => {
  const seconds = typeof value === 'string' ? parseInt(value, 10) : Number(value)
  if (!seconds || Number.isNaN(seconds)) return ''
  return new Date(seconds * 1000).toLocaleString()
}
</script>

<style scoped>
.groups-view {
  padding: 20px;
}
.header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 20px;
}
.actions {
  display: flex;
  gap: 10px;
}
.no-results {
  text-align: center;
  padding: 20px;
  color: #999;
}
</style>
