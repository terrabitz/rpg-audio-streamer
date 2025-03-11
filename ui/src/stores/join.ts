import { getApiV1JoinToken, postApiV1Join } from '@/apiClient'
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
      const { data } = await getApiV1JoinToken<true>()
      token.value = data.token
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
      const { data } = await postApiV1Join<true>({
        body: { token }
      })
      return data.success
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
