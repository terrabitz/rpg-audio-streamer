import { type AudioTrack } from '@/stores/audio'
import { onMounted, onUnmounted } from 'vue'
import { useAudioStore } from '../stores/audio'
import { useWebSocketStore } from '../stores/websocket'

export function wsHandlers() {
  const wsStore = useWebSocketStore()
  const audioStore = useAudioStore()

  function handleSyncAll(message: { method: string, payload: { tracks: AudioTrack[] } }) {
    if (message.method === 'syncAll' && message.payload.tracks) {
      audioStore.syncTracks(message.payload.tracks)
    }
  }

  function handleSyncTrack(message: { method: string, payload: Partial<AudioTrack> }) {
    if (message.method === 'syncTrack' && message.payload.fileID) {
      const { fileID, ...updates } = message.payload
      audioStore.updateTrackState(fileID, updates)
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
