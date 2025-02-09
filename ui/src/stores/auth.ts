import { apiClient } from '@/plugins/axios'
import type { AuthResponse } from '@/types/auth'
import { defineStore } from 'pinia'
import { ref } from 'vue'

export const useAuthStore = defineStore('auth', () => {
  const authenticated = ref(false)
  const loading = ref(true)

  async function checkAuthStatus() {
    try {
      const response = await apiClient.get(`/auth/status`)
      const data = response.data as AuthResponse

      authenticated.value = data.authenticated
    } catch (error) {
      console.error('Failed to check auth status:', error)
      authenticated.value = false
    } finally {
      loading.value = false
    }
  }

  async function logout() {
    try {
      await apiClient.get(`/auth/logout`)
      authenticated.value = false
    } catch (error) {
      console.error('Failed to logout:', error)
    }
  }

  return {
    authenticated,
    loading,
    checkAuthStatus,
    logout
  }
})
