import { defineStore } from 'pinia'
import { ref } from 'vue'

const reconnectIntervalMs = 3000

export interface WebSocketMessage<T = unknown> {
  method: string
  senderId?: string
  payload: T
}

interface StoredMessage<T = unknown> extends WebSocketMessage<T> {
  timestamp: number
  direction: 'sent' | 'received'
}

type MessageHandler<T = unknown> = (message: WebSocketMessage<T>) => void

function getWebSocketUrl(): string {
  const apiBase = import.meta.env.VITE_API_BASE_URL
  let baseUrl: URL

  try {
    baseUrl = new URL(apiBase)
  } catch {
    baseUrl = new URL(apiBase, window.location.origin)
  }

  const protocol = baseUrl.protocol === 'https:' ? 'wss:' : 'ws:'
  return `${protocol}//${baseUrl.host}/api/v1/ws`
}

export const useWebSocketStore = defineStore('websocket', () => {
  const isConnected = ref(false)
  const messageHandlers = ref<MessageHandler[]>([])
  const messageHistory = ref<StoredMessage[]>([])
  let socket: WebSocket | null = null

  function connect(token?: string) {
    if (socket?.readyState === WebSocket.OPEN) {
      return
    }

    const baseUrl = getWebSocketUrl()
    const url = token ? `${baseUrl}?token=${encodeURIComponent(token)}` : baseUrl
    socket = new WebSocket(url)

    socket.onopen = () => {
      isConnected.value = true
      console.log('WebSocket connected')
    }

    socket.onmessage = (event) => {
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
        console.error(`Failed to parse WebSocket\nmessage:${event.data}\nerror:${error}`)
      }
    }

    socket.onclose = (event: CloseEvent) => {
      isConnected.value = false
      const reason = event.reason || 'No reason provided'
      const code = event.code
      console.log(`WebSocket disconnected: [${code}] ${reason}`)
      setTimeout(() => connect(token), reconnectIntervalMs)
    }

    socket.onerror = (error) => {
      console.error('WebSocket error:', error)
      disconnect()
      setTimeout(() => connect(token), reconnectIntervalMs)
    }
  }

  function disconnect() {
    socket?.close()
    socket = null
    isConnected.value = false
  }

  function sendMessage<T>(method: string, payload: T) {
    if (!socket || socket.readyState !== WebSocket.OPEN) {
      console.warn('WebSocket is not connected. Cannot send message:', method)
      return
    }

    const msg: WebSocketMessage<T> = { method, payload }
    socket.send(JSON.stringify(msg))

    messageHistory.value.push({
      ...msg,
      timestamp: Date.now(),
      direction: 'sent'
    })
  }

  function addMessageHandler(handler: MessageHandler) {
    messageHandlers.value.push(handler)
  }

  function removeMessageHandler(handler: MessageHandler) {
    const index = messageHandlers.value.indexOf(handler)
    if (index > -1) {
      messageHandlers.value.splice(index, 1)
    }
  }

  function clearMessageHistory() {
    messageHistory.value = []
  }

  return {
    isConnected,
    connect,
    disconnect,
    sendMessage,
    addMessageHandler,
    removeMessageHandler,
    messageHistory,
    clearMessageHistory
  }
})
