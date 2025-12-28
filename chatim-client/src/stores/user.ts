import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import type { User } from '@/types'
import { authApi } from '@/api'

export const useUserStore = defineStore('user', () => {
  const currentUser = ref<User | null>(null)
  const token = ref<string | null>(localStorage.getItem('token'))
  const isLoggedIn = ref(!!token.value)

  const currentUserId = computed(() => {
    return currentUser.value?.id || currentUser.value?.user_id
  })

  async function login(loginData: any) {
    const res = await authApi.login(loginData)
    token.value = res.token
    localStorage.setItem('token', res.token)
    isLoggedIn.value = true
    await fetchCurrentUser()
    return res
  }

  async function fetchCurrentUser() {
    try {
      const res = await authApi.getCurrentUser()
      currentUser.value = res.data || null
    } catch (e) {
      console.error(e)
    }
  }

  async function logout() {
    try {
      await authApi.logout()
    } catch (e) {
      console.error('Logout failed:', e)
    } finally {
      token.value = null
      currentUser.value = null
      isLoggedIn.value = false
      localStorage.removeItem('token')
    }
  }

  return {
    currentUser,
    currentUserId,
    token,
    isLoggedIn,
    login,
    fetchCurrentUser,
    logout
  }
})
