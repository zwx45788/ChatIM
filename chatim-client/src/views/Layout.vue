<template>
  <div class="layout-container">
    <el-aside width="250px" class="sidebar">
      <div class="user-info">
        <el-avatar :size="40" :src="userStore.currentUser?.avatar">{{ userStore.currentUser?.nickname?.charAt(0) }}</el-avatar>
        <span class="username">{{ userStore.currentUser?.nickname }}</span>
        <el-button link @click="handleLogout">Logout</el-button>
      </div>
      <el-menu router :default-active="$route.path" class="menu">
        <el-menu-item index="/">
          <el-icon><ChatDotRound /></el-icon>
          <span>Chats</span>
        </el-menu-item>
        <el-menu-item index="/contacts">
          <el-icon><User /></el-icon>
          <span>Contacts</span>
        </el-menu-item>
        <el-menu-item index="/groups">
          <el-icon><Connection /></el-icon>
          <span>Groups</span>
        </el-menu-item>
      </el-menu>
    </el-aside>
    <el-main class="main-content">
      <router-view />
    </el-main>
  </div>
</template>

<script setup lang="ts">
import { useUserStore } from '@/stores/user'
import { useRouter } from 'vue-router'
import { wsManager } from '@/utils/websocket'
import { onMounted } from 'vue'

const userStore = useUserStore()
const router = useRouter()

onMounted(() => {
  if (userStore.token) {
    // wsManager.connect(userStore.token) // Handled by App.vue
    userStore.fetchCurrentUser()
  }
})

const handleLogout = () => {
  // wsManager.disconnect() // 不需要手动断开了，App.vue 会自动处理
  userStore.logout()
  router.push('/login')
}
</script>

<style scoped>
.layout-container {
  display: flex;
  height: 100vh;
}
.sidebar {
  background-color: #fff;
  border-right: 1px solid #dcdfe6;
  display: flex;
  flex-direction: column;
}
.user-info {
  padding: 20px;
  display: flex;
  align-items: center;
  gap: 10px;
  border-bottom: 1px solid #eee;
}
.username {
  flex: 1;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.menu {
  border-right: none;
  flex: 1;
}
.main-content {
  padding: 0;
  background-color: #f5f7fa;
}
</style>
