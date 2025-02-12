import { onMounted, onUnmounted } from 'vue'
import { useAudioStore } from '../stores/audio'
import { useWebSocketStore } from '../stores/websocket'

export function usePlaybackSync() {
  const wsStore = useWebSocketStore()
  const audioStore = useAudioStore()

  function handleSync(message: any) {
    if (message.method === 'sync' && message.payload.tracks) {
      // First update all track states
      audioStore.syncTracks(message.payload.tracks)

      // Then ensure all tracks that should be playing are playing
      message.payload.tracks.forEach((track: any) => {
        if (track.fileName && track.isPlaying) {
          audioStore.updateTrackState(track.fileName, {
            isPlaying: true,
            currentTime: track.currentTime || 0,
            volume: track.volume || 100,
            isRepeating: track.isRepeating || false
          })
        }
      })
    }
  }

  onMounted(() => {
    wsStore.addMessageHandler(handleSync)
  })

  onUnmounted(() => {
    wsStore.removeMessageHandler(handleSync)
  })
}
