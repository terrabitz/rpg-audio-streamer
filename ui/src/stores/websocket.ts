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

export interface WebSocketMessage {
  method: string
  payload: {
    [key: string]: any
  }
}

interface StoredMessage extends WebSocketMessage {
  timestamp: number
  direction: 'sent' | 'received'
}

type MessageHandler = (message: WebSocketMessage) => void

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
        console.log(message)
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

  function sendMessage(method: string, payload: any) {
    if (socket.value && isConnected.value) {
      const msg = { method, payload }
      socket.value.send(JSON.stringify(msg))

      messageHistory.value.push({
        ...msg,
        timestamp: Date.now(),
        direction: 'sent'
      })
    }
  }

  function broadcast(method: string, fileName: string, payload: any = {}) {
    sendMessage(method, { fileName, ...payload })
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
