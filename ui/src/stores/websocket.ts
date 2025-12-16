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

  async function connect(token?: string) {
    if (socket?.readyState === WebSocket.OPEN) {
      return
    }

    socket = await newSocket(token)
  }

  function newSocket(token?: string): Promise<WebSocket> {
    return new Promise((resolve, reject) => {
      const baseUrl = getWebSocketUrl()
      const url = token ? `${baseUrl}?token=${encodeURIComponent(token)}` : baseUrl
      let s = new WebSocket(url)

      s.onopen = () => {
        isConnected.value = true
        resolve(s)
        console.log('WebSocket connected')
      }

      s.onmessage = receiveMessage

      s.onclose = (event: CloseEvent) => {
        isConnected.value = false
        const reason = event.reason || 'No reason provided'
        const code = event.code
        console.log(`WebSocket disconnected: [${code}] ${reason}`)
        if (!event.wasClean) {
          setTimeout(() => connect(token), reconnectIntervalMs)
        }
      }

      s.onerror = (error) => {
        isConnected.value = false
        console.error('WebSocket error:', error)
        reject(error)
      }
    })
  }

  function receiveMessage<T>(event: MessageEvent) {
    try {
      const message = JSON.parse(event.data) as WebSocketMessage
      console.log('Received WebSocket message:', message)
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

  function sendMessage<T>(method: string, payload: T) {
    if (!socket || socket.readyState !== WebSocket.OPEN) {
      throw new Error('WebSocket is not connected')
    }

    const msg: WebSocketMessage<T> = { method, payload }
    console.log('Sending WebSocket message:', msg)
    socket.send(JSON.stringify(msg))

    messageHistory.value.push({
      ...msg,
      timestamp: Date.now(),
      direction: 'sent'
    })
  }

  function disconnect() {
    if (socket) {
      socket.close(1000, 'Client disconnected')
      socket = null
    }
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
