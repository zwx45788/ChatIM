<script setup lang="ts">
import { RouterView } from 'vue-router'
import { useUserStore } from '@/stores/user'
import { wsManager } from '@/utils/websocket'
import { watch, onMounted } from 'vue'

const userStore = useUserStore()

// 监听 token 变化，自动管理 WebSocket 连接
// 这样无论是登录(token变有)还是退出(token变无)，都会自动处理
watch(() => userStore.token, (newToken) => {
  if (newToken) {
    wsManager.connect(newToken)
  } else {
    wsManager.disconnect()
  }
})

// 页面刷新/加载时，如果已登录，也建立连接
onMounted(() => {
  if (userStore.token) {
    wsManager.connect(userStore.token)
  }
})
</script>

<template>
  <RouterView />
</template>

<style>
#app {
  height: 100vh;
  width: 100vw;
  margin: 0;
  padding: 0;
  overflow: hidden;
}
</style>
