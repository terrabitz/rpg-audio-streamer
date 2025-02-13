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

export interface WebSocketMessage<T = any> {
  method: string
  senderId?: string
  payload: T
}

interface StoredMessage<T = any> extends WebSocketMessage<T> {
  timestamp: number
  direction: 'sent' | 'received'
}

type MessageHandler<T = any> = (message: WebSocketMessage<T>) => void

export const useWebSocketStore = defineStore('websocket', () => {
  const isConnected = ref(false)
  const socket = ref<WebSocket | null>(null)
  const messageHandlers = ref<MessageHandler[]>([])
  const messageHistory = ref<StoredMessage[]>([])

  function addMessageHandler(handler: MessageHandler) {
    messageHandlers.value.push(handler)
  }

  function removeMessageHandler(handler: MessageHandler) {
    const index = messageHandlers.value.indexOf(handler)
    if (index > -1) {
      messageHandlers.value.splice(index, 1)
    }
  }

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

    socket.value.onmessage = (event) => {
      try {
        const message = JSON.parse(event.data) as WebSocketMessage
        const storedMessage: StoredMessage = {
          ...message,
          timestamp: Date.now(),
          direction: 'received'
        }
        messageHistory.value.push(storedMessage)
        messageHandlers.value.forEach(handler => handler(message))
      } catch (error) {
        console.error('Failed to parse WebSocket message:', event.data)
      }
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

  function sendMessage<T>(method: string, payload: T) {
    if (socket.value && isConnected.value) {
      const msg: WebSocketMessage<T> = { method, payload }
      socket.value.send(JSON.stringify(msg))

      messageHistory.value.push({
        ...msg,
        timestamp: Date.now(),
        direction: 'sent'
      })
    }
  }

  function broadcast<T>(method: string, payload: T = {} as T) {
    sendMessage(method, payload)
  }

  function clearMessageHistory() {
    messageHistory.value = []
  }

  return {
    isConnected,
    socket,
    connect,
    disconnect,
    sendMessage,
    broadcast,
    addMessageHandler,
    removeMessageHandler,
    messageHistory,
    clearMessageHistory
  }
})
