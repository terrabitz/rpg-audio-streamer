import { apiClient } from '@/plugins/axios'
import type { AuthResponse, User } from '@/types/auth'
import { defineStore } from 'pinia'
import { ref } from 'vue'

export const useAuthStore = defineStore('auth', () => {
  const authenticated = ref(false)
  const loading = ref(true)
  const user = ref<User | null>(null)

  async function checkAuthStatus() {
    try {
      const response = await apiClient.get(`/auth/status`)
      const data = response.data as AuthResponse

      authenticated.value = data.authenticated
      user.value = data.user
    } catch (error) {
      console.error('Failed to check auth status:', error)
      authenticated.value = false
      user.value = null
    } finally {
      loading.value = false
    }
  }

  async function logout() {
    try {
      await apiClient.get(`/auth/logout`)
      authenticated.value = false
      user.value = null
    } catch (error) {
      console.error('Failed to logout:', error)
    }
  }

  return {
    authenticated,
    loading,
    user,
    checkAuthStatus,
    logout
  }
})
