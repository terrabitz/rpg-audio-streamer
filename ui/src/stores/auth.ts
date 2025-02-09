import { apiClient } from '@/plugins/axios'
import type { AuthResponse, LoginRequest, LoginResponse } from '@/types/auth'
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
    logout,
    login
  }
})
