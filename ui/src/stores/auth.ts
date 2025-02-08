import { apiClient } from '@/plugins/axios'
import { defineStore } from 'pinia'
import { ref } from 'vue'

export const useAuthStore = defineStore('auth', () => {
  const authenticated = ref(false)
  const loading = ref(true)
  const user = ref(null)

  async function checkAuthStatus() {
    try {
      const response = await apiClient.get(`/auth/status`)
      authenticated.value = response.data.authenticated
      user.value = response.data.user
    } catch (error) {
      console.error('Failed to check auth status:', error)
      authenticated.value = false
    } finally {
      loading.value = false
    }
  }

  return {
    authenticated,
    loading,
    checkAuthStatus
  }
})
