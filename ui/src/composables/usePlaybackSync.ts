import { onMounted, onUnmounted, type Ref } from 'vue'
import { useAudioStore } from '../stores/audio'
import { useWebSocketStore } from '../stores/websocket'

export function usePlaybackSync(audioElements: Ref<Record<string, HTMLAudioElement>>) {
  const wsStore = useWebSocketStore()
  const audioStore = useAudioStore()

  function handleSync(message: any) {
    if (message.method === 'sync' && message.payload.tracks) {
      // Get current track names before sync
      const previousTracks = new Set(Object.keys(audioStore.tracks))

      // Update store state
      audioStore.syncTracks(message.payload.tracks)

      // Get new track names after sync
      const newTracks = new Set(message.payload.tracks.map((t: any) => t.fileName))

      // Stop and cleanup removed audio elements
      previousTracks.forEach(fileName => {
        if (!newTracks.has(fileName)) {
          const audio = audioElements.value[fileName]
          if (audio) {
            audio.pause()
            audio.src = ''
            delete audioElements.value[fileName]
          }
        }
      })

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
