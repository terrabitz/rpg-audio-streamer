import { apiClient } from '@/plugins/axios'
import type { AuthResponse, LoginRequest, LoginResponse, Role } from '@/types/auth'
import { defineStore } from 'pinia'
import { ref } from 'vue'

export const useAuthStore = defineStore('auth', () => {
  const authenticated = ref(false)
  const loading = ref(false)
  const role = ref<Role | null>(null)

  async function checkAuthStatus() {
    try {
      const response = await apiClient.get(`/auth/status`)
      const data = response.data as AuthResponse

      authenticated.value = data.authenticated
      role.value = data.authenticated && data.role ? data.role : null
    } catch (error) {
      console.error('Failed to check auth status:', error)
      authenticated.value = false
      role.value = null
    } finally {
      loading.value = false
    }
  }

  async function login(username: string, password: string) {
    loading.value = true
    try {
      const loginData: LoginRequest = { username, password }
      const response = await apiClient.post('/login', loginData)
      const data = response.data as LoginResponse

      if (data.success) {
        await checkAuthStatus()
      } else {
        throw new Error(data.error || 'Login failed')
      }
    } catch (error) {
      console.error('Login failed:', error)
      throw error
    } finally {
      loading.value = false
    }
  }

  async function logout() {
    try {
      await apiClient.post(`/auth/logout`)
      authenticated.value = false
      role.value = null
    } catch (error) {
      console.error('Failed to logout:', error)
    }
  }

  return {
    authenticated,
    loading,
    role,
    checkAuthStatus,
    logout,
    login
  }
})
