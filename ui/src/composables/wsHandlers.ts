import { onMounted, onUnmounted } from 'vue'
import { useAudioStore } from '../stores/audio'
import { useWebSocketStore } from '../stores/websocket'

interface Track {
  fileName: string
  isPlaying: boolean
  currentTime: number
  volume: number
  isRepeating: boolean
}

interface SyncAllPayload {
  tracks: Track[]
}

interface SyncTrackPayload {
  fileName: string
  isPlaying?: boolean
  currentTime?: number
  volume?: number
  isRepeating?: boolean
}

export function wsHandlers() {
  const wsStore = useWebSocketStore()
  const audioStore = useAudioStore()

  function handleSyncAll(message: { method: string, payload: SyncAllPayload }) {
    if (message.method === 'syncAll' && message.payload.tracks) {
      audioStore.syncTracks(message.payload.tracks)
    }
  }

  function handleSyncTrack(message: { method: string, payload: SyncTrackPayload }) {
    if (message.method === 'syncTrack' && message.payload.fileName) {
      const { fileName, ...updates } = message.payload
      audioStore.updateTrackState(fileName, updates)
    }
  }

  onMounted(() => {
    wsStore.addMessageHandler(handleSyncAll)
    wsStore.addMessageHandler(handleSyncTrack)
  })

  onUnmounted(() => {
    wsStore.removeMessageHandler(handleSyncAll)
    wsStore.removeMessageHandler(handleSyncTrack)
  })
}
