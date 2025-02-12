import { apiClient } from '@/plugins/axios'
import type { JoinResponse, JoinTokenResponse } from '@/types/join'
import { defineStore } from 'pinia'
import { ref } from 'vue'

export const useJoinStore = defineStore('join', () => {
  const token = ref<string | null>(null)
  const loading = ref(false)
  const error = ref<string | null>(null)

  async function fetchToken() {
    loading.value = true
    error.value = null

    try {
      const response = await apiClient.get<JoinTokenResponse>('/join-token')
      token.value = response.data.token
    } catch (err) {
      console.error('Failed to fetch join token:', err)
      error.value = 'Failed to fetch join token'
      token.value = null
    } finally {
      loading.value = false
    }
  }

  async function submitJoinToken(token: string) {
    loading.value = true
    error.value = null

    try {
      const response = await apiClient.post<JoinResponse>('/join', { token })
      return response.data.success
    } catch (err) {
      console.error('Failed to join table:', err)
      error.value = 'Failed to join table'
      return false
    } finally {
      loading.value = false
    }
  }

  function clearToken() {
    token.value = null
    error.value = null
  }

  return {
    token,
    loading,
    error,
    fetchToken,
    clearToken,
    submitJoinToken
  }
})
