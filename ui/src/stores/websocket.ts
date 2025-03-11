import { defineStore } from 'pinia'
import { ref, shallowRef } from 'vue'
import { WSClient } from '../client/wsClient'

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
  const messageHandlers = ref<MessageHandler[]>([])
  const messageHistory = ref<StoredMessage[]>([])
  const client = shallowRef<WSClient>(new WSClient())

  function connect() {
    client.value.connect(
      // onOpen
      () => {
        isConnected.value = true
        console.log('WebSocket connected')
      },
      // onMessage
      (data) => {
        try {
          const message = JSON.parse(data) as WebSocketMessage
          const storedMessage: StoredMessage = {
            ...message,
            timestamp: Date.now(),
            direction: 'received'
          }
          messageHistory.value.push(storedMessage)
          messageHandlers.value.forEach(handler => handler(message))
        } catch (error) {
          console.error('Failed to parse WebSocket message:', data)
        }
      },
      // onClose
      () => {
        isConnected.value = false
        console.log('WebSocket disconnected')
        setTimeout(connect, 5000)
      },
      // onError
      (error) => {
        console.error('WebSocket error:', error)
        client.value.disconnect()
      }
    )
  }

  function disconnect() {
    client.value.disconnect()
    isConnected.value = false
  }

  function sendMessage<T>(method: string, payload: T) {
    if (isConnected.value) {
      const msg: WebSocketMessage<T> = { method, payload }
      client.value.sendMessage(msg)

      messageHistory.value.push({
        ...msg,
        timestamp: Date.now(),
        direction: 'sent'
      })
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
