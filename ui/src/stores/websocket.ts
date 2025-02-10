import { defineStore } from 'pinia'
import { ref } from 'vue'

export function getWebSocketUrl(): string {
  const apiBase = import.meta.env.VITE_API_BASE_URL
  let baseUrl: URL

  try {
    // Try parsing as absolute URL
    baseUrl = new URL(apiBase)
  } catch {
    // If parsing fails, treat as relative URL
    baseUrl = new URL(apiBase, window.location.origin)
  }

  const protocol = baseUrl.protocol === 'https:' ? 'wss:' : 'ws:'
  return `${protocol}//${baseUrl.host}${baseUrl.pathname}/ws`
}

export const useWebSocketStore = defineStore('websocket', () => {
  const isConnected = ref(false)
  const socket = ref<WebSocket | null>(null)

  function connect() {
    if (socket.value?.readyState === WebSocket.OPEN) {
      return
    }

    const wsUrl = getWebSocketUrl()
    socket.value = new WebSocket(wsUrl)

    socket.value.onopen = () => {
      isConnected.value = true
      console.log('WebSocket connected')
    }

    socket.value.onclose = () => {
      isConnected.value = false
      console.log('WebSocket disconnected')
      setTimeout(connect, 5000) // Attempt to reconnect after 5 seconds
    }

    socket.value.onerror = (error) => {
      console.error('WebSocket error:', error)
      socket.value?.close()
    }
  }

  function disconnect() {
    socket.value?.close()
    socket.value = null
    isConnected.value = false
  }

  return {
    isConnected,
    socket,
    connect,
    disconnect
  }
})
