import { onMounted, onUnmounted, ref } from 'vue'

interface WSMessage {
  type: string
  payload: {
    fileName: string
    [key: string]: any
  }
}

function getWebSocketUrl(): string {
  const apiUrl = new URL(import.meta.env.VITE_API_BASE_URL)
  const wsProtocol = apiUrl.protocol === 'https:' ? 'wss:' : 'ws:'
  return `${wsProtocol}//${apiUrl.host}${apiUrl.pathname}/ws`
}

export function useWebSocket(onMessage: (message: WSMessage) => void) {
  const ws = ref<WebSocket | null>(null)
  const isConnected = ref(false)

  function connect() {
    const wsUrl = getWebSocketUrl()
    ws.value = new WebSocket(wsUrl)

    ws.value.onopen = () => {
      isConnected.value = true
    }

    ws.value.onclose = () => {
      isConnected.value = false
      setTimeout(connect, 1000)
    }

    ws.value.onmessage = (event) => {
      const data = JSON.parse(event.data)
      onMessage(data)
    }
  }

  function broadcast(type: string, fileName: string, payload: any = {}) {
    if (!isConnected.value || !ws.value) return

    ws.value.send(JSON.stringify({
      type,
      payload: {
        fileName,
        ...payload
      }
    }))
  }

  onMounted(() => {
    connect()
  })

  onUnmounted(() => {
    if (ws.value) {
      ws.value.close()
    }
  })

  return {
    isConnected,
    broadcast
  }
}
