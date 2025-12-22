<template>
  <div class="groups-view">
    <div class="header">
      <h2>My Groups</h2>
      <el-button type="primary" @click="showCreateGroupDialog = true">Create Group</el-button>
    </div>
    <el-table :data="groups" style="width: 100%">
      <el-table-column prop="name" label="Group Name" />
      <el-table-column prop="description" label="Description" />
      <el-table-column prop="member_count" label="Members" width="100" />
      <el-table-column label="Action">
        <template #default="scope">
          <el-button size="small" @click="enterGroup(scope.row)">Chat</el-button>
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
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { groupApi } from '@/api'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
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
</style>
